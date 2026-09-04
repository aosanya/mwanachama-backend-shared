package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/entitygraphtest"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

// TestBackend_Conformance runs the shared entitygraphtest suite against a
// real Postgres database. Skipped unless POSTGRES_URL is set — mirroring
// mwanachama-backend-api-gateway's split ("make test" covers memory with no
// database; "make pg && make test-pg" covers Postgres) so this package's
// default `go test ./...` needs no docker daemon.
func TestBackend_Conformance(t *testing.T) {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres conformance test (see Makefile's test-pg target)")
	}

	ctx := context.Background()
	db, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tables := postgres.DefaultTableNames("egtest_")
	if err := applyDDL(ctx, db, postgres.DDL(tables)); err != nil {
		t.Fatalf("applying DDL: %v", err)
	}
	t.Cleanup(func() {
		_ = applyDDL(context.Background(), db, postgres.DropDDL(tables))
	})

	b := postgres.NewBackend(db, tables)
	entitygraphtest.Run(t, b, b)
}

// applyDDL runs a multi-statement SQL script as one command, the way
// database/sql's ExecContext accepts it via the pgx stdlib driver (pgx
// supports Postgres's simple query protocol for multi-statement text).
func applyDDL(ctx context.Context, db *sql.DB, script string) error {
	_, err := db.ExecContext(ctx, script)
	return err
}
