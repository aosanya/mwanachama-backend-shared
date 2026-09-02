// Package schema defines the versioned type-system used to describe an
// entity-graph: what entity types exist, what properties and relationships
// each one carries, and how those relationships connect back to other types
// in the same [Schema]. Pure data structures — no logic, no storage
// dependency.
//
// Ported from github.com/aosanya/CodeValdSharedLib's "types" package, scoped
// down to just the schema-definition types entitygraph needs (the
// route-generation / service-registration types that package also carried
// are dropped here — mwanachama-api-gateway registers its own HTTP routes by
// hand, it does not derive them from a schema).
package schema

import "time"

// PropertyType is the data type of a single property in a [TypeDefinition].
//
// Primitive types map directly to Go/JSON primitives. Choice types declare
// the field shape: [PropertyTypeOption] carries its closed value set inline
// on [PropertyDefinition.Options]; [PropertyTypeSelect] and
// [PropertyTypeMultiSelect] declare only the field shape, with the runtime
// value list owned by the caller. Complex types carry additional
// configuration on [PropertyDefinition].
type PropertyType string

const (
	// Primitive types.

	// PropertyTypeString is a UTF-8 text value.
	PropertyTypeString PropertyType = "string"

	// PropertyTypeInteger is a 64-bit signed integer.
	PropertyTypeInteger PropertyType = "integer"

	// PropertyTypeFloat is a 64-bit floating-point number.
	PropertyTypeFloat PropertyType = "float"

	// PropertyTypeNumber is a generic JSON number — accepts either integer or
	// floating-point values. Use this when the schema does not need to commit
	// to int vs float at definition time (e.g. "hours", "rating", "score").
	PropertyTypeNumber PropertyType = "number"

	// PropertyTypeDate is an ISO 8601 date (e.g. "2026-01-15").
	PropertyTypeDate PropertyType = "date"

	// PropertyTypeDatetime is an ISO 8601 date + time (e.g. "2026-01-15T10:30:00Z").
	PropertyTypeDatetime PropertyType = "datetime"

	// PropertyTypeBoolean is a true/false flag.
	PropertyTypeBoolean PropertyType = "boolean"

	// PropertyTypeUUID is an immutable RFC 4122 UUID string. System-assigned
	// at entity creation time; never set by user input.
	PropertyTypeUUID PropertyType = "uuid"

	// Choice types.

	// PropertyTypeOption is a single fixed value from a closed, schema-defined
	// set declared inline via [PropertyDefinition.Options]; DataManager
	// implementations validate writes against that list.
	PropertyTypeOption PropertyType = "option"

	// PropertyTypeSelect is a single value chosen from a list curated by the
	// caller at runtime (not baked into the schema).
	PropertyTypeSelect PropertyType = "select"

	// PropertyTypeMultiSelect is one or more values chosen from a
	// runtime-curated list.
	PropertyTypeMultiSelect PropertyType = "multiselect"

	// Complex types — require additional configuration on [PropertyDefinition].

	// PropertyTypeRating is a numeric rating with a configurable range and
	// labels. A non-nil [RatingConfig] must be supplied on the
	// [PropertyDefinition].
	PropertyTypeRating PropertyType = "rating"

	// PropertyTypeArray is an ordered, homogeneous list of values. The
	// element type is declared via [PropertyDefinition.ElementType]; when
	// ElementType is the zero value, the array is treated as untyped (any
	// JSON value).
	PropertyTypeArray PropertyType = "array"
)

// RatingConfig holds the configuration for a property of type
// [PropertyTypeRating].
type RatingConfig struct {
	// Min is the lowest allowed rating value (e.g. 1).
	Min int

	// Max is the highest allowed rating value (e.g. 5).
	Max int

	// Labels are optional human-readable names for each value from Min to
	// Max. If provided, len(Labels) must equal Max - Min + 1.
	Labels []string
}

// PropertyDefinition describes a single named property within a
// [TypeDefinition].
type PropertyDefinition struct {
	// Name is the property identifier (e.g. "status", "priority").
	Name string

	// Type is the data type for this property.
	Type PropertyType

	// Required indicates that every instance of this type must supply this
	// property.
	Required bool

	// RatingConfig holds the configuration for rating properties. Must be
	// non-nil when Type is [PropertyTypeRating]; ignored for all other
	// types.
	RatingConfig *RatingConfig

	// Options is the closed set of allowed string values for properties of
	// type [PropertyTypeOption]. An empty slice on an option-typed property
	// means "no values allowed"; callers must declare the full set at
	// schema-definition time.
	Options []string

	// ElementType declares the data type of each element in an array
	// property. Must be set when Type is [PropertyTypeArray]; the zero
	// value is treated as "untyped" (any JSON value permitted). Nested
	// arrays are not supported — ElementType itself may not be
	// [PropertyTypeArray].
	ElementType PropertyType
}

