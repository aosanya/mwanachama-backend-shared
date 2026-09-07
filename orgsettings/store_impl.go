// store_impl.go — org-settings Get/Put implementation for [Store]. Ported
// from mwanachama-backend-api-gateway's internal/store/postgres's original
// database/sql-backed orgsettings_store.go, moved onto GORM here to match
// this repo's own gormstore convention (DEV-1683 follow-up).
package orgsettings

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings/gormstore"
	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
)

// Store is the concrete implementation of [Repository].
type Store struct {
	db     *gorm.DB
	tables TableNames
}

// NewStore constructs a [Store] backed by db, reading and writing the table
// named by t (see [DefaultTableNames]). Callers must run [Migrate] against
// the same db and t before use. Returns an error if db is nil.
func NewStore(db *gorm.DB, t TableNames) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("NewStore: db must not be nil")
	}
	return &Store{db: db, tables: t}, nil
}

// Get returns the settings record for slug, or [ErrNotFound] when there is
// no row.
func (s *Store) Get(ctx context.Context, slug string) (models.Settings, error) {
	var row gormstore.SettingsRow
	err := s.db.WithContext(ctx).Table(s.tables.OrgSettings).Where("slug = ?", slug).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Settings{}, ErrNotFound
		}
		return models.Settings{}, fmt.Errorf("Get: %w", err)
	}
	return gormstore.SettingsFromRow(row), nil
}

// Put upserts a settings record by slug and returns the persisted value.
// Attributes is validated against [models.DefaultOrgSettingsProperties]
// first — a value of the wrong Range fails the call before any row is
// written, mirroring mwanachama-backend-actor's CreateActor.
//
// Uses an explicit ON CONFLICT clause rather than GORM's Save — Save treats
// a non-empty primary key as "do an UPDATE", which would silently affect
// zero rows on the very first Put for a slug instead of inserting one.
func (s *Store) Put(ctx context.Context, in models.Settings) (models.Settings, error) {
	if err := models.ValidateAttributes(models.DefaultOrgSettingsProperties(), in.Attributes); err != nil {
		return models.Settings{}, fmt.Errorf("%w: %v", ErrInvalidSettings, err)
	}
	row := gormstore.SettingsToRow(in)
	err := s.db.WithContext(ctx).Table(s.tables.OrgSettings).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "slug"}},
			UpdateAll: true,
		}).
		Create(&row).Error
	if err != nil {
		return models.Settings{}, fmt.Errorf("Put: %w", err)
	}
	return gormstore.SettingsFromRow(row), nil
}

// Compile-time interface check.
var _ Repository = (*Store)(nil)
