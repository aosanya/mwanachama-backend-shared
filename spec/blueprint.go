package spec

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
)

type Blueprint struct {
	Module  string   `json:"module"`
	Objects []Object `json:"objects"`
}

func ParseBlueprint(raw []byte) (*Blueprint, error) {
	var b Blueprint
	if err := decode(raw, &b); err != nil {
		return nil, fmt.Errorf("blueprint: %w", err)
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &b, nil
}

func LoadBlueprint(path string) (*Blueprint, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("blueprint: read %s: %w", path, err)
	}
	return ParseBlueprint(raw)
}

func (b *Blueprint) Validate() error {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if !SegmentPattern.MatchString(b.Module) {
		add("module %q is not a usable name segment", b.Module)
	}
	if len(b.Objects) == 0 {
		add("a blueprint declares at least one object")
	}

	seen := map[string]bool{}
	for _, o := range b.Objects {
		if o.Role == "" {
			add("an object declares no role, and a role is the only name a blueprint object has")
			continue
		}
		if !NamePattern.MatchString(o.Role) {
			add("role %q is not a usable name", o.Role)
			continue
		}
		if seen[o.Role] {
			add("role %q is declared twice", o.Role)
		}
		seen[o.Role] = true

		if o.Name != "" {
			add("role %q declares the name %q, which is the domain's to choose", o.Role, o.Name)
		}
		if o.Table != "" {
			add("role %q declares the table %q, which is the domain's to choose", o.Role, o.Table)
		}
		if strings.TrimSpace(o.Description) == "" {
			add("role %q has no description", o.Role)
		}

		named := o
		named.Name = o.Role
		problems = append(problems, named.validate()...)
	}

	return asError("blueprint", problems)
}

func (b *Blueprint) Object(role string) (Object, bool) {
	for _, o := range b.Objects {
		if o.Role == role {
			return o, true
		}
	}
	return Object{}, false
}

func (b *Blueprint) Roles() []string {
	out := make([]string, 0, len(b.Objects))
	for _, o := range b.Objects {
		out = append(out, o.Role)
	}
	sort.Strings(out)
	return out
}

func (b *Blueprint) Load(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec: read %s: %w", path, err)
	}
	return b.Parse(raw)
}

func (b *Blueprint) Parse(raw []byte) (*Spec, error) {
	var s Spec
	if err := decode(raw, &s); err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	if err := b.Apply(&s); err != nil {
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func (b *Blueprint) Apply(s *Spec) error {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if s.Module != b.Module {
		add("domain %q is declared against the module %q, and this is the blueprint for %q",
			s.Domain, s.Module, b.Module)
		return asError("spec", problems)
	}

	for i, o := range s.Objects {
		if o.Role == "" {
			continue
		}
		declared, ok := b.Object(o.Role)
		if !ok {
			add("object %q claims the role %q, which the module does not declare — it fills one of %s, or it drops the role and declares its own fields",
				o.Name, o.Role, strings.Join(b.Roles(), ", "))
			continue
		}
		merged, errs := extend(declared, o)
		problems = append(problems, errs...)
		s.Objects[i] = merged
	}

	return asError("spec", problems)
}

func extend(declared, domain Object) (Object, []string) {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	out := domain
	out.Fields = make([]Field, len(declared.Fields))
	copy(out.Fields, declared.Fields)

	overridden := map[string]bool{}
	for _, f := range domain.Fields {
		at := -1
		for i, d := range out.Fields {
			if d.Name == f.Name {
				at = i
				break
			}
		}
		if at < 0 {
			add("object %q: the module declares no field %q for the role %q, and a domain adds fields by declaring an object of its own",
				domain.Name, f.Name, domain.Role)
			continue
		}
		if overridden[f.Name] {
			add("object %q: field %q is overridden twice", domain.Name, f.Name)
			continue
		}
		overridden[f.Name] = true

		if !onlyDefault(f) {
			add("object %q: field %q may set a default and nothing else — its type, its rules and its description are the module's declaration",
				domain.Name, f.Name)
			continue
		}
		out.Fields[at].Default = f.Default
	}

	out.Indexes = append(append([]Index{}, declared.Indexes...), domain.Indexes...)
	return out, problems
}

func onlyDefault(f Field) bool {
	bare := Field{Name: f.Name, Default: f.Default}
	return reflect.DeepEqual(f, bare)
}
