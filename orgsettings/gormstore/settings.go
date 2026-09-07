// Package gormstore holds every GORM-specific piece of orgsettings: the row
// struct, its conversion to/from models.Settings, and table migration —
// mirroring mwanachama-backend-actor's identical gormstore/ split. Nothing
// outside this package (and the root orgsettings package's *_impl.go files,
// which call it) needs to know GORM exists.
package gormstore

import (
	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
)

// SettingsRow is the GORM row for a [models.Settings].
type SettingsRow struct {
	Slug                  string `gorm:"primaryKey"`
	DisplayName           string `gorm:"not null;default:''"`
	PrimaryColor          string `gorm:"not null;default:''"`
	AccentColor           string `gorm:"not null;default:''"`
	LogoURL               string `gorm:"not null;default:''"`
	SupportEmail          string `gorm:"not null;default:''"`
	DefaultDiallingRegion string `gorm:"not null;default:''"`
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
	return SettingsRow{
		Slug:                  s.Slug,
		DisplayName:           s.DisplayName,
		PrimaryColor:          s.PrimaryColor,
		AccentColor:           s.AccentColor,
		LogoURL:               s.LogoURL,
		SupportEmail:          s.SupportEmail,
		DefaultDiallingRegion: s.DefaultDiallingRegion,
		UpdatedAt:             models.NowRFC3339(),
	}
}

// SettingsFromRow converts a row back to the domain Settings.
func SettingsFromRow(r SettingsRow) models.Settings {
	return models.Settings{
		Slug:                  r.Slug,
		DisplayName:           r.DisplayName,
		PrimaryColor:          r.PrimaryColor,
		AccentColor:           r.AccentColor,
		LogoURL:               r.LogoURL,
		SupportEmail:          r.SupportEmail,
		DefaultDiallingRegion: r.DefaultDiallingRegion,
	}
}
