package specstore_test

import (
	"context"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func TestS24_ListCappedByDefaultOnceAStoreOptsIn(t *testing.T) {
	st, _ := newStore(t)
	st.CapLists(specstore.DefaultListCap)
	ctx := context.Background()

	const seeded = 150
	for i := 0; i < seeded; i++ {
		if err := st.Insert(ctx, roleTicket, Ticket{ID: specstore.NewID(), RaisedBy: "amos"}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	got, err := specstore.List[Ticket](st, st.Query(ctx, roleTicket), roleTicket)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) >= seeded {
		t.Fatalf("S24: a capped store returned all %d seeded rows from a query with no Limit", len(got))
	}
}

func seedTickets(t *testing.T, st *specstore.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := st.Insert(context.Background(), roleTicket, Ticket{ID: specstore.NewID(), RaisedBy: "amos"}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
}

func TestListCapsAtTheDefaultAndNoMore(t *testing.T) {
	st, _ := newStore(t)
	st.CapLists(specstore.DefaultListCap)
	seedTickets(t, st, specstore.DefaultListCap+20)
	got, err := specstore.List[Ticket](st, st.Query(context.Background(), roleTicket), roleTicket)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != specstore.DefaultListCap {
		t.Fatalf("len = %d, want %d", len(got), specstore.DefaultListCap)
	}
}

func TestListKeepsTheCallersOwnLimit(t *testing.T) {
	st, _ := newStore(t)
	st.CapLists(specstore.DefaultListCap)
	seedTickets(t, st, specstore.DefaultListCap+20)
	ctx := context.Background()
	for limit, want := range map[int]int{5: 5, specstore.DefaultListCap + 10: specstore.DefaultListCap + 10, -1: specstore.DefaultListCap + 20} {
		got, err := specstore.List[Ticket](st, st.Query(ctx, roleTicket).Limit(limit), roleTicket)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want {
			t.Errorf("Limit(%d): len = %d, want %d", limit, len(got), want)
		}
	}
}

func TestListDoesNotLeakItsCapIntoTheCallersQuery(t *testing.T) {
	st, _ := newStore(t)
	st.CapLists(specstore.DefaultListCap)
	seedTickets(t, st, specstore.DefaultListCap+20)
	q := st.Query(context.Background(), roleTicket)
	if _, err := specstore.List[Ticket](st, q, roleTicket); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != int64(specstore.DefaultListCap+20) {
		t.Fatalf("count after List = %d, want %d", n, specstore.DefaultListCap+20)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil || len(rows) != specstore.DefaultListCap+20 {
		t.Fatalf("Find after List = %d rows, err %v", len(rows), err)
	}
}

func TestListIsUncappedUntilAStoreOptsIn(t *testing.T) {
	st, _ := newStore(t)
	seedTickets(t, st, specstore.DefaultListCap+20)
	got, err := specstore.List[Ticket](st, st.Query(context.Background(), roleTicket), roleTicket)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != specstore.DefaultListCap+20 {
		t.Fatalf("len = %d, want %d", len(got), specstore.DefaultListCap+20)
	}
}

func TestARunScopedStoreInheritsTheListCap(t *testing.T) {
	st, _ := newStore(t)
	st.CapLists(7)
	run := forRun(t, st, "run-1")
	seedTickets(t, run, 20)
	got, err := specstore.List[Ticket](run, run.Query(context.Background(), roleTicket), roleTicket)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 {
		t.Fatalf("len = %d, want 7", len(got))
	}
}
