package dispatch_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func TestS23_PinsKnownDefect_DeclaredToolNameNotCheckedAgainstDerivedNames(t *testing.T) {
	raw := `{"operations":{
	  "open_entry":{"method":"GET","path":"/entries/{slug}","call":"Read",
	    "action":"x.entry.open","description":"Open one entry.",
	    "args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]},
	  "fetch_entry":{"method":"GET","path":"/entries/{slug}/fetch","call":"Read",
	    "action":"x.entry.fetch","tool":"x_entry_open","description":"Fetch one entry.",
	    "args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}
	}}`
	spec, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse() = %v, want a valid spec (S23 pins that this currently parses)", err)
	}

	tools, err := dispatch.Tools(spec, dispatch.Deps{Manager: &toolManager{}})
	if err != nil {
		t.Fatalf("Tools() = %v, want it to currently succeed (S23 pins the bug, not a refusal)", err)
	}

	names := map[string]int{}
	for _, tl := range tools {
		names[tl.Name]++
	}
	if names["x_entry_open"] != 2 {
		t.Fatalf("tool name %q published %d time(s), want 2 — S23's defect (two operations both end up named %q: one derives it from its own action, the other declares it explicitly) no longer reproduces; if this is because it is now refused at Validate(), invert this test to assert the refusal and close S23", "x_entry_open", names["x_entry_open"], "x_entry_open")
	}
}
