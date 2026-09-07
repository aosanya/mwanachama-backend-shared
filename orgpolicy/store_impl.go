// store_impl.go — org-policy Get/Set/PublicAddressCapFor/
// SetPublicAddressCapFor implementation for [Store]. Ported from
// mwanachama-backend-api-gateway's internal/store/postgres's original
// database/sql-backed orgpolicy_store.go, moved onto GORM here to match
// this repo's own gormstore convention (DEV-1683 follow-up) — with the
// individual override's storage moved off the gateway's own (now-retired)
// member table and onto its own org_policy_overrides table (see
// gormstore/override.go).
package orgpolicy

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy/gormstore"
	"github.com/aosanya/mwanachama-backend-shared/orgpolicy/models"
)

// Store is the concrete implementation of [Repository].
type Store struct {
	db     *gorm.DB
	tables TableNames
	exists MemberExists
}

// NewStore constructs a [Store] backed by db, reading and writing the
// tables named by t (see [DefaultTableNames]). exists is consulted by
// SetPublicAddressCapFor to refuse naming a member who does not exist — nil
// means every memberID is accepted (matching the gateway's own
// NewOrgPolicyStore, whose MemberExistence is likewise optional). Callers
// must run [Migrate] against the same db and t before use. Returns an error
// if db is nil.
func NewStore(db *gorm.DB, t TableNames, exists MemberExists) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("NewStore: db must not be nil")
	}
	return &Store{db: db, tables: t, exists: exists}, nil
}

// Get returns the organization's policy. A missing row is the compiled
// default, not an error — the only way to be here without one is a plane
// whose migrations have not finished, and answering an error would take
// down every address publish on a plane whose addresses are otherwise fine.
func (s *Store) Get(ctx context.Context) (Policy, error) {
	var row gormstore.PolicyRow
	err := s.db.WithContext(ctx).Table(s.tables.Policy).Where("singleton").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Policy{
			PublicAddressCap:     models.DefaultPublicAddressCap,
			ChapterMembershipCap: models.DefaultChapterMembershipCap,
			FreeTextMaxLengthCap: models.DefaultFreeTextMaxLengthCap,
		}, nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("Get: %w", err)
	}
	return gormstore.PolicyFromRow(row), nil
}

// Set replaces the organization's policy and records who did it. Validates
// every field against models.ValidateCap/ValidateMembershipCap/
// ValidateFreeTextMaxLengthCap first, wrapped in ErrBadCap — a caller that
// built a Policy and forgot a field would otherwise write a zero the
// database's own CHECK constraints refuse anyway, but refusing here in Go
// gives the caller ErrBadCap's sentence instead of a raw driver error.
//
// Whole-row, and the partial update is the caller's job (see the gateway's
// setOrgPolicy, which reads the stored policy and overlays only the fields
// a request actually carried before calling this). An upsert rather than an
// update, so a plane that somehow has no row gets one the first time an
// operator sets a number.
func (s *Store) Set(ctx context.Context, p Policy) (Policy, error) {
	if err := models.ValidateCap(p.PublicAddressCap); err != nil {
		return Policy{}, fmt.Errorf("%w: %v", ErrBadCap, err)
	}
	if err := models.ValidateMembershipCap(p.ChapterMembershipCap); err != nil {
		return Policy{}, fmt.Errorf("%w: %v", ErrBadCap, err)
	}
	if err := models.ValidateFreeTextMaxLengthCap(p.FreeTextMaxLengthCap); err != nil {
		return Policy{}, fmt.Errorf("%w: %v", ErrBadCap, err)
	}
	row := gormstore.PolicyToRow(p)
	err := s.db.WithContext(ctx).Table(s.tables.Policy).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "singleton"}},
			UpdateAll: true,
		}).
		Create(&row).Error
	if err != nil {
		return Policy{}, fmt.Errorf("Set: %w", err)
	}
	return gormstore.PolicyFromRow(row), nil
}

// PublicAddressCapFor returns one member's individual raise, or nil where
// they have never been named.
func (s *Store) PublicAddressCapFor(ctx context.Context, memberID string) (*int, error) {
	var row gormstore.OverrideRow
	err := s.db.WithContext(ctx).Table(s.tables.Overrides).Where("member_id = ?", memberID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("PublicAddressCapFor: %w", err)
	}
	return &row.PublicAddressCap, nil
}

// SetPublicAddressCapFor grants or clears one member's individual raise.
func (s *Store) SetPublicAddressCapFor(ctx context.Context, memberID string, cap *int) error {
	if s.exists != nil && !s.exists(ctx, memberID) {
		return ErrNoSuchMember
	}
	if cap == nil {
		// Cleared, not set to the organization's number — the difference
		// shows the next time that number moves.
		return s.db.WithContext(ctx).Table(s.tables.Overrides).
			Where("member_id = ?", memberID).Delete(&gormstore.OverrideRow{}).Error
	}
	row := gormstore.OverrideRow{MemberID: memberID, PublicAddressCap: *cap}
	return s.db.WithContext(ctx).Table(s.tables.Overrides).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "member_id"}},
			UpdateAll: true,
		}).
		Create(&row).Error
}

// Compile-time interface check.
var _ Repository = (*Store)(nil)
