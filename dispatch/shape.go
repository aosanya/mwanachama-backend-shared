package dispatch

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
