package entitygraph

import (
	"context"

	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// SchemaManager is the schema storage contract injected into a concrete
// DataManager implementation. It separates the mutable draft schema (one
// document per agency, overwritten by SetSchema) from the immutable
// published history (append-only snapshots produced by Publish and
// Activate).
//
// Updating the draft does not affect live traffic — callers must Publish
// and then Activate a version before it is used by CreateEntity /
// CreateRelationship.
type SchemaManager interface {
	// Draft — one mutable document per agency.

	// SetSchema overwrites the agency's current draft schema. The draft is
	// never versioned; only published snapshots carry version numbers.
	// ValidateSchema is NOT called here — invalid drafts are permitted
	// until Publish.
	SetSchema(ctx context.Context, s schema.Schema) error

	// GetSchema returns the agency's current draft schema. Returns
	// ErrSchemaNotFound if no draft has been created yet.
	GetSchema(ctx context.Context, agencyID string) (schema.Schema, error)

	// Published — immutable, append-only.

	// Publish validates the current draft (ValidateSchema) and snapshots
	// it into the published collection as a new version with Active =
	// false. The version number is auto-assigned (highest existing + 1;
	// first publish = 1). Returns an error and creates no snapshot if
	// validation fails or no draft exists.
	Publish(ctx context.Context, agencyID string) error

	// Activate promotes the given published version to active, setting
	// Active = true on the target and Active = false on any previously
	// active version, in a single transaction. Returns ErrSchemaNotFound
	// if the version does not exist.
	Activate(ctx context.Context, agencyID string, version int) error

	// GetActive returns the single published version where Active ==
	// true. Returns ErrSchemaNotFound if no version has been activated
	// yet.
	GetActive(ctx context.Context, agencyID string) (schema.Schema, error)

	// GetVersion returns a specific published version. Returns
	// ErrSchemaNotFound if the version does not exist.
	GetVersion(ctx context.Context, agencyID string, version int) (schema.Schema, error)

	// ListVersions returns all published versions for the agency in
	// ascending version order. Includes both active and inactive
	// versions.
	ListVersions(ctx context.Context, agencyID string) ([]schema.Schema, error)
}
