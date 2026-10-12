package dispatch

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

func planFor(op Operation, t reflect.Type) ([]slot, error) {
	var out []slot
	position := 0
	for _, a := range op.Args {
		if !a.positional() {
			continue
		}
		want, spread, err := paramType(t, position+1)
		if err != nil {
			return nil, err
		}
		position++
		out = append(out, slot{arg: a, typ: want, spread: spread})
	}
	return out, nil
}

func properties(op Operation, plan []slot, docs map[string]FieldDoc) ([]property, error) {
	var out []property
	seen := map[string]bool{}
	add := func(p property) error {
		if seen[p.key] {
			return fmt.Errorf("two arguments both arrive as %q", p.key)
		}
		seen[p.key] = true
		out = append(out, p)
		return nil
	}

	for _, sl := range plan {
		if fromSession(sl.arg) {
			continue
		}
		if !sl.arg.Whole {
			doc := docs[sl.arg.Field]
			schema := schemaFor(sl.typ, docs, 0)
			if len(doc.Values) > 0 {
				schema["enum"] = doc.Values
			}
			if sl.spread && sl.arg.Repeated {
				schema = map[string]any{"type": "array", "items": schema}
			}
			describe(schema, firstOf(sl.arg.Description, doc.Description))
			if err := add(property{
				key:      sl.arg.As,
				schema:   schema,
				required: sl.arg.Required || sl.arg.From == FromPath,
			}); err != nil {
				return nil, err
			}
			continue
		}
		fields, err := flatten(op, sl, docs)
		if err != nil {
			return nil, err
		}
		for _, p := range fields {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}

	for _, a := range op.Args {
		if a.positional() || a.Ignored || fromSession(a) {
			continue
		}
		target, ok := wholeStruct(plan)
		if !ok {
			return nil, fmt.Errorf("argument %q overwrites %s, but no whole request was bound", a.As, a.Into)
		}
		field, ok := target.FieldByName(a.Into)
		if !ok {
			return nil, fmt.Errorf("%s has no field %s", target, a.Into)
		}
		doc, ok := docs[a.Field]
		if !ok {
			doc = docs[target.Name()+"."+jsonKey(field)]
		}
		schema := schemaFor(field.Type, docs, 0)
		if len(doc.Values) > 0 {
			schema["enum"] = doc.Values
		}
		describe(schema, firstOf(a.Description, doc.Description))
		if err := add(property{key: a.As, schema: schema, required: true}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func flatten(op Operation, sl slot, docs map[string]FieldDoc) ([]property, error) {
	if sl.typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("a whole %s is not a struct and cannot be spread over named arguments", sl.typ)
	}
	overwritten := map[string]bool{}
	for _, a := range op.Args {
		if a.Into != "" {
			overwritten[a.Into] = true
		}
	}

	var out []property
	for i := 0; i < sl.typ.NumField(); i++ {
		f := sl.typ.Field(i)
		if f.PkgPath != "" || overwritten[f.Name] {
			continue
		}
		key := jsonKey(f)
		if sl.arg.From == FromQuery {
			key = queryKey(f)
		}
		if key == "" {
			continue
		}
		doc := docs[sl.typ.Name()+"."+key]
		if doc.ReadOnly {
			continue
		}
		schema := schemaFor(f.Type, docs, 0)
		if len(doc.Values) > 0 {
			schema["enum"] = doc.Values
		}
		describe(schema, doc.Description)
		out = append(out, property{key: key, schema: schema, required: doc.Required})
	}
	return out, nil
}

func wholeStruct(plan []slot) (reflect.Type, bool) {
	for _, sl := range plan {
		if sl.arg.Whole && sl.typ.Kind() == reflect.Struct {
			return sl.typ, true
		}
	}
	return nil, false
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func describe(schema map[string]any, text string) {
	if text != "" {
		schema["description"] = text
	}
}

func object(props []property) map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, p := range props {
		properties[p.key] = p.schema
		if p.required {
			required = append(required, p.key)
		}
	}
	sort.Strings(required)
	out := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

var timeType = reflect.TypeOf(time.Time{})

var rawType = reflect.TypeOf(json.RawMessage{})

func schemaFor(t reflect.Type, docs map[string]FieldDoc, depth int) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t == rawType:
		return map[string]any{}
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string"}
		}
		return map[string]any{"type": "array", "items": schemaFor(t.Elem(), docs, depth+1)}
	case reflect.Map:
		return map[string]any{"type": "object"}
	case reflect.Struct:
		if depth >= 3 {
			return map[string]any{"type": "object"}
		}
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			key := jsonKey(f)
			if key == "" {
				continue
			}
			doc := docs[t.Name()+"."+key]
			if doc.ReadOnly {
				continue
			}
			field := schemaFor(f.Type, docs, depth+1)
			if len(doc.Values) > 0 {
				field["enum"] = doc.Values
			}
			describe(field, doc.Description)
			properties[key] = field
			if doc.Required {
				required = append(required, key)
			}
		}
		out := map[string]any{"type": "object", "properties": properties}
		if len(required) > 0 {
			sort.Strings(required)
			out["required"] = required
		}
		return out
	default:
		return map[string]any{}
	}
}

func jsonKey(f reflect.StructField) string {
	tag, ok := f.Tag.Lookup("json")
	if !ok {
		return f.Name
	}
	name := strings.Split(tag, ",")[0]
	switch name {
	case "-":
		return ""
	case "":
		return f.Name
	}
	return name
}
