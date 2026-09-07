package models

import (
	"encoding/json"
	"testing"
)

// LogoURL, SupportEmail and Attributes are optional — an org that hasn't set
// a logo, support address or any declared property yet must not send
// empty/null keys the client shell would render as a broken image / mailto
// link, or a caller mistake for a real (empty) attribute bag.
func TestSettingsOptionalFieldsOmittedWhenEmpty(t *testing.T) {
	s := Settings{Slug: "org1", DisplayName: "Org One", PrimaryColor: "#123456", AccentColor: "#abcdef"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"logo_url", "support_email", "attributes"} {
		if _, ok := got[key]; ok {
			t.Errorf("unset %q should be omitted: %s", key, b)
		}
	}
	for _, key := range []string{"slug", "display_name", "primary_color", "accent_color"} {
		if _, ok := got[key]; !ok {
			t.Errorf("Settings missing required key %q: %s", key, b)
		}
	}
}

func TestSettingsOptionalFieldsPresentWhenSet(t *testing.T) {
	s := Settings{
		Slug:         "org1",
		DisplayName:  "Org One",
		PrimaryColor: "#123456",
		AccentColor:  "#abcdef",
		LogoURL:      "https://example.org/logo.png",
		SupportEmail: "support@example.org",
		Attributes:   map[string]any{"future_flag": true},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["logo_url"] != s.LogoURL || got["support_email"] != s.SupportEmail {
		t.Errorf("round-trip lost logo_url/support_email: %s", b)
	}
	attrs, ok := got["attributes"].(map[string]any)
	if !ok || attrs["future_flag"] != true {
		t.Errorf("round-trip lost attributes: %s", b)
	}
}
