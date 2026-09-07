package orgsettings

import (
	"context"
	"errors"
)

// DefaultRegion adapts this domain's record to the one fact the blind index
// needs from it: the organization's default dialling region (DEV-1227).
//
// It satisfies mwanachama-backend-auth's phonesalt.RegionSource
// **structurally** — neither package imports the other, and that is the
// point. The indexer is entitled to a two-letter region code and to nothing
// else on this record; a direct dependency would hand it the support email
// and the logo URL as well, and would tie the phone-salt domain to a
// settings lookup it has no business knowing about.
type DefaultRegion struct {
	repo Repository
	slug string
}

// NewDefaultRegion binds the adapter to one organization's settings record.
//
// The slug is fixed at construction rather than passed per call because the
// gateway's phone-salt domain is already single-organization — Repository.Hash
// takes no organization argument, since the live salt *is* the organization's.
// A region that could vary per call while the salt could not would be two
// different answers to "whose number is this", and the mismatch would show up
// as unmatchable digests rather than as an error.
func NewDefaultRegion(repo Repository, slug string) *DefaultRegion {
	return &DefaultRegion{repo: repo, slug: slug}
}

// DefaultDiallingRegion returns the configured region, or the empty string
// when nobody has set one.
//
// A missing settings record reads as unset rather than as an error: an
// organization that has not been branded yet has certainly not chosen a
// dialling region either, and both states mean the same thing to the caller —
// there is no region, so nothing may be canonicalized. The refusal is the
// indexer's (phonesalt.ErrNoDiallingRegion), where it can say what the caller
// was trying to do.
func (d *DefaultRegion) DefaultDiallingRegion(ctx context.Context) (string, error) {
	s, err := d.repo.Get(ctx, d.slug)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.DefaultDiallingRegion, nil
}
