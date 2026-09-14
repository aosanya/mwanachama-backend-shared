// Package httpwire is the JSON wire helpers and route-table shape every
// GORM-backed repo's routes/ package rebuilds by hand — the same functions,
// copied near-verbatim across actor/assetmanager/comm/forms/git. Handlers,
// error-to-status tables, and auth stay with the caller: this package only
// speaks bytes and addresses, never decides whether a request is allowed.
package httpwire

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// WriteJSON serialises v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteErr writes {"error": msg}.
func WriteErr(w http.ResponseWriter, code int, msg string) {
	WriteJSON(w, code, map[string]string{"error": msg})
}

// ReadJSON decodes a JSON body into v, refusing unknown fields.
func ReadJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// ReadJSONFields decodes a JSON body into v the way ReadJSON does, and also
// reports which top-level keys the request body actually carried — for a
// caller that must tell an absent field apart from an explicit null.
func ReadJSONFields(r *http.Request, v any) (map[string]bool, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(raw))
	for k := range raw {
		present[k] = true
	}
	return present, nil
}

// Bearer extracts a token from the Authorization header, stripping a
// "Bearer " prefix if present.
func Bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return h
}

// Route is one address a routes/ package answers, relative to wherever the
// mounting process prefixes it.
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// Pattern returns the http.ServeMux registration pattern for this route
// once mounted under prefix.
func (r Route) Pattern(prefix string) string {
	return r.Method + " " + prefix + r.Path
}

// StatusFor looks err up against table via errors.Is, returning the
// matching status or fallback if none match. Callers keep their own
// sentinel-error-to-status tables; this only shares the lookup-and-fallback
// mechanics, not the mapping itself.
func StatusFor(err error, table map[error]int, fallback int) int {
	for sentinel, code := range table {
		if errors.Is(err, sentinel) {
			return code
		}
	}
	return fallback
}
