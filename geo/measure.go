package geo

import "math"

const EarthRadiusMetres = 6371008.8

const publicScale = 1000

type Box struct {
	MinLon float64 `json:"min_lon"`
	MinLat float64 `json:"min_lat"`
	MaxLon float64 `json:"max_lon"`
	MaxLat float64 `json:"max_lat"`
}

func (b Box) Contains(p Position) bool {
	return p.Lon() >= b.MinLon && p.Lon() <= b.MaxLon && p.Lat() >= b.MinLat && p.Lat() <= b.MaxLat
}

func Distance(a, b Position) float64 {
	lat1, lat2 := radians(a.Lat()), radians(b.Lat())
	dLat := lat2 - lat1
	dLon := radians(b.Lon() - a.Lon())
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * EarthRadiusMetres * math.Asin(math.Min(1, math.Sqrt(h)))
}

func BoundingBox(g Geometry) Box {
	var positions []Position
	switch g.Kind {
	case KindPoint:
		positions = []Position{g.Point}
	case KindLineString:
		positions = g.LineString
	case KindPolygon:
		positions = polygonPositions(g.Polygon)
	case KindMultiPolygon:
		for _, p := range g.MultiPolygon {
			positions = append(positions, polygonPositions(p)...)
		}
	}
	if len(positions) == 0 {
		return Box{}
	}
	b := Box{MinLon: positions[0].Lon(), MinLat: positions[0].Lat(), MaxLon: positions[0].Lon(), MaxLat: positions[0].Lat()}
	for _, p := range positions[1:] {
		b.MinLon = math.Min(b.MinLon, p.Lon())
		b.MinLat = math.Min(b.MinLat, p.Lat())
		b.MaxLon = math.Max(b.MaxLon, p.Lon())
		b.MaxLat = math.Max(b.MaxLat, p.Lat())
	}
	return b
}

func Contains(g Geometry, p Position) bool {
	switch g.Kind {
	case KindPoint:
		return g.Point == p
	case KindLineString:
		return onRing(Ring(g.LineString), p)
	case KindPolygon:
		return polygonContains(g.Polygon, p)
	case KindMultiPolygon:
		for _, poly := range g.MultiPolygon {
			if polygonContains(poly, p) {
				return true
			}
		}
	}
	return false
}

func RoundForPublic(p Position) Position {
	return Position{roundTo(p.Lon()), roundTo(p.Lat())}
}

func polygonContains(poly Polygon, p Position) bool {
	if len(poly) == 0 || !ringContains(poly[0], p) {
		return false
	}
	for _, hole := range poly[1:] {
		if ringContains(hole, p) && !onRing(hole, p) {
			return false
		}
	}
	return true
}

func ringContains(ring Ring, p Position) bool {
	if onRing(ring, p) {
		return true
	}
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a, b := ring[i], ring[j]
		if (a.Lat() > p.Lat()) != (b.Lat() > p.Lat()) {
			crossLon := a.Lon() + (p.Lat()-a.Lat())*(b.Lon()-a.Lon())/(b.Lat()-a.Lat())
			if p.Lon() < crossLon {
				inside = !inside
			}
		}
	}
	return inside
}

func onRing(ring Ring, p Position) bool {
	for i := 1; i < len(ring); i++ {
		if onSegment(ring[i-1], ring[i], p) {
			return true
		}
	}
	return false
}

func onSegment(a, b, p Position) bool {
	cross := (b.Lon()-a.Lon())*(p.Lat()-a.Lat()) - (b.Lat()-a.Lat())*(p.Lon()-a.Lon())
	if math.Abs(cross) > 1e-12 {
		return false
	}
	return p.Lon() >= math.Min(a.Lon(), b.Lon()) && p.Lon() <= math.Max(a.Lon(), b.Lon()) &&
		p.Lat() >= math.Min(a.Lat(), b.Lat()) && p.Lat() <= math.Max(a.Lat(), b.Lat())
}

func polygonPositions(poly Polygon) []Position {
	if len(poly) == 0 {
		return nil
	}
	return poly[0]
}

func radians(deg float64) float64 { return deg * math.Pi / 180 }

func roundTo(v float64) float64 { return math.Round(v*publicScale) / publicScale }
