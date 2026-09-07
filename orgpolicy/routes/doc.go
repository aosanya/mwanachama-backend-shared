// Package routes is orgpolicy's own HTTP surface over Get/Set: decode a
// request, call one Repository method, encode the response — mirroring
// mwanachama-backend-actor's routes/ and this repo's own orgsettings/routes
// exactly.
//
// Scope: GetOrgPolicy/PutOrgPolicy only — the organization-wide Policy
// row. Deliberately NOT here: the per-member individual-cap endpoints
// (GET/PUT /v1/members/{memberID}/public-address-cap) and the chapter-cap
// read (GET /v1/members/{memberID}/chapter-cap), even though each is a
// thin wrapper over this package's Repository. Each composes with a domain
// this package must not depend on — member visibility fencing
// (requireMemberVisible/requireContactRead), the address count
// (d.Addresses.CountPublic), and the registration list
// (d.Members.ListForMember) — so they stay gateway-side, calling this
// package's Repository directly, the same way the role/custody act-log
// composition stays in the gateway rather than being pulled into an
// extracted package (see mwanachama-backend-actor's own routes/doc.go for
// the identical reasoning about its excluded five).
//
// A route built from this package still needs a caller-identity/capability
// gate wrapped around it before it is safe to serve, in both directions —
// unlike orgsettings' guest-readable GET, org-policy answers exactly how
// much plaintext an organization tolerates, which is not a fact for an
// unauthenticated caller. The mounting process supplies that gate by
// wrapping the http.HandlerFunc this package returns, not by this package
// reaching for a session or a capability itself.
package routes
