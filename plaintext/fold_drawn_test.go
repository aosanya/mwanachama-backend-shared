package plaintext

import "testing"

func TestFoldDrawnNameGivesOneKeyToNamesThatDrawTheSame(t *testing.T) {
	want := "kertváros körzet"
	for _, name := range []string{"Kertváros Körzet", "KERTVÁROS KÖRZET", "Kertváros Körzet", "  Kertváros   Körzet  ", "Kertváros Körzet", "Kertváros　Körzet", "Ｋertváros Körzet", "Kertváros Körzet.", "Kertváros Körzet...", "Kertváros Körzet．", "Kertváros Körzet . "} {
		if got := FoldDrawnName(name); got != want {
			t.Errorf("FoldDrawnName(%q) = %q, want %q", name, got, want)
		}
	}
}
func TestFoldDrawnNameKeepsDifferentNamesApart(t *testing.T) {
	if FoldDrawnName("Kertváros") == FoldDrawnName("Kertvaros") {
		t.Fatal("accents were folded away")
	}
	if FoldDrawnName("Kert-város") == FoldDrawnName("Kert város") {
		t.Fatal("a hyphen was folded into a space")
	}
	if FoldDrawnName("Kertváros 2.") != FoldDrawnName("Kertváros 2") {
		t.Fatal("a trailing dot after a digit was kept")
	}
}
func TestFoldDrawnNameOfOnlyDotsStaysNotEmpty(t *testing.T) {
	for _, name := range []string{".", "...", "．", " . . "} {
		if FoldDrawnName(name) == "" {
			t.Errorf("FoldDrawnName(%q) is empty", name)
		}
	}
}
