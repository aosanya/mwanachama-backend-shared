package spec_test

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const orphanSpecV1 = `{
  "module": "record", "domain": "probe", "instance": "orphans",
  "objects": [{
    "name": "widget", "description": "One thing on a shelf.",
    "fields": [
      {"name": "org", "type": "string", "description": "Owner.", "primary": true},
      {"name": "id", "type": "string", "description": "Key.", "primary": true},
      {"name": "code", "type": "string", "description": "Unique code.", "unique": true, "required": true},
      {"name": "color", "type": "string", "description": "Color.", "required": true},
      {"name": "weight", "type": "int", "description": "Weight.", "required": true},
      {"name": "note", "type": "string", "description": "Note.", "nullable": true}
    ],
    "indexes": [{"name": "by_color", "fields": ["org", "code"]}]
  }]
}`

const orphanSpecV2 = `{
  "module": "record", "domain": "probe", "instance": "orphans",
  "objects": [{
    "name": "widget", "description": "One thing on a shelf.",
    "fields": [
      {"name": "org", "type": "string", "description": "Owner.", "primary": true},
      {"name": "id", "type": "string", "description": "Key.", "primary": true},
      {"name": "code", "type": "string", "description": "Unique code.", "unique": true, "required": true}
    ],
    "indexes": [{"name": "by_color", "fields": ["org", "code"]}]
  }]
}`

func migrateBoth(t *testing.T, db *gorm.DB) (*spec.Spec, *spec.Spec) {
	t.Helper()
	s1, err := spec.Parse([]byte(orphanSpecV1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := s1.TableFor(s1.Objects[0])
	if err := db.Exec("insert into " + table + " (org, id, code, color, weight) values ('o', 'w1', 'c1', 'red', 5)").Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	s2, err := spec.Parse([]byte(orphanSpecV2))
	if err != nil {
		t.Fatalf("parse v2: %v", err)
	}
	if err := spec.Migrate(db, s2); err != nil {
		t.Fatalf("migrate v2: %v", err)
	}
	return s1, s2
}

func TestS53_DroppedRequiredFieldsKeepTheirDataAndStopBlockingWrites(t *testing.T) {
	db := open(t)
	_, s2 := migrateBoth(t, db)
	table := s2.TableFor(s2.Objects[0])

	if err := db.Exec("insert into " + table + " (org, id, code) values ('o', 'w2', 'c2')").Error; err != nil {
		t.Fatalf("a write naming only the declared columns failed: %v", err)
	}
	var color string
	var weight int
	if err := db.Raw("select color, weight from "+table+" where id = 'w1'").Row().Scan(&color, &weight); err != nil {
		t.Fatalf("read the orphan columns: %v", err)
	}
	if color != "red" || weight != 5 {
		t.Fatalf("the dropped columns lost their data: color=%q weight=%d", color, weight)
	}
}

func TestS53_RelaxingKeepsTheKeyTheUniqueConstraintAndTheIndexes(t *testing.T) {
	db := open(t)
	_, s2 := migrateBoth(t, db)
	table := s2.TableFor(s2.Objects[0])

	if err := db.Exec("insert into " + table + " (org, id, code) values ('o', 'w1', 'c9')").Error; err == nil {
		t.Error("the composite primary key no longer rejects a repeated (org, id)")
	}
	if err := db.Exec("insert into " + table + " (org, id, code) values ('o', 'w3', 'c1')").Error; err == nil {
		t.Error("the unique constraint on code no longer rejects a repeated code")
	}
	var indexes []string
	db.Raw(`select name from sqlite_master where type = 'index' and tbl_name = ?`, table).Scan(&indexes)
	if !strings.Contains(strings.Join(indexes, " "), "by_color") {
		t.Errorf("the declared index is gone: %v", indexes)
	}
}

func TestS53_TheTestCopyIsRelaxedToo(t *testing.T) {
	db := open(t)
	_, s2 := migrateBoth(t, db)
	test := s2.TestTableFor(s2.Objects[0])

	if err := db.Exec("insert into " + test + " (run_id, org, id, code) values ('r1', 'o', 'w1', 'c1')").Error; err != nil {
		t.Fatalf("a write to the test copy naming only the declared columns failed: %v", err)
	}
	if err := db.Exec("insert into " + test + " (run_id, org, id, code) values ('r1', 'o', 'w1', 'c2')").Error; err == nil {
		t.Error("the test copy's key (run_id, org, id) no longer rejects a repeat")
	}
}

func TestS53_ASecondMigrateChangesNothing(t *testing.T) {
	db := open(t)
	_, s2 := migrateBoth(t, db)
	table := s2.TableFor(s2.Objects[0])

	var before string
	db.Raw(`select sql from sqlite_master where name = ?`, table).Scan(&before)
	if err := spec.Migrate(db, s2); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var after string
	db.Raw(`select sql from sqlite_master where name = ?`, table).Scan(&after)
	if before != after {
		t.Fatalf("a second Migrate rewrote the table:\n%s\n%s", before, after)
	}
}

func TestS53_ADroppedColumnThatNeverBlockedIsLeftAlone(t *testing.T) {
	db := open(t)
	s1, err := spec.Parse([]byte(orphanSpecV1))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	table := s1.TableFor(s1.Objects[0])
	if err := db.Exec("alter table " + table + " add column legacy_note text not null default 'x'").Error; err != nil {
		t.Fatalf("add a defaulted not-null column: %v", err)
	}
	var before string
	db.Raw(`select sql from sqlite_master where name = ?`, table).Scan(&before)
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate again: %v", err)
	}
	var after string
	db.Raw(`select sql from sqlite_master where name = ?`, table).Scan(&after)
	if before != after {
		t.Fatalf("a not-null orphan with a default was rebuilt:\n%s\n%s", before, after)
	}
}

func TestS53_PostgresRelaxesAnOrphanWithADropNotNull(t *testing.T) {
	got := spec.RelaxStatements("t_1", []string{"color", "we\"ird"})
	want := []string{
		`alter table t_1 alter column "color" drop not null`,
		`alter table t_1 alter column "we""ird" drop not null`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %v want %v", got, want)
	}
}
