package memory

import "time"

// nowUTC is the single clock this package reads from, kept in UTC to match
// what the Postgres backend round-trips out of a TIMESTAMPTZ column.
func nowUTC() time.Time { return time.Now().UTC() }
