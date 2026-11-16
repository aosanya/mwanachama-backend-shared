package geo

import (
	"errors"
	"sort"
)

var ErrNoWard = errors.New("geo: the point is inside no ward")

type Ward struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Boundary Geometry `json:"boundary"`
}
type wardEntry struct {
	ward Ward
	box  Box
}
type WardIndex struct{ entries []wardEntry }

func NewWardIndex(wards []Ward) (*WardIndex, error) {
	entries := make([]wardEntry, 0, len(wards))
	for _, w := range wards {
		if w.Boundary.Kind != KindPolygon && w.Boundary.Kind != KindMultiPolygon {
			return nil, invalid("ward %q needs a Polygon or MultiPolygon boundary, has %q", w.ID, w.Boundary.Kind)
		}
		if err := w.Boundary.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, wardEntry{ward: w, box: BoundingBox(w.Boundary)})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].ward.ID < entries[j].ward.ID
	})
	return &WardIndex{entries: entries}, nil
}
func (ix *WardIndex) WardOf(p Position) (Ward, error) {
	for _, e := range ix.entries {
		if e.box.Contains(p) && Contains(e.ward.Boundary, p) {
			return e.ward, nil
		}
	}
	return Ward{}, ErrNoWard
}
func (ix *WardIndex) WardsOf(p Position) []Ward {
	var out []Ward
	for _, e := range ix.entries {
		if e.box.Contains(p) && Contains(e.ward.Boundary, p) {
			out = append(out, e.ward)
		}
	}
	return out
}
func (ix *WardIndex) WardOfGeometry(g Geometry) (Ward, error) {
	switch g.Kind {
	case KindPoint:
		return ix.WardOf(g.Point)
	case KindPolygon, KindMultiPolygon, KindLineString:
		box := BoundingBox(g)
		return ix.WardOf(Position{(box.MinLon + box.MaxLon) / 2, (box.MinLat + box.MaxLat) / 2})
	}
	return Ward{}, invalid("type %q has no place in a ward", g.Kind)
}
