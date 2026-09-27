package dispatch_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const invBlueprint = `{
  "module": "inventory",
  "objects": [
    {
      "role": "item",
      "description": "One stocked thing.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "sku", "type": "string", "description": "Stock keeping unit.", "required": true},
        {"name": "qty", "type": "int", "description": "How many on hand."},
        {"name": "created_at", "type": "timestamp", "description": "When it was first stocked."}
      ]
    }
  ]
}`

func invSpecJSON(instance string) string {
	return fmt.Sprintf(`{
  "module": "inventory",
  "domain": "warehouse",
  "instance": %q,
  "objects": [
    {"name": "stock_item", "table": "items", "role": "item", "description": "A shelf item."}
  ]
}`, instance)
}

type Item struct {
	ID        string `json:"id"`
	SKU       string `json:"sku"`
	Qty       int    `json:"qty"`
	CreatedAt string `json:"created_at"`
}

var errItemNotFound = errors.New("item not found")

type invManager struct {
	st *specstore.Store
}

func newInvManager(t *testing.T, db *gorm.DB, s *spec.Spec) *invManager {
	t.Helper()
	st, err := specstore.New(db, s, map[string]any{"item": Item{}})
	if err != nil {
		t.Fatalf("specstore.New: %v", err)
	}
	return &invManager{st: st}
}

func (m *invManager) CreateItem(ctx context.Context, in Item) (Item, error) {
	in.ID = specstore.NewID()
	in.CreatedAt = "2026-09-27T00:00:00Z"
	if err := m.st.Insert(ctx, "item", in); err != nil {
		return Item{}, err
	}
	return in, nil
}

func (m *invManager) GetItem(ctx context.Context, id string) (Item, error) {
	var out Item
	q := m.st.Query(ctx, "item").Where("id = ?", id)
	if err := m.st.Take(q, "item", &out, errItemNotFound); err != nil {
		return Item{}, err
	}
	return out, nil
}

func (m *invManager) ListItems(ctx context.Context) ([]Item, error) {
	q := m.st.Query(ctx, "item")
	return specstore.List[Item](m.st, q, "item")
}

func openInvSpec(t *testing.T, instance string) *spec.Spec {
	t.Helper()
	bp, err := spec.ParseBlueprint([]byte(invBlueprint))
	if err != nil {
		t.Fatalf("parse blueprint: %v", err)
	}
	s, err := bp.Parse([]byte(invSpecJSON(instance)))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return s
}

func openInvDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

func TestFreshFixtureDomain_TwoInstancesOfSameModuleAreIsolated(t *testing.T) {
	db := openInvDB(t)
	sA := openInvSpec(t, "acme")
	sB := openInvSpec(t, "globex")

	if err := spec.Migrate(db, sA); err != nil {
		t.Fatalf("migrate A: %v", err)
	}
	if err := spec.Migrate(db, sB); err != nil {
		t.Fatalf("migrate B: %v", err)
	}

	mgrA := newInvManager(t, db, sA)
	mgrB := newInvManager(t, db, sB)

	if _, err := mgrA.CreateItem(context.Background(), Item{SKU: "A-1", Qty: 5}); err != nil {
		t.Fatalf("create A: %v", err)
	}

	listB, err := mgrB.ListItems(context.Background())
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	t.Logf("instance A table = %s, instance B table = %s, B sees %d rows after A wrote one",
		mgrA.st.Table("item"), mgrB.st.Table("item"), len(listB))
	if len(listB) != 0 {
		t.Errorf("instance B saw %d rows after instance A wrote one; want 0 (tables: A=%s B=%s)",
			len(listB), mgrA.st.Table("item"), mgrB.st.Table("item"))
	}
}

