package dispatch_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const widgetBlueprintJSON = `{
  "module": "widgetworks",
  "objects": [
    {
      "role": "widget",
      "description": "a manufactured thing this module catalogues",
      "fields": [
        {"name": "id", "type": "string", "primary": true, "description": "the widget's key"},
        {"name": "code", "type": "string", "required": true, "unique": true, "matches": "widget_code", "description": "a caller-facing code"},
        {"name": "status", "type": "enum", "values": ["draft", "published"], "default": "draft", "description": "lifecycle state"},
        {"name": "meta", "type": "json", "description": "free-form metadata"}
      ],
      "indexes": [
        {"name": "idx_status", "fields": ["status"], "description": "status lookup"},
        {"name": "idx_meta_tag", "path": {"field": "meta", "path": "specs.tag"}, "description": "tag lookup"}
      ]
    }
  ]
}`

func widgetDomainSpec(instance string) string {
	return fmt.Sprintf(`{
  "module": "widgetworks", "domain": "widgetworks", "instance": %q,
  "objects": [{"name": "widget", "role": "widget", "table": "widgets", "description": "a widget this domain sells", "fields": [], "indexes": []}]
}`, instance)
}

type widgetRow struct {
	ID     string
	Code   string
	Status string
	Meta   string
}

type widgetManager struct {
	st *specstore.Store
}

func newWidgetManager(t *testing.T, db *gorm.DB, s *spec.Spec) *widgetManager {
	t.Helper()
	st, err := specstore.New(db, s, map[string]any{"widget": widgetRow{}})
	if err != nil {
		t.Fatalf("specstore.New: %v", err)
	}
	return &widgetManager{st: st}
}

func (m *widgetManager) Create(ctx context.Context, w widgetRow) (widgetRow, error) {
	w.ID = specstore.NewID()
	if w.Status == "" {
		w.Status = "draft"
	}
	if err := m.st.Insert(ctx, "widget", w); err != nil {
		return widgetRow{}, err
	}
	return w, nil
}

func (m *widgetManager) Get(ctx context.Context, id string) (widgetRow, error) {
	var out widgetRow
	q := m.st.Query(ctx, "widget").Where("id = ?", id)
	if err := m.st.Take(q, "widget", &out, fmt.Errorf("widget not found")); err != nil {
		return widgetRow{}, err
	}
	return out, nil
}

func (m *widgetManager) List(ctx context.Context) ([]widgetRow, error) {
	return specstore.List[widgetRow](m.st, m.st.Query(ctx, "widget"), "widget")
}

const widgetOperations = `{
  "base": "/v1",
  "operations": {
    "create": {"method":"POST","path":"/widgets","call":"Create","action":"widgetworks.widget.create",
      "args":[{"from":"body","whole":true}],"returns":[{"body":true}]},
    "get": {"method":"GET","path":"/widgets/{id}","call":"Get","action":"widgetworks.widget.get",
      "args":[{"from":"path","as":"id"}],"returns":[{"body":true}]},
    "list": {"method":"GET","path":"/widgets","call":"List","action":"widgetworks.widget.list",
      "args":[],"returns":[{"body":true}]}
  }
}`

func openWidgetDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

func buildWidgetServer(t *testing.T, db *gorm.DB, s *spec.Spec) (*httptest.Server, *widgetManager) {
	t.Helper()
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	m := newWidgetManager(t, db, s)

	opSpec, err := dispatch.Parse([]byte(widgetOperations))
	if err != nil {
		t.Fatalf("dispatch.Parse: %v", err)
	}
	routes, err := dispatch.Dispatch(opSpec, dispatch.Deps{Manager: m})
	if err != nil {
		t.Fatalf("dispatch.Dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return httptest.NewServer(mux), m
}

func loadWidgetSpec(t *testing.T, instance string) *spec.Spec {
	t.Helper()
	bp, err := spec.ParseBlueprint([]byte(widgetBlueprintJSON))
	if err != nil {
		t.Fatalf("ParseBlueprint: %v", err)
	}
	s, err := bp.Parse([]byte(widgetDomainSpec(instance)))
	if err != nil {
		t.Fatalf("bp.Parse: %v", err)
	}
	return s
}

// Loophole #9: a write through the real generated HTTP route and a read
// through specstore directly must agree, including a document-path filter
// built with the same spec.DocPathExpr the index itself uses.
func TestFixture_HTTPWriteAgreesWithDirectStoreRead(t *testing.T) {
	db := openWidgetDB(t)
	s := loadWidgetSpec(t, "acme")
	srv, m := buildWidgetServer(t, db, s)
	defer srv.Close()

	body := `{"code":"W-1","meta":"{\"specs\":{\"tag\":\"blue\"}}"}`
	resp, err := http.Post(srv.URL+"/v1/widgets", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	var created widgetRow
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || created.Status != "draft" {
		t.Fatalf("created = %+v, want a minted ID and default status", created)
	}

	getResp, err := http.Get(srv.URL + "/v1/widgets/" + created.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", getResp.StatusCode)
	}

	direct, err := m.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("direct Get: %v", err)
	}
	if direct.Code != "W-1" {
		t.Fatalf("direct.Code = %q, want W-1", direct.Code)
	}

	expr := spec.DocPathExpr("meta", "specs.tag", "sqlite")
	var tag string
	row := db.Raw("select "+expr+" from acme_widgetworks_widgets where id = ?", created.ID).Row()
	if err := row.Scan(&tag); err != nil {
		t.Fatalf("doc path query: %v", err)
	}
	if tag != "blue" {
		t.Fatalf("doc path read %q, want blue; DocPathExpr disagrees with the stored document", tag)
	}
}

// Loophole #8: two instances of the same domain, migrated into one database,
// must land on genuinely separate tables, and a write to one must not be
// visible through the other's manager.
func TestFixture_TwoInstancesAreIsolated(t *testing.T) {
	db := openWidgetDB(t)
	sa := loadWidgetSpec(t, "tenanta")
	sb := loadWidgetSpec(t, "tenantb")

	if err := spec.Migrate(db, sa); err != nil {
		t.Fatalf("migrate a: %v", err)
	}
	if err := spec.Migrate(db, sb); err != nil {
		t.Fatalf("migrate b: %v", err)
	}

	ma := newWidgetManager(t, db, sa)
	mb := newWidgetManager(t, db, sb)

	if _, err := ma.Create(context.Background(), widgetRow{Code: "A-1"}); err != nil {
		t.Fatalf("create a: %v", err)
	}

	listA, err := ma.List(context.Background())
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	listB, err := mb.List(context.Background())
	if err != nil {
		t.Fatalf("list b: %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("tenant a sees %d widgets, want 1", len(listA))
	}
	if len(listB) != 0 {
		t.Fatalf("tenant b sees %d widgets from tenant a's table, want 0 (tenant bleed)", len(listB))
	}
	if sa.TableFor(sa.Objects[0]) == sb.TableFor(sb.Objects[0]) {
		t.Fatalf("both instances produced the same physical table name %q", sa.TableFor(sa.Objects[0]))
	}
}

// Loophole #5: migrating the same spec repeatedly must not grow the table
// set or error, and existing rows must survive.
func TestFixture_MigrateIsIdempotentWithLiveData(t *testing.T) {
	db := openWidgetDB(t)
	s := loadWidgetSpec(t, "acme")
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate 1: %v", err)
	}
	m := newWidgetManager(t, db, s)
	if _, err := m.Create(context.Background(), widgetRow{Code: "W-1"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}

	rows, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d after re-migrating, want 1 (idempotent, no data loss)", len(rows))
	}
}
