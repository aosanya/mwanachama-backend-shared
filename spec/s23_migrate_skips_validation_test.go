package spec_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

// S23: Migrate builds DDL straight from a *Spec's Object/Field names, and
// never calls Validate itself. Validate is the only place NamePattern is
// enforced, so a *Spec obtained by any means other than spec.Load/Parse —
// a hand-built struct, or one decoded from storage without going back
// through Parse — reaches Migrate with zero defense against an identifier
// that is not an identifier at all. Here a single field's Name alone
// injects a second, undeclared column into the generated CREATE TABLE.
func TestS23_MigrateBuildsDDLFromAnUnvalidatedSpec(t *testing.T) {
	db := open(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	s := &spec.Spec{
		Module:   "m",
		Domain:   "d",
		Instance: "i",
		Objects: []spec.Object{
			{
				Name:        "widget",
				Description: "widget",
				Table:       "widgets",
				Fields: []spec.Field{
					{Name: "id", Type: spec.TypeString, Primary: true, Description: "id"},
					{Name: "a text default 'x', injected", Type: spec.TypeText, Description: "malicious"},
				},
			},
		},
	}

	if err := s.Validate(); err == nil {
		t.Fatal("expected the crafted spec to fail Validate (the field name is not NamePattern)")
	}

	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("Migrate on an unvalidated spec refused the crafted DDL: %v", err)
	}

	rows, err := db.Raw("select sql from sqlite_master where type = 'table' and name = 'i_m_widgets'").Rows()
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("table i_m_widgets was not created")
	}
	var ddl string
	if err := rows.Scan(&ddl); err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	t.Logf("actual DDL Migrate ran with no error, from one field name alone: %s", ddl)

	if err := db.Exec("insert into i_m_widgets (id, a, injected) values ('1','x','snuck-in')").Error; err != nil {
		t.Fatalf("the injected column is not usable: %v", err)
	}
}
