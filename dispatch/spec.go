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
	FromPath   Source = "path"
	FromQuery  Source = "query"
	FromBody   Source = "body"
	FromCaller Source = "caller"
	FromDevice Source = "device"
)

var sources = map[Source]bool{
	FromPath: true, FromQuery: true, FromBody: true,
	FromCaller: true, FromDevice: true,
}

var methods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true,
}

var ActionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Arg struct {
	From        Source `json:"from"`
	As          string `json:"as,omitempty"`
	Repeated    bool   `json:"repeated,omitempty"`
	Whole       bool   `json:"whole,omitempty"`
	Into        string `json:"into,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Field       string `json:"field,omitempty"`
	Ignored     bool   `json:"ignored,omitempty"`
	Refers      string `json:"refers,omitempty"`
}

func (a Arg) positional() bool { return a.Into == "" && !a.Ignored }

type Return struct {
	As   string `json:"as,omitempty"`
	Body bool   `json:"body,omitempty"`
	Once bool   `json:"once,omitempty"`
}

type Operation struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	WhenQuery   string   `json:"whenQuery,omitempty"`
	Call        string   `json:"call"`
	Handled     bool     `json:"handled,omitempty"`
	Action      string   `json:"action"`
	Tool        string   `json:"tool,omitempty"`
	NoTool      bool     `json:"-"`
	Status      int      `json:"status,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Args        []Arg    `json:"args,omitempty"`
	Returns     []Return `json:"returns,omitempty"`
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
	seenTool := map[string]string{}

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
		switch {
		case op.Call == "" && !op.Handled:
			add("operation %q names no manager method, and does not declare that the module handles it", name)
		case op.Call != "" && op.Handled:
			add("operation %q both names the manager method %s and claims the module handles it; it is one or the other", name, op.Call)
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

		if op.NoTool && op.Tool != "" {
			add("operation %q both opts out of the tool surface and publishes the tool %q; it is one or the other", name, op.Tool)
		} else if op.Tool != "" && !NamePattern.MatchString(op.Tool) {
			add("operation %q: tool name %q is not a usable name", name, op.Tool)
		} else if op.Tool != "" || (!op.Handled && !op.NoTool && op.Action != "") {
			published := publishedName(op)
			if first, taken := seenTool[published]; taken {
				add("operations %q and %q both publish the tool %q", first, name, published)
			} else {
				seenTool[published] = name
			}
		}

		address := op.Method + " " + op.Path
		if op.WhenQuery != "" {
			if !NamePattern.MatchString(op.WhenQuery) {
				add("operation %q: whenQuery %q is not a usable query parameter name", name, op.WhenQuery)
			}
			if op.Handled {
				add("operation %q is selected by ?%s but the module handles it; a selected alternative is dispatched", name, op.WhenQuery)
			}
		} else if first, taken := seenAddress[address]; taken {
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

	problems = append(problems, s.checkAlternatives()...)

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("dispatch: %s", strings.Join(problems, "; "))
	}
	return nil
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
