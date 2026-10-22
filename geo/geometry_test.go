package geo_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/geo"
)

const square = `{"type":"Polygon","coordinates":[[[0,0],[10,0],[10,10],[0,10],[0,0]],[[4,4],[6,4],[6,6],[4,6],[4,4]]]}`

func TestParseAcceptsTheThreeKinds(t *testing.T) {
	for name, raw := range map[string]string{
		"point":             `{"type":"Point","coordinates":[36.8219,-1.2921]}`,
		"point with height": `{"type":"Point","coordinates":[36.8219,-1.2921,1795]}`,
		"polygon":           square,
		"multipolygon":      `{"type":"MultiPolygon","coordinates":[[[[0,0],[1,0],[1,1],[0,0]]],[[[5,5],[6,5],[6,6],[5,5]]]]}`,
		"with a bbox":       `{"type":"Point","coordinates":[1,2],"bbox":[1,2,1,2]}`,
	} {
		if _, err := geo.Parse([]byte(raw)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestParseRefusesWhatIsNotAPlace(t *testing.T) {
	for name, raw := range map[string]string{
		"not json":          `{"type":`,
		"no type":           `{"coordinates":[1,2]}`,
		"line string":       `{"type":"LineString","coordinates":[[0,0],[1,1]]}`,
		"lowercase type":    `{"type":"point","coordinates":[1,2]}`,
		"no coordinates":    `{"type":"Point"}`,
		"null coordinates":  `{"type":"Point","coordinates":null}`,
		"one number":        `{"type":"Point","coordinates":[1]}`,
		"four numbers":      `{"type":"Point","coordinates":[1,2,3,4]}`,
		"longitude too far": `{"type":"Point","coordinates":[181,0]}`,
		"latitude too far":  `{"type":"Point","coordinates":[0,-91]}`,
		"text coordinate":   `{"type":"Point","coordinates":["1","2"]}`,
		"open ring":         `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`,
		"short ring":        `{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`,
		"no rings":          `{"type":"Polygon","coordinates":[]}`,
		"no polygons":       `{"type":"MultiPolygon","coordinates":[]}`,
		"trailing data":     `{"type":"Point","coordinates":[1,2]} {}`,
		"a bare array":      `[1,2]`,
	} {
		_, err := geo.Parse([]byte(raw))
		if !errors.Is(err, geo.ErrInvalidGeometry) {
			t.Errorf("%s: want ErrInvalidGeometry, got %v", name, err)
		}
	}
}

func TestAGeometryRoundTripsThroughJSON(t *testing.T) {
	g, err := geo.Parse([]byte(square))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var back geo.Geometry
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Kind != geo.KindPolygon || len(back.Polygon) != 2 || back.Polygon[1][2] != (geo.Position{6, 6}) {
		t.Fatalf("round trip lost the shape: %+v", back)
	}
}

func TestUnmarshalRefusesAnInvalidGeometry(t *testing.T) {
	var g geo.Geometry
	if err := json.Unmarshal([]byte(`{"type":"Point","coordinates":[200,0]}`), &g); !errors.Is(err, geo.ErrInvalidGeometry) {
		t.Fatalf("want ErrInvalidGeometry, got %v", err)
	}
}

func TestMarshalRefusesAnUnknownKind(t *testing.T) {
	if _, err := json.Marshal(geo.Geometry{Kind: "Circle"}); err == nil {
		t.Fatal("an unknown kind marshalled")
	}
	if err := (geo.Geometry{}).Validate(); err == nil {
		t.Fatal("the zero geometry validated")
	}
	if err := geo.NewPoint(36.8, -1.3).Validate(); err != nil {
		t.Fatalf("a built point did not validate: %v", err)
	}
}
