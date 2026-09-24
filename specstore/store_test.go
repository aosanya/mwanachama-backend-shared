package specstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const roleTicket = "ticket"

const ticketSpec = `{
  "module": "record",
  "domain": "support",
  "instance": "support",
  "objects": [
    {
      "name": "ticket",
      "table": "tickets",
      "role": "ticket",
      "description": "One thing somebody asked for.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key for this ticket.", "primary": true},
        {"name": "raised_by", "type": "string", "description": "Who asked for it."},
        {"name": "key_hash", "type": "string", "description": "The hash of the key that opens it."},
        {"name": "doc", "type": "json", "description": "Everything else about it."},
        {"name": "answered_at", "type": "timestamp", "description": "When it was answered."},
        {"name": "replies", "type": "int", "description": "How many answers it has had."},
        {"name": "closed", "type": "bool", "description": "Whether it needs anything more."}
      ]
    }
  ]
}`

type Ticket struct {
	ID         string `json:"id"`
	RaisedBy   string `json:"raised_by"`
	KeyHash    string `json:"-"`
	Doc        string `json:"doc"`
	AnsweredAt string `json:"answered_at,omitempty"`
	Replies    int    `json:"replies"`
	Closed     bool   `json:"closed"`
}

var errNoTicket = errors.New("no such ticket")

func loadSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := spec.Parse([]byte(ticketSpec))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return s
}

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func newStore(t *testing.T) (*specstore.Store, *gorm.DB) {
	t.Helper()
	db := openDB(t)
	s := loadSpec(t)
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{roleTicket: Ticket{}})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return st, db
}

// The column a field lands in is derived from its name, so the rule that
// derives it is worth pinning — especially over an initialism, which is
// where a naive camel-to-snake split scatters letters.
func TestColumnName(t *testing.T) {
	for _, tc := range []struct{ field, want string }{
		{"ID", "id"},
		{"Slug", "slug"},
		{"SubmittedBy", "submitted_by"},
		{"RunID", "run_id"},
		{"SuggestionID", "suggestion_id"},
		{"KeyHash", "key_hash"},
		{"ModuleConfigs", "module_configs"},
		{"TemplatesFiled", "templates_filed"},
		{"MetAt", "met_at"},
	} {
		if got := specstore.ColumnName(tc.field); got != tc.want {
			t.Errorf("ColumnName(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
}

// A field a json tag hides is still stored. The join is the field name for
// exactly this reason: a hash kept out of every API response by `json:"-"`
// would otherwise stop being written, and nothing would report it.
func TestEncodeCarriesAFieldThatJSONHides(t *testing.T) {
	s := loadSpec(t)
	o, ok := s.ByRole(roleTicket)
	if !ok {
		t.Fatal("no object fills the ticket role")
	}

	row, err := specstore.Encode(o, Ticket{ID: "t1", KeyHash: "deadbeef"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if row["key_hash"] != "deadbeef" {
		t.Errorf("key_hash = %v, want the hash stored", row["key_hash"])
	}

	var back Ticket
	if err := specstore.Decode(o, row, &back); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back.KeyHash != "deadbeef" {
		t.Errorf("KeyHash = %q, want it read back", back.KeyHash)
	}
}

// Every declared column is written every time. A map missing a key means
// "leave it alone" to an update, so a field could never be cleared.
func TestEncodeWritesEveryDeclaredColumn(t *testing.T) {
	s := loadSpec(t)
	o, _ := s.ByRole(roleTicket)

	row, err := specstore.Encode(o, Ticket{ID: "t1"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for _, f := range o.Fields {
		if _, ok := row[f.Name]; !ok {
			t.Errorf("%q is declared but absent from the row", f.Name)
		}
	}
	if got, ok := row["answered_at"]; !ok || got != "" {
		t.Errorf("answered_at = %v, want an empty value present so it can be cleared", got)
	}
	if row["doc"] != nil {
		t.Errorf("doc = %v, want an empty document stored as NULL", row["doc"])
	}
}

// A declared column with no field to hold it, or a field with no column to
// land in, fails when the store is built rather than dropping a value on
// every write.
func TestNewRefusesASpecThatDisagreesWithTheType(t *testing.T) {
	db := openDB(t)

	undeclared := struct {
		ID      string
		Unknown string
	}{}
	_, err := specstore.New(db, loadSpec(t), map[string]any{roleTicket: undeclared})
	if err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Errorf("err = %v, want a refusal naming the undeclared column", err)
	}

	missing := struct{ ID string }{}
	_, err = specstore.New(db, loadSpec(t), map[string]any{roleTicket: missing})
	if err == nil || !strings.Contains(err.Error(), "does not carry") {
		t.Errorf("err = %v, want a refusal naming the column nothing carries", err)
	}
}

// A role the domain fills no object for stops the store being built, which is
// the moment at which stopping is cheap.
func TestNewRefusesAnUnfilledRole(t *testing.T) {
	_, err := specstore.New(openDB(t), loadSpec(t), map[string]any{"invoice": Ticket{}})
	if err == nil || !strings.Contains(err.Error(), "invoice") {
		t.Errorf("err = %v, want the unfilled role named", err)
	}
}

// The round trip through a real table is what proves the codec and the
// declared DDL agree: SQLite hands a boolean back as int64 and text as either
// a string or a byte slice, and a field that survives that has survived the
// only conversion this store does.
func TestStoreRoundTripsThroughARealTable(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()

	in := Ticket{
		ID:         specstore.NewID(),
		RaisedBy:   "amos",
		KeyHash:    "deadbeef",
		Doc:        `{"subject":"the door"}`,
		AnsweredAt: "2026-09-24T09:00:00Z",
		Replies:    2,
		Closed:     true,
	}
	if err := st.Insert(ctx, roleTicket, in); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got Ticket
	if err := st.Take(st.Query(ctx, roleTicket).Where("id = ?", in.ID), roleTicket, &got, errNoTicket); err != nil {
		t.Fatalf("take: %v", err)
	}
	if got != in {
		t.Errorf("read back\n  %+v\nwant\n  %+v", got, in)
	}

	all, err := specstore.List[Ticket](st, st.Query(ctx, roleTicket), roleTicket)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || all[0].ID != in.ID {
		t.Errorf("list = %+v, want the one ticket", all)
	}
}

// A read that matches nothing answers the caller's own sentinel, not a
// zero value the caller would have to test for.
func TestTakeAnswersTheCallersSentinel(t *testing.T) {
	st, _ := newStore(t)
	var got Ticket
	err := st.Take(st.Query(context.Background(), roleTicket).Where("id = ?", "nope"), roleTicket, &got, errNoTicket)
	if !errors.Is(err, errNoTicket) {
		t.Errorf("err = %v, want the caller's sentinel", err)
	}
}

// The table a role reads and writes is the spec's, which is what lets two
// domains mount the same module in one database.
func TestTableComesFromTheSpec(t *testing.T) {
	st, _ := newStore(t)
	if got := st.Table(roleTicket); got != "support_record_tickets" {
		t.Errorf("Table = %q, want the physical name the spec declares", got)
	}
}
