package spec_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const namedLegacySpec = `{
  "module": "auth",
  "domain": "platform",
  "instance": "platform",
  "mount": "auth",
  "objects": [
    {
      "role": "credential",
      "name": "operator_credential",
      "table": "operator_credentials",
      "description": "A console sign-in.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "subject_id", "type": "string", "description": "Who it signs in as.", "required": true}
      ]
    }
  ]
}`

func namedLegacyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

// A module whose pre-spec table names followed no convention at all -- auth's
// operator_credential carries neither an instance nor a module segment -- can
// only be adopted if the spec is told the name. Nothing derives it.
func TestAdoptLegacyFollowsAnExplicitlyNamedTable(t *testing.T) {
	db := namedLegacyDB(t)
	s, err := spec.Parse([]byte(namedLegacySpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	legacy := spec.Legacy{Tables: map[string]string{"credential": "operator_credential"}}

	if err := db.Exec(`create table operator_credential (id text primary key, member_id text)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := db.Exec(`insert into operator_credential (id, member_id) values ('opcred-1', 'm1')`).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	if err := spec.AdoptLegacy(db, s, spec.Legacy{}); err != nil {
		t.Fatalf("adopt with no names: %v", err)
	}
	if !db.Migrator().HasTable("operator_credential") {
		t.Fatal("an unnamed legacy table was adopted by accident, so the test proves nothing")
	}

	if err := spec.AdoptLegacy(db, s, spec.Legacy{
		Tables:  legacy.Tables,
		Columns: map[string]string{"member_id": "subject_id"},
	}); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	declared := s.TableFor(s.Objects[0])
	if !db.Migrator().HasTable(declared) {
		t.Fatalf("legacy table was not renamed to %s", declared)
	}
	if db.Migrator().HasTable("operator_credential") {
		t.Fatal("the legacy table is still there")
	}
	held, err := spec.HasColumn(db, declared, "subject_id")
	if err != nil {
		t.Fatalf("has column: %v", err)
	}
	if !held {
		t.Fatal("member_id was not renamed to subject_id")
	}

	var rows []map[string]any
	if err := db.Table(declared).Find(&rows).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rows) != 1 || rows[0]["subject_id"] != "m1" {
		t.Fatalf("the adopted row did not survive: %v", rows)
	}
}

func TestAdoptedWithSeesAnExplicitlyNamedLegacyTable(t *testing.T) {
	db := namedLegacyDB(t)
	s, err := spec.Parse([]byte(namedLegacySpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	l := spec.Legacy{Tables: map[string]string{"credential": "operator_credential"}}

	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(`create table operator_credential (id text primary key)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	done, err := spec.AdoptedWith(db, s, l)
	if err != nil {
		t.Fatalf("adopted: %v", err)
	}
	if done {
		t.Fatal("adoption reported complete while the named legacy table is still present")
	}
}
