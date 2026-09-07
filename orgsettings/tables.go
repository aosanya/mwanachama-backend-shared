package orgsettings

import (
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings/gormstore"
)

// TableNames configures which physical table a [Store] reads and writes.
// See [gormstore.TableNames].
type TableNames = gormstore.TableNames

// DefaultTableNames returns the real production table name, "org_settings".
// See [gormstore.DefaultTableNames].
func DefaultTableNames() TableNames {
	return gormstore.DefaultTableNames()
}

// Migrate creates or updates the table t names. See [gormstore.Migrate].
func Migrate(db *gorm.DB, t TableNames) error {
	return gormstore.Migrate(db, t)
}
