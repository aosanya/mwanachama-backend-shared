package httpwire

import (
	"log/slog"
	"regexp"
)

var (
	packagePrefix  = regexp.MustCompile(`(^|: )(?:mwanachama[a-z]*|dispatch|plaintext|specstore|httpwire): `)
	functionPrefix = regexp.MustCompile(`(^|: )[A-Z][a-z0-9]+(?:[A-Z][a-z0-9]*)+: `)
)

func PlainMessage(message string) string {
	for {
		next := functionPrefix.ReplaceAllString(packagePrefix.ReplaceAllString(message, "$1"), "$1")
		if next == message {
			return message
		}
		message = next
	}
}

type PlainError struct{ Cause error }

func (e PlainError) Error() string {
	return PlainMessage(e.Cause.Error())
}
func (e PlainError) Unwrap() error {
	return e.Cause
}
func PlainWithLog(err error, route string) string {
	plain := PlainMessage(err.Error())
	if plain != err.Error() {
		slog.Default().Info("a request was refused; the reply is the plain text", "route", route, "error", err.Error(), "reply", plain)
	}
	return plain
}
