package dispatch

import (
	"errors"
	"strings"
)

var errBusy = errors.New("another write got there first, try again")

var busyMarkers = []string{
	"database table is locked",
	"database is locked",
	"database is deadlocked",
	"SQLITE_BUSY",
	"could not serialize access",
	"deadlock detected",
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, marker := range busyMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
