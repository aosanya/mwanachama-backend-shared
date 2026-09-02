package memory

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/google/uuid"

	"github.com/aosanya/mwanachama-go-shared/entitygraph"
)

// CreateEntity implements entitygraph.DataManager. See postgres.Backend's
// CreateEntity doc for why inline req.Relationships are stored without
// schema validation — that's the caller's job.
func (b *Backend) CreateEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := nowUTC()
	e := entitygraph.Entity{
		ID:         uuid.NewString(),
		AgencyID:   req.AgencyID,
		TypeID:     req.TypeID,
		Properties: cloneProps(req.Properties),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	b.entities[e.ID] = e

	for _, rel := range req.Relationships {
		r := entitygraph.Relationship{
			ID:        uuid.NewString(),
			AgencyID:  req.AgencyID,
			Name:      rel.Name,
			FromID:    e.ID,
			ToID:      rel.ToID,
			CreatedAt: now,
		}
		b.relationships[r.ID] = r
	}

	return e, nil
}

// GetEntity implements entitygraph.DataManager.
func (b *Backend) GetEntity(ctx context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	e, ok := b.entities[entityID]
	if !ok || e.AgencyID != agencyID || e.Deleted {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	return e, nil
}

// UpdateEntity implements entitygraph.DataManager.
func (b *Backend) UpdateEntity(ctx context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	e, ok := b.entities[entityID]
	if !ok || e.AgencyID != agencyID || e.Deleted {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if e.Properties == nil {
		e.Properties = map[string]any{}
	}
	for k, v := range req.Properties {
		e.Properties[k] = v
	}
	e.UpdatedAt = nowUTC()
	b.entities[entityID] = e
	return e, nil
}

// DeleteEntity implements entitygraph.DataManager (soft delete).
func (b *Backend) DeleteEntity(ctx context.Context, agencyID, entityID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	e, ok := b.entities[entityID]
	if !ok || e.AgencyID != agencyID || e.Deleted {
		return entitygraph.ErrEntityNotFound
	}
	now := nowUTC()
	e.Deleted = true
	e.DeletedAt = &now
	e.UpdatedAt = now
	b.entities[entityID] = e
	return nil
}

// ListEntities implements entitygraph.DataManager.
func (b *Backend) ListEntities(ctx context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := []entitygraph.Entity{}
	for _, e := range b.entities {
		if e.Deleted {
			continue
		}
		if filter.AgencyID != "" && e.AgencyID != filter.AgencyID {
			continue
		}
		if filter.TypeID != "" && e.TypeID != filter.TypeID {
			continue
		}
		if !propsContain(e.Properties, filter.Properties) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// UpsertEntity implements entitygraph.DataManager: looks up the agency's
// active schema for req.TypeID's UniqueKey, then scans for a non-deleted
// entity of the same type whose properties match on every key field.
func (b *Backend) UpsertEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	active, err := b.GetActive(ctx, req.AgencyID)
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

	b.mu.Lock()
	defer b.mu.Unlock()

	for id, e := range b.entities {
		if e.Deleted || e.AgencyID != req.AgencyID || e.TypeID != req.TypeID {
			continue
		}
		if uniqueKeyMatches(e.Properties, req.Properties, td.UniqueKey) {
			if e.Properties == nil {
				e.Properties = map[string]any{}
			}
			for k, v := range req.Properties {
				e.Properties[k] = v
			}
			e.UpdatedAt = nowUTC()
			b.entities[id] = e
			return e, nil
		}
	}

	now := nowUTC()
	e := entitygraph.Entity{
		ID:         uuid.NewString(),
		AgencyID:   req.AgencyID,
		TypeID:     req.TypeID,
		Properties: cloneProps(req.Properties),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	b.entities[e.ID] = e
	return e, nil
}

// uniqueKeyMatches reports whether existing carries the same values as
// candidate for every field in key (nil == nil counts as a match, mirroring
// how the Postgres backend serializes a missing field as JSON null).
func uniqueKeyMatches(existing, candidate map[string]any, key []string) bool {
	for _, field := range key {
		if !equalJSON(existing[field], candidate[field]) {
			return false
		}
	}
	return true
}

// equalJSON compares two property values the same way the Postgres backend
// effectively does (by their JSON representation), so callers can't observe
// a false negative from Go's own equality rules on differing numeric types
// (e.g. int vs float64).
func equalJSON(a, b any) bool {
	aj, aerr := json.Marshal(a)
	bj, berr := json.Marshal(b)
	if aerr != nil || berr != nil {
		return false
	}
	return string(aj) == string(bj)
}

func cloneProps(props map[string]any) map[string]any {
	if props == nil {
		return nil
	}
	out := make(map[string]any, len(props))
	for k, v := range props {
		out[k] = v
	}
	return out
}

// propsContain reports whether entity contains every key/value in filter.
func propsContain(entity, filter map[string]any) bool {
	for k, v := range filter {
		if !equalJSON(entity[k], v) {
			return false
		}
	}
	return true
}
