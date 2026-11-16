package specstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func countTickets(t *testing.T, st *specstore.Store) int {
	t.Helper()
	all, err := specstore.List[Ticket](st, st.Query(context.Background(), roleTicket), roleTicket)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return len(all)
}

func TestInTransactionCommitsEveryWriteWhenTheFunctionSucceeds(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	err := st.InTransaction(ctx, func(tx *specstore.Store) error {
		if err := tx.Insert(ctx, roleTicket, Ticket{ID: specstore.NewID(), RaisedBy: "amos"}); err != nil {
			return err
		}
		return tx.Insert(ctx, roleTicket, Ticket{ID: specstore.NewID(), RaisedBy: "neema"})
	})
	if err != nil {
		t.Fatalf("InTransaction: %v", err)
	}
	if got := countTickets(t, st); got != 2 {
		t.Errorf("%d tickets after commit, want 2", got)
	}
}

func TestInTransactionRollsBackEveryWriteWhenTheFunctionFails(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	boom := errors.New("second write failed")
	err := st.InTransaction(ctx, func(tx *specstore.Store) error {
		if err := tx.Insert(ctx, roleTicket, Ticket{ID: specstore.NewID(), RaisedBy: "amos"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTransaction = %v, want the function's own error", err)
	}
	if got := countTickets(t, st); got != 0 {
		t.Errorf("%d tickets after rollback, want 0", got)
	}
}
