package dispatch

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type minWindowQuery struct {
	Limit  *int `query:"limit,min=1"`
	Offset int  `query:"offset,min=0"`
	Plain  int  `query:"plain"`
}

func minWindowDecode(t *testing.T, rawQuery string) (minWindowQuery, error) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	v, err := wholeQuery(reflect.TypeOf(minWindowQuery{}), req)
	if err != nil {
		return minWindowQuery{}, err
	}
	return v.Interface().(minWindowQuery), nil
}
func TestAQueryFieldWithAMinimumRefusesWhatIsBelowIt(t *testing.T) {
	cases := map[string]string{"limit=0": "limit must be a whole number of at least 1", "limit=-1": "limit must be a whole number of at least 1", "offset=-1": "offset must be a whole number of at least 0", "limit=1&offset=-5": "offset must be a whole number of at least 0"}
	for rawQuery, want := range cases {
		if _, err := minWindowDecode(t, rawQuery); err == nil || err.Error() != want {
			t.Errorf("%s: got %v, want %q", rawQuery, err, want)
		}
	}
}
func TestAQueryFieldWithAMinimumTakesWhatIsAtOrAboveItAndWhatIsAbsent(t *testing.T) {
	got, err := minWindowDecode(t, "limit=1&offset=0&plain=-4")
	if err != nil || got.Limit == nil || *got.Limit != 1 || got.Plain != -4 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got, err := minWindowDecode(t, ""); err != nil || got.Limit != nil {
		t.Fatalf("absent: got %+v, %v", got, err)
	}
}
