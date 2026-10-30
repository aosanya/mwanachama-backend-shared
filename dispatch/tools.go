package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
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
	Handle      func(context.Context, Request) (any, error)
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
		if op.Handled || op.NoTool {
			continue
		}
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
		handle := invokerFor(op, plan, props, method, table, fallback, d.Authorize, d.Caller, d.Device)
		out = append(out, Tool{
			Name:        publishedName(op),
			Title:       op.Title,
			Description: op.Description,
			Action:      op.Action,
			InputSchema: schema,
			Invoke:      argumentsOnly(handle),
			Handle:      handle,
		})
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("dispatch: %s", strings.Join(problems, "; "))
	}
	return out, nil
}

func toolName(action string) string { return strings.ReplaceAll(action, ".", "_") }

func publishedName(op Operation) string {
	if op.Tool != "" {
		return op.Tool
	}
	return toolName(op.Action)
}

func argumentsOnly(handle func(context.Context, Request) (any, error)) func(context.Context, json.RawMessage) (any, error) {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handle(ctx, Request{Arguments: raw})
	}
}

func invokerFor(op Operation, plan []slot, props []property, method reflect.Value, table map[error]int, fallback int, authorize Authorizer, caller, device Caller) func(context.Context, Request) (any, error) {
	known := map[string]bool{}
	required := map[string]bool{}
	for _, p := range props {
		known[p.key] = true
		required[p.key] = p.required
	}

	return func(ctx context.Context, req Request) (any, error) {
		if authorize != nil {
			if err := authorize(ctx, op.Action); err != nil {
				return nil, ErrForbidden
			}
		}

		ctx, err := conversationFromMeta(ctx, req.Meta)
		if err != nil {
			return nil, err
		}

		in, err := arguments(req.Arguments)
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

		args, err := bindTool(ctx, op, plan, in, caller, device)
		if err != nil {
			return nil, err
		}

		results := method.Call(append([]reflect.Value{reflect.ValueOf(ctx)}, args...))
		if last := results[len(results)-1]; !last.IsNil() {
			callErr := last.Interface().(error)
			if resolve(callErr, table, fallback).code == fallback {
				return nil, errors.New(opaque)
			}
			return nil, callErr
		}
		return rendered(op, results[:len(results)-1]), nil
	}
}
