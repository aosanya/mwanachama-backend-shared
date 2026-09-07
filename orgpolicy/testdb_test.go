package orgpolicy_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy"
)

// newTestStore builds an [orgpolicy.Store] backed by a fresh in-memory
// sqlite database, migrated the same way a real deployment would via
// [orgpolicy.Migrate] — mirrors mwanachama-backend-actor's newTestManager
// and this repo's own orgsettings' newTestStore.
//
// exists stands in for member existence; nil means every memberID is
// accepted, matching NewStore's own default.
func newTestStore(t *testing.T, exists orgpolicy.MemberExists) orgpolicy.Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	tables := orgpolicy.DefaultTableNames()
	if err := orgpolicy.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := orgpolicy.NewStore(db, tables, exists)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestNewStore_NilDB(t *testing.T) {
	if _, err := orgpolicy.NewStore(nil, orgpolicy.DefaultTableNames(), nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

func alwaysExists(context.Context, string) bool { return true }
func neverExists(context.Context, string) bool  { return false }
