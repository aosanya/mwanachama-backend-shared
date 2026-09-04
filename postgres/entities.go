package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

const entityCols = `id, type_id, properties, created_at, updated_at, deleted, deleted_at`

// CreateEntity implements entitygraph.DataManager. Inline
// req.Relationships (if any) are inserted as relationship rows in the same
// transaction — FromID is the new entity, ToID/Name come from each request
// entry. Schema-level validation (that a label is declared, that Required
// relationships are all present) is the caller's job, via
// entitygraph.FindTypeDef/ValidateCreateRelationship — CreateEntity is a
// storage operation, not a schema-aware one, matching how
// CodeValdGit/CodeValdWork call those helpers themselves before invoking
// DataManager methods.
func (b *Backend) CreateEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
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
	q := fmt.Sprintf(`INSERT INTO %s (id, type_id, properties, created_at, updated_at)
	                   VALUES ($1, $2, $3, now(), now())
	                   RETURNING %s`, b.tables.Entities, entityCols)
	out, err := scanEntity(tx.QueryRowContext(ctx, q, id, req.TypeID, propsJSON))
	if err != nil {
		return entitygraph.Entity{}, classify(err)
	}

	for _, rel := range req.Relationships {
		relQ := fmt.Sprintf(`INSERT INTO %s (id, name, from_id, to_id, properties, created_at)
		                      VALUES ($1, $2, $3, $4, '{}'::jsonb, now())`, b.tables.Relationships)
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
func (b *Backend) GetEntity(ctx context.Context, entityID string) (entitygraph.Entity, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE id = $1 AND NOT deleted`, entityCols, b.tables.Entities)
	out, err := scanEntity(b.db.QueryRowContext(ctx, q, entityID))
	if errors.Is(err, sql.ErrNoRows) {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if err != nil {
		return entitygraph.Entity{}, err
	}
	return out, nil
}

// UpdateEntity implements entitygraph.DataManager. Only the keys present in
// req.Properties are patched — existing keys not mentioned are left
// unchanged (jsonb `||` semantics).
func (b *Backend) UpdateEntity(ctx context.Context, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	patchJSON, err := jsonOrEmpty(req.Properties)
	if err != nil {
		return entitygraph.Entity{}, err
	}
	q := fmt.Sprintf(`UPDATE %s SET properties = properties || $2::jsonb, updated_at = now()
	                   WHERE id = $1 AND NOT deleted
	                   RETURNING %s`, b.tables.Entities, entityCols)
	out, err := scanEntity(b.db.QueryRowContext(ctx, q, entityID, patchJSON))
	if errors.Is(err, sql.ErrNoRows) {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if err != nil {
		return entitygraph.Entity{}, err
	}
	return out, nil
}

// DeleteEntity implements entitygraph.DataManager (soft delete).
// Relationships referencing the entity are left in place as orphans, per
// the interface contract.
func (b *Backend) DeleteEntity(ctx context.Context, entityID string) error {
	q := fmt.Sprintf(`UPDATE %s SET deleted = true, deleted_at = now(), updated_at = now()
	                   WHERE id = $1 AND NOT deleted`, b.tables.Entities)
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

// ListEntities implements entitygraph.DataManager. Zero-value filter fields
// are ignored; filter.Properties, when set, restricts results to entities
// whose properties contain every given key/value (jsonb containment).
func (b *Backend) ListEntities(ctx context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	var propsFilter any
	if len(filter.Properties) > 0 {
		raw, err := json.Marshal(filter.Properties)
		if err != nil {
			return nil, err
		}
		propsFilter = raw
	}
	q := fmt.Sprintf(`SELECT %s FROM %s
	                   WHERE NOT deleted
	                     AND ($1 = '' OR type_id = $1)
	                     AND ($2::jsonb IS NULL OR properties @> $2::jsonb)
	                   ORDER BY created_at, id`, entityCols, b.tables.Entities)
	rows, err := b.db.QueryContext(ctx, q, filter.TypeID, propsFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entitygraph.Entity{}
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertEntity implements entitygraph.DataManager. Looks up the active
// schema to find req.TypeID's UniqueKey, computes a deterministic key from
// those properties, and lets Postgres's ON CONFLICT do the find-or-merge
// atomically (entities_unique_key_idx is the conflict target).
func (b *Backend) UpsertEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
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

	q := fmt.Sprintf(`INSERT INTO %[1]s (id, type_id, properties, unique_key, created_at, updated_at)
	                   VALUES ($1, $2, $3, $4, now(), now())
	                   ON CONFLICT (type_id, unique_key) WHERE unique_key IS NOT NULL AND NOT deleted
	                   DO UPDATE SET properties = %[1]s.properties || EXCLUDED.properties, updated_at = now()
	                   RETURNING %[2]s`, b.tables.Entities, entityCols)
	out, err := scanEntity(b.db.QueryRowContext(ctx, q, uuid.NewString(), req.TypeID, propsJSON, string(uniqueKey)))
	if err != nil {
		return entitygraph.Entity{}, classify(err)
	}
	return out, nil
}
