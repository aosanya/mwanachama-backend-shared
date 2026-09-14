// Package gormtest is the sqlite/Postgres test-database harness every
// GORM-backed repo in this family rebuilds by hand: an in-memory sqlite
// *gorm.DB for fast unit tests, and a POSTGRES_URL-gated real-Postgres one
// for integration tests. Migration and table cleanup stay with the caller
// — each repo's own Migrate/TableNames shape differs.
package gormtest

import (
	"context"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

// OpenSQLiteDB opens a fresh in-memory sqlite *gorm.DB. The caller migrates
// it with its own Migrate function before use.
func OpenSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite): %v", err)
	}
	return db
}

// OpenPostgresDB opens POSTGRES_URL via this repo's own postgres.Open,
// wrapped in GORM's Postgres dialector, and skips the calling test if
// POSTGRES_URL is unset. prefix identifies the caller in a skip/fatal
// message only — table naming and migration are still the caller's own.
// The returned cleanup closes the underlying connection; the caller is
// still responsible for dropping its own tables in its own t.Cleanup.
func OpenPostgresDB(t *testing.T, prefix string) (*gorm.DB, func()) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test")
	}

	sqlDB, err := postgres.Open(context.Background(), postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("postgres.Open(%s): %v", prefix, err)
	}

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open(postgres, %s): %v", prefix, err)
	}

	return db, func() { _ = sqlDB.Close() }
}
