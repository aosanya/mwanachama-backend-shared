package dispatch

import (
	"fmt"
	"net/http"
	"sort"
)

type alternative struct {
	name    string
	op      Operation
	handler http.HandlerFunc
}

func addressOf(op Operation) string { return op.Method + " " + op.Path }

func (s *Spec) checkAlternatives() []string {
	var problems []string
	selectors := map[string]map[string]string{}
	defaults := map[string]Operation{}
	for _, name := range sortedKeys(s.Operations) {
		op := s.Operations[name]
		address := addressOf(op)
		if op.WhenQuery == "" {
			defaults[address] = op
			continue
		}
		if selectors[address] == nil {
			selectors[address] = map[string]string{}
		}
		if first, taken := selectors[address][op.WhenQuery]; taken {
			problems = append(problems, fmt.Sprintf(
				"operations %q and %q both answer %q when ?%s is present", first, name, address, op.WhenQuery))
		} else {
			selectors[address][op.WhenQuery] = name
		}
	}
	for address, byKey := range selectors {
		def, ok := defaults[address]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%q is answered only when a query parameter is present (%s); one operation there declares no whenQuery to answer the rest",
				address, joinedKeys(byKey)))
			continue
		}
		if def.Handled {
			problems = append(problems, fmt.Sprintf(
				"%q is handled by the module, so its whenQuery alternatives (%s) can never be reached", address, joinedKeys(byKey)))
		}
	}
	return problems
}

func joinedKeys(m map[string]string) string {
	keys := sortedKeys(m)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += "?" + k
	}
	return out
}

func selecting(def alternative, others []alternative) http.HandlerFunc {
	sorted := append([]alternative(nil), others...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].op.WhenQuery < sorted[j].op.WhenQuery })
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		for _, alt := range sorted {
			if query.Get(alt.op.WhenQuery) != "" {
				alt.handler(w, r)
				return
			}
		}
		def.handler(w, r)
	}
}

func mergeAlternatives(routes []Route, ops []Operation) []Route {
	groups := map[string][]alternative{}
	for i, op := range routes {
		address := addressOf(ops[i])
		groups[address] = append(groups[address], alternative{op: ops[i], handler: op.Handler})
	}
	var out []Route
	seen := map[string]bool{}
	for i, route := range routes {
		address := addressOf(ops[i])
		group := groups[address]
		if len(group) == 1 {
			out = append(out, route)
			continue
		}
		if seen[address] {
			continue
		}
		seen[address] = true
		var def alternative
		var others []alternative
		for _, alt := range group {
			if alt.op.WhenQuery == "" {
				def = alt
			} else {
				others = append(others, alt)
			}
		}
		out = append(out, Route{
			Method:  def.op.Method,
			Path:    route.Path,
			Action:  def.op.Action,
			Handler: selecting(def, others),
		})
	}
	return out
}
