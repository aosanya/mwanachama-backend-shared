package plaintext

import (
	"errors"
	"strings"
)

var ErrUnstorableText = errors.New("plaintext: a value holds a NUL byte or other bytes the database cannot store")

const (
	sqlstateCharacterNotInRepertoire = "22021"
	sqlstateUntranslatableCharacter  = "22P05"
)

type sqlStateError interface {
	error
	SQLState() string
}

func Unstorable(err error) error {
	if err == nil {
		return nil
	}
	var stateErr sqlStateError
	if errors.As(err, &stateErr) {
		if state := stateErr.SQLState(); state == sqlstateCharacterNotInRepertoire || state == sqlstateUntranslatableCharacter {
			return ErrUnstorableText
		}
	}
	if strings.Contains(err.Error(), "invalid byte sequence for encoding") {
		return ErrUnstorableText
	}
	return nil
}
