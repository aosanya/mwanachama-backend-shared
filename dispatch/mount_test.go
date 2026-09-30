package dispatch_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func muxOf(routes []dispatch.Route) http.Handler {
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

func newTable(anonymous ...string) *dispatch.Table {
	return dispatch.NewTable("t", []byte(specJSON),
		map[string]error{"ErrMissing": errMissing, "ErrConflict": errConflict}, anonymous...)
}

func TestTableBuildsTheSameRoutesAsDispatch(t *testing.T) {
	built, err := newTable().Build(&manager{}, dispatch.Mount{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(built) != 4 {
		t.Fatalf("built %d routes, want 4", len(built))
	}
	for _, r := range built {
		if r.Action == "" {
			t.Errorf("%s %s carries no action", r.Method, r.Path)
		}
	}
}

func TestTableSplitsOnTheAllowlistAndNothingElse(t *testing.T) {
	table := newTable("t.thing.open")
	split := table.Split(&manager{}, dispatch.Mount{})

	if len(split.Anonymous) != 1 || split.Anonymous[0].Action != "t.thing.open" {
		t.Fatalf("anonymous = %+v, want only the named action", split.Anonymous)
	}
	if len(split.Gated) != 3 {
		t.Fatalf("gated = %d routes, want the other three", len(split.Gated))
	}
}

func TestATableWithAnEmptyAllowlistGatesEverything(t *testing.T) {
	split := newTable().Split(&manager{}, dispatch.Mount{})
	if len(split.Anonymous) != 0 {
		t.Fatalf("%d routes are anonymous with an empty allowlist", len(split.Anonymous))
	}
}

func TestTheGatedHalfCarriesTheAuthorizerAndThePublicHalfDoesNot(t *testing.T) {
	asked := map[string]int{}
	table := newTable("t.thing.open")
	split := table.Split(&manager{}, dispatch.Mount{
		Authorize: func(ctx context.Context, action string) error {
			asked[action]++
			return nil
		},
	})

	mux := muxOf(append(split.Anonymous, split.Gated...))
	do(t, mux, "GET", "/v1/things/one", "")
	if asked["t.thing.open"] != 0 {
		t.Fatalf("the anonymous route asked the authorizer %d times", asked["t.thing.open"])
	}
	do(t, mux, "GET", "/v1/things", "")
	if asked["t.thing.list"] != 1 {
		t.Fatalf("the gated route asked the authorizer %d times, want 1", asked["t.thing.list"])
	}
}

func TestATableRefusesAManagerMissingAMethod(t *testing.T) {
	if _, err := newTable().Build(struct{}{}, dispatch.Mount{}); err == nil {
		t.Fatal("a manager with none of the methods built without complaint")
	}
}

func TestAnonymousActionsIsACopy(t *testing.T) {
	table := newTable("t.thing.open")
	got := table.AnonymousActions()
	got[0] = "t.thing.rewritten"
	if table.AnonymousActions()[0] != "t.thing.open" {
		t.Fatal("a caller rewrote the table's own allowlist")
	}
}

func TestUnmappedSentinelsNamesBothDirections(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	clean := dispatch.UnmappedSentinels(s, map[string]error{"ErrMissing": errMissing, "ErrConflict": errConflict})
	if len(clean) != 0 {
		t.Fatalf("a spec that maps every supplied sentinel reported %v", clean)
	}

	short := dispatch.UnmappedSentinels(s, map[string]error{
		"ErrMissing": errMissing, "ErrConflict": errConflict, "ErrStray": errMissing,
	})
	if len(short) != 1 {
		t.Fatalf("an unmapped sentinel reported %v, want exactly one problem", short)
	}
}

func TestUnmappedSentinelsReadsExportedVars(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

import "errors"

var ErrMapped = errors.New("mapped")

var ErrForgotten = errors.New("forgotten")

var ErrNotASentinel = "not an error"
`
	if err := os.WriteFile(filepath.Join(dir, "errors.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := dispatch.Parse([]byte(`{
      "operations": {"one": {"method":"GET","path":"/one","call":"One","action":"t.one.read",
                     "returns":[{"body":true}]}},
      "errors": {"ErrMapped": 404}
    }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	problems := dispatch.UnmappedSentinels(s, nil, dir)
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want only ErrForgotten", problems)
	}
	if !strings.Contains(problems[0], "ErrForgotten") {
		t.Fatalf("problem = %q, want it to name ErrForgotten", problems[0])
	}
}
