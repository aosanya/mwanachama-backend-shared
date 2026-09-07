// postgres_integration_test.go exercises the orgsettings Store against a
// real Postgres database, rather than the in-memory sqlite-backed store the
// rest of this package's tests use.
//
// Skipped unless POSTGRES_URL is set. The unit tests elsewhere in this
// package already exhaustively cover Get/Put's business logic — including
// ErrInvalidSettings, which is Go-level only now (DEV-1683 follow-up moved
// default_dialling_region off a dedicated column with a database CHECK and
// into Attributes, matching mwanachama-backend-actor's phone/email, so
// there is no longer a Postgres-only guarantee to exercise here). This
// file's job is narrower — prove the real Postgres wiring (GORM
// AutoMigrate, the JSONB Attributes round-trip, and the ON CONFLICT upsert)
// works end-to-end.
package orgsettings_test

import (
	"context"
	"os"
	"testing"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
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

	in := orgsettings.Settings{
		Slug: "acme",
		Attributes: map[string]any{
			"display_name":            "Acme",
			"primary_color":           "#111",
			"accent_color":            "#222",
			"logo_url":                "l",
			"support_email":           "s@a",
			"default_dialling_region": "KE",
		},
	}
	if _, err := s.Put(ctx, in); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "acme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DisplayName() != "Acme" || got.DefaultDiallingRegion() != "KE" {
		t.Fatalf("JSONB round-trip lost fields: %+v", got)
	}

	// Put upserts — the ON CONFLICT path, against real Postgres rather than
	// sqlite's dialect (store_impl_test.go already covers this on sqlite;
	// this confirms the same clause.OnConflict SQL is valid Postgres).
	if _, err := s.Put(ctx, orgsettings.Settings{
		Slug:       "acme",
		Attributes: map[string]any{"display_name": "Acme Movement"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err = s.Get(ctx, "acme")
	if err != nil {
		t.Fatalf("Get after upsert: %v", err)
	}
	if got.DisplayName() != "Acme Movement" {
		t.Fatalf("expected upsert to replace display_name, got %+v", got)
	}
}
