package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	errThingMissing = errors.New("thing missing")
	errThingLocked  = errors.New("thing locked")
)

type lockedError struct{ Holder string }

func (e lockedError) Error() string {
	return "locked by " + e.Holder
}

type thing struct {
	ID string `json:"id"`
}
type thingManager struct{}

func (thingManager) Get(_ context.Context, id string) (thing, error) {
	switch id {
	case "gone":
		return thing{}, errThingMissing
	case "held":
		return thing{}, lockedError{Holder: "ana"}
	case "boom":
		return thing{}, errors.New("database exploded")
	}
	return thing{ID: id}, nil
}
func (thingManager) List(_ context.Context) ([]thing, error) {
	return []thing{{ID: "a"}, {ID: "b"}}, nil
}

const thingOperations = `{"base":"/v1","operations":{
	"get_thing":{"method":"GET","path":"/things/{id}","call":"Get","returns":[{"body":true}],"action":"mod.thing.get","args":[{"from":"path","as":"id"}]},
	"list_things":{"method":"GET","path":"/things","call":"List","returns":[{"body":true}],"action":"mod.thing.list"}},
	"errors":{"errThingMissing":404,"errThingLocked":409}}`

func thingSentinels() map[string]error {
	return map[string]error{"errThingMissing": errThingMissing, "errThingLocked": errThingLocked}
}
func serve(t *testing.T, routes []Route, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
func errorBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return body
}
func TestTableBuildsRoutesUnderTheBaseAndAnswersThem(t *testing.T) {
	table := NewTable([]byte(thingOperations), thingSentinels())
	routes, err := table.Build(thingManager{}, Mount{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("got %d routes", len(routes))
	}
	rec := serve(t, routes, "GET", "/v1/things/t1", nil)
	if rec.Code != 200 || errorBody(t, rec)["id"] != "t1" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	rec = serve(t, routes, "GET", "/v1/things", nil)
	var list []thing
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 2 || list[0].ID != "a" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
func TestTableMapsASentinelToItsDeclaredStatusAndKeepsTheMessage(t *testing.T) {
	routes := NewTable([]byte(thingOperations), thingSentinels()).Routes(thingManager{}, Mount{})
	rec := serve(t, routes, "GET", "/v1/things/gone", nil)
	if rec.Code != 404 || errorBody(t, rec)["error"] != "thing missing" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
func TestTableRedactsAnUnmappedFailureToAnOpaqueFallback(t *testing.T) {
	routes := NewTable([]byte(thingOperations), thingSentinels()).Routes(thingManager{}, Mount{})
	rec := serve(t, routes, "GET", "/v1/things/boom", nil)
	body := errorBody(t, rec)
	if rec.Code != 500 || body["error"] != "internal error" || strings.Contains(rec.Body.String(), "exploded") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
func TestTableAnswersForbiddenWhenTheAuthorizerRefusesAndNeverCallsTheManager(t *testing.T) {
	var asked []string
	deny := func(_ context.Context, action string) error {
		asked = append(asked, action)
		return errors.New("no grant")
	}
	routes := NewTable([]byte(thingOperations), thingSentinels()).Routes(thingManager{}, Mount{Authorize: deny})
	rec := serve(t, routes, "GET", "/v1/things/t1", nil)
	if rec.Code != 403 || errorBody(t, rec)["error"] != "the caller may not perform that action" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if len(asked) != 1 || asked[0] != "mod.thing.get" || strings.Contains(rec.Body.String(), "no grant") {
		t.Fatalf("asked %v, body %s", asked, rec.Body)
	}
}
func TestTableRefusesAMalformedConversationHeaderBeforeCallingTheManager(t *testing.T) {
	routes := NewTable([]byte(thingOperations), thingSentinels()).Routes(thingManager{}, Mount{})
	rec := serve(t, routes, "GET", "/v1/things/t1", map[string]string{ConversationHeader: "has space"})
	if rec.Code != 400 {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	rec = serve(t, routes, "GET", "/v1/things/t1", map[string]string{ConversationHeader: "conv-9"})
	if rec.Code != 200 {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
func TestTableBuildReportsASpecThatDoesNotParseAndRoutesPanicsOnIt(t *testing.T) {
	table := NewTable([]byte(`{"operations":`), nil)
	if _, err := table.Build(thingManager{}, Mount{}); err == nil || !strings.HasPrefix(err.Error(), "dispatch:") {
		t.Fatalf("got %v", err)
	}
	if _, err := table.Shape(); err == nil {
		t.Fatal("shape of an unparsable spec must fail")
	}
	if _, err := table.UnknownAnonymousActions(); err == nil {
		t.Fatal("unknown anonymous actions of an unparsable spec must fail")
	}
	if _, err := table.UnmappedSentinels(); err == nil {
		t.Fatal("unmapped sentinels of an unparsable spec must fail")
	}
	defer func() {
		if msg, _ := recover().(string); !strings.HasPrefix(msg, "dispatch: ") {
			t.Fatalf("expected a dispatch panic, got %v", msg)
		}
	}()
	table.Routes(thingManager{}, Mount{})
}
func TestTableSpecIsParsedOnceAndShared(t *testing.T) {
	table := NewTable([]byte(thingOperations), thingSentinels())
	first, err := table.Spec()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := table.Spec()
	if first != second {
		t.Fatal("the same parsed spec must be returned on every call")
	}
}
func TestTableAnonymousActionsIsACopyTheCallerCannotUseToWidenAccess(t *testing.T) {
	table := NewTable([]byte(thingOperations), thingSentinels(), "mod.thing.list")
	got := table.AnonymousActions()
	got[0] = "mod.thing.get"
	if again := table.AnonymousActions(); len(again) != 1 || again[0] != "mod.thing.list" {
		t.Fatalf("got %v", again)
	}
}
func TestTableSplitOpensOnlyNamedActionsWithoutTheAuthorizer(t *testing.T) {
	deny := func(context.Context, string) error {
		return errors.New("no")
	}
	table := NewTable([]byte(thingOperations), thingSentinels(), "mod.thing.list")
	split := table.Split(thingManager{}, Mount{Authorize: deny})
	if len(split.Anonymous) != 1 || split.Anonymous[0].Action != "mod.thing.list" {
		t.Fatalf("got %+v", split)
	}
	if len(split.Gated) != 1 || split.Gated[0].Action != "mod.thing.get" {
		t.Fatalf("got %+v", split)
	}
	if rec := serve(t, split.Anonymous, "GET", "/v1/things", nil); rec.Code != 200 {
		t.Fatalf("anonymous route got %d %s", rec.Code, rec.Body)
	}
	if rec := serve(t, split.Gated, "GET", "/v1/things/t1", nil); rec.Code != 403 {
		t.Fatalf("gated route got %d %s", rec.Code, rec.Body)
	}
}
func TestTableShapeAndUnknownAnonymousActions(t *testing.T) {
	table := NewTable([]byte(thingOperations), thingSentinels(), "mod.thing.zzz", "mod.thing.list", "mod.thing.aaa")
	shape, err := table.Shape()
	if err != nil || len(shape) != 2 || shape[0].Path != "/v1/things/{id}" || shape[0].Handler != nil {
		t.Fatalf("got %+v, %v", shape, err)
	}
	unknown, err := table.UnknownAnonymousActions()
	if err != nil || len(unknown) != 2 || unknown[0] != "mod.thing.aaa" || unknown[1] != "mod.thing.zzz" {
		t.Fatalf("got %v, %v", unknown, err)
	}
}
func TestTableUnmappedSentinelsListsSuppliedOnesTheSpecGivesNoStatus(t *testing.T) {
	sentinels := thingSentinels()
	sentinels["errZebra"] = errors.New("zebra")
	sentinels["errApple"] = errors.New("apple")
	got, err := NewTable([]byte(thingOperations), sentinels).UnmappedSentinels()
	if err != nil || len(got) != 2 || got[0] != "errApple" || got[1] != "errZebra" {
		t.Fatalf("got %v, %v", got, err)
	}
}
func TestDispatchRefusals(t *testing.T) {
	spec, err := Parse([]byte(thingOperations))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Dispatch(nil, Deps{Manager: thingManager{}}); err == nil || !strings.Contains(err.Error(), "spec must not be nil") {
		t.Fatalf("got %v", err)
	}
	if _, err := Dispatch(spec, Deps{Errors: thingSentinels()}); err == nil || !strings.Contains(err.Error(), "manager must not be nil") {
		t.Fatalf("got %v", err)
	}
	_, err = Dispatch(spec, Deps{Manager: thingManager{}, Errors: map[string]error{"errThingMissing": errThingMissing}})
	if err == nil || !strings.Contains(err.Error(), "errThingLocked") || !strings.Contains(err.Error(), "no sentinel was supplied") {
		t.Fatalf("got %v", err)
	}
}
func TestDispatchNamesEveryOperationWhoseManagerMethodIsWrong(t *testing.T) {
	raw := `{"operations":{
		"missing":{"method":"GET","path":"/m","call":"Nope","returns":[{"body":true}],"action":"mod.thing.missing"},
		"arity":{"method":"GET","path":"/a","call":"Get","returns":[{"body":true}],"action":"mod.thing.arity"},
		"returns":{"method":"GET","path":"/r/{id}","call":"Get","returns":[{"as":"a"},{"as":"b"}],"action":"mod.thing.returns","args":[{"from":"path","as":"id"}]}}}`
	spec, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Dispatch(spec, Deps{Manager: thingManager{}})
	if err == nil {
		t.Fatal("expected refusals")
	}
	for _, want := range []string{`operation "missing" calls Nope`, `operation "arity": Get takes 1 arguments, the spec declares 0`, `operation "returns": Get returns 1 values, the spec renders 2`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("want %q in %q", want, err)
		}
	}
	if strings.Index(err.Error(), "arity") > strings.Index(err.Error(), `"missing"`) {
		t.Fatalf("problems must be sorted: %q", err)
	}
}
func TestByTypeAnswersItsStatusWithDetailMergedIntoTheErrorBody(t *testing.T) {
	table := map[error]int{ByType(lockedError{}, func(e lockedError) map[string]any {
		return map[string]any{"holder": e.Holder}
	}): 409}
	got := resolve(errors.Join(errors.New("wrapped"), lockedError{Holder: "ana"}), table, 500)
	if got.code != 409 {
		t.Fatalf("got %+v", got)
	}
	body := got.body("locked by ana")
	if body["holder"] != "ana" || body["error"] != "locked by ana" || len(body) != 2 {
		t.Fatalf("got %v", body)
	}
}
func TestByTypeWithNoDetailAnswersTheStatusAlone(t *testing.T) {
	table := map[error]int{ByType(lockedError{}, nil): 423}
	got := resolve(lockedError{Holder: "x"}, table, 500)
	if got.code != 423 || len(got.body("m")) != 1 {
		t.Fatalf("got %+v", got)
	}
}
func TestResolveFallsBackWhenNoSentinelOrTypeMatches(t *testing.T) {
	table := map[error]int{errThingMissing: 404, ByType(lockedError{}, nil): 423}
	if got := resolve(errors.New("other"), table, 502); got.code != 502 || got.detail != nil {
		t.Fatalf("got %+v", got)
	}
	if got := resolve(errors.Join(errors.New("ctx"), errThingMissing), table, 502); got.code != 404 {
		t.Fatalf("got %+v", got)
	}
	if msg := (&typeMatch{}).Error(); !strings.Contains(msg, "type") {
		t.Fatalf("got %q", msg)
	}
}
func TestByTypeStatusReachesTheWire(t *testing.T) {
	spec, err := Parse([]byte(thingOperations))
	if err != nil {
		t.Fatal(err)
	}
	routes, err := Dispatch(spec, Deps{Manager: thingManager{}, Errors: map[string]error{"errThingMissing": errThingMissing, "errThingLocked": ByType(lockedError{}, func(e lockedError) map[string]any {
		return map[string]any{"holder": e.Holder}
	})}})
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, routes, "GET", "/v1/things/held", nil)
	body := errorBody(t, rec)
	if rec.Code != 409 || body["holder"] != "ana" || body["error"] != "locked by ana" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
func writeSentinelFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "errors.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestPackageUnmappedSentinelsNamesSuppliedAndExportedOnesWithNoStatus(t *testing.T) {
	dir := writeSentinelFile(t, `package x

import (
	"errors"
	"fmt"
)

var (
	ErrMapped   = errors.New("mapped")
	ErrLoose    = errors.New("loose")
	ErrWrapped  = fmt.Errorf("wrapped")
	errPrivate  = errors.New("private")
	NotAnError  = errors.New("not named err")
)
`)
	if err := os.WriteFile(filepath.Join(dir, "skipped_test.go"), []byte("package x\n\nvar ErrInTest = errors.New(\"t\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := &Spec{Errors: map[string]int{"ErrMapped": 404}}
	got := UnmappedSentinels(spec, map[string]error{"ErrMapped": errThingMissing, "ErrSupplied": errThingLocked}, dir)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if !strings.HasPrefix(got[0], "ErrLoose is exported by ") || !strings.HasPrefix(got[1], "ErrSupplied is supplied to the dispatcher") {
		t.Fatalf("got %v", got)
	}
}
func TestPackageUnmappedSentinelsIsEmptyWhenEverythingIsMapped(t *testing.T) {
	dir := writeSentinelFile(t, "package x\n\nimport \"errors\"\n\nvar ErrOnly = errors.New(\"only\")\n")
	spec := &Spec{Errors: map[string]int{"ErrOnly": 409}}
	if got := UnmappedSentinels(spec, map[string]error{"ErrOnly": errThingLocked}, dir); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
func TestPackageUnmappedSentinelsReportsADirectoryItCannotParse(t *testing.T) {
	dir := writeSentinelFile(t, "package x\n\nvar broken = (\n")
	got := UnmappedSentinels(&Spec{}, nil, dir)
	if len(got) != 1 || !strings.HasPrefix(got[0], "dispatch: parse ") {
		t.Fatalf("got %v", got)
	}
}
