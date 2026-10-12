package dispatch_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func specWith(args string) string {
	return `{"operations":{"open":{"method":"GET","path":"/things/{slug}","call":"Open","action":"t.thing.open",
		"description":"open one","args":[` + args + `],"returns":[{"body":true}]}}}`
}

func TestARefersMarkIsHeldToModuleDotObject(t *testing.T) {
	for _, bad := range []string{"Agency.role", "agency", "agency.role.code", "agency-role", "agency.", ".role", "agency.Role"} {
		_, err := dispatch.Parse([]byte(specWith(`{"from":"path","as":"slug","refers":"` + bad + `"},{"from":"query","as":"k"}`)))
		if err == nil || !strings.Contains(err.Error(), "is not <module>.<object>") {
			t.Errorf("refers %q: want a refusal naming the pattern, got %v", bad, err)
		}
	}
	for _, good := range []string{"agency.role", "git.blob", "t.thing_2"} {
		if _, err := dispatch.Parse([]byte(specWith(`{"from":"path","as":"slug","refers":"` + good + `"},{"from":"query","as":"k"}`))); err != nil {
			t.Errorf("refers %q: %v", good, err)
		}
	}
}

func TestRefersIsRefusedWhereNoValueIsBound(t *testing.T) {
	cases := map[string]string{
		"a whole request":    `{"from":"path","as":"slug"},{"from":"query","whole":true,"refers":"t.thing"}`,
		"an ignored segment": `{"from":"path","as":"slug","ignored":true,"refers":"t.thing"},{"from":"query","as":"k"}`,
		"the caller":         `{"from":"path","as":"slug"},{"from":"caller","as":"k","refers":"t.thing"}`,
		"the device":         `{"from":"path","as":"slug"},{"from":"device","as":"k","refers":"t.thing"}`,
	}
	for what, args := range cases {
		_, err := dispatch.Parse([]byte(specWith(args)))
		if err == nil || !strings.Contains(err.Error(), "refer") {
			t.Errorf("%s: want a refusal about the reference, got %v", what, err)
		}
	}
}

func TestReferencesEnumeratesOnlyTheMarkedArguments(t *testing.T) {
	s, err := dispatch.Parse([]byte(`{"operations":{
		"open":  {"method":"GET","path":"/things/{slug}","call":"Open","action":"t.thing.open",
		          "args":[{"from":"path","as":"slug","refers":"t.thing"},{"from":"query","as":"k"}],"returns":[{"body":true}]},
		"list":  {"method":"GET","path":"/things","call":"List","action":"t.thing.list",
		          "args":[{"from":"query","as":"status","repeated":true,"refers":"t.status"}],"returns":[{"body":true}]},
		"issue": {"method":"POST","path":"/things/{slug}/links","call":"Issue","action":"t.link.issue",
		          "args":[{"from":"path","as":"slug"},{"from":"body","as":"label","refers":"t.label"}],"returns":[{"as":"link"},{"as":"key"}]}
	}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []dispatch.Reference{
		{Operation: "issue", As: "label", From: dispatch.FromBody, Refers: "t.label"},
		{Operation: "list", As: "status", From: dispatch.FromQuery, Repeated: true, Refers: "t.status"},
		{Operation: "open", As: "slug", From: dispatch.FromPath, Refers: "t.thing"},
	}
	got := s.References()
	if len(got) != len(want) {
		t.Fatalf("references: got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reference %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	plain, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if refs := plain.References(); len(refs) != 0 {
		t.Fatalf("an unmarked spec declares no reference: %+v", refs)
	}
}

func TestAMarkedArgumentBindsAndToolsExactlyAsAnUnmarkedOne(t *testing.T) {
	unmarked := specWith(`{"from":"path","as":"slug"},{"from":"query","as":"k"}`)
	marked := specWith(`{"from":"path","as":"slug","refers":"t.thing"},{"from":"query","as":"k"}`)
	var schemas [][]byte
	var bodies []string
	for _, raw := range []string{unmarked, marked} {
		s, err := dispatch.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		m := &manager{}
		d := dispatch.Deps{Manager: m}
		routes, err := dispatch.Dispatch(s, d)
		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		mux := http.NewServeMux()
		for _, r := range routes {
			mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
		}
		rec := do(t, mux, http.MethodGet, "/things/abc?k=z", "")
		if rec.Code != http.StatusOK || m.opened != "abc" || m.key != "z" {
			t.Fatalf("status %d opened %q key %q", rec.Code, m.opened, m.key)
		}
		bodies = append(bodies, rec.Body.String())
		tools, err := dispatch.Tools(s, d)
		if err != nil {
			t.Fatalf("tools: %v", err)
		}
		if len(tools) != 1 {
			t.Fatalf("one tool, got %d", len(tools))
		}
		schemas = append(schemas, tools[0].InputSchema)
		if _, err := tools[0].Invoke(context.Background(), json.RawMessage(`{"slug":"abc","k":"z"}`)); err != nil {
			t.Fatalf("invoke: %v", err)
		}
	}
	if bodies[0] != bodies[1] {
		t.Fatalf("the route answers the same with and without the mark:\n%s\n%s", bodies[0], bodies[1])
	}
	if !bytes.Equal(schemas[0], schemas[1]) {
		t.Fatalf("the tool schema is the same with and without the mark:\n%s\n%s", schemas[0], schemas[1])
	}
}
