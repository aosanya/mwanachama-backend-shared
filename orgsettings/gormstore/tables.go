package gormstore

import "gorm.io/gorm"

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

// Migrate creates or updates the org_settings table via GORM's AutoMigrate.
// No raw-SQL constraint sync is needed here (unlike
// mwanachama-backend-actor's syncUniqueAttributeIndexes) — every declared
// value lives in the Attributes JSONB blob with no per-property database
// constraint, validated only in Go by models.ValidateAttributes before
// every Put, the same as mwanachama-backend-actor's phone/email Range
// checks. Callers run this once at startup (or in test setup) before
// constructing a store with the same db and t.
func Migrate(db *gorm.DB, t TableNames) error {
	return db.Table(t.OrgSettings).AutoMigrate(&SettingsRow{})
}
