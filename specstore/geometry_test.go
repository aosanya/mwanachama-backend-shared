package specstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/geo"
	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const rolePlace = "place"

const placeSpec = `{
  "module": "atlas",
  "domain": "town",
  "instance": "twn1",
  "objects": [
    {
      "name": "place",
      "table": "places",
      "role": "place",
      "description": "One named place.",
      "fields": [
        {"name": "id", "type": "string", "description": "Which place.", "primary": true},
        {"name": "shape", "type": "geometry", "description": "Where it is."},
        {"name": "outline", "type": "geometry", "description": "Its boundary, when it has one.", "nullable": true},
        {"name": "drawn", "type": "geometry", "description": "The shape as a client sent it."}
      ]
    }
  ]
}`

type Place struct {
	ID      string        `json:"id"`
	Shape   geo.Geometry  `json:"shape"`
	Outline *geo.Geometry `json:"outline"`
	Drawn   string        `json:"drawn"`
}

func newPlaceStore(t *testing.T) (*specstore.Store, *spec.Spec) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s, err := spec.Parse([]byte(placeSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := specstore.New(db, s, map[string]any{rolePlace: Place{}})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return st, s
}

func TestAGeometryRoundTrips(t *testing.T) {
	st, _ := newPlaceStore(t)
	ctx := context.Background()
	outline := geo.NewPolygon(geo.Ring{{0, 0}, {1, 0}, {1, 1}, {0, 0}})
	in := Place{
		ID:      "p1",
		Shape:   geo.NewPoint(36.8219, -1.2921),
		Outline: &outline,
		Drawn:   `{"type":"MultiPolygon","coordinates":[[[[0,0],[2,0],[2,2],[0,0]]]]}`,
	}
	if err := st.Insert(ctx, rolePlace, in); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var out Place
	if err := st.Take(st.Query(ctx, rolePlace).Where("id = ?", "p1"), rolePlace, &out, errors.New("missing")); err != nil {
		t.Fatalf("take: %v", err)
	}
	if out.Shape.Kind != geo.KindPoint || out.Shape.Point != (geo.Position{36.8219, -1.2921}) {
		t.Fatalf("shape read back as %+v", out.Shape)
	}
	if out.Outline == nil || out.Outline.Kind != geo.KindPolygon || !geo.Contains(*out.Outline, geo.Position{0.75, 0.25}) {
		t.Fatalf("outline read back as %+v", out.Outline)
	}
	if g, err := geo.Parse([]byte(out.Drawn)); err != nil || g.Kind != geo.KindMultiPolygon {
		t.Fatalf("drawn read back as %q (%v)", out.Drawn, err)
	}
}

func TestAnAbsentGeometryIsStoredAsNull(t *testing.T) {
	st, s := newPlaceStore(t)
	ctx := context.Background()
	if err := st.Insert(ctx, rolePlace, Place{ID: "p2"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var nulls int64
	if err := st.Query(ctx, rolePlace).Where("shape IS NULL AND outline IS NULL AND drawn IS NULL").Count(&nulls).Error; err != nil {
		t.Fatal(err)
	}
	if nulls != 1 {
		t.Fatalf("an absent place was not stored as NULL in %s", s.TableFor(st.Object(rolePlace)))
	}
}

func TestAnInvalidGeometryIsRefusedOnWrite(t *testing.T) {
	st, _ := newPlaceStore(t)
	ctx := context.Background()
	for name, p := range map[string]Place{
		"one-point line": {ID: "a", Drawn: `{"type":"LineString","coordinates":[[0,0]]}`},
		"open ring":      {ID: "b", Drawn: `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`},
		"off the planet": {ID: "c", Drawn: `{"type":"Point","coordinates":[0,95]}`},
		"not json":       {ID: "d", Drawn: `here`},
		"built badly":    {ID: "e", Shape: geo.Geometry{Kind: "Circle"}},
	} {
		if err := st.Insert(ctx, rolePlace, p); err == nil {
			t.Errorf("%s: written", name)
		}
	}
	var n int64
	if err := st.Query(ctx, rolePlace).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("refused writes left %d rows (%v)", n, err)
	}
}

func TestAGeometryColumnIsADocumentColumn(t *testing.T) {
	_, s := newPlaceStore(t)
	for dialect, want := range map[string]string{"postgres": "shape jsonb", "sqlite": "shape text"} {
		ddl := strings.Join(s.DDL(dialect), "\n")
		if !strings.Contains(ddl, want+",") && !strings.Contains(ddl, want+"\n") && !strings.Contains(ddl, want+")") {
			t.Errorf("%s DDL lacks %q:\n%s", dialect, want, ddl)
		}
		if strings.Contains(ddl, want+" default") {
			t.Errorf("%s DDL gives a place a default:\n%s", dialect, ddl)
		}
	}
}
