package spec_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestS23_OpenBug_MigrateRefusesAnInvalidSpec(t *testing.T) {
	s := &spec.Spec{
		Module:   "inventory",
		Domain:   "warehouse",
		Instance: "acme",
		Objects: []spec.Object{
			{
				Name:        "bin",
				Table:       "bins",
				Description: "A storage bin.",
				Fields: []spec.Field{
					{Name: "id", Type: spec.TypeString, Description: "key", Primary: true},
					{Name: "capacity", Type: spec.TypeInt, Description: "how much it holds"},
				},
			},
			{
				Name:        "pallet",
				Table:       "bins",
				Description: "A pallet, declared against the SAME table name as bin above.",
				Fields: []spec.Field{
					{Name: "id", Type: spec.TypeString, Description: "key", Primary: true},
					{Name: "weight", Type: spec.TypeInt, Description: "how heavy it is"},
				},
			},
		},
	}

	if verr := s.Validate(); verr == nil {
		t.Fatalf("test setup broken: s.Validate() should refuse two objects sharing one table, got nil")
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	err = spec.Migrate(db, s)
	if err == nil {
		var cols []struct{ Name string }
		db.Raw("PRAGMA table_info(acme_inventory_bins)").Scan(&cols)
		t.Errorf("S23: spec.Migrate(db, s) returned nil for a spec s.Validate() itself refuses "+
			"(object %q and object %q both declare table %q). Got: Migrate ran DDL anyway and "+
			"acme_inventory_bins now has whichever object's columns migrated first — a caller of "+
			"the other object silently writes into the wrong table with no error at any point. "+
			"Want: Migrate refuses the same way Validate does, the same defense dispatch.Dispatch, "+
			"dispatch.Parse and dispatch.Tools already apply to their own spec before using it.",
			"bin", "pallet", "bins")
	}
}
