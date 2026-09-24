package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"
)

type FieldDoc struct {
	Description string
	Required    bool
	ReadOnly    bool
	Values      []string
}

type Tool struct {
	Name        string
	Title       string
	Description string
	Action      string
	InputSchema json.RawMessage
	Invoke      func(context.Context, json.RawMessage) (any, error)
}

type slot struct {
	arg    Arg
	typ    reflect.Type
	spread bool
}

type property struct {
	key      string
	schema   map[string]any
	required bool
}

func Tools(s *Spec, d Deps) ([]Tool, error) {
	if s == nil {
		return nil, errors.New("dispatch: spec must not be nil")
	}
	if d.Manager == nil {
		return nil, errors.New("dispatch: manager must not be nil")
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}

	table, err := statusTable(s, d)
	if err != nil {
		return nil, err
	}

	mgr := reflect.ValueOf(d.Manager)
	fallback := d.Fallback
	if fallback == 0 {
		fallback = 500
	}

	var out []Tool
	var problems []string
	for _, name := range sortedKeys(s.Operations) {
		op := s.Operations[name]
		if strings.TrimSpace(op.Description) == "" {
			problems = append(problems, fmt.Sprintf(
				"operation %q has no description, and a tool nobody can read is a tool nobody calls", name))
			continue
		}
		method := mgr.MethodByName(op.Call)
		if !method.IsValid() {
			problems = append(problems, fmt.Sprintf(
				"operation %q calls %s, which %T does not have", name, op.Call, d.Manager))
			continue
		}
		if err := checkSignature(name, op, method.Type()); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		plan, err := planFor(op, method.Type())
		if err != nil {
			problems = append(problems, fmt.Sprintf("operation %q: %v", name, err))
			continue
		}
		props, err := properties(op, plan, d.Fields)
		if err != nil {
			problems = append(problems, fmt.Sprintf("operation %q: %v", name, err))
			continue
		}
		schema, err := json.Marshal(object(props))
		if err != nil {
			problems = append(problems, fmt.Sprintf("operation %q: %v", name, err))
			continue
		}
		out = append(out, Tool{
			Name:        toolName(op.Action),
			Title:       op.Title,
			Description: op.Description,
			Action:      op.Action,
			InputSchema: schema,
			Invoke:      invokerFor(op, plan, props, method, table, fallback),
		})
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("dispatch: %s", strings.Join(problems, "; "))
	}
	return out, nil
}

func toolName(action string) string { return strings.ReplaceAll(action, ".", "_") }

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
		if a.positional() {
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

func invokerFor(op Operation, plan []slot, props []property, method reflect.Value, table map[error]int, fallback int) func(context.Context, json.RawMessage) (any, error) {
	known := map[string]bool{}
	required := map[string]bool{}
	for _, p := range props {
		known[p.key] = true
		required[p.key] = p.required
	}

	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		in, err := arguments(raw)
		if err != nil {
			return nil, err
		}
		for key := range in {
			if !known[key] {
				return nil, fmt.Errorf("%s takes no argument %q", toolName(op.Action), key)
			}
		}
		for key := range required {
			if required[key] && len(in[key]) == 0 {
				return nil, fmt.Errorf("%s needs %q", toolName(op.Action), key)
			}
		}

		args, err := bindTool(op, plan, in)
		if err != nil {
			return nil, err
		}

		results := method.Call(append([]reflect.Value{reflect.ValueOf(ctx)}, args...))
		if last := results[len(results)-1]; !last.IsNil() {
			callErr := last.Interface().(error)
			if httpwire.StatusFor(callErr, table, fallback) == fallback {
				return nil, errors.New(opaque)
			}
			return nil, callErr
		}
		return rendered(op, results[:len(results)-1]), nil
	}
}

func arguments(raw json.RawMessage) (map[string]json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return map[string]json.RawMessage{}, nil
	}
	var in map[string]json.RawMessage
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, errors.New("the arguments are not a JSON object")
	}
	return in, nil
}

