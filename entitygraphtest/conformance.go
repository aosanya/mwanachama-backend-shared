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

// Run exercises dm/sm end to end: schema draft → publish → activate, entity
// CRUD, upsert-by-unique-key, relationships, and graph traversal. agencyID
// scopes every call so the same suite can run more than once against a
// shared backend (e.g. once per subtest) without cross-contamination.
func Run(t *testing.T, dm entitygraph.DataManager, sm entitygraph.SchemaManager, agencyID string) {
	t.Helper()

	t.Run("schema lifecycle", func(t *testing.T) { testSchemaLifecycle(t, sm, agencyID) })
	t.Run("entity CRUD", func(t *testing.T) { testEntityCRUD(t, dm, agencyID) })
	t.Run("upsert by unique key", func(t *testing.T) { testUpsert(t, dm, sm, agencyID) })
	t.Run("relationships", func(t *testing.T) { testRelationships(t, dm, agencyID) })
	t.Run("traverse graph", func(t *testing.T) { testTraverse(t, dm, agencyID) })
}

func testSchema() schema.Schema {
	return schema.Schema{
		AgencyID: "placeholder", // callers overwrite AgencyID before SetSchema
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

func testSchemaLifecycle(t *testing.T, sm entitygraph.SchemaManager, agencyID string) {
	ctx := context.Background()

	if _, err := sm.GetSchema(ctx, agencyID); !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		t.Fatalf("GetSchema before SetSchema: got %v, want ErrSchemaNotFound", err)
	}

	draft := testSchema()
	draft.AgencyID = agencyID
	if err := sm.SetSchema(ctx, draft); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}
	got, err := sm.GetSchema(ctx, agencyID)
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if len(got.Types) != 1 || got.Types[0].Name != "Widget" {
		t.Fatalf("GetSchema: got %+v", got)
	}

	if _, err := sm.GetActive(ctx, agencyID); !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		t.Fatalf("GetActive before Publish: got %v, want ErrSchemaNotFound", err)
	}

	if err := sm.Publish(ctx, agencyID); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	versions, err := sm.ListVersions(ctx, agencyID)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("ListVersions after first publish: got %+v", versions)
	}

	if err := sm.Activate(ctx, agencyID, 1); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	active, err := sm.GetActive(ctx, agencyID)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active.Version != 1 || !active.Active {
		t.Fatalf("GetActive: got %+v", active)
	}

	if err := sm.Activate(ctx, agencyID, 99); !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		t.Fatalf("Activate unknown version: got %v, want ErrSchemaNotFound", err)
	}

	// Publish a second version and re-activate — exactly one version
	// should end up active.
	if err := sm.SetSchema(ctx, draft); err != nil {
		t.Fatalf("SetSchema #2: %v", err)
	}
	if err := sm.Publish(ctx, agencyID); err != nil {
		t.Fatalf("Publish #2: %v", err)
	}
	if err := sm.Activate(ctx, agencyID, 2); err != nil {
		t.Fatalf("Activate #2: %v", err)
	}
	versions, err = sm.ListVersions(ctx, agencyID)
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

func testEntityCRUD(t *testing.T, dm entitygraph.DataManager, agencyID string) {
	ctx := context.Background()

	e, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "abc"},
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if e.ID == "" || e.Deleted {
		t.Fatalf("CreateEntity: got %+v", e)
	}

	got, err := dm.GetEntity(ctx, agencyID, e.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if entitygraph.StringProp(got.Properties, "sku") != "abc" {
		t.Fatalf("GetEntity: got properties %+v", got.Properties)
	}

	updated, err := dm.UpdateEntity(ctx, agencyID, e.ID, entitygraph.UpdateEntityRequest{
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

	list, err := dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: agencyID, TypeID: "Widget"})
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

	if err := dm.DeleteEntity(ctx, agencyID, e.ID); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	if _, err := dm.GetEntity(ctx, agencyID, e.ID); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("GetEntity after delete: got %v, want ErrEntityNotFound", err)
	}
	if err := dm.DeleteEntity(ctx, agencyID, e.ID); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("DeleteEntity twice: got %v, want ErrEntityNotFound", err)
	}
}

func testUpsert(t *testing.T, dm entitygraph.DataManager, sm entitygraph.SchemaManager, agencyID string) {
	ctx := context.Background()

	first, err := dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "upsert-1", "count": float64(1)},
	})
	if err != nil {
		t.Fatalf("UpsertEntity (insert): %v", err)
	}

	second, err := dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "upsert-1", "count": float64(2)},
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
	draft, err := sm.GetSchema(ctx, agencyID)
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	draft.Types = append(draft.Types, schema.TypeDefinition{Name: "Unkeyed"})
	if err := sm.SetSchema(ctx, draft); err != nil {
		t.Fatalf("SetSchema (adding Unkeyed): %v", err)
	}
	if err := sm.Publish(ctx, agencyID); err != nil {
		t.Fatalf("Publish (adding Unkeyed): %v", err)
	}
	versions, err := sm.ListVersions(ctx, agencyID)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if err := sm.Activate(ctx, agencyID, versions[len(versions)-1].Version); err != nil {
		t.Fatalf("Activate (adding Unkeyed): %v", err)
	}
	if _, err := dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Unkeyed"}); !errors.Is(err, entitygraph.ErrUniqueKeyNotDefined) {
		t.Fatalf("UpsertEntity on unkeyed type: got %v, want ErrUniqueKeyNotDefined", err)
	}
}

