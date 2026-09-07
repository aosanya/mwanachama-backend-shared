// postgres_integration_test.go exercises the orgsettings Store against a
// real Postgres database, rather than the in-memory sqlite-backed store the
// rest of this package's tests use.
//
// Skipped unless POSTGRES_URL is set. The unit tests elsewhere in this
// package already exhaustively cover Get/Put's business logic; this file's
// job is narrower — prove the real Postgres wiring (GORM AutoMigrate, the
// ON CONFLICT upsert, and the dialling-region CHECK, which sqlite cannot
// enforce) works end-to-end.
package orgsettings_test

import (
	"context"
	"os"
	"testing"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
	"github.com/aosanya/mwanachama-backend-shared/orgsettings/models"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

// newPostgresStore opens POSTGRES_URL via mwanachama-backend-shared/postgres.Open
// (the same DSN parsing, pgx driver, and pooling every other repo already
// uses), wraps that connection with GORM's Postgres dialector, migrates the
// real org_settings table, and returns a ready-to-use Store. Skips the
// calling test if POSTGRES_URL is unset. Rows this test wrote are dropped
// on cleanup — the table itself is shared production shape, not a
// per-test scratch table (org_settings has no per-instance naming to
// scope one with).
func newPostgresStore(t *testing.T) (orgsettings.Repository, *gorm.DB) {
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

	tables := orgsettings.DefaultTableNames()
	if err := orgsettings.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store, err := orgsettings.NewStore(db, tables)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store, db
}

func cleanupSlug(t *testing.T, db *gorm.DB, table, slug string) {
	t.Helper()
	t.Cleanup(func() {
		db.Exec("DELETE FROM "+table+" WHERE slug = ?", slug)
	})
}

func TestOrgSettingsRoundTripLive(t *testing.T) {
	s, db := newPostgresStore(t)
	ctx := context.Background()
	cleanupSlug(t, db, "org_settings", "acme")

	in := models.Settings{
		Slug: "acme", DisplayName: "Acme", PrimaryColor: "#111",
		AccentColor: "#222", LogoURL: "l", SupportEmail: "s@a",
	}
	if _, err := s.Put(ctx, in); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "acme")
	if err != nil || got.DisplayName != "Acme" {
		t.Fatalf("Get: %v %+v", err, got)
	}
}

// TestOrgSettingsDiallingRegionRoundTripLive is DEV-1258's half of G366: the
// region the canonicalizer parses against is useless if the store cannot
// carry it.
//
// It asserts three separate things, because the column has three states and
// two of them are easy to get wrong:
//
//  1. A set region survives a Put/Get round trip.
//  2. An unset one comes back as the empty string rather than an error or a
//     guessed country — phone-salt.md's accepted trade-off is that a wrong
//     region is unrecoverable, so "nobody has set one" has to be
//     representable and distinguishable.
//  3. The CHECK refuses a malformed code through the store, not merely
//     through psql. A guard that only holds when you write SQL by hand is
//     one call site away from being bypassed.
func TestOrgSettingsDiallingRegionRoundTripLive(t *testing.T) {
	s, db := newPostgresStore(t)
	ctx := context.Background()
	cleanupSlug(t, db, "org_settings", "withregion")
	cleanupSlug(t, db, "org_settings", "noregion")
	cleanupSlug(t, db, "org_settings", "badregion")

	set, err := s.Put(ctx, models.Settings{Slug: "withregion", DefaultDiallingRegion: "KE"})
	if err != nil {
		t.Fatalf("Put with region: %v", err)
	}
	if set.DefaultDiallingRegion != "KE" {
		t.Errorf("Put returned region %q, want %q", set.DefaultDiallingRegion, "KE")
	}
	got, err := s.Get(ctx, "withregion")
	if err != nil {
		t.Fatalf("Get with region: %v", err)
	}
	if got.DefaultDiallingRegion != "KE" {
		t.Errorf("Get returned region %q, want %q", got.DefaultDiallingRegion, "KE")
	}

	if _, err := s.Put(ctx, models.Settings{Slug: "noregion"}); err != nil {
		t.Fatalf("Put without region: %v", err)
	}
	unset, err := s.Get(ctx, "noregion")
	if err != nil {
		t.Fatalf("Get without region: %v", err)
	}
	if unset.DefaultDiallingRegion != "" {
		t.Errorf("unset region came back as %q, want empty", unset.DefaultDiallingRegion)
	}

	if _, err := s.Put(ctx, models.Settings{Slug: "badregion", DefaultDiallingRegion: "ke"}); err == nil {
		t.Error("Put accepted lowercase region \"ke\"; the CHECK should refuse it")
	} else if err != orgsettings.ErrInvalidDiallingRegion {
		t.Errorf("Put with bad region returned %v, want ErrInvalidDiallingRegion", err)
	}
}
