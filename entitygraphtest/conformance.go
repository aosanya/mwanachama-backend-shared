// Package entitygraphtest is a conformance suite run against any
// entitygraph.DataManager + entitygraph.SchemaManager pair — the memory and
// Postgres backends both call Run so they're held to the same behavior
// instead of drifting apart under separate hand-written test suites.
package entitygraphtest

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// relationshipDataManager is entitygraph.DataManager plus the relationship
// methods that no longer live on that interface (see its doc) — Backend,
// PerCollectionBackend, and the memory backend all still implement these as
// plain exported methods, so this suite can keep exercising them without the
// shared interface needing to declare them.
type relationshipDataManager interface {
	entitygraph.DataManager
	CreateRelationship(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error)
	DeleteRelationship(ctx context.Context, relationshipID string) error
	ListRelationships(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error)
}

// Run exercises dm/sm end to end: schema draft → publish → activate, entity
// CRUD, upsert-by-unique-key, and relationships.
func Run(t *testing.T, dm relationshipDataManager, sm entitygraph.SchemaManager) {
	t.Helper()

	t.Run("schema lifecycle", func(t *testing.T) { testSchemaLifecycle(t, sm) })
	t.Run("entity CRUD", func(t *testing.T) { testEntityCRUD(t, dm) })
	t.Run("upsert by unique key", func(t *testing.T) { testUpsert(t, dm, sm) })
	t.Run("relationships", func(t *testing.T) { testRelationships(t, dm) })
}

func testSchema() schema.Schema {
	return schema.Schema{
		Types: []schema.TypeDefinition{
			{
				Name:       "Widget",
				Properties: []schema.PropertyDefinition{{Name: "sku", Type: schema.PropertyTypeString}},
				UniqueKey:  []string{"sku"},
				Relationships: []schema.RelationshipDefinition{
					{Name: "contains", ToType: "Widget", ToMany: true},
				},
			},
		},
	}
}

func testSchemaLifecycle(t *testing.T, sm entitygraph.SchemaManager) {
	ctx := context.Background()

	if _, err := sm.GetSchema(ctx); !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		t.Fatalf("GetSchema before SetSchema: got %v, want ErrSchemaNotFound", err)
	}

	draft := testSchema()
	if err := sm.SetSchema(ctx, draft); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}
	got, err := sm.GetSchema(ctx)
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if len(got.Types) != 1 || got.Types[0].Name != "Widget" {
		t.Fatalf("GetSchema: got %+v", got)
	}

	if _, err := sm.GetActive(ctx); !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		t.Fatalf("GetActive before Publish: got %v, want ErrSchemaNotFound", err)
	}

	if err := sm.Publish(ctx); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	versions, err := sm.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("ListVersions after first publish: got %+v", versions)
	}

	if err := sm.Activate(ctx, 1); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	active, err := sm.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active.Version != 1 || !active.Active {
		t.Fatalf("GetActive: got %+v", active)
	}

	if err := sm.Activate(ctx, 99); !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		t.Fatalf("Activate unknown version: got %v, want ErrSchemaNotFound", err)
	}

	// Publish a second version and re-activate — exactly one version
	// should end up active.
	if err := sm.SetSchema(ctx, draft); err != nil {
		t.Fatalf("SetSchema #2: %v", err)
	}
	if err := sm.Publish(ctx); err != nil {
		t.Fatalf("Publish #2: %v", err)
	}
	if err := sm.Activate(ctx, 2); err != nil {
		t.Fatalf("Activate #2: %v", err)
	}
	versions, err = sm.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions #2: %v", err)
	}
	activeCount := 0
	for _, v := range versions {
		if v.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active version, got %d across %+v", activeCount, versions)
	}
}

