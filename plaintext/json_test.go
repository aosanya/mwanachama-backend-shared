package plaintext

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"
)

func TestCheckJSONStrings_NamesTheFieldHoldingAControlCharacter(t *testing.T) {
	cases := map[string]struct{ body, field string }{"top level": {`{"name":"a\u0000b"}`, "name"}, "second field": {`{"ok":"fine","message":"a\u0007b"}`, "message"}, "nested object": {`{"outer":{"inner":"a\u0000b"}}`, "outer.inner"}, "array element": {`{"items":["x","a\u0000b"]}`, "items[1]"}, "array of items": {`{"items":[{"a":1},{"label":"a\u001bb"}]}`, "items[1].label"}, "object key": {`{"ke\u0000y":"x"}`, "ke\x00y"}, "top level text": {`"a\u0000b"`, "the request body"}, "top level list": {`["a\u0000b"]`, "[0]"}, "after numbers": {`{"n":12345678901234567890,"flag":true,"none":null,"name":"a\u0000b"}`, "name"}}
	for name, c := range cases {
		err := CheckJSONStrings([]byte(c.body))
		if !errors.Is(err, ErrNotPlainText) {
			t.Fatalf("%s: %v, want a refusal", name, err)
		}
		if !strings.Contains(err.Error(), c.field) {
			t.Fatalf("%s: %q does not name %q", name, err, c.field)
		}
	}
}
func TestCheckJSONStrings_ARawNulByteInsideTextIsRefusedEvenThoughTheJSONIsBroken(t *testing.T) {
	err := CheckJSONStrings([]byte("{\"outer\":{\"name\":\"a\x00b\"}}"))
	if !errors.Is(err, ErrNotPlainText) || !strings.Contains(err.Error(), "outer") {
		t.Fatalf("a raw NUL byte: %v, want a refusal naming the object it is in", err)
	}
}
func TestCheckJSONStrings_AdmitsOrdinaryTextAndLeavesBrokenJSONToTheHandler(t *testing.T) {
	for _, body := range []string{`{"name":"Árvíztűrő tükörfúrógép 🌳","note":"első\nmásodik\r\nharmadik\toszlop"}`, `{"nested":{"list":[1,2.5,"x",{"deep":"y"}]},"n":null}`, `[]`, `{}`, `123`, ``, `{"name":`, `not json`, `{"a":"x"} {"b":"a\u0000b"}`} {
		if err := CheckJSONStrings([]byte(body)); err != nil {
			t.Fatalf("%q: %v, want it admitted", body, err)
		}
	}
}
func TestUnstorable_RecognisesWhatPostgresRefusesForANulByte(t *testing.T) {
	typed := &pgconn.PgError{Code: "22021", Message: "invalid byte sequence for encoding \"UTF8\": 0x00"}
	if !errors.Is(Unstorable(typed), ErrUnstorableText) {
		t.Fatal("SQLSTATE 22021 is not recognised")
	}
	if !errors.Is(Unstorable(errors.New("insert failed: invalid byte sequence for encoding \"UTF8\": 0x00 (SQLSTATE 22021)")), ErrUnstorableText) {
		t.Fatal("the driver's plain text is not recognised")
	}
	if Unstorable(&pgconn.PgError{Code: "23505"}) != nil || Unstorable(errors.New("connection reset")) != nil || Unstorable(nil) != nil {
		t.Fatal("an unrelated error was taken for unstorable text")
	}
}
