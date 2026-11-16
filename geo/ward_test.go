package geo

import (
	"errors"
	"testing"
)

func square(minLon, minLat, maxLon, maxLat float64) Geometry {
	return NewPolygon(Ring{{minLon, minLat}, {maxLon, minLat}, {maxLon, maxLat}, {minLon, maxLat}, {minLon, minLat}})
}
func threeWards(t *testing.T) *WardIndex {
	t.Helper()
	ix, err := NewWardIndex([]Ward{{ID: "w-kertvaros", Name: "Kertváros", Boundary: square(19.10, 47.50, 19.20, 47.60)}, {ID: "w-ovaros", Name: "Óváros", Boundary: square(19.00, 47.50, 19.10, 47.60)}, {ID: "w-malomsor", Name: "Malomsor", Boundary: NewPolygon(Ring{{19.00, 47.40}, {19.20, 47.40}, {19.20, 47.50}, {19.00, 47.50}, {19.00, 47.40}}, Ring{{19.08, 47.43}, {19.12, 47.43}, {19.12, 47.47}, {19.08, 47.47}, {19.08, 47.43}})}})
	if err != nil {
		t.Fatal(err)
	}
	return ix
}
func TestWardOfFindsTheWardAPointFallsIn(t *testing.T) {
	ix := threeWards(t)
	cases := map[string]Position{"w-kertvaros": {19.15, 47.55}, "w-ovaros": {19.05, 47.55}, "w-malomsor": {19.02, 47.42}}
	for want, at := range cases {
		got, err := ix.WardOf(at)
		if err != nil || got.ID != want {
			t.Errorf("WardOf(%v) = %q, %v; want %q", at, got.ID, err, want)
		}
	}
}
func TestWardOfOutsideEveryWardIsAnError(t *testing.T) {
	_, err := threeWards(t).WardOf(Position{20.5, 48.0})
	if !errors.Is(err, ErrNoWard) {
		t.Fatalf("got %v, want ErrNoWard", err)
	}
}
func TestWardOfSkipsAHoleInABoundary(t *testing.T) {
	_, err := threeWards(t).WardOf(Position{19.10, 47.45})
	if !errors.Is(err, ErrNoWard) {
		t.Fatalf("a point in the hole belongs to no ward, got %v", err)
	}
}
func TestWardOfOnASharedEdgeIsDecidedByWardID(t *testing.T) {
	ix := threeWards(t)
	at := Position{19.10, 47.55}
	got, err := ix.WardOf(at)
	if err != nil || got.ID != "w-kertvaros" {
		t.Fatalf("got %q, %v; want w-kertvaros", got.ID, err)
	}
	if n := len(ix.WardsOf(at)); n != 2 {
		t.Fatalf("a shared edge lies in 2 wards, got %d", n)
	}
}
func TestWardOfGeometryUsesTheParcelCentre(t *testing.T) {
	ix := threeWards(t)
	got, err := ix.WardOfGeometry(square(19.12, 47.52, 19.14, 47.54))
	if err != nil || got.ID != "w-kertvaros" {
		t.Fatalf("got %q, %v; want w-kertvaros", got.ID, err)
	}
}
func TestNewWardIndexRejectsABoundaryThatIsNotAnArea(t *testing.T) {
	_, err := NewWardIndex([]Ward{{ID: "w", Boundary: NewPoint(19, 47)}})
	if !errors.Is(err, ErrInvalidGeometry) {
		t.Fatalf("got %v, want ErrInvalidGeometry", err)
	}
}
func TestContainsRespectsMultiPolygon(t *testing.T) {
	g := NewMultiPolygon(square(0, 0, 1, 1).Polygon, square(5, 5, 6, 6).Polygon)
	if !Contains(g, Position{5.5, 5.5}) || Contains(g, Position{3, 3}) {
		t.Fatal("multipolygon containment wrong")
	}
}
func TestDistanceBetweenTwoDegreesOfLatitude(t *testing.T) {
	d := Distance(Position{19, 47}, Position{19, 48})
	if d < 111000 || d > 111400 {
		t.Fatalf("one degree of latitude is about 111 km, got %.0f m", d)
	}
}
