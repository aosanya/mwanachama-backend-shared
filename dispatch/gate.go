package dispatch

import "sort"

type Split struct {
	Anonymous []Route
	Gated     []Route
}

func Anonymous(routes []Route, actions ...string) Split {
	allowed := make(map[string]bool, len(actions))
	for _, a := range actions {
		allowed[a] = true
	}

	var split Split
	for _, r := range routes {
		if r.Action != "" && allowed[r.Action] {
			split.Anonymous = append(split.Anonymous, r)
			continue
		}
		split.Gated = append(split.Gated, r)
	}
	return split
}

func (s Split) Unmatched(routes []Route, actions []string) []string {
	present := make(map[string]bool, len(routes))
	for _, r := range routes {
		present[r.Action] = true
	}
	var out []string
	for _, a := range actions {
		if !present[a] {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}
