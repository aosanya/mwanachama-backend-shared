package models

import "time"

// TimeLayout is the fixed-width, nanosecond-precision RFC 3339 layout
// timestamp fields on this package's rows are written with, matching
// mwanachama-backend-actor's models/time.go convention: a GORM-managed
// time.Time column is not used, so every repo that made this choice writes
// the same layout rather than each inventing its own.
const TimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// NowRFC3339 returns the current UTC time formatted per [TimeLayout].
func NowRFC3339() string {
	return time.Now().UTC().Format(TimeLayout)
}
