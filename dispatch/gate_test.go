package dispatch_test

import (
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func routesFor(actions ...string) []dispatch.Route {
	out := make([]dispatch.Route, 0, len(actions))
	for _, a := range actions {
		out = append(out, dispatch.Route{Method: http.MethodGet, Path: "/" + a, Action: a})
	}
	return out
}

func actionsOf(routes []dispatch.Route) []string {
	out := make([]string, 0, len(routes))
	for _, r := range routes {
		out = append(out, r.Action)
	}
	return out
}

func TestAnonymousKeepsOnlyWhatIsNamed(t *testing.T) {
	split := dispatch.Anonymous(routesFor("t.entry.open", "t.entry.list", "t.entry.delete"), "t.entry.open")

	if got := actionsOf(split.Anonymous); len(got) != 1 || got[0] != "t.entry.open" {
		t.Fatalf("anonymous = %v, want just t.entry.open", got)
	}
	if got := actionsOf(split.Gated); len(got) != 2 {
		t.Fatalf("gated = %v, want the other two", got)
	}
}

func TestAnUnrecognisedActionIsGated(t *testing.T) {
	routes := routesFor("t.entry.open", "t.entry.brand_new")
	split := dispatch.Anonymous(routes, "t.entry.open")

	for _, r := range split.Anonymous {
		if r.Action == "t.entry.brand_new" {
			t.Fatal("an action absent from the allowlist was published anonymously")
		}
	}
	if len(split.Gated) != 1 || split.Gated[0].Action != "t.entry.brand_new" {
		t.Fatalf("gated = %v, want the unrecognised action", actionsOf(split.Gated))
	}
}

func TestARouteWithNoActionIsGated(t *testing.T) {
	routes := []dispatch.Route{{Method: http.MethodGet, Path: "/loose"}}

	split := dispatch.Anonymous(routes, "")
	if len(split.Anonymous) != 0 {
		t.Fatalf("a route with no action was published anonymously")
	}
	if len(split.Gated) != 1 {
		t.Fatalf("gated = %d routes, want 1", len(split.Gated))
	}
}

func TestEmptyAllowlistGatesEverything(t *testing.T) {
	routes := routesFor("t.entry.open", "t.entry.list")

	split := dispatch.Anonymous(routes)
	if len(split.Anonymous) != 0 {
		t.Fatalf("anonymous = %v, want nothing", actionsOf(split.Anonymous))
	}
	if len(split.Gated) != 2 {
		t.Fatalf("gated = %d, want 2", len(split.Gated))
	}
}

func TestUnmatchedNamesTheAllowlistEntriesThatMatchNothing(t *testing.T) {
	routes := routesFor("t.entry.open")

	var split dispatch.Split
	got := split.Unmatched(routes, []string{"t.entry.open", "t.entry.opne"})
	if len(got) != 1 || got[0] != "t.entry.opne" {
		t.Fatalf("unmatched = %v, want the misspelled entry", got)
	}
}
