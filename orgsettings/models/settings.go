// Package models holds orgsettings's domain types — plain Go, no GORM tags
// and no storage awareness, mirroring mwanachama-backend-actor's
// models/gormstore split.
package models

// Settings is the public branding/config record for one organization.
type Settings struct {
	Slug         string `json:"slug"`
	DisplayName  string `json:"display_name"`
	PrimaryColor string `json:"primary_color"`
	AccentColor  string `json:"accent_color"`
	LogoURL      string `json:"logo_url,omitempty"`
	SupportEmail string `json:"support_email,omitempty"`

	// DefaultDiallingRegion is the ISO-3166-1 alpha-2 code every
	// nationally-formatted phone number in this organization is canonicalized
	// against before it is hashed (G366, decided by DSN-1479 on 2026-08-22).
	//
	// It is the one field on this record that is not branding, and it is here
	// for a reason the type cannot state: its first consumer is the country
	// code chip at member/member-signin-phone.html:69, on an
	// **unauthenticated** screen, so it has to be readable before anyone has
	// signed in.
	//
	// Empty means **nobody has set one yet**, and it is never a stand-in for a
	// country. phone-salt.md's accepted trade-off is that a wrong region
	// mis-hashes every national number entered under it with nothing to
	// re-hash from, so phonenumber.Canonicalize refuses an empty region rather
	// than assuming one. A caller that defaults it has re-introduced exactly
	// the failure G366 was raised to close.
	DefaultDiallingRegion string `json:"default_dialling_region,omitempty"`
}
