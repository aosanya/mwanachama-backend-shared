package dispatch_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var errBoom = errors.New("the disk caught fire")

type note struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
	State   string `json:"state"`
	Written string `json:"written_at"`
	Hidden  string `json:"-"`
}

type noteFilter struct {
	State string
	Limit int
}

type toolManager struct {
	put    note
	filter noteFilter
	kinds  []string
	forgot string
}

func (m *toolManager) Put(ctx context.Context, n note) (note, error) {
	m.put = n
	return n, nil
}

func (m *toolManager) Read(ctx context.Context, slug string) (note, error) {
	if slug == "missing" {
		return note{}, errMissing
	}
	if slug == "boom" {
		return note{}, errBoom
	}
	return note{Slug: slug, Title: "read back"}, nil
}

func (m *toolManager) Find(ctx context.Context, f noteFilter) ([]note, error) {
	m.filter = f
	return []note{{Slug: "one"}}, nil
}

func (m *toolManager) Issue(ctx context.Context, slug, label string) (note, string, error) {
	return note{Slug: slug}, "raw-" + label, nil
}

func (m *toolManager) Forget(ctx context.Context, slug string) error {
	m.forgot = slug
	return nil
}

func (m *toolManager) Keep(ctx context.Context, slug, state string) (note, error) {
	m.put = note{Slug: slug, State: state}
	return m.put, nil
}

func (m *toolManager) Tally(ctx context.Context, kinds ...string) (int, error) {
	m.kinds = kinds
	return len(kinds), nil
}

const toolSpec = `{
  "operations": {
    "put":    {"method":"PUT","path":"/notes/{slug}","call":"Put","action":"t.note.put",
               "title":"Write note","description":"Create the note or replace the one at this slug.",
               "args":[{"from":"body","whole":true},
                       {"from":"path","as":"slug","into":"Slug","description":"The note to write."}],
               "returns":[{"body":true}]},
    "read":   {"method":"GET","path":"/notes/{slug}","call":"Read","action":"t.note.read",
               "description":"Read one note.",
               "args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]},
    "find":   {"method":"GET","path":"/notes","call":"Find","action":"t.note.find",
               "description":"List the notes matching the filter.",
               "args":[{"from":"query","whole":true}],"returns":[{"body":true}]},
    "issue":  {"method":"POST","path":"/notes/{slug}/links","call":"Issue","action":"t.link.issue",
               "description":"Mint a key for one note.",
               "args":[{"from":"path","as":"slug"},
                       {"from":"body","as":"label","required":true,"description":"What the key is for."}],
               "returns":[{"as":"link"},{"as":"key","once":true}]},
    "forget": {"method":"DELETE","path":"/notes/{slug}","call":"Forget","action":"t.note.forget",
               "description":"Delete one note.",
               "args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]},
    "keep":   {"method":"POST","path":"/notes/{slug}/state","call":"Keep","action":"t.note.keep",
               "description":"Move the note between states.",
               "args":[{"from":"path","as":"slug"},
                       {"from":"body","as":"state","required":true,"field":"note.state"}],
               "returns":[{"body":true}]},
    "tally":  {"method":"GET","path":"/tally","call":"Tally","action":"t.note.tally",
               "description":"Count the notes of each named kind.",
               "args":[{"from":"query","as":"kind","repeated":true}],"returns":[{"body":true}]}
  },
  "errors": {"ErrMissing": 404}
}`

var noteFields = map[string]dispatch.FieldDoc{
	"note.id":         {Description: "Storage key.", ReadOnly: true},
	"note.written_at": {Description: "When it was written.", ReadOnly: true},
	"note.title":      {Description: "The note's heading.", Required: true},
	"note.state":      {Description: "Where it has got to.", Values: []string{"draft", "kept"}},
}

func buildTools(t *testing.T) (map[string]dispatch.Tool, *toolManager) {
	t.Helper()
	s, err := dispatch.Parse([]byte(toolSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &toolManager{}
	tools, err := dispatch.Tools(s, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing},
		Fields:  noteFields,
	})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	byName := map[string]dispatch.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName, m
}

func schemaOf(t *testing.T, tool dispatch.Tool) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(tool.InputSchema, &out); err != nil {
		t.Fatalf("schema of %s: %v", tool.Name, err)
	}
	return out
}

func propsOf(t *testing.T, tool dispatch.Tool) map[string]any {
	t.Helper()
	props, ok := schemaOf(t, tool)["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no properties", tool.Name)
	}
	return props
}

func call(t *testing.T, tool dispatch.Tool, args string) (any, error) {
	t.Helper()
	return tool.Invoke(context.Background(), json.RawMessage(args))
}

func TestToolNamesComeFromTheAction(t *testing.T) {
	tools, _ := buildTools(t)
	for _, want := range []string{"t_note_put", "t_note_read", "t_note_find", "t_link_issue", "t_note_forget", "t_note_tally"} {
		if _, ok := tools[want]; !ok {
			t.Fatalf("no tool named %s, got %v", want, tools)
		}
	}
	if got := tools["t_note_put"].Title; got != "Write note" {
		t.Fatalf("title = %q", got)
	}
	if got := tools["t_note_put"].Description; !strings.Contains(got, "replace the one at this slug") {
		t.Fatalf("description = %q", got)
	}
}

