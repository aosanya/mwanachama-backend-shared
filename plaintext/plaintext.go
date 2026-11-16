package plaintext

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrNotOneVisibleLine = errors.New("plaintext: not one visible line")

func CheckLine(label, text string) error {
	if problem := Problem(label, text); problem != "" {
		return fmt.Errorf("%w: %s", ErrNotOneVisibleLine, problem)
	}
	return nil
}
func Problem(label, text string) string {
	if !utf8.ValidString(text) {
		return fmt.Sprintf("a %s is not valid UTF-8", label)
	}
	drawsSomething := false
	for _, r := range text {
		if IsUnseenOrLineBreaking(r) {
			return fmt.Sprintf("a %s is one line with no control or invisible characters", label)
		}
		drawsSomething = drawsSomething || unicode.In(r, unicode.L, unicode.N, unicode.P, unicode.S)
	}
	if strings.TrimSpace(text) != "" && !drawsSomething {
		return fmt.Sprintf("a %s needs at least one letter, digit or symbol that shows", label)
	}
	return ""
}
func IsUnseenOrLineBreaking(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp, unicode.Co, unicode.Cs, unicode.Variation_Selector) || !unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Zs) || inRanges(r, drawNothingRanges)
}

type runeRange struct{ first, last rune }

var drawNothingRanges = []runeRange{{0x034F, 0x034F}, {0x115F, 0x1160}, {0x17B4, 0x17B5}, {0x2065, 0x2065}, {0x2800, 0x2800}, {0x3164, 0x3164}, {0xFFA0, 0xFFA0}, {0xFFF0, 0xFFF8}, {0xE0000, 0xE0FFF}}

func inRanges(r rune, ranges []runeRange) bool {
	for _, span := range ranges {
		if r >= span.first && r <= span.last {
			return true
		}
	}
	return false
}
func HasNUL(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] == 0 {
			return true
		}
	}
	return false
}
