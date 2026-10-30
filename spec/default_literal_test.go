package spec_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestNumericDefaultsAreEmittedAsTheParsedValue(t *testing.T) {
	cases := []struct{ typ, def, want string }{
		{"int", "+7", "n bigint default 7"},
		{"int", "007", "n bigint default 7"},
		{"float", "1e3", "n double precision default 1000"},
		{"float", "-.5", "n double precision default -0.5"},
		{"bool", "1", "n boolean default true"},
		{"bool", "F", "n boolean default false"},
	}
	for _, c := range cases {
		s, err := spec.Parse(numericDefaultSpec(c.typ, c.def))
		if err != nil {
			t.Fatalf("%s default %q should load: %v", c.typ, c.def, err)
		}
		ddl := strings.Join(s.DDL("postgres"), "\n")
		if !strings.Contains(ddl, c.want) {
			t.Errorf("%s default %q: want %q in the DDL, got\n%s", c.typ, c.def, c.want, ddl)
		}
	}
}

func TestNonFiniteAndNonNumericDefaultsAreRefusedAtLoad(t *testing.T) {
	for _, h := range []struct{ typ, def string }{
		{"float", "NaN"},
		{"float", "Inf"},
		{"float", "-Infinity"},
		{"int", "1.5"},
		{"int", "99999999999999999999"},
		{"int", " 7"},
		{"bool", "yes"},
	} {
		if _, err := spec.Parse(numericDefaultSpec(h.typ, h.def)); err == nil {
			t.Errorf("a %s field defaulting to %q should be refused at load", h.typ, h.def)
		}
	}
}

func TestMigrateRefusesAHostileDefaultOnASpecThatSkippedLoad(t *testing.T) {
	s, err := spec.Parse(numericDefaultSpec("int", "7"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s.Objects[0].Fields[1].Default = "0) ; create table s55_planted (x int); --"

	db := open(t)
	if err := spec.Migrate(db, s); err == nil {
		t.Fatal("Migrate should refuse a numeric default that is not a number")
	}
	var planted int64
	db.Raw(`select count(*) from sqlite_master where name = 's55_planted'`).Scan(&planted)
	if planted != 0 {
		t.Fatalf("the stacked statement ran: %d planted tables", planted)
	}
	for _, stmt := range s.DDL("sqlite") {
		if strings.Contains(stmt, "s55_planted (x int)") && !strings.Contains(stmt, "'0) ; create table s55_planted (x int); --'") {
			t.Fatalf("DDL carries the hostile default unquoted: %s", stmt)
		}
	}
}
