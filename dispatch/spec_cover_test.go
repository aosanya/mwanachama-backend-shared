package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

const validOne = `{"base":"/v1","operations":{"list_things":{"method":"GET","path":"/things","call":"ListThings","returns":[{"body":true}],"action":"mod.thing.list"}}}`

func parseProblem(t *testing.T, raw string) string {
	t.Helper()
	_, err := Parse([]byte(raw))
	if err == nil {
		t.Fatalf("expected a refusal for %s", raw)
	}
	return err.Error()
}
func TestParseAcceptsAMinimalOperation(t *testing.T) {
	s, err := Parse([]byte(validOne))
	if err != nil {
		t.Fatal(err)
	}
	if s.Base != "/v1" || s.Operations["list_things"].status() != 200 {
		t.Fatalf("unexpected spec %+v", s)
	}
}
func TestParseRefusesMalformedJSON(t *testing.T) {
	if msg := parseProblem(t, `{"operations":`); !strings.HasPrefix(msg, "dispatch:") {
		t.Fatalf("got %q", msg)
	}
}
func TestParseRefusesASpecWithNoOperations(t *testing.T) {
	if msg := parseProblem(t, `{"operations":{}}`); !strings.Contains(msg, "at least one operation") {
		t.Fatalf("got %q", msg)
	}
}
func TestValidateRefusals(t *testing.T) {
	op := func(body string) string {
		return `{"operations":{"do_it":{` + body + `}}}`
	}
	cases := map[string]struct{ raw, want string }{"bad operation name": {`{"operations":{"Bad-Name":{"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b.c"}}}`, `is not a usable name`}, "unroutable method": {op(`"method":"TRACE","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b.c"`), `is not one this dispatcher routes`}, "path without slash": {op(`"method":"GET","path":"x","call":"C","returns":[{"body":true}],"action":"a.b.c"`), `does not start with /`}, "no call, unhandled": {op(`"method":"GET","path":"/x","action":"a.b.c"`), `names no manager method`}, "call and handled": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"handled":true,"action":"a.b.c"`), `it is one or the other`}, "no action": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}]`), `declares no action`}, "two-part action": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b"`), `is not <module>.<resource>.<verb>`}, "uppercase action": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"A.b.c"`), `is not <module>.<resource>.<verb>`}, "tool name invalid": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b.c","tool":"Bad Tool"`), `tool name "Bad Tool" is not a usable name`}, "tool true": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b.c","tool":true`), `says nothing an omitted tool does not`}, "tool number": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b.c","tool":7`), `is a tool name or false`}, "whenQuery invalid": {op(`"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"a.b.c","whenQuery":"Bad Q"`), `is not a usable query parameter name`}, "whenQuery handled": {op(`"method":"GET","path":"/x","handled":true,"action":"a.b.c","whenQuery":"q"`), `a selected alternative is dispatched`}, "read returns once": {op(`"method":"GET","path":"/x","call":"C","action":"a.b.c","returns":[{"as":"secret","once":true}]`), `a read cannot be the only chance`}}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if msg := parseProblem(t, c.raw); !strings.Contains(msg, c.want) {
				t.Fatalf("want %q in %q", c.want, msg)
			}
		})
	}
}
func TestValidateRefusesDuplicates(t *testing.T) {
	cases := map[string]struct{ raw, want string }{"same action": {`{"operations":{
			"a":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.v"},
			"b":{"method":"GET","path":"/b","call":"C","returns":[{"body":true}],"action":"m.r.v"}}}`, `both claim the action "m.r.v"`}, "same address": {`{"operations":{
			"a":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.one"},
			"b":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.two"}}}`, `both answer "GET /a"`}, "same tool": {`{"operations":{
			"a":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.one","tool":"shared"},
			"b":{"method":"POST","path":"/b","call":"C","returns":[{"body":true}],"action":"m.r.two","tool":"shared"}}}`, `both publish the tool "shared"`}, "same once value": {`{"operations":{
			"a":{"method":"POST","path":"/a","call":"C","action":"m.r.one","returns":[{"as":"key","once":true}]},
			"b":{"method":"POST","path":"/b","call":"C","action":"m.r.two","returns":[{"as":"key","once":true}]}}}`, `both return "key" once`}}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if msg := parseProblem(t, c.raw); !strings.Contains(msg, c.want) {
				t.Fatalf("want %q in %q", c.want, msg)
			}
		})
	}
}
func TestSameAddressIsAllowedWhenSelectedByAQuery(t *testing.T) {
	raw := `{"operations":{
		"plain":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.plain"},
		"by_q":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.byq","whenQuery":"q"}}}`
	if _, err := Parse([]byte(raw)); err != nil {
		t.Fatalf("a selected alternative must not collide: %v", err)
	}
}
func TestValidateReportsEveryProblemSortedInOneError(t *testing.T) {
	raw := `{"operations":{
		"a":{"method":"TRACE","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.one"},
		"b":{"method":"GET","path":"b","call":"C","returns":[{"body":true}],"action":"m.r.two"}}}`
	msg := parseProblem(t, raw)
	i, j := strings.Index(msg, `method "TRACE"`), strings.Index(msg, `path "b" does not start`)
	if i < 0 || j < 0 || i > j {
		t.Fatalf("expected both problems sorted, got %q", msg)
	}
}
func TestToolFalseOptsOutAndRoundTrips(t *testing.T) {
	raw := `{"method":"GET","path":"/x","call":"C","returns":[{"body":true}],"action":"m.r.v","tool":false}`
	var op Operation
	if err := json.Unmarshal([]byte(raw), &op); err != nil {
		t.Fatal(err)
	}
	if !op.NoTool || op.Tool != "" {
		t.Fatalf("got %+v", op)
	}
	out, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"tool":false`) {
		t.Fatalf("opt-out lost on marshal: %s", out)
	}
}
func TestToolNameRoundTripsAndNullMeansOmitted(t *testing.T) {
	var named Operation
	if err := json.Unmarshal([]byte(`{"method":"GET","path":"/x","tool":"my_tool"}`), &named); err != nil {
		t.Fatal(err)
	}
	if named.Tool != "my_tool" || named.NoTool {
		t.Fatalf("got %+v", named)
	}
	out, _ := json.Marshal(named)
	if !strings.Contains(string(out), `"tool":"my_tool"`) {
		t.Fatalf("got %s", out)
	}
	var null Operation
	if err := json.Unmarshal([]byte(`{"method":"GET","path":"/x","tool":null}`), &null); err != nil {
		t.Fatal(err)
	}
	if null.Tool != "" || null.NoTool {
		t.Fatalf("got %+v", null)
	}
}
func TestShapeListsRoutesSortedByOperationNameWithBase(t *testing.T) {
	s, err := Parse([]byte(`{"base":"/v1","operations":{
		"zeta":{"method":"POST","path":"/z","call":"C","returns":[{"body":true}],"action":"m.r.zeta"},
		"alpha":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.alpha"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	routes := Shape(s)
	if len(routes) != 2 || routes[0].Path != "/v1/a" || routes[0].Action != "m.r.alpha" || routes[1].Method != "POST" || routes[1].Path != "/v1/z" {
		t.Fatalf("got %+v", routes)
	}
	if Shape(nil) != nil || Handled(nil) != nil {
		t.Fatal("a nil spec has no routes")
	}
}
func TestHandledReturnsOnlyModuleHandledOperations(t *testing.T) {
	s, err := Parse([]byte(`{"operations":{
		"b_handled":{"method":"POST","path":"/b","handled":true,"action":"m.r.b"},
		"a_called":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.a"},
		"a_handled":{"method":"POST","path":"/c","handled":true,"action":"m.r.c"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := Handled(s)
	if len(got) != 2 || got[0].Action != "m.r.c" || got[1].Action != "m.r.b" {
		t.Fatalf("got %+v", got)
	}
}
func TestPathParamsIgnoresEmptyAndPlainSegments(t *testing.T) {
	got := pathParams("/a/{id}/b/{}/c/{x_y}/{open")
	if len(got) != 2 || !got["id"] || !got["x_y"] {
		t.Fatalf("got %v", got)
	}
}
func TestRefersIsCheckedAndListed(t *testing.T) {
	bad := map[string]struct{ arg, want string }{"shape": {`{"from":"path","as":"id","refers":"nope"}`, `not <module>.<object>`}, "whole": {`{"from":"body","whole":true,"refers":"m.o"}`, `takes a whole request`}, "caller": {`{"from":"caller","as":"who","refers":"m.o"}`, `comes from the session`}}
	for name, c := range bad {
		t.Run(name, func(t *testing.T) {
			raw := `{"operations":{"o":{"method":"GET","path":"/x/{id}","call":"C","returns":[{"body":true}],"action":"m.r.v","args":[` + c.arg + `]}}}`
			if msg := parseProblem(t, raw); !strings.Contains(msg, c.want) {
				t.Fatalf("want %q in %q", c.want, msg)
			}
		})
	}
	s, err := Parse([]byte(`{"operations":{
		"b":{"method":"GET","path":"/b/{id}","call":"C","returns":[{"body":true}],"action":"m.r.b","args":[{"from":"path","as":"id","refers":"cat.entry"}]},
		"a":{"method":"GET","path":"/a","call":"C","returns":[{"body":true}],"action":"m.r.a","args":[{"from":"query","as":"q"}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	refs := s.References()
	if len(refs) != 1 || refs[0].Operation != "b" || refs[0].As != "id" || refs[0].Refers != "cat.entry" || refs[0].From != FromPath {
		t.Fatalf("got %+v", refs)
	}
}
func TestAnonymousSplitsRoutesAndGatesAnythingNotNamed(t *testing.T) {
	routes := []Route{{Method: "GET", Path: "/open", Action: "m.r.open"}, {Method: "GET", Path: "/closed", Action: "m.r.closed"}, {Method: "GET", Path: "/none"}}
	split := Anonymous(routes, "m.r.open")
	if len(split.Anonymous) != 1 || split.Anonymous[0].Path != "/open" || len(split.Gated) != 2 {
		t.Fatalf("got %+v", split)
	}
	if got := Anonymous(routes).Anonymous; len(got) != 0 {
		t.Fatalf("with no actions named nothing is public, got %+v", got)
	}
}
func TestAnonymousRouteWithAGatedAlternativeStaysGated(t *testing.T) {
	routes := []Route{{Method: "GET", Path: "/x", Action: "m.r.open", Alternatives: []string{"m.r.secret"}}}
	if split := Anonymous(routes, "m.r.open"); len(split.Anonymous) != 0 || len(split.Gated) != 1 {
		t.Fatalf("got %+v", split)
	}
	if split := Anonymous(routes, "m.r.open", "m.r.secret"); len(split.Anonymous) != 1 {
		t.Fatalf("got %+v", split)
	}
}
func TestUnmatchedNamesActionsWithNoRouteSorted(t *testing.T) {
	routes := []Route{{Action: "m.r.here"}}
	got := Split{}.Unmatched(routes, []string{"m.r.zz", "m.r.here", "m.r.aa"})
	if len(got) != 2 || got[0] != "m.r.aa" || got[1] != "m.r.zz" {
		t.Fatalf("got %v", got)
	}
}
func TestConversationHeaderIsCarriedOnTheContext(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(ConversationHeader, "conv-1:a.b")
	ctx, err := conversationFromHeader(r)
	if err != nil || ConversationOf(ctx) != "conv-1:a.b" {
		t.Fatalf("got %q, %v", ConversationOf(ctx), err)
	}
}
func TestConversationHeaderRefusals(t *testing.T) {
	twice := httptest.NewRequest("GET", "/", nil)
	twice.Header.Add(ConversationHeader, "one")
	twice.Header.Add(ConversationHeader, "two")
	if _, err := conversationFromHeader(twice); err == nil || !strings.Contains(err.Error(), "sent more than once") {
		t.Fatalf("got %v", err)
	}
	for _, bad := range []string{"-leading", "has space", strings.Repeat("a", 129)} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set(ConversationHeader, bad)
		if _, err := conversationFromHeader(r); !errors.Is(err, ErrInvalidConversation) {
			t.Fatalf("%q: got %v", bad, err)
		}
	}
}
func TestConversationHeaderAbsentOrEmptyCarriesNothing(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	ctx, err := conversationFromHeader(r)
	if err != nil || ConversationOf(ctx) != "" {
		t.Fatalf("got %q, %v", ConversationOf(ctx), err)
	}
	r.Header.Set(ConversationHeader, "")
	ctx, err = conversationFromHeader(r)
	if err != nil || ConversationOf(ctx) != "" {
		t.Fatalf("got %q, %v", ConversationOf(ctx), err)
	}
}
func TestConversationFromMeta(t *testing.T) {
	ctx, err := conversationFromMeta(context.Background(), map[string]any{ConversationMetaKey: "abc-1"})
	if err != nil || ConversationOf(ctx) != "abc-1" {
		t.Fatalf("got %q, %v", ConversationOf(ctx), err)
	}
	for _, meta := range []map[string]any{nil, {}, {ConversationMetaKey: nil}} {
		ctx, err := conversationFromMeta(context.Background(), meta)
		if err != nil || ConversationOf(ctx) != "" {
			t.Fatalf("%v: got %q, %v", meta, ConversationOf(ctx), err)
		}
	}
	if _, err := conversationFromMeta(context.Background(), map[string]any{ConversationMetaKey: 42}); !errors.Is(err, ErrInvalidConversation) {
		t.Fatalf("a non-string id must be refused, got %v", err)
	}
	if _, err := conversationFromMeta(context.Background(), map[string]any{ConversationMetaKey: "bad id"}); err == nil || !strings.Contains(err.Error(), "_meta.conversation_id") {
		t.Fatalf("the refusal must name the field, got %v", err)
	}
}
func TestWithConversationIgnoresAnEmptyID(t *testing.T) {
	ctx := WithConversation(context.Background(), "")
	if ConversationOf(ctx) != "" {
		t.Fatal("empty id must not be stored")
	}
	if ConversationOf(context.Background()) != "" {
		t.Fatal("bare context has no conversation")
	}
}
