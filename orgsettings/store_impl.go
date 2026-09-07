// store_impl.go — org-settings Get/Put implementation for [Store]. Ported
// from mwanachama-backend-api-gateway's internal/store/postgres's original
// database/sql-backed orgchrome_store.go, moved onto GORM here to match
// this repo's own gormstore convention (DEV-1683 follow-up).
package orgsettings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings/gormstore"
	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
)

// sqlstateCheckViolation is Postgres's SQLSTATE for a violated CHECK
// constraint — org_settings_dialling_region_shape is the only one this
// table carries.
const sqlstateCheckViolation = "23514"

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
func (s *Store) Put(ctx context.Context, in models.Settings) (models.Settings, error) {
	row := gormstore.SettingsToRow(in)
	err := s.db.WithContext(ctx).Table(s.tables.OrgSettings).Save(&row).Error
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == sqlstateCheckViolation {
			return models.Settings{}, ErrInvalidDiallingRegion
		}
		return models.Settings{}, fmt.Errorf("Put: %w", err)
	}
	return gormstore.SettingsFromRow(row), nil
}

// Compile-time interface check.
var _ Repository = (*Store)(nil)
