package plaintext

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

var ErrNotOneVisibleLine = errors.New("plaintext: not one visible line")

func CheckLine(label, text string) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("%w: a %s is not valid UTF-8", ErrNotOneVisibleLine, label)
	}
	for _, r := range text {
		if IsUnseenOrLineBreaking(r) {
			return fmt.Errorf("%w: a %s is one line with no control characters", ErrNotOneVisibleLine, label)
		}
	}
	return nil
}
func IsUnseenOrLineBreaking(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}
func HasNUL(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] == 0 {
			return true
		}
	}
	return false
}
