package spec

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type Setting struct {
	Name        string    `json:"name"`
	Type        FieldType `json:"type"`
	Description string    `json:"description"`
	Values      []string  `json:"values,omitempty"`
	Default     any       `json:"default,omitempty"`
}

type Layer string

const (
	LayerModule     Layer = "module"
	LayerDefinition Layer = "definition"
	LayerOverride   Layer = "override"
)

type Override struct {
	Value any    `json:"value"`
	By    string `json:"by"`
	At    string `json:"at"`
}

type Effective struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
	Layer Layer  `json:"layer"`
	By    string `json:"by,omitempty"`
	At    string `json:"at,omitempty"`
}

var settingTypes = map[FieldType]bool{
	TypeString: true, TypeText: true, TypeInt: true, TypeFloat: true,
	TypeBool: true, TypeEnum: true, TypeJSON: true,
}

func (s *Spec) Setting(name string) (Setting, bool) {
	return findSetting(s.Settings, name)
}

func findSetting(settings []Setting, name string) (Setting, bool) {
	for _, st := range settings {
		if st.Name == name {
			return st, true
		}
	}
	return Setting{}, false
}

func settingsProblems(settings []Setting) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	seen := map[string]bool{}
	for _, st := range settings {
		if !NamePattern.MatchString(st.Name) {
			add("setting %q is not a usable name", st.Name)
			continue
		}
		if seen[st.Name] {
			add("setting %q is declared twice", st.Name)
		}
		seen[st.Name] = true
		if !settingTypes[st.Type] {
			add("setting %q has the type %q, which a setting cannot hold", st.Name, st.Type)
			continue
		}
		if strings.TrimSpace(st.Description) == "" {
			add("setting %q has no description", st.Name)
		}
		if st.Type == TypeEnum && len(st.Values) == 0 {
			add("setting %q is an enum with no values", st.Name)
		}
		if st.Type != TypeEnum && len(st.Values) > 0 {
			add("setting %q declares values, and only an enum has values", st.Name)
		}
		if st.Default == nil {
			add("setting %q has no default, and the module's value is the layer every other one falls back to", st.Name)
			continue
		}
		if problem := st.check(st.Default); problem != "" {
			add("setting %q default: %s", st.Name, problem)
		}
	}
	return problems
}

func (st Setting) check(value any) string {
	switch st.Type {
	case TypeString, TypeText:
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("%v is not a string", value)
		}
	case TypeEnum:
		v, ok := value.(string)
		if !ok || !holds(st.Values, v) {
			return fmt.Sprintf("%v is not one of %s", value, strings.Join(st.Values, ", "))
		}
	case TypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("%v is not true or false", value)
		}
	case TypeInt:
		n, ok := number(value)
		if !ok || n != math.Trunc(n) {
			return fmt.Sprintf("%v is not a whole number", value)
		}
	case TypeFloat:
		if _, ok := number(value); !ok {
			return fmt.Sprintf("%v is not a number", value)
		}
	case TypeJSON:
		if _, err := json.Marshal(value); err != nil {
			return fmt.Sprintf("%v is not a document: %v", value, err)
		}
	}
	return ""
}

func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

func CheckSettings(settings []Setting, values map[string]any) []string {
	var problems []string
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		st, ok := findSetting(settings, name)
		if !ok {
			problems = append(problems, fmt.Sprintf("setting %q is not declared by this module", name))
			continue
		}
		if problem := st.check(values[name]); problem != "" {
			problems = append(problems, fmt.Sprintf("setting %q: %s", name, problem))
		}
	}
	return problems
}

func (s *Spec) CheckSettings(values map[string]any) []string {
	return CheckSettings(s.Settings, values)
}

func Resolve(settings []Setting, definition map[string]any, overrides map[string]Override, name string) (Effective, error) {
	st, ok := findSetting(settings, name)
	if !ok {
		return Effective{}, fmt.Errorf("setting %q is not declared by this module", name)
	}
	if o, ok := overrides[name]; ok && st.check(o.Value) == "" {
		return Effective{Name: name, Value: o.Value, Layer: LayerOverride, By: o.By, At: o.At}, nil
	}
	if v, ok := definition[name]; ok && st.check(v) == "" {
		return Effective{Name: name, Value: v, Layer: LayerDefinition}, nil
	}
	return Effective{Name: name, Value: st.Default, Layer: LayerModule}, nil
}

func ResolveAll(settings []Setting, definition map[string]any, overrides map[string]Override) []Effective {
	out := make([]Effective, 0, len(settings))
	for _, st := range settings {
		e, _ := Resolve(settings, definition, overrides, st.Name)
		out = append(out, e)
	}
	return out
}
