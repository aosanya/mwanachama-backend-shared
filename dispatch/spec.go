package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type Source string

const (
	FromPath  Source = "path"
	FromQuery Source = "query"
	FromBody  Source = "body"
)

var sources = map[Source]bool{FromPath: true, FromQuery: true, FromBody: true}

var methods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true,
}

var ActionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Arg struct {
	From     Source `json:"from"`
	As       string `json:"as,omitempty"`
	Repeated bool   `json:"repeated,omitempty"`
	Whole    bool   `json:"whole,omitempty"`
	Into     string `json:"into,omitempty"`
}

func (a Arg) positional() bool { return a.Into == "" }

type Return struct {
	As   string `json:"as,omitempty"`
	Body bool   `json:"body,omitempty"`
	Once bool   `json:"once,omitempty"`
}

type Operation struct {
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	Call    string   `json:"call"`
	Action  string   `json:"action"`
	Status  int      `json:"status,omitempty"`
	Args    []Arg    `json:"args,omitempty"`
	Returns []Return `json:"returns,omitempty"`
}

type Spec struct {
	Base       string               `json:"base,omitempty"`
	Operations map[string]Operation `json:"operations"`
	Errors     map[string]int       `json:"errors,omitempty"`
}

type file struct {
	Base       string               `json:"base"`
	Operations map[string]Operation `json:"operations"`
	Errors     map[string]int       `json:"errors"`
}

func Parse(raw []byte) (*Spec, error) {
	var f file
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("dispatch: %w", err)
	}
	s := &Spec{Base: f.Base, Operations: f.Operations, Errors: f.Errors}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Spec) Validate() error {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if len(s.Operations) == 0 {
		add("a spec declares at least one operation")
	}

	seenAction := map[string]string{}
	seenAddress := map[string]string{}
	seenOnce := map[string]string{}

	for _, name := range sortedKeys(s.Operations) {
		op := s.Operations[name]
		if !NamePattern.MatchString(name) {
			add("operation %q is not a usable name", name)
			continue
		}

		if !methods[op.Method] {
			add("operation %q: method %q is not one this dispatcher routes", name, op.Method)
		}
		if !strings.HasPrefix(op.Path, "/") {
			add("operation %q: path %q does not start with /", name, op.Path)
		}
		if op.Call == "" {
			add("operation %q names no manager method", name)
		}

		if op.Action == "" {
			add("operation %q declares no action; an ungated operation is the one nobody notices", name)
		} else if !ActionPattern.MatchString(op.Action) {
			add("operation %q: action %q is not <module>.<resource>.<verb>", name, op.Action)
		} else if first, taken := seenAction[op.Action]; taken {
			add("operations %q and %q both claim the action %q", first, name, op.Action)
		} else {
			seenAction[op.Action] = name
		}

		address := op.Method + " " + op.Path
		if first, taken := seenAddress[address]; taken {
			add("operations %q and %q both answer %q", first, name, address)
		} else {
			seenAddress[address] = name
		}

		problems = append(problems, op.validate(name)...)

		for _, r := range op.Returns {
			if !r.Once {
				continue
			}
			if op.Method == http.MethodGet {
				add("operation %q returns %q once, but a read cannot be the only chance to see a value", name, r.As)
			}
			if first, taken := seenOnce[r.As]; taken {
				add("operations %q and %q both return %q once", first, name, r.As)
			} else {
				seenOnce[r.As] = name
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("dispatch: %s", strings.Join(problems, "; "))
	}
	return nil
}

func (op Operation) validate(name string) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	declared := pathParams(op.Path)
	bound := map[string]bool{}
	wholes := 0
	overwrites := 0
	for i, a := range op.Args {
		if !sources[a.From] {
			add("operation %q: argument %d comes from %q, which is not path, query or body", name, i, a.From)
		}
		if a.Into != "" {
			if a.From != FromPath {
				add("operation %q: argument %d overwrites %s from the %s, but only the address may outrank the body",
					name, i, a.Into, a.From)
			}
			if a.Whole {
				add("operation %q: argument %d is whole and also overwrites %s", name, i, a.Into)
			}
			if a.As == "" {
				add("operation %q: argument %d overwrites %s from nothing", name, i, a.Into)
			} else if !declared[a.As] {
				add("operation %q: argument %q comes from the path, which declares no {%s}", name, a.As, a.As)
			} else {
				bound[a.As] = true
			}
			overwrites++
			continue
		}
		if a.Whole {
			wholes++
			if a.From == FromPath {
				add("operation %q: argument %d takes the whole path, which is not a value", name, i)
			}
			if a.As != "" {
				add("operation %q: argument %d takes a whole request and also names %q", name, i, a.As)
			}
			if a.Repeated {
				add("operation %q: argument %d is both whole and repeated", name, i)
			}
			continue
		}
		if a.As == "" {
			add("operation %q: argument %d has no name and does not take a whole request", name, i)
			continue
		}
		if a.From == FromPath {
			if !declared[a.As] {
				add("operation %q: argument %q comes from the path, which declares no {%s}", name, a.As, a.As)
			}
			bound[a.As] = true
		}
		if a.Repeated && a.From != FromQuery {
			add("operation %q: argument %q is repeated, which only a query parameter can be", name, a.As)
		}
	}
	for p := range declared {
		if !bound[p] {
			add("operation %q: path declares {%s}, which no argument binds", name, p)
		}
	}
	if wholes > 1 {
		add("operation %q takes %d whole requests, and there is only one", name, wholes)
	}
	if overwrites > 0 && wholes != 1 {
		add("operation %q overwrites a field, but takes no whole request to overwrite it on", name)
	}

	if len(op.Returns) == 0 {
		add("operation %q renders nothing; a return is declared even when it is the whole body", name)
	}
	bodies := 0
	for _, r := range op.Returns {
		if r.Body {
			bodies++
		}
	}
	switch {
	case bodies > 1:
		add("operation %q declares %d whole-body returns", name, bodies)
	case bodies == 1 && len(op.Returns) > 1:
		add("operation %q mixes a whole-body return with named ones", name)
	case bodies == 0:
		for i, r := range op.Returns {
			if r.As == "" {
				add("operation %q: return %d has no name and is not the body", name, i)
			}
		}
	}

	if op.Status != 0 && (op.Status < 100 || op.Status > 599) {
		add("operation %q: status %d is not an HTTP status", name, op.Status)
	}
	return problems
}

func pathParams(path string) map[string]bool {
	out := map[string]bool{}
	for _, seg := range strings.Split(path, "/") {
		if len(seg) > 2 && strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			out[seg[1:len(seg)-1]] = true
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (op Operation) status() int {
	if op.Status != 0 {
		return op.Status
	}
	return http.StatusOK
}
