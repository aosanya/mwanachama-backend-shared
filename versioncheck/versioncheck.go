package versioncheck

import (
	"errors"
	"time"
)

const wholeRecord = "record"

type Field struct {
	Name   string
	Sent   string
	Stored string
}

type StaleError struct {
	Field string
}

func (e *StaleError) Error() string {
	return e.Field + " changed since you read it"
}

func Stale(version string, updatedAt time.Time, fields []Field) error {
	if version == "" || sameSecond(version, updatedAt) {
		return nil
	}
	return &StaleError{Field: firstChanged(fields)}
}

func IsStale(err error) (*StaleError, bool) {
	var stale *StaleError
	if errors.As(err, &stale) {
		return stale, true
	}
	return nil, false
}

func sameSecond(version string, updatedAt time.Time) bool {
	seen, err := parse(version)
	if err != nil {
		return false
	}
	return seen.Truncate(time.Second).Equal(updatedAt.Truncate(time.Second))
}

func parse(version string) (time.Time, error) {
	seen, err := time.Parse(time.RFC3339Nano, version)
	if err == nil {
		return seen, nil
	}
	return time.Parse(time.RFC3339, version)
}

func firstChanged(fields []Field) string {
	if len(fields) == 0 {
		return wholeRecord
	}
	for _, f := range fields {
		if f.Sent != f.Stored {
			return f.Name
		}
	}
	return fields[0].Name
}
