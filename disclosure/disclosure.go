package disclosure

import (
	"errors"
	"fmt"
)

type Cell struct {
	Key   string
	Count int
}
type Outcome struct {
	Key       string
	Count     int
	Published bool
}
type Result struct {
	Outcomes       []Outcome
	Total          int
	TotalPublished bool
}

var (
	ErrThreshold      = errors.New("threshold must be at least 2")
	ErrNegativeCount  = errors.New("count must not be negative")
	ErrDuplicateGroup = errors.New("group key appears twice")
)

func Apply(cells []Cell, threshold int) (Result, error) {
	if threshold < 2 {
		return Result{}, ErrThreshold
	}
	seen := make(map[string]bool, len(cells))
	total := 0
	for _, c := range cells {
		if c.Count < 0 {
			return Result{}, fmt.Errorf("%w: %s", ErrNegativeCount, c.Key)
		}
		if seen[c.Key] {
			return Result{}, fmt.Errorf("%w: %s", ErrDuplicateGroup, c.Key)
		}
		seen[c.Key] = true
		total += c.Count
	}
	suppressed := make([]bool, len(cells))
	for i, c := range cells {
		suppressed[i] = c.Count > 0 && c.Count < threshold
	}
	totalPublished := true
	for needsCover(cells, suppressed, threshold) {
		next := smallestPublished(cells, suppressed)
		if next < 0 {
			totalPublished = false
			break
		}
		suppressed[next] = true
	}
	outcomes := make([]Outcome, len(cells))
	for i, c := range cells {
		outcomes[i] = Outcome{Key: c.Key, Published: !suppressed[i]}
		if !suppressed[i] {
			outcomes[i].Count = c.Count
		}
	}
	result := Result{Outcomes: outcomes, TotalPublished: totalPublished}
	if totalPublished {
		result.Total = total
	}
	return result, nil
}
func needsCover(cells []Cell, suppressed []bool, threshold int) bool {
	hidden, sum := 0, 0
	for i, c := range cells {
		if suppressed[i] {
			hidden++
			sum += c.Count
		}
	}
	return hidden == 1 || (hidden > 1 && sum < threshold)
}
func smallestPublished(cells []Cell, suppressed []bool) int {
	best := -1
	for i, c := range cells {
		if suppressed[i] || c.Count == 0 {
			continue
		}
		if best < 0 || c.Count < cells[best].Count {
			best = i
		}
	}
	return best
}
