package memory

import (
	"context"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// SetSchema implements entitygraph.SchemaManager.
func (b *Backend) SetSchema(ctx context.Context, s schema.Schema) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.schemaDraft = &s
	return nil
}

// GetSchema implements entitygraph.SchemaManager.
func (b *Backend) GetSchema(ctx context.Context) (schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.schemaDraft == nil {
		return schema.Schema{}, entitygraph.ErrSchemaNotFound
	}
	return *b.schemaDraft, nil
}

// Publish implements entitygraph.SchemaManager.
func (b *Backend) Publish(ctx context.Context) error {
	draft, err := b.GetSchema(ctx)
	if err != nil {
		return err
	}
	if err := entitygraph.ValidateSchema(draft); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft.Version = len(b.schemaVersions) + 1
	draft.Active = false
	b.schemaVersions = append(b.schemaVersions, draft)
	return nil
}

// Activate implements entitygraph.SchemaManager.
func (b *Backend) Activate(ctx context.Context, version int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	idx := -1
	for i, s := range b.schemaVersions {
		if s.Version == version {
			idx = i
		}
	}
	if idx == -1 {
		return entitygraph.ErrSchemaNotFound
	}
	for i := range b.schemaVersions {
		b.schemaVersions[i].Active = i == idx
	}
	return nil
}

// GetActive implements entitygraph.SchemaManager.
func (b *Backend) GetActive(ctx context.Context) (schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.schemaVersions {
		if s.Active {
			return s, nil
		}
	}
	return schema.Schema{}, entitygraph.ErrSchemaNotFound
}

// GetVersion implements entitygraph.SchemaManager.
func (b *Backend) GetVersion(ctx context.Context, version int) (schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.schemaVersions {
		if s.Version == version {
			return s, nil
		}
	}
	return schema.Schema{}, entitygraph.ErrSchemaNotFound
}

// ListVersions implements entitygraph.SchemaManager, ascending version
// order (the order versions were appended in).
func (b *Backend) ListVersions(ctx context.Context) ([]schema.Schema, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]schema.Schema, len(b.schemaVersions))
	copy(out, b.schemaVersions)
	return out, nil
}
