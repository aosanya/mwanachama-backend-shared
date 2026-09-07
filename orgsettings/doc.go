// Package orgsettings is the unauthenticated org-branding/config lookup
// used by the client shell (colours, logo, name), plus the
// organization-wide settings expected to grow alongside it — named for
// that future scope rather than just what it holds today (DEV-1683).
//
// Structured like mwanachama-backend-actor: domain types in models/, GORM
// row/migration in gormstore/, and an HTTP surface in routes/, with this
// root package holding the Repository interface and its implementation —
// the same split every GORM-backed repo in this family uses.
package orgsettings

import (
	"context"
	"errors"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
)

// ErrNotFound is returned when no settings record exists for a slug.
var ErrNotFound = errors.New("orgsettings: not found")

// ErrInvalidSettings is returned when Put's Attributes fails
// [ValidateAttributes] against [DefaultOrgSettingsProperties] — the
// caller's mistake, not a server fault. Mirrors mwanachama-backend-actor's
// ErrInvalidActor exactly: a generic write-was-invalid sentinel wrapping
// the specific field.Errorf detail, kept local to this package rather than
// routed through the gateway's cross-domain domerr.ErrInvalidReference
// sentinel, since domerr hasn't moved to this repo (a separate, not-yet-done
// pass); callers that want a domerr-shaped error can wrap this themselves.
var ErrInvalidSettings = errors.New("orgsettings: invalid settings")

// Settings, PublicSettings and Property are aliases of their models.
// counterparts, and DefaultOrgSettingsProperties/ValidateAttributes forward
// to the models. functions of the same name — so a caller needs only this
// package's import, never models's directly.
type (
	Settings       = models.Settings
	PublicSettings = models.PublicSettings
	Property       = models.Property
)

// DefaultOrgSettingsProperties returns the built-in property catalog. See
// [models.DefaultOrgSettingsProperties].
func DefaultOrgSettingsProperties() []Property {
	return models.DefaultOrgSettingsProperties()
}

// ValidateAttributes checks attrs against properties. See
// [models.ValidateAttributes].
func ValidateAttributes(properties []Property, attrs map[string]any) error {
	return models.ValidateAttributes(properties, attrs)
}

// Repository is the persistence boundary for the org-settings domain.
type Repository interface {
	Get(ctx context.Context, slug string) (Settings, error)
	Put(ctx context.Context, s Settings) (Settings, error)
}
