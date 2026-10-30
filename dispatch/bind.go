package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

func bind(op Operation, t reflect.Type, r *http.Request, caller, device Caller) ([]reflect.Value, error) {
	var body map[string]json.RawMessage
	if needsBody(op) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("the request body is not a JSON object")
		}
	}

	out := make([]reflect.Value, 0, len(op.Args))
	whole := -1
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
		if a.Whole {
			v, err := wholeValue(a, want, r)
			if err != nil {
				return nil, err
			}
			whole = len(out)
			out = append(out, v)
			continue
		}
		if a.Repeated && spread {
			for _, s := range r.URL.Query()[a.As] {
				v, err := fromText(s, want, a.As)
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			}
			continue
		}
		v, err := bindOne(a, want, r, body, caller, device)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	if whole >= 0 {
		if err := overwrite(op, out[whole], r, caller, device); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func overwrite(op Operation, target reflect.Value, r *http.Request, caller, device Caller) error {
	for _, a := range op.Args {
		if a.Into == "" {
			continue
		}
		field := target.FieldByName(a.Into)
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%s has no settable field %s", target.Type(), a.Into)
		}
		text := r.PathValue(a.As)
		switch a.From {
		case FromCaller:
			text = callerOf(r.Context(), caller)
		case FromDevice:
			text = callerOf(r.Context(), device)
		}
		v, err := fromText(text, field.Type(), a.As)
		if err != nil {
			return err
		}
		field.Set(v)
	}
	return nil
}

func needsBody(op Operation) bool {
	for _, a := range op.Args {
		if a.From == FromBody && !a.Whole {
			return true
		}
	}
	return false
}

func wholeBody(want reflect.Type, r *http.Request) (reflect.Value, error) {
	v := reflect.New(want)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("the request body is not a valid %s: %v", want, err)
	}
	return v.Elem(), nil
}

func wholeQuery(want reflect.Type, r *http.Request) (reflect.Value, error) {
	if want.Kind() != reflect.Struct {
		return fromText(r.URL.RawQuery, want, "query")
	}
	v := reflect.New(want).Elem()
	query := r.URL.Query()
	for i := 0; i < want.NumField(); i++ {
		f := want.Field(i)
		if f.PkgPath != "" {
			continue
		}
		key := queryKey(f)
		if key == "" || !query.Has(key) {
			continue
		}
		field, err := fromTexts(query[key], f.Type, key)
		if err != nil {
			return reflect.Value{}, err
		}
		v.Field(i).Set(field)
	}
	return v, nil
}

func queryKey(f reflect.StructField) string {
	tag, ok := f.Tag.Lookup("query")
	if ok {
		if tag == "-" {
			return ""
		}
		return strings.Split(tag, ",")[0]
	}
	return snakeCase(f.Name)
}

func snakeCase(field string) string {
	var b strings.Builder
	runes := []rune(field)
	for i, r := range runes {
		upper := r >= 'A' && r <= 'Z'
		if upper && i > 0 {
			prevLower := runes[i-1] >= 'a' && runes[i-1] <= 'z' || runes[i-1] >= '0' && runes[i-1] <= '9'
			plural := i+2 == len(runes) && runes[i+1] == 's'
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' && !plural
			if prevLower || nextLower {
				b.WriteByte('_')
			}
		}
		if upper {
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func paramType(t reflect.Type, i int) (reflect.Type, bool, error) {
	if t.IsVariadic() && i >= t.NumIn()-1 {
		return t.In(t.NumIn() - 1).Elem(), true, nil
	}
	if i >= t.NumIn() {
		return nil, false, fmt.Errorf("the spec declares more arguments than the method takes")
	}
	return t.In(i), false, nil
}

func wholeValue(a Arg, want reflect.Type, r *http.Request) (reflect.Value, error) {
	if a.From == FromQuery {
		return wholeQuery(want, r)
	}
	return wholeBody(want, r)
}

func bindOne(a Arg, want reflect.Type, r *http.Request, body map[string]json.RawMessage, caller, device Caller) (reflect.Value, error) {
	switch a.From {
	case FromCaller:
		return fromText(callerOf(r.Context(), caller), want, a.As)
	case FromDevice:
		return fromText(callerOf(r.Context(), device), want, a.As)
	case FromPath:
		return fromText(r.PathValue(a.As), want, a.As)
	case FromQuery:
		if a.Repeated {
			return fromTexts(r.URL.Query()[a.As], want, a.As)
		}
		return fromText(r.URL.Query().Get(a.As), want, a.As)
	default:
		raw, ok := body[a.As]
		if !ok {
			return reflect.Zero(want), nil
		}
		v := reflect.New(want)
		if err := json.Unmarshal(raw, v.Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("%s: %v", a.As, err)
		}
		return v.Elem(), nil
	}
}

func fromText(s string, want reflect.Type, name string) (reflect.Value, error) {
	if want.Kind() == reflect.String {
		v := reflect.New(want).Elem()
		v.SetString(s)
		return v, nil
	}
	if s == "" {
		return reflect.Zero(want), nil
	}
	v := reflect.New(want)
	if err := json.Unmarshal([]byte(s), v.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("%s: %q is not a %s", name, s, want)
	}
	return v.Elem(), nil
}

func fromTexts(values []string, want reflect.Type, name string) (reflect.Value, error) {
	if want.Kind() == reflect.Slice {
		out := reflect.MakeSlice(want, 0, len(values))
		for _, s := range values {
			v, err := fromText(s, want.Elem(), name)
			if err != nil {
				return reflect.Value{}, err
			}
			out = reflect.Append(out, v)
		}
		return out, nil
	}
	if len(values) == 0 {
		return reflect.Zero(want), nil
	}
	return fromText(values[0], want, name)
}

func callerOf(ctx context.Context, caller Caller) string {
	if caller == nil {
		return ""
	}
	return caller(ctx)
}
