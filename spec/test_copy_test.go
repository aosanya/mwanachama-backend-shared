package spec_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func widgetWithUniqueCodeAndIndex() string {
	return declObject("widget", `{"name": "by_label", "fields": ["label"], "unique": true}`,
		declField("id", "string", `"primary": true`),
		declField("label", "string", ""),
		declField("code", "string", `"unique": true`),
	)
}

func TestTestSuffixIsAppendedToTheLivePhysicalName(t *testing.T) {
	s := declSpec(t, "cp1", widgetV1())
	o := s.Objects[0]

	if spec.TestSuffix != "_test" {
		t.Fatalf("TestSuffix = %q, want _test", spec.TestSuffix)
	}
	if got, want := s.TestTableFor(o), s.TableFor(o)+"_test"; got != want {
		t.Fatalf("TestTableFor = %q, want %q", got, want)
	}
	if got := s.RawNameFor(o); got != "record_main_widget" {
		t.Fatalf("RawNameFor = %q: the raw builder names the live object and takes no suffix", got)
	}
	live, isTest := spec.LiveTableOf(s.TestTableFor(o))
	if !isTest || live != s.TableFor(o) {
		t.Fatalf("LiveTableOf(test) = %q, %v", live, isTest)
	}
	if _, isTest := spec.LiveTableOf(s.TableFor(o)); isTest {
		t.Fatalf("a live table name read as a test table")
	}
	if got, want := spec.InstanceOf(s.TestTableFor(o)), "cp1"; got != want {
		t.Fatalf("InstanceOf(test) = %q, want %q", got, want)
	}
}

func TestMigrateCreatesTheTestCopyBesideTheLiveTable(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp2", widgetV1(), gadgetV1())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, o := range s.Objects {
		live := columnsOf(t, db, s.TableFor(o))
		test := columnsOf(t, db, s.TestTableFor(o))
		if len(test) == 0 {
			t.Fatalf("%s has no test copy", s.TableFor(o))
		}
		want := append([]string{spec.RunIDColumn}, live...)
		sort.Strings(want)
		if !reflect.DeepEqual(test, want) {
			t.Fatalf("test copy of %s has columns %v, want the live ones %v plus %s", o.Name, test, live, spec.RunIDColumn)
		}
		for _, name := range live {
			if name == spec.RunIDColumn {
				t.Fatalf("the live table of %s carries %s", o.Name, spec.RunIDColumn)
			}
		}
	}
}

func TestTheRunIDColumnIsRequiredSoARowMustNameItsRun(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp3", widgetV1())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	test := s.TestTableFor(s.Objects[0])

	if err := db.Exec("insert into " + test + " (id) values ('w1')").Error; err == nil {
		t.Fatalf("a row with no run id went into %s", test)
	}
	if err := db.Exec("insert into "+test+" (run_id, id) values (?, 'w1')", "run-1").Error; err != nil {
		t.Fatalf("a row naming its run was refused: %v", err)
	}
}

func TestAWriteToTheLiveTableLeavesTheTestCopyEmpty(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp4", widgetV1())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	live, test := s.TableFor(s.Objects[0]), s.TestTableFor(s.Objects[0])

	if err := db.Exec("insert into " + live + " (id, alpha, beta) values ('w1', 'a', 'b')").Error; err != nil {
		t.Fatalf("live insert: %v", err)
	}
	if got := countRows(t, db, test); got != 0 {
		t.Fatalf("test copy holds %d rows after a live write, want 0", got)
	}
	if err := db.Exec("insert into " + test + " (run_id, id) values ('run-1', 'w1')").Error; err != nil {
		t.Fatalf("test insert: %v", err)
	}
	if got := countRows(t, db, live); got != 1 {
		t.Fatalf("live table holds %d rows after a test write, want the 1 it had", got)
	}
}

func TestRunsShareOneTestCopyWithoutCollidingOnKeyOrUniqueValue(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp5", widgetWithUniqueCodeAndIndex())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	test := s.TestTableFor(s.Objects[0])
	insert := "insert into " + test + " (run_id, id, label, code) values (?, 'w1', 'l', 'c')"

	if err := db.Exec(insert, "run-1").Error; err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if err := db.Exec(insert, "run-2").Error; err != nil {
		t.Fatalf("a second run could not write the same key, label and unique code: %v", err)
	}
	if err := db.Exec(insert, "run-1").Error; err == nil {
		t.Fatalf("one run wrote the same key twice")
	}
	if err := db.Exec("insert into " + test + " (run_id, id, label, code) values ('run-1', 'w2', 'l2', 'c')").Error; err == nil {
		t.Fatalf("one run wrote the same unique code twice")
	}
	if err := db.Exec("insert into " + test + " (run_id, id, label, code) values ('run-1', 'w3', 'l', 'c3')").Error; err == nil {
		t.Fatalf("one run wrote the same value under a unique index twice")
	}
}

func TestTheLiveTableKeepsItsOwnKeyAndUniqueness(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp6", widgetWithUniqueCodeAndIndex())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	live := s.TableFor(s.Objects[0])

	if err := db.Exec("insert into " + live + " (id, label, code) values ('w1', 'l', 'c')").Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := db.Exec("insert into " + live + " (id, label, code) values ('w2', 'l2', 'c')").Error; err == nil {
		t.Fatalf("the live table took the same unique code twice")
	}
}

func TestMigrateIsIdempotentForTheTestCopy(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp7", widgetWithUniqueCodeAndIndex())
	for i := 0; i < 3; i++ {
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}
}

