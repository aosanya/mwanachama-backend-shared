package models

import (
	"encoding/json"
	"testing"
)

// Attributes is optional — an org with no declared property set yet must
// not send a null/empty "attributes" key a caller could mistake for a real
// (empty) bag.
func TestSettingsAttributesOmittedWhenEmpty(t *testing.T) {
	s := Settings{Slug: "org1"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := got["attributes"]; ok {
		t.Errorf("unset attributes should be omitted: %s", b)
	}
	if _, ok := got["slug"]; !ok {
		t.Errorf("Settings missing required key %q: %s", "slug", b)
	}
}

func TestSettingsAttributesPresentWhenSet(t *testing.T) {
	s := Settings{
		Slug: "org1",
		Attributes: map[string]any{
			"display_name":  "Org One",
			"primary_color": "#123456",
			"accent_color":  "#abcdef",
			"logo_url":      "https://example.org/logo.png",
			"support_email": "support@example.org",
		},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Settings
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.DisplayName() != s.DisplayName() || got.LogoURL() != s.LogoURL() {
		t.Errorf("round-trip lost attributes: %s", b)
	}
}

// TestSettingsAccessorsReadDeclaredProperties pins the accessor methods to
// the JSON keys DefaultOrgSettingsProperties declares — a mismatch here
// means an accessor silently always returns "".
func TestSettingsAccessorsReadDeclaredProperties(t *testing.T) {
	s := Settings{Attributes: map[string]any{
		"display_name":            "d",
		"primary_color":           "p",
		"accent_color":            "a",
		"logo_url":                "l",
		"support_email":           "e",
		"default_dialling_region": "KE",
	}}
	cases := []struct {
		name string
		got  string
	}{
		{"display_name", s.DisplayName()},
		{"primary_color", s.PrimaryColor()},
		{"accent_color", s.AccentColor()},
		{"logo_url", s.LogoURL()},
		{"support_email", s.SupportEmail()},
		{"default_dialling_region", s.DefaultDiallingRegion()},
	}
	for _, c := range cases {
		if want := s.Attributes[c.name]; c.got != want {
			t.Errorf("Settings.%s-shaped accessor = %q, want %q", c.name, c.got, want)
		}
	}
}
