//go:build postgres

package spec_test

import (
	"fmt"
	"os"
	"testing"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func postgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test")
	}
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

func TestDropIndexesLeavesAConstraintBackedIndexAlone(t *testing.T) {
	db := postgresDB(t)
	table := fmt.Sprintf("spec_drop_idx_%d", os.Getpid())
	t.Cleanup(func() { db.Exec("drop table if exists " + table) })

	ddl := fmt.Sprintf(`create table %s (
		id text constraint auth_legacy_pkey primary key,
		phone text constraint auth_legacy_phone_key unique,
		note text
	)`, table)
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := db.Exec(fmt.Sprintf("create index auth_legacy_note on %s (note)", table)).Error; err != nil {
		t.Fatalf("seed the plain index: %v", err)
	}

	l := spec.Legacy{IndexPrefixes: []string{"auth_"}}
	if err := spec.DropIndexes(db, table, l); err != nil {
		t.Fatalf("DropIndexes refused an index that backs a constraint: %v", err)
	}

	var left []string
	q := `select i.relname from pg_index x
		join pg_class i on i.oid = x.indexrelid
		join pg_class t on t.oid = x.indrelid
		where t.relname = ? order by i.relname`
	if err := db.Raw(q, table).Scan(&left).Error; err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	want := map[string]bool{"auth_legacy_pkey": true, "auth_legacy_phone_key": true}
	if len(left) != len(want) {
		t.Fatalf("indexes left = %v, want exactly the two constraint-backed ones", left)
	}
	for _, name := range left {
		if !want[name] {
			t.Errorf("index %q survived but should have been dropped", name)
		}
	}
}
