package httpwire

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlainMessage_DropsPackageAndFunctionNames(t *testing.T) {
	cases := []struct{ name, in, want string }{{"a row saved twice", "CreateRecordRow: mwanachamaagency: the same answers were saved a moment ago", "the same answers were saved a moment ago"}, {"a role that is gone", "DeleteRole: mwanachamaagency: role not found", "role not found"}, {"a link key taken", "mwanachamaforms: public link key is already taken", "public link key is already taken"}, {"a refusal that keeps its tail", `CreateRecordRow: mwanachamaagency: this role does not add rows to the record: only the role that adds rows to "Intake" may`, `this role does not add rows to the record: only the role that adds rows to "Intake" may`}, {"already plain", "name must be text", "name must be text"}, {"a sentence with a colon", "Note: the group is full", "Note: the group is full"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PlainMessage(c.in); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
func TestWriteErr_NeverCarriesAPackageOrFunctionName(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteErr(rec, 409, "CreateRecordRow: mwanachamaagency: the same answers were saved a moment ago")
	body := rec.Body.String()
	for _, leak := range []string{"mwanachama", "CreateRecordRow"} {
		if strings.Contains(body, leak) {
			t.Fatalf("%s names %q", body, leak)
		}
	}
}
