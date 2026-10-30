package dispatch

import (
	"errors"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"
)

type typeMatch struct {
	find func(error) (map[string]any, bool)
}

func (m *typeMatch) Error() string { return "dispatch: matches an error by its type" }

func ByType[E error](prototype E, detail func(E) map[string]any) error {
	return &typeMatch{find: func(err error) (map[string]any, bool) {
		var found E
		if !errors.As(err, &found) {
			return nil, false
		}
		if detail == nil {
			return nil, true
		}
		return detail(found), true
	}}
}

type outcome struct {
	code   int
	detail map[string]any
}

func resolve(err error, table map[error]int, fallback int) outcome {
	for key, code := range table {
		matcher, typed := key.(*typeMatch)
		if !typed {
			continue
		}
		if detail, ok := matcher.find(err); ok {
			return outcome{code: code, detail: detail}
		}
	}
	return outcome{code: httpwire.StatusFor(err, table, fallback)}
}

func (o outcome) body(message string) map[string]any {
	out := make(map[string]any, len(o.detail)+1)
	for key, value := range o.detail {
		out[key] = value
	}
	out["error"] = message
	return out
}
