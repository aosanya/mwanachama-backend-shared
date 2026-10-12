package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

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

func fromSession(a Arg) bool { return a.From == FromCaller || a.From == FromDevice }

func sessionOf(a Arg, caller, device Caller) Caller {
	if a.From == FromDevice {
		return device
	}
	return caller
}

func bindTool(ctx context.Context, op Operation, plan []slot, in map[string]json.RawMessage, caller, device Caller) ([]reflect.Value, error) {
	out := make([]reflect.Value, 0, len(plan))
	whole := -1
	for _, sl := range plan {
		if fromSession(sl.arg) {
			v, err := fromText(callerOf(ctx, sessionOf(sl.arg, caller, device)), sl.typ, sl.arg.As)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			continue
		}
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
		if err := overwriteFromArgs(ctx, op, out[whole], in, caller, device); err != nil {
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

func overwriteFromArgs(ctx context.Context, op Operation, target reflect.Value, in map[string]json.RawMessage, caller, device Caller) error {
	for _, a := range op.Args {
		if a.Into == "" {
			continue
		}
		field := target.FieldByName(a.Into)
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%s has no settable field %s", target.Type(), a.Into)
		}
		if fromSession(a) {
			v, err := fromText(callerOf(ctx, sessionOf(a, caller, device)), field.Type(), a.As)
			if err != nil {
				return err
			}
			field.Set(v)
			continue
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
