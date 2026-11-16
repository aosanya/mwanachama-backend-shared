package gormutil_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aosanya/mwanachama-backend-shared/gormutil"
)

func TestIsUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"postgres unique violation", &pgconn.PgError{Code: "23505", ConstraintName: "poll_answers_uniq"}, true},
		{"postgres unique violation wrapped", fmt.Errorf("store: %w", &pgconn.PgError{Code: "23505"}), true},
		{"sqlite unique violation", errors.New("UNIQUE constraint failed: poll_answers.member_id"), true},
		{"postgres foreign key violation", &pgconn.PgError{Code: "23503"}, false},
		{"sqlite not null violation", errors.New("NOT NULL constraint failed: poll_answers.member_id"), false},
		{"unrelated", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		if got := gormutil.IsUniqueViolation(c.err); got != c.want {
			t.Errorf("%s: IsUniqueViolation = %v, want %v", c.name, got, c.want)
		}
	}
}
