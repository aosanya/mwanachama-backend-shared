package dispatch_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type upsertManager struct {
	got record
}

func (m *upsertManager) Upsert(ctx context.Context, rec record) (record, error) {
	m.got = rec
	return rec, nil
}

const upsertSpec = `{
  "operations": {
    "upsert": {"method":"PUT","path":"/records/{slug}","call":"Upsert","action":"t.record.upsert",
               "args":[{"from":"body","whole":true},{"from":"path","as":"slug","into":"Slug"}],
               "returns":[{"body":true}]}
  }
}`

func buildUpsert(t *testing.T) (http.Handler, *upsertManager) {
	t.Helper()
	s, err := dispatch.Parse([]byte(upsertSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &upsertManager{}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: m})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux, m
}

func TestPathOverwritesTheBodyField(t *testing.T) {
	h, m := buildUpsert(t)

	rec := do(t, h, http.MethodPut, "/records/alpha", `{"slug":"alpha","name":"Alpha"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.got.Slug != "alpha" || m.got.Name != "Alpha" {
		t.Fatalf("record = %+v, want alpha/Alpha", m.got)
	}
}

func TestTheAddressOutranksAContradictingBody(t *testing.T) {
	h, m := buildUpsert(t)

	rec := do(t, h, http.MethodPut, "/records/alpha", `{"slug":"somewhere-else","name":"Alpha"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.got.Slug != "alpha" {
		t.Fatalf("slug = %q, want alpha; a body claiming another slug must not write to it", m.got.Slug)
	}
}

func TestAnAbsentBodyFieldStillTakesTheAddress(t *testing.T) {
	h, m := buildUpsert(t)

	rec := do(t, h, http.MethodPut, "/records/alpha", `{"name":"Alpha"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.got.Slug != "alpha" {
		t.Fatalf("slug = %q, want alpha", m.got.Slug)
	}
}

func TestOverwriteRefusals(t *testing.T) {
	cases := map[string]string{
		"an overwrite from the body": `{"operations":{"a":{"method":"PUT","path":"/a","call":"Upsert",
			"action":"t.a.put","args":[{"from":"body","whole":true},{"from":"body","as":"slug","into":"Slug"}],
			"returns":[{"body":true}]}}}`,
		"an overwrite from the query": `{"operations":{"a":{"method":"PUT","path":"/a","call":"Upsert",
			"action":"t.a.put","args":[{"from":"body","whole":true},{"from":"query","as":"slug","into":"Slug"}],
			"returns":[{"body":true}]}}}`,
		"an overwrite with nothing to overwrite on": `{"operations":{"a":{"method":"PUT","path":"/a/{slug}","call":"Upsert",
			"action":"t.a.put","args":[{"from":"path","as":"slug","into":"Slug"}],"returns":[{"body":true}]}}}`,
		"an overwrite from a path segment that does not exist": `{"operations":{"a":{"method":"PUT","path":"/a","call":"Upsert",
			"action":"t.a.put","args":[{"from":"body","whole":true},{"from":"path","as":"slug","into":"Slug"}],
			"returns":[{"body":true}]}}}`,
	}
	for what, raw := range cases {
		t.Run(what, func(t *testing.T) {
			if _, err := dispatch.Parse([]byte(raw)); err == nil {
				t.Errorf("accepted %s", what)
			}
		})
	}
}

func TestDispatchRefusesAnOverwriteOfAFieldThatIsNotThere(t *testing.T) {
	raw := `{"operations":{"a":{"method":"PUT","path":"/a/{slug}","call":"Upsert",
		"action":"t.a.put","args":[{"from":"body","whole":true},{"from":"path","as":"slug","into":"Nonexistent"}],
		"returns":[{"body":true}]}}}`
	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: &upsertManager{}})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	if rec := do(t, mux, http.MethodPut, "/a/alpha", `{"name":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
