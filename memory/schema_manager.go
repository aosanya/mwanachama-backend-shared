package memory

import (
	"context"

	"github.com/aosanya/mwanachama-go-shared/entitygraph"
	"github.com/aosanya/mwanachama-go-shared/schema"
)

// SetSchema implements entitygraph.SchemaManager.
func (b *Backend) SetSchema(ctx context.Context, s schema.Schema) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.schemaDrafts[s.AgencyID] = s
	return nil
}

// GetSchema implements entitygraph.SchemaManager.
func (b *Backend) GetSchema(ctx context.Context, agencyID string) (schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.schemaDrafts[agencyID]
	if !ok {
		return schema.Schema{}, entitygraph.ErrSchemaNotFound
	}
	return s, nil
}

// Publish implements entitygraph.SchemaManager.
func (b *Backend) Publish(ctx context.Context, agencyID string) error {
	draft, err := b.GetSchema(ctx, agencyID)
	if err != nil {
		return err
	}
	if err := entitygraph.ValidateSchema(draft); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft.Version = len(b.schemaVersions[agencyID]) + 1
	draft.Active = false
	b.schemaVersions[agencyID] = append(b.schemaVersions[agencyID], draft)
	return nil
}

// Activate implements entitygraph.SchemaManager.
func (b *Backend) Activate(ctx context.Context, agencyID string, version int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	versions := b.schemaVersions[agencyID]
	idx := -1
	for i, s := range versions {
		if s.Version == version {
			idx = i
		}
	}
	if idx == -1 {
		return entitygraph.ErrSchemaNotFound
	}
	for i := range versions {
		versions[i].Active = i == idx
	}
	return nil
}

// GetActive implements entitygraph.SchemaManager.
func (b *Backend) GetActive(ctx context.Context, agencyID string) (schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.schemaVersions[agencyID] {
		if s.Active {
			return s, nil
		}
	}
	return schema.Schema{}, entitygraph.ErrSchemaNotFound
}

// GetVersion implements entitygraph.SchemaManager.
func (b *Backend) GetVersion(ctx context.Context, agencyID string, version int) (schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.schemaVersions[agencyID] {
		if s.Version == version {
			return s, nil
		}
	}
	return schema.Schema{}, entitygraph.ErrSchemaNotFound
}

// ListVersions implements entitygraph.SchemaManager, ascending version
// order (the order versions were appended in).
func (b *Backend) ListVersions(ctx context.Context, agencyID string) ([]schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]schema.Schema, len(b.schemaVersions[agencyID]))
	copy(out, b.schemaVersions[agencyID])
	return out, nil
}