func TestAWholeBodyIsSpreadOverNamedArguments(t *testing.T) {
	tools, _ := buildTools(t)
	props := propsOf(t, tools["t_note_put"])
	for _, want := range []string{"slug", "title", "body", "state"} {
		if _, ok := props[want]; !ok {
			t.Fatalf("property %q is missing from %v", want, props)
		}
	}
	if _, ok := props["Hidden"]; ok {
		t.Fatalf("a field tagged out of JSON reached the schema: %v", props)
	}
}

func TestAReadOnlyFieldIsNotAnArgument(t *testing.T) {
	tools, _ := buildTools(t)
	props := propsOf(t, tools["t_note_put"])
	for _, unwanted := range []string{"id", "written_at"} {
		if _, ok := props[unwanted]; ok {
			t.Fatalf("read-only %q reached the schema: %v", unwanted, props)
		}
	}
}

func TestADeclaredEnumReachesTheSchema(t *testing.T) {
	tools, _ := buildTools(t)
	state, _ := propsOf(t, tools["t_note_put"])["state"].(map[string]any)
	values, _ := state["enum"].([]any)
	if len(values) != 2 || values[0] != "draft" || values[1] != "kept" {
		t.Fatalf("state schema = %v", state)
	}
	if state["description"] != "Where it has got to." {
		t.Fatalf("state carries no description: %v", state)
	}
}

func TestANamedArgumentBorrowsTheFieldItWrites(t *testing.T) {
	tools, _ := buildTools(t)
	state, _ := propsOf(t, tools["t_note_keep"])["state"].(map[string]any)
	values, _ := state["enum"].([]any)
	if len(values) != 2 || values[0] != "draft" {
		t.Fatalf("state schema = %v", state)
	}
	if state["description"] != "Where it has got to." {
		t.Fatalf("state schema = %v", state)
	}
}

func TestTheRequiredSetIsThePathPlusWhatTheFieldsDeclare(t *testing.T) {
	tools, _ := buildTools(t)
	required, _ := schemaOf(t, tools["t_note_put"])["required"].([]any)
	got := map[string]bool{}
	for _, r := range required {
		got[r.(string)] = true
	}
	if !got["slug"] || !got["title"] || len(got) != 2 {
		t.Fatalf("required = %v", required)
	}
}

func TestASchemaTakesNoArgumentItDidNotDeclare(t *testing.T) {
	tools, _ := buildTools(t)
	for name, tool := range tools {
		schema := schemaOf(t, tool)
		if schema["type"] != "object" {
			t.Fatalf("%s: schema type = %v", name, schema["type"])
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("%s: additionalProperties = %v", name, schema["additionalProperties"])
		}
	}
}

func TestThePathArgumentBecomesTheBodyField(t *testing.T) {
	tools, m := buildTools(t)
	if _, err := call(t, tools["t_note_put"], `{"slug":"from-the-path","title":"A note","state":"draft"}`); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if m.put.Slug != "from-the-path" || m.put.Title != "A note" || m.put.State != "draft" {
		t.Fatalf("manager saw %+v", m.put)
	}
}

func TestAWholeQueryStructTakesItsOwnKeys(t *testing.T) {
	tools, m := buildTools(t)
	if _, err := call(t, tools["t_note_find"], `{"state":"kept","limit":3}`); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if m.filter.State != "kept" || m.filter.Limit != 3 {
		t.Fatalf("filter = %+v", m.filter)
	}
}

func TestARepeatedArgumentSpreadsIntoAVariadic(t *testing.T) {
	tools, m := buildTools(t)
	out, err := call(t, tools["t_note_tally"], `{"kind":["draft","kept"]}`)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if len(m.kinds) != 2 || m.kinds[0] != "draft" || m.kinds[1] != "kept" {
		t.Fatalf("kinds = %v", m.kinds)
	}
	if out != 2 {
		t.Fatalf("out = %v", out)
	}
}

func TestOneValueWhereAListWasDeclaredIsTakenAsAListOfOne(t *testing.T) {
	tools, m := buildTools(t)
	out, err := call(t, tools["t_note_tally"], `{"kind":"draft"}`)
	if err != nil {
		t.Fatalf("a single value was refused where a list was declared: %v", err)
	}
	if len(m.kinds) != 1 || m.kinds[0] != "draft" {
		t.Fatalf("kinds = %v", m.kinds)
	}
	if out != 1 {
		t.Fatalf("out = %v", out)
	}
}

func TestAListIsStillAList(t *testing.T) {
	tools, m := buildTools(t)
	if _, err := call(t, tools["t_note_tally"], `{"kind":["draft","kept"]}`); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if len(m.kinds) != 2 {
		t.Fatalf("kinds = %v", m.kinds)
	}
}

