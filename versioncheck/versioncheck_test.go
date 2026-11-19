package versioncheck

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestStale(t *testing.T) {
	updatedAt := time.Date(2026, 11, 16, 9, 30, 15, 123456789, time.UTC)
	fields := []Field{
		{Name: "title", Sent: "a", Stored: "a"},
		{Name: "body", Sent: "old", Stored: "new"},
		{Name: "due", Sent: "x", Stored: "y"},
	}
	same := []Field{
		{Name: "title", Sent: "a", Stored: "a"},
		{Name: "body", Sent: "b", Stored: "b"},
	}

	cases := []struct {
		name      string
		version   string
		fields    []Field
		wantField string
		wantStale bool
	}{
		{"empty version accepted", "", fields, "", false},
		{"current nano form accepted", updatedAt.Format(time.RFC3339Nano), fields, "", false},
		{"current second form accepted", updatedAt.Format(time.RFC3339), fields, "", false},
		{"current in another zone accepted", updatedAt.In(time.FixedZone("x", 3600)).Format(time.RFC3339Nano), fields, "", false},
		{"old names the differing field", updatedAt.Add(-time.Minute).Format(time.RFC3339Nano), fields, "body", true},
		{"old with no differing field names the first", updatedAt.Add(-time.Minute).Format(time.RFC3339), same, "title", true},
		{"newer than stored is stale", updatedAt.Add(time.Minute).Format(time.RFC3339), fields, "body", true},
		{"unparsable is stale", "yesterday", fields, "body", true},
		{"unparsable with empty list names record", "yesterday", nil, "record", true},
		{"old with empty list names record", updatedAt.Add(-time.Hour).Format(time.RFC3339), []Field{}, "record", true},
		{"empty list with current version accepted", updatedAt.Format(time.RFC3339), nil, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Stale(c.version, updatedAt, c.fields)
			stale, ok := IsStale(err)
			if ok != c.wantStale {
				t.Fatalf("IsStale = %v, want %v (err %v)", ok, c.wantStale, err)
			}
			if !c.wantStale {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if stale.Field != c.wantField {
				t.Fatalf("Field = %q, want %q", stale.Field, c.wantField)
			}
			want := c.wantField + " changed since you read it"
			if err.Error() != want {
				t.Fatalf("Error() = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestIsStaleSeesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("update: %w", &StaleError{Field: "body"})
	stale, ok := IsStale(wrapped)
	if !ok || stale.Field != "body" {
		t.Fatalf("IsStale = %v, %v", stale, ok)
	}
}

func TestIsStaleRefusesOtherErrors(t *testing.T) {
	if stale, ok := IsStale(errors.New("boom")); ok || stale != nil {
		t.Fatalf("IsStale = %v, %v", stale, ok)
	}
	if stale, ok := IsStale(nil); ok || stale != nil {
		t.Fatalf("IsStale(nil) = %v, %v", stale, ok)
	}
}
