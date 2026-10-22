package recur

import (
	"errors"
	"testing"
	"time"
)

var nairobi, _ = time.LoadLocation("Africa/Nairobi")
var budapest, _ = time.LoadLocation("Europe/Budapest")

func day(loc *time.Location, y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, loc)
}

func mustRule(t *testing.T, s string) *Rule {
	t.Helper()
	r, err := ParseRule(s)
	if err != nil {
		t.Fatalf("ParseRule(%q): %v", s, err)
	}
	return &r
}

func dates(ts []time.Time) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Format("2006-01-02 15:04 MST"))
	}
	return out
}

func TestParseRuleRoundTrips(t *testing.T) {
	for _, s := range []string{
		"FREQ=DAILY",
		"FREQ=DAILY;INTERVAL=3",
		"FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE,FR",
		"FREQ=MONTHLY;BYMONTHDAY=1,15,-1",
		"FREQ=MONTHLY;BYDAY=-1FR",
		"FREQ=MONTHLY;INTERVAL=3;BYDAY=2TU",
		"FREQ=YEARLY;BYMONTH=1,4,9",
		"FREQ=YEARLY;INTERVAL=2;BYDAY=4TH;BYMONTH=11",
	} {
		r := mustRule(t, s)
		if got := r.String(); got != s {
			t.Errorf("ParseRule(%q).String() = %q", s, got)
		}
	}
}

func TestParseRuleAcceptsPrefixAndCase(t *testing.T) {
	r := mustRule(t, "RRULE:freq=weekly;byday=mo;wkst=MO")
	if r.String() != "FREQ=WEEKLY;BYDAY=MO" {
		t.Fatalf("got %q", r.String())
	}
}

func TestParseRuleRefuses(t *testing.T) {
	for _, s := range []string{
		"",
		"FREQ=HOURLY",
		"FREQ=DAILY;INTERVAL=0",
		"FREQ=DAILY;INTERVAL=x",
		"FREQ=DAILY;COUNT=3",
		"FREQ=DAILY;UNTIL=20270101T000000Z",
		"FREQ=DAILY;FREQ=WEEKLY",
		"FREQ=WEEKLY;BYDAY=XX",
		"FREQ=WEEKLY;BYDAY=1MO",
		"FREQ=MONTHLY;BYDAY=5MO",
		"FREQ=MONTHLY;BYMONTHDAY=0",
		"FREQ=MONTHLY;BYMONTHDAY=32",
		"FREQ=YEARLY;BYMONTH=13",
		"FREQ=YEARLY;BYDAY=1MO",
		"FREQ=WEEKLY;WKST=SU",
		"FREQ",
	} {
		if _, err := ParseRule(s); !errors.Is(err, ErrInvalidRule) {
			t.Errorf("ParseRule(%q) err = %v, want ErrInvalidRule", s, err)
		}
	}
}

