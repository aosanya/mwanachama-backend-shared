package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy"
)

// Route is one address this package answers, relative to wherever the
// mounting process prefixes it (e.g. "/v1/org-policy") — mirrors
// mwanachama-backend-actor's routes.Route and this repo's own
// orgsettings/routes.Route exactly.
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

// Routes is every address this package answers: GetOrgPolicy and
// PutOrgPolicy. callerID resolves the caller's own identity for
// PutOrgPolicy's updated_by — see PutOrgPolicy.
//
// Both need the SAME capability gate wrapped around them by the mounting
// process (unlike orgsettings, whose two GETs are public and PUT alone is
// gated) — see doc.go for why org-policy is never public.
func Routes(repo orgpolicy.Repository, callerID func(*http.Request) string) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "", Handler: GetOrgPolicy(repo)},
		{Method: http.MethodPut, Path: "", Handler: PutOrgPolicy(repo, callerID)},
	}
}
