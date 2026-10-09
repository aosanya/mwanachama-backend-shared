package spec_test

import (
	"strconv"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func numericDefaultSpec(fieldType, def string) []byte {
	return []byte(`{"instance":"dflt","module":"m","objects":[{"name":"thing","description":"d","fields":[` +
		`{"name":"id","type":"string","description":"d","primary":true},` +
		`{"name":"n","type":"` + fieldType + `","description":"d","default":` + strconv.Quote(def) + `}]}]}`)
}

func TestS55_OpenHole_NumericDefaultReachesTheDDLUnquoted(t *testing.T) {
	for _, ok := range []struct{ typ, def string }{{"int", "7"}, {"float", "1.5"}, {"bool", "true"}} {
		if _, err := spec.Parse(numericDefaultSpec(ok.typ, ok.def)); err != nil {
			t.Fatalf("precondition: a %s field defaulting to %q should load, got %v", ok.typ, ok.def, err)
		}
	}

	hostile := []struct{ typ, def string }{
		{"int", "0) ; create table s55_planted (x int); --"},
		{"int", "1 check (1=1)"},
		{"int", "(select 7)"},
		{"float", "abc"},
		{"bool", "1); drop table anything; --"},
	}
	for _, h := range hostile {
		s, err := spec.Parse(numericDefaultSpec(h.typ, h.def))
		if err == nil {
			db := open(t)
			migrateErr := spec.Migrate(db, s)
			var planted int64
			db.Raw(`select count(*) from sqlite_master where name = 's55_planted'`).Scan(&planted)
			t.Errorf("S55 (P3): a %s field defaulting to %q loaded and reached the DDL as raw SQL (migrate err %v, planted tables %d); want spec.Parse to refuse a %s default that is not a %s literal",
				h.typ, h.def, migrateErr, planted, h.typ, h.typ)
		}
	}
}
