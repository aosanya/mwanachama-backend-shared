package dispatch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var (
	errRoleGone = errors.New("mwanachamaagency: role not found")
	errKeyTaken = errors.New("mwanachamaforms: public link key is already taken")
)

type failingManager struct{ err error }

func (m failingManager) Fail(context.Context) error {
	return m.err
}
func answerTo(t *testing.T, err error) (int, string) {
	t.Helper()
	table := map[error]int{errRoleGone: http.StatusNotFound, errKeyTaken: http.StatusConflict}
	method := reflect.ValueOf(failingManager{err: err}).MethodByName("Fail")
	handler := handlerFor(Operation{Method: "POST", Path: "/x"}, method, table, http.StatusInternalServerError, nil, nil, nil)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest("POST", "/x", strings.NewReader("{}")))
	return rec.Code, rec.Body.String()
}
func TestMappedErrorsAnswerInPlainText(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		want   string
	}{{"a role that is gone", fmt.Errorf("DeleteRole: %w", errRoleGone), 404, "role not found"}, {"a link key taken", errKeyTaken, 409, "public link key is already taken"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := answerTo(t, c.err)
			if status != c.status {
				t.Fatalf("status %d, want %d", status, c.status)
			}
			if !strings.Contains(body, c.want) {
				t.Fatalf("%s does not say %q", body, c.want)
			}
			for _, leak := range []string{"mwanachama", "DeleteRole"} {
				if strings.Contains(body, leak) {
					t.Fatalf("%s names %q", body, leak)
				}
			}
		})
	}
}
func TestToolErrorsAnswerInPlainTextAndKeepTheirCause(t *testing.T) {
	table := map[error]int{errRoleGone: http.StatusNotFound}
	method := reflect.ValueOf(failingManager{err: fmt.Errorf("DeleteRole: %w", errRoleGone)}).MethodByName("Fail")
	invoke := invokerFor(Operation{Method: "POST", Path: "/x"}, nil, nil, method, table, http.StatusInternalServerError, nil, nil, nil)
	_, err := invoke(context.Background(), Request{})
	if err == nil {
		t.Fatal("want an error")
	}
	if err.Error() != "role not found" {
		t.Fatalf("got %q", err.Error())
	}
	if !errors.Is(err, errRoleGone) {
		t.Fatal("the cause was lost")
	}
}
