package models

import "testing"

func ptr(n int) *int { return &n }

// **An individual cap raises and never lowers**, which is the owner's ask
// taken literally: *"a member may have the cap increased individually."*
func TestEffectiveRaisesAndNeverLowers(t *testing.T) {
	cases := []struct {
		name     string
		org      int
		override *int
		want     int
	}{
		{"nobody named moves with the organization", 3, nil, 3},
		{"a raise applies", 1, ptr(5), 5},
		{
			// The case that decides the design. An organization that raises
			// its cap from 1 to 5 must not leave yesterday's
			// individually-raised member behind at 3.
			"an organization that overtakes an old raise carries the member with it",
			5, ptr(3), 5,
		},
		{"a cap of zero is a real setting", 0, nil, 0},
		{"and can still be raised for one member", 0, ptr(2), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Effective(c.org, c.override); got != c.want {
				t.Fatalf("Effective(%d, %v) = %d, want %d", c.org, c.override, got, c.want)
			}
		})
	}
}
