package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"
)

type Route = httpwire.Route

type Deps struct {
	Manager any

	Errors map[string]error

	Fallback int
}

var ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()

var errType = reflect.TypeOf((*error)(nil)).Elem()

func Dispatch(s *Spec, d Deps) ([]Route, error) {
	if s == nil {
		return nil, fmt.Errorf("dispatch: spec must not be nil")
	}
	if d.Manager == nil {
		return nil, fmt.Errorf("dispatch: manager must not be nil")
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
		fallback = http.StatusInternalServerError
	}

	var out []Route
	var problems []string
	for _, name := range sortedKeys(s.Operations) {
		op := s.Operations[name]
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
		out = append(out, Route{
			Method:  op.Method,
			Path:    s.Base + op.Path,
			Action:  op.Action,
			Handler: handlerFor(op, method, table, fallback),
		})
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("dispatch: %s", strings.Join(problems, "; "))
	}
	return out, nil
}

func statusTable(s *Spec, d Deps) (map[error]int, error) {
	table := map[error]int{}
	var missing []string
	for _, name := range sortedKeys(s.Errors) {
		sentinel, ok := d.Errors[name]
		if !ok || sentinel == nil {
			missing = append(missing, name)
			continue
		}
		table[sentinel] = s.Errors[name]
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"dispatch: the spec maps %s to a status, but no sentinel was supplied for them",
			strings.Join(missing, ", "))
	}
	return table, nil
}

func checkSignature(name string, op Operation, t reflect.Type) error {
	if t.NumIn() < 1 || t.In(0) != ctxType {
		return fmt.Errorf("operation %q: %s does not take a context first", name, op.Call)
	}
	declared := 0
	for _, a := range op.Args {
		if a.positional() {
			declared++
		}
	}
	want := t.NumIn() - 1
	if t.IsVariadic() {
		if declared < want-1 {
			return fmt.Errorf("operation %q: %s takes at least %d arguments, the spec declares %d",
				name, op.Call, want-1, declared)
		}
	} else if declared != want {
		return fmt.Errorf("operation %q: %s takes %d arguments, the spec declares %d",
			name, op.Call, want, declared)
	}

	if t.NumOut() < 1 || t.Out(t.NumOut()-1) != errType {
		return fmt.Errorf("operation %q: %s does not return an error last", name, op.Call)
	}
	values := t.NumOut() - 1
	if len(op.Returns) != values {
		if !(values == 0 && len(op.Returns) == 1 && op.Returns[0].Body) {
			return fmt.Errorf("operation %q: %s returns %d values, the spec renders %d",
				name, op.Call, values, len(op.Returns))
		}
	}
	return nil
}

const opaque = "internal error"

func handlerFor(op Operation, method reflect.Value, table map[error]int, fallback int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		args, err := bind(op, method.Type(), r)
		if err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}

		results := method.Call(append([]reflect.Value{reflect.ValueOf(r.Context())}, args...))
		if last := results[len(results)-1]; !last.IsNil() {
			callErr := last.Interface().(error)
			code := httpwire.StatusFor(callErr, table, fallback)
			if code == fallback {
				httpwire.WriteErr(w, code, opaque)
				return
			}
			httpwire.WriteErr(w, code, callErr.Error())
			return
		}

		render(w, op, results[:len(results)-1])
	}
}

func render(w http.ResponseWriter, op Operation, values []reflect.Value) {
	if len(values) == 0 {
		w.WriteHeader(op.status())
		return
	}
	if len(op.Returns) == 1 && op.Returns[0].Body {
		httpwire.WriteJSON(w, op.status(), values[0].Interface())
		return
	}
	body := make(map[string]any, len(op.Returns))
	for i, ret := range op.Returns {
		if i < len(values) {
			body[ret.As] = values[i].Interface()
		}
	}
	httpwire.WriteJSON(w, op.status(), body)
}

func bind(op Operation, t reflect.Type, r *http.Request) ([]reflect.Value, error) {
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
		v, err := bindOne(a, want, r, body)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	if whole >= 0 {
		if err := overwrite(op, out[whole], r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func overwrite(op Operation, target reflect.Value, r *http.Request) error {
	for _, a := range op.Args {
		if a.Into == "" {
			continue
		}
		field := target.FieldByName(a.Into)
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%s has no settable field %s", target.Type(), a.Into)
		}
		v, err := fromText(r.PathValue(a.As), field.Type(), a.As)
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
	return strings.ToLower(f.Name)
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

func bindOne(a Arg, want reflect.Type, r *http.Request, body map[string]json.RawMessage) (reflect.Value, error) {
	switch a.From {
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
