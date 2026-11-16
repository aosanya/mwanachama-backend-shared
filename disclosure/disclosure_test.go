package disclosure

import (
	"errors"
	"math/rand"
	"testing"
)

func published(r Result, key string) (bool, int) {
	for _, o := range r.Outcomes {
		if o.Key == key {
			return o.Published, o.Count
		}
	}
	return false, -1
}
func TestEveryCellAtOrAboveThresholdIsPublished(t *testing.T) {
	r, err := Apply([]Cell{{"Óváros", 12}, {"Kertváros", 9}, {"Malomsor", 5}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range r.Outcomes {
		if !o.Published {
			t.Fatalf("%s was suppressed", o.Key)
		}
	}
	if !r.TotalPublished || r.Total != 26 {
		t.Fatalf("total = %d published = %v", r.Total, r.TotalPublished)
	}
}
func TestSmallCellIsSuppressedAndAnotherCoversIt(t *testing.T) {
	r, err := Apply([]Cell{{"Óváros", 12}, {"Kertváros", 3}, {"Malomsor", 9}, {"Határrét", 20}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := published(r, "Kertváros"); ok {
		t.Fatal("small cell published")
	}
	if ok, _ := published(r, "Malomsor"); ok {
		t.Fatal("next smallest cell was not suppressed to cover the small one")
	}
	if ok, n := published(r, "Óváros"); !ok || n != 12 {
		t.Fatal("unrelated cell lost")
	}
	if !r.TotalPublished || r.Total != 44 {
		t.Fatalf("total = %d published = %v", r.Total, r.TotalPublished)
	}
}
func TestTwoSmallCellsWhoseSumIsStillSmallNeedACover(t *testing.T) {
	r, _ := Apply([]Cell{{"a", 1}, {"b", 2}, {"c", 30}, {"d", 40}}, 5)
	if ok, _ := published(r, "c"); ok {
		t.Fatal("hidden sum of 3 is below the threshold and must be covered")
	}
	if ok, _ := published(r, "d"); !ok {
		t.Fatal("larger cell suppressed needlessly")
	}
}
func TestTwoSmallCellsWhoseSumReachesThresholdNeedNoCover(t *testing.T) {
	r, _ := Apply([]Cell{{"a", 2}, {"b", 3}, {"c", 30}}, 5)
	if ok, n := published(r, "c"); !ok || n != 30 {
		t.Fatal("cover added although the hidden sum reaches the threshold")
	}
}
func TestTotalIsWithheldWhenNothingIsLeftToCover(t *testing.T) {
	r, _ := Apply([]Cell{{"a", 1}, {"b", 0}}, 5)
	if r.TotalPublished || r.Total != 0 {
		t.Fatal("total published although it would reveal the only person")
	}
}
func TestZeroCellsAreNotSuppressed(t *testing.T) {
	r, _ := Apply([]Cell{{"a", 0}, {"b", 10}}, 5)
	for _, o := range r.Outcomes {
		if !o.Published {
			t.Fatalf("%s suppressed", o.Key)
		}
	}
}
func TestSuppressedCellsCarryNoCount(t *testing.T) {
	r, _ := Apply([]Cell{{"a", 2}, {"b", 7}, {"c", 50}}, 5)
	for _, o := range r.Outcomes {
		if !o.Published && o.Count != 0 {
			t.Fatalf("%s leaks %d", o.Key, o.Count)
		}
	}
}
func TestInputOrderIsKept(t *testing.T) {
	r, _ := Apply([]Cell{{"z", 9}, {"a", 8}, {"m", 7}}, 5)
	if r.Outcomes[0].Key != "z" || r.Outcomes[1].Key != "a" || r.Outcomes[2].Key != "m" {
		t.Fatal("order changed")
	}
}
func TestRejectsBadInput(t *testing.T) {
	if _, err := Apply(nil, 1); !errors.Is(err, ErrThreshold) {
		t.Fatalf("threshold: %v", err)
	}
	if _, err := Apply([]Cell{{"a", -1}}, 5); !errors.Is(err, ErrNegativeCount) {
		t.Fatalf("negative: %v", err)
	}
	if _, err := Apply([]Cell{{"a", 1}, {"a", 2}}, 5); !errors.Is(err, ErrDuplicateGroup) {
		t.Fatalf("duplicate: %v", err)
	}
}
func TestNoSmallCountIsRecoverableFromThePublishedFigures(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 2000; round++ {
		threshold := 2 + rng.Intn(8)
		cells := make([]Cell, 1+rng.Intn(8))
		for i := range cells {
			cells[i] = Cell{Key: string(rune('a' + i)), Count: rng.Intn(3 * threshold)}
		}
		r, err := Apply(cells, threshold)
		if err != nil {
			t.Fatal(err)
		}
		hidden, hiddenSum, shown := 0, 0, 0
		for i, o := range r.Outcomes {
			if o.Published {
				shown += o.Count
				continue
			}
			hidden++
			hiddenSum += cells[i].Count
		}
		if hidden == 0 {
			continue
		}
		if hidden == 1 && r.TotalPublished {
			t.Fatalf("round %d: single hidden cell is recoverable from the total: %v", round, cells)
		}
		if r.TotalPublished && r.Total-shown != hiddenSum {
			t.Fatalf("round %d: total inconsistent", round)
		}
		if r.TotalPublished && hiddenSum > 0 && hiddenSum < threshold {
			t.Fatalf("round %d: hidden sum %d below threshold %d is recoverable: %v", round, hiddenSum, threshold, cells)
		}
		for i, o := range r.Outcomes {
			if o.Published && cells[i].Count > 0 && cells[i].Count < threshold {
				t.Fatalf("round %d: small cell published: %v", round, cells)
			}
		}
	}
}
func TestFoldSmallMergesSmallCellsIntoOther(t *testing.T) {
	kept, folded, err := FoldSmall([]Cell{{"a", 12}, {"b", 2}, {"c", 3}, {"d", 0}, {"e", 5}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(folded) != 2 || folded[0] != "b" || folded[1] != "c" {
		t.Fatalf("folded = %v", folded)
	}
	want := []Cell{{"a", 12}, {"d", 0}, {"e", 5}, {OtherKey, 5}}
	if len(kept) != len(want) {
		t.Fatalf("kept = %v", kept)
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Fatalf("kept = %v", kept)
		}
	}
}
func TestFoldSmallWithholdsAnOtherCellThatIsStillSmall(t *testing.T) {
	kept, _, err := FoldSmall([]Cell{{"a", 12}, {"b", 2}, {"c", 9}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Apply(kept, 5)
	if err != nil {
		t.Fatal(err)
	}
	if ok, n := published(r, OtherKey); ok || n != 0 {
		t.Fatal("small other cell published")
	}
	hidden := 0
	for _, o := range r.Outcomes {
		if !o.Published {
			hidden++
		}
	}
	if hidden < 2 {
		t.Fatal("lone suppression is recoverable")
	}
}
func TestFoldSmallRejectsReservedKeyAndLowThreshold(t *testing.T) {
	if _, _, err := FoldSmall([]Cell{{OtherKey, 9}}, 5); !errors.Is(err, ErrReservedKey) {
		t.Fatal(err)
	}
	if _, _, err := FoldSmall([]Cell{{"a", 9}}, 1); !errors.Is(err, ErrThreshold) {
		t.Fatal(err)
	}
}
