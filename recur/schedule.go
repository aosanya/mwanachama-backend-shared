package recur

import (
	"fmt"
	"sort"
	"time"
)

type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("%w: %q is not a YYYY-MM-DD date", ErrInvalidSchedule, s)
	}
	return Date{t.Year(), t.Month(), t.Day()}, nil
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

type Schedule struct {
	Start time.Time
	End   time.Time
	Zone  *time.Location
	Rule  *Rule
	Dates []Date
}

const maxEmptyPeriods = 4000

func (s Schedule) Validate() error {
	if s.Start.IsZero() {
		return fmt.Errorf("%w: a schedule needs a start", ErrInvalidSchedule)
	}
	if s.Zone == nil {
		return fmt.Errorf("%w: a schedule needs a time zone", ErrInvalidSchedule)
	}
	if (s.Rule == nil) == (len(s.Dates) == 0) {
		return fmt.Errorf("%w: a schedule is a rule or a list of dates, exactly one", ErrInvalidSchedule)
	}
	if !s.End.IsZero() && s.End.Before(s.Start) {
		return fmt.Errorf("%w: the end falls before the start", ErrInvalidSchedule)
	}
	if s.Rule != nil {
		return s.Rule.Validate()
	}
	for _, d := range s.Dates {
		if d.Month < time.January || d.Month > time.December || d.Day < 1 || d.Day > daysIn(d.Year, d.Month) {
			return fmt.Errorf("%w: %s is not a calendar date", ErrInvalidSchedule, d)
		}
	}
	return nil
}

func (s Schedule) at(c civil) time.Time {
	l := s.Start.In(s.Zone)
	return time.Date(c.y, c.m, c.d, l.Hour(), l.Minute(), l.Second(), 0, s.Zone)
}

func (s Schedule) inRange(t time.Time) bool {
	return !t.Before(s.Start) && (s.End.IsZero() || !t.After(s.End))
}

func (s Schedule) walk(from time.Time, visit func(time.Time) bool) {
	if s.Validate() != nil {
		return
	}
	if s.Rule == nil {
		ts := make([]time.Time, 0, len(s.Dates))
		for _, d := range s.Dates {
			ts = append(ts, s.at(civil{d.Year, d.Month, d.Day}))
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })
		for i, t := range ts {
			if i > 0 && t.Equal(ts[i-1]) {
				continue
			}
			if s.inRange(t) && !t.Before(from) && !visit(t) {
				return
			}
		}
		return
	}
	l := s.Start.In(s.Zone)
	first := civil{l.Year(), l.Month(), l.Day()}
	k := 0
	if from.After(s.Start) {
		f := from.In(s.Zone)
		k = s.Rule.periodsBetween(first, civil{f.Year(), f.Month(), f.Day()})
		if k > 0 {
			k--
		}
	}
	for empty := 0; empty < maxEmptyPeriods; k++ {
		cs := s.Rule.period(first, k)
		found := false
		for _, c := range cs {
			t := s.at(c)
			if !s.End.IsZero() && t.After(s.End) {
				return
			}
			if t.Before(s.Start) || t.Before(from) {
				continue
			}
			found = true
			if !visit(t) {
				return
			}
		}
		if found {
			empty = 0
		} else {
			empty++
		}
	}
}

func (s Schedule) Next(after time.Time) (time.Time, bool) {
	var next time.Time
	s.walk(after, func(t time.Time) bool {
		if t.After(after) {
			next = t
			return false
		}
		return true
	})
	return next, !next.IsZero()
}

func (s Schedule) Between(from, to time.Time) []time.Time {
	var out []time.Time
	s.walk(from, func(t time.Time) bool {
		if !t.Before(to) {
			return false
		}
		out = append(out, t)
		return true
	})
	return out
}

func (s Schedule) Period(at time.Time) (start, end time.Time, ok bool) {
	s.walk(s.Start, func(t time.Time) bool {
		if t.After(at) {
			if !start.IsZero() {
				end = t
			}
			return false
		}
		start = t
		return true
	})
	if start.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	if end.IsZero() {
		end = s.End
	}
	return start, end, true
}
