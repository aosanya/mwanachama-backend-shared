package gormstore

import (
	"fmt"

	"gorm.io/gorm"
)

// TableNames configures which physical table orgsettings reads and writes.
// One field, not instance-parameterized like mwanachama-backend-actor's —
// org_settings is a singleton per deployment (one organization, one row),
// not a domain a mounting process mounts multiple copies of — but the
// struct still exists, rather than hard-coding the name, so a test can
// migrate a differently-named scratch table without colliding with a
// concurrent test run, the same reason mwanachama-backend-auth's
// TableNames exists despite also being fixed-name in production.
type TableNames struct {
	OrgSettings string
}

// DefaultTableNames returns the real production table name, "org_settings".
func DefaultTableNames() TableNames {
	return TableNames{OrgSettings: "org_settings"}
}

// Migrate creates or updates the org_settings table, via GORM's AutoMigrate
// plus the CHECK constraint AutoMigrate cannot express from a struct tag
// alone. Callers run this once at startup (or in test setup) before
// constructing a store with the same db and t.
func Migrate(db *gorm.DB, t TableNames) error {
	if err := db.Table(t.OrgSettings).AutoMigrate(&SettingsRow{}); err != nil {
		return err
	}
	return syncDiallingRegionCheck(db, t.OrgSettings)
}

// syncDiallingRegionCheck applies default_dialling_region's CHECK —
// ISO-3166-1 alpha-2 uppercase or empty — the same dialect-branching,
// drop-then-add idiom mwanachama-backend-forms's syncConstraints uses for
// its own CHECK constraints. Postgres-only, skipped on sqlite (same as
// every other repo's constraint sync): sqlite's ALTER TABLE cannot add a
// CHECK to an existing table, and the in-memory sqlite test harness this
// package's tests use doesn't need the constraint enforced to exercise
// Get/Put's business logic.
//
// Per G366/DSN-1479 (2026-08-22): a wrong region silently mis-hashes every
// nationally-formatted phone number entered under it with nothing to
// re-hash from — the wrongness is unrecoverable, which is exactly the class
// of value a Go-only guard (one call site away from being bypassed) is the
// wrong tool for.
func syncDiallingRegionCheck(db *gorm.DB, table string) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	constraint := table + "_dialling_region_shape"
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, table, constraint),
		fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s CHECK (default_dialling_region = '' OR default_dialling_region ~ '^[A-Z]{2}$')`,
			table, constraint),
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("syncDiallingRegionCheck: %w", err)
		}
	}
	return nil
}
