package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// PerCollectionBackend is an alternative to [Backend]: instead of one
// shared `entities` table discriminated by a type_id column, each
// TypeDefinition gets its own physical table, named by its
// StorageCollection, and each relationship type gets its own physical
// table too, named after the two types it connects (relationshipEdges,
// e.g. "registered_at" from Member to Group gives table "memberXgroup"),
// with from_id/to_id as real foreign keys straight to those two content
// tables — no generic edge table, no id-registry involved in relationship
// storage at all.
//
// A small shared id-registry table (tables.Entities — see
// [DefaultPerCollectionTableNames]) still exists, but only for one job:
// GetEntity/UpdateEntity/DeleteEntity take no type argument, so they
// resolve which physical table to query with one indexed lookup
// (resolveTable) instead of probing every per-type table in turn.
//
// SchemaManager is inherited unchanged from the embedded Backend — it
// never touches entities or relationships. Every entitygraph.DataManager
// method is overridden below (CreateEntity, GetEntity, UpdateEntity,
// DeleteEntity, ListEntities, UpsertEntity), plus the relationship methods
// consumers embed locally (CreateRelationship, DeleteRelationship,
// ListRelationships — see entitygraph.DataManager's doc).
//
// Don't reach for this by default — see [Backend]'s own doc and
// mwanachama-backend-shared/CLAUDE.md's "one entities table, not one per
// type" invariant. Use it only when a consumer needs the physical tables
// split (e.g. mwanachama-backend-actor's "member" instance, DSN-1700).
type PerCollectionBackend struct {
	*Backend
	tableFor     map[string]string // TypeDefinition.Name -> physical entity table
	allTables    []string          // distinct StorageCollection values, schema order
	idsTable     string            // the id-registry table (tables.Entities)
	relTableFor  map[string]string // RelationshipDefinition.Name -> physical relationship table
	allRelTables []string          // distinct relationship tables, schema order
}

// NewPerCollectionBackend constructs a PerCollectionBackend over db. tables
// should come from [DefaultPerCollectionTableNames] (tables.Entities names
// the id-registry table, not a content table; tables.Relationships is
// unused — relationships live in their own per-pair tables, named by
// relationshipEdges). One physical table per distinct
// TypeDefinition.StorageCollection in sch holds entity content. Returns an
// error if any type in sch has no StorageCollection set. Does not verify
// the tables exist — run a migration built from [PerCollectionDDL] first.
func NewPerCollectionBackend(db *sql.DB, tables TableNames, sch schema.Schema) (*PerCollectionBackend, error) {
	if tables.Entities == "" {
		return nil, fmt.Errorf("NewPerCollectionBackend: tables.Entities must name the id-registry table")
	}
	tableFor := make(map[string]string, len(sch.Types))
	for _, td := range sch.Types {
		if td.StorageCollection == "" {
			return nil, fmt.Errorf("NewPerCollectionBackend: type %q has no StorageCollection", td.Name)
		}
		tableFor[td.Name] = td.StorageCollection
	}

	edges := relationshipEdges(sch)
	relTableFor := make(map[string]string, len(edges))
	seenRelTable := make(map[string]bool, len(edges))
	var allRelTables []string
	for _, e := range edges {
		relTableFor[e.relName] = e.table
		if !seenRelTable[e.table] {
			seenRelTable[e.table] = true
			allRelTables = append(allRelTables, e.table)
		}
	}

	return &PerCollectionBackend{
		Backend:      NewBackend(db, tables),
		tableFor:     tableFor,
		allTables:    distinctCollections(sch),
		idsTable:     tables.Entities,
		relTableFor:  relTableFor,
		allRelTables: allRelTables,
	}, nil
}

