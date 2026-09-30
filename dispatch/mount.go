package dispatch

import (
	"fmt"
	"sort"
	"sync"
)

type Mount struct {
	Authorize Authorizer
	Caller    Caller
}

type Table struct {
	load      func() (*Spec, error)
	once      sync.Once
	spec      *Spec
	err       error
	sentinels map[string]error
	anonymous []string
}

func NewTable(operations []byte, sentinels map[string]error, anonymous ...string) *Table {
	return &Table{
		load:      func() (*Spec, error) { return Parse(operations) },
		sentinels: sentinels,
		anonymous: anonymous,
	}
}

func (t *Table) Spec() (*Spec, error) {
	t.once.Do(func() { t.spec, t.err = t.load() })
	return t.spec, t.err
}

func (t *Table) AnonymousActions() []string {
	return append([]string(nil), t.anonymous...)
}

func (t *Table) Build(manager any, m Mount) ([]Route, error) {
	s, err := t.Spec()
	if err != nil {
		return nil, err
	}
	return Dispatch(s, Deps{
		Manager: manager, Errors: t.sentinels, Authorize: m.Authorize, Caller: m.Caller,
	})
}

func (t *Table) Routes(manager any, m Mount) []Route {
	out, err := t.Build(manager, m)
	if err != nil {
		panic(fmt.Sprintf("dispatch: %v", err))
	}
	return out
}

func (t *Table) Split(manager any, m Mount) Split {
	open := Anonymous(t.Routes(manager, Mount{Caller: m.Caller}), t.anonymous...)
	gated := Anonymous(t.Routes(manager, m), t.anonymous...)
	return Split{Anonymous: open.Anonymous, Gated: gated.Gated}
}

func (t *Table) Shape() ([]Route, error) {
	s, err := t.Spec()
	if err != nil {
		return nil, err
	}
	return Shape(s), nil
}

func (t *Table) UnknownAnonymousActions() ([]string, error) {
	shape, err := t.Shape()
	if err != nil {
		return nil, err
	}
	return Split{}.Unmatched(shape, t.anonymous), nil
}

func (t *Table) UnmappedSentinels() ([]string, error) {
	s, err := t.Spec()
	if err != nil {
		return nil, err
	}
	var out []string
	for name := range t.sentinels {
		if _, ok := s.Errors[name]; !ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}
