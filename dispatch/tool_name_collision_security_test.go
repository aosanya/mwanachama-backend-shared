package dispatch_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func TestS63_OpenHole_ExplicitToolNameTakesAnotherOperationsDerivedName(t *testing.T) {
	raw := toolSpecWith(t, "forget", `"t_note_read"`)
	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		return
	}
	if _, ok := s.Operations["read"]; !ok || s.Operations["read"].Tool != "" {
		t.Fatalf("precondition: the read operation must publish its derived name t_note_read")
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{Manager: &toolManager{}, Errors: map[string]error{"ErrMissing": errMissing}, Fields: noteFields})
	if err != nil {
		return
	}
	actions := map[string][]string{}
	for _, tool := range tools {
		actions[tool.Name] = append(actions[tool.Name], tool.Action)
	}
	if got := actions["t_note_read"]; len(got) > 1 {
		t.Fatalf("S63 (P2): forget declares \"tool\": \"t_note_read\", the name read derives from its action; Parse and Tools accepted it and published %d tools named t_note_read for actions %v, so an MCP server keeps one and a caller asking to read a note may run the delete — want the spec refused naming both operations, as two explicit names already are", len(got), got)
	}
}
