package plaintext

import (
	"golang.org/x/text/unicode/norm"
	"strings"
)

var lookalikeLetters = map[rune]rune{'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'х': 'x', 'у': 'y', 'і': 'i', 'ј': 'j', 'ѕ': 's', 'ԁ': 'd', 'ԛ': 'q', 'ԝ': 'w', 'ο': 'o', 'α': 'a', 'ρ': 'p', 'υ': 'u', 'ν': 'v', 'ι': 'i', 'κ': 'k', 'ı': 'i'}

func FoldLookalike(name string) string {
	mapped := strings.Map(func(r rune) rune {
		if to, ok := lookalikeLetters[r]; ok {
			return to
		}
		return r
	}, FoldDrawnName(name))
	return norm.NFC.String(mapped)
}
