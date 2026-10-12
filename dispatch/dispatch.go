package dispatch

import (
	"context"
	"errors"
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

	Fields map[string]FieldDoc

	Authorize Authorizer

	Caller Caller

	Device Caller
}

type Authorizer func(ctx context.Context, action string) error

type Caller func(ctx context.Context) string

var ErrForbidden = errors.New("dispatch: the caller may not perform that action")

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
		if op.Handled {
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
		out = append(out, Route{
			Method:  op.Method,
			Path:    s.Base + op.Path,
			Action:  op.Action,
			Handler: handlerFor(op, method, table, fallback, d.Authorize, d.Caller, d.Device),
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

func handlerFor(op Operation, method reflect.Value, table map[error]int, fallback int, authorize Authorizer, caller, device Caller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authorize != nil {
			if err := authorize(r.Context(), op.Action); err != nil {
				httpwire.WriteErr(w, http.StatusForbidden, ErrForbidden.Error())
				return
			}
		}

		ctx, err := conversationFromHeader(r)
		if err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		r = r.WithContext(ctx)

		args, err := bind(op, method.Type(), r, caller, device)
		if err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}

		results := method.Call(append([]reflect.Value{reflect.ValueOf(ctx)}, args...))
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
