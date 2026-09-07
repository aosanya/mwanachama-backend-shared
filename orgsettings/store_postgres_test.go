package orgsettings_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

// liveDB opens a real Postgres connection and ensures the org_settings table
// exists, skipping the test when POSTGRES_URL is unset — mirroring
// postgres/backend_test.go's TestBackend_Conformance so `go test ./...` needs
// no docker daemon by default (see Makefile's test-pg target).
func liveDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test (see Makefile's test-pg target)")
	}
	ctx := context.Background()
	db, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := orgsettings.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func cleanupSlug(t *testing.T, db *sql.DB, slug string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM org_settings WHERE slug = $1`, slug)
	})
}

func TestOrgSettingsRoundTrip(t *testing.T) {
	db := liveDB(t)
	s := orgsettings.NewStore(db)
	ctx := context.Background()
	cleanupSlug(t, db, "acme")

	in := orgsettings.Settings{
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

// TestOrgSettingsDiallingRegionRoundTrip is DEV-1258's half of G366: the
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
func TestOrgSettingsDiallingRegionRoundTrip(t *testing.T) {
	db := liveDB(t)
	s := orgsettings.NewStore(db)
	ctx := context.Background()
	cleanupSlug(t, db, "withregion")
	cleanupSlug(t, db, "noregion")
	cleanupSlug(t, db, "badregion")

	set, err := s.Put(ctx, orgsettings.Settings{Slug: "withregion", DefaultDiallingRegion: "KE"})
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

	if _, err := s.Put(ctx, orgsettings.Settings{Slug: "noregion"}); err != nil {
		t.Fatalf("Put without region: %v", err)
	}
	unset, err := s.Get(ctx, "noregion")
	if err != nil {
		t.Fatalf("Get without region: %v", err)
	}
	if unset.DefaultDiallingRegion != "" {
		t.Errorf("unset region came back as %q, want empty", unset.DefaultDiallingRegion)
	}

	if _, err := s.Put(ctx, orgsettings.Settings{Slug: "badregion", DefaultDiallingRegion: "ke"}); err == nil {
		t.Error("Put accepted lowercase region \"ke\"; the CHECK should refuse it")
	} else if err != orgsettings.ErrInvalidDiallingRegion {
		t.Errorf("Put with bad region returned %v, want ErrInvalidDiallingRegion", err)
	}
}
