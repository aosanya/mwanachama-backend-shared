package orgsettings_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
)

// newTestStore builds an [orgsettings.Store] backed by a fresh in-memory
// sqlite database, migrated the same way a real deployment would via
// [orgsettings.Migrate] — mirroring mwanachama-backend-actor's
// newTestManager, so business logic is exercised against real GORM/SQL
// behavior with no container and no POSTGRES_URL. The dialling-region CHECK
// is postgres-only (see gormstore's syncDiallingRegionCheck), so it is not
// exercised here — that lives in postgres_integration_test.go.
func newTestStore(t *testing.T) orgsettings.Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	tables := orgsettings.DefaultTableNames()
	if err := orgsettings.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := orgsettings.NewStore(db, tables)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestNewStore_NilDB(t *testing.T) {
	if _, err := orgsettings.NewStore(nil, orgsettings.DefaultTableNames()); err == nil {
		t.Fatal("expected error for nil db")
	}
}
