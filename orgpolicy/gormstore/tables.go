package gormstore

import (
	"fmt"

	"gorm.io/gorm"
)

// TableNames configures which physical tables orgpolicy reads and writes.
// Policy is a singleton (one deployment, one row); Overrides has one row per
// named member. Neither is instance-parameterized — org_policy is not a
// domain a mounting process mounts multiple copies of — but the struct still
// exists so a test can migrate a differently-named scratch pair.
type TableNames struct {
	Policy    string
	Overrides string
}

// DefaultTableNames returns the real production table names, ported
// unchanged from the original migrations: "org_policy" and
// "org_policy_overrides" (new — see override.go).
func DefaultTableNames() TableNames {
	return TableNames{Policy: "org_policy", Overrides: "org_policy_overrides"}
}

// Migrate creates or updates both tables via GORM's AutoMigrate, plus the
// CHECK constraints AutoMigrate cannot express from a struct tag alone —
// the same dialect-branching, drop-then-add idiom
// mwanachama-backend-forms's syncConstraints and orgsettings's (former)
// syncDiallingRegionCheck use. Callers run this once at startup (or in test
// setup) before constructing a store with the same db and t.
func Migrate(db *gorm.DB, t TableNames) error {
	if err := db.Table(t.Policy).AutoMigrate(&PolicyRow{}); err != nil {
		return err
	}
	if err := db.Table(t.Overrides).AutoMigrate(&OverrideRow{}); err != nil {
		return err
	}
	return syncPolicyChecks(db, t.Policy)
}

// syncPolicyChecks applies org_policy's four CHECKs, ported unchanged from
// migrations 000046/000054/000055: singleton must be true (so at most one
// row can ever exist — a second insert either collides on the primary key
// or fails this CHECK), and each cap's floor (public_address_cap >= 0,
// chapter_membership_cap >= 1, free_text_max_length_cap >= 1) — see
// models.ValidateCap/ValidateMembershipCap/ValidateFreeTextMaxLengthCap for
// why the floors differ. Postgres-only, skipped on sqlite (same as every
// other repo's constraint sync): the in-memory sqlite test harness this
// package's tests use doesn't need the constraint enforced to exercise
// Get/Set's business logic, which already checks the same floors in Go
// before any row is written.
func syncPolicyChecks(db *gorm.DB, table string) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	checks := map[string]string{
		table + "_singleton_check":               "singleton",
		table + "_public_address_cap_check":       "public_address_cap >= 0",
		table + "_chapter_membership_cap_check":   "chapter_membership_cap >= 1",
		table + "_free_text_max_length_cap_check": "free_text_max_length_cap >= 1",
	}
	for name, expr := range checks {
		stmts := []string{
			fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, table, name),
			fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s)`, table, name, expr),
		}
		for _, sql := range stmts {
			if err := db.Exec(sql).Error; err != nil {
				return fmt.Errorf("syncPolicyChecks: %s: %w", name, err)
			}
		}
	}
	return nil
}
