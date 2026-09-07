package gormstore

// OverrideRow is one member's individual public-address-cap raise. Its own
// table (org_policy_overrides), not a column on any member table — the
// gateway's original org_policy_store.go wrote this onto the gateway's own
// (now-retired) `member` table directly; that column has no equivalent on
// mwanachama-backend-actor's member_actors, and this package must not
// depend on actor to reach one, so the override gets a home of its own
// (DEV-1683 follow-up). Member existence is checked via a caller-supplied
// callback (see the root package's MemberExists), the same
// depend-on-a-function-not-a-store pattern the gateway's own
// MemberExistence already used.
type OverrideRow struct {
	MemberID         string `gorm:"primaryKey"`
	PublicAddressCap int
}
