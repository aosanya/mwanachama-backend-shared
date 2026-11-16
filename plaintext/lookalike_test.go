package plaintext

import "testing"

func TestFoldLookalikeMeetsLettersThatLookAlike(t *testing.T) {
	same := [][2]string{{"Pap Irén", "Раp  IRÉN."}, {"Pap Irén", "Paρ Irе́n"}, {"Liaison", "Lіaison"}, {"Mayor", "Ｍаyor"}}
	for _, pair := range same {
		if FoldLookalike(pair[0]) != FoldLookalike(pair[1]) {
			t.Errorf("%q and %q fold apart: %q %q", pair[0], pair[1], FoldLookalike(pair[0]), FoldLookalike(pair[1]))
		}
	}
}
func TestFoldLookalikeKeepsHungarianLettersApart(t *testing.T) {
	apart := [][2]string{{"Pap Irén", "Pap Iren"}, {"Kovács", "Kovacs"}, {"Őze", "Oze"}, {"Török", "Torok"}, {"Nyúl", "Nyul"}}
	for _, pair := range apart {
		if FoldLookalike(pair[0]) == FoldLookalike(pair[1]) {
			t.Errorf("%q and %q fold together as %q", pair[0], pair[1], FoldLookalike(pair[0]))
		}
	}
}
