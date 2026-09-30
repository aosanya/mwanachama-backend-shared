package dispatch_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var errGone = errors.New("gone")

const twoOps = `{"operations":{
  "open_thing":{"method":"GET","path":"/things/{slug}","call":"Open","action":"m.thing.open",
    "args":[{"from":"path","as":"slug"},{"from":"query","as":"k"}],"returns":[{"body":true}]},
  "retire_thing":{"method":"DELETE","path":"/things/{id}","call":"Retire","action":"m.thing.retire",
    "status":204,"args":[{"from":"path","as":"id"}],"returns":[{"body":true}]}},
  "errors":{"errGone":404}}`

func newTable() *dispatch.Table {
	return dispatch.NewTable([]byte(twoOps), map[string]error{"errGone": errGone}, "m.thing.open")
}

func TestATableBuildsTheSameRoutesTheLadderDid(t *testing.T) {
	routes := newTable().Routes(&manager{}, dispatch.Mount{})
	if len(routes) != 2 {
		t.Fatalf("built %d routes, want 2", len(routes))
	}
}

func TestSplitSeparatesTheNamedActionsFromEveryOtherOne(t *testing.T) {
	split := newTable().Split(&manager{}, dispatch.Mount{})
	if len(split.Anonymous) != 1 || split.Anonymous[0].Action != "m.thing.open" {
		t.Fatalf("anonymous = %v, want just the named action", split.Anonymous)
	}
	if len(split.Gated) != 1 || split.Gated[0].Action != "m.thing.retire" {
		t.Fatalf("gated = %v, want everything not named", split.Gated)
	}
}

func TestTheAnonymousHalfIsNotWrappedInTheAuthorizer(t *testing.T) {
	asked := map[string]bool{}
	split := newTable().Split(&manager{}, dispatch.Mount{
		Authorize: func(ctx context.Context, action string) error {
			asked[action] = true
			return dispatch.ErrForbidden
		},
	})
	for _, r := range split.Anonymous {
		req := httptest.NewRequest(http.MethodGet, "/things/x", nil)
		req.SetPathValue("slug", "x")
		r.Handler(httptest.NewRecorder(), req)
	}
	if asked["m.thing.open"] {
		t.Error("the anonymous half asked the authorizer, so a public route is gated after all")
	}

	for _, r := range split.Gated {
		req := httptest.NewRequest(http.MethodDelete, "/things/x", nil)
		req.SetPathValue("id", "x")
		r.Handler(httptest.NewRecorder(), req)
	}
	if !asked["m.thing.retire"] {
		t.Error("the gated half did not ask the authorizer")
	}
}

func TestAnActionNamedAnonymousThatNoOperationDeclaresIsReported(t *testing.T) {
	table := dispatch.NewTable([]byte(twoOps), map[string]error{"errGone": errGone},
		"m.thing.open", "m.thing.renamed_away")
	unknown, err := table.UnknownAnonymousActions()
	if err != nil {
		t.Fatalf("UnknownAnonymousActions: %v", err)
	}
	if len(unknown) != 1 || unknown[0] != "m.thing.renamed_away" {
		t.Fatalf("unknown = %v, want the action nothing declares", unknown)
	}
}

func TestASuppliedSentinelTheSpecNeverMapsIsReported(t *testing.T) {
	table := dispatch.NewTable([]byte(twoOps), map[string]error{
		"errGone": errGone, "errUnmapped": errors.New("unmapped"),
	}, "m.thing.open")
	missing, err := table.UnmappedSentinels()
	if err != nil {
		t.Fatalf("UnmappedSentinels: %v", err)
	}
	if len(missing) != 1 || missing[0] != "errUnmapped" {
		t.Fatalf("missing = %v, want the sentinel that would redact to a 500", missing)
	}
}
