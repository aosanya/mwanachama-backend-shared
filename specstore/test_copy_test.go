package specstore_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func columnNamesOf(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var names []string
	if err := db.Raw(`select name from pragma_table_info(?)`, table).Scan(&names).Error; err != nil {
		t.Fatalf("columns of %s: %v", table, err)
	}
	sort.Strings(names)
	return names
}

func rowCount(t *testing.T, db *gorm.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Raw("select count(*) from " + table).Scan(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestAMountedModuleHasBothTablesAndTheTestOneHasTheRunIDColumn(t *testing.T) {
	st, db := newStore(t)

	if got, want := st.TestTable(roleTicket), st.Table(roleTicket)+spec.TestSuffix; got != want {
		t.Fatalf("TestTable = %q, want %q", got, want)
	}

	live := columnNamesOf(t, db, st.Table(roleTicket))
	test := columnNamesOf(t, db, st.TestTable(roleTicket))
	if len(live) == 0 || len(test) == 0 {
		t.Fatalf("a table is missing: live %v, test %v", live, test)
	}
	want := append([]string{spec.RunIDColumn}, live...)
	sort.Strings(want)
	if !reflect.DeepEqual(test, want) {
		t.Fatalf("test table columns %v, want the live ones plus %s: %v", test, spec.RunIDColumn, want)
	}
	for _, name := range live {
		if name == spec.RunIDColumn {
			t.Fatalf("the live table has %s", spec.RunIDColumn)
		}
	}
}

func TestAStoreWriteToTheLiveTableLeavesTheTestTableEmpty(t *testing.T) {
	st, db := newStore(t)

	if err := st.Insert(context.Background(), roleTicket, Ticket{ID: specstore.NewID(), RaisedBy: "amos"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := rowCount(t, db, st.Table(roleTicket)); got != 1 {
		t.Fatalf("live rows = %d, want 1", got)
	}
	if got := rowCount(t, db, st.TestTable(roleTicket)); got != 0 {
		t.Fatalf("test rows = %d after a live write, want 0", got)
	}
}
