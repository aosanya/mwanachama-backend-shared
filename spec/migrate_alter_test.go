package spec_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

type sqlLog struct {
	mu    sync.Mutex
	stmts []string
}

func recordSQL(db *gorm.DB) *sqlLog {
	l := &sqlLog{}
	db.Callback().Raw().Before("gorm:raw").Register("test:record_sql", func(tx *gorm.DB) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.stmts = append(l.stmts, tx.Statement.SQL.String())
	})
	return l
}

func (l *sqlLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stmts = nil
}

func (l *sqlLog) starting(prefix string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, s := range l.stmts {
		if strings.HasPrefix(strings.TrimSpace(s), prefix) && !strings.Contains(s, spec.TestSuffix+" ") {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func declField(name, typ, flags string) string {
	if flags != "" {
		flags = ", " + flags
	}
	return fmt.Sprintf(`{"name": %q, "type": %q, "description": "d"%s}`, name, typ, flags)
}

func declObject(name string, indexes string, fields ...string) string {
	if indexes != "" {
		indexes = `, "indexes": [` + indexes + `]`
	}
	return fmt.Sprintf(`{"name": %q, "description": "d", "fields": [%s]%s}`,
		name, strings.Join(fields, ", "), indexes)
}

func declSpec(t *testing.T, instance string, objects ...string) *spec.Spec {
	t.Helper()
	raw := fmt.Sprintf(`{"module": "record", "domain": "probe", "instance": %q, "objects": [%s]}`,
		instance, strings.Join(objects, ", "))
	s, err := spec.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, raw)
	}
	return s
}

func columnsOf(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var names []string
	if err := db.Raw(`select name from pragma_table_info(?)`, table).Scan(&names).Error; err != nil {
		t.Fatalf("columns of %s: %v", table, err)
	}
	sort.Strings(names)
	return names
}

func widgetV1() string {
	return declObject("widget", "",
		declField("id", "string", `"primary": true`),
		declField("alpha", "string", ""),
		declField("beta", "string", ""),
	)
}

func widgetV2() string {
	return declObject("widget", "",
		declField("id", "string", `"primary": true`),
		declField("alpha", "string", ""),
		declField("beta", "string", ""),
		declField("gamma", "text", ""),
		declField("delta", "bool", `"required": true`),
		declField("epsilon", "int", ""),
	)
}

func gadgetV1() string {
	return declObject("gadget", "",
		declField("id", "string", `"primary": true`),
		declField("label", "string", ""),
	)
}

func TestMigrateAddsTheColumnsADeclaredObjectGainedAndKeepsItsRows(t *testing.T) {
	db := open(t)
	log := recordSQL(db)

	v1 := declSpec(t, "evo1", widgetV1())
	if err := spec.Migrate(db, v1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := v1.TableFor(v1.Objects[0])
	if err := db.Exec("insert into " + table + " (id, alpha, beta) values ('r1', 'a1', 'b1')").Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	v2 := declSpec(t, "evo1", widgetV2())
	log.reset()
	if err := spec.Migrate(db, v2); err != nil {
		t.Fatalf("migrate v2: %v", err)
	}

	want := []string{
		"alter table " + table + " add column gamma text default ''",
		"alter table " + table + " add column delta boolean not null default false",
		"alter table " + table + " add column epsilon bigint default 0",
	}
	if got := log.starting("alter table"); !reflect.DeepEqual(got, want) {
		t.Fatalf("alters on the first evolving Migrate:\n got  %q\n want %q", got, want)
	}

	var alpha, beta, gamma string
	var delta bool
	var epsilon int
	row := db.Raw("select alpha, beta, gamma, delta, epsilon from " + table + " where id = 'r1'").Row()
	if err := row.Scan(&alpha, &beta, &gamma, &delta, &epsilon); err != nil {
		t.Fatalf("read the existing row: %v", err)
	}
	if alpha != "a1" || beta != "b1" || gamma != "" || delta || epsilon != 0 {
		t.Fatalf("existing row after evolving: alpha=%q beta=%q gamma=%q delta=%v epsilon=%d, want a1 b1 '' false 0",
			alpha, beta, gamma, delta, epsilon)
	}

	before := columnsOf(t, db, table)
	log.reset()
	if err := spec.Migrate(db, v2); err != nil {
		t.Fatalf("second migrate v2: %v", err)
	}
	if got := log.starting("alter table"); len(got) != 0 {
		t.Fatalf("a second Migrate altered again: %q", got)
	}
	if after := columnsOf(t, db, table); !reflect.DeepEqual(before, after) {
		t.Fatalf("second Migrate changed the columns: %v then %v", before, after)
	}
}

func TestMigrateIssuesNoAlterWhenNothingIsNew(t *testing.T) {
	db := open(t)
	log := recordSQL(db)

	s := declSpec(t, "evo2", widgetV2(), gadgetV1())
	for run := 1; run <= 2; run++ {
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("migrate run %d: %v", run, err)
		}
		if got := log.starting("alter table"); len(got) != 0 {
			t.Fatalf("run %d issued alters on a spec with nothing new: %q", run, got)
		}
	}
}

func TestMigrateNeverAddsAPrimaryKeyColumnAndLeavesItToTheModule(t *testing.T) {
	db := open(t)

	s := declSpec(t, "evo3", widgetV2())
	table := s.TableFor(s.Objects[0])
	if err := db.Exec("create table " + table + " (alpha text, beta text)").Error; err != nil {
		t.Fatalf("hand-made table: %v", err)
	}

	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("a table from before the key column existed is the module's to repair, so Migrate must not refuse it: %v", err)
	}
	want := []string{"alpha", "beta", "delta", "epsilon", "gamma"}
	if cols := columnsOf(t, db, table); !reflect.DeepEqual(cols, want) {
		t.Fatalf("columns after Migrate: %v, want the ordinary ones added and no id: %v", cols, want)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestMigrateLeavesAMissingCompositeKeyColumnToTheModule(t *testing.T) {
	db := open(t)

	s := declSpec(t, "evo3b", declObject("link", "",
		declField("left", "string", `"primary": true`),
		declField("right", "string", `"primary": true`),
	))
	table := s.TableFor(s.Objects[0])
	if err := db.Exec("create table " + table + " (left text not null)").Error; err != nil {
		t.Fatalf("hand-made table: %v", err)
	}

	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if cols := columnsOf(t, db, table); !reflect.DeepEqual(cols, []string{"left"}) {
		t.Fatalf("Migrate added a key column: %v", cols)
	}
}

func TestMigrateAltersOnlyTheTableThatGainedAField(t *testing.T) {
	db := open(t)
	log := recordSQL(db)

	v1 := declSpec(t, "evo4", widgetV1(), gadgetV1())
	if err := spec.Migrate(db, v1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	widget, gadget := v1.TableFor(v1.Objects[0]), v1.TableFor(v1.Objects[1])
	gadgetBefore := columnsOf(t, db, gadget)

	v2 := declSpec(t, "evo4", widgetV2(), gadgetV1())
	log.reset()
	if err := spec.Migrate(db, v2); err != nil {
		t.Fatalf("migrate v2: %v", err)
	}

	alters := log.starting("alter table")
	if len(alters) != 3 {
		t.Fatalf("want the 3 alters of the widget, got %q", alters)
	}
	for _, stmt := range alters {
		if !strings.HasPrefix(stmt, "alter table "+widget+" ") {
			t.Errorf("an alter touched a table that gained nothing: %s", stmt)
		}
	}
	if got := columnsOf(t, db, gadget); !reflect.DeepEqual(got, gadgetBefore) {
		t.Errorf("the gadget table changed: %v then %v", gadgetBefore, got)
	}
}

func TestMigrateIndexesAFieldThatArrivedAfterItsTable(t *testing.T) {
	db := open(t)

	v1 := declSpec(t, "evo5", widgetV1())
	if err := spec.Migrate(db, v1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}

	v2 := declSpec(t, "evo5", declObject("widget", `{"name": "by_color", "fields": ["color"]}`,
		declField("id", "string", `"primary": true`),
		declField("alpha", "string", ""),
		declField("beta", "string", ""),
		declField("color", "string", ""),
	))
	if err := spec.Migrate(db, v2); err != nil {
		t.Fatalf("migrate v2 with an index on the new field: %v", err)
	}
	table := v2.TableFor(v2.Objects[0])
	if !db.Migrator().HasIndex(table, v2.IndexFor(v2.Objects[0], v2.Objects[0].Indexes[0])) {
		t.Fatalf("the index on the field that arrived later was not created")
	}
}

func TestMigrateAddsAUniqueFieldAsAUniqueIndex(t *testing.T) {
	db := open(t)

	v1 := declSpec(t, "evo6", widgetV1())
	if err := spec.Migrate(db, v1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := v1.TableFor(v1.Objects[0])

	v2 := declSpec(t, "evo6", declObject("widget", "",
		declField("id", "string", `"primary": true`),
		declField("alpha", "string", ""),
		declField("beta", "string", ""),
		declField("code", "string", `"unique": true, "nullable": true`),
	))
	if err := spec.Migrate(db, v2); err != nil {
		t.Fatalf("migrate v2 with a unique field: %v", err)
	}
	if err := db.Exec("insert into " + table + " (id, code) values ('a', 'X')").Error; err != nil {
		t.Fatalf("first row: %v", err)
	}
	if err := db.Exec("insert into " + table + " (id, code) values ('b', 'X')").Error; err == nil {
		t.Fatalf("a unique field added later accepted a duplicate")
	}
}

func TestMigrateRollsBackTheAltersOfATableWhenOneFails(t *testing.T) {
	db := open(t)

	v1 := declSpec(t, "evo7", widgetV1())
	if err := spec.Migrate(db, v1); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	table := v1.TableFor(v1.Objects[0])
	for _, id := range []string{"a", "b"} {
		if err := db.Exec("insert into "+table+" (id, alpha) values (?, '')", id).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	v2 := declSpec(t, "evo7", declObject("widget", "",
		declField("id", "string", `"primary": true`),
		declField("alpha", "string", ""),
		declField("beta", "string", ""),
		declField("gamma", "string", ""),
		declField("code", "string", `"unique": true`),
	))
	if err := spec.Migrate(db, v2); err == nil {
		t.Fatalf("a unique, non-null field added to a table with two rows cannot hold, yet Migrate succeeded")
	}
	if cols := columnsOf(t, db, table); !reflect.DeepEqual(cols, []string{"alpha", "beta", "id"}) {
		t.Fatalf("a failed evolution left columns behind: %v", cols)
	}
}

func TestAlterDDLNamesTheStatementsForAPostgresAccountThatGainedFiveFields(t *testing.T) {
	account := declObject("account", "",
		declField("id", "string", `"primary": true`),
		declField("agency_id", "string", `"required": true, "immutable": true`),
		declField("name", "string", `"required": true`),
		declField("key_set", "bool", ""),
		declField("last_test_result", "string", ""),
		declField("last_tested_at", "string", ""),
		declField("last_test_prompt", "string", ""),
		declField("last_test_reply", "string", ""),
		declField("last_test_ms", "int", ""),
		declField("chat_enabled", "bool", ""),
		declField("chat_label", "string", ""),
	)
	s := declSpec(t, "aihub", account)
	table := s.TableFor(s.Objects[0])

	existing := map[string][]string{table: {
		"id", "agency_id", "name", "key_set", "last_test_result", "last_tested_at",
	}}
	got, err := s.AlterDDL("postgres", existing)
	if err != nil {
		t.Fatalf("AlterDDL: %v", err)
	}
	want := []string{
		"alter table " + table + " add column last_test_prompt text default ''",
		"alter table " + table + " add column last_test_reply text default ''",
		"alter table " + table + " add column last_test_ms bigint default 0",
		"alter table " + table + " add column chat_enabled boolean default false",
		"alter table " + table + " add column chat_label text default ''",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statements:\n got  %q\n want %q", got, want)
	}
}

func TestAlterDDLSnapshotsEveryKindOfAddedColumnOnPostgres(t *testing.T) {
	s := declSpec(t, "kinds", declObject("thing", "",
		declField("id", "string", `"primary": true`),
		declField("note", "text", `"required": true`),
		declField("size", "int", `"required": true`),
		declField("ratio", "float", ""),
		declField("raw", "bytes", `"required": true`),
		declField("doc", "json", ""),
		declField("must_doc", "json", `"required": true`),
		declField("spot", "geometry", `"required": true`),
		declField("seen", "timestamp", `"nullable": true`),
		declField("stage", "enum", `"values": ["open", "shut"], "default": "open"`),
		declField("code", "string", `"unique": true, "nullable": true`),
	))
	table := s.TableFor(s.Objects[0])

	got, err := s.AlterDDL("postgres", map[string][]string{table: {"id"}})
	if err != nil {
		t.Fatalf("AlterDDL: %v", err)
	}
	want := []string{
		"alter table " + table + " add column note text not null default ''",
		"alter table " + table + " add column size bigint not null default 0",
		"alter table " + table + " add column ratio double precision default 0",
		"alter table " + table + " add column raw bytea not null default ''",
		"alter table " + table + " add column doc jsonb",
		"alter table " + table + " add column must_doc jsonb",
		"alter table " + table + " add column spot jsonb",
		"alter table " + table + " add column seen text",
		"alter table " + table + " add column stage text default 'open'",
		"alter table " + table + " add column code text",
		"create unique index if not exists " + table + "_code_uq on " + table + " (code)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statements:\n got  %q\n want %q", got, want)
	}
}

func TestAlterDDLSkipsATableThatIsNotThereAndOneWithNothingNew(t *testing.T) {
	s := declSpec(t, "skip1", widgetV2(), gadgetV1())
	widget, gadget := s.TableFor(s.Objects[0]), s.TableFor(s.Objects[1])

	got, err := s.AlterDDL("postgres", map[string][]string{
		gadget: {"id", "label"},
	})
	if err != nil || len(got) != 0 {
		t.Fatalf("a table not in the existing set is created by DDL, not altered, and a complete one needs nothing: %q, %v (widget %s)", got, err, widget)
	}
}

func TestAlterDDLRefusesAMissingPrimaryKeyColumnPlainly(t *testing.T) {
	s := declSpec(t, "skip2", widgetV2())
	table := s.TableFor(s.Objects[0])

	got, err := s.AlterDDL("postgres", map[string][]string{table: {"alpha"}})
	if err == nil || got != nil {
		t.Fatalf("want a refusal and no statements, got %q, %v", got, err)
	}
	for _, part := range []string{table, "primary key", "id"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("the refusal %q does not mention %q", err, part)
		}
	}
}

func TestAlterDDLRefusesAUniqueIndexNameLongerThanThePostgresLimit(t *testing.T) {
	long := strings.Repeat("a", 40)
	s := declSpec(t, "skip3", declObject("thing", "",
		declField("id", "string", `"primary": true`),
		declField(long, "string", `"unique": true`),
	))
	table := s.TableFor(s.Objects[0])

	_, err := s.AlterDDL("postgres", map[string][]string{table: {"id"}})
	if err == nil || !strings.Contains(err.Error(), long) {
		t.Fatalf("want a refusal naming the field whose unique index cannot be named, got %v", err)
	}
}

func TestDDLStillCreatesAFreshTableWithItsUniqueClause(t *testing.T) {
	s := declSpec(t, "fresh", declObject("thing", "",
		declField("id", "string", `"primary": true`),
		declField("code", "string", `"unique": true`),
	))
	stmts := s.DDL("postgres")[:1]
	if len(s.DDL("postgres")) != 3 {
		t.Fatalf("a table with no declared indexes is one statement, then its test copy and the copy's unique index: %q", s.DDL("postgres"))
	}
	if !strings.Contains(stmts[0], "code text default '' unique") {
		t.Fatalf("a new table keeps the unique clause on its column: %q", stmts)
	}
	if strings.Contains(stmts[0], "_uq") {
		t.Fatalf("a new table must not use the alter path's index: %q", stmts)
	}
}
