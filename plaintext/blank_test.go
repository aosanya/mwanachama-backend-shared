package plaintext

import (
	"errors"
	"testing"
)

func TestCheckLineRefusesWhatDrawsNothing(t *testing.T) {
	for label, raw := range map[string]string{"hangul filler": "ㅤㅤㅤ", "halfwidth hangul filler": "ﾠ", "hangul choseong filler": "ᅟ", "hangul jungseong filler": "ᅠ", "braille blank": "⠀", "private use": "", "supplementary private": "\U000f0000", "combining grapheme": "͏", "khmer inherent vowel": "឴", "variation selector": "️", "mongolian variation": "᠋", "unassigned": "͸", "tag character": "\U000e0041", "only a combining mark": "́", "only marks and a space": " ́̋ ", "filler between words": "AnnaㅤPolgármester", "braille blank in a name": "Anna⠀Kovács", "private use in a name": "AnnaKovács", "variation after a letter": "Anna️"} {
		t.Run(label, func(t *testing.T) {
			if err := CheckLine("name", raw); !errors.Is(err, ErrNotOneVisibleLine) {
				t.Fatalf("%q was admitted: %v", raw, err)
			}
		})
	}
}
func TestCheckLineAdmitsAccentedAndDecomposedNames(t *testing.T) {
	for _, raw := range []string{"Kovács Ádám", "Őze Ödön", "Szűcs Zita", "Kóvács Béla", "Őze Ödön", "Sűcs", "Anna-Mária O'Neil", "Jr. 2.", "Árvíztűrő tükörfúrógép", "山田 太郎", "Zoë", ""} {
		if err := CheckLine("name", raw); err != nil {
			t.Fatalf("%q was refused: %v", raw, err)
		}
	}
}
