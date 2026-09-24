package spec_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const clinicSpec = "testdata/clinic.record.json"

const garageSpec = "testdata/garage.record.json"

var fixtures = []string{clinicSpec, garageSpec}

func open(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func blueprint(t *testing.T) *spec.Blueprint {
	t.Helper()
	b, err := spec.LoadBlueprint(filepath.Join("testdata", "record.blueprint.json"))
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}
	return b
}

func load(t *testing.T, path string) *spec.Spec {
	t.Helper()
	s, err := blueprint(t).Load(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	return s
}

func tables(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var names []string
	if err := db.Raw(`select name from sqlite_master where type='table' and name not like 'sqlite_%'`).
		Scan(&names).Error; err != nil {
		t.Fatalf("list tables: %v", err)
	}
	sort.Strings(names)
	return names
}

// Two domains, one module, one database: the whole point of declaring
// objects rather than writing them. Neither spec changes a line of Go.
func TestTwoDomainsCoexist(t *testing.T) {
	db := open(t)

	clinic := load(t, clinicSpec)
	garage := load(t, garageSpec)

	if err := spec.Migrate(db, clinic); err != nil {
		t.Fatalf("migrate clinic: %v", err)
	}
	if err := spec.Migrate(db, garage); err != nil {
		t.Fatalf("migrate garage: %v", err)
	}

	got := tables(t, db)
	// <instance>_<module>_<table>. Both domains declare a page_view object,
	// and it is the instance segment that keeps them apart; the module
	// segment keeps all of them clear of whatever other module is mounted in
	// the same database.
	want := []string{
		"clinic_record_page_views", "clinic_record_patients", "clinic_record_visits",
		"garage_record_page_views", "garage_record_parts", "garage_record_vehicles",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("tables =\n  %v\nwant\n  %v", got, want)
	}

	// Each domain calls the object by its own noun; the module finds it by
	// role. This is the join that lets one rule serve both.
	for _, tc := range []struct {
		s         *spec.Spec
		wantName  string
		wantTable string
	}{
		{clinic, "patient", "clinic_record_patients"},
		{garage, "vehicle", "garage_record_vehicles"},
	} {
		o, ok := tc.s.ByRole("entry")
		if !ok {
			t.Fatalf("%s: no object fills the entry role", tc.s.Domain)
		}
		if o.Name != tc.wantName {
			t.Errorf("%s: entry is called %q, want %q", tc.s.Domain, o.Name, tc.wantName)
		}
		if tc.s.TableFor(o) != tc.wantTable {
			t.Errorf("%s: entry table = %q, want %q", tc.s.Domain, tc.s.TableFor(o), tc.wantTable)
		}
	}

	// An object with no role is the domain's own, and no rule touches it.
	p, ok := garage.Object("part")
	if !ok {
		t.Fatal("the garage declares a part object the clinic does not")
	}
	if p.Role != "" {
		t.Errorf("part has role %q, want none — it is the garage's own", p.Role)
	}
	if _, ok := clinic.Object("part"); ok {
		t.Error("the clinic should not have a part object")
	}
}

// A domain may take a default the module declared, and the module's own
// values still hold.
func TestADomainsDefaultReachesTheDDL(t *testing.T) {
	garage := load(t, garageSpec)
	ddl := strings.Join(garage.DDL("sqlite"), "\n")
	if !strings.Contains(ddl, "state text default 'open'") {
		t.Errorf("DDL does not carry the domain's default:\n%s", ddl)
	}
	clinic := load(t, clinicSpec)
	if !strings.Contains(strings.Join(clinic.DDL("sqlite"), "\n"), "state text default 'pending'") {
		t.Error("the clinic should keep the module's own default")
	}
}

// The module states the roles its rules need. A domain that fills none of
// them should stop the process starting, not fail a request later.
func TestRequireRoles(t *testing.T) {
	needed := []string{"entry", "page_view"}

	for _, path := range fixtures {
		s := load(t, path)
		if err := s.RequireRoles(needed...); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}

	thin := load(t, clinicSpec)
	err := thin.RequireRoles(append(needed, "loan", "reservation")...)
	if err == nil {
		t.Fatal("a missing role was accepted")
	}
	for _, want := range []string{"loan", "reservation"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to name %q", err, want)
		}
	}
}

