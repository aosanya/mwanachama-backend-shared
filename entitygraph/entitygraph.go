// Package entitygraph provides the generic DataManager and SchemaManager
// interfaces — a typed, graph-structured entity store backed by a versioned
// [schema.Schema] — plus a Postgres implementation of both.
//
// Ported from github.com/aosanya/CodeValdSharedLib/entitygraph (ArangoDB-
// backed, built for CodeValdCortex agencies) for
// github.com/aosanya/mwanachama-backend-git and
// github.com/aosanya/mwanachama-backend-taskmanager, which port CodeValdGit's and
// CodeValdWork's business logic onto this contract. The interfaces below are
// unchanged from the original; only the backing storage differs.
package entitygraph

import (
	"context"
	"errors"
)

// ErrInvalidRelationship is returned by CreateRelationship when the edge
// label is not declared in the source TypeDefinition.Relationships, or when
// the target entity's TypeID does not match RelationshipDefinition.ToType.
var ErrInvalidRelationship = errors.New("invalid relationship")

// ErrRelationshipCardinalityViolation is returned by CreateRelationship when
// a second edge with the same label is created from the same source entity
// and RelationshipDefinition.ToMany is false (functional / at-most-one).
var ErrRelationshipCardinalityViolation = errors.New("relationship cardinality violation")

// ErrRequiredRelationshipViolation is returned when an operation (e.g.
// DeleteEntity) would leave an entity without at least one edge for a
// relationship declared with Required = true.
var ErrRequiredRelationshipViolation = errors.New("required relationship violation")

// ErrSchemaNotFound is returned by SchemaManager methods when no schema
// document (draft or published) exists for the given version.
var ErrSchemaNotFound = errors.New("schema not found")

// ErrEntityNotFound is returned by GetEntity, UpdateEntity, DeleteEntity, and
// CreateRelationship when the referenced entity does not exist.
var ErrEntityNotFound = errors.New("entity not found")

// ErrEntityAlreadyExists is returned by CreateEntity when an entity with the
// same ID already exists.
var ErrEntityAlreadyExists = errors.New("entity already exists")

// ErrRelationshipNotFound is returned by DeleteRelationship when no
// relationship with the given ID exists.
var ErrRelationshipNotFound = errors.New("relationship not found")

// ErrImmutableType is returned by UpdateEntity when the entity's
// TypeDefinition has Immutable set to true.
var ErrImmutableType = errors.New("entity type is immutable")

// ErrUniqueKeyNotDefined is returned by UpsertEntity when the entity's
// TypeDefinition does not declare a UniqueKey (schema.TypeDefinition.UniqueKey
// is empty or nil).
var ErrUniqueKeyNotDefined = errors.New("unique key not defined for type")

// DataManager is the business-logic entry point for entity lifecycle.
// Relationship CRUD (CreateRelationship/DeleteRelationship/
// ListRelationships) is deliberately not part of this interface — each
// backend (postgres.Backend, postgres.PerCollectionBackend, memory.Backend)
// still implements those as plain exported methods, but each consumer knows
// its own fixed set of relationship labels and physical tables at compile
// time, so it declares its own narrower local interface embedding
// DataManager plus the relationship methods it needs, rather than routing
// through one generic, storage-agnostic filter here.
//
// Schema operations are not in scope — see SchemaManager.
//
// Immutable types (TypeDefinition.Immutable == true) reject UpdateEntity
// with ErrImmutableType.
//
// All methods accept context.Context as the first argument for cancellation
// and deadline propagation.
type DataManager interface {
	// CreateEntity creates a new entity of the given type. The TypeID must
	// match a TypeDefinition.Name in the current schema. Returns
	// ErrEntityAlreadyExists if an entity with the same ID already exists.
	CreateEntity(ctx context.Context, req CreateEntityRequest) (Entity, error)

	// GetEntity returns the entity identified by entityID. Returns
	// ErrEntityNotFound if no entity matches.
	GetEntity(ctx context.Context, entityID string) (Entity, error)

	// UpdateEntity patches the properties of an existing entity. Returns
	// ErrEntityNotFound if the entity does not exist. Returns
	// ErrImmutableType if the entity's type has Immutable set to true.
	UpdateEntity(ctx context.Context, entityID string, req UpdateEntityRequest) (Entity, error)

	// DeleteEntity soft-deletes the entity by setting Deleted=true and
	// recording DeletedAt. Relationships referencing the entity are
	// retained as orphans. Returns ErrEntityNotFound if the entity does not
	// exist.
	DeleteEntity(ctx context.Context, entityID string) error

	// ListEntities returns all entities matching the filter. Soft-deleted
	// entities are excluded from the results.
	ListEntities(ctx context.Context, filter EntityFilter) ([]Entity, error)

	// UpsertEntity creates or merges an entity using the type's UniqueKey.
	// If a non-deleted entity whose UniqueKey property values match the
	// request already exists, its properties are patched (merged) and the
	// updated entity is returned. Otherwise a new entity is inserted.
	// Returns ErrUniqueKeyNotDefined if the TypeDefinition has no
	// UniqueKey declared.
	UpsertEntity(ctx context.Context, req CreateEntityRequest) (Entity, error)
}