func TestNamedReturnsBecomeAToolResultObject(t *testing.T) {
	tools, _ := buildTools(t)
	out, err := call(t, tools["t_link_issue"], `{"slug":"one","label":"for-ann"}`)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	body, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("out = %T", out)
	}
	if body["key"] != "raw-for-ann" {
		t.Fatalf("body = %v", body)
	}
	if got, ok := body["link"].(note); !ok || got.Slug != "one" {
		t.Fatalf("body = %v", body)
	}
}

func TestAMethodReturningOnlyAnErrorRendersNothing(t *testing.T) {
	tools, m := buildTools(t)
	out, err := call(t, tools["t_note_forget"], `{"slug":"one"}`)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %v", out)
	}
	if m.forgot != "one" {
		t.Fatalf("forgot = %q", m.forgot)
	}
}

func TestAnUnknownArgumentIsRefused(t *testing.T) {
	tools, _ := buildTools(t)
	_, err := call(t, tools["t_note_read"], `{"slug":"one","sector":"health"}`)
	if err == nil || !strings.Contains(err.Error(), `takes no argument "sector"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestAMissingRequiredArgumentIsRefused(t *testing.T) {
	tools, _ := buildTools(t)
	_, err := call(t, tools["t_link_issue"], `{"slug":"one"}`)
	if err == nil || !strings.Contains(err.Error(), `needs "label"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestArgumentsThatAreNotAnObjectAreRefused(t *testing.T) {
	tools, _ := buildTools(t)
	_, err := call(t, tools["t_note_read"], `["one"]`)
	if err == nil || !strings.Contains(err.Error(), "not a JSON object") {
		t.Fatalf("err = %v", err)
	}
}

func TestADeclaredSentinelReachesTheCaller(t *testing.T) {
	tools, _ := buildTools(t)
	_, err := call(t, tools["t_note_read"], `{"slug":"missing"}`)
	if !errors.Is(err, errMissing) {
		t.Fatalf("err = %v", err)
	}
}

func TestAnUndeclaredErrorIsOpaque(t *testing.T) {
	tools, _ := buildTools(t)
	_, err := call(t, tools["t_note_read"], `{"slug":"boom"}`)
	if err == nil || err.Error() != "internal error" {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(err, errBoom) {
		t.Fatalf("the caller was told what broke: %v", err)
	}
}

func TestAnOperationWithNoDescriptionIsRefused(t *testing.T) {
	s, err := dispatch.Parse([]byte(`{
      "operations": {
        "read": {"method":"GET","path":"/notes/{slug}","call":"Read","action":"t.note.read",
                 "args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}
      }
    }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = dispatch.Tools(s, dispatch.Deps{Manager: &toolManager{}})
	if err == nil || !strings.Contains(err.Error(), "no description") {
		t.Fatalf("err = %v", err)
	}
}

func TestToolsRefuseAManagerMissingTheMethod(t *testing.T) {
	s, err := dispatch.Parse([]byte(`{
      "operations": {
        "vanish": {"method":"POST","path":"/vanish","call":"Vanish","action":"t.note.vanish",
                   "description":"Not a method this manager has.",
                   "args":[],"returns":[{"body":true}]}
      }
    }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = dispatch.Tools(s, dispatch.Deps{Manager: &toolManager{}})
	if err == nil || !strings.Contains(err.Error(), "does not have") {
		t.Fatalf("err = %v", err)
	}
}

// A tool's name is what callers hardcode, so a module converting a
// hand-written surface may keep the names it published.
func TestDeclaredToolNameIsPublished(t *testing.T) {
	const raw = `{"operations":{"open_entry":{
	  "method":"GET","path":"/entries/{slug}","call":"Open","action":"x.entry.open",
	  "tool":"catalog_open_entry","description":"Open one entry.",
	  "args":[{"from":"path","as":"slug","description":"The entry to open."},
	          {"from":"query","as":"k","description":"A share key."}],
	  "returns":[{"body":true}]}}}`

	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{Manager: &manager{}})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "catalog_open_entry" {
		t.Fatalf("tools = %+v, want the declared name", tools)
	}
}

func TestTwoOperationsCannotPublishOneToolName(t *testing.T) {
	const raw = `{"operations":{
	  "open_entry":{"method":"GET","path":"/entries/{slug}","call":"Open","action":"x.entry.open",
	    "tool":"same","description":"Open one entry.",
	    "args":[{"from":"path","as":"slug","description":"The entry."},{"from":"query","as":"k","description":"A key."}],
	    "returns":[{"body":true}]},
	  "read_entry":{"method":"GET","path":"/entries/{slug}/admin","call":"Open","action":"x.entry.read",
	    "tool":"same","description":"Read one entry.",
	    "args":[{"from":"path","as":"slug","description":"The entry."},{"from":"query","as":"k","description":"A key."}],
	    "returns":[{"body":true}]}}}`

	_, err := dispatch.Parse([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "both publish the tool") {
		t.Fatalf("err = %v, want the collision refused", err)
	}
}