// resolveTable looks up which physical table entityID lives in, via the
// id registry — one indexed lookup instead of probing every per-type
// table. Returns entitygraph.ErrEntityNotFound if entityID is unregistered.
func (b *PerCollectionBackend) resolveTable(ctx context.Context, entityID string) (string, error) {
	q := fmt.Sprintf(`SELECT type_id FROM %s WHERE id = $1`, b.idsTable)
	var typeID string
	err := b.db.QueryRowContext(ctx, q, entityID).Scan(&typeID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", entitygraph.ErrEntityNotFound
	}
	if err != nil {
		return "", err
	}
	table, ok := b.tableFor[typeID]
	if !ok {
		return "", fmt.Errorf("resolveTable: id %s registered under unknown type %q", entityID, typeID)
	}
	return table, nil
}

// CreateEntity implements entitygraph.DataManager, routed to
// tableFor[req.TypeID] and registered in the id registry in the same
// transaction — see [Backend.CreateEntity] for the shape this mirrors.
func (b *PerCollectionBackend) CreateEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	table, ok := b.tableFor[req.TypeID]
	if !ok {
		return entitygraph.Entity{}, fmt.Errorf("CreateEntity: type %q has no table", req.TypeID)
	}
	propsJSON, err := jsonOrEmpty(req.Properties)
	if err != nil {
		return entitygraph.Entity{}, err
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	defer func() { _ = tx.Rollback() }()

	id := uuid.NewString()
	idQ := fmt.Sprintf(`INSERT INTO %s (id, type_id) VALUES ($1, $2)`, b.idsTable)
	if _, err := tx.ExecContext(ctx, idQ, id, req.TypeID); err != nil {
		return entitygraph.Entity{}, classify(err)
	}

	q := fmt.Sprintf(`INSERT INTO %s (id, type_id, properties, created_at, updated_at)
	                   VALUES ($1, $2, $3, now(), now())
	                   RETURNING %s`, table, entityCols)
	out, err := scanEntity(tx.QueryRowContext(ctx, q, id, req.TypeID, propsJSON))
	if err != nil {
		return entitygraph.Entity{}, classify(err)
	}

	// Each relationship's own table (relTableFor[rel.Name]) carries a real
	// FK straight to the two content tables it connects — an unknown
	// rel.ToID rejects this INSERT on its own, classified below exactly
	// like a direct CreateRelationship call would.
	for _, rel := range req.Relationships {
		relTable, ok := b.relTableFor[rel.Name]
		if !ok {
			return entitygraph.Entity{}, fmt.Errorf("CreateEntity: relationship %q has no table", rel.Name)
		}
		relQ := fmt.Sprintf(`INSERT INTO %s (id, name, from_id, to_id, properties, created_at)
		                      VALUES ($1, $2, $3, $4, '{}'::jsonb, now())`, relTable)
		if _, err := tx.ExecContext(ctx, relQ, uuid.NewString(), rel.Name, out.ID, rel.ToID); err != nil {
			return entitygraph.Entity{}, classify(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return entitygraph.Entity{}, err
	}
	return out, nil
}

// GetEntity implements entitygraph.DataManager.
func (b *PerCollectionBackend) GetEntity(ctx context.Context, entityID string) (entitygraph.Entity, error) {
	table, err := b.resolveTable(ctx, entityID)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE id = $1 AND NOT deleted`, entityCols, table)
	out, err := scanEntity(b.db.QueryRowContext(ctx, q, entityID))
	if errors.Is(err, sql.ErrNoRows) {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if err != nil {
		return entitygraph.Entity{}, err
	}
	return out, nil
}

// UpdateEntity implements entitygraph.DataManager.
func (b *PerCollectionBackend) UpdateEntity(ctx context.Context, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	table, err := b.resolveTable(ctx, entityID)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	patchJSON, err := jsonOrEmpty(req.Properties)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	q := fmt.Sprintf(`UPDATE %s SET properties = properties || $2::jsonb, updated_at = now()
	                   WHERE id = $1 AND NOT deleted
	                   RETURNING %s`, table, entityCols)
	out, err := scanEntity(b.db.QueryRowContext(ctx, q, entityID, patchJSON))
	if errors.Is(err, sql.ErrNoRows) {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if err != nil {
		return entitygraph.Entity{}, err
	}
	return out, nil
}

// DeleteEntity implements entitygraph.DataManager (soft delete). The id
// registry row is left in place — matching Backend.DeleteEntity, which
// also leaves the (soft-deleted) entities row and any relationships
// referencing it, per the interface contract.
func (b *PerCollectionBackend) DeleteEntity(ctx context.Context, entityID string) error {
	table, err := b.resolveTable(ctx, entityID)
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`UPDATE %s SET deleted = true, deleted_at = now(), updated_at = now()
	                   WHERE id = $1 AND NOT deleted`, table)
	res, err := b.db.ExecContext(ctx, q, entityID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return entitygraph.ErrEntityNotFound
	}
	return nil
}

// ListEntities implements entitygraph.DataManager. filter.TypeID set
// routes to that one table, unchanged from Backend.ListEntities' shape.
// Unset queries every table (the registry has no use here — a list
// returns full rows, not an existence check) and merges, re-sorted by
// (created_at, id) since each table's own result is only locally ordered.
func (b *PerCollectionBackend) ListEntities(ctx context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	var propsFilter any
	if len(filter.Properties) > 0 {
		raw, err := json.Marshal(filter.Properties)
		if err != nil {
			return nil, err
		}
		propsFilter = raw
	}

	tables := b.allTables
	if filter.TypeID != "" {
		table, ok := b.tableFor[filter.TypeID]
		if !ok {
			return []entitygraph.Entity{}, nil
		}
		tables = []string{table}
	}

	out := []entitygraph.Entity{}
	for _, table := range tables {
		q := fmt.Sprintf(`SELECT %s FROM %s
		                   WHERE NOT deleted
		                     AND ($1 = '' OR type_id = $1)
		                     AND ($2::jsonb IS NULL OR properties @> $2::jsonb)
		                   ORDER BY created_at, id`, entityCols, table)
		rows, err := b.db.QueryContext(ctx, q, filter.TypeID, propsFilter)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			e, err := scanEntity(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, e)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if len(tables) > 1 {
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].CreatedAt.Equal(out[j].CreatedAt) {
				return out[i].ID < out[j].ID
			}
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		})
	}
	return out, nil
}

// UpsertEntity implements entitygraph.DataManager, routed to
// tableFor[req.TypeID], then registers the FINAL id (new or the existing
// row ON CONFLICT resolved to) in the id registry, idempotently — see
// [Backend.UpsertEntity] for the shape this mirrors.
func (b *PerCollectionBackend) UpsertEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	table, ok := b.tableFor[req.TypeID]
	if !ok {
		return entitygraph.Entity{}, fmt.Errorf("UpsertEntity: type %q has no table", req.TypeID)
	}
	active, err := b.GetActive(ctx)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	td, err := entitygraph.FindTypeDef(active, req.TypeID)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	if len(td.UniqueKey) == 0 {
		return entitygraph.Entity{}, entitygraph.ErrUniqueKeyNotDefined
	}

	keyVals := make([]any, len(td.UniqueKey))
	for i, field := range td.UniqueKey {
		keyVals[i] = req.Properties[field]
	}
	uniqueKey, err := json.Marshal(keyVals)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	propsJSON, err := jsonOrEmpty(req.Properties)
	if err != nil {
		return entitygraph.Entity{}, err
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	defer func() { _ = tx.Rollback() }()

	q := fmt.Sprintf(`INSERT INTO %[1]s (id, type_id, properties, unique_key, created_at, updated_at)
	                   VALUES ($1, $2, $3, $4, now(), now())
	                   ON CONFLICT (type_id, unique_key) WHERE unique_key IS NOT NULL AND NOT deleted
	                   DO UPDATE SET properties = %[1]s.properties || EXCLUDED.properties, updated_at = now()
	                   RETURNING %[2]s`, table, entityCols)
	out, err := scanEntity(tx.QueryRowContext(ctx, q, uuid.NewString(), req.TypeID, propsJSON, string(uniqueKey)))
	if err != nil {
		return entitygraph.Entity{}, classify(err)
	}

	// out.ID is the freshly minted id on a new row, or the pre-existing
	// row's id when ON CONFLICT resolved to one — either way it's already
	// registered unless this is the new-row case, so DO NOTHING is the
	// idempotent, correct move here rather than assuming which happened.
	idQ := fmt.Sprintf(`INSERT INTO %s (id, type_id) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, b.idsTable)
	if _, err := tx.ExecContext(ctx, idQ, out.ID, req.TypeID); err != nil {
		return entitygraph.Entity{}, classify(err)
	}

	if err := tx.Commit(); err != nil {
		return entitygraph.Entity{}, err
	}
	return out, nil
}

// CreateRelationship implements entitygraph.DataManager, routed to
// relTableFor[req.Name]. from_id/to_id carry a real FK straight to the two
// content tables that relationship connects (PerCollectionDDL) — an
// unknown endpoint rejects the INSERT and classify maps it to
// entitygraph.ErrEntityNotFound, the same error a single-table Backend
// reports for the same case.
func (b *PerCollectionBackend) CreateRelationship(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	table, ok := b.relTableFor[req.Name]
	if !ok {
		return entitygraph.Relationship{}, fmt.Errorf("CreateRelationship: relationship %q has no table", req.Name)
	}
	propsJSON, err := jsonOrEmpty(req.Properties)
	if err != nil {
		return entitygraph.Relationship{}, err
	}
	q := fmt.Sprintf(`INSERT INTO %s (id, name, from_id, to_id, properties, created_at)
	                   VALUES ($1, $2, $3, $4, $5, now())
	                   RETURNING %s`, table, relationshipCols)
	out, err := scanRelationship(b.db.QueryRowContext(ctx, q, uuid.NewString(), req.Name, req.FromID, req.ToID, propsJSON))
	if err != nil {
		return entitygraph.Relationship{}, classify(err)
	}
	return out, nil
}

// DeleteRelationship implements entitygraph.DataManager (hard delete) — same
// fan-out-over-allRelTables shape ListRelationships uses for its unfiltered
// case.
func (b *PerCollectionBackend) DeleteRelationship(ctx context.Context, relationshipID string) error {
	for _, table := range b.allRelTables {
		q := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, table)
		res, err := b.db.ExecContext(ctx, q, relationshipID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
	}
	return entitygraph.ErrRelationshipNotFound
}

// ListRelationships implements entitygraph.DataManager. filter.Name set
// routes to that one table. Unset queries every relationship table and
// merges, re-sorted by (created_at, id) since each table's own result is
// only locally ordered — same shape as ListEntities' unfiltered case.
func (b *PerCollectionBackend) ListRelationships(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	tables := b.allRelTables
	if filter.Name != "" {
		table, ok := b.relTableFor[filter.Name]
		if !ok {
			return []entitygraph.Relationship{}, nil
		}
		tables = []string{table}
	}

	out := []entitygraph.Relationship{}
	for _, table := range tables {
		q := fmt.Sprintf(`SELECT %s FROM %s
		                   WHERE ($1 = '' OR from_id = $1)
		                     AND ($2 = '' OR to_id = $2)
		                     AND ($3 = '' OR name = $3)
		                   ORDER BY created_at, id`, relationshipCols, table)
		rows, err := b.db.QueryContext(ctx, q, filter.FromID, filter.ToID, filter.Name)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			r, err := scanRelationship(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if len(tables) > 1 {
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].CreatedAt.Equal(out[j].CreatedAt) {
				return out[i].ID < out[j].ID
			}
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		})
	}
	return out, nil
}