// RelationshipDefinition declares a legal directed edge from the owning
// [TypeDefinition] to another type within the same [Schema].
//
// Validation rules applied by DataManager.CreateRelationship:
//   - The edge label must match a RelationshipDefinition.Name on the source
//     entity's TypeDefinition.
//   - The target entity's TypeID must equal ToType.
//   - If ToMany is false, a second edge with the same label from the same
//     source returns entitygraph.ErrRelationshipCardinalityViolation.
type RelationshipDefinition struct {
	// Name is the edge label stored on the relationship row (e.g.
	// "has_todo", "assigned_to"). Must be unique within the owning
	// TypeDefinition.
	Name string

	// Label is the human-readable display name (e.g. "Assignees").
	Label string

	// ToType is the TypeDefinition.Name of the target entity class. Must
	// reference a type declared in the same Schema.
	ToType string

	// ToMany controls cardinality.
	//   false → at most one target entity (functional)
	//   true  → zero or more targets (collection; unbounded)
	ToMany bool

	// Required indicates that at least one edge of this label must exist on
	// every entity of the owning type.
	Required bool

	// Inverse is the optional name of the reciprocal relationship label on
	// the ToType. If set, DataManager implementations may use this to
	// auto-create the inverse edge; behaviour is implementation-defined and
	// not enforced by the schema layer.
	Inverse string

	// Properties is the ordered list of property definitions carried on
	// relationship rows of this type. When non-empty, DataManager
	// implementations store these fields alongside from/to in the
	// relationships table. The inverse edge (if Inverse is set) receives
	// the same Properties so callers traversing in either direction see
	// identical metadata.
	Properties []PropertyDefinition
}

// TypeDefinition declares a named class of entity within a [Schema].
type TypeDefinition struct {
	// Name is the unique type identifier within the Schema (e.g. "Task").
	Name string

	// DisplayName is a human-readable label for this type (e.g. "Task").
	DisplayName string

	// Properties is the ordered list of property definitions for this type.
	Properties []PropertyDefinition

	// Relationships is the ordered list of relationship definitions for
	// this type. Each entry declares a legal directed edge this type may
	// form to another type in the same Schema. An empty slice means this
	// type has no declared outbound relationships; the DataManager will
	// reject any CreateRelationship call whose label is not listed here.
	Relationships []RelationshipDefinition

	// StorageCollection is a carry-over from the ArangoDB-backed original
	// (github.com/aosanya/CodeValdSharedLib/entitygraph), where it named the
	// backing collection for instances of this type. The Postgres
	// DataManager in this repo stores every entity in one `entities` table
	// keyed by TypeID — it does not read this field to route storage.
	// Ported callers (mwanachama-git, mwanachama-taskmanager) may still set
	// it, purely as a label carried over from CodeValdGit/CodeValdWork's
	// schema.go, with no functional effect here.
	StorageCollection string

	// Immutable indicates that instances of this type cannot be updated
	// after creation. UpdateEntity returns entitygraph.ErrImmutableType
	// when called on an entity whose TypeDefinition has Immutable set to
	// true. Only CreateEntity and DeleteEntity are valid for immutable
	// types.
	Immutable bool

	// UniqueKey is the ordered list of property names that together form a
	// composite natural key for this type (e.g. ["sha"] or ["agent_id"]).
	// When set, DataManager.UpsertEntity uses these property values to
	// locate an existing non-deleted entity and merge the supplied
	// properties onto it, instead of inserting a duplicate. All names must
	// reference a PropertyDefinition.Name declared in Properties. An empty
	// or nil slice means no unique key is defined — UpsertEntity returns
	// entitygraph.ErrUniqueKeyNotDefined for this type.
	UniqueKey []string
}

// Schema is a versioned, immutable collection of [TypeDefinition]s for one
// agency. Updating the schema produces a new version; previous versions are
// preserved.
type Schema struct {
	// ID is the unique identifier for this schema version (UUID).
	ID string

	// AgencyID is the agency this schema belongs to.
	AgencyID string

	// Version is the auto-incrementing version number (1, 2, 3, …). The
	// first publish produces Version 1; each subsequent call increments by
	// one. Draft documents always carry Version 0.
	Version int

	// Active is true for the single published schema version that is
	// currently in use for write operations (CreateEntity,
	// CreateRelationship). Only one published version per agency can be
	// active at a time. Draft documents always have Active = false.
	Active bool

	// Tag is the human-readable version label (e.g. "v1", "v2").
	Tag string

	// Types is the ordered list of type definitions in this schema version.
	Types []TypeDefinition

	// CreatedAt is the time this schema version was created.
	CreatedAt time.Time
}
