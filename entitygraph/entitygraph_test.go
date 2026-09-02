package entitygraph_test

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-go-shared/entitygraph"
	"github.com/aosanya/mwanachama-go-shared/schema"
)

// testSchema builds a minimal Schema with Agency, Goal, and Workflow types
// for use in lookup and validation tests.
func testSchema() schema.Schema {
	return schema.Schema{
		ID:      "test-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []schema.TypeDefinition{
			{
				Name:        "Agency",
				DisplayName: "Agency",
				Properties: []schema.PropertyDefinition{
					{Name: "name", Type: schema.PropertyTypeString, Required: true},
				},
				Relationships: []schema.RelationshipDefinition{
					{Name: "has_goal", Label: "Goals", ToType: "Goal", ToMany: true},
					{Name: "has_workflow", Label: "Workflows", ToType: "Workflow", ToMany: true},
				},
			},
			{
				Name:        "Goal",
				DisplayName: "Goal",
				Properties: []schema.PropertyDefinition{
					{Name: "title", Type: schema.PropertyTypeString, Required: true},
				},
				Relationships: []schema.RelationshipDefinition{
					{Name: "belongs_to_agency", Label: "Agency", ToType: "Agency", ToMany: false},
				},
			},
			{
				Name:          "Workflow",
				DisplayName:   "Workflow",
				Properties:    []schema.PropertyDefinition{{Name: "name", Type: schema.PropertyTypeString, Required: true}},
				Relationships: nil,
			},
		},
	}
}

// ── FindTypeDef ──────────────────────────────────────────────────────────────

func TestFindTypeDef_ExistingType_ReturnsDef(t *testing.T) {
	s := testSchema()
	td, err := entitygraph.FindTypeDef(s, "Agency")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if td.Name != "Agency" {
		t.Errorf("got name %q, want %q", td.Name, "Agency")
	}
}

