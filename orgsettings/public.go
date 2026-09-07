package orgsettings

// PublicSettings is what an **unauthenticated** reader gets from this object,
// and it is a separate type on purpose (DEV-1284).
//
// # Why a type and not a comment
//
// `GET /v1/org-settings/{slug}` carries no session at all — router.go
// registers it with no capability wrapper, because a guest home and a
// sign-in screen have to carry the organization's mark before anybody has
// authenticated, which is what white-label means at the door.
//
// Until this type existed the handler returned the whole `Settings` row, and
// nothing leaked only because of which columns happened to exist. That is an
// accident of coverage, not a rule being honoured — RLS on the retired
// Supabase party plane (gone now; DSN-1349) kept `paybill` and
// `statement_format` behind the session; this plane has no RLS by design
// (authorization is Go, beside the handler), so the fence has to be a Go
// instrument or it is nothing.
//
// The property this buys: **adding a field to Settings does not widen the
// guest read.** A new column reaches a guest only if somebody adds it here
// too, which is a deliberate line in a diff rather than a silent consequence
// of a migration. TestPublicSettingsFieldsAreDeliberate fails when the two
// types drift, so the decision cannot be skipped by accident.
type PublicSettings struct {
	Slug         string `json:"slug"`
	DisplayName  string `json:"display_name"`
	PrimaryColor string `json:"primary_color"`
	AccentColor  string `json:"accent_color"`
	LogoURL      string `json:"logo_url,omitempty"`
	SupportEmail string `json:"support_email,omitempty"`

	// DefaultDiallingRegion is guest-visible deliberately, and it is the one
	// field here that is not branding.
	//
	// Its first consumer is the country-code chip at
	// `member/member-signin-phone.html:69`, which is on the sign-in screen —
	// so a region withheld until after authentication could not pre-fill the
	// screen that needs it. Publishing it is not an exposure: it is the
	// country the organization operates in, which its own branding already
	// announces.
	DefaultDiallingRegion string `json:"default_dialling_region,omitempty"`
}

// Public projects the stored record onto what a guest may see.
//
// Written as an explicit field list rather than by embedding or copying the
// struct, because the whole point is that a field has to be named here to
// cross the fence.
func (s Settings) Public() PublicSettings {
	return PublicSettings{
		Slug:                  s.Slug,
		DisplayName:           s.DisplayName,
		PrimaryColor:          s.PrimaryColor,
		AccentColor:           s.AccentColor,
		LogoURL:               s.LogoURL,
		SupportEmail:          s.SupportEmail,
		DefaultDiallingRegion: s.DefaultDiallingRegion,
	}
}
