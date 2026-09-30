package dispatch

import (
	"fmt"
	"sync"
)

type Mount struct {
	Authorize Authorizer
	Caller    Caller
	Fields    map[string]FieldDoc
	Fallback  int
}

type Table struct {
	name       string
	operations func() (*Spec, error)
	sentinels  map[string]error
	anonymous  []string
}

func NewTable(name string, operations []byte, sentinels map[string]error, anonymous ...string) *Table {
	return &Table{
		name:       name,
		operations: sync.OnceValues(func() (*Spec, error) { return Parse(operations) }),
		sentinels:  sentinels,
		anonymous:  anonymous,
	}
}

func (t *Table) Spec() (*Spec, error) { return t.operations() }

func (t *Table) AnonymousActions() []string {
	return append([]string(nil), t.anonymous...)
}

func (t *Table) deps(manager any, m Mount) Deps {
	return Deps{
		Manager:   manager,
		Errors:    t.sentinels,
		Fields:    m.Fields,
		Fallback:  m.Fallback,
		Authorize: m.Authorize,
		Caller:    m.Caller,
	}
}

func (t *Table) Build(manager any, m Mount) ([]Route, error) {
	s, err := t.operations()
	if err != nil {
		return nil, err
	}
	return Dispatch(s, t.deps(manager, m))
}

func (t *Table) Routes(manager any, m Mount) []Route {
	out, err := t.Build(manager, m)
	if err != nil {
		panic(fmt.Sprintf("%s routes: %v", t.name, err))
	}
	return out
}

func (t *Table) Split(manager any, m Mount) Split {
	public := Anonymous(t.Routes(manager, Mount{Fields: m.Fields, Fallback: m.Fallback}), t.anonymous...)
	gated := Anonymous(t.Routes(manager, m), t.anonymous...)
	return Split{Anonymous: public.Anonymous, Gated: gated.Gated}
}

func (t *Table) PublicRoutes(manager any, m Mount) []Route {
	return t.Split(manager, m).Anonymous
}

func (t *Table) GatedRoutes(manager any, m Mount) []Route {
	return t.Split(manager, m).Gated
}

func (t *Table) BuildTools(manager any, m Mount) ([]Tool, error) {
	s, err := t.operations()
	if err != nil {
		return nil, err
	}
	return Tools(s, t.deps(manager, m))
}
