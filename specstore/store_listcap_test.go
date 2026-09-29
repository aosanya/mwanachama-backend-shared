package specstore_test

import (
	"context"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func TestS24_OpenBug_ListHasNoDefaultCap(t *testing.T) {
	st, _ := newStore(t)
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
		t.Fatalf("S24 open bug: List() on a query with no Limit returned all %d seeded rows; a caller who names no limit should get a capped default page from the engine itself, not the whole live table — every module built on this engine (agency, permissions, catalog) inherits this by construction unless it remembers to call .Limit() on every query it hands in", len(got))
	}
}