func TestFreshFixtureDomain_MigrateTwiceIsIdempotent(t *testing.T) {
	db := openInvDB(t)
	s := openInvSpec(t, "acme")
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate 1: %v", err)
	}
	mgr := newInvManager(t, db, s)
	if _, err := mgr.CreateItem(context.Background(), Item{SKU: "X-1", Qty: 1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate 2: %v", err)
	}
	items, err := mgr.ListItems(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	t.Logf("after two migrates, %d rows survive", len(items))
	if len(items) != 1 {
		t.Errorf("re-running Migrate changed row count: got %d, want 1", len(items))
	}
}

func TestFreshFixtureDomain_63ByteTruncationCollisionIsRefused(t *testing.T) {
	table1 := strings.Repeat("a", 60)
	table2 := strings.Repeat("a", 59) + "b"
	instance := "zz"
	module := "inventory"
	physical1 := instance + "_" + module + "_" + table1
	physical2 := instance + "_" + module + "_" + table2
	if physical1[:spec.MaxIdentifier] != physical2[:spec.MaxIdentifier] {
		t.Fatalf("probe construction error: the two physical names do not actually agree for the first %d bytes (%q vs %q)",
			spec.MaxIdentifier, physical1[:spec.MaxIdentifier], physical2[:spec.MaxIdentifier])
	}

	raw := fmt.Sprintf(`{
  "module": %q,
  "domain": "warehouse",
  "instance": %q,
  "objects": [
    {"name": "x1", "table": %q, "description": "one.", "fields": [{"name": "id", "type": "string", "description": "key.", "primary": true}]},
    {"name": "x2", "table": %q, "description": "two.", "fields": [{"name": "id", "type": "string", "description": "key.", "primary": true}]}
  ]
}`, module, instance, table1, table2)

	_, err := spec.Parse([]byte(raw))
	t.Logf("63-byte-collision spec parse error: %v", err)
	t.Logf("physical names truncate identically: %q", physical1[:spec.MaxIdentifier])
	if err == nil {
		t.Errorf("two table names (%q vs %q) whose physical names differ only past MaxIdentifier(%d) — both truncating to %q — were accepted without refusal",
			table1, table2, spec.MaxIdentifier, physical1[:spec.MaxIdentifier])
	}
}

func TestFreshFixtureDomain_SpecsThatShouldBeRefusedAreRefused(t *testing.T) {
	cases := map[string]string{
		"required_with_default": `{"module":"inventory","objects":[{"name":"x","table":"xs","description":"d","fields":[
			{"name":"id","type":"string","description":"d","primary":true},
			{"name":"n","type":"string","description":"d","required":true,"default":"z"}
		]}]}`,
		"duplicate_role": `{"module":"inventory","objects":[
			{"name":"x","table":"xs","role":"item","description":"d","fields":[{"name":"id","type":"string","description":"d","primary":true}]},
			{"name":"y","table":"ys","role":"item","description":"d","fields":[{"name":"id","type":"string","description":"d","primary":true}]}
		]}`,
		"field_on_undeclared_role": `{"module":"inventory","domain":"w","instance":"acme","objects":[
			{"name":"z","table":"zs","role":"item","description":"d","fields":[{"name":"extra","type":"string","description":"d","default":"x"}]}
		]}`,
	}
	bp, err := spec.ParseBlueprint([]byte(invBlueprint))
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var perr error
			if name == "field_on_undeclared_role" {
				_, perr = bp.Parse([]byte(raw))
			} else {
				_, perr = spec.Parse([]byte(raw))
			}
			t.Logf("%s: %v", name, perr)
			if perr == nil {
				t.Errorf("%s: spec that should be refused per declared-domains.md was accepted", name)
			}
		})
	}
}

const invOpsJSON = `{
  "base": "/v1/inventory",
  "operations": {
    "create_item": {
      "method": "POST", "path": "/items", "call": "CreateItem", "action": "inventory.item.create",
      "description": "Create an item.",
      "args": [{"from": "body", "whole": true}],
      "returns": [{"body": true}]
    },
    "get_item": {
      "method": "GET", "path": "/items/{id}", "call": "GetItem", "action": "inventory.item.get",
      "description": "Get an item.",
      "args": [{"from": "path", "as": "id"}],
      "returns": [{"body": true}]
    },
    "list_items": {
      "method": "GET", "path": "/items", "call": "ListItems", "action": "inventory.item.list",
      "description": "List items.",
      "args": [],
      "returns": [{"body": true}]
    }
  }
}`

func TestFreshFixtureDomain_DispatchRouteRoundTripsThroughSpecstore(t *testing.T) {
	db := openInvDB(t)
	s := openInvSpec(t, "acme")
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mgr := newInvManager(t, db, s)

	ds, err := dispatch.Parse([]byte(invOpsJSON))
	if err != nil {
		t.Fatalf("parse ops: %v", err)
	}

	var authCalls []string
	routes, err := dispatch.Dispatch(ds, dispatch.Deps{
		Manager: mgr,
		Authorize: func(ctx context.Context, action string) error {
			authCalls = append(authCalls, action)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.Handle(rt.Pattern(""), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	body := strings.NewReader(`{"sku":"WIDGET-1","qty":7,"id":"attacker-chosen-id","created_at":"1999-01-01T00:00:00Z"}`)
	resp, err := http.Post(srv.URL+"/v1/inventory/items", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var created Item
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Logf("POST /items status=%d created=%+v", resp.StatusCode, created)

	direct, err := mgr.GetItem(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("specstore read-back for id %q failed: %v", created.ID, err)
	}
	t.Logf("specstore read-back: %+v", direct)
	if direct.SKU != "WIDGET-1" {
		t.Errorf("round trip: wrote sku %q, read back %q", "WIDGET-1", direct.SKU)
	}

	if created.ID == "attacker-chosen-id" {
		t.Errorf("SERVER-OWNED FIELD OVERRIDE: caller-supplied id %q reached storage untouched", created.ID)
	}
	if created.CreatedAt == "1999-01-01T00:00:00Z" {
		t.Errorf("SERVER-OWNED FIELD OVERRIDE: caller-supplied created_at reached storage untouched")
	}

	getResp, err := http.Get(srv.URL + "/v1/inventory/items/" + created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer getResp.Body.Close()
	t.Logf("GET /items/{id} status=%d", getResp.StatusCode)

	listResp, err := http.Get(srv.URL + "/v1/inventory/items")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer listResp.Body.Close()
	var listed []Item
	json.NewDecoder(listResp.Body).Decode(&listed)
	t.Logf("GET /items -> %d items", len(listed))

	t.Logf("authorize called for actions: %v (want all 3: create/get/list)", authCalls)
	want := map[string]bool{"inventory.item.create": false, "inventory.item.get": false, "inventory.item.list": false}
	for _, a := range authCalls {
		want[a] = true
	}
	for action, called := range want {
		if !called {
			t.Errorf("AUTHORIZE GAP: action %q never reached the authorizer despite being dispatched", action)
		}
	}
}
