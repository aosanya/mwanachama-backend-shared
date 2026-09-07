package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// writeJSON and writeErr mirror the gateway's own internal/api/http/wire.go
// byte-for-byte on purpose: a caller of a route built from this package
// must not be able to tell, from the response shape, that it moved here.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes {"error": msg}.
func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// readJSONFields decodes a JSON body into v (refusing unknown fields) and
// returns which top-level keys the body actually named — mirrors the
// gateway's own readJSONFields byte-for-byte, needed here because
// PutOrgPolicy's "absent means unchanged, present-but-null is still
// refused" rule can't be expressed by decoding alone.
func readJSONFields(r *http.Request, v any) (map[string]bool, error) {
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
