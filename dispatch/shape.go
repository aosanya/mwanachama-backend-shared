package dispatch

// Handled is every address the spec declares but the module supplies the
// handler for, in declaration order. An orchestration -- several calls, a
// seam the module owns, a security-critical ordering -- cannot be one Call,
// so it is declared and bound rather than left out of the table entirely.
func Handled(s *Spec) []Operation {
	if s == nil {
		return nil
	}
	var out []Operation
	for _, name := range sortedKeys(s.Operations) {
		if op := s.Operations[name]; op.Handled {
			out = append(out, op)
		}
	}
	return out
}

func Shape(s *Spec) []Route {
	if s == nil {
		return nil
	}
	names := sortedKeys(s.Operations)
	out := make([]Route, 0, len(names))
	for _, name := range names {
		op := s.Operations[name]
		out = append(out, Route{Method: op.Method, Path: s.Base + op.Path, Action: op.Action})
	}
	return out
}
