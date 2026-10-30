package spec_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const adoptTypesSpec = `{
  "module": "comm", "domain": "platform", "instance": "platform",
  "objects": [{
    "name": "message", "description": "One message.",
    "fields": [
      {"name": "id", "type": "string", "description": "Key.", "primary": true},
      {"name": "body", "type": "text", "description": "Body."},
      {"name": "sent_at", "type": "timestamp", "description": "When.", "nullable": true},
      {"name": "size", "type": "int", "description": "Size."},
      {"name": "score", "type": "float", "description": "Score."},
      {"name": "seen", "type": "bool", "description": "Seen."},
      {"name": "raw", "type": "bytes", "description": "Raw."},
      {"name": "doc", "type": "json", "description": "Doc."}
    ]
  }]
}`

func TestS39_PostgresAdoptionNamesEveryColumnWhoseTypeTheEngineCannotRead(t *testing.T) {
	s, err := spec.Parse([]byte(adoptTypesSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	held := map[string]string{
		"id": "character varying", "body": "text",
		"sent_at": "timestamp with time zone",
		"size":    "integer", "score": "numeric", "seen": "boolean",
		"raw": "bytea", "doc": "json",
	}
	problems := spec.AdoptionTypeProblems("platform_message", held, s.Objects[0], "postgres", nil)
	if len(problems) != 1 || !strings.Contains(problems[0], "platform_message.sent_at is timestamp with time zone") {
		t.Fatalf("want exactly the timestamptz column named, got %v", problems)
	}
}

func TestS39_ColumnsRenamedOnAdoptionAreCheckedUnderTheirOldName(t *testing.T) {
	s, err := spec.Parse([]byte(adoptTypesSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	held := map[string]string{"id": "text", "when_sent": "date"}
	problems := spec.AdoptionTypeProblems("t", held, s.Objects[0], "postgres", map[string]string{"when_sent": "sent_at"})
	if len(problems) != 1 || !strings.Contains(problems[0], "t.when_sent is date") {
		t.Fatalf("got %v", problems)
	}
}

func TestS39_TypeFamiliesDoNotConflateLookalikeNames(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"interval", "bigint"},
		{"point", "bigint"},
		{"uuid", "text"},
		{"timestamp without time zone", "text"},
		{"date", "text"},
	} {
		if spec.TypeFamily(c.a) == spec.TypeFamily(c.b) {
			t.Errorf("%q and %q must not be one family", c.a, c.b)
		}
	}
	for _, c := range []struct{ a, b string }{
		{"integer", "bigint"}, {"character varying", "text"}, {"jsonb", "json"}, {"numeric", "double precision"},
	} {
		if spec.TypeFamily(c.a) != spec.TypeFamily(c.b) {
			t.Errorf("%q and %q should be one family", c.a, c.b)
		}
	}
}

func TestS39_SQLiteAdoptionIsUnchangedByDifferingColumnTypes(t *testing.T) {
	db := namedLegacyDB(t)
	s, err := spec.Parse([]byte(adoptTypesSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	legacy := spec.LegacyTables(s, s.Objects[0])[0]
	if err := db.Exec(`create table ` + legacy + ` (id varchar(36) primary key, body clob, sent_at datetime, size integer, score real, seen numeric, raw blob, doc text)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := spec.AdoptLegacy(db, s, spec.Legacy{}); err != nil {
		t.Fatalf("SQLite is dynamically typed and adoption must not refuse on it: %v", err)
	}
	if !db.Migrator().HasTable(s.TableFor(s.Objects[0])) {
		t.Fatal("the legacy table was not adopted")
	}
}
