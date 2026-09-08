package models

import "fmt"

// ValidateCap reports whether n is a number the organization's cap can be
// set to. Zero is allowed and is not the same as unset — an organization may
// decide no address of anyone's is ever stored in the clear.
func ValidateCap(n int) error {
	if n < 0 {
		return fmt.Errorf("a cap cannot be negative")
	}
	return nil
}

// ValidateOverride reports whether one member's individual cap can be set to
// n while the organization's cap is orgCap. An individual cap may only
// raise — refused rather than quietly clamped.
func ValidateOverride(n, orgCap int) error {
	if n < 0 {
		return fmt.Errorf("a cap cannot be negative")
	}
	if n <= orgCap {
		return fmt.Errorf(
			"an individual cap raises one member above the organization's %d — "+
				"set the organization's own cap to lower everybody", orgCap)
	}
	return nil
}

// ValidateMembershipCap reports whether n is a number the organization's
// structure-membership cap can be set to. Unlike ValidateCap, zero is refused:
// a structure registration is not an exception to anything, it is what a
// member is.
func ValidateMembershipCap(n int) error {
	if n < 1 {
		return fmt.Errorf(
			"a member has to be able to join at least one structure — "+
				"close enrollment by making structures undiscoverable, not with a cap of %d", n)
	}
	return nil
}

// ValidateFreeTextMaxLengthCap reports whether n is a number the
// organization's free-text ceiling can be set to. One is the floor: a
// free-text answer that may hold no characters is not a stricter survey, it
// is a broken one.
func ValidateFreeTextMaxLengthCap(n int) error {
	if n < 1 {
		return fmt.Errorf(
			"a free-text answer has to be able to hold at least one character — "+
				"stop asking free-text questions rather than capping them at %d", n)
	}
	return nil
}

// ValidateMaxLength reports whether one question's max_length is a number an
// author may set while the organization's ceiling is capValue. The ceiling
// is checked where a number is set and nowhere else — lowering it never
// reaches back into questions already written.
func ValidateMaxLength(n, capValue int) error {
	if n < 1 {
		return fmt.Errorf(
			"a max length of %d would leave the answer box unable to hold anything — "+
				"leave it unset for the organization's %d", n, capValue)
	}
	if n > capValue {
		return fmt.Errorf(
			"this organization caps a free-text answer at %d characters and this question asks for %d",
			capValue, n)
	}
	return nil
}
