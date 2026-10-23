package recur

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type Wire struct {
	Start    string   `json:"start"`
	End      string   `json:"end,omitempty"`
	Zone     string   `json:"zone"`
	Rule     string   `json:"rule,omitempty"`
	Dates    []string `json:"dates,omitempty"`
	OpenDays int      `json:"open_days,omitempty"`
}

func ReadSchedule(raw []byte) (Schedule, error) {
	var w Wire
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return Schedule{}, fmt.Errorf("%w: %v", ErrInvalidSchedule, err)
	}
	return w.Schedule()
}

func (w Wire) Schedule() (Schedule, error) {
	if w.Zone == "" {
		return Schedule{}, fmt.Errorf("%w: a schedule needs a time zone", ErrInvalidSchedule)
	}
	zone, err := time.LoadLocation(w.Zone)
	if err != nil {
		return Schedule{}, fmt.Errorf("%w: unknown time zone %q", ErrInvalidSchedule, w.Zone)
	}
	start, err := time.Parse(time.RFC3339, w.Start)
	if err != nil {
		return Schedule{}, fmt.Errorf("%w: start %q is not RFC 3339", ErrInvalidSchedule, w.Start)
	}
	s := Schedule{Start: start, Zone: zone, OpenDays: w.OpenDays}
	if w.End != "" {
		if s.End, err = time.Parse(time.RFC3339, w.End); err != nil {
			return Schedule{}, fmt.Errorf("%w: end %q is not RFC 3339", ErrInvalidSchedule, w.End)
		}
	}
	if w.Rule != "" {
		rule, err := ParseRule(w.Rule)
		if err != nil {
			return Schedule{}, err
		}
		s.Rule = &rule
	}
	for _, raw := range w.Dates {
		d, err := ParseDate(raw)
		if err != nil {
			return Schedule{}, err
		}
		s.Dates = append(s.Dates, d)
	}
	if len(s.Dates) > 0 && s.End.IsZero() {
		return Schedule{}, fmt.Errorf("%w: a list of dates needs an end", ErrInvalidSchedule)
	}
	return s, s.Validate()
}

func WireOf(s Schedule) Wire {
	w := Wire{Start: s.Start.Format(time.RFC3339), OpenDays: s.OpenDays}
	if s.Zone != nil {
		w.Zone = s.Zone.String()
	}
	if !s.End.IsZero() {
		w.End = s.End.Format(time.RFC3339)
	}
	if s.Rule != nil {
		w.Rule = s.Rule.String()
	}
	for _, d := range s.Dates {
		w.Dates = append(w.Dates, d.String())
	}
	return w
}
