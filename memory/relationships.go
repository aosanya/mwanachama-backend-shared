package memory

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// sortRelationships orders results deterministically (created_at, then id)
// to match the Postgres backend's ORDER BY.
func sortRelationships(rs []entitygraph.Relationship) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].CreatedAt.Equal(rs[j].CreatedAt) {
			return rs[i].ID < rs[j].ID
		}
		return rs[i].CreatedAt.Before(rs[j].CreatedAt)
	})
}

// CreateRelationship implements entitygraph.DataManager. Mirrors the
// Postgres backend's foreign-key check: both endpoints must exist and be
// non-deleted.
func (b *Backend) CreateRelationship(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	from, ok := b.entities[req.FromID]
	if !ok || from.Deleted {
		return entitygraph.Relationship{}, entitygraph.ErrEntityNotFound
	}
	to, ok := b.entities[req.ToID]
	if !ok || to.Deleted {
		return entitygraph.Relationship{}, entitygraph.ErrEntityNotFound
	}

	r := entitygraph.Relationship{
		ID:         uuid.NewString(),
		Name:       req.Name,
		FromID:     req.FromID,
		ToID:       req.ToID,
		Properties: cloneProps(req.Properties),
		CreatedAt:  nowUTC(),
	}
	b.relationships[r.ID] = r
	return r, nil
}

// DeleteRelationship implements entitygraph.DataManager.
func (b *Backend) DeleteRelationship(ctx context.Context, relationshipID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, ok := b.relationships[relationshipID]
	if !ok {
		return entitygraph.ErrRelationshipNotFound
	}
	delete(b.relationships, relationshipID)
	return nil
}

// ListRelationships implements entitygraph.DataManager.
func (b *Backend) ListRelationships(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := []entitygraph.Relationship{}
	for _, r := range b.relationships {
		if filter.FromID != "" && r.FromID != filter.FromID {
			continue
		}
		if filter.ToID != "" && r.ToID != filter.ToID {
			continue
		}
		if filter.Name != "" && r.Name != filter.Name {
			continue
		}
		out = append(out, r)
	}
	sortRelationships(out)
	return out, nil
}
