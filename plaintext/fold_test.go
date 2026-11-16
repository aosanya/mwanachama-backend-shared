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
