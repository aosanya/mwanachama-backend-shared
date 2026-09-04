package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

const relationshipCols = `id, name, from_id, to_id, properties, created_at`

// CreateRelationship implements entitygraph.DataManager. A foreign-key
// violation against either endpoint (from_id/to_id must reference an
// existing entities row) is reported as entitygraph.ErrEntityNotFound.
func (b *Backend) CreateRelationship(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	propsJSON, err := jsonOrEmpty(req.Properties)
	if err != nil {
		return entitygraph.Relationship{}, err
	}
	q := fmt.Sprintf(`INSERT INTO %s (id, name, from_id, to_id, properties, created_at)
	                   VALUES ($1, $2, $3, $4, $5, now())
	                   RETURNING %s`, b.tables.Relationships, relationshipCols)
	out, err := scanRelationship(b.db.QueryRowContext(ctx, q, uuid.NewString(), req.Name, req.FromID, req.ToID, propsJSON))
	if err != nil {
		return entitygraph.Relationship{}, classify(err)
	}
	return out, nil
}

// DeleteRelationship implements entitygraph.DataManager. Edges are hard
// deleted — there is no soft-delete concept for relationships.
func (b *Backend) DeleteRelationship(ctx context.Context, relationshipID string) error {
	q := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, b.tables.Relationships)
	res, err := b.db.ExecContext(ctx, q, relationshipID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return entitygraph.ErrRelationshipNotFound
	}
	return nil
}

// ListRelationships implements entitygraph.DataManager. Zero-value filter
// fields are ignored (no filtering on that field).
func (b *Backend) ListRelationships(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s
	                   WHERE ($1 = '' OR from_id = $1)
	                     AND ($2 = '' OR to_id = $2)
	                     AND ($3 = '' OR name = $3)
	                   ORDER BY created_at, id`, relationshipCols, b.tables.Relationships)
	rows, err := b.db.QueryContext(ctx, q, filter.FromID, filter.ToID, filter.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entitygraph.Relationship{}
	for rows.Next() {
		r, err := scanRelationship(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