func TestBetween(t *testing.T) {
	cases := []struct {
		name  string
		start time.Time
		end   time.Time
		rule  string
		from  time.Time
		to    time.Time
		want  []string
	}{
		{
			name: "every three days", start: day(nairobi, 2026, 10, 1, 9), rule: "FREQ=DAILY;INTERVAL=3",
			from: day(nairobi, 2026, 10, 1, 0), to: day(nairobi, 2026, 10, 12, 0),
			want: []string{"2026-10-01 09:00 EAT", "2026-10-04 09:00 EAT", "2026-10-07 09:00 EAT", "2026-10-10 09:00 EAT"},
		},
		{
			name: "every second week on monday and thursday", start: day(nairobi, 2026, 10, 7, 9), rule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,TH",
			from: day(nairobi, 2026, 10, 1, 0), to: day(nairobi, 2026, 11, 1, 0),
			want: []string{"2026-10-08 09:00 EAT", "2026-10-19 09:00 EAT", "2026-10-22 09:00 EAT"},
		},
		{
			name: "weekly on the start's weekday", start: day(nairobi, 2026, 10, 7, 9), rule: "FREQ=WEEKLY",
			from: day(nairobi, 2026, 10, 1, 0), to: day(nairobi, 2026, 10, 22, 0),
			want: []string{"2026-10-07 09:00 EAT", "2026-10-14 09:00 EAT", "2026-10-21 09:00 EAT"},
		},
		{
			name: "monthly on the 31st skips short months", start: day(nairobi, 2026, 1, 31, 8), rule: "FREQ=MONTHLY",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2026, 6, 1, 0),
			want: []string{"2026-01-31 08:00 EAT", "2026-03-31 08:00 EAT", "2026-05-31 08:00 EAT"},
		},
		{
			name: "monthly on the first and the last day", start: day(nairobi, 2026, 1, 1, 8), rule: "FREQ=MONTHLY;BYMONTHDAY=1,-1",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2026, 3, 2, 0),
			want: []string{"2026-01-01 08:00 EAT", "2026-01-31 08:00 EAT", "2026-02-01 08:00 EAT", "2026-02-28 08:00 EAT", "2026-03-01 08:00 EAT"},
		},
		{
			name: "monthly on the last friday", start: day(nairobi, 2026, 1, 1, 8), rule: "FREQ=MONTHLY;BYDAY=-1FR",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2026, 4, 1, 0),
			want: []string{"2026-01-30 08:00 EAT", "2026-02-27 08:00 EAT", "2026-03-27 08:00 EAT"},
		},
		{
			name: "every second month on the second tuesday", start: day(nairobi, 2026, 1, 1, 8), rule: "FREQ=MONTHLY;INTERVAL=2;BYDAY=2TU",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2026, 7, 1, 0),
			want: []string{"2026-01-13 08:00 EAT", "2026-03-10 08:00 EAT", "2026-05-12 08:00 EAT"},
		},
		{
			name: "first monday and fourth wednesday", start: day(nairobi, 2026, 10, 1, 8), rule: "FREQ=MONTHLY;BYDAY=1MO,4WE",
			from: day(nairobi, 2026, 10, 1, 0), to: day(nairobi, 2026, 12, 1, 0),
			want: []string{"2026-10-05 08:00 EAT", "2026-10-28 08:00 EAT", "2026-11-02 08:00 EAT", "2026-11-25 08:00 EAT"},
		},
		{
			name: "yearly in chosen months", start: day(nairobi, 2026, 1, 15, 8), rule: "FREQ=YEARLY;BYMONTH=1,7",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2028, 1, 1, 0),
			want: []string{"2026-01-15 08:00 EAT", "2026-07-15 08:00 EAT", "2027-01-15 08:00 EAT", "2027-07-15 08:00 EAT"},
		},
		{
			name: "yearly on the fourth thursday of november", start: day(nairobi, 2026, 1, 1, 8), rule: "FREQ=YEARLY;BYMONTH=11;BYDAY=4TH",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2028, 1, 1, 0),
			want: []string{"2026-11-26 08:00 EAT", "2027-11-25 08:00 EAT"},
		},
		{
			name: "yearly on february 29 waits for a leap year", start: day(nairobi, 2028, 2, 29, 8), rule: "FREQ=YEARLY",
			from: day(nairobi, 2028, 1, 1, 0), to: day(nairobi, 2033, 1, 1, 0),
			want: []string{"2028-02-29 08:00 EAT", "2032-02-29 08:00 EAT"},
		},
		{
			name: "every two years", start: day(nairobi, 2026, 3, 1, 8), rule: "FREQ=YEARLY;INTERVAL=2",
			from: day(nairobi, 2026, 1, 1, 0), to: day(nairobi, 2031, 1, 1, 0),
			want: []string{"2026-03-01 08:00 EAT", "2028-03-01 08:00 EAT", "2030-03-01 08:00 EAT"},
		},
		{
			name: "the end is inclusive", start: day(nairobi, 2026, 10, 1, 9), end: day(nairobi, 2026, 10, 3, 9), rule: "FREQ=DAILY",
			from: day(nairobi, 2026, 9, 1, 0), to: day(nairobi, 2027, 1, 1, 0),
			want: []string{"2026-10-01 09:00 EAT", "2026-10-02 09:00 EAT", "2026-10-03 09:00 EAT"},
		},
		{
			name: "the window's end is exclusive", start: day(nairobi, 2026, 10, 1, 9), rule: "FREQ=DAILY",
			from: day(nairobi, 2026, 10, 2, 9), to: day(nairobi, 2026, 10, 4, 9),
			want: []string{"2026-10-02 09:00 EAT", "2026-10-03 09:00 EAT"},
		},
		{
			name: "daily filtered to weekdays", start: day(nairobi, 2026, 10, 22, 9), rule: "FREQ=DAILY;BYDAY=MO,TU,WE,TH,FR",
			from: day(nairobi, 2026, 10, 22, 0), to: day(nairobi, 2026, 10, 28, 0),
			want: []string{"2026-10-22 09:00 EAT", "2026-10-23 09:00 EAT", "2026-10-26 09:00 EAT", "2026-10-27 09:00 EAT"},
		},
		{
			name: "keeps the wall clock across daylight saving", start: day(budapest, 2026, 10, 23, 9), rule: "FREQ=DAILY",
			from: day(budapest, 2026, 10, 24, 0), to: day(budapest, 2026, 10, 27, 0),
			want: []string{"2026-10-24 09:00 CEST", "2026-10-25 09:00 CET", "2026-10-26 09:00 CET"},
		},
		{
			name: "a window far from the start", start: day(nairobi, 2020, 1, 6, 9), rule: "FREQ=WEEKLY;INTERVAL=3",
			from: day(nairobi, 2026, 10, 1, 0), to: day(nairobi, 2026, 11, 1, 0),
			want: []string{"2026-10-19 09:00 EAT"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Schedule{Start: c.start, End: c.end, Zone: c.start.Location(), Rule: mustRule(t, c.rule)}
			got := dates(s.Between(c.from, c.to))
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestDatesSchedule(t *testing.T) {
	var ds []Date
	for _, s := range []string{"2027-05-03", "2027-01-04", "2027-08-30", "2027-01-04"} {
		d, err := ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		ds = append(ds, d)
	}
	s := Schedule{Start: day(nairobi, 2027, 1, 1, 8), Zone: nairobi, Dates: ds}
	got := dates(s.Between(day(nairobi, 2026, 1, 1, 0), day(nairobi, 2028, 1, 1, 0)))
	want := []string{"2027-01-04 08:00 EAT", "2027-05-03 08:00 EAT", "2027-08-30 08:00 EAT"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got %v, want %v", got, want)
	}
	next, ok := s.Next(day(nairobi, 2027, 1, 4, 8))
	if !ok || !next.Equal(day(nairobi, 2027, 5, 3, 8)) {
		t.Fatalf("Next = %v %v", next, ok)
	}
	if _, ok := s.Next(day(nairobi, 2027, 8, 30, 8)); ok {
		t.Fatal("a list of dates has nothing after its last")
	}
	start, end, ok := s.Period(day(nairobi, 2027, 6, 1, 0))
	if !ok || !start.Equal(day(nairobi, 2027, 5, 3, 8)) || !end.Equal(day(nairobi, 2027, 8, 30, 8)) {
		t.Fatalf("Period = %v %v %v", start, end, ok)
	}
}

func TestNext(t *testing.T) {
	s := Schedule{Start: day(nairobi, 2026, 10, 1, 9), Zone: nairobi, Rule: mustRule(t, "FREQ=MONTHLY;BYDAY=-1FR")}
	cases := []struct {
		after time.Time
		want  time.Time
	}{
		{day(nairobi, 2026, 1, 1, 0), day(nairobi, 2026, 10, 30, 9)},
		{day(nairobi, 2026, 10, 30, 9), day(nairobi, 2026, 11, 27, 9)},
		{day(nairobi, 2026, 10, 30, 8), day(nairobi, 2026, 10, 30, 9)},
		{day(nairobi, 2030, 6, 1, 0), day(nairobi, 2030, 6, 28, 9)},
	}
	for _, c := range cases {
		got, ok := s.Next(c.after)
		if !ok || !got.Equal(c.want) {
			t.Errorf("Next(%v) = %v %v, want %v", c.after, got, ok, c.want)
		}
	}
}

func TestNextStopsAtTheEnd(t *testing.T) {
	s := Schedule{Start: day(nairobi, 2026, 10, 1, 9), End: day(nairobi, 2026, 10, 31, 0), Zone: nairobi, Rule: mustRule(t, "FREQ=WEEKLY")}
	if _, ok := s.Next(day(nairobi, 2026, 10, 29, 9)); ok {
		t.Fatal("no occurrence after the end")
	}
}

func TestNextGivesUpOnARuleThatNeverMatches(t *testing.T) {
	s := Schedule{Start: day(nairobi, 2026, 1, 1, 9), Zone: nairobi, Rule: mustRule(t, "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30")}
	if _, ok := s.Next(day(nairobi, 2026, 1, 1, 0)); ok {
		t.Fatal("february 30 never occurs")
	}
}

func TestPeriod(t *testing.T) {
	weekly := Schedule{Start: day(nairobi, 2026, 10, 5, 9), Zone: nairobi, Rule: mustRule(t, "FREQ=WEEKLY")}
	ended := Schedule{Start: day(nairobi, 2026, 10, 5, 9), End: day(nairobi, 2026, 10, 20, 17), Zone: nairobi, Rule: mustRule(t, "FREQ=WEEKLY")}
	cases := []struct {
		name       string
		s          Schedule
		at         time.Time
		start, end time.Time
		ok         bool
	}{
		{"inside a period", weekly, day(nairobi, 2026, 10, 15, 12), day(nairobi, 2026, 10, 12, 9), day(nairobi, 2026, 10, 19, 9), true},
		{"on an occurrence", weekly, day(nairobi, 2026, 10, 12, 9), day(nairobi, 2026, 10, 12, 9), day(nairobi, 2026, 10, 19, 9), true},
		{"before the start", weekly, day(nairobi, 2026, 10, 1, 0), time.Time{}, time.Time{}, false},
		{"the last period ends at the schedule's end", ended, day(nairobi, 2026, 10, 20, 12), day(nairobi, 2026, 10, 19, 9), day(nairobi, 2026, 10, 20, 17), true},
		{"after the end the last period still answers", ended, day(nairobi, 2026, 12, 1, 0), day(nairobi, 2026, 10, 19, 9), day(nairobi, 2026, 10, 20, 17), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end, ok := c.s.Period(c.at)
			if ok != c.ok || !start.Equal(c.start) || !end.Equal(c.end) {
				t.Fatalf("Period = %v %v %v, want %v %v %v", start, end, ok, c.start, c.end, c.ok)
			}
		})
	}
}

func TestPeriodWithNoEndAndNoNextIsOpen(t *testing.T) {
	d, _ := ParseDate("2027-01-04")
	s := Schedule{Start: day(nairobi, 2027, 1, 1, 8), Zone: nairobi, Dates: []Date{d}}
	start, end, ok := s.Period(day(nairobi, 2027, 3, 1, 0))
	if !ok || !start.Equal(day(nairobi, 2027, 1, 4, 8)) || !end.IsZero() {
		t.Fatalf("Period = %v %v %v", start, end, ok)
	}
}

func TestValidate(t *testing.T) {
	r := mustRule(t, "FREQ=DAILY")
	d, _ := ParseDate("2027-01-04")
	start := day(nairobi, 2026, 10, 1, 9)
	cases := []struct {
		name string
		s    Schedule
		ok   bool
	}{
		{"a rule", Schedule{Start: start, Zone: nairobi, Rule: r}, true},
		{"dates", Schedule{Start: start, Zone: nairobi, Dates: []Date{d}}, true},
		{"no start", Schedule{Zone: nairobi, Rule: r}, false},
		{"no zone", Schedule{Start: start, Rule: r}, false},
		{"neither", Schedule{Start: start, Zone: nairobi}, false},
		{"both", Schedule{Start: start, Zone: nairobi, Rule: r, Dates: []Date{d}}, false},
		{"end before start", Schedule{Start: start, End: start.Add(-time.Hour), Zone: nairobi, Rule: r}, false},
		{"bad date", Schedule{Start: start, Zone: nairobi, Dates: []Date{{2027, 2, 30}}}, false},
		{"bad rule", Schedule{Start: start, Zone: nairobi, Rule: &Rule{Freq: Daily}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.s.Validate()
			if (err == nil) != c.ok {
				t.Fatalf("Validate() = %v", err)
			}
			if !c.ok && len(c.s.Between(start, start.AddDate(1, 0, 0))) != 0 {
				t.Fatal("an invalid schedule yields nothing")
			}
		})
	}
}

func TestParseDateRefuses(t *testing.T) {
	for _, s := range []string{"", "2027-13-01", "2027-02-30", "04/01/2027"} {
		if _, err := ParseDate(s); !errors.Is(err, ErrInvalidSchedule) {
			t.Errorf("ParseDate(%q) err = %v", s, err)
		}
	}
}
