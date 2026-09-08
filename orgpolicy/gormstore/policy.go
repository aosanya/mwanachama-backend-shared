// Package gormstore holds every GORM-specific piece of orgpolicy: the row
// structs, their conversion to/from models types, and table migration —
// mirroring mwanachama-backend-actor's identical gormstore/ split. Nothing
// outside this package (and the root orgpolicy package's *_impl.go files,
// which call it) needs to know GORM exists.
package gormstore

import (
	"time"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy/models"
)

// PolicyRow is the GORM row for a [models.Policy]. Singleton is the
// deliberate CHECK-backed constant every row must carry — see
// syncSingletonCheck — the same "one row, no organization id" shape
// mwanachama-backend-auth's phone_salt uses, ported from the original
// org_policy table.
type PolicyRow struct {
	Singleton            bool `gorm:"primaryKey;default:true"`
	PublicAddressCap     int
	ChapterMembershipCap int
	FreeTextMaxLengthCap int
	UpdatedAt            time.Time
	UpdatedBy            string
}

// PolicyToRow converts a domain Policy to its row shape.
func PolicyToRow(p models.Policy) PolicyRow {
	return PolicyRow{
		Singleton:            true,
		PublicAddressCap:     p.PublicAddressCap,
		ChapterMembershipCap: p.StructureMembershipCap,
		FreeTextMaxLengthCap: p.FreeTextMaxLengthCap,
		UpdatedBy:            p.UpdatedBy,
	}
}

// PolicyFromRow converts a row back to the domain Policy.
func PolicyFromRow(r PolicyRow) models.Policy {
	return models.Policy{
		PublicAddressCap:       r.PublicAddressCap,
		StructureMembershipCap: r.ChapterMembershipCap,
		FreeTextMaxLengthCap:   r.FreeTextMaxLengthCap,
		UpdatedAt:              r.UpdatedAt,
		UpdatedBy:              r.UpdatedBy,
	}
}
