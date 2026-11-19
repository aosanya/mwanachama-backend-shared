package dispatch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type pagedQuery struct {
	Limit    int      `query:"limit,min=1"`
	Offset   *int     `query:"offset,min=0"`
	Kinds    []string `query:"kind"`
	Hidden   string   `query:"-"`
	RiskID   string
	APIKeys  string
	secret   string
	Untagged string
}

type openedBody struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

func newRequest(method, target, body string) *http.Request {
	return httptest.NewRequest(method, target, strings.NewReader(body))
}

func TestSnakeCaseSplitsWordsTheWayAColumnIsNamed(t *testing.T) {
	cases := map[string]string{
		"RiskID":      "risk_id",
		"Name":        "name",
		"SubmittedBy": "submitted_by",
		"OptionIDs":   "option_ids",
		"Level2Name":  "level2_name",
		"already":     "already",
	}
	for in, want := range cases {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryKeyHonoursTagThenFallsBackToSnakeCase(t *testing.T) {
	typ := reflect.TypeOf(pagedQuery{})
	want := map[string]string{"Limit": "limit", "Hidden": "", "RiskID": "risk_id", "Kinds": "kind"}
	for name, key := range want {
		f, _ := typ.FieldByName(name)
		if got := queryKey(f); got != key {
			t.Errorf("queryKey(%s) = %q, want %q", name, got, key)
		}
	}
}

func TestQueryMinimumReadsOnlyAWellFormedMinOption(t *testing.T) {
	type sample struct {
		A int `query:"a,min=3"`
		B int `query:"b,min=abc"`
		C int `query:"c"`
		D int
	}
	typ := reflect.TypeOf(sample{})
	check := map[string]struct {
		n  int64
		ok bool
	}{"A": {3, true}, "B": {0, false}, "C": {0, false}, "D": {0, false}}
	for name, want := range check {
		f, _ := typ.FieldByName(name)
		n, ok := queryMinimum(f)
		if n != want.n || ok != want.ok {
			t.Errorf("queryMinimum(%s) = %d,%v want %d,%v", name, n, ok, want.n, want.ok)
		}
	}
}

func TestWholeQueryFillsFieldsAndRepeatedKeys(t *testing.T) {
	r := newRequest("GET", "/x?limit=5&offset=0&kind=a&kind=b&risk_id=r1&Hidden=zzz", "")
	v, err := wholeQuery(reflect.TypeOf(pagedQuery{}), r)
	if err != nil {
		t.Fatal(err)
	}
	got := v.Interface().(pagedQuery)
	if got.Limit != 5 || got.Offset == nil || *got.Offset != 0 {
		t.Fatalf("numbers not bound: %+v", got)
	}
	if !reflect.DeepEqual(got.Kinds, []string{"a", "b"}) {
		t.Fatalf("repeated key lost order: %v", got.Kinds)
	}
	if got.RiskID != "r1" || got.Hidden != "" {
		t.Fatalf("tag handling wrong: %+v", got)
	}
}

func TestWholeQueryRefusesNumbersBelowTheDeclaredMinimum(t *testing.T) {
	cases := map[string]string{
		"/x?limit=0":   "limit must be a whole number of at least 1",
		"/x?offset=-1": "offset must be a whole number of at least 0",
		"/x?limit=abc": "limit must be a whole number",
	}
	for target, want := range cases {
		_, err := wholeQuery(reflect.TypeOf(pagedQuery{}), newRequest("GET", target, ""))
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", target, err, want)
		}
	}
}

func TestWholeQueryOnANonStructTakesTheRawQueryString(t *testing.T) {
	v, err := wholeQuery(reflect.TypeOf(""), newRequest("GET", "/x?a=1&b=2", ""))
	if err != nil || v.String() != "a=1&b=2" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestWholeBodyRefusesUnknownFieldsAndWrongTypes(t *testing.T) {
	typ := reflect.TypeOf(openedBody{})
	cases := map[string]string{
		`{"name":"a","extra":1}`: "extra is not a field this request takes",
		`{"name":5}`:             "name must be text",
		`{"name":`:               "the body is not valid JSON",
	}
	for body, want := range cases {
		_, err := wholeBody(typ, newRequest("POST", "/x", body))
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", body, err, want)
		}
	}
	v, err := wholeBody(typ, newRequest("POST", "/x", `{"name":"a"}`))
	if err != nil || v.Interface().(openedBody).Name != "a" {
		t.Fatalf("good body: %v %v", v, err)
	}
}

func TestFromTextParsesNonStringKindsAndNamesTheFieldOnFailure(t *testing.T) {
	v, err := fromText("42", reflect.TypeOf(0), "n")
	if err != nil || v.Int() != 42 {
		t.Fatalf("int: %v %v", v, err)
	}
	v, err = fromText("", reflect.TypeOf(0), "n")
	if err != nil || v.Int() != 0 {
		t.Fatalf("empty int should be zero: %v %v", v, err)
	}
	cases := []struct {
		text string
		typ  reflect.Type
		want string
	}{
		{"x", reflect.TypeOf(0), "n must be a whole number"},
		{"x", reflect.TypeOf(true), "n must be true or false"},
		{"x", reflect.TypeOf(1.5), "n must be a number"},
		{"x", reflect.TypeOf([]string{}), "n must be a list"},
		{"x", reflect.TypeOf(map[string]int{}), "n must be an object"},
		{"x", reflect.TypeOf(&openedBody{}), "n must be an object"},
	}
	for _, c := range cases {
		_, err := fromText(c.text, c.typ, "n")
		if err == nil || err.Error() != c.want {
			t.Errorf("%v: err = %v, want %q", c.typ, err, c.want)
		}
	}
}

func TestFromTextsBuildsSlicesAndTakesTheFirstForScalars(t *testing.T) {
	v, err := fromTexts([]string{"1", "2", "3"}, reflect.TypeOf([]int{}), "n")
	if err != nil || !reflect.DeepEqual(v.Interface(), []int{1, 2, 3}) {
		t.Fatalf("slice: %v %v", v, err)
	}
	if _, err := fromTexts([]string{"1", "x"}, reflect.TypeOf([]int{}), "n"); err == nil {
		t.Fatal("a bad element must fail the whole list")
	}
	v, err = fromTexts([]string{"7", "8"}, reflect.TypeOf(0), "n")
	if err != nil || v.Int() != 7 {
		t.Fatalf("scalar should take first: %v %v", v, err)
	}
	v, err = fromTexts(nil, reflect.TypeOf(0), "n")
	if err != nil || v.Int() != 0 {
		t.Fatalf("scalar of nothing should be zero: %v %v", v, err)
	}
}

func TestParamTypeCountsVariadicTailAndRefusesExtraArguments(t *testing.T) {
	fn := func(context.Context, string, ...int) {}
	typ := reflect.TypeOf(fn)
	got, spread, err := paramType(typ, 1)
	if err != nil || got.Kind() != reflect.String || spread {
		t.Fatalf("fixed: %v %v %v", got, spread, err)
	}
	got, spread, err = paramType(typ, 2)
	if err != nil || got.Kind() != reflect.Int || !spread {
		t.Fatalf("variadic: %v %v %v", got, spread, err)
	}
	got, spread, err = paramType(typ, 5)
	if err != nil || got.Kind() != reflect.Int || !spread {
		t.Fatalf("variadic beyond: %v %v %v", got, spread, err)
	}
	plain := reflect.TypeOf(func(context.Context, string) {})
	if _, _, err := paramType(plain, 2); err == nil || !strings.Contains(err.Error(), "more arguments") {
		t.Fatalf("extra arg: %v", err)
	}
}

func TestCallerOfAnswersEmptyForNoResolver(t *testing.T) {
	if callerOf(context.Background(), nil) != "" {
		t.Fatal("nil caller must be empty")
	}
	who := func(context.Context) string { return "ana" }
	if callerOf(context.Background(), who) != "ana" {
		t.Fatal("caller not resolved")
	}
}

func TestBindOneRequiresAQueryParameterAndTakesAbsentBodyKeysAsZero(t *testing.T) {
	str := reflect.TypeOf("")
	r := newRequest("GET", "/x", "")
	_, err := bindOne(Arg{From: FromQuery, As: "q", Required: true}, str, r, nil, nil, nil)
	if err == nil || err.Error() != "q: the query parameter is required" {
		t.Fatalf("required query: %v", err)
	}
	r = newRequest("GET", "/x?q=", "")
	if _, err := bindOne(Arg{From: FromQuery, As: "q", Required: true}, str, r, nil, nil, nil); err != nil {
		t.Fatalf("present but empty must pass: %v", err)
	}
	v, err := bindOne(Arg{From: FromBody, As: "missing"}, reflect.TypeOf(0), r, map[string]json.RawMessage{}, nil, nil)
	if err != nil || v.Int() != 0 {
		t.Fatalf("absent body key: %v %v", v, err)
	}
}

func TestBindOneReadsCallerAndDeviceFromTheMountNotTheRequest(t *testing.T) {
	str := reflect.TypeOf("")
	r := newRequest("GET", "/x?actor=forged", "")
	caller := func(context.Context) string { return "real-caller" }
	device := func(context.Context) string { return "real-device" }
	v, _ := bindOne(Arg{From: FromCaller, As: "actor"}, str, r, nil, caller, device)
	if v.String() != "real-caller" {
		t.Fatalf("caller = %q", v)
	}
	v, _ = bindOne(Arg{From: FromDevice, As: "device"}, str, r, nil, caller, device)
	if v.String() != "real-device" {
		t.Fatalf("device = %q", v)
	}
}

func TestBodyArgumentOfTheWrongTypeNamesTheArgument(t *testing.T) {
	r := newRequest("POST", "/x", `{"count":"many"}`)
	op := Operation{Args: []Arg{{From: FromBody, As: "count"}}}
	_, err := bind(op, reflect.TypeOf(func(context.Context, int) {}), r, nil, nil)
	if err == nil || err.Error() != "count must be a whole number" {
		t.Fatalf("err = %v", err)
	}
}

func TestBindRefusesABodyThatIsNotAnObject(t *testing.T) {
	r := newRequest("POST", "/x", `[1,2]`)
	op := Operation{Args: []Arg{{From: FromBody, As: "count"}}}
	_, err := bind(op, reflect.TypeOf(func(context.Context, int) {}), r, nil, nil)
	if err == nil || err.Error() != "the request body is not a JSON object" {
		t.Fatalf("err = %v", err)
	}
}

func TestBindSpreadsARepeatedQueryIntoVariadicArguments(t *testing.T) {
	r := newRequest("GET", "/x?id=1&id=2&id=3", "")
	op := Operation{Args: []Arg{{From: FromQuery, As: "id", Repeated: true}}}
	out, err := bind(op, reflect.TypeOf(func(context.Context, ...int) {}), r, nil, nil)
	if err != nil || len(out) != 3 || out[2].Int() != 3 {
		t.Fatalf("out = %v, err = %v", out, err)
	}
	r = newRequest("GET", "/x?id=1&id=x", "")
	if _, err := bind(op, reflect.TypeOf(func(context.Context, ...int) {}), r, nil, nil); err == nil {
		t.Fatal("a bad repeated value must be refused")
	}
}

func TestBindOverwritesWholeBodyFieldsFromPathAndCaller(t *testing.T) {
	op := Operation{Args: []Arg{
		{From: FromBody, Whole: true},
		{From: FromCaller, As: "owner", Into: "Owner"},
		{From: FromPath, As: "name", Into: "Name"},
	}}
	r := newRequest("POST", "/x", `{"name":"forged","owner":"forged"}`)
	r.SetPathValue("name", "from-path")
	caller := func(context.Context) string { return "ana" }
	out, err := bind(op, reflect.TypeOf(func(context.Context, openedBody) {}), r, caller, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].Interface().(openedBody)
	if got.Owner != "ana" || got.Name != "from-path" {
		t.Fatalf("body claims survived: %+v", got)
	}
}

func TestBindRefusesIntoAFieldTheTypeLacks(t *testing.T) {
	op := Operation{Args: []Arg{
		{From: FromBody, Whole: true},
		{From: FromCaller, As: "who", Into: "Nope"},
	}}
	r := newRequest("POST", "/x", `{}`)
	_, err := bind(op, reflect.TypeOf(func(context.Context, openedBody) {}), r, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "has no settable field Nope") {
		t.Fatalf("err = %v", err)
	}
}

func TestBindRefusesASpecDeclaringMoreArgumentsThanTheMethodTakes(t *testing.T) {
	op := Operation{Args: []Arg{{From: FromPath, As: "a"}, {From: FromPath, As: "b"}}}
	_, err := bind(op, reflect.TypeOf(func(context.Context, string) {}), newRequest("GET", "/x", ""), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "more arguments") {
		t.Fatalf("err = %v", err)
	}
}
