package recur

import (
	"errors"
	"testing"
)

func TestReadScheduleReadsARule(t *testing.T) {
	s, err := ReadSchedule([]byte(`{"start":"2026-11-02T09:00:00+03:00","zone":"Africa/Nairobi","rule":"FREQ=WEEKLY;BYDAY=MO","open_days":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Rule == nil || s.Zone.String() != "Africa/Nairobi" || s.OpenDays != 2 || !s.End.IsZero() {
		t.Fatalf("read %+v", s)
	}
	next, ok := s.Next(day(nairobi, 2026, 11, 2, 9))
	if !ok || !next.Equal(day(nairobi, 2026, 11, 9, 9)) {
		t.Fatalf("next = %v", next)
	}
}

func TestReadScheduleReadsDatesWithAnEnd(t *testing.T) {
	s, err := ReadSchedule([]byte(`{"start":"2026-01-05T08:00:00+03:00","end":"2026-12-31T23:59:59+03:00","zone":"Africa/Nairobi","dates":["2026-05-04","2026-01-05"]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Between(day(nairobi, 2026, 1, 1, 0), day(nairobi, 2027, 1, 1, 0))
	if len(got) != 2 || !got[1].Equal(day(nairobi, 2026, 5, 4, 8)) {
		t.Fatalf("between = %v", dates(got))
	}
}

func TestReadScheduleRefuses(t *testing.T) {
	cases := map[string]string{
		"dates without an end":  `{"start":"2026-01-05T08:00:00Z","zone":"UTC","dates":["2026-01-05"]}`,
		"no zone":               `{"start":"2026-01-05T08:00:00Z","rule":"FREQ=DAILY"}`,
		"unknown zone":          `{"start":"2026-01-05T08:00:00Z","zone":"Mars/Olympus","rule":"FREQ=DAILY"}`,
		"no start":              `{"zone":"UTC","rule":"FREQ=DAILY"}`,
		"bad end":               `{"start":"2026-01-05T08:00:00Z","end":"soon","zone":"UTC","rule":"FREQ=DAILY"}`,
		"rule and dates":        `{"start":"2026-01-05T08:00:00Z","end":"2026-02-01T00:00:00Z","zone":"UTC","rule":"FREQ=DAILY","dates":["2026-01-06"]}`,
		"neither rule nor date": `{"start":"2026-01-05T08:00:00Z","zone":"UTC"}`,
		"negative open days":    `{"start":"2026-01-05T08:00:00Z","zone":"UTC","rule":"FREQ=DAILY","open_days":-1}`,
		"comm's old names":      `{"starts_at":"2026-01-05T08:00:00Z","zone":"UTC","rule":"FREQ=DAILY"}`,
		"not json":              `{`,
	}
	for name, body := range cases {
		if _, err := ReadSchedule([]byte(body)); !errors.Is(err, ErrInvalidSchedule) && !errors.Is(err, ErrInvalidRule) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestWireOfRoundTrips(t *testing.T) {
	in := `{"start":"2026-01-05T08:00:00Z","end":"2026-03-01T00:00:00Z","zone":"Europe/Budapest","rule":"FREQ=MONTHLY;BYDAY=1MO","open_days":7}`
	s, err := ReadSchedule([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	w := WireOf(s)
	again, err := w.Schedule()
	if err != nil {
		t.Fatal(err)
	}
	if w.Zone != "Europe/Budapest" || w.OpenDays != 7 || w.End != "2026-03-01T00:00:00Z" || w.Rule != again.Rule.String() {
		t.Fatalf("wire = %+v", w)
	}
}

func TestOpenDaysClosesAPeriodEarly(t *testing.T) {
	s := Schedule{Start: day(budapest, 2026, 1, 5, 9), Zone: budapest, Rule: mustRule(t, "FREQ=MONTHLY;BYDAY=1MO"), OpenDays: 7}
	opens, closes, ok := s.Period(day(budapest, 2026, 3, 3, 12))
	if !ok || !opens.Equal(day(budapest, 2026, 3, 2, 9)) || !closes.Equal(day(budapest, 2026, 3, 9, 9)) {
		t.Fatalf("period = %v .. %v", opens, closes)
	}
}

func TestOpenDaysLongerThanThePeriodClosesAtTheNext(t *testing.T) {
	s := Schedule{Start: day(budapest, 2026, 1, 5, 9), Zone: budapest, Rule: mustRule(t, "FREQ=WEEKLY"), OpenDays: 10}
	_, closes, ok := s.Period(day(budapest, 2026, 1, 6, 9))
	if !ok || !closes.Equal(day(budapest, 2026, 1, 12, 9)) {
		t.Fatalf("closes = %v", closes)
	}
}

func TestOpenDaysBoundsAnOpenEndedLastPeriod(t *testing.T) {
	s := Schedule{Start: day(budapest, 2026, 3, 27, 9), Zone: budapest, Rule: mustRule(t, "FREQ=YEARLY"), OpenDays: 3}
	_, closes, ok := s.Period(day(budapest, 2026, 3, 28, 9))
	if !ok || !closes.Equal(day(budapest, 2026, 3, 30, 9)) {
		t.Fatalf("closes across daylight saving = %v", closes)
	}
}
