package plaintext

import "testing"

const (
	composedCafe   = "Café körzet"
	decomposedCafe = "Café körzet"
)

func TestFoldNameGivesOneFormToComposedAndDecomposedSpellings(t *testing.T) {
	if FoldName(composedCafe) != FoldName(decomposedCafe) {
		t.Fatalf("%q and %q fold apart", composedCafe, decomposedCafe)
	}
}
func TestFoldNameKeepsCaseAndInnerSpacesApart(t *testing.T) {
	if FoldName("Cafe") == FoldName("cafe") {
		t.Fatal("case was folded")
	}
	if FoldName("Cafe  bar") == FoldName("Cafe bar") {
		t.Fatal("inner spaces were folded")
	}
}
func TestFoldNameDropsOuterSpace(t *testing.T) {
	if FoldName("  Cafe ") != "Cafe" {
		t.Fatalf("got %q", FoldName("  Cafe "))
	}
}
func TestSameNameIsTrueForTheTwoSpellingsOfOneName(t *testing.T) {
	if !SameName(composedCafe, decomposedCafe) || SameName(composedCafe, "Café") {
		t.Fatal("SameName disagrees with the fold")
	}
}
func TestSameNameIgnoringCaseCountsCaseAndSpellingTogether(t *testing.T) {
	if !SameNameIgnoringCase("CAFE\u0301", "caf\u00e9") {
		t.Fatal("case and form were not folded together")
	}
	if SameNameIgnoringCase("CAFE\u0301", "caf") {
		t.Fatal("different names were folded together")
	}
}
func TestFoldDrawnNameGivesOneFormToWidthDotAndSpaceVariants(t *testing.T) {
	want := FoldDrawnName("Kertváros körzet")
	for _, variant := range []string{"Ｋertváros körzet", "Kertváros ｋöｒｚｅｔ", "Kertváros körzet.", "Kertváros körzet...", "Kertváros körzet．", "Kertváros körzet. ", "Kertváros körzet ", "Kertváros  körzet", "Kertváros körzet", "Kertváros　körzet", "Kertváros körzet", " Kertváros körzet "} {
		if got := FoldDrawnName(variant); got != want {
			t.Errorf("FoldDrawnName(%q) = %q, want %q", variant, got, want)
		}
	}
}
func TestFoldDrawnNameKeepsAccentsAndPunctuationApart(t *testing.T) {
	seen := map[string]string{}
	for _, name := range []string{"Kertváros körzet", "Kertvaros korzet", "Kertváros körzet 2", "Kertváros. körzet", ".Kertváros körzet", "Kertváros-körzet", "Kertváros körzet ház"} {
		key := FoldDrawnName(name)
		if other, taken := seen[key]; taken {
			t.Errorf("%q and %q fold together as %q", name, other, key)
		}
		seen[key] = name
	}
}
func TestFoldDrawnNameOfOnlyDotsStaysNonEmpty(t *testing.T) {
	for _, name := range []string{".", "...", ". ."} {
		if FoldDrawnName(name) == "" {
			t.Errorf("FoldDrawnName(%q) is empty", name)
		}
	}
}
func TestFoldDrawnNameIgnoresCase(t *testing.T) {
	if FoldDrawnName("KERTVÁROS Körzet") != FoldDrawnName("kertváros körzet") {
		t.Fatal("case was not folded")
	}
}
