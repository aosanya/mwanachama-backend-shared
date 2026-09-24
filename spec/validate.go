package spec

// Validation is what makes a spec refusable at the moment refusing it is
// cheap. Every problem is reported, not just the first, because a spec is
// edited by hand and a list of what is wrong beats one round trip per
// mistake.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Validate reports every way the spec is unusable, rather than the first.
func (s *Spec) Validate() error {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	// Both are segments of every physical name this spec emits, so both are
	// held to the stricter alphabet rather than to NamePattern.
	if !SegmentPattern.MatchString(s.Instance) {
		add("instance %q is not a usable name segment", s.Instance)
	}
	if !SegmentPattern.MatchString(s.Module) {
		add("module %q is not a usable name segment", s.Module)
	}
	if len(s.Objects) == 0 {
		add("a spec declares at least one object")
	}

	seenObject := map[string]bool{}
	seenTable := map[string]bool{}
	seenRole := map[string]string{}
	// Keyed by the name the database will actually hold, so that two names
	// differing only past MaxIdentifier are caught here rather than merging
	// into one relation later. Indexes share the table namespace, so both
	// kinds go in one map.
	seenPhysical := map[string]string{}
	claim := func(kind, what, physical string) {
		if len(physical) > MaxIdentifier {
			add("%s %q needs %d bytes as %q, and the database truncates at %d",
				kind, what, len(physical), physical, MaxIdentifier)
			physical = physical[:MaxIdentifier]
		}
		if first, taken := seenPhysical[physical]; taken {
			add("%s %q and %s both become %q in the database", kind, what, first, physical)
			return
		}
		seenPhysical[physical] = kind + " " + strconv.Quote(what)
	}
	for _, o := range s.Objects {
		if !NamePattern.MatchString(o.Name) {
			add("object %q is not a usable name", o.Name)
			continue
		}
		if seenObject[o.Name] {
			add("object %q is declared twice", o.Name)
		}
		seenObject[o.Name] = true

		if strings.TrimSpace(o.Description) == "" {
			add("object %q has no description", o.Name)
		}

		if o.Role != "" {
			if !NamePattern.MatchString(o.Role) {
				add("object %q: role %q is not a usable name", o.Name, o.Role)
			}
			// Two objects claiming one role would leave a rule with no way
			// to know which of them it operates on.
			if first, taken := seenRole[o.Role]; taken {
				add("objects %q and %q both claim the role %q", first, o.Name, o.Role)
			} else {
				seenRole[o.Role] = o.Name
			}
		}

		table := o.table()
		if !NamePattern.MatchString(table) {
			add("object %q: table %q is not a usable name", o.Name, table)
		} else {
			if seenTable[table] {
				add("table %q is declared by more than one object", table)
			}
			seenTable[table] = true
			claim("table", table, s.TableFor(o))
			for _, idx := range o.Indexes {
				if NamePattern.MatchString(idx.Name) {
					claim("index", table+"."+idx.Name, s.IndexFor(o, idx))
				}
			}
		}

		problems = append(problems, o.validate()...)
	}

	return asError("spec", problems)
}

func asError(subject string, problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s: %s", subject, strings.Join(problems, "; "))
}

func (o Object) validate() []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if len(o.Fields) == 0 {
		if o.Role != "" {
			add("object %q fills the role %q and declares no fields of its own, so it has to be read through the module's blueprint rather than loaded on its own",
				o.Name, o.Role)
		} else {
			add("object %q declares no fields", o.Name)
		}
		return problems
	}

	seen := map[string]bool{}
	primaries := 0
	for _, f := range o.Fields {
		if !NamePattern.MatchString(f.Name) {
			add("object %q: field %q is not a usable name", o.Name, f.Name)
			continue
		}
		if seen[f.Name] {
			add("object %q: field %q is declared twice", o.Name, f.Name)
		}
		seen[f.Name] = true

		if strings.TrimSpace(f.Description) == "" {
			add("object %q: field %q has no description", o.Name, f.Name)
		}

		if !fieldTypes[f.Type] {
			add("object %q: field %q has unknown type %q", o.Name, f.Name, f.Type)
		}
		if f.Type == TypeEnum && len(f.Values) == 0 {
			add("object %q: enum field %q declares no values", o.Name, f.Name)
		}
		if f.Type != TypeEnum && len(f.Values) > 0 {
			add("object %q: field %q declares values but is not an enum", o.Name, f.Name)
		}
		if f.Type == TypeEnum && f.Default != "" && !holds(f.Values, f.Default) {
			add("object %q: field %q defaults to %q, which is not one of %s",
				o.Name, f.Name, f.Default, strings.Join(f.Values, ", "))
		}
		if f.Default != "" && (f.Required || f.Primary) {
			add("object %q: field %q is required and carries the default %q, and a default is exactly what would let an omitted value pass unnoticed",
				o.Name, f.Name, f.Default)
		}
		if f.Primary {
			primaries++
		}
		if f.Primary && f.Type == TypeJSON {
			add("object %q: field %q cannot be both a document and a key", o.Name, f.Name)
		}
	}
	if primaries == 0 {
		add("object %q declares no primary key", o.Name)
	}

	seenIndex := map[string]bool{}
	for _, idx := range o.Indexes {
		if !NamePattern.MatchString(idx.Name) {
			add("object %q: index %q is not a usable name", o.Name, idx.Name)
			continue
		}
		if seenIndex[idx.Name] {
			add("object %q: index %q is declared twice", o.Name, idx.Name)
		}
		seenIndex[idx.Name] = true

		switch {
		case idx.Path != nil && len(idx.Fields) > 0:
			add("object %q: index %q names both fields and a path", o.Name, idx.Name)
		case idx.Path != nil:
			if !seen[idx.Path.Field] {
				add("object %q: index %q indexes unknown field %q", o.Name, idx.Name, idx.Path.Field)
			}
			if !DocPathPattern.MatchString(idx.Path.Path) {
				add("object %q: index %q has an unusable path %q", o.Name, idx.Name, idx.Path.Path)
			}
		case len(idx.Fields) > 0:
			for _, f := range idx.Fields {
				if !seen[f] {
					add("object %q: index %q indexes unknown field %q", o.Name, idx.Name, f)
				}
			}
		default:
			add("object %q: index %q indexes nothing", o.Name, idx.Name)
		}

		if idx.NotDeleted && !seen["deleted"] {
			add("object %q: index %q is scoped to undeleted rows but there is no deleted field", o.Name, idx.Name)
		}
	}
	return problems
}

func holds(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}
