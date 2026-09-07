// Package gormstore holds every GORM-specific piece of orgsettings: the row
// struct, its conversion to/from models.Settings, and table migration —
// mirroring mwanachama-backend-actor's identical gormstore/ split. Nothing
// outside this package (and the root orgsettings package's *_impl.go files,
// which call it) needs to know GORM exists.
package gormstore

import (
	"gorm.io/datatypes"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
)

// SettingsRow is the GORM row for a [models.Settings]. Slug is the only
// dedicated column beyond bookkeeping — every declared value
// (display_name, primary_color, accent_color, logo_url, support_email,
// default_dialling_region) lives in Attributes, matching
// mwanachama-backend-actor's ActorRow exactly.
type SettingsRow struct {
	Slug string `gorm:"primaryKey"`
	// Attributes stores the whole property bag as native JSONB, the same
	// datatypes.JSONMap round-trip mwanachama-backend-actor's ActorRow uses.
	Attributes datatypes.JSONMap
	// UpdatedAt stays a hand-formatted RFC 3339 string rather than a
	// GORM-managed time.Time column, matching mwanachama-backend-actor's
	// timestamp convention (models/time.go) — not read back into
	// models.Settings today (no caller needs it), but stamped on every write
	// so the row itself carries an honest last-write time.
	UpdatedAt string
}

// SettingsToRow converts a domain Settings to its row shape, stamping
// UpdatedAt.
func SettingsToRow(s models.Settings) SettingsRow {
	var attrs datatypes.JSONMap
	if len(s.Attributes) > 0 {
		attrs = datatypes.JSONMap(s.Attributes)
	}
	return SettingsRow{
		Slug:       s.Slug,
		Attributes: attrs,
		UpdatedAt:  models.NowRFC3339(),
	}
}

// SettingsFromRow converts a row back to the domain Settings.
func SettingsFromRow(r SettingsRow) models.Settings {
	s := models.Settings{Slug: r.Slug}
	if len(r.Attributes) > 0 {
		s.Attributes = map[string]any(r.Attributes)
	}
	return s
}
