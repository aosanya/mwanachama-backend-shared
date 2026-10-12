package dispatch

import (
	"fmt"
	"regexp"
)

var RefersPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

type Reference struct {
	Operation string
	As        string
	From      Source
	Repeated  bool
	Refers    string
}

func (a Arg) checkRefers(name string, i int) []string {
	if a.Refers == "" {
		return nil
	}
	var problems []string
	add := func(format string, v ...any) { problems = append(problems, fmt.Sprintf(format, v...)) }
	if !RefersPattern.MatchString(a.Refers) {
		add("operation %q: argument %d refers to %q, which is not <module>.<object>", name, i, a.Refers)
	}
	switch {
	case a.Whole:
		add("operation %q: argument %d takes a whole request, which is not one value that could refer to a thing", name, i)
	case a.Ignored:
		add("operation %q: argument %d is ignored, and binds no value that could refer to a thing", name, i)
	case a.From == FromCaller || a.From == FromDevice:
		add("operation %q: argument %d comes from the session, which no request names as a reference", name, i)
	}
	return problems
}

func (s *Spec) References() []Reference {
	var out []Reference
	for _, name := range sortedKeys(s.Operations) {
		for _, a := range s.Operations[name].Args {
			if a.Refers == "" {
				continue
			}
			out = append(out, Reference{
				Operation: name, As: a.As, From: a.From, Repeated: a.Repeated, Refers: a.Refers,
			})
		}
	}
	return out
}