func testEntityCRUD(t *testing.T, dm entitygraph.DataManager) {
	ctx := context.Background()

	e, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		TypeID: "Widget", Properties: map[string]any{"sku": "abc"},
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if e.ID == "" || e.Deleted {
		t.Fatalf("CreateEntity: got %+v", e)
	}

	got, err := dm.GetEntity(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if entitygraph.StringProp(got.Properties, "sku") != "abc" {
		t.Fatalf("GetEntity: got properties %+v", got.Properties)
	}

	updated, err := dm.UpdateEntity(ctx, e.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"color": "red"},
	})
	if err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}
	if entitygraph.StringProp(updated.Properties, "sku") != "abc" {
		t.Fatalf("UpdateEntity dropped an untouched key: got %+v", updated.Properties)
	}
	if entitygraph.StringProp(updated.Properties, "color") != "red" {
		t.Fatalf("UpdateEntity: got %+v", updated.Properties)
	}

	list, err := dm.ListEntities(ctx, entitygraph.EntityFilter{TypeID: "Widget"})
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	found := false
	for _, le := range list {
		if le.ID == e.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListEntities: %q missing from %+v", e.ID, list)
	}

	if err := dm.DeleteEntity(ctx, e.ID); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	if _, err := dm.GetEntity(ctx, e.ID); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("GetEntity after delete: got %v, want ErrEntityNotFound", err)
	}
	if err := dm.DeleteEntity(ctx, e.ID); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("DeleteEntity twice: got %v, want ErrEntityNotFound", err)
	}
}

func testUpsert(t *testing.T, dm entitygraph.DataManager, sm entitygraph.SchemaManager) {
	ctx := context.Background()

	first, err := dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
		TypeID: "Widget", Properties: map[string]any{"sku": "upsert-1", "count": float64(1)},
	})
	if err != nil {
		t.Fatalf("UpsertEntity (insert): %v", err)
	}

	second, err := dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
		TypeID: "Widget", Properties: map[string]any{"sku": "upsert-1", "count": float64(2)},
	})
	if err != nil {
		t.Fatalf("UpsertEntity (merge): %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("UpsertEntity should have merged into %q, created %q instead", first.ID, second.ID)
	}
	if entitygraph.Float64Prop(second.Properties, "count") != 2 {
		t.Fatalf("UpsertEntity merge: got properties %+v", second.Properties)
	}

	// A type with no UniqueKey declared must reject Upsert.
	draft, err := sm.GetSchema(ctx)
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	draft.Types = append(draft.Types, schema.TypeDefinition{Name: "Unkeyed"})
	if err := sm.SetSchema(ctx, draft); err != nil {
		t.Fatalf("SetSchema (adding Unkeyed): %v", err)
	}
	if err := sm.Publish(ctx); err != nil {
		t.Fatalf("Publish (adding Unkeyed): %v", err)
	}
	versions, err := sm.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if err := sm.Activate(ctx, versions[len(versions)-1].Version); err != nil {
		t.Fatalf("Activate (adding Unkeyed): %v", err)
	}
	if _, err := dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{TypeID: "Unkeyed"}); !errors.Is(err, entitygraph.ErrUniqueKeyNotDefined) {
		t.Fatalf("UpsertEntity on unkeyed type: got %v, want ErrUniqueKeyNotDefined", err)
	}
}

func testRelationships(t *testing.T, dm relationshipDataManager) {
	ctx := context.Background()

	parent, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{TypeID: "Widget", Properties: map[string]any{"sku": "rel-parent"}})
	if err != nil {
		t.Fatalf("CreateEntity(parent): %v", err)
	}
	child, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{TypeID: "Widget", Properties: map[string]any{"sku": "rel-child"}})
	if err != nil {
		t.Fatalf("CreateEntity(child): %v", err)
	}

	if _, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		Name: "contains", FromID: parent.ID, ToID: "does-not-exist",
	}); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("CreateRelationship to unknown entity: got %v, want ErrEntityNotFound", err)
	}

	rel, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		Name: "contains", FromID: parent.ID, ToID: child.ID,
	})
	if err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}
	if rel.ID == "" || rel.FromID != parent.ID || rel.ToID != child.ID {
		t.Fatalf("CreateRelationship: got %+v", rel)
	}

	list, err := dm.ListRelationships(ctx, entitygraph.RelationshipFilter{FromID: parent.ID})
	if err != nil {
		t.Fatalf("ListRelationships: %v", err)
	}
	if len(list) != 1 || list[0].ID != rel.ID {
		t.Fatalf("ListRelationships: got %+v", list)
	}

	if err := dm.DeleteRelationship(ctx, rel.ID); err != nil {
		t.Fatalf("DeleteRelationship: %v", err)
	}
	if err := dm.DeleteRelationship(ctx, rel.ID); !errors.Is(err, entitygraph.ErrRelationshipNotFound) {
		t.Fatalf("DeleteRelationship twice: got %v, want ErrRelationshipNotFound", err)
	}
	list, err = dm.ListRelationships(ctx, entitygraph.RelationshipFilter{FromID: parent.ID})
	if err != nil {
		t.Fatalf("ListRelationships after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListRelationships after delete: got %+v, want empty", list)
	}
}
