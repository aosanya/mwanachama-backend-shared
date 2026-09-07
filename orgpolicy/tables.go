package orgpolicy

import (
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy/gormstore"
)

// TableNames configures which physical tables a [Store] reads and writes.
// See [gormstore.TableNames].
type TableNames = gormstore.TableNames

// DefaultTableNames returns the real production table names. See
// [gormstore.DefaultTableNames].
func DefaultTableNames() TableNames {
	return gormstore.DefaultTableNames()
}

// Migrate creates or updates the tables t names. See [gormstore.Migrate].
func Migrate(db *gorm.DB, t TableNames) error {
	return gormstore.Migrate(db, t)
}
