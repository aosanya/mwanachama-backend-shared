package spec

import (
	"fmt"
	"regexp"
	"strings"
)

var AttachTargetPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

type Attachment struct {
	Module string `json:"module"`
	Items  string `json:"items"`
}

func (a Attachment) ItemsRole() string {
	_, role, _ := strings.Cut(a.Items, ".")
	return role
}

func (s *Spec) Attached(module string) []Attachment {
	var out []Attachment
	for _, a := range s.Attach {
		if a.Module == module {
			out = append(out, a)
		}
	}
	return out
}

func (s *Spec) attachProblems() []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	roles := map[string]bool{}
	for _, o := range s.Objects {
		if o.Role != "" {
			roles[o.Role] = true
		}
	}
	seen := map[Attachment]bool{}
	for i, a := range s.Attach {
		if !SegmentPattern.MatchString(a.Module) {
			add("attachment %d: module %q is not a usable name segment", i, a.Module)
		} else if a.Module == s.Module {
			add("attachment %d: the module %q cannot attach to itself", i, a.Module)
		}
		if !AttachTargetPattern.MatchString(a.Items) {
			add("attachment %d: items %q is not <module>.<role>", i, a.Items)
			continue
		}
		owner, role, _ := strings.Cut(a.Items, ".")
		if owner != s.Module {
			add("attachment %d: items %q names the module %q, and this spec is %q's — an attachment points at this spec's own objects", i, a.Items, owner, s.Module)
		} else if !roles[role] {
			add("attachment %d: items %q names the role %q, which no object in this spec claims", i, a.Items, role)
		}
		if seen[a] {
			add("attachment %d: %q is attached to %q twice", i, a.Module, a.Items)
		}
		seen[a] = true
	}
	return problems
}
