// Package vocab is the closed-membership-set-plus-transition-table idiom
// independently built by taskmanager (TaskStatus/WorkflowRunStatus),
// accounting (AccountType), and digitaltwin (AssetType/StationType/
// AssetStatus), generalized over any comparable type. Membership and
// transitions are independent: a type with no lifecycle uses only Set, a
// type with no closed vocabulary but real transitions uses only
// Transitions.
package vocab

// Set is a closed membership set over a comparable type T, typically a
// defined string type standing in for an enum.
type Set[T comparable] map[T]bool

// NewSet builds a Set containing exactly members.
func NewSet[T comparable](members ...T) Set[T] {
	s := make(Set[T], len(members))
	for _, m := range members {
		s[m] = true
	}
	return s
}

// Contains reports whether v is a member of s.
func (s Set[T]) Contains(v T) bool {
	return s[v]
}

// Transitions maps each state of a lifecycle type T to the set of states
// it may move to next. A state absent from the table (a terminal state, or
// one that was never given outgoing transitions) permits none.
type Transitions[T comparable] map[T]Set[T]

// NewTransitions builds a Transitions table from a state -> allowed-next-
// states map.
func NewTransitions[T comparable](table map[T][]T) Transitions[T] {
	t := make(Transitions[T], len(table))
	for from, tos := range table {
		t[from] = NewSet(tos...)
	}
	return t
}

// CanTransitionTo reports whether moving from "from" to "next" is valid
// per t.
func (t Transitions[T]) CanTransitionTo(from, next T) bool {
	return t[from].Contains(next)
}
