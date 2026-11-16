package plaintext

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

var ErrUnstorableText = errors.New("plaintext: a value holds a NUL byte or other bytes the database cannot store")

const (
	sqlstateCharacterNotInRepertoire = "22021"
	sqlstateUntranslatableCharacter  = "22P05"
)

func Unstorable(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == sqlstateCharacterNotInRepertoire || pgErr.Code == sqlstateUntranslatableCharacter) {
		return ErrUnstorableText
	}
	if strings.Contains(err.Error(), "invalid byte sequence for encoding") {
		return ErrUnstorableText
	}
	return nil
}
