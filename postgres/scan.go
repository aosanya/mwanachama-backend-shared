package postgres

import (
	"database/sql"
	"encoding/json"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// scanner is satisfied by both *sql.Row and *sql.Rows, so row-scanning code
// works unchanged whichever one a query returns.
type scanner interface {
	Scan(dest ...any) error
}

// scanEntity reads one row in the column order CreateEntity/GetEntity/
// ListEntities/UpsertEntity all select in: id, type_id, properties,
// created_at, updated_at, deleted, deleted_at.
func scanEntity(s scanner) (entitygraph.Entity, error) {
	var (
		e         entitygraph.Entity
		propsRaw  []byte
		deletedAt sql.NullTime
	)
	if err := s.Scan(&e.ID, &e.TypeID, &propsRaw, &e.CreatedAt, &e.UpdatedAt, &e.Deleted, &deletedAt); err != nil {
		return entitygraph.Entity{}, err
	}
	if len(propsRaw) > 0 {
		if err := json.Unmarshal(propsRaw, &e.Properties); err != nil {
			return entitygraph.Entity{}, err
		}
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		e.DeletedAt = &t
	}
	return e, nil
}

// scanRelationship reads one row in the column order CreateRelationship/
// GetRelationship/ListRelationships all select in: id, name, from_id,
// to_id, properties, created_at.
func scanRelationship(s scanner) (entitygraph.Relationship, error) {
	var (
		r        entitygraph.Relationship
		propsRaw []byte
	)
	if err := s.Scan(&r.ID, &r.Name, &r.FromID, &r.ToID, &propsRaw, &r.CreatedAt); err != nil {
		return entitygraph.Relationship{}, err
	}
	if len(propsRaw) > 0 {
		if err := json.Unmarshal(propsRaw, &r.Properties); err != nil {
			return entitygraph.Relationship{}, err
		}
	}
	return r, nil
}

// jsonOrEmpty marshals props to JSON, using an empty object for a nil/empty
// map so the properties column is never NULL.
func jsonOrEmpty(props map[string]any) ([]byte, error) {
	if len(props) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(props)
}
