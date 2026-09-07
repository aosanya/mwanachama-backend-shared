package orgchrome

import (
	"encoding/json"
	"testing"
)

// LogoURL and SupportEmail are optional — an org that hasn't set a logo or
// support address yet must not send empty-string keys the client shell
// would render as a broken image / mailto link.
func TestChromeOptionalFieldsOmittedWhenEmpty(t *testing.T) {
	c := Chrome{Slug: "org1", DisplayName: "Org One", PrimaryColor: "#123456", AccentColor: "#abcdef"}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"logo_url", "support_email"} {
		if _, ok := got[key]; ok {
			t.Errorf("unset %q should be omitted: %s", key, b)
		}
	}
	for _, key := range []string{"slug", "display_name", "primary_color", "accent_color"} {
		if _, ok := got[key]; !ok {
			t.Errorf("Chrome missing required key %q: %s", key, b)
		}
	}
}

func TestChromeOptionalFieldsPresentWhenSet(t *testing.T) {
	c := Chrome{
		Slug:         "org1",
		DisplayName:  "Org One",
		PrimaryColor: "#123456",
		AccentColor:  "#abcdef",
		LogoURL:      "https://example.org/logo.png",
		SupportEmail: "support@example.org",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Chrome
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != c {
		t.Errorf("round-trip = %+v, want %+v", got, c)
	}
}
