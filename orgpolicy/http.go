// http.go — this package's own HTTP surface: decode a request, call one
// Repository method, encode the response. Lives in the root package rather
// than a separate routes/ subpackage so a caller needs only this package's
// import, never a second one — the same reason Policy aliases models.Policy
// instead of making a caller import models directly (see doc.go).
package orgpolicy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Route is one address this package answers, relative to wherever the
// mounting process prefixes it (e.g. "/v1/org-policy").
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
// gated) — org-policy is never public.
func Routes(repo Repository, callerID func(*http.Request) string) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "", Handler: GetOrgPolicy(repo)},
		{Method: http.MethodPut, Path: "", Handler: PutOrgPolicy(repo, callerID)},
	}
}

// GetOrgPolicy handles GET /.
func GetOrgPolicy(repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := repo.Get(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

// PutOrgPolicy handles PUT /.
//
// Sets the organization's settable numbers: the cap on how many of a
// member's addresses may be stored in the clear, the cap on how many
// chapters one member may be registered at, and the ceiling on a free-text
// answer's length.
//
// **Absent means unchanged, and present-but-null is still refused.** With
// one row for the whole organization an absent field could only be a
// mistake, and zero is a real setting for the address cap — a forgotten
// field must never be able to mean it. A field the request did not carry
// keeps its stored value, so a console panel saving one number cannot blank
// the other; a field carrying an explicit `null` is still refused by name;
// and a request carrying none of the three is refused rather than recorded
// as an edit that changed nothing but updated_by.
func PutOrgPolicy(repo Repository, callerID func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			PublicAddressCap     *int `json:"public_address_cap"`
			ChapterMembershipCap *int `json:"chapter_membership_cap"`
			FreeTextMaxLengthCap *int `json:"free_text_max_length_cap"`
		}
		raw, err := readJSONFields(r, &in)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if !raw["public_address_cap"] && !raw["chapter_membership_cap"] &&
			!raw["free_text_max_length_cap"] {
			writeErr(w, http.StatusBadRequest,
				"name a setting to change: public_address_cap, chapter_membership_cap, "+
					"free_text_max_length_cap, or any combination")
			return
		}
		if raw["public_address_cap"] && in.PublicAddressCap == nil {
			writeErr(w, http.StatusBadRequest, "public_address_cap cannot be null")
			return
		}
		if raw["chapter_membership_cap"] && in.ChapterMembershipCap == nil {
			writeErr(w, http.StatusBadRequest, "chapter_membership_cap cannot be null")
			return
		}
		if raw["free_text_max_length_cap"] && in.FreeTextMaxLengthCap == nil {
			writeErr(w, http.StatusBadRequest, "free_text_max_length_cap cannot be null")
			return
		}

		p, err := repo.Get(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if in.PublicAddressCap != nil {
			p.PublicAddressCap = *in.PublicAddressCap
		}
		if in.ChapterMembershipCap != nil {
			p.ChapterMembershipCap = *in.ChapterMembershipCap
		}
		if in.FreeTextMaxLengthCap != nil {
			p.FreeTextMaxLengthCap = *in.FreeTextMaxLengthCap
		}
		p.UpdatedBy = callerID(r)

		out, err := repo.Set(r.Context(), p)
		if err != nil {
			if errors.Is(err, ErrBadCap) {
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
