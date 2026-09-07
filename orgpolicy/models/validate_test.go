package models

import (
	"strings"
	"testing"
)

// The write door refuses an override that would not raise, so a stored
// number is never one Effective quietly out-votes.
func TestValidateOverrideOnlyAcceptsARaise(t *testing.T) {
	if err := ValidateOverride(5, 3); err != nil {
		t.Fatalf("5 above an organization cap of 3 is a raise: %v", err)
	}
	for _, n := range []int{3, 2, 0} {
		if err := ValidateOverride(n, 3); err == nil {
			t.Fatalf("ValidateOverride(%d, 3) should refuse, got nil", n)
		}
	}
	// The refusal names the organization's number, so an operator knows what
	// they are up against rather than guessing at a rejected form.
	err := ValidateOverride(2, 3)
	if err == nil || !strings.Contains(err.Error(), "3") {
		t.Fatalf("the refusal should name the organization's cap, got %v", err)
	}
}

func TestValidateCap(t *testing.T) {
	// Zero is allowed and is not "unset": an organization may decide no
	// address of anyone's is ever stored in the clear.
	if err := ValidateCap(0); err != nil {
		t.Fatalf("zero is a real setting: %v", err)
	}
	if err := ValidateCap(-1); err == nil {
		t.Fatal("a negative cap is not settable, got nil")
	}
}

// DEV-1612 · the free-text ceiling refuses zero for the membership cap's
// reason and not the address cap's: an answer box that may hold no
// characters is a broken survey, not a strict one.
func TestFreeTextCapFloorIsOne(t *testing.T) {
	if err := ValidateFreeTextMaxLengthCap(1); err != nil {
		t.Fatalf("ValidateFreeTextMaxLengthCap(1) = %v, want nil", err)
	}
	for _, n := range []int{0, -1} {
		if err := ValidateFreeTextMaxLengthCap(n); err == nil {
			t.Fatalf("ValidateFreeTextMaxLengthCap(%d) should refuse, got nil", n)
		}
	}
}

// A question's own number is checked against the organization's, and the
// refusal names both.
func TestValidateMaxLengthAgainstTheOrganizationsCeiling(t *testing.T) {
	if err := ValidateMaxLength(1000, 1000); err != nil {
		t.Fatalf("the ceiling itself is settable: %v", err)
	}
	if err := ValidateMaxLength(500, 1000); err != nil {
		t.Fatalf("ValidateMaxLength(500, 1000) = %v, want nil", err)
	}
	err := ValidateMaxLength(999999999, 1000)
	if err == nil {
		t.Fatal("ValidateMaxLength(999999999, 1000) should refuse, got nil")
	}
	if !strings.Contains(err.Error(), "1000") || !strings.Contains(err.Error(), "999999999") {
		t.Fatalf("the refusal should name both numbers, got %q", err)
	}
	if err := ValidateMaxLength(0, 1000); err == nil {
		t.Fatal("ValidateMaxLength(0, 1000) should refuse, got nil")
	}
}

// ValidateMembershipCap refuses zero, unlike ValidateCap — a chapter
// registration is not an exception to anything, it is what a member is.
func TestValidateMembershipCap(t *testing.T) {
	if err := ValidateMembershipCap(1); err != nil {
		t.Fatalf("one is the floor: %v", err)
	}
	if err := ValidateMembershipCap(0); err == nil {
		t.Fatal("zero should be refused here, got nil")
	}
}
