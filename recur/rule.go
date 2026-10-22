package recur

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Frequency string

const (
	Daily   Frequency = "DAILY"
	Weekly  Frequency = "WEEKLY"
	Monthly Frequency = "MONTHLY"
	Yearly  Frequency = "YEARLY"
)

const Last = -1

type Weekday struct {
	Day time.Weekday
	Nth int
}

type Rule struct {
	Freq       Frequency
	Interval   int
	ByDay      []Weekday
	ByMonthDay []int
	ByMonth    []time.Month
}

var (
	ErrInvalidRule     = errors.New("recur: invalid rule")
	ErrInvalidSchedule = errors.New("recur: invalid schedule")
)

var dayCodes = map[string]time.Weekday{
	"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday,
	"TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday,
}

var codeOf = map[time.Weekday]string{
	time.Sunday: "SU", time.Monday: "MO", time.Tuesday: "TU", time.Wednesday: "WE",
	time.Thursday: "TH", time.Friday: "FR", time.Saturday: "SA",
}

func ParseRule(s string) (Rule, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "RRULE:")
	r := Rule{Interval: 1}
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || value == "" {
			return Rule{}, fmt.Errorf("%w: %q is not KEY=VALUE", ErrInvalidRule, part)
		}
		key = strings.ToUpper(key)
		if seen[key] {
			return Rule{}, fmt.Errorf("%w: %s given twice", ErrInvalidRule, key)
		}
		seen[key] = true
		var err error
		switch key {
		case "FREQ":
			r.Freq = Frequency(strings.ToUpper(value))
		case "INTERVAL":
			r.Interval, err = strconv.Atoi(value)
		case "BYDAY":
			r.ByDay, err = parseByDay(value)
		case "BYMONTHDAY":
			r.ByMonthDay, err = parseInts(value)
		case "BYMONTH":
			var ms []int
			ms, err = parseInts(value)
			for _, m := range ms {
				r.ByMonth = append(r.ByMonth, time.Month(m))
			}
		case "WKST":
			if strings.ToUpper(value) != "MO" {
				err = fmt.Errorf("%w: only WKST=MO is supported", ErrInvalidRule)
			}
		default:
			err = fmt.Errorf("%w: %s is not supported", ErrInvalidRule, key)
		}
		if err != nil {
			if errors.Is(err, ErrInvalidRule) {
				return Rule{}, err
			}
			return Rule{}, fmt.Errorf("%w: %s: %v", ErrInvalidRule, key, err)
		}
	}
	return r, r.Validate()
}

func parseInts(value string) ([]int, error) {
	var out []int
	for _, v := range strings.Split(value, ",") {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func parseByDay(value string) ([]Weekday, error) {
	var out []Weekday
	for _, v := range strings.Split(strings.ToUpper(value), ",") {
		if len(v) < 2 {
			return nil, fmt.Errorf("%q is not a weekday", v)
		}
		day, ok := dayCodes[v[len(v)-2:]]
		if !ok {
			return nil, fmt.Errorf("%q is not a weekday", v)
		}
		w := Weekday{Day: day}
		if prefix := v[:len(v)-2]; prefix != "" {
			n, err := strconv.Atoi(prefix)
			if err != nil {
				return nil, fmt.Errorf("%q is not a weekday", v)
			}
			w.Nth = n
		}
		out = append(out, w)
	}
	return out, nil
}

func (r Rule) Validate() error {
	switch r.Freq {
	case Daily, Weekly, Monthly, Yearly:
	default:
		return fmt.Errorf("%w: FREQ must be DAILY, WEEKLY, MONTHLY or YEARLY", ErrInvalidRule)
	}
	if r.Interval < 1 {
		return fmt.Errorf("%w: INTERVAL must be at least 1", ErrInvalidRule)
	}
	for _, w := range r.ByDay {
		if w.Nth != 0 && w.Nth != Last && (w.Nth < 1 || w.Nth > 4) {
			return fmt.Errorf("%w: a weekday's position must be 1 to 4 or -1", ErrInvalidRule)
		}
		if w.Nth != 0 && r.Freq != Monthly && r.Freq != Yearly {
			return fmt.Errorf("%w: a weekday's position needs FREQ=MONTHLY or YEARLY", ErrInvalidRule)
		}
	}
	for _, d := range r.ByMonthDay {
		if d == 0 || d < -31 || d > 31 {
			return fmt.Errorf("%w: BYMONTHDAY must be 1 to 31 or -1 to -31", ErrInvalidRule)
		}
	}
	for _, m := range r.ByMonth {
		if m < time.January || m > time.December {
			return fmt.Errorf("%w: BYMONTH must be 1 to 12", ErrInvalidRule)
		}
	}
	if r.Freq == Yearly && len(r.ByMonth) == 0 && (len(r.ByDay) > 0 || len(r.ByMonthDay) > 0) {
		return fmt.Errorf("%w: a yearly rule naming days must name its months", ErrInvalidRule)
	}
	return nil
}

func (r Rule) String() string {
	parts := []string{"FREQ=" + string(r.Freq)}
	if r.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.Interval))
	}
	if len(r.ByDay) > 0 {
		var ds []string
		for _, w := range r.ByDay {
			code := codeOf[w.Day]
			if w.Nth != 0 {
				code = strconv.Itoa(w.Nth) + code
			}
			ds = append(ds, code)
		}
		parts = append(parts, "BYDAY="+strings.Join(ds, ","))
	}
	if len(r.ByMonthDay) > 0 {
		parts = append(parts, "BYMONTHDAY="+joinInts(r.ByMonthDay))
	}
	if len(r.ByMonth) > 0 {
		var ms []int
		for _, m := range r.ByMonth {
			ms = append(ms, int(m))
		}
		parts = append(parts, "BYMONTH="+joinInts(ms))
	}
	return strings.Join(parts, ";")
}

