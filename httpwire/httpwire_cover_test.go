package httpwire

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func requestWith(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
}
func TestReadJSON_RefusesAFieldTheTypeDoesNotHave(t *testing.T) {
	var s sample
	err := ReadJSON(requestWith(`{"name":"a","admin":true}`), &s)
	if err == nil || !strings.Contains(err.Error(), "admin") {
		t.Fatalf("err = %v, want a refusal naming the unknown field", err)
	}
}
func TestReadJSON_RefusesAMalformedBody(t *testing.T) {
	var s sample
	if err := ReadJSON(requestWith(`{"name":`), &s); err == nil {
		t.Fatal("a truncated body was accepted")
	}
}
func TestReadJSON_FillsTheKnownFields(t *testing.T) {
	var s sample
	if err := ReadJSON(requestWith(`{"name":"Amina","age":7}`), &s); err != nil {
		t.Fatal(err)
	}
	if s != (sample{Name: "Amina", Age: 7}) {
		t.Fatalf("decoded = %+v", s)
	}
}
func TestReadJSONFields_ReportsWhichKeysTheCallerSent(t *testing.T) {
	var s sample
	present, err := ReadJSONFields(requestWith(`{"name":"","age":0}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	if !present["name"] || !present["age"] || len(present) != 2 {
		t.Fatalf("present = %v, want name and age even when zero", present)
	}
}
func TestReadJSONFields_AbsentKeyIsNotPresent(t *testing.T) {
	var s sample
	present, err := ReadJSONFields(requestWith(`{"name":"a"}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	if present["age"] {
		t.Fatalf("age reported present: %v", present)
	}
}
func TestReadJSONFields_RefusesUnknownFieldAndGarbage(t *testing.T) {
	var s sample
	if _, err := ReadJSONFields(requestWith(`{"nope":1}`), &s); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := ReadJSONFields(requestWith(`not json`), &s); err == nil {
		t.Fatal("garbage accepted")
	}
}
func TestReadJSONFields_RefusesANonObjectBody(t *testing.T) {
	var s []int
	if _, err := ReadJSONFields(requestWith(`[1,2]`), &s); err == nil {
		t.Fatal("an array body gave a field set")
	}
}
func TestBearer(t *testing.T) {
	cases := []struct{ name, header, want string }{{"no header", "", ""}, {"bearer scheme is stripped", "Bearer abc.def", "abc.def"}, {"a bare token is returned whole", "abc.def", "abc.def"}, {"lowercase scheme is not stripped", "bearer abc", "bearer abc"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.header != "" {
				r.Header.Set("Authorization", c.header)
			}
			if got := Bearer(r); got != c.want {
				t.Fatalf("Bearer = %q, want %q", got, c.want)
			}
		})
	}
}
func TestRoutePattern_JoinsMethodPrefixAndPath(t *testing.T) {
	r := Route{Method: "GET", Path: "/polls/{id}"}
	if got := r.Pattern("/v1"); got != "GET /v1/polls/{id}" {
		t.Fatalf("Pattern = %q", got)
	}
	if got := r.Pattern(""); got != "GET /polls/{id}" {
		t.Fatalf("Pattern without prefix = %q", got)
	}
}
func TestStatusFor(t *testing.T) {
	errGone := errors.New("gone")
	errBad := errors.New("bad")
	table := map[error]int{errGone: 404, errBad: 400}
	cases := []struct {
		name string
		err  error
		want int
	}{{"direct sentinel", errGone, 404}, {"wrapped sentinel", fmt.Errorf("load: %w", errBad), 400}, {"unmapped error takes the fallback", errors.New("other"), 500}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StatusFor(c.err, table, 500); got != c.want {
				t.Fatalf("StatusFor = %d, want %d", got, c.want)
			}
		})
	}
	if got := StatusFor(errGone, nil, 418); got != 418 {
		t.Fatalf("empty table = %d, want fallback 418", got)
	}
}
func TestWriteErr_ReplyIsJSONWithThePlainMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteErr(rec, http.StatusConflict, "CreateRecordRow: mwanachamaagency: already saved")
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"already saved"}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
func TestPlainError_ReadsPlainAndStillUnwrapsToTheCause(t *testing.T) {
	cause := errors.New("DeleteRole: mwanachamaagency: role not found")
	err := PlainError{Cause: cause}
	if err.Error() != "role not found" {
		t.Fatalf("Error = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("PlainError does not unwrap to its cause")
	}
}
func TestPlainWithLog_ReturnsThePlainTextAndLeavesCleanTextAlone(t *testing.T) {
	got := PlainWithLog(errors.New("CreateRecordRow: mwanachamaagency: saved before"), "POST /records")
	if got != "saved before" {
		t.Fatalf("PlainWithLog = %q", got)
	}
	if got := PlainWithLog(errors.New("nothing to strip"), "GET /x"); got != "nothing to strip" {
		t.Fatalf("clean message = %q", got)
	}
}