func bindTool(op Operation, plan []slot, in map[string]json.RawMessage) ([]reflect.Value, error) {
	out := make([]reflect.Value, 0, len(plan))
	whole := -1
	for _, sl := range plan {
		if sl.arg.Whole {
			v, err := wholeFromArgs(op, sl, in)
			if err != nil {
				return nil, err
			}
			whole = len(out)
			out = append(out, v)
			continue
		}
		if sl.arg.Repeated && sl.spread {
			values, err := spreadFromArgs(sl, in)
			if err != nil {
				return nil, err
			}
			out = append(out, values...)
			continue
		}
		v, err := valueFromArgs(sl.typ, in[sl.arg.As], sl.arg.As)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	if whole >= 0 {
		if err := overwriteFromArgs(op, out[whole], in); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func wholeFromArgs(op Operation, sl slot, in map[string]json.RawMessage) (reflect.Value, error) {
	if sl.typ.Kind() != reflect.Struct {
		return valueFromArgs(sl.typ, in[sl.arg.As], sl.arg.As)
	}
	body := map[string]json.RawMessage{}
	for i := 0; i < sl.typ.NumField(); i++ {
		f := sl.typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		key := jsonKey(f)
		if sl.arg.From == FromQuery {
			key = queryKey(f)
		}
		if key == "" {
			continue
		}
		if raw, ok := in[key]; ok {
			body[jsonKey(f)] = raw
		}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return reflect.Value{}, err
	}
	v := reflect.New(sl.typ)
	if err := json.Unmarshal(encoded, v.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("the arguments are not a valid %s: %v", sl.typ, err)
	}
	return v.Elem(), nil
}

func spreadFromArgs(sl slot, in map[string]json.RawMessage) ([]reflect.Value, error) {
	raw, ok := in[sl.arg.As]
	if !ok {
		return nil, nil
	}
	items, err := listOfOne(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", sl.arg.As, err)
	}
	out := make([]reflect.Value, 0, len(items))
	for _, item := range items {
		v, err := valueFromArgs(sl.typ, item, sl.arg.As)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func listOfOne(raw json.RawMessage) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return items, nil
	}
	var single any
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, errors.New("neither a list nor a value")
	}
	return []json.RawMessage{raw}, nil
}

func overwriteFromArgs(op Operation, target reflect.Value, in map[string]json.RawMessage) error {
	for _, a := range op.Args {
		if a.Into == "" {
			continue
		}
		field := target.FieldByName(a.Into)
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%s has no settable field %s", target.Type(), a.Into)
		}
		v, err := valueFromArgs(field.Type(), in[a.As], a.As)
		if err != nil {
			return err
		}
		field.Set(v)
	}
	return nil
}

func valueFromArgs(want reflect.Type, raw json.RawMessage, name string) (reflect.Value, error) {
	if len(raw) == 0 {
		return reflect.Zero(want), nil
	}
	v := reflect.New(want)
	if err := json.Unmarshal(raw, v.Interface()); err != nil {
		if want.Kind() == reflect.Slice {
			return oneElementSlice(want, raw, name)
		}
		return reflect.Value{}, fmt.Errorf("%s: %v", name, err)
	}
	return v.Elem(), nil
}

func oneElementSlice(want reflect.Type, raw json.RawMessage, name string) (reflect.Value, error) {
	element := reflect.New(want.Elem())
	if err := json.Unmarshal(raw, element.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("%s: %v", name, err)
	}
	return reflect.Append(reflect.MakeSlice(want, 0, 1), element.Elem()), nil
}

func rendered(op Operation, values []reflect.Value) any {
	if len(values) == 0 {
		return nil
	}
	if len(op.Returns) == 1 && op.Returns[0].Body {
		return values[0].Interface()
	}
	body := make(map[string]any, len(op.Returns))
	for i, ret := range op.Returns {
		if i < len(values) {
			body[ret.As] = values[i].Interface()
		}
	}
	return body
}
