package specstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const roleReading = "reading"

const readingSpec = `{
  "module": "record",
  "domain": "support",
  "instance": "support",
  "objects": [
    {
      "name": "reading",
      "table": "readings",
      "role": "reading",
      "description": "One measurement somebody took.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key for this reading.", "primary": true},
        {"name": "ceiling", "type": "int", "description": "The highest value accepted, absent when there is no limit.", "nullable": true},
        {"name": "amount", "type": "float", "description": "What was measured, absent when nothing was.", "nullable": true},
        {"name": "agreed", "type": "bool", "description": "Whether the taker agreed with it, absent when unasked.", "nullable": true},
        {"name": "marks", "type": "json", "description": "The values offered as quick picks."},
        {"name": "tags", "type": "json", "description": "Whatever the taker filed it under."},
        {"name": "margin", "type": "float", "description": "How far out it may be."}
      ]
    }
  ]
}`

type Reading struct {
	ID      string    `json:"id"`
	Ceiling *int      `json:"ceiling,omitempty"`
	Amount  *float64  `json:"amount,omitempty"`
	Agreed  *bool     `json:"agreed,omitempty"`
	Marks   []float64 `json:"marks,omitempty"`
	Tags    []string  `json:"tags,omitempty"`
	Margin  float64   `json:"margin"`
}

var errNoReading = errors.New("no such reading")

func readingStore(t *testing.T) *specstore.Store {
	t.Helper()
	db := openDB(t)
	s, err := spec.Parse([]byte(readingSpec))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{roleReading: Reading{}})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return st
}

func readBack(t *testing.T, st *specstore.Store, id string) Reading {
	t.Helper()
	var got Reading
	q := st.Query(context.Background(), roleReading).Where("id = ?", id)
	if err := st.Take(q, roleReading, &got, errNoReading); err != nil {
		t.Fatalf("take: %v", err)
	}
	return got
}

// An absent value has to stay absent. Without this the three columns would
// read back as 0, 0 and false, which are answers somebody gave rather than
// ones nobody did — the distinction a nullable column exists to keep.
func TestAnAbsentValueStaysAbsent(t *testing.T) {
	st := readingStore(t)
	in := Reading{ID: specstore.NewID()}
	if err := st.Insert(context.Background(), roleReading, in); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got := readBack(t, st, in.ID)
	if got.Ceiling != nil || got.Amount != nil || got.Agreed != nil {
		t.Errorf("read back %+v, want every nullable column still absent", got)
	}
}

// The zero value is a value. A stored 0 or false has to come back as one
// rather than as nothing, which is the other half of the same distinction.
func TestAStoredZeroIsNotAnAbsentValue(t *testing.T) {
	st := readingStore(t)
	ceiling, amount, agreed := 0, 0.0, false
	in := Reading{ID: specstore.NewID(), Ceiling: &ceiling, Amount: &amount, Agreed: &agreed}
	if err := st.Insert(context.Background(), roleReading, in); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got := readBack(t, st, in.ID)
	switch {
	case got.Ceiling == nil || *got.Ceiling != 0:
		t.Errorf("Ceiling = %v, want a stored 0", got.Ceiling)
	case got.Amount == nil || *got.Amount != 0:
		t.Errorf("Amount = %v, want a stored 0", got.Amount)
	case got.Agreed == nil || *got.Agreed != false:
		t.Errorf("Agreed = %v, want a stored false", got.Agreed)
	}
}

func TestAPresentValueRoundTrips(t *testing.T) {
	st := readingStore(t)
	ceiling, amount, agreed := 240, 12.5, true
	in := Reading{
		ID:      specstore.NewID(),
		Ceiling: &ceiling,
		Amount:  &amount,
		Agreed:  &agreed,
		Marks:   []float64{50, 100, 250.5},
		Tags:    []string{"kitchen", "morning"},
		Margin:  0.25,
	}
	if err := st.Insert(context.Background(), roleReading, in); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got := readBack(t, st, in.ID)
	switch {
	case got.Ceiling == nil || *got.Ceiling != ceiling:
		t.Errorf("Ceiling = %v, want %d", got.Ceiling, ceiling)
	case got.Amount == nil || *got.Amount != amount:
		t.Errorf("Amount = %v, want %v", got.Amount, amount)
	case got.Agreed == nil || *got.Agreed != agreed:
		t.Errorf("Agreed = %v, want %v", got.Agreed, agreed)
	case len(got.Marks) != 3 || got.Marks[2] != 250.5:
		t.Errorf("Marks = %v, want the three picks back", got.Marks)
	case len(got.Tags) != 2 || got.Tags[0] != "kitchen":
		t.Errorf("Tags = %v, want the two tags back", got.Tags)
	case got.Margin != 0.25:
		t.Errorf("Margin = %v, want the fraction kept", got.Margin)
	}
}

// An empty list is stored as NULL, so a column never written stays
// distinguishable from one holding an empty document.
func TestAnEmptyListIsStoredAsNull(t *testing.T) {
	s, err := spec.Parse([]byte(readingSpec))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	o, _ := s.ByRole(roleReading)

	row, err := specstore.Encode(o, Reading{ID: "r1"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if row["marks"] != nil || row["tags"] != nil {
		t.Errorf("marks = %v, tags = %v, want both stored as NULL", row["marks"], row["tags"])
	}
}

// A nullable column carried by a plain value, or a pointer landing in a
// column nobody declared nullable, is the silent half of this: both compile,
// both write, and both lose the distinction on every row. They fail when the
// store is built instead.
func TestNewRefusesACarrierThatCannotHoldAnAbsentValue(t *testing.T) {
	s, err := spec.Parse([]byte(readingSpec))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	flattened := struct {
		ID      string
		Ceiling int
		Amount  *float64
		Agreed  *bool
		Marks   []float64
		Tags    []string
		Margin  float64
	}{}
	_, err = specstore.New(openDB(t), s, map[string]any{roleReading: flattened})
	if err == nil || !strings.Contains(err.Error(), "read back as the zero value") {
		t.Errorf("err = %v, want a refusal naming the flattened nullable column", err)
	}

	pointed := struct {
		ID      string
		Ceiling *int
		Amount  *float64
		Agreed  *bool
		Marks   []float64
		Tags    []string
		Margin  *float64
	}{}
	_, err = specstore.New(openDB(t), s, map[string]any{roleReading: pointed})
	if err == nil || !strings.Contains(err.Error(), "does not declare it nullable") {
		t.Errorf("err = %v, want a refusal naming the undeclared pointer", err)
	}
}
