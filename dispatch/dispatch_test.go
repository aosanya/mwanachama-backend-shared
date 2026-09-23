package dispatch_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var errMissing = errors.New("no such thing")

var errConflict = errors.New("already decided")

type thing struct {
	Slug   string `json:"slug"`
	Secret string `json:"-"`
}

type manager struct {
	opened   string
	key      string
	statuses []string
}

func (m *manager) Open(ctx context.Context, slug, k string) (thing, error) {
	m.opened, m.key = slug, k
	if slug == "missing" {
		return thing{}, errMissing
	}
	return thing{Slug: slug, Secret: "never"}, nil
}

func (m *manager) List(ctx context.Context, statuses ...string) ([]thing, error) {
	m.statuses = statuses
	return []thing{{Slug: "one"}}, nil
}

func (m *manager) Issue(ctx context.Context, slug, label string) (thing, string, error) {
	if slug == "taken" {
		return thing{}, "", errConflict
	}
	return thing{Slug: slug}, "raw-" + label, nil
}

func (m *manager) Retire(ctx context.Context, id string) error {
	if id == "missing" {
		return errMissing
	}
	return nil
}

const specJSON = `{
  "base": "/v1",
  "operations": {
    "open":   {"method":"GET","path":"/things/{slug}","call":"Open","action":"t.thing.open",
               "args":[{"from":"path","as":"slug"},{"from":"query","as":"k"}],
               "returns":[{"body":true}]},
    "list":   {"method":"GET","path":"/things","call":"List","action":"t.thing.list",
               "args":[{"from":"query","as":"status","repeated":true}],
               "returns":[{"body":true}]},
    "issue":  {"method":"POST","path":"/things/{slug}/links","call":"Issue","action":"t.link.issue","status":201,
               "args":[{"from":"path","as":"slug"},{"from":"body","as":"label"}],
               "returns":[{"as":"link"},{"as":"key","once":true}]},
    "retire": {"method":"POST","path":"/links/{id}/retire","call":"Retire","action":"t.link.retire","status":204,
               "args":[{"from":"path","as":"id"}],
               "returns":[{"body":true}]}
  },
  "errors": {"ErrMissing": 404, "ErrConflict": 409}
}`

