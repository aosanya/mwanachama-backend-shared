package dispatch

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type sampleBody struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Tags  []string `json:"tags"`
	Inner struct {
		Text string `json:"text"`
	} `json:"inner"`
}

func TestDescribeDecodeError_NamesTheFieldAndNeverTheServersTypes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{{"number for text", `{"name":42}`, "name must be text"}, {"text for number", `{"count":"x"}`, "count must be a whole number"}, {"object for list", `{"tags":{}}`, "tags must be a list"}, {"text for object", `{"inner":"x"}`, "inner must be an object"}, {"nested field", `{"inner":{"text":1}}`, "inner.text must be text"}, {"unknown field", `{"colour":"red"}`, "colour is not a field this request takes"}, {"broken json", `{"name":`, "the body is not valid JSON"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(c.body))
			_, err := wholeBody(reflect.TypeOf(sampleBody{}), r)
			if err == nil {
				t.Fatal("want an error")
			}
			if err.Error() != c.want {
				t.Fatalf("got %q, want %q", err.Error(), c.want)
			}
			assertNoInternals(t, err.Error())
		})
	}
}
func TestDescribeDecodeError_ToolArgumentsNameTheFieldToo(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		raw  string
		want string
	}{{"whole struct", reflect.TypeOf(sampleBody{}), `{"name":42}`, ""}, {"one value", reflect.TypeOf(0), `"x"`, "count must be a whole number"}, {"list element", reflect.TypeOf([]int{}), `"x"`, "ids must be a whole number"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "whole struct" {
				_, err := wholeFromArgs(Operation{}, slot{arg: Arg{As: "doc", Whole: true}, typ: c.typ}, map[string]json.RawMessage{"name": json.RawMessage("42")})
				if err == nil || err.Error() != "name must be text" {
					t.Fatalf("got %v", err)
				}
				assertNoInternals(t, err.Error())
				return
			}
			name := "count"
			if c.name == "list element" {
				name = "ids"
			}
			_, err := valueFromArgs(c.typ, json.RawMessage(c.raw), name)
			if err == nil || err.Error() != c.want {
				t.Fatalf("got %v, want %q", err, c.want)
			}
			assertNoInternals(t, err.Error())
		})
	}
}
func TestFromText_NamesWhatTheValueMustBe(t *testing.T) {
	_, err := fromText("abc", reflect.TypeOf(0), "limit")
	if err == nil || err.Error() != "limit must be a whole number" {
		t.Fatalf("got %v", err)
	}
}
func assertNoInternals(t *testing.T, text string) {
	t.Helper()
	for _, leak := range []string{"models.", "Go struct", "Go value", "json:", "dispatch."} {
		if strings.Contains(text, leak) {
			t.Fatalf("%q names the server's internals (%q)", text, leak)
		}
	}
}
