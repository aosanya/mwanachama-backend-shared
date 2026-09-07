package orgchrome

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// publicFields is the guest-visible column list, written out once.
//
// This slice IS the fence (DEV-1284). Adding a name to it is the deliberate
// act of publishing a column to unauthenticated readers, and the tests below
// are what make it deliberate rather than incidental.
var publicFields = []string{
	"slug",
	"display_name",
	"primary_color",
	"accent_color",
	"logo_url",
	"support_email",
	"default_dialling_region",
}

func jsonNames(t *testing.T, v any) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	out := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		for j := 0; j < len(tag); j++ {
			if tag[j] == ',' {
				tag = tag[:j]
				break
			}
		}
		if tag == "" || tag == "-" {
			continue
		}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

// TestPublicChromeFieldsAreDeliberate pins the guest projection to an explicit
// list. It fails when somebody adds a field to PublicChrome without adding it
// here — which is the moment a reviewer should be asked whether a guest may
// read that column.
func TestPublicChromeFieldsAreDeliberate(t *testing.T) {
	want := append([]string(nil), publicFields...)
	sort.Strings(want)
	got := jsonNames(t, PublicChrome{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the guest projection changed.\n got: %v\nwant: %v\n\nIf this is intended, add the column to publicFields and say in the diff why an unauthenticated reader may see it.", got, want)
	}
}

// TestANewChromeColumnDoesNotReachGuestsByDefault is the property DEV-1284 was
// filed for, and it is the one that actually protects anything.
//
// Before the split, `GET /v1/org-chrome/{slug}` returned the whole row, so a
// migration adding `paybill` would have served it to unauthenticated readers
// with no code change and no review. This test asserts the two types are
// allowed to differ, and that Chrome having a field PublicChrome lacks is the
// normal, safe state rather than a bug.
//
// It is written as a positive assertion about the *mechanism* — the projection
// is an explicit field list — because a test that merely compared the two
// types would have to be deleted the first time they legitimately diverged,
// which is exactly when the fence starts mattering.
func TestANewChromeColumnDoesNotReachGuestsByDefault(t *testing.T) {
	full := Chrome{
		Slug:                  "acme",
		DisplayName:           "Acme",
		PrimaryColor:          "#111",
		AccentColor:           "#222",
		LogoURL:               "l",
		SupportEmail:          "s@a",
		DefaultDiallingRegion: "KE",
	}

	blob, err := json.Marshal(full.Public())
	if err != nil {
		t.Fatalf("marshalling the public projection: %v", err)
	}
	var served map[string]any
	if err := json.Unmarshal(blob, &served); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	allowed := map[string]bool{}
	for _, f := range publicFields {
		allowed[f] = true
	}
	for k := range served {
		if !allowed[k] {
			t.Errorf("the guest read served %q, which is not on the fence", k)
		}
	}

	// Every allowed field that was set must actually survive the projection —
	// a fence that dropped a column the screens need would be caught here
	// rather than as a blank organization name in a browser.
	for _, f := range publicFields {
		if _, ok := served[f]; !ok {
			t.Errorf("the guest read dropped %q, which the fence admits", f)
		}
	}
}

// TestPublicProjectionCarriesTheValues guards against a Public() that returns
// the right shape with the wrong contents — a copy-paste slip in an explicit
// field list is easy and silent.
func TestPublicProjectionCarriesTheValues(t *testing.T) {
	in := Chrome{
		Slug: "s", DisplayName: "d", PrimaryColor: "p", AccentColor: "a",
		LogoURL: "l", SupportEmail: "e", DefaultDiallingRegion: "KE",
	}
	got := in.Public()
	want := PublicChrome{
		Slug: "s", DisplayName: "d", PrimaryColor: "p", AccentColor: "a",
		LogoURL: "l", SupportEmail: "e", DefaultDiallingRegion: "KE",
	}
	if got != want {
		t.Errorf("Public() = %+v, want %+v", got, want)
	}
}
