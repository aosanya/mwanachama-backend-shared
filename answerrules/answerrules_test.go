package answerrules

import "testing"

func TestCheckTimeRange_NeedsBothTimesAndToAfterFrom(t *testing.T) {
	if err := CheckTimeRange("09:00", "17:30"); err != nil {
		t.Fatalf("a normal range was refused: %v", err)
	}
	for _, c := range [][2]string{{"", "17:00"}, {"09:00", ""}, {"9am", "17:00"}, {"09:00", "09:00"}, {"18:00", "09:00"}} {
		if err := CheckTimeRange(c[0], c[1]); err == nil {
			t.Errorf("%q to %q was accepted", c[0], c[1])
		}
	}
}

func TestTimeRangeWords_RoundTripsThroughTimeRangeOfWords(t *testing.T) {
	words := TimeRangeWords("09:00", "17:30")
	if words != "from 09:00 to 17:30" {
		t.Fatalf("words = %q", words)
	}
	from, to, ok := TimeRangeOfWords(words)
	if !ok || from != "09:00" || to != "17:30" {
		t.Fatalf("got %q %q %v", from, to, ok)
	}
	if _, _, ok := TimeRangeOfWords("nine to five"); ok {
		t.Error("loose words were read as a range")
	}
}

func TestPlaceRefFits_AddressOrStreetFollowedByWhatWasPicked(t *testing.T) {
	for _, ref := range []string{"address:a1", "street:Fő tér"} {
		if !PlaceRefFits(ref) {
			t.Errorf("%q refused", ref)
		}
	}
	for _, ref := range []string{"", "address:", "street:  ", "road:x", "a1"} {
		if PlaceRefFits(ref) {
			t.Errorf("%q accepted", ref)
		}
	}
}