func testRelationships(t *testing.T, dm entitygraph.DataManager, agencyID string) {
	ctx := context.Background()

	parent, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "rel-parent"}})
	if err != nil {
		t.Fatalf("CreateEntity(parent): %v", err)
	}
	child, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "rel-child"}})
	if err != nil {
		t.Fatalf("CreateEntity(child): %v", err)
	}

	if _, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID, Name: "contains", FromID: parent.ID, ToID: "does-not-exist",
	}); !errors.Is(err, entitygraph.ErrEntityNotFound) {
		t.Fatalf("CreateRelationship to unknown entity: got %v, want ErrEntityNotFound", err)
	}

	rel, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID, Name: "contains", FromID: parent.ID, ToID: child.ID,
	})
	if err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}

	got, err := dm.GetRelationship(ctx, agencyID, rel.ID)
	if err != nil {
		t.Fatalf("GetRelationship: %v", err)
	}
	if got.FromID != parent.ID || got.ToID != child.ID {
		t.Fatalf("GetRelationship: got %+v", got)
	}

	list, err := dm.ListRelationships(ctx, entitygraph.RelationshipFilter{AgencyID: agencyID, FromID: parent.ID})
	if err != nil {
		t.Fatalf("ListRelationships: %v", err)
	}
	if len(list) != 1 || list[0].ID != rel.ID {
		t.Fatalf("ListRelationships: got %+v", list)
	}

	if err := dm.DeleteRelationship(ctx, agencyID, rel.ID); err != nil {
		t.Fatalf("DeleteRelationship: %v", err)
	}
	if _, err := dm.GetRelationship(ctx, agencyID, rel.ID); !errors.Is(err, entitygraph.ErrRelationshipNotFound) {
		t.Fatalf("GetRelationship after delete: got %v, want ErrRelationshipNotFound", err)
	}
}

func testTraverse(t *testing.T, dm entitygraph.DataManager, agencyID string) {
	ctx := context.Background()

	root, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "traverse-root"}})
	if err != nil {
		t.Fatalf("CreateEntity(root): %v", err)
	}
	mid, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "traverse-mid"}})
	if err != nil {
		t.Fatalf("CreateEntity(mid): %v", err)
	}
	leaf, err := dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Widget", Properties: map[string]any{"sku": "traverse-leaf"}})
	if err != nil {
		t.Fatalf("CreateEntity(leaf): %v", err)
	}
	if _, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{AgencyID: agencyID, Name: "contains", FromID: root.ID, ToID: mid.ID}); err != nil {
		t.Fatalf("CreateRelationship(root->mid): %v", err)
	}
	if _, err := dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{AgencyID: agencyID, Name: "contains", FromID: mid.ID, ToID: leaf.ID}); err != nil {
		t.Fatalf("CreateRelationship(mid->leaf): %v", err)
	}

	// Depth 1 outbound from root reaches only mid (plus root itself).
	shallow, err := dm.TraverseGraph(ctx, entitygraph.TraverseGraphRequest{AgencyID: agencyID, StartID: root.ID, Direction: "outbound", Depth: 1})
	if err != nil {
		t.Fatalf("TraverseGraph depth 1: %v", err)
	}
	if !hasVertex(shallow.Vertices, mid.ID) || hasVertex(shallow.Vertices, leaf.ID) {
		t.Fatalf("TraverseGraph depth 1: got vertices %+v", shallow.Vertices)
	}

	// Depth 2 outbound from root reaches mid and leaf.
	deep, err := dm.TraverseGraph(ctx, entitygraph.TraverseGraphRequest{AgencyID: agencyID, StartID: root.ID, Direction: "outbound", Depth: 2})
	if err != nil {
		t.Fatalf("TraverseGraph depth 2: %v", err)
	}
	if !hasVertex(deep.Vertices, mid.ID) || !hasVertex(deep.Vertices, leaf.ID) {
		t.Fatalf("TraverseGraph depth 2: got vertices %+v", deep.Vertices)
	}
	if len(deep.Edges) != 2 {
		t.Fatalf("TraverseGraph depth 2: got %d edges, want 2", len(deep.Edges))
	}

	// Inbound from leaf reaches mid, not root at depth 1.
	inbound, err := dm.TraverseGraph(ctx, entitygraph.TraverseGraphRequest{AgencyID: agencyID, StartID: leaf.ID, Direction: "inbound", Depth: 1})
	if err != nil {
		t.Fatalf("TraverseGraph inbound: %v", err)
	}
	if !hasVertex(inbound.Vertices, mid.ID) || hasVertex(inbound.Vertices, root.ID) {
		t.Fatalf("TraverseGraph inbound depth 1: got vertices %+v", inbound.Vertices)
	}
}

func hasVertex(vs []entitygraph.Entity, id string) bool {
	for _, v := range vs {
		if v.ID == id {
			return true
		}
	}
	return false
}
