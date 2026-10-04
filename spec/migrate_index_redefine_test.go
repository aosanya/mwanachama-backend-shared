package spec_test

import (
	"fmt"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const indexRedefineSpecV1 = `{
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
        {"name": "sku", "type": "string", "description": "Catalog number."}
      ],
      "indexes": [
        {"name": "sku", "fields": ["sku"], "unique": false}
      ]
    }
  ]
}`

const indexRedefineSpecV2 = `{
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
        {"name": "sku", "type": "string", "description": "Catalog number."}
      ],
      "indexes": [
        {"name": "sku", "fields": ["sku"], "unique": true}
      ]
    }
  ]
}`

func TestS54_OpenBug_SecondMigrateDoesNotEnforceARedeclaredUniqueIndex(t *testing.T) {
	db := open(t)

	s1, err := spec.Parse([]byte(indexRedefineSpecV1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := s1.TableFor(s1.Objects[0])

	s2, err := spec.Parse([]byte(indexRedefineSpecV2))
	if err != nil {
		t.Fatalf("parse v2 (same index name 'sku', now unique): %v", err)
	}
	if err := spec.Migrate(db, s2); err != nil {
		t.Fatalf("migrate v2 on the already-provisioned database: %v", err)
	}

	if err := db.Exec(fmt.Sprintf("insert into %s (id, sku) values (?, ?)", table), "w1", "SKU-1").Error; err != nil {
		t.Fatalf("insert w1: %v", err)
	}
	err2 := db.Exec(fmt.Sprintf("insert into %s (id, sku) values (?, ?)", table), "w2", "SKU-1").Error
	if err2 == nil {
		t.Fatalf("S54 open bug: after a second Migrate redeclaring index 'sku' as unique:true, inserting a second row (w2) with the same sku (SKU-1) as the first (w1) succeeded with no error — createIndexes (spec/migrate.go) emits \"create unique index if not exists %s_sku_idx\", which is a no-op once an index of that name already exists, so the blueprint's new uniqueness constraint is silently never applied to an already-provisioned table, unlike S52/S53 this leaves no error at all: the duplicate row the domain declared impossible is stored and read back as ordinary data", table)
	}
}
