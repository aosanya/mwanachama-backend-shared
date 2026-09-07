// Package orgpolicy holds the organization-wide numbers an operator sets and
// every other domain reads — how many of a member's addresses the shared
// database may hold in the clear (DEV-1563), how many chapters one member
// may be registered at (DEV-1604), and how long a free-text survey answer
// may run (DEV-1612) — plus the per-member raise on the first of those.
//
// Structured like mwanachama-backend-actor (and this repo's own
// orgsettings): domain types and pure validation in models/, GORM
// row/migration in gormstore/, and an HTTP surface in routes/, with this
// root package holding the Repository interface and its implementation.
//
// **Unlike orgsettings, this is not public.** How much plaintext an
// organization is willing to hold is exactly the fact somebody sizing up a
// stolen dump would like for free — every route routes/ builds is
// capability-gated in both directions, and the mounting process supplies
// that gate (see routes/doc.go).
package orgpolicy

import (
	"context"
	"errors"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy/models"
)

// ErrNoSuchMember is returned when an individual cap names nobody. Distinct
// from a member who simply holds no raise: naming the wrong member must not
// look like a write that succeeded.
var ErrNoSuchMember = errors.New("orgpolicy: no such member")

// ErrBadCap is returned when a cap is not a number an organization can be
// set to, or when an individual cap would lower a member below everyone
// else. Wraps the specific models.Validate* detail, mirroring
// orgsettings.ErrInvalidSettings's shape.
var ErrBadCap = errors.New("orgpolicy: cap is not settable to that")

// Policy is an alias of its models. counterpart, and the functions below
// forward to their models. namesakes, so a caller needs only this package's
// import, never models's directly — mirrors orgsettings's identical
// convenience aliases.
type Policy = models.Policy

// Effective forwards to [models.Effective].
func Effective(orgCap int, override *int) int { return models.Effective(orgCap, override) }

// ValidateCap forwards to [models.ValidateCap].
func ValidateCap(n int) error { return models.ValidateCap(n) }

// ValidateOverride forwards to [models.ValidateOverride].
func ValidateOverride(n, orgCap int) error { return models.ValidateOverride(n, orgCap) }

// ValidateMembershipCap forwards to [models.ValidateMembershipCap].
func ValidateMembershipCap(n int) error { return models.ValidateMembershipCap(n) }

// ValidateFreeTextMaxLengthCap forwards to [models.ValidateFreeTextMaxLengthCap].
func ValidateFreeTextMaxLengthCap(n int) error { return models.ValidateFreeTextMaxLengthCap(n) }

// ValidateMaxLength forwards to [models.ValidateMaxLength].
func ValidateMaxLength(n, capValue int) error { return models.ValidateMaxLength(n, capValue) }

// Default* forward to their models. namesakes.
const (
	DefaultPublicAddressCap     = models.DefaultPublicAddressCap
	DefaultChapterMembershipCap = models.DefaultChapterMembershipCap
	DefaultFreeTextMaxLengthCap = models.DefaultFreeTextMaxLengthCap
)

// MemberExists is the one thing this package needs from the member domain:
// whether an individual cap names anybody. Kept as a function rather than a
// store reference so this package does not become a second place that
// decides who exists, and so it never has to import
// mwanachama-backend-actor — mirrors the gateway's own original
// MemberExistence exactly.
type MemberExists func(ctx context.Context, memberID string) bool

// Repository stores the organization's policy and the individual raises.
type Repository interface {
	// Get returns the organization's policy. A plane with no row answers
	// the compiled defaults rather than an error: a missing policy is a
	// migration state, and every caller inventing its own fallback is how
	// two numbers for one policy start.
	Get(ctx context.Context) (Policy, error)

	// Set replaces the organization's policy and records who did it.
	// Refuses with ErrBadCap when a number fails its models.Validate*
	// check.
	Set(ctx context.Context, p Policy) (Policy, error)

	// PublicAddressCapFor returns one member's individual raise, or nil
	// where they have never been named.
	PublicAddressCapFor(ctx context.Context, memberID string) (*int, error)

	// SetPublicAddressCapFor grants or clears one member's individual
	// raise. A nil cap clears it, putting the member back on the
	// organization's number. Refuses with ErrNoSuchMember when memberID
	// names nobody, per this Store's MemberExists.
	SetPublicAddressCapFor(ctx context.Context, memberID string, cap *int) error
}
