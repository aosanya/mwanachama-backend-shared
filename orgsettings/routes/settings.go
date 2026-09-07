// settings.go — HTTP routes over orgsettings.Repository's Get/Put. See
// doc.go for what is and is not in scope and why.
package routes

import (
	"errors"
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
)

// GetOrgSettings handles GET /{slug}.
//
// DEV-1284 · This route carries **no session at all** — a guest home and a
// sign-in screen have to render the organization's mark before anybody has
// authenticated. So what it returns is the guest projection
// (orgsettings.PublicSettings), never the stored row.
func GetOrgSettings(repo orgsettings.Repository) http.HandlerFunc {
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
func GetOwnOrgSettings(repo orgsettings.Repository, ownSlug string) http.HandlerFunc {
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
// policy — see doc.go for why that gate is deliberately not this package's
// to add.
func PutOrgSettings(repo orgsettings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in orgsettings.Settings
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
			if errors.Is(err, orgsettings.ErrInvalidSettings) {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
