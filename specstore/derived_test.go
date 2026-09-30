package specstore_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func reflectTypeOf(v any) reflect.Type { return reflect.TypeOf(v) }

type TicketWithDerived struct {
	ID         string   `json:"id"`
	RaisedBy   string   `json:"raised_by"`
	KeyHash    string   `json:"-"`
	Doc        string   `json:"doc"`
	AnsweredAt string   `json:"answered_at,omitempty"`
	Replies    int      `json:"replies"`
	Closed     bool     `json:"closed"`
	LabelIDs   []string `json:"label_ids,omitempty" spec:"-"`
	RaisedByOn string   `json:"raised_by_on,omitempty" spec:"-"`
}

func TestADerivedFieldIsNotAColumnAndDoesNotBlockTheStore(t *testing.T) {
	db := openDB(t)
	s := loadSpec(t)
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := specstore.New(db, s, map[string]any{roleTicket: TicketWithDerived{}}); err != nil {
		t.Fatalf("a carrier whose extra fields are all spec:\"-\" must be accepted: %v", err)
	}
}

func TestAnUntaggedExtraFieldIsStillRefused(t *testing.T) {
	db := openDB(t)
	s := loadSpec(t)
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	type TicketWithStray struct {
		TicketWithDerived
		Stray string `json:"stray"`
	}
	_, err := specstore.New(db, s, map[string]any{roleTicket: TicketWithStray{}})
	if err == nil {
		t.Fatal("an extra field with no spec:\"-\" must still be refused")
	}
	if !strings.Contains(err.Error(), "stray") {
		t.Fatalf("the refusal must name the field, got: %v", err)
	}
}

func TestADerivedFieldIsNeverWrittenAndNeverRead(t *testing.T) {
	db := openDB(t)
	s := loadSpec(t)
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{roleTicket: TicketWithDerived{}})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	in := TicketWithDerived{
		ID:         "t1",
		RaisedBy:   "amos",
		KeyHash:    "sha256:abc",
		LabelIDs:   []string{"l1", "l2"},
		RaisedByOn: "never stored",
	}
	if err := st.Insert(ctx, roleTicket, in); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var back TicketWithDerived
	if err := st.Take(st.Query(ctx, roleTicket).Where("id = ?", "t1"), roleTicket, &back, errNoTicket); err != nil {
		t.Fatalf("take: %v", err)
	}
	if back.RaisedBy != "amos" || back.KeyHash != "sha256:abc" {
		t.Fatalf("the declared columns must round-trip, got %+v", back)
	}
	if back.LabelIDs != nil || back.RaisedByOn != "" {
		t.Fatalf("a derived field must come back empty for its owner to fill, got %+v", back)
	}
}

func TestADerivedFieldIsNotEvenAKnownColumn(t *testing.T) {
	cols := specstore.ColumnsOf(reflectTypeOf(TicketWithDerived{}))
	if cols["label_ids"] || cols["raised_by_on"] {
		t.Fatalf("ColumnsOf must not report a derived field as a column, got %v", cols)
	}
	if !cols["raised_by"] {
		t.Fatalf("ColumnsOf must still report a real column, got %v", cols)
	}
}
