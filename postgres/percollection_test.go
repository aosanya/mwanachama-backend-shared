package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/aosanya/mwanachama-backend-shared/entitygraphtest"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// twoTypeSchema mirrors the shape PerCollectionBackend exists for
// (mwanachama-backend-actor's Member/Group) closely enough to prove the
// thing entitygraphtest.Run can't: routing across more than one physical
// table. entitygraphtest's own testSchema() declares exactly one type, so
// running it alone would only prove the single-table case still works.
func twoTypeSchema() schema.Schema {
	return schema.Schema{
		Types: []schema.TypeDefinition{
			{
				Name:              "Widget",
				StorageCollection: "pctest_widgets",
				Properties:        []schema.PropertyDefinition{{Name: "sku", Type: schema.PropertyTypeString}},
				Relationships: []schema.RelationshipDefinition{
					{Name: "stored_in", ToType: "Bin", ToMany: false},
				},
			},
			{
				Name:              "Bin",
				StorageCollection: "pctest_bins",
				Properties:        []schema.PropertyDefinition{{Name: "label", Type: schema.PropertyTypeString}},
			},
		},
	}
}

// newPerCollectionBackend opens POSTGRES_URL, creates a scratch
// PerCollectionBackend for twoTypeSchema, seeds+activates its schema, and
// returns it ready to use. Skips the calling test if POSTGRES_URL is
// unset. Tables are dropped on cleanup.
func newPerCollectionBackend(t *testing.T) *postgres.PerCollectionBackend {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping PerCollectionBackend test")
	}

	ctx := context.Background()
	db, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	sch := twoTypeSchema()
	tables := postgres.DefaultPerCollectionTableNames("pctest_")
	if err := applyDDL(ctx, db, postgres.PerCollectionDDL(sch, tables)); err != nil {
		t.Fatalf("applying PerCollectionDDL: %v", err)
	}
	t.Cleanup(func() {
		_ = applyDDL(context.Background(), db, postgres.PerCollectionDropDDL(sch, tables))
	})

	b, err := postgres.NewPerCollectionBackend(db, tables, sch)
	if err != nil {
		t.Fatalf("NewPerCollectionBackend: %v", err)
	}
	if err := b.SetSchema(ctx, sch); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}
	if err := b.Publish(ctx); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := b.Activate(ctx, 1); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	return b
}

