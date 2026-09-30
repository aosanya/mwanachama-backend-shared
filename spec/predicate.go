package spec

import "fmt"

type PredicateOp string

const (
	OpNotEmpty PredicateOp = "not_empty"
	OpIsTrue   PredicateOp = "is_true"
	OpIsFalse  PredicateOp = "is_false"
)

type Predicate struct {
	Field string      `json:"field"`
	Op    PredicateOp `json:"op"`
}

var predicateOps = map[PredicateOp]bool{OpNotEmpty: true, OpIsTrue: true, OpIsFalse: true}

func (p Predicate) sql() string {
	switch p.Op {
	case OpNotEmpty:
		return p.Field + " <> ''"
	case OpIsTrue:
		return p.Field + " = true"
	case OpIsFalse:
		return p.Field + " = false"
	}
	return ""
}

func (p Predicate) fitsType(t FieldType) bool {
	switch p.Op {
	case OpNotEmpty:
		return t == TypeString || t == TypeText || t == TypeEnum
	case OpIsTrue, OpIsFalse:
		return t == TypeBool
	}
	return false
}

func (idx Index) conditions() []string {
	var out []string
	if idx.NotDeleted {
		out = append(out, "deleted = false")
	}
	for _, p := range idx.Where {
		out = append(out, p.sql())
	}
	return out
}

func (idx Index) validateWhere(o Object, fields map[string]Field) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range idx.Where {
		if !predicateOps[p.Op] {
			out = append(out, fmt.Sprintf("object %q: index %q has an unknown condition %q", o.Name, idx.Name, p.Op))
			continue
		}
		f, ok := fields[p.Field]
		if !ok {
			out = append(out, fmt.Sprintf("object %q: index %q is conditioned on unknown field %q", o.Name, idx.Name, p.Field))
			continue
		}
		if !p.fitsType(f.Type) {
			out = append(out, fmt.Sprintf("object %q: index %q applies %q to %q, which is %s", o.Name, idx.Name, p.Op, p.Field, f.Type))
			continue
		}
		key := p.Field + " " + string(p.Op)
		if seen[key] {
			out = append(out, fmt.Sprintf("object %q: index %q states %q on %q twice", o.Name, idx.Name, p.Op, p.Field))
		}
		seen[key] = true
	}
	return out
}
