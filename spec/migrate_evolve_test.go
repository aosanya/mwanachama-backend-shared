package spec_test

import (
	"fmt"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const evolveSpecV1 = `{
  "module": "record",
  "domain": "probe",
  "instance": "probe1",
  "objects": [
    {
      "name": "widget",
      "role": "widget",
      "description": "One thing on a shelf.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "name", "type": "string", "description": "What it is called."}
      ]
    }
  ]
}`

const evolveSpecV2 = `{
  "module": "record",
  "domain": "probe",
  "instance": "probe1",
  "objects": [
    {
      "name": "widget",
      "role": "widget",
      "description": "One thing on a shelf.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "name", "type": "string", "description": "What it is called."},
        {"name": "color", "type": "string", "description": "What color it is."}
      ]
    }
  ]
}`

func TestS52_OpenBug_SecondMigrateDoesNotAddANewColumn(t *testing.T) {
	db := open(t)

	s1, err := spec.Parse([]byte(evolveSpecV1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := s1.TableFor(s1.Objects[0])

	s2, err := spec.Parse([]byte(evolveSpecV2))
	if err != nil {
		t.Fatalf("parse v2 (same object, 'color' added): %v", err)
	}
	if err := spec.Migrate(db, s2); err != nil {
		t.Fatalf("migrate v2 on the already-provisioned database: %v", err)
	}

	cols, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		t.Fatalf("read columns of %s: %v", table, err)
	}
	held := map[string]bool{}
	for _, c := range cols {
		held[c.Name()] = true
	}
	if !held["color"] {
		t.Fatalf("S52 open bug: after a second Migrate adding 'color' to the widget object, %s has columns %v — the blueprint gained a field but the already-provisioned table's schema was never altered to match, so Migrate is not the whole storage story for a domain that evolves its own blueprint, only for one that never has", table, held)
	}

	if err := db.Exec(fmt.Sprintf("insert into %s (id, name, color) values (?, ?, ?)", table), "w1", "widget one", "red").Error; err != nil {
		t.Fatalf("S52 open bug: insert naming the newly-declared 'color' column failed: %v — a caller who follows every CLAUDE.md's own convention ('adding a field means editing the blueprint and the Go type together') gets a raw driver error instead of the write it declared, on any instance migrated before the field was added", err)
	}
}
