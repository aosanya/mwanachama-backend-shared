// http.go — this package's own HTTP surface: decode a request, call one
// Repository method, encode the response. Lives in the root package rather
// than a separate routes/ subpackage so a caller needs only this package's
// import, never a second one — the same reason Settings/PublicSettings/
// Property alias their models. counterparts instead of making a caller
// import models directly (see doc.go).
package orgsettings

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Route is one address this package answers, relative to wherever the
// mounting process prefixes it (e.g. "/v1/org-settings") — enough to build
// one *http.ServeMux entry from, without the mounting process
// hand-spelling each path/method pair itself.
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// Pattern returns the http.ServeMux registration pattern for this route
// once mounted under prefix — r.Method+" "+prefix+r.Path, net/http's own
// "METHOD /path" syntax (Go 1.22+ mux patterns).
func (r Route) Pattern(prefix string) string {
	return r.Method + " " + prefix + r.Path
}

// Routes is every address this package answers: GetOrgSettings,
// GetOwnOrgSettings and PutOrgSettings. ownSlug is the organization this
// gateway is — see [GetOwnOrgSettings] — since org-settings has no
// ResourceNames override the way mwanachama-backend-actor's Group does; the
// URL noun ("org-settings") is not org-configurable.
//
// A mounting process builds its mux from it directly:
//
//	for _, rt := range orgsettings.Routes(repo, ownSlug) {
//	    mux.HandleFunc(rt.Pattern(prefix), rt.Handler) // GET routes are public
//	}
//
// PUT needs a different gate than the two GETs (write vs. no auth at all),
// so a mounting process that wants to wrap it separately calls
// [PutOrgSettings] directly instead of ranging over this slice.
func Routes(repo Repository, ownSlug string) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "", Handler: GetOwnOrgSettings(repo, ownSlug)},
		{Method: http.MethodGet, Path: "/{slug}", Handler: GetOrgSettings(repo)},
		{Method: http.MethodPut, Path: "/{slug}", Handler: PutOrgSettings(repo)},
	}
}

// GetOrgSettings handles GET /{slug}.
//
// DEV-1284 · This route carries **no session at all** — a guest home and a
// sign-in screen have to render the organization's mark before anybody has
// authenticated. So what it returns is the guest projection
// (PublicSettings), never the stored row.
func GetOrgSettings(repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := repo.Get(r.Context(), r.PathValue("slug"))
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out.Public())
	}
}

// GetOwnOrgSettings handles GET / for the organization ownSlug names — the
// call a *connecting* handset makes (DEV-1512), which holds the gateway's
// URL and nothing else, since one deployment serves exactly one
// organization. Same guest projection as [GetOrgSettings].
func GetOwnOrgSettings(repo Repository, ownSlug string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := repo.Get(r.Context(), ownSlug)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out.Public())
	}
}

// PutOrgSettings handles PUT /{slug}. No capability gate: whether a caller
// may write an organization's settings at all is the mounting process's own
// policy.
func PutOrgSettings(repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Settings
		if err := readJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		in.Slug = r.PathValue("slug")
		out, err := repo.Put(r.Context(), in)
		if err != nil {
			// ErrInvalidSettings is this domain's own attribute-validation
			// sentinel — kept local rather than routed through the
			// gateway's cross-domain domerr (which this package must not
			// depend on), so it needs its own status mapping here rather
			// than leaving it to the mounting process's generic store-error
			// handling.
			if errors.Is(err, ErrInvalidSettings) {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

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

// readJSON decodes a JSON body into v, refusing unknown fields the same way
// the gateway's readJSON does.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
