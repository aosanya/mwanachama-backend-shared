package dispatch_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type keyManager struct {
	gotKey    deviceKey
	gotCaller string
	gotDevice string
}

type deviceKey struct {
	KeyID       string `json:"key_id"`
	PublicKey   string `json:"public_key"`
	PublishedBy string `json:"published_by"`
	DeviceID    string `json:"device_id"`
}

func (m *keyManager) Publish(ctx context.Context, k deviceKey, caller, device string) (deviceKey, error) {
	m.gotKey, m.gotCaller, m.gotDevice = k, caller, device
	k.PublishedBy, k.DeviceID = caller, device
	return k, nil
}

const deviceSpec = `{
  "operations": {
    "publish_key": {"method":"POST","path":"/keys","call":"Publish","action":"t.key.publish","status":201,
      "title":"Publish a key",
      "description":"Publishes one of the caller's handset keys.",
      "args":[{"from":"body","whole":true},
              {"from":"caller","as":"caller_id","description":"The owner, from the session."},
              {"from":"device","as":"device_id","description":"The handset, from the session."}],
      "returns":[{"body":true}]}
  }
}`

func buildDevice(t *testing.T) (http.Handler, *keyManager) {
	t.Helper()
	s, err := dispatch.Parse([]byte(deviceSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &keyManager{}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{
		Manager: m,
		Caller:  func(context.Context) string { return "actor-1" },
		Device:  func(context.Context) string { return "handset-9" },
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux, m
}

func TestTheDeviceComesFromTheSession(t *testing.T) {
	h, m := buildDevice(t)

	rec := do(t, h, http.MethodPost, "/keys", `{"key_id":"k1","public_key":"pk"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.gotCaller != "actor-1" {
		t.Errorf("caller = %q, want actor-1", m.gotCaller)
	}
	if m.gotDevice != "handset-9" {
		t.Errorf("device = %q, want handset-9", m.gotDevice)
	}
}

// The handset is a session fact, so a body claiming one is overruled by it
// for the same reason a body claiming a caller is.
func TestADeviceInTheBodyDoesNotOverruleTheSession(t *testing.T) {
	h, m := buildDevice(t)

	rec := do(t, h, http.MethodPost, "/keys",
		`{"key_id":"k1","public_key":"pk","published_by":"somebody-else","device_id":"not-my-handset"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.gotDevice != "handset-9" {
		t.Fatalf("device = %q, want the session's handset-9", m.gotDevice)
	}
	if m.gotCaller != "actor-1" {
		t.Fatalf("caller = %q, want the session's actor-1", m.gotCaller)
	}
}

// A session-sourced argument is never offered as a tool argument of its
// own, and the session's value is what the call uses whatever an agent
// supplies in the body. Hiding a *field* of a whole body is a different
// mechanism — FieldDoc.ReadOnly — and is asserted separately below.
func TestADeviceIsNotAToolArgumentAndTheSessionStillWins(t *testing.T) {
	tools := deviceTools(t, nil)

	raw := string(tools[0].InputSchema)
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		t.Fatalf("schema is not an object: %v", err)
	}
	for _, name := range []string{"caller_id", "device_id"} {
		if _, offered := schema.Properties[name]; offered && name == "caller_id" {
			t.Errorf("the schema offers %q, which no argument should have added: %s", name, raw)
		}
	}

	out, err := tools[0].Invoke(context.Background(),
		json.RawMessage(`{"key_id":"k1","public_key":"pk","device_id":"not-my-handset","published_by":"somebody-else"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	got, ok := out.(deviceKey)
	if !ok {
		t.Fatalf("invoke returned %T", out)
	}
	if got.DeviceID != "handset-9" || got.PublishedBy != "actor-1" {
		t.Fatalf("tool call used %+v, want the session's handset-9 and actor-1", got)
	}
}

// A field the store decides is kept out of the schema by declaring it
// read-only, which is how a module hides the two provenance fields a whole
// body carries.
func TestAReadOnlyFieldLeavesTheToolSchema(t *testing.T) {
	tools := deviceTools(t, map[string]dispatch.FieldDoc{
		"deviceKey.published_by": {ReadOnly: true},
		"deviceKey.device_id":    {ReadOnly: true},
	})

	raw := string(tools[0].InputSchema)
	for _, hidden := range []string{"published_by", "device_id"} {
		if strings.Contains(raw, hidden) {
			t.Errorf("the schema still offers %q after it was declared read-only: %s", hidden, raw)
		}
	}
}

func deviceTools(t *testing.T, fields map[string]dispatch.FieldDoc) []dispatch.Tool {
	t.Helper()
	s, err := dispatch.Parse([]byte(deviceSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{
		Manager: &keyManager{},
		Fields:  fields,
		Caller:  func(context.Context) string { return "actor-1" },
		Device:  func(context.Context) string { return "handset-9" },
	})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	return tools
}

func TestADeviceArgumentCannotBeAnythingElse(t *testing.T) {
	_, err := dispatch.Parse([]byte(`{
	  "operations": {
	    "publish_key": {"method":"POST","path":"/keys","call":"Publish","action":"t.key.publish",
	      "args":[{"from":"device","as":"device_id","repeated":true}]}
	  }
	}`))
	if err == nil || !strings.Contains(err.Error(), "comes from the session") {
		t.Fatalf("err = %v, want a refusal naming the session", err)
	}
}
