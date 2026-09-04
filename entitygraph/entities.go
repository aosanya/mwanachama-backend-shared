package entitygraph

import "time"

// Entity is an instance of a typed real-world object managed by a
// DataManager. TypeID matches TypeDefinition.Name in the current schema.
// Properties hold the current state values; no schema validation is
// performed. Deleted and DeletedAt are set by DeleteEntity (soft delete) —
// the entity is never hard-deleted.
type Entity struct {
	// ID is the unique identifier for this entity (UUID).
	ID string `json:"id"`

	// TypeID matches TypeDefinition.Name in the current schema
	// (e.g. "Task", "Repository").
	TypeID string `json:"typeId"`

	// Properties holds the current state values keyed by property name. No
	// schema validation is applied.
	Properties map[string]any `json:"properties,omitempty"`

	// CreatedAt is the time this entity was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the time this entity was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// Deleted is true once DeleteEntity has been called.
	Deleted bool `json:"deleted,omitempty"`

	// DeletedAt is set when DeleteEntity is called; nil until then.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// CreateEntityRequest is the input for creating a new entity.
type CreateEntityRequest struct {
	// TypeID must match a TypeDefinition.Name in the current schema.
	TypeID string

	// Properties are the initial state values for the entity.
	Properties map[string]any

	// Relationships are optional inline edges to create atomically with
	// the entity. Each entry is validated against the TypeDefinition
	// before any writes are made. If any validation fails the entire
	// operation is aborted. Required relationships
	// (RelationshipDefinition.Required == true) must be supplied here;
	// omitting them causes ErrRequiredRelationshipViolation.
	Relationships []EntityRelationshipRequest
}

// EntityRelationshipRequest carries a single relationship to create
// alongside a new entity in [CreateEntityRequest].
type EntityRelationshipRequest struct {
	// Name is the edge label — must match a RelationshipDefinition.Name
	// declared on the entity's TypeDefinition.
	Name string

	// ToID is the target entity ID.
	ToID string
}

// UpdateEntityRequest is the input for patching an entity's properties.
// Only the keys present in Properties are updated; absent keys are left
// unchanged.
type UpdateEntityRequest struct {
	// Properties are the property values to patch onto the entity.
	Properties map[string]any
}

// EntityFilter scopes a ListEntities query. Zero-value fields are ignored
// (no filtering applied for that field).
type EntityFilter struct {
	// TypeID restricts results to entities of this type. If empty, all
	// entity types are included.
	TypeID string

	// Properties restricts results to entities whose stored properties
	// contain all of the specified key-value pairs (exact string match
	// per key). If nil or empty, no property-level filtering is applied.
	Properties map[string]any
}

// Relationship is a directed graph edge between two entities.
type Relationship struct {
	// ID is the unique identifier for this relationship (UUID).
	ID string `json:"id"`

	// Name is the semantic label for this edge (e.g. "assigned_to",
	// "has_branch").
	Name string `json:"name"`

	// FromID is the source entity ID.
	FromID string `json:"fromId"`

	// ToID is the target entity ID.
	ToID string `json:"toId"`

	// Properties are optional metadata carried on the edge.
	Properties map[string]any `json:"properties,omitempty"`

	// CreatedAt is the time this relationship was created.
	CreatedAt time.Time `json:"createdAt"`
}

// CreateRelationshipRequest is the input for creating a directed graph edge
// between two entities.
type CreateRelationshipRequest struct {
	// Name is the semantic label for the edge.
	Name string

	// FromID is the source entity ID.
	FromID string

	// ToID is the target entity ID.
	ToID string

	// Properties are optional metadata to store on the edge.
	Properties map[string]any
}

// RelationshipFilter scopes a ListRelationships query. Zero-value fields
// are ignored (no filtering applied for that field).
type RelationshipFilter struct {
	// FromID filters by source entity ID; empty means any source.
	FromID string

	// ToID filters by target entity ID; empty means any target.
	ToID string

	// Name filters by relationship type label; empty means all labels.
	Name string
}
