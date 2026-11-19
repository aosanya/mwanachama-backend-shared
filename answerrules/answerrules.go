package answerrules

import (
	"fmt"
	"strings"
	"time"
)

const (
	TimeLayout      = "15:04"
	PlaceRefAddress = "address:"
	PlaceRefStreet  = "street:"
)

func CheckTimeRange(from, to string) error {
	start, err := time.Parse(TimeLayout, from)
	if err != nil {
		return fmt.Errorf("time_range answer requires value_time as HH:MM: %w", err)
	}
	end, err := time.Parse(TimeLayout, to)
	if err != nil {
		return fmt.Errorf("time_range answer requires value_time_to as HH:MM: %w", err)
	}
	if !end.After(start) {
		return fmt.Errorf("value_time_to %s must be after value_time %s", to, from)
	}
	return nil
}

func TimeRangeWords(from, to string) string {
	return "from " + from + " to " + to
}

func TimeRangeOfWords(words string) (string, string, bool) {
	rest, found := strings.CutPrefix(words, "from ")
	if !found {
		return "", "", false
	}
	from, to, found := strings.Cut(rest, " to ")
	return from, to, found
}

func PlaceRefFits(ref string) bool {
	for _, prefix := range []string{PlaceRefAddress, PlaceRefStreet} {
		if strings.HasPrefix(ref, prefix) && strings.TrimSpace(strings.TrimPrefix(ref, prefix)) != "" {
			return true
		}
	}
	return false
}