func build(t *testing.T) (http.Handler, *manager, []dispatch.Route) {
	t.Helper()
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &manager{}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing, "ErrConflict": errConflict},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux, m, routes
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestBindsPathAndQuery(t *testing.T) {
	h, m, _ := build(t)

	rec := do(t, h, http.MethodGet, "/v1/things/alpha?k=opensesame", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.opened != "alpha" || m.key != "opensesame" {
		t.Fatalf("bound (%q, %q), want (alpha, opensesame)", m.opened, m.key)
	}
}

func TestBindsARepeatedQueryToAVariadic(t *testing.T) {
	h, m, _ := build(t)

	if rec := do(t, h, http.MethodGet, "/v1/things?status=a&status=b", ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(m.statuses) != 2 || m.statuses[0] != "a" || m.statuses[1] != "b" {
		t.Fatalf("statuses = %v, want [a b]", m.statuses)
	}
}

func TestNamedReturnsBecomeAnObject(t *testing.T) {
	h, _, _ := build(t)

	rec := do(t, h, http.MethodPost, "/v1/things/alpha/links", `{"label":"pilot"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Link thing  `json:"link"`
		Key  string `json:"key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Link.Slug != "alpha" || got.Key != "raw-pilot" {
		t.Fatalf("got %+v, want link.slug=alpha and key=raw-pilot", got)
	}
}

func TestRenderingHonoursTheTypesOwnJSONTags(t *testing.T) {
	h, _, _ := build(t)

	rec := do(t, h, http.MethodGet, "/v1/things/alpha", "")
	if strings.Contains(rec.Body.String(), "never") {
		t.Errorf("a json:\"-\" field reached the response: %s", rec.Body.String())
	}
}

func TestSentinelErrorsMapToTheirDeclaredStatus(t *testing.T) {
	h, _, _ := build(t)

	if rec := do(t, h, http.MethodGet, "/v1/things/missing", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing: status = %d, want 404", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, "/v1/things/taken/links", `{"label":"x"}`); rec.Code != http.StatusConflict {
		t.Errorf("taken: status = %d, want 409", rec.Code)
	}
}

func TestAMethodReturningOnlyAnErrorRendersNoBody(t *testing.T) {
	h, _, _ := build(t)

	rec := do(t, h, http.MethodPost, "/v1/links/abc/retire", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func TestBadJSONIsABadRequest(t *testing.T) {
	h, _, _ := build(t)

	if rec := do(t, h, http.MethodPost, "/v1/things/alpha/links", `{"label":`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestEveryRouteCarriesItsAction(t *testing.T) {
	_, _, routes := build(t)

	for _, r := range routes {
		if r.Action == "" {
			t.Errorf("%s %s carries no action, so a mount cannot gate it", r.Method, r.Path)
		}
	}
}

func TestBaseIsPrefixedOntoEveryPath(t *testing.T) {
	_, _, routes := build(t)

	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/v1/") {
			t.Errorf("path %q does not carry the spec's base", r.Path)
		}
	}
}

func TestAnUndeclaredMethodIsNotRouted(t *testing.T) {
	_, _, routes := build(t)

	if len(routes) != 4 {
		t.Fatalf("routed %d operations, want exactly the 4 the spec declares", len(routes))
	}
}

func TestDispatchRefusesAManagerMissingTheMethod(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	type empty struct{}
	_, err = dispatch.Dispatch(s, dispatch.Deps{
		Manager: empty{},
		Errors:  map[string]error{"ErrMissing": errMissing, "ErrConflict": errConflict},
	})
	if err == nil {
		t.Fatal("a manager with none of the declared methods was accepted")
	}
}

func TestDispatchRefusesAnUnsuppliedSentinel(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = dispatch.Dispatch(s, dispatch.Deps{
		Manager: &manager{},
		Errors:  map[string]error{"ErrMissing": errMissing},
	})
	if err == nil {
		t.Fatal("a spec naming an error with no sentinel behind it was accepted")
	}
	if !strings.Contains(err.Error(), "ErrConflict") {
		t.Errorf("error does not name the missing sentinel: %v", err)
	}
}

func TestParseRefusals(t *testing.T) {
	cases := map[string]string{
		"an operation with no action": `{"operations":{"a":{"method":"GET","path":"/a","call":"Open",
			"returns":[{"body":true}]}}}`,
		"two operations claiming one action": `{"operations":{
			"a":{"method":"GET","path":"/a","call":"Open","action":"t.a.read","returns":[{"body":true}]},
			"b":{"method":"GET","path":"/b","call":"Open","action":"t.a.read","returns":[{"body":true}]}}}`,
		"two operations at one address": `{"operations":{
			"a":{"method":"GET","path":"/a","call":"Open","action":"t.a.read","returns":[{"body":true}]},
			"b":{"method":"GET","path":"/a","call":"Open","action":"t.b.read","returns":[{"body":true}]}}}`,
		"a once value on a read": `{"operations":{"a":{"method":"GET","path":"/a","call":"Open",
			"action":"t.a.read","returns":[{"as":"x"},{"as":"key","once":true}]}}}`,
		"one once value on two operations": `{"operations":{
			"a":{"method":"POST","path":"/a","call":"Open","action":"t.a.make","returns":[{"as":"x"},{"as":"key","once":true}]},
			"b":{"method":"POST","path":"/b","call":"Open","action":"t.b.make","returns":[{"as":"x"},{"as":"key","once":true}]}}}`,
		"a path parameter nothing binds": `{"operations":{"a":{"method":"GET","path":"/a/{slug}","call":"Open",
			"action":"t.a.read","returns":[{"body":true}]}}}`,
		"an argument from the path the path does not declare": `{"operations":{"a":{"method":"GET","path":"/a","call":"Open",
			"action":"t.a.read","args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}}}`,
		"a repeated argument that is not a query": `{"operations":{"a":{"method":"POST","path":"/a","call":"Open",
			"action":"t.a.make","args":[{"from":"body","as":"x","repeated":true}],"returns":[{"body":true}]}}}`,
		"an operation rendering nothing": `{"operations":{"a":{"method":"GET","path":"/a","call":"Open",
			"action":"t.a.read"}}}`,
		"a body return mixed with named ones": `{"operations":{"a":{"method":"GET","path":"/a","call":"Open",
			"action":"t.a.read","returns":[{"body":true},{"as":"x"}]}}}`,
		"an action that is not module.resource.verb": `{"operations":{"a":{"method":"GET","path":"/a","call":"Open",
			"action":"open","returns":[{"body":true}]}}}`,
	}
	for what, raw := range cases {
		t.Run(what, func(t *testing.T) {
			if _, err := dispatch.Parse([]byte(raw)); err == nil {
				t.Errorf("accepted %s", what)
			}
		})
	}
}

func TestDispatchRefusesAnArityMismatch(t *testing.T) {
	raw := `{"operations":{"open":{"method":"GET","path":"/things/{slug}","call":"Open",
		"action":"t.thing.open","args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}}}`
	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := dispatch.Dispatch(s, dispatch.Deps{Manager: &manager{}}); err == nil {
		t.Fatal("a spec declaring 1 argument for a 2-argument method was accepted")
	}
}