func TestMigrateEvolvesTheTestCopyWithTheLiveTable(t *testing.T) {
	db := open(t)
	if err := spec.Migrate(db, declSpec(t, "cp8", widgetV1())); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	v2 := declSpec(t, "cp8", widgetV2())
	if err := spec.Migrate(db, v2); err != nil {
		t.Fatalf("migrate v2: %v", err)
	}
	o := v2.Objects[0]
	test := columnsOf(t, db, v2.TestTableFor(o))
	for _, want := range []string{"gamma", "delta", "epsilon", spec.RunIDColumn} {
		if !holdsColumn(test, want) {
			t.Fatalf("test copy columns %v lack %s after the live table gained it", test, want)
		}
	}
}

func TestAlterDDLNamesATestCopyThatLacksAColumn(t *testing.T) {
	s := declSpec(t, "cp9", widgetV2())
	o := s.Objects[0]
	held := []string{"id", "alpha", "beta"}

	stmts, err := s.AlterDDL("postgres", map[string][]string{
		s.TestTableFor(o): append([]string{spec.RunIDColumn}, held...),
	})
	if err != nil {
		t.Fatalf("AlterDDL: %v", err)
	}
	if len(stmts) != 3 {
		t.Fatalf("want the 3 alters of the test copy, got %q", stmts)
	}
	for _, stmt := range stmts {
		if !strings.HasPrefix(stmt, "alter table "+s.TestTableFor(o)+" ") {
			t.Fatalf("alter aimed at the wrong table: %s", stmt)
		}
	}
}

func TestDDLCreatesTheTestCopyAfterTheLiveTablesAndIndexesLeadWithTheRunID(t *testing.T) {
	s := declSpec(t, "cp10", widgetWithUniqueCodeAndIndex())
	stmts := s.DDL("postgres")
	o := s.Objects[0]

	var sawTable, sawLabelIndex, sawCodeIndex bool
	for _, stmt := range stmts {
		switch {
		case strings.HasPrefix(stmt, "create table if not exists "+s.TestTableFor(o)+" "):
			sawTable = true
			if !strings.Contains(stmt, "primary key (run_id, id)") {
				t.Errorf("test copy key does not lead with the run id: %s", stmt)
			}
			if strings.Contains(stmt, "unique") {
				t.Errorf("test copy keeps a table-wide unique clause that would hold runs apart: %s", stmt)
			}
		case strings.Contains(stmt, s.TestIndexFor(o, o.Indexes[0])):
			sawLabelIndex = true
			if !strings.Contains(stmt, "(run_id, label)") {
				t.Errorf("declared index of the copy does not lead with the run id: %s", stmt)
			}
		case strings.Contains(stmt, s.TestTableFor(o)+"_code_uq"):
			sawCodeIndex = true
			if !strings.Contains(stmt, "(run_id, code)") {
				t.Errorf("unique field of the copy is not unique per run: %s", stmt)
			}
		}
	}
	if !sawTable || !sawLabelIndex || !sawCodeIndex {
		t.Fatalf("DDL lacks part of the test copy (table %v, declared index %v, unique field %v):\n%s",
			sawTable, sawLabelIndex, sawCodeIndex, strings.Join(stmts, "\n"))
	}
}

func TestAFieldCannotBeNamedLikeTheRunIDColumn(t *testing.T) {
	raw := `{"module": "record", "domain": "probe", "instance": "cp11", "objects": [` +
		declObject("widget", "",
			declField("id", "string", `"primary": true`),
			declField("run_id", "string", ""),
		) + `]}`
	_, err := spec.Parse([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "reserved for the run id") {
		t.Fatalf("a field named run_id was accepted: %v", err)
	}
}

func TestATestCopyNameThatWouldBeTruncatedIsRefusedAtLoad(t *testing.T) {
	long := strings.Repeat("a", spec.MaxIdentifier-len("cp12")-1-spec.MountHashLength-1-spec.HashLength-len("_x")-len("_idx")-len(spec.TestSuffix)+1)
	raw := `{"module": "record", "domain": "probe", "instance": "cp12", "objects": [` +
		declObject("widget", `{"name": "`+long+`x", "fields": ["id"]}`,
			declField("id", "string", `"primary": true`),
		) + `]}`
	_, err := spec.Parse([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "test index") {
		t.Fatalf("an index whose test name overflows was accepted: %v", err)
	}
}

func TestSettingsTablesGetNoTestCopy(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp13", widgetV1())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, table := range s.SettingsTables() {
		if got := columnsOf(t, db, table+spec.TestSuffix); len(got) != 0 {
			t.Fatalf("settings table %s has a test copy", table)
		}
	}
}

func TestLookupNameAnswersATestTableWithItsLiveRecord(t *testing.T) {
	db := open(t)
	s := declSpec(t, "cp14", widgetV1())
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	o := s.Objects[0]

	got, found, err := spec.LookupName(db, s.TestTableFor(o))
	if err != nil || !found {
		t.Fatalf("LookupName(test) = %+v, %v, %v", got, found, err)
	}
	if got.Physical != s.TableFor(o) || got.Raw != s.RawNameFor(o) {
		t.Fatalf("LookupName(test) = %+v, want the live record", got)
	}
}

func TestTheNameRegistryRecordsLiveTablesOnly(t *testing.T) {
	s := declSpec(t, "cp15", widgetV1(), gadgetV1())
	for _, rec := range s.NameRecords() {
		if strings.HasSuffix(rec.Physical, spec.TestSuffix) {
			t.Fatalf("the registry records a test table: %+v", rec)
		}
	}
	if got := len(s.NameRecords()); got != 2 {
		t.Fatalf("records = %d, want one per object", got)
	}
}

func countRows(t *testing.T, db *gorm.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Raw("select count(*) from " + table).Scan(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func holdsColumn(in []string, want string) bool {
	for _, s := range in {
		if s == want {
			return true
		}
	}
	return false
}
