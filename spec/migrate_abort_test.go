package spec_test

import (
	"fmt"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const abortSpecV1 = `{
  "module": "record",
  "domain": "probe",
  "instance": "probe3",
  "objects": [
    {
      "name": "widget",
      "role": "widget",
      "description": "One thing on a shelf.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "sku", "type": "string", "description": "Catalog code."}
      ]
    }
  ]
}`

const abortSpecV2 = `{
  "module": "record",
  "domain": "probe",
  "instance": "probe3",
  "objects": [
    {
      "name": "widget",
      "role": "widget",
      "description": "One thing on a shelf.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "sku", "type": "string", "description": "Catalog code.", "unique": true}
      ],
      "indexes": [
        {"name": "sku_unique", "fields": ["sku"], "unique": true}
      ]
    },
    {
      "name": "gadget",
      "role": "gadget",
      "description": "A second, brand-new object declared in the same blueprint change as widget's new unique index.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "label", "type": "string", "description": "What it is called."}
      ]
    }
  ]
}`

func TestS54_OpenBug_MigrateAbandonsEveryLaterObjectOnAnEarlierOnesDDLFailure(t *testing.T) {
	db := open(t)

	s1, err := spec.Parse([]byte(abortSpecV1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	widgetTable := s1.TableFor(s1.Objects[0])
	if err := db.Exec(fmt.Sprintf("insert into %s (id, sku) values (?, ?)", widgetTable), "w1", "DUP").Error; err != nil {
		t.Fatalf("insert w1: %v", err)
	}
	if err := db.Exec(fmt.Sprintf("insert into %s (id, sku) values (?, ?)", widgetTable), "w2", "DUP").Error; err != nil {
		t.Fatalf("insert w2 (duplicate sku, no unique constraint declared yet): %v", err)
	}

	s2, err := spec.Parse([]byte(abortSpecV2))
	if err != nil {
		t.Fatalf("parse v2 (widget.sku is now unique; gadget is a brand-new, unrelated object): %v", err)
	}
	gadgetTable := s2.TableFor(s2.Objects[1])

	migrateErr := spec.Migrate(db, s2)
	if migrateErr == nil {
		t.Fatalf("S54 open bug: expected Migrate to report widget's new unique index colliding with its own pre-existing duplicate 'DUP' rows, got nil")
	}

	if !db.Migrator().HasTable(gadgetTable) {
		t.Fatalf("S54 open bug: Migrate on a two-object blueprint failed widget's own new unique index (colliding with widget's pre-existing duplicate data) and never created gadget's table at all, even though gadget is a brand-new object with no relationship to widget or to the failing index — got Migrate error %v, want gadget's table (%s) to exist regardless of widget's own unrelated failure", migrateErr, gadgetTable)
	}
}
