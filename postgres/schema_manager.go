package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// SetSchema implements entitygraph.SchemaManager. Overwrites the agency's
// draft in place — ValidateSchema is intentionally not called here, per the
// interface contract (invalid drafts are allowed until Publish).
func (b *Backend) SetSchema(ctx context.Context, s schema.Schema) error {
	doc, err := json.Marshal(s)
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`INSERT INTO %[1]s (agency_id, document, updated_at)
	                   VALUES ($1, $2, now())
	                   ON CONFLICT (agency_id) DO UPDATE SET document = EXCLUDED.document, updated_at = now()`,
		b.tables.SchemaDrafts)
	_, err = b.db.ExecContext(ctx, q, s.AgencyID, doc)
	return err
}

// GetSchema implements entitygraph.SchemaManager.
func (b *Backend) GetSchema(ctx context.Context, agencyID string) (schema.Schema, error) {
	q := fmt.Sprintf(`SELECT document FROM %s WHERE agency_id = $1`, b.tables.SchemaDrafts)
	var doc []byte
	err := b.db.QueryRowContext(ctx, q, agencyID).Scan(&doc)
	if errors.Is(err, sql.ErrNoRows) {
		return schema.Schema{}, entitygraph.ErrSchemaNotFound
	}
	if err != nil {
		return schema.Schema{}, err
	}
	var s schema.Schema
	if err := json.Unmarshal(doc, &s); err != nil {
		return schema.Schema{}, err
	}
	return s, nil
}

// Publish implements entitygraph.SchemaManager: validates the current draft
// and snapshots it as a new inactive published version (highest existing +
// 1; first publish = 1).
func (b *Backend) Publish(ctx context.Context, agencyID string) error {
	draft, err := b.GetSchema(ctx, agencyID)
	if err != nil {
		return err
	}
	if err := entitygraph.ValidateSchema(draft); err != nil {
		return err
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var nextVersion int
	q := fmt.Sprintf(`SELECT COALESCE(MAX(version), 0) + 1 FROM %s WHERE agency_id = $1`, b.tables.SchemaVersions)
	if err := tx.QueryRowContext(ctx, q, agencyID).Scan(&nextVersion); err != nil {
		return err
	}

	draft.Version = nextVersion
	draft.Active = false
	doc, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	insertQ := fmt.Sprintf(`INSERT INTO %s (agency_id, version, document, active, created_at)
	                         VALUES ($1, $2, $3, false, now())`, b.tables.SchemaVersions)
	if _, err := tx.ExecContext(ctx, insertQ, agencyID, nextVersion, doc); err != nil {
		return classify(err)
	}

	return tx.Commit()
}

// Activate implements entitygraph.SchemaManager: flips the previously
// active version (if any) off and the requested version on inside a single
// transaction. schema_versions_one_active_idx is the backstop guaranteeing
// at most one active row per agency even if this ever raced.
func (b *Backend) Activate(ctx context.Context, agencyID string, version int) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	deactivateQ := fmt.Sprintf(`UPDATE %s SET active = false WHERE agency_id = $1 AND active`, b.tables.SchemaVersions)
	if _, err := tx.ExecContext(ctx, deactivateQ, agencyID); err != nil {
		return err
	}

	activateQ := fmt.Sprintf(`UPDATE %s SET active = true WHERE agency_id = $1 AND version = $2`, b.tables.SchemaVersions)
	res, err := tx.ExecContext(ctx, activateQ, agencyID, version)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return entitygraph.ErrSchemaNotFound
	}

	return tx.Commit()
}

// GetActive implements entitygraph.SchemaManager.
func (b *Backend) GetActive(ctx context.Context, agencyID string) (schema.Schema, error) {
	q := fmt.Sprintf(`SELECT document FROM %s WHERE agency_id = $1 AND active`, b.tables.SchemaVersions)
	return b.scanOneSchemaVersion(ctx, q, agencyID)
}

// GetVersion implements entitygraph.SchemaManager.
func (b *Backend) GetVersion(ctx context.Context, agencyID string, version int) (schema.Schema, error) {
	q := fmt.Sprintf(`SELECT document FROM %s WHERE agency_id = $1 AND version = $2`, b.tables.SchemaVersions)
	return b.scanOneSchemaVersion(ctx, q, agencyID, version)
}

func (b *Backend) scanOneSchemaVersion(ctx context.Context, q string, args ...any) (schema.Schema, error) {
	var doc []byte
	err := b.db.QueryRowContext(ctx, q, args...).Scan(&doc)
	if errors.Is(err, sql.ErrNoRows) {
		return schema.Schema{}, entitygraph.ErrSchemaNotFound
	}
	if err != nil {
		return schema.Schema{}, err
	}
	var s schema.Schema
	if err := json.Unmarshal(doc, &s); err != nil {
		return schema.Schema{}, err
	}
	return s, nil
}

// ListVersions implements entitygraph.SchemaManager, ascending version
// order.
func (b *Backend) ListVersions(ctx context.Context, agencyID string) ([]schema.Schema, error) {
	q := fmt.Sprintf(`SELECT document FROM %s WHERE agency_id = $1 ORDER BY version`, b.tables.SchemaVersions)
	rows, err := b.db.QueryContext(ctx, q, agencyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []schema.Schema{}
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var s schema.Schema
		if err := json.Unmarshal(doc, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
