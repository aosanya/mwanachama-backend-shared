package specstore_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const instantSpec = `{
  "module": "record",
  "domain": "support",
  "instance": "support",
  "objects": [
    {
      "name": "event",
      "table": "events",
      "role": "event",
      "description": "One thing that happened.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "happened_at", "type": "timestamp", "description": "When it happened."},
        {"name": "settled_at", "type": "timestamp", "description": "When it was settled.", "nullable": true}
      ]
    }
  ]
}`

const roleEvent = "event"

type Event struct {
	ID         string
	HappenedAt time.Time
	SettledAt  *time.Time
}

func eventObject(t *testing.T) spec.Object {
	t.Helper()
	s, err := spec.Parse([]byte(instantSpec))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	o, ok := s.ByRole(roleEvent)
	if !ok {
		t.Fatal("no event object")
	}
	return o
}

func TestATimeIsCarriedAsAnInstant(t *testing.T) {
	o := eventObject(t)
	at := time.Date(2026, 9, 30, 14, 5, 6, 123456789, time.UTC)

	row, err := specstore.Encode(o, Event{ID: "e1", HappenedAt: at})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got, want := row["happened_at"], "2026-09-30T14:05:06.123456789Z"; got != want {
		t.Fatalf("happened_at = %v, want %q", got, want)
	}

	var out Event
	if err := specstore.Decode(o, row, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.HappenedAt.Equal(at) {
		t.Fatalf("round trip lost the instant: %v, want %v", out.HappenedAt, at)
	}
}

func TestAnInstantIsStoredInUTCWhateverZoneItArrivesIn(t *testing.T) {
	o := eventObject(t)
	nairobi := time.FixedZone("EAT", 3*60*60)
	at := time.Date(2026, 9, 30, 17, 0, 0, 0, nairobi)

	row, err := specstore.Encode(o, Event{ID: "e1", HappenedAt: at})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got, want := row["happened_at"], "2026-09-30T14:00:00.000000000Z"; got != want {
		t.Fatalf("happened_at = %v, want %q — an offset in the stored text orders two identical instants apart", got, want)
	}
}

// A timestamp is a text column, so ORDER BY over it is a string comparison.
func TestStoredInstantsSortChronologically(t *testing.T) {
	o := eventObject(t)
	base := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	instants := []time.Time{
		base.Add(500 * time.Millisecond),
		base.Add(50 * time.Millisecond),
		base.Add(2 * time.Second),
		base,
		base.Add(1500 * time.Millisecond),
	}

	var stored []string
	for _, at := range instants {
		row, err := specstore.Encode(o, Event{ID: "e", HappenedAt: at})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		stored = append(stored, row["happened_at"].(string))
	}

	lexical := append([]string(nil), stored...)
	sort.Strings(lexical)

	chronological := append([]time.Time{}, instants...)
	sort.Slice(chronological, func(i, j int) bool { return chronological[i].Before(chronological[j]) })

	for i, at := range chronological {
		want := at.UTC().Format(specstore.TimeLayout)
		if lexical[i] != want {
			t.Fatalf("sorted as text, position %d is %q, chronologically it is %q — a trimmed fractional part orders .5 before .05",
				i, lexical[i], want)
		}
	}
}

func TestANullableInstantKeepsAbsentApartFromTheZeroInstant(t *testing.T) {
	o := eventObject(t)

	row, err := specstore.Encode(o, Event{ID: "e1"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if row["settled_at"] != nil {
		t.Fatalf("settled_at = %v, want nil", row["settled_at"])
	}

	var out Event
	if err := specstore.Decode(o, row, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SettledAt != nil {
		t.Fatalf("an absent instant read back as %v", *out.SettledAt)
	}

	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	row, err = specstore.Encode(o, Event{ID: "e1", SettledAt: &at})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out = Event{}
	if err := specstore.Decode(o, row, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SettledAt == nil || !out.SettledAt.Equal(at) {
		t.Fatalf("settled_at = %v, want %v", out.SettledAt, at)
	}
}

// What a driver hands back for a real timestamptz column, which is what a
// legacy table adopted onto a declared name still holds.
func TestAnInstantReadsBackFromADriverTimeAndFromOtherLayouts(t *testing.T) {
	o := eventObject(t)
	at := time.Date(2026, 9, 30, 14, 0, 0, 0, time.UTC)

	for _, raw := range []any{
		at,
		"2026-09-30T14:00:00.000000000Z",
		"2026-09-30T14:00:00Z",
		"2026-09-30 14:00:00+00:00",
		"2026-09-30 14:00:00",
	} {
		var out Event
		if err := specstore.Decode(o, map[string]any{"happened_at": raw}, &out); err != nil {
			t.Fatalf("decode %v: %v", raw, err)
		}
		if !out.HappenedAt.Equal(at) {
			t.Errorf("decode %v gave %v, want %v", raw, out.HappenedAt, at)
		}
	}
}

func TestACarrierTheCodecCannotReadIsRefusedRatherThanStored(t *testing.T) {
	o := eventObject(t)

	type bad struct {
		ID         string
		HappenedAt []byte
		SettledAt  *time.Time
	}
	_, err := specstore.Encode(o, bad{ID: "e1", HappenedAt: []byte{0xde, 0xad}})
	if err == nil {
		t.Fatal("encode accepted a []byte for a declared timestamp; reflect would have stored \"<[]uint8 Value>\"")
	}
	if !strings.Contains(err.Error(), "carried as") {
		t.Fatalf("err = %v, want it to name the carried type", err)
	}
}

func TestATimeCarriedByANonTimestampFieldIsRefused(t *testing.T) {
	s, err := spec.Parse([]byte(`{
	  "module": "record", "domain": "support", "instance": "support",
	  "objects": [{"name":"event","table":"events","role":"event","description":"One thing that happened.",
	    "fields":[{"name":"id","type":"string","description":"Storage key.","primary":true},
	              {"name":"happened_at","type":"string","description":"When, as a label."}]}]}`))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	o, _ := s.ByRole(roleEvent)

	type labelled struct {
		ID         string
		HappenedAt time.Time
	}
	_, err = specstore.Encode(o, labelled{ID: "e1", HappenedAt: time.Now()})
	if err == nil {
		t.Fatal("encode accepted a time.Time for a declared string")
	}
	if !strings.Contains(err.Error(), "time.Time") {
		t.Fatalf("err = %v, want it to name time.Time", err)
	}
}

func TestAnInstantSurvivesTheDatabase(t *testing.T) {
	db := openDB(t)
	s, err := spec.Parse([]byte(instantSpec))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{roleEvent: Event{}})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	at := time.Date(2026, 9, 30, 14, 5, 6, 123456789, time.UTC)
	if err := st.Insert(ctx, roleEvent, Event{ID: "e1", HappenedAt: at}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var out Event
	if err := st.Take(st.Query(ctx, roleEvent).Where("id = ?", "e1"), roleEvent, &out, errNoTicket); err != nil {
		t.Fatalf("take: %v", err)
	}
	if !out.HappenedAt.Equal(at) {
		t.Fatalf("read back %v, want %v", out.HappenedAt, at)
	}
	if out.SettledAt != nil {
		t.Fatalf("settled_at read back as %v, want nil", *out.SettledAt)
	}
}
