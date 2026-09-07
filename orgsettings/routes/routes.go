package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
)

// Route is one address this package answers, relative to wherever the
// mounting process prefixes it (e.g. "/v1/org-settings") — enough to build
// one *http.ServeMux entry from, without the mounting process
// hand-spelling each path/method pair itself. Mirrors
// mwanachama-backend-actor's routes.Route exactly.
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
//	for _, rt := range routes.Routes(repo, ownSlug) {
//	    mux.HandleFunc(rt.Pattern(prefix), rt.Handler) // GET routes are public
//	}
//
// PUT needs a different gate than the two GETs (write vs. no auth at all),
// so a mounting process that wants to wrap it separately calls
// [PutOrgSettings] directly instead of ranging over this slice — see doc.go.
func Routes(repo orgsettings.Repository, ownSlug string) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "", Handler: GetOwnOrgSettings(repo, ownSlug)},
		{Method: http.MethodGet, Path: "/{slug}", Handler: GetOrgSettings(repo)},
		{Method: http.MethodPut, Path: "/{slug}", Handler: PutOrgSettings(repo)},
	}
}