// Two objects claiming one role would leave a rule with no way to know
// which of them it operates on.
func TestValidate_RefusesADuplicateRole(t *testing.T) {
	body := `{"module":"record","domain":"d","instance":"i","objects":[
	  {"name":"a","role":"entry","fields":[{"name":"x","type":"string","primary":true}]},
	  {"name":"b","role":"entry","fields":[{"name":"x","type":"string","primary":true}]}]}`
	_, err := spec.Parse([]byte(body))
	if err == nil {
		t.Fatal("two objects claimed the entry role and the spec loaded")
	}
	if !strings.Contains(err.Error(), "both claim the role") {
		t.Errorf("err = %v", err)
	}
}

// Migrating twice must be a no-op, because a process runs it at every start.
func TestMigrateIsIdempotent(t *testing.T) {
	db := open(t)
	s := load(t, clinicSpec)

	for i := 0; i < 3; i++ {
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}
	if n := len(tables(t, db)); n != 3 {
		t.Errorf("tables = %d, want 3", n)
	}
}

// A domain's own vocabulary becomes an index without the module learning the
// word: the clinic indexes a blood pressure, the garage a chassis number.
func TestIndexesComeFromTheDomain(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{clinicSpec, "json_extract(doc, '$.vitals.blood_pressure')"},
		{garageSpec, "json_extract(doc, '$.identifiers.vin')"},
	} {
		s := load(t, tc.path)
		ddl := strings.Join(s.DDL("sqlite"), "\n")
		if !strings.Contains(ddl, tc.want) {
			t.Errorf("%s: DDL does not index %s", tc.path, tc.want)
		}
	}

	// And the same declaration is jsonb on Postgres.
	s := load(t, clinicSpec)
	pg := strings.Join(s.DDL("postgres"), "\n")
	if !strings.Contains(pg, `doc #>> '{vitals,blood_pressure}'`) {
		t.Error("postgres DDL does not use the jsonb path operator")
	}
	if !strings.Contains(pg, "doc jsonb") {
		t.Error("postgres DDL does not declare doc as jsonb")
	}
	if strings.Contains(strings.Join(s.DDL("sqlite"), "\n"), "jsonb") {
		t.Error("sqlite DDL should not mention jsonb")
	}
}

