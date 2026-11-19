package recur

import (
	"testing"
	"time"
)

func TestAnIntervalPastTheLargestThatCanOccurYieldsNoLaterOccurrence(t *testing.T) {
	s := Schedule{Start: day(nairobi, 2026, 1, 1, 9), Zone: nairobi, Rule: mustRule(t, "FREQ=DAILY;INTERVAL=4000000")}
	if got, ok := s.Next(day(nairobi, 2026, 1, 1, 9)); ok {
		t.Fatalf("Next = %v; want none", got)
	}
}

func TestNothingIsGivenAfterTheYear9999(t *testing.T) {
	s := Schedule{Start: day(time.UTC, 9999, 12, 30, 9), Zone: time.UTC, Rule: mustRule(t, "FREQ=DAILY")}
	got := s.Between(day(time.UTC, 9999, 12, 29, 0), time.Date(9999, 12, 31, 23, 0, 0, 0, time.UTC))
	if len(got) != 2 {
		t.Fatalf("Between = %v; want the 30th and the 31st", got)
	}
	if next, ok := s.Next(got[1]); ok {
		t.Fatalf("Next after 9999-12-31 = %v; want none", next)
	}
}

func TestAPeriodLongAfterTheStartIsFoundFromAShortWalk(t *testing.T) {
	daily := Schedule{Start: day(nairobi, 2000, 1, 1, 9), Zone: nairobi, Rule: mustRule(t, "FREQ=DAILY")}
	start, end, ok := daily.Period(day(nairobi, 2026, 11, 19, 12))
	if !ok || !start.Equal(day(nairobi, 2026, 11, 19, 9)) || !end.Equal(day(nairobi, 2026, 11, 20, 9)) {
		t.Fatalf("Period = %v %v %v", start, end, ok)
	}
}

func TestAPeriodOfASparseScheduleWidensItsLookbackUntilItFindsAnOccurrence(t *testing.T) {
	yearly := Schedule{Start: day(nairobi, 1990, 3, 1, 9), Zone: nairobi, Rule: mustRule(t, "FREQ=YEARLY")}
	start, end, ok := yearly.Period(day(nairobi, 2026, 11, 19, 12))
	if !ok || !start.Equal(day(nairobi, 2026, 3, 1, 9)) || !end.Equal(day(nairobi, 2027, 3, 1, 9)) {
		t.Fatalf("Period = %v %v %v", start, end, ok)
	}
}

func TestDailyCountsAcrossAClockChangeStayWholeDays(t *testing.T) {
	s := Schedule{Start: day(budapest, 2026, 3, 20, 9), Zone: budapest, Rule: mustRule(t, "FREQ=DAILY;INTERVAL=3")}
	got := dates(s.Between(day(budapest, 2026, 3, 20, 0), day(budapest, 2026, 4, 3, 0)))
	want := []string{"2026-03-20 09:00 CET", "2026-03-23 09:00 CET", "2026-03-26 09:00 CET", "2026-03-29 09:00 CEST", "2026-04-01 09:00 CEST"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v; want %v", got, want)
		}
	}
}
