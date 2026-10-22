package geo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const (
	KindPoint        = "Point"
	KindPolygon      = "Polygon"
	KindMultiPolygon = "MultiPolygon"
)

var ErrInvalidGeometry = errors.New("geo: not a valid geometry")

type Position [2]float64

func (p Position) Lon() float64 { return p[0] }

func (p Position) Lat() float64 { return p[1] }

type Ring []Position

type Polygon []Ring

type MultiPolygon []Polygon

type Geometry struct {
	Kind         string
	Point        Position
	Polygon      Polygon
	MultiPolygon MultiPolygon
}

func NewPoint(lon, lat float64) Geometry {
	return Geometry{Kind: KindPoint, Point: Position{lon, lat}}
}

func NewPolygon(rings ...Ring) Geometry {
	return Geometry{Kind: KindPolygon, Polygon: Polygon(rings)}
}

func NewMultiPolygon(polygons ...Polygon) Geometry {
	return Geometry{Kind: KindMultiPolygon, MultiPolygon: MultiPolygon(polygons)}
}

type wire struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func Parse(raw []byte) (Geometry, error) {
	var w wire
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&w); err != nil {
		return Geometry{}, invalid("%v", err)
	}
	if dec.More() {
		return Geometry{}, invalid("trailing data after the geometry")
	}
	if len(w.Coordinates) == 0 || string(w.Coordinates) == "null" {
		return Geometry{}, invalid("%q carries no coordinates", w.Type)
	}
	g := Geometry{Kind: w.Type}
	var err error
	switch w.Type {
	case KindPoint:
		g.Point, err = decodePosition(w.Coordinates)
	case KindPolygon:
		var rings []json.RawMessage
		if err = json.Unmarshal(w.Coordinates, &rings); err != nil {
			return Geometry{}, invalid("polygon coordinates: %v", err)
		}
		g.Polygon, err = decodePolygon(rings)
	case KindMultiPolygon:
		var polygons [][]json.RawMessage
		if err = json.Unmarshal(w.Coordinates, &polygons); err != nil {
			return Geometry{}, invalid("multipolygon coordinates: %v", err)
		}
		if len(polygons) == 0 {
			return Geometry{}, invalid("a multipolygon needs at least one polygon")
		}
		for _, rings := range polygons {
			p, perr := decodePolygon(rings)
			if perr != nil {
				return Geometry{}, perr
			}
			g.MultiPolygon = append(g.MultiPolygon, p)
		}
	default:
		return Geometry{}, invalid("type %q is not one of Point, Polygon, MultiPolygon", w.Type)
	}
	if err != nil {
		return Geometry{}, err
	}
	return g, nil
}

func Validate(raw []byte) error {
	_, err := Parse(raw)
	return err
}

func (g Geometry) Validate() error {
	raw, err := g.MarshalJSON()
	if err != nil {
		return err
	}
	return Validate(raw)
}

func (g Geometry) MarshalJSON() ([]byte, error) {
	var coordinates any
	switch g.Kind {
	case KindPoint:
		coordinates = g.Point
	case KindPolygon:
		coordinates = g.Polygon
	case KindMultiPolygon:
		coordinates = g.MultiPolygon
	default:
		return nil, invalid("type %q is not one of Point, Polygon, MultiPolygon", g.Kind)
	}
	return json.Marshal(struct {
		Type        string `json:"type"`
		Coordinates any    `json:"coordinates"`
	}{g.Kind, coordinates})
}

func (g *Geometry) UnmarshalJSON(raw []byte) error {
	parsed, err := Parse(raw)
	if err != nil {
		return err
	}
	*g = parsed
	return nil
}

func decodePolygon(rings []json.RawMessage) (Polygon, error) {
	if len(rings) == 0 {
		return nil, invalid("a polygon needs at least one ring")
	}
	out := make(Polygon, 0, len(rings))
	for i, rawRing := range rings {
		var positions []json.RawMessage
		if err := json.Unmarshal(rawRing, &positions); err != nil {
			return nil, invalid("ring %d: %v", i, err)
		}
		ring, err := decodeRing(positions)
		if err != nil {
			return nil, invalid("ring %d: %v", i, err)
		}
		out = append(out, ring)
	}
	return out, nil
}

func decodeRing(positions []json.RawMessage) (Ring, error) {
	if len(positions) < 4 {
		return nil, fmt.Errorf("a ring needs at least four positions, has %d", len(positions))
	}
	ring := make(Ring, 0, len(positions))
	for _, raw := range positions {
		p, err := decodePosition(raw)
		if err != nil {
			return nil, err
		}
		ring = append(ring, p)
	}
	if ring[0] != ring[len(ring)-1] {
		return nil, fmt.Errorf("a ring must end where it starts")
	}
	return ring, nil
}

func decodePosition(raw json.RawMessage) (Position, error) {
	var numbers []float64
	if err := json.Unmarshal(raw, &numbers); err != nil {
		return Position{}, invalid("position %s: %v", raw, err)
	}
	if len(numbers) < 2 || len(numbers) > 3 {
		return Position{}, invalid("position %s needs a longitude and a latitude", raw)
	}
	p := Position{numbers[0], numbers[1]}
	if math.IsNaN(p.Lon()) || p.Lon() < -180 || p.Lon() > 180 {
		return Position{}, invalid("longitude %v is outside -180..180", p.Lon())
	}
	if math.IsNaN(p.Lat()) || p.Lat() < -90 || p.Lat() > 90 {
		return Position{}, invalid("latitude %v is outside -90..90", p.Lat())
	}
	return p, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidGeometry, fmt.Sprintf(format, args...))
}