// TestPerCollectionBackend_RoutesByType is the thing this backend exists to
// prove: two types, two physical tables, and every entity method resolves
// the right one — including GetEntity/UpdateEntity/DeleteEntity, which take
// no type argument and so must go through the id registry (resolveTable)
// to find it.
func TestPerCollectionBackend_RoutesByType(t *testing.T) {
	b := newPerCollectionBackend(t)
	ctx := context.Background()

	widget, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		TypeID: "Widget", Properties: map[string]any{"sku": "w1"},
	})
	if err != nil {
		t.Fatalf("CreateEntity(Widget): %v", err)
	}
	bin, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		TypeID: "Bin", Properties: map[string]any{"label": "b1"},
	})
	if err != nil {
		t.Fatalf("CreateEntity(Bin): %v", err)
	}
	if widget.ID == bin.ID {
		t.Fatalf("Widget and Bin minted the same id %q", widget.ID)
	}

	// GetEntity resolves both, despite carrying no type hint.
	gotWidget, err := b.GetEntity(ctx, widget.ID)
	if err != nil || gotWidget.TypeID != "Widget" {
		t.Fatalf("GetEntity(widget): %+v, %v", gotWidget, err)
	}
	gotBin, err := b.GetEntity(ctx, bin.ID)
	if err != nil || gotBin.TypeID != "Bin" {
		t.Fatalf("GetEntity(bin): %+v, %v", gotBin, err)
	}

	// UpdateEntity likewise.
	updated, err := b.UpdateEntity(ctx, bin.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"label": "b1-renamed"},
	})
	if err != nil || updated.Properties["label"] != "b1-renamed" {
		t.Fatalf("UpdateEntity(bin): %+v, %v", updated, err)
	}

	// A relationship between the two types succeeds — proves the FK on
	// relationships.from_id/to_id targets the shared id registry, not a
	// single content table (there isn't one to target here).
	rel, err := b.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		Name: "stored_in", FromID: widget.ID, ToID: bin.ID,
	})
	if err != nil {
		t.Fatalf("CreateRelationship(widget->bin): %v", err)
	}
	if rel.FromID != widget.ID || rel.ToID != bin.ID {
		t.Fatalf("CreateRelationship: got %+v", rel)
	}

	// A relationship naming an id that was never created reports
	// ErrEntityNotFound — the same behavior Backend gets from a real FK
	// violation, reproduced here against the registry instead.
	if _, err := b.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		Name: "stored_in", FromID: widget.ID, ToID: "does-not-exist",
	}); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("CreateRelationship(unknown ToID): got %v, want ErrEntityNotFound", err)
	}

	// DeleteEntity resolves and soft-deletes the right table's row.
	if err := b.DeleteEntity(ctx, widget.ID); err != nil {
		t.Fatalf("DeleteEntity(widget): %v", err)
	}
	if _, err := b.GetEntity(ctx, widget.ID); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("GetEntity(deleted widget): got %v, want ErrEntityNotFound", err)
	}

	// An entirely unknown id is ErrEntityNotFound from Get/Update/Delete
	// alike, not a table-not-found error or a panic.
	if _, err := b.GetEntity(ctx, "never-existed"); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("GetEntity(unknown): got %v, want ErrEntityNotFound", err)
	}
	if _, err := b.UpdateEntity(ctx, "never-existed", entitygraph.UpdateEntityRequest{}); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("UpdateEntity(unknown): got %v, want ErrEntityNotFound", err)
	}
	if err := b.DeleteEntity(ctx, "never-existed"); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("DeleteEntity(unknown): got %v, want ErrEntityNotFound", err)
	}

	// ListEntities: TypeID set stays single-table; unset spans both and
	// comes back merged in one order.
	widgets, err := b.ListEntities(ctx, entitygraph.EntityFilter{TypeID: "Widget"})
	if err != nil || len(widgets) != 1 {
		t.Fatalf("ListEntities(TypeID=Widget): %+v, %v", widgets, err)
	}
	all, err := b.ListEntities(ctx, entitygraph.EntityFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("ListEntities(no TypeID): %+v, %v", all, err)
	}
}

// TestPerCollectionBackend_Conformance runs the same entitygraphtest suite
// Backend's own TestBackend_Conformance runs, against a PerCollectionBackend
// built from a schema with (deliberately) only one type — matching what the
// suite itself declares. This is a weaker check than
// TestPerCollectionBackend_RoutesByType above (it never exercises more than
// one physical table) but still worth running: it confirms the override
// methods don't drift from Backend's documented behavior for the ordinary,
// single-type case.
func TestPerCollectionBackend_Conformance(t *testing.T) {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping PerCollectionBackend conformance test")
	}

	ctx := context.Background()
	db, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	sch := schema.Schema{
		Types: []schema.TypeDefinition{{
			Name:              "Widget",
			StorageCollection: "pcconform_widgets",
			Properties:        []schema.PropertyDefinition{{Name: "sku", Type: schema.PropertyTypeString}},
			UniqueKey:         []string{"sku"},
			Relationships:     []schema.RelationshipDefinition{{Name: "contains", ToType: "Widget", ToMany: true}},
		}},
	}
	tables := postgres.DefaultPerCollectionTableNames("pcconform_")
	if err := applyDDL(ctx, db, postgres.PerCollectionDDL(sch, tables)); err != nil {
		t.Fatalf("applying PerCollectionDDL: %v", err)
	}
	t.Cleanup(func() {
		_ = applyDDL(context.Background(), db, postgres.PerCollectionDropDDL(sch, tables))
	})

	b, err := postgres.NewPerCollectionBackend(db, tables, sch)
	if err != nil {
		t.Fatalf("NewPerCollectionBackend: %v", err)
	}
	entitygraphtest.Run(t, b, b)
}
