package plaintext

import (
	"errors"
	"testing"
)

func TestCheckTextRefusesControlBytesAndKeepsLayout(t *testing.T) {
	refused := map[string]string{"nul": "a\u0000b", "bell": "a\u0007b", "escape": "a\u001bb", "delete": "a\u007fb", "c1 control": "a\u0085b", "invalid utf8": "a\xc3b"}
	for label, text := range refused {
		err := CheckText("note", text)
		if !errors.Is(err, ErrNotPlainText) {
			t.Errorf("%s: CheckText(%q) = %v, want ErrNotPlainText", label, text, err)
		}
	}
	accepted := map[string]string{"empty": "", "hungarian": "Árvíztűrő tükörfúrógép", "newline": "Kertváros\nMalom utca 4.", "windows": "Kertváros\r\nMalom utca 4.", "tab": "Malom utca\t4.", "line sep": "a b", "zero width": "a‍b"}
	for label, text := range accepted {
		if err := CheckText("note", text); err != nil {
			t.Errorf("%s: CheckText(%q) = %v, want nil", label, text, err)
		}
	}
}
