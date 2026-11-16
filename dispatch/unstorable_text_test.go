package dispatch

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

var postgresNulRefusal = fmt.Errorf("SetName: %w", &pgconn.PgError{Code: "22021", Message: `invalid byte sequence for encoding "UTF8": 0x00`})

func TestAStoreRefusalOfANulByteAnswers400NotTheOpaqueFallback(t *testing.T) {
	status, body := answerTo(t, postgresNulRefusal)
	if status != http.StatusBadRequest {
		t.Fatalf("status %d %s, want 400", status, body)
	}
	if strings.Contains(body, "UTF8") || strings.Contains(body, "SetName") {
		t.Fatalf("%s leaks the driver's text", body)
	}
}
func TestAStoreRefusalOfANulByteOverAToolIsARefusalToo(t *testing.T) {
	method := reflect.ValueOf(failingManager{err: postgresNulRefusal}).MethodByName("Fail")
	invoke := invokerFor(Operation{Method: "POST", Path: "/x"}, nil, nil, method, nil, http.StatusInternalServerError, nil, nil, nil)
	_, err := invoke(context.Background(), Request{})
	if err == nil || strings.Contains(err.Error(), "internal error") || strings.Contains(err.Error(), "UTF8") {
		t.Fatalf("got %v, want the plain NUL refusal", err)
	}
}