func TestFindTypeDef_UnknownType_ReturnsError(t *testing.T) {
	s := testSchema()
	if _, err := entitygraph.FindTypeDef(s, "NonExistent"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindTypeDef_EmptySchema_ReturnsError(t *testing.T) {
	if _, err := entitygraph.FindTypeDef(schema.Schema{ID: "empty"}, "Agency"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── FindRelationshipDef ──────────────────────────────────────────────────────

func TestFindRelationshipDef_ExistingLabel_ReturnsDef(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Agency")
	rd, err := entitygraph.FindRelationshipDef(td, "has_goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rd.ToType != "Goal" {
		t.Errorf("got ToType %q, want %q", rd.ToType, "Goal")
	}
	if !rd.ToMany {
		t.Error("expected ToMany = true for has_goal")
	}
}

func TestFindRelationshipDef_FunctionalLabel_ToManyFalse(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Goal")
	rd, err := entitygraph.FindRelationshipDef(td, "belongs_to_agency")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rd.ToMany {
		t.Error("expected ToMany = false for belongs_to_agency")
	}
}

func TestFindRelationshipDef_UnknownLabel_ReturnsError(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Agency")
	if _, err := entitygraph.FindRelationshipDef(td, "nonexistent_label"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindRelationshipDef_TypeWithNoRelationships_ReturnsError(t *testing.T) {
	td := schema.TypeDefinition{Name: "Bare", Relationships: nil}
	if _, err := entitygraph.FindRelationshipDef(td, "has_goal"); err == nil {
		t.Fatal("expected error for type with no relationships, got nil")
	}
}

// ── ValidateCreateRelationship ───────────────────────────────────────────────

func TestValidateCreateRelationship_ValidEdge(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Agency")
	if err := entitygraph.ValidateCreateRelationship(td, "has_goal", "Goal"); err != nil {
		t.Errorf("unexpected error for valid edge: %v", err)
	}
}

func TestValidateCreateRelationship_UnknownLabel_ReturnsErrInvalidRelationship(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Agency")
	err := entitygraph.ValidateCreateRelationship(td, "unknown_label", "Goal")
	if !errors.Is(err, entitygraph.ErrInvalidRelationship) {
		t.Errorf("got %v, want ErrInvalidRelationship", err)
	}
}

func TestValidateCreateRelationship_WrongToType_ReturnsErrInvalidRelationship(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Agency")
	err := entitygraph.ValidateCreateRelationship(td, "has_goal", "WrongType")
	if !errors.Is(err, entitygraph.ErrInvalidRelationship) {
		t.Errorf("got %v, want ErrInvalidRelationship", err)
	}
}

func TestValidateCreateRelationship_FunctionalEdge(t *testing.T) {
	s := testSchema()
	td, _ := entitygraph.FindTypeDef(s, "Goal")
	if err := entitygraph.ValidateCreateRelationship(td, "belongs_to_agency", "Agency"); err != nil {
		t.Errorf("unexpected error for valid functional edge: %v", err)
	}
}

// ── ValidateSchema ───────────────────────────────────────────────────────────

func TestValidateSchema_ValidSchema_NoError(t *testing.T) {
	if err := entitygraph.ValidateSchema(testSchema()); err != nil {
		t.Errorf("unexpected error for valid schema: %v", err)
	}
}

func TestValidateSchema_EmptySchema_NoError(t *testing.T) {
	s := schema.Schema{ID: "empty", AgencyID: "agency-1"}
	if err := entitygraph.ValidateSchema(s); err != nil {
		t.Errorf("unexpected error for empty schema: %v", err)
	}
}

func TestValidateSchema_DuplicateTypeName_ReturnsError(t *testing.T) {
	s := schema.Schema{
		ID:       "dup-names",
		AgencyID: "agency-1",
		Types: []schema.TypeDefinition{
			{Name: "Pump"},
			{Name: "Pump"},
		},
	}
	if err := entitygraph.ValidateSchema(s); err == nil {
		t.Fatal("expected error for duplicate type name, got nil")
	}
}

func TestValidateSchema_InverseToTypeNotFound_ReturnsError(t *testing.T) {
	s := schema.Schema{
		ID:       "bad-inverse-totype",
		AgencyID: "agency-1",
		Types: []schema.TypeDefinition{
			{
				Name: "Agency",
				Relationships: []schema.RelationshipDefinition{
					{Name: "has_goal", ToType: "Goal", ToMany: true, Inverse: "belongs_to_agency"},
				},
			},
			// Goal type is missing — ToType not found in schema
		},
	}
	if err := entitygraph.ValidateSchema(s); err == nil {
		t.Fatal("expected error when ToType not found in schema, got nil")
	}
}

func TestValidateSchema_InverseNotDeclaredOnToType_ReturnsError(t *testing.T) {
	s := schema.Schema{
		ID:       "missing-inverse-decl",
		AgencyID: "agency-1",
		Types: []schema.TypeDefinition{
			{
				Name: "Agency",
				Relationships: []schema.RelationshipDefinition{
					{Name: "has_goal", ToType: "Goal", ToMany: true, Inverse: "belongs_to_agency"},
				},
			},
			{
				Name:          "Goal",
				Relationships: nil, // does NOT declare belongs_to_agency
			},
		},
	}
	if err := entitygraph.ValidateSchema(s); err == nil {
		t.Fatal("expected error when inverse not declared on ToType, got nil")
	}
}

func TestValidateSchema_ValidInverse_NoError(t *testing.T) {
	s := schema.Schema{
		ID:       "valid-inverse",
		AgencyID: "agency-1",
		Types: []schema.TypeDefinition{
			{
				Name: "Agency",
				Relationships: []schema.RelationshipDefinition{
					{Name: "has_goal", ToType: "Goal", ToMany: true, Inverse: "belongs_to_agency"},
				},
			},
			{
				Name: "Goal",
				Relationships: []schema.RelationshipDefinition{
					{Name: "belongs_to_agency", ToType: "Agency", ToMany: false},
				},
			},
		},
	}
	if err := entitygraph.ValidateSchema(s); err != nil {
		t.Errorf("unexpected error for valid inverse: %v", err)
	}
}

func TestValidateSchema_UniqueKeyFieldNotInProperties_ReturnsError(t *testing.T) {
	s := schema.Schema{
		ID:       "bad-unique-key",
		AgencyID: "agency-1",
		Types: []schema.TypeDefinition{
			{
				Name:       "Agent",
				Properties: []schema.PropertyDefinition{{Name: "name", Type: schema.PropertyTypeString}},
				UniqueKey:  []string{"agent_id"}, // not declared in Properties
			},
		},
	}
	if err := entitygraph.ValidateSchema(s); err == nil {
		t.Fatal("expected error for UniqueKey field not found in Properties, got nil")
	}
}

func TestValidateSchema_UniqueKeyFieldInProperties_NoError(t *testing.T) {
	s := schema.Schema{
		ID:       "good-unique-key",
		AgencyID: "agency-1",
		Types: []schema.TypeDefinition{
			{
				Name:       "Agent",
				Properties: []schema.PropertyDefinition{{Name: "agent_id", Type: schema.PropertyTypeString}},
				UniqueKey:  []string{"agent_id"},
			},
		},
	}
	if err := entitygraph.ValidateSchema(s); err != nil {
		t.Errorf("unexpected error for valid UniqueKey: %v", err)
	}
}

// ── property converters ───────────────────────────────────────────────────────

func TestStringProp(t *testing.T) {
	props := map[string]any{"name": "kazi", "count": 3}
	if got := entitygraph.StringProp(props, "name"); got != "kazi" {
		t.Errorf("got %q, want %q", got, "kazi")
	}
	if got := entitygraph.StringProp(props, "count"); got != "" {
		t.Errorf("got %q for non-string value, want empty", got)
	}
	if got := entitygraph.StringProp(props, "missing"); got != "" {
		t.Errorf("got %q for missing key, want empty", got)
	}
}

func TestBoolProp(t *testing.T) {
	props := map[string]any{"active": true, "name": "kazi"}
	if got := entitygraph.BoolProp(props, "active"); !got {
		t.Error("expected true")
	}
	if got := entitygraph.BoolProp(props, "name"); got {
		t.Error("expected false for non-bool value")
	}
	if got := entitygraph.BoolProp(props, "missing"); got {
		t.Error("expected false for missing key")
	}
}

func TestInt64Prop(t *testing.T) {
	props := map[string]any{"a": int64(5), "b": 6, "c": float64(7), "d": "x"}
	if got := entitygraph.Int64Prop(props, "a"); got != 5 {
		t.Errorf("int64 case: got %d", got)
	}
	if got := entitygraph.Int64Prop(props, "b"); got != 6 {
		t.Errorf("int case: got %d", got)
	}
	if got := entitygraph.Int64Prop(props, "c"); got != 7 {
		t.Errorf("float64 case: got %d", got)
	}
	if got := entitygraph.Int64Prop(props, "d"); got != 0 {
		t.Errorf("non-numeric case: got %d, want 0", got)
	}
}

func TestFloat64Prop(t *testing.T) {
	props := map[string]any{"a": float64(1.5), "b": float32(2.5), "c": 3, "d": int64(4), "e": "x"}
	if got := entitygraph.Float64Prop(props, "a"); got != 1.5 {
		t.Errorf("float64 case: got %v", got)
	}
	if got := entitygraph.Float64Prop(props, "b"); got != 2.5 {
		t.Errorf("float32 case: got %v", got)
	}
	if got := entitygraph.Float64Prop(props, "c"); got != 3 {
		t.Errorf("int case: got %v", got)
	}
	if got := entitygraph.Float64Prop(props, "d"); got != 4 {
		t.Errorf("int64 case: got %v", got)
	}
	if got := entitygraph.Float64Prop(props, "e"); got != 0 {
		t.Errorf("non-numeric case: got %v, want 0", got)
	}
}
