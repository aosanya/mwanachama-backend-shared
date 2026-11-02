package dispatch_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

const renamedToolSpec = `{"operations":{"open_entry":{
  "method":"GET","path":"/entries/{slug}","call":"Open","action":"x.entry.open",
  "tool":"catalog_open_entry","description":"Open one entry.",
  "args":[{"from":"path","as":"slug","required":true,"description":"The entry to open."},
          {"from":"query","as":"k","required":true,"description":"A share key."}],
  "returns":[{"body":true}]}}}`

func renamedTool(t *testing.T) dispatch.Tool {
	t.Helper()
	s, err := dispatch.Parse([]byte(renamedToolSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{Manager: &manager{}})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %+v", tools)
	}
	return tools[0]
}

func TestAnUnknownArgumentNamesThePublishedTool(t *testing.T) {
	tool := renamedTool(t)
	_, err := call(t, tool, `{"slug":"one","k":"key","sector":"health"}`)
	if err == nil || !strings.HasPrefix(err.Error(), `catalog_open_entry takes no argument "sector"`) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "x_entry_open") {
		t.Fatalf("err = %v names the action", err)
	}
}

func TestAMissingArgumentNamesThePublishedTool(t *testing.T) {
	tool := renamedTool(t)
	_, err := call(t, tool, `{"slug":"one"}`)
	if err == nil || !strings.HasPrefix(err.Error(), `catalog_open_entry needs "k"`) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "x_entry_open") {
		t.Fatalf("err = %v names the action", err)
	}
}
