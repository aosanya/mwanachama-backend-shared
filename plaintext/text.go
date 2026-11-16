package plaintext

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

var ErrNotPlainText = errors.New("plaintext: not plain text")

func CheckText(label, text string) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrNotPlainText, label)
	}
	for _, r := range text {
		if isControlOtherThanLayout(r) {
			return fmt.Errorf("%w: %s may hold line breaks and tabs but no other control characters", ErrNotPlainText, label)
		}
	}
	return nil
}
func isControlOtherThanLayout(r rune) bool {
	if r == '\n' || r == '\r' || r == '\t' {
		return false
	}
	return unicode.IsControl(r)
}
