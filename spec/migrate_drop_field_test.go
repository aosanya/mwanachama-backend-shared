package spec_test

import (
	"context"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const dropFieldSpecV1 = `{
  "module": "record",
  "domain": "probe",
  "instance": "probe4",
  "objects": [
    {
      "name": "widget",
      "role": "widget",
      "description": "One thing on a shelf.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "name", "type": "string", "description": "What it is called."},
        {"name": "color", "type": "string", "description": "What color it is.", "required": true}
      ]
    }
  ]
}`

const dropFieldSpecV2 = `{
  "module": "record",
  "domain": "probe",
  "instance": "probe4",
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

type widgetAfterColorDropped struct {
	ID   string
	Name string
}

func TestS53_OpenBug_DroppedRequiredFieldOrphansANotNullColumn(t *testing.T) {
	db := open(t)

	s1, err := spec.Parse([]byte(dropFieldSpecV1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if err := spec.Migrate(db, s1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := s1.TableFor(s1.Objects[0])
	if err := db.Exec("insert into "+table+" (id, name, color) values (?, ?, ?)", "w1", "widget one", "red").Error; err != nil {
		t.Fatalf("seed a row under v1: %v", err)
	}

	s2, err := spec.Parse([]byte(dropFieldSpecV2))
	if err != nil {
		t.Fatalf("parse v2 (color removed): %v", err)
	}
	if err := spec.Migrate(db, s2); err != nil {
		t.Fatalf("migrate v2 on the already-provisioned database: %v", err)
	}

	st, err := specstore.New(db, s2, map[string]any{"widget": widgetAfterColorDropped{}})
	if err != nil {
		t.Fatalf("specstore.New on v2 spec: %v", err)
	}

	err = st.Insert(context.Background(), "widget", &widgetAfterColorDropped{ID: "w2", Name: "widget two"})
	if err != nil {
		t.Fatalf("S53 open bug: inserting a row declared entirely by the widget object's current fields (id, name — color was dropped from the blueprint) failed: %v — Migrate left the old 'color' column in place as 'not null' with no default, so an insert naming only the still-declared columns is rejected by the database for a column the domain no longer declares at all, on any instance migrated before the field was dropped", err)
	}
}
