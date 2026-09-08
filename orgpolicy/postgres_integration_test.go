// postgres_integration_test.go exercises the orgpolicy Store against a real
// Postgres database. Skipped unless POSTGRES_URL is set. The unit tests
// elsewhere in this package already exhaustively cover Get/Set's business
// logic; this file's job is narrower — prove the CHECK constraints
// syncPolicyChecks applies (sqlite cannot enforce them) actually refuse a
// bad row at the database, not merely in Go.
package orgpolicy_test

import (
	"context"
	"os"
	"testing"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

func newPostgresStore(t *testing.T) (orgpolicy.Repository, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test")
	}

	ctx := context.Background()
	sqlDB, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("postgres.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	tables := orgpolicy.DefaultTableNames()
	if err := orgpolicy.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := orgpolicy.NewStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store, db
}

func TestOrgPolicyRoundTripLive(t *testing.T) {
	s, db := newPostgresStore(t)
	ctx := context.Background()
	t.Cleanup(func() { db.Exec("DELETE FROM org_policy_overrides") })

	out, err := s.Set(ctx, orgpolicy.Policy{PublicAddressCap: 7, StructureMembershipCap: 2, FreeTextMaxLengthCap: 300, UpdatedBy: "live-test"})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if out.PublicAddressCap != 7 {
		t.Fatalf("Set returned %+v", out)
	}
	got, err := s.Get(ctx)
	if err != nil || got.PublicAddressCap != 7 {
		t.Fatalf("Get: %+v, %v", got, err)
	}

	five := 5
	if err := s.SetPublicAddressCapFor(ctx, "live-member-1", &five); err != nil {
		t.Fatalf("SetPublicAddressCapFor: %v", err)
	}
	if override, err := s.PublicAddressCapFor(ctx, "live-member-1"); err != nil || override == nil || *override != 5 {
		t.Fatalf("PublicAddressCapFor: %v, %v", override, err)
	}
}

// TestOrgPolicyCheckConstraintsRefuseAtTheDatabase proves syncPolicyChecks
// actually applied — a guard that only holds in Go (models.ValidateCap
// etc.) is one call site away from being bypassed.
func TestOrgPolicyCheckConstraintsRefuseAtTheDatabase(t *testing.T) {
	_, db := newPostgresStore(t)
	tables := orgpolicy.DefaultTableNames()

	if err := db.Exec(
		"INSERT INTO "+tables.Policy+" (singleton, public_address_cap, chapter_membership_cap, free_text_max_length_cap) "+
			"VALUES (true, -1, 5, 1000) ON CONFLICT (singleton) DO UPDATE SET public_address_cap = -1",
	).Error; err == nil {
		t.Error("the database accepted public_address_cap = -1; its CHECK should refuse it")
	}
	if err := db.Exec(
		"UPDATE "+tables.Policy+" SET chapter_membership_cap = 0 WHERE singleton",
	).Error; err == nil {
		t.Error("the database accepted chapter_membership_cap = 0; its CHECK should refuse it")
	}
}
