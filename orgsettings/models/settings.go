// Package models holds orgsettings's domain types — plain Go, no GORM tags
// and no storage awareness, mirroring mwanachama-backend-actor's
// models/gormstore split.
package models

// Settings is the public branding/config record for one organization. Slug
// is the only structural field — its primary key, and therefore the
// party's identity; every other value is a declared attribute (see
// DefaultOrgSettingsProperties), matching how
// mwanachama-backend-actor/models.Actor keeps its extensible values in
// Attributes rather than as dedicated columns.
type Settings struct {
	Slug string `json:"slug"`

	// Attributes is this record's extension point — organization-declared
	// properties (DefaultOrgSettingsProperties, or a longer
	// organization-supplied list), JSONB-backed, validated by
	// ValidateAttributes before every Put. Every branding value
	// (display_name, primary_color, accent_color, logo_url, support_email)
	// and default_dialling_region live here rather than as named fields
	// (DEV-1683 follow-up) — see the accessor methods below for reading
	// them, and public.go's Public() for how the guest-visible subset
	// crosses the fence explicitly by name despite living in this generic
	// bag.
	Attributes map[string]any `json:"attributes,omitempty"`
}

// stringAttr reads a string attribute, or "" when absent or not a string —
// a malformed value reads the same as unset rather than panicking, since
// ValidateAttributes is what is supposed to have refused it before it ever
// reached storage.
func (s Settings) stringAttr(name string) string {
	v, _ := s.Attributes[name].(string)
	return v
}

// DisplayName is the organization's official name, e.g. "Umoja Movement".
func (s Settings) DisplayName() string { return s.stringAttr("display_name") }

// PrimaryColor is the brand hex authored by the organization.
func (s Settings) PrimaryColor() string { return s.stringAttr("primary_color") }

// AccentColor is the secondary brand hex.
func (s Settings) AccentColor() string { return s.stringAttr("accent_color") }

// LogoURL is the full logo URL (this plane serves a URL, not a Storage path).
func (s Settings) LogoURL() string { return s.stringAttr("logo_url") }

// SupportEmail is the member-facing support address. Empty means the party
// published none.
func (s Settings) SupportEmail() string { return s.stringAttr("support_email") }

// DefaultDiallingRegion is the ISO-3166-1 alpha-2 code every
// nationally-formatted phone number in this organization is canonicalized
// against before it is hashed (G366, decided by DSN-1479 on 2026-08-22).
//
// It is the one attribute here that is not branding, and it is here for a
// reason the type cannot state: its first consumer is the country code chip
// at member/member-signin-phone.html:69, on an **unauthenticated** screen,
// so it has to be readable before anyone has signed in.
//
// Empty means **nobody has set one yet**, and it is never a stand-in for a
// country. phone-salt.md's accepted trade-off is that a wrong region
// mis-hashes every national number entered under it with nothing to
// re-hash from, so phonenumber.Canonicalize refuses an empty region rather
// than assuming one. A caller that defaults it has re-introduced exactly
// the failure G366 was raised to close.
func (s Settings) DefaultDiallingRegion() string { return s.stringAttr("default_dialling_region") }
