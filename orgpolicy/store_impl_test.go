package orgpolicy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy"
)

func TestStoreGetWithNoRowAnswersCompiledDefaults(t *testing.T) {
	s := newTestStore(t, nil)
	p, err := s.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.PublicAddressCap != 50 || p.ChapterMembershipCap != 5 || p.FreeTextMaxLengthCap != 1000 {
		t.Fatalf("expected compiled defaults, got %+v", p)
	}
}

func TestStoreSetThenGetRoundTrips(t *testing.T) {
	s := newTestStore(t, nil)
	ctx := context.Background()

	in := orgpolicy.Policy{PublicAddressCap: 10, ChapterMembershipCap: 3, FreeTextMaxLengthCap: 500, UpdatedBy: "op-1"}
	out, err := s.Set(ctx, in)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if out.PublicAddressCap != 10 || out.UpdatedBy != "op-1" {
		t.Fatalf("Set returned %+v", out)
	}

	got, err := s.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.PublicAddressCap != 10 || got.ChapterMembershipCap != 3 || got.FreeTextMaxLengthCap != 500 {
		t.Fatalf("Get after Set = %+v", got)
	}

	// Set upserts — a second Set replaces the row, not adds one.
	if _, err := s.Set(ctx, orgpolicy.Policy{PublicAddressCap: 20, ChapterMembershipCap: 3, FreeTextMaxLengthCap: 500}); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	got, err = s.Get(ctx)
	if err != nil || got.PublicAddressCap != 20 {
		t.Fatalf("expected upsert to replace public_address_cap, got %+v (err %v)", got, err)
	}
}

func TestStoreSetRejectsBadCaps(t *testing.T) {
	s := newTestStore(t, nil)
	ctx := context.Background()
	cases := []orgpolicy.Policy{
		{PublicAddressCap: -1, ChapterMembershipCap: 1, FreeTextMaxLengthCap: 1},
		{PublicAddressCap: 0, ChapterMembershipCap: 0, FreeTextMaxLengthCap: 1},
		{PublicAddressCap: 0, ChapterMembershipCap: 1, FreeTextMaxLengthCap: 0},
	}
	for _, c := range cases {
		if _, err := s.Set(ctx, c); !errors.Is(err, orgpolicy.ErrBadCap) {
			t.Errorf("Set(%+v) = %v, want ErrBadCap", c, err)
		}
	}
}

func TestOverridePutThenGet(t *testing.T) {
	s := newTestStore(t, alwaysExists)
	ctx := context.Background()

	if got, err := s.PublicAddressCapFor(ctx, "m1"); err != nil || got != nil {
		t.Fatalf("unnamed member should answer nil, got %v (err %v)", got, err)
	}

	five := 5
	if err := s.SetPublicAddressCapFor(ctx, "m1", &five); err != nil {
		t.Fatalf("SetPublicAddressCapFor: %v", err)
	}
	got, err := s.PublicAddressCapFor(ctx, "m1")
	if err != nil || got == nil || *got != 5 {
		t.Fatalf("expected 5, got %v (err %v)", got, err)
	}

	// A nil cap clears the override rather than storing "no raise" as a
	// value — the member goes back to moving with the organization's number.
	if err := s.SetPublicAddressCapFor(ctx, "m1", nil); err != nil {
		t.Fatalf("clearing: %v", err)
	}
	if got, err := s.PublicAddressCapFor(ctx, "m1"); err != nil || got != nil {
		t.Fatalf("expected the override to be cleared, got %v (err %v)", got, err)
	}
}

func TestOverrideRefusesAMemberWhoDoesNotExist(t *testing.T) {
	s := newTestStore(t, neverExists)
	ten := 10
	err := s.SetPublicAddressCapFor(context.Background(), "ghost", &ten)
	if !errors.Is(err, orgpolicy.ErrNoSuchMember) {
		t.Fatalf("expected ErrNoSuchMember, got %v", err)
	}
}
