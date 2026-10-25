package geo_test

import (
	"math"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/geo"
)

func mustParse(t *testing.T, raw string) geo.Geometry {
	t.Helper()
	g, err := geo.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestContainsHonoursHolesAndEdges(t *testing.T) {
	g := mustParse(t, square)
	for name, c := range map[string]struct {
		p    geo.Position
		want bool
	}{
		"inside":                       {geo.Position{2, 2}, true},
		"in the hole":                  {geo.Position{5, 5}, false},
		"on the hole edge":             {geo.Position{4, 5}, true},
		"on the outer edge":            {geo.Position{10, 5}, true},
		"on a corner":                  {geo.Position{0, 0}, true},
		"outside":                      {geo.Position{11, 5}, false},
		"level with a vertex, outside": {geo.Position{-1, 10}, false},
	} {
		if got := geo.Contains(g, c.p); got != c.want {
			t.Errorf("%s: Contains(%v) = %v, want %v", name, c.p, got, c.want)
		}
	}
}

func TestContainsAcrossAMultiPolygonAndAPoint(t *testing.T) {
	g := mustParse(t, `{"type":"MultiPolygon","coordinates":[[[[0,0],[1,0],[1,1],[0,1],[0,0]]],[[[5,5],[6,5],[6,6],[5,6],[5,5]]]]}`)
	if !geo.Contains(g, geo.Position{5.5, 5.5}) || geo.Contains(g, geo.Position{3, 3}) {
		t.Fatal("multipolygon membership is wrong")
	}
	p := geo.NewPoint(1, 2)
	if !geo.Contains(p, geo.Position{1, 2}) || geo.Contains(p, geo.Position{1, 2.0001}) {
		t.Fatal("a point contains only itself")
	}
}

func TestDistance(t *testing.T) {
	nairobi := geo.Position{36.8219, -1.2921}
	mombasa := geo.Position{39.6682, -4.0435}
	if d := geo.Distance(nairobi, mombasa); math.Abs(d-440_000) > 5_000 {
		t.Fatalf("Nairobi to Mombasa = %.0f m, want about 440 km", d)
	}
	if d := geo.Distance(nairobi, nairobi); d != 0 {
		t.Fatalf("a place is %v m from itself", d)
	}
	degree := geo.Distance(geo.Position{0, 0}, geo.Position{0, 1})
	if math.Abs(degree-111_195) > 10 {
		t.Fatalf("one degree of latitude = %.0f m", degree)
	}
	if d := geo.Distance(geo.Position{0, 0}, geo.Position{180, 0}); math.Abs(d-math.Pi*geo.EarthRadiusMetres) > 1 {
		t.Fatalf("antipodes = %.0f m", d)
	}
}

func TestBoundingBox(t *testing.T) {
	b := geo.BoundingBox(mustParse(t, `{"type":"MultiPolygon","coordinates":[[[[0,0],[1,0],[1,1],[0,0]]],[[[-5,5],[6,5],[6,-2],[-5,5]]]]}`))
	if b != (geo.Box{MinLon: -5, MinLat: -2, MaxLon: 6, MaxLat: 5}) {
		t.Fatalf("box = %+v", b)
	}
	if !b.Contains(geo.Position{0, 0}) || b.Contains(geo.Position{7, 0}) {
		t.Fatal("box membership is wrong")
	}
	if p := geo.BoundingBox(geo.NewPoint(3, 4)); p != (geo.Box{MinLon: 3, MinLat: 4, MaxLon: 3, MaxLat: 4}) {
		t.Fatalf("point box = %+v", p)
	}
	if z := geo.BoundingBox(geo.Geometry{}); z != (geo.Box{}) {
		t.Fatalf("empty box = %+v", z)
	}
}

func TestRoundForPublicMovesAPointNoMoreThanAboutAHundredMetres(t *testing.T) {
	p := geo.Position{36.821946, -1.292066}
	r := geo.RoundForPublic(p)
	if r != (geo.Position{36.822, -1.292}) {
		t.Fatalf("rounded to %v", r)
	}
	if d := geo.Distance(p, r); d > 100 {
		t.Fatalf("rounding moved the point %.0f m", d)
	}
	if geo.RoundForPublic(geo.Position{36.8216, -1.2924}) != geo.RoundForPublic(geo.Position{36.8224, -1.2916}) {
		t.Fatal("two homes in the same cell read apart")
	}
}

func TestALineHoldsOnlyThePointsOnIt(t *testing.T) {
	g := mustParse(t, `{"type":"LineString","coordinates":[[0,0],[2,0],[2,2]]}`)
	if !geo.Contains(g, geo.Position{1, 0}) || !geo.Contains(g, geo.Position{2, 1}) {
		t.Fatal("a point on the line is not held")
	}
	if geo.Contains(g, geo.Position{1, 1}) {
		t.Fatal("a point off the line is held")
	}
	if b := geo.BoundingBox(g); b != (geo.Box{MinLon: 0, MinLat: 0, MaxLon: 2, MaxLat: 2}) {
		t.Fatalf("box = %+v", b)
	}
}
