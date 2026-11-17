package dispatch

import (
	"encoding/json"
	"reflect"
	"testing"
)

type EmbeddedInnerExported struct {
	Title string `json:"title"`
}

type embeddedWithExported struct {
	EmbeddedInnerExported
	Todos []string `json:"todos"`
}

type embeddedNamed struct {
	EmbeddedInnerExported `json:"issue"`
	Todos                 []string `json:"todos"`
}

func keysOf(t reflect.Type) []string {
	var keys []string
	for _, at := range fieldsOf(t) {
		keys = append(keys, jsonKey(at.field))
	}
	return keys
}

func TestAnEmbeddedStructIsFlattenedIntoItsParentsFields(t *testing.T) {
	if got := keysOf(reflect.TypeOf(embeddedWithExported{})); !reflect.DeepEqual(got, []string{"title", "todos"}) {
		t.Errorf("keys = %v, want title and todos", got)
	}
}

func TestAnEmbeddedStructWithAJSONNameStaysOneField(t *testing.T) {
	if got := keysOf(reflect.TypeOf(embeddedNamed{})); !reflect.DeepEqual(got, []string{"issue", "todos"}) {
		t.Errorf("keys = %v, want issue and todos", got)
	}
}

func TestAFlattenedFieldIsMarkedEmbeddedAndAnOwnFieldIsNot(t *testing.T) {
	for _, at := range fieldsOf(reflect.TypeOf(embeddedWithExported{})) {
		if want := jsonKey(at.field) == "title"; at.embedded != want {
			t.Errorf("%s embedded = %v, want %v", jsonKey(at.field), at.embedded, want)
		}
	}
}

func TestTheSchemaOfAStructWithAnEmbeddedStructListsTheEmbeddedFieldsAtTheTop(t *testing.T) {
	var schema struct {
		Properties map[string]any `json:"properties"`
	}
	raw, err := json.Marshal(schemaFor(reflect.TypeOf(embeddedWithExported{}), nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"title", "todos"} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("the schema lacks %q: %s", key, raw)
		}
	}
	if _, ok := schema.Properties["EmbeddedInnerExported"]; ok {
		t.Errorf("the schema nests the embedded struct: %s", raw)
	}
}
