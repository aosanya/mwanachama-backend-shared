package spec_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const taggedThing = `{"module":"record","domain":"d","instance":"i","objects":[{
  "name":"thing","description":"a thing","fields":[
    {"name":"id","type":"string","primary":true,"description":"the id"},
    {"name":"tag","type":"string","description":"a tag, empty when the thing has none"},
    {"name":"deleted","type":"bool","description":"soft-delete marker"}],
  "indexes":[{"name":"tag_uniq","fields":["tag"],"unique":true,"not_deleted":true,
    "where":[{"field":"tag","op":"not_empty"}]}]}]}`

func TestAnIndexNarrowsToRowsThatCarryTheField(t *testing.T) {
	s, err := spec.Parse([]byte(taggedThing))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ddl := strings.Join(s.DDL("sqlite"), "\n")
	if !strings.Contains(ddl, "where deleted = false and tag <> ''") {
		t.Errorf("DDL does not carry both conditions:\n%s", ddl)
	}
}

func TestNotDeletedAloneStillRendersAsItAlwaysDid(t *testing.T) {
	s := load(t, clinicSpec)
	for _, stmt := range s.DDL("sqlite") {
		if strings.Contains(stmt, " where ") && !strings.Contains(stmt, "where deleted = false") {
			t.Errorf("an index grew a condition nobody declared: %s", stmt)
		}
	}
}

func TestAnEmptyValueIsNotIndexedAndADuplicateStillCollides(t *testing.T) {
	s, err := spec.Parse([]byte(taggedThing))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	db := open(t)
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	table := s.TableFor(s.Objects[0])

	insert := func(id, tag string) error {
		return db.Table(table).Create(map[string]any{
			"id": id, "tag": tag, "deleted": false,
		}).Error
	}

	if err := insert("a", ""); err != nil {
		t.Fatalf("first untagged row: %v", err)
	}
	if err := insert("b", ""); err != nil {
		t.Fatalf("a second untagged row collided with the first: %v", err)
	}
	if err := insert("c", "t-1"); err != nil {
		t.Fatalf("first tagged row: %v", err)
	}
	if err := insert("d", "t-1"); err == nil {
		t.Fatal("a duplicate tag was accepted, so the index is not unique")
	}
}

func TestValidate_RefusesAConditionItCannotStand(t *testing.T) {
	object := func(indexes string) string {
		return `{"module":"record","domain":"d","instance":"i","objects":[{
		  "name":"thing","description":"a thing","fields":[
		    {"name":"id","type":"string","primary":true,"description":"the id"},
		    {"name":"count","type":"int","description":"how many"}],
		  "indexes":[` + indexes + `]}]}`
	}
	for _, tc := range []struct{ name, indexes, want string }{
		{"an operator nobody supplies",
			`{"name":"i","fields":["id"],"where":[{"field":"id","op":"starts_with"}]}`,
			"unknown condition"},
		{"a field the object does not declare",
			`{"name":"i","fields":["id"],"where":[{"field":"nope","op":"not_empty"}]}`,
			"conditioned on unknown field"},
		{"emptiness asked of a number",
			`{"name":"i","fields":["id"],"where":[{"field":"count","op":"not_empty"}]}`,
			`applies "not_empty" to "count", which is int`},
		{"truth asked of a string",
			`{"name":"i","fields":["id"],"where":[{"field":"id","op":"is_true"}]}`,
			`applies "is_true" to "id", which is string`},
		{"the same condition twice",
			`{"name":"i","fields":["id"],"where":[{"field":"id","op":"not_empty"},{"field":"id","op":"not_empty"}]}`,
			"twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spec.Parse([]byte(object(tc.indexes)))
			if err == nil {
				t.Fatal("spec loaded, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