func joinInts(ns []int) string {
	var s []string
	for _, n := range ns {
		s = append(s, strconv.Itoa(n))
	}
	return strings.Join(s, ",")
}

type civil struct {
	y int
	m time.Month
	d int
}

func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (r Rule) daysInMonth(y int, m time.Month, fallback int) []int {
	n := daysIn(y, m)
	var md []int
	for _, d := range r.ByMonthDay {
		if d < 0 {
			d = n + 1 + d
		}
		if d >= 1 && d <= n {
			md = append(md, d)
		}
	}
	var wd []int
	for _, w := range r.ByDay {
		var matches []int
		for d := 1; d <= n; d++ {
			if time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Weekday() == w.Day {
				matches = append(matches, d)
			}
		}
		switch {
		case w.Nth == 0:
			wd = append(wd, matches...)
		case w.Nth == Last:
			wd = append(wd, matches[len(matches)-1])
		case w.Nth <= len(matches):
			wd = append(wd, matches[w.Nth-1])
		}
	}
	var out []int
	switch {
	case len(r.ByMonthDay) > 0 && len(r.ByDay) > 0:
		in := map[int]bool{}
		for _, d := range wd {
			in[d] = true
		}
		for _, d := range md {
			if in[d] {
				out = append(out, d)
			}
		}
	case len(r.ByMonthDay) > 0:
		out = md
	case len(r.ByDay) > 0:
		out = wd
	case fallback <= n:
		out = []int{fallback}
	}
	return out
}

func (r Rule) monthAllowed(m time.Month) bool {
	if len(r.ByMonth) == 0 {
		return true
	}
	for _, b := range r.ByMonth {
		if b == m {
			return true
		}
	}
	return false
}

func (r Rule) dayAllowed(c civil) bool {
	if !r.monthAllowed(c.m) {
		return false
	}
	if len(r.ByMonthDay) == 0 && len(r.ByDay) == 0 {
		return true
	}
	for _, d := range r.daysInMonth(c.y, c.m, 0) {
		if d == c.d {
			return true
		}
	}
	return false
}

func (r Rule) period(first civil, k int) []civil {
	var out []civil
	switch r.Freq {
	case Daily:
		t := time.Date(first.y, first.m, first.d+k*r.Interval, 0, 0, 0, 0, time.UTC)
		c := civil{t.Year(), t.Month(), t.Day()}
		if r.dayAllowed(c) {
			out = append(out, c)
		}
	case Weekly:
		base := time.Date(first.y, first.m, first.d, 0, 0, 0, 0, time.UTC)
		monday := base.AddDate(0, 0, -mondayOffset(base.Weekday())+7*k*r.Interval)
		days := []time.Weekday{base.Weekday()}
		if len(r.ByDay) > 0 {
			days = nil
			for _, w := range r.ByDay {
				days = append(days, w.Day)
			}
		}
		for _, wd := range days {
			t := monday.AddDate(0, 0, mondayOffset(wd))
			c := civil{t.Year(), t.Month(), t.Day()}
			if r.monthAllowed(c.m) {
				out = append(out, c)
			}
		}
	case Monthly:
		t := time.Date(first.y, first.m+time.Month(k*r.Interval), 1, 0, 0, 0, 0, time.UTC)
		if r.monthAllowed(t.Month()) {
			for _, d := range r.daysInMonth(t.Year(), t.Month(), first.d) {
				out = append(out, civil{t.Year(), t.Month(), d})
			}
		}
	case Yearly:
		y := first.y + k*r.Interval
		months := r.ByMonth
		if len(months) == 0 {
			months = []time.Month{first.m}
		}
		for _, m := range months {
			for _, d := range r.daysInMonth(y, m, first.d) {
				out = append(out, civil{y, m, d})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].before(out[j]) })
	return dedupe(out)
}

func (r Rule) periodsBetween(first, at civil) int {
	a := time.Date(first.y, first.m, first.d, 0, 0, 0, 0, time.UTC)
	b := time.Date(at.y, at.m, at.d, 0, 0, 0, 0, time.UTC)
	var n int
	switch r.Freq {
	case Daily:
		n = int(b.Sub(a).Hours() / 24)
	case Weekly:
		ma := a.AddDate(0, 0, -mondayOffset(a.Weekday()))
		mb := b.AddDate(0, 0, -mondayOffset(b.Weekday()))
		n = int(mb.Sub(ma).Hours()/24) / 7
	case Monthly:
		n = (at.y-first.y)*12 + int(at.m-first.m)
	case Yearly:
		n = at.y - first.y
	}
	if n < 0 {
		return 0
	}
	return n / r.Interval
}

func mondayOffset(w time.Weekday) int {
	return (int(w) + 6) % 7
}

func (c civil) before(o civil) bool {
	if c.y != o.y {
		return c.y < o.y
	}
	if c.m != o.m {
		return c.m < o.m
	}
	return c.d < o.d
}

func dedupe(cs []civil) []civil {
	var out []civil
	for i, c := range cs {
		if i == 0 || c != cs[i-1] {
			out = append(out, c)
		}
	}
	return out
}