// A spec is refused when it loads, not when a request arrives.
func TestValidate_RefusesABadSpec(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"no primary key",
			`{"module":"record","domain":"d","instance":"i","objects":[{"name":"a","fields":[{"name":"x","type":"string"}]}]}`,
			"no primary key"},
		{"unknown type",
			`{"module":"record","domain":"d","instance":"i","objects":[{"name":"a","fields":[{"name":"x","type":"blob","primary":true}]}]}`,
			"unknown type"},
		{"enum with no values",
			`{"module":"record","domain":"d","instance":"i","objects":[{"name":"a","fields":[{"name":"x","type":"enum","primary":true}]}]}`,
			"declares no values"},
		{"duplicate table",
			`{"module":"record","domain":"d","instance":"i","objects":[
			  {"name":"a","table":"t","fields":[{"name":"x","type":"string","primary":true}]},
			  {"name":"b","table":"t","fields":[{"name":"x","type":"string","primary":true}]}]}`,
			"more than one object"},
		{"index on an unknown field",
			`{"module":"record","domain":"d","instance":"i","objects":[{"name":"a","fields":[{"name":"x","type":"string","primary":true}],
			  "indexes":[{"name":"i","fields":["nope"]}]}]}`,
			"unknown field"},
		{"not_deleted without a deleted field",
			`{"module":"record","domain":"d","instance":"i","objects":[{"name":"a","fields":[{"name":"x","type":"string","primary":true}],
			  "indexes":[{"name":"i","fields":["x"],"not_deleted":true}]}]}`,
			"no deleted field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spec.Parse([]byte(tc.body))
			if err == nil {
				t.Fatal("spec loaded, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// Names reach the database as SQL text, so a name that could close an
// identifier and continue the statement must be refused outright.
func TestValidate_RefusesNamesThatAreNotNames(t *testing.T) {
	for _, bad := range []string{
		`a"; drop table x; --`,
		"a b",
		"A",
		"_a",
		"a__b",
		"a-",
		"",
		"1a",
	} {
		t.Run(bad, func(t *testing.T) {
			body := `{"module":"record","domain":"d","instance":"i","objects":[{"name":"` + bad +
				`","fields":[{"name":"x","type":"string","primary":true}]}]}`
			if _, err := spec.Parse([]byte(body)); err == nil {
				t.Fatalf("object name %q was accepted", bad)
			}
		})
	}
}

// Postgres truncates an identifier at 63 bytes and says nothing, so two
// names that agree that far are one relation. Both of these pass every other
// check in the spec: the names are well formed and distinct, and the
// difference between them is simply past the cut.
//
// The instance below is a real published slug, de-hyphenated
// (agricultural-commodity-exchange-warehouse-receipt-system, 51 characters),
// which is what makes this a live risk rather than a contrived one.
func TestValidate_RefusesIdentifiersTheDatabaseWouldMerge(t *testing.T) {
	const long = "agriculturalcommodityexchangewarehousereceiptsystem"

	t.Run("two tables", func(t *testing.T) {
		body := `{"module":"record","domain":"d","instance":"` + long + `","objects":[
		  {"name":"a","table":"entries","description":"one","fields":[{"name":"x","type":"string","description":"k","primary":true}]},
		  {"name":"b","table":"entry_privates","description":"two","fields":[{"name":"x","type":"string","description":"k","primary":true}]}]}`
		err := parseErr(t, body)
		// Both halves are worth stating: that it is too long, and that the
		// two of them land on one name.
		for _, want := range []string{"truncates at 63", "both become"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("err = %v, want it to mention %q", err, want)
			}
		}
	})

	// An index name is the table name plus more, so it overflows first: a
	// spec whose tables all fit can still lose an index. A lost index is the
	// quiet one — "create index if not exists" makes the second a no-op, and
	// the query that reads as indexed simply is not.
	t.Run("an index before its table", func(t *testing.T) {
		body := `{"module":"record","domain":"d","instance":"` + long + `","objects":[
		  {"name":"a","table":"t","description":"one","fields":[
		    {"name":"x","type":"string","description":"k","primary":true},
		    {"name":"y","type":"string","description":"a field"}],
		   "indexes":[
		     {"name":"status_and_priority","fields":["x"]},
		     {"name":"status_and_position","fields":["y"]}]}]}`
		err := parseErr(t, body)
		if !strings.Contains(err.Error(), "index") {
			t.Errorf("err = %v, want it to name the index", err)
		}
	})
}

// The instance and module segments give up the underscore, because a
// physical name is <instance>_<module>_<table> and a separator that is legal
// inside a segment cannot be read back apart.
func TestValidate_RefusesAnUnderscoreInASegment(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"instance", `{"module":"record","domain":"d","instance":"my_shop","objects":[
		  {"name":"a","description":"one","fields":[{"name":"x","type":"string","description":"k","primary":true}]}]}`},
		{"module", `{"module":"my_record","domain":"d","instance":"shop","objects":[
		  {"name":"a","description":"one","fields":[{"name":"x","type":"string","description":"k","primary":true}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := parseErr(t, tc.body)
			if !strings.Contains(err.Error(), "usable name segment") {
				t.Errorf("err = %v, want it to refuse the segment", err)
			}
		})
	}
}

func parseErr(t *testing.T, body string) error {
	t.Helper()
	_, err := spec.Parse([]byte(body))
	if err == nil {
		t.Fatal("spec loaded, want a refusal")
	}
	return err
}

// An unknown key is a typo, and a typo that is silently ignored is a field
// nobody notices is missing.
func TestParse_RefusesUnknownKeys(t *testing.T) {
	body := `{"module":"record","domain":"d","instance":"i","objekts":[]}`
	if _, err := spec.Parse([]byte(body)); err == nil {
		t.Fatal("unknown key accepted")
	}
}

// A composite key is declared by marking several fields primary.
func TestCompositePrimaryKey(t *testing.T) {
	s := load(t, clinicSpec)
	ddl := strings.Join(s.DDL("sqlite"), "\n")
	if !strings.Contains(ddl, "primary key (page, slug)") {
		t.Error("page_view should have a composite key")
	}
}

// A required field must not carry a default: the default is exactly what
// would stop an omitted value being noticed.
func TestRequiredFieldsHaveNoDefault(t *testing.T) {
	for _, path := range fixtures {
		s := load(t, path)
		for _, stmt := range s.DDL("postgres") {
			for _, line := range strings.Split(stmt, "\n") {
				line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
				if strings.Contains(line, "not null") && strings.Contains(line, "default") {
					t.Errorf("%s: column is both required and defaulted: %q", path, line)
				}
			}
		}
	}
}

// A declared object or field nobody can explain is the one that rots, and
// the spec is the only place the explanation would live.
func TestValidate_RequiresDescriptions(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"object without one",
			`{"module":"record","domain":"d","instance":"i","objects":[
			  {"name":"a","table":"a","fields":[{"name":"x","type":"string","description":"d","primary":true}]}]}`,
			`object "a" has no description`},
		{"field without one",
			`{"module":"record","domain":"d","instance":"i","objects":[
			  {"name":"a","description":"d","table":"a","fields":[{"name":"x","type":"string","primary":true}]}]}`,
			`field "x" has no description`},
		{"a blank one does not count",
			`{"module":"record","domain":"d","instance":"i","objects":[
			  {"name":"a","description":"   ","table":"a","fields":[{"name":"x","type":"string","description":"d","primary":true}]}]}`,
			`object "a" has no description`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spec.Parse([]byte(tc.body))
			if err == nil {
				t.Fatal("spec loaded, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// Every object and field in the shipped fixtures says something, rather
// than restating its own name.
//
// The floor is a sentence — three words — not a character count: "Who
// issued it." is a complete description and a length threshold would reject
// it while passing twenty characters of padding.
func TestFixturesAreDescribed(t *testing.T) {
	sentence := func(s string) bool { return len(strings.Fields(s)) >= 3 }
	restates := func(desc, name string) bool {
		d := strings.ToLower(strings.Trim(desc, " ."))
		return d == strings.ReplaceAll(name, "_", " ") || d == name
	}

	for _, path := range fixtures {
		s := load(t, path)
		for _, o := range s.Objects {
			if !sentence(o.Description) {
				t.Errorf("%s: object %q is not described in a sentence: %q", path, o.Name, o.Description)
			}
			if restates(o.Description, o.Name) {
				t.Errorf("%s: object %q only restates its name", path, o.Name)
			}
			for _, f := range o.Fields {
				if !sentence(f.Description) {
					t.Errorf("%s: %s.%s is not described in a sentence: %q",
						path, o.Name, f.Name, f.Description)
				}
				if restates(f.Description, f.Name) {
					t.Errorf("%s: %s.%s only restates its name", path, o.Name, f.Name)
				}
			}
		}
	}
}

// Postgres requires an expression index's expression in its own
// parentheses: `on t ((expr))`. A single pair is a syntax error there and
// accepted by SQLite, so only an explicit assertion catches it — this test
// exists because the real database rejected the generated DDL while every
// SQLite test passed.
func TestExpressionIndexesAreDoubleParenthesised(t *testing.T) {
	for _, path := range fixtures {
		s := load(t, path)
		for _, dialect := range []string{"postgres", "sqlite"} {
			for _, stmt := range s.DDL(dialect) {
				if !strings.Contains(stmt, "create index") && !strings.Contains(stmt, "create unique index") {
					continue
				}
				// An expression index is one whose target is not a bare
				// column list; both dialects spell the expression with a
				// function or operator in it.
				isExpr := strings.Contains(stmt, "#>>") || strings.Contains(stmt, "json_extract")
				if !isExpr {
					continue
				}
				if !strings.Contains(stmt, "((") {
					t.Errorf("%s/%s: expression index is not double-parenthesised:\n  %s", path, dialect, stmt)
				}
			}
		}
	}
}
