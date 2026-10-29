package specstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func forRun(t *testing.T, st *specstore.Store, id string) *specstore.Store {
	t.Helper()
	scoped, err := st.ForRun(id)
	if err != nil {
		t.Fatalf("for run %s: %v", id, err)
	}
	return scoped
}

func TestARunStoreWritesOnlyToTheTestTableAndStampsItsRunID(t *testing.T) {
	st, db := newStore(t)
	run := forRun(t, st, "run-1")

	if err := run.Insert(context.Background(), roleTicket, Ticket{ID: "t1", RaisedBy: "amos"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := rowCount(t, db, st.Table(roleTicket)); got != 0 {
		t.Fatalf("live rows = %d after a run write, want 0", got)
	}
	var stamped []string
	if err := db.Raw("select " + spec.RunIDColumn + " from " + st.TestTable(roleTicket)).Scan(&stamped).Error; err != nil {
		t.Fatalf("read run ids: %v", err)
	}
	if len(stamped) != 1 || stamped[0] != "run-1" {
		t.Fatalf("run ids = %v, want [run-1]", stamped)
	}
}

func TestTwoRunsWriteTheSameKeyAndEachReadsOnlyItsOwnRows(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	first, second := forRun(t, st, "run-1"), forRun(t, st, "run-2")

	if err := first.Insert(ctx, roleTicket, Ticket{ID: "same", RaisedBy: "farmer"}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := second.Insert(ctx, roleTicket, Ticket{ID: "same", RaisedBy: "shop"}); err != nil {
		t.Fatalf("second insert with the same id: %v", err)
	}

	for run, want := range map[*specstore.Store]string{first: "farmer", second: "shop"} {
		got, err := specstore.List[Ticket](run, run.Query(ctx, roleTicket), roleTicket)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 1 || got[0].RaisedBy != want {
			t.Fatalf("run %s read %+v, want one ticket raised by %s", run.RunID(), got, want)
		}
	}
}

func TestARunStoreUpdateTouchesOnlyItsOwnRow(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	first, second := forRun(t, st, "run-1"), forRun(t, st, "run-2")
	for _, run := range []*specstore.Store{first, second} {
		if err := run.Insert(ctx, roleTicket, Ticket{ID: "same", Replies: 1}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	if err := first.Query(ctx, roleTicket).Where("id = ?", "same").Update("replies", 9).Error; err != nil {
		t.Fatalf("update: %v", err)
	}

	var other Ticket
	if err := second.Take(second.Query(ctx, roleTicket).Where("id = ?", "same"), roleTicket, &other, errNoTicket); err != nil {
		t.Fatalf("take: %v", err)
	}
	if other.Replies != 1 {
		t.Fatalf("the other run's replies = %d, want 1", other.Replies)
	}
}

func TestRowsOfRunCountsEachTableAndRemoveRunDeletesOnlyThatRun(t *testing.T) {
	st, db := newStore(t)
	ctx := context.Background()
	first, second := forRun(t, st, "run-1"), forRun(t, st, "run-2")
	for _, id := range []string{"a", "b"} {
		if err := first.Insert(ctx, roleTicket, Ticket{ID: id}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	if err := second.Insert(ctx, roleTicket, Ticket{ID: "a"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := st.Insert(ctx, roleTicket, Ticket{ID: "live"}); err != nil {
		t.Fatalf("live insert: %v", err)
	}

	rows, err := specstore.RowsOfRun(ctx, db, st.Spec(), "run-1")
	if err != nil {
		t.Fatalf("rows of run: %v", err)
	}
	if len(rows) != 1 || rows[0].Table != st.TestTable(roleTicket) || rows[0].Rows != 2 {
		t.Fatalf("rows of run-1 = %+v, want 2 in %s", rows, st.TestTable(roleTicket))
	}

	removed, err := specstore.RemoveRun(ctx, db, st.Spec(), "run-1")
	if err != nil {
		t.Fatalf("remove run: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if got := rowCount(t, db, st.TestTable(roleTicket)); got != 1 {
		t.Fatalf("test rows left = %d, want the other run's 1", got)
	}
	if got := rowCount(t, db, st.Table(roleTicket)); got != 1 {
		t.Fatalf("live rows = %d, want 1 untouched", got)
	}
}

func TestARunWithNoIDIsRefused(t *testing.T) {
	st, db := newStore(t)
	if _, err := st.ForRun(""); !errors.Is(err, specstore.ErrRunIDEmpty) {
		t.Fatalf("ForRun(\"\") = %v, want ErrRunIDEmpty", err)
	}
	if _, err := specstore.RemoveRun(context.Background(), db, st.Spec(), ""); !errors.Is(err, specstore.ErrRunIDEmpty) {
		t.Fatalf("RemoveRun(\"\") = %v, want ErrRunIDEmpty", err)
	}
	if _, err := specstore.RowsOfRun(context.Background(), db, st.Spec(), ""); !errors.Is(err, specstore.ErrRunIDEmpty) {
		t.Fatalf("RowsOfRun(\"\") = %v, want ErrRunIDEmpty", err)
	}
}
