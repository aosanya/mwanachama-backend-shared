package plaintext

import (
	"errors"
	"testing"
)

func TestCheckLineRefusesWhatADisplayNameRefuses(t *testing.T) {
	for label, raw := range map[string]string{"nul": "Kert\u0000város", "bell": "Kert\u0007város", "newline": "Kert\nváros", "right to left": "Kert\u202Eváros", "zero width space": "Kert​város", "line separator": "Kert város", "invalid utf-8": "Kert\xffváros"} {
		t.Run(label, func(t *testing.T) {
			if err := CheckLine("name", raw); !errors.Is(err, ErrNotOneVisibleLine) {
				t.Fatalf("%q was admitted: %v", raw, err)
			}
		})
	}
}
func TestCheckLineAdmitsPlainHungarianText(t *testing.T) {
	for _, raw := range []string{"Kertváros körzet", "Őze Ödön", "Anna-Mária O'Neil", "Árvíztűrő tükörfúrógép"} {
		if err := CheckLine("name", raw); err != nil {
			t.Fatalf("%q was refused: %v", raw, err)
		}
	}
}
func TestHasNUL(t *testing.T) {
	if !HasNUL("a\x00b") || HasNUL("ab") {
		t.Fatal("HasNUL misreads a NUL byte")
	}
}
