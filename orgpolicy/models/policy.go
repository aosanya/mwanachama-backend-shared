// Package models holds orgpolicy's domain types and pure validation logic —
// plain Go, no GORM tags and no storage awareness, mirroring
// mwanachama-backend-actor's models/gormstore split (and orgsettings's,
// this repo's own sibling).
package models

import "time"

// DefaultPublicAddressCap is the number a gateway uses when it has no policy
// row at all — a plane whose migrations have not run.
//
// **Fifty, by owner decision (2026-08-26).** Every address except a
// published one is a keyed hash; this number is the whole of what keeps the
// exception an exception.
const DefaultPublicAddressCap = 50

// DefaultStructureMembershipCap is how many chapters one member may be
// registered at when the gateway has no policy row at all.
//
// **Five, by owner decision (2026-08-27).**
const DefaultStructureMembershipCap = 5

// DefaultFreeTextMaxLengthCap is the ceiling a gateway uses when it has no
// policy row at all.
//
// **A thousand (DEV-1612).**
const DefaultFreeTextMaxLengthCap = 1000

// Policy is the organization's settable numbers.
type Policy struct {
	// PublicAddressCap is how many of a member's addresses may be stored as
	// plaintext, before any individual raise. The unit is rows holding a
	// plaintext, not reachable addresses — a retired public address still
	// holds one and still counts.
	PublicAddressCap int `json:"public_address_cap"`

	// StructureMembershipCap is how many chapters one member may hold a live
	// registration at, home chapter included. The unit is registrations the
	// member holds, not chapters they are counted in — derived ancestor
	// membership spends nothing against this number.
	StructureMembershipCap int `json:"structure_membership_cap"`

	// FreeTextMaxLengthCap is the largest max_length an author may set on a
	// free-text question, and the length every free-text answer in this
	// organization is held to when a question omits one.
	FreeTextMaxLengthCap int `json:"free_text_max_length_cap"`

	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by,omitempty"`
}

// Effective is the cap that actually applies to one member: the
// organization's number, raised by an individual override where one has
// been granted.
//
// An override raises and never lowers — the write door (ValidateOverride)
// refuses one that would not raise at the time it is set, and this function
// guarantees it keeps raising afterwards even as the organization's own
// number moves. nil override means the member has never been named and
// moves with the organization.
func Effective(orgCap int, override *int) int {
	if override != nil && *override > orgCap {
		return *override
	}
	return orgCap
}
