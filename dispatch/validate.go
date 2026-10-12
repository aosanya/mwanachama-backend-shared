package dispatch

import "fmt"

func (op Operation) validate(name string) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	declared := pathParams(op.Path)
	bound := map[string]bool{}
	wholes := 0
	overwrites := 0
	for i, a := range op.Args {
		if !sources[a.From] {
			add("operation %q: argument %d comes from %q, which is not path, query, body, caller or device", name, i, a.From)
		}
		problems = append(problems, a.checkRefers(name, i)...)
		if a.From == FromCaller || a.From == FromDevice {
			switch {
			case a.Whole || a.Repeated || a.Ignored:
				add("operation %q: argument %d comes from the session and cannot be anything else", name, i)
			case a.As == "":
				add("operation %q: argument %d comes from the session and names nothing", name, i)
			case a.Required:
				add("operation %q: argument %q comes from the caller, which no request can be required to supply", name, a.As)
			}
			if a.Into != "" {
				overwrites++
			}
			continue
		}
		if a.Ignored {
			switch {
			case a.From != FromPath:
				add("operation %q: argument %d is ignored, which only an address segment may be", name, i)
			case a.Whole || a.Repeated || a.Into != "":
				add("operation %q: argument %d is ignored and cannot be anything else", name, i)
			case a.As == "":
				add("operation %q: argument %d is ignored and names nothing", name, i)
			case !declared[a.As]:
				add("operation %q: argument %q is ignored, and the path declares no {%s}", name, a.As, a.As)
			default:
				bound[a.As] = true
			}
			continue
		}
		if a.Into != "" {
			if a.From != FromPath {
				add("operation %q: argument %d overwrites %s from the %s, but only the address or the caller may outrank the body",
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
		if !bound[p] && !op.Handled {
			add("operation %q: path declares {%s}, which no argument binds", name, p)
		}
	}
	if wholes > 1 {
		add("operation %q takes %d whole requests, and there is only one", name, wholes)
	}
	if overwrites > 0 && wholes != 1 {
		add("operation %q overwrites a field, but takes no whole request to overwrite it on", name)
	}

	if len(op.Returns) == 0 && !op.Handled {
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
