// policy.go — HTTP routes over orgpolicy.Repository's Get/Set. See doc.go
// for what is and is not in scope and why.
package routes

import (
	"errors"
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy"
)

// GetOrgPolicy handles GET /.
func GetOrgPolicy(repo orgpolicy.Repository) http.HandlerFunc {
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
func PutOrgPolicy(repo orgpolicy.Repository, callerID func(*http.Request) string) http.HandlerFunc {
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
			if errors.Is(err, orgpolicy.ErrBadCap) {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
