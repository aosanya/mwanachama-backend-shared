package dispatch_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

const openRead = `"open":{"method":"GET","path":"/e/{slug}","call":"Open","action":"t.entry.open","args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}`

func TestAWriteCannotShareTheActionAnAnonymousReadIsOpenedBy(t *testing.T) {
	raw := `{"operations":{` + openRead + `,
	 "wipe":{"method":"DELETE","path":"/e/{slug}","call":"Retire","action":"t.entry.open","args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}}}`

	_, err := dispatch.Parse([]byte(raw))
	if err == nil {
		t.Fatal("a DELETE claiming the same action as an open GET was accepted, so naming that action in an anonymous allowlist would publish the DELETE with no session")
	}
	if !strings.Contains(err.Error(), `both claim the action "t.entry.open"`) {
		t.Fatalf("refused for another reason: %v", err)
	}
}

func TestAnOperationWithNoActionIsRefusedAtLoad(t *testing.T) {
	raw := `{"operations":{` + openRead + `,
	 "wipe":{"method":"DELETE","path":"/e/{slug}","call":"Retire","args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}}}`

	_, err := dispatch.Parse([]byte(raw))
	if err == nil {
		t.Fatal("an operation declaring no action was accepted, leaving a route no gate or grant can name")
	}
	if !strings.Contains(err.Error(), `"wipe" declares no action`) {
		t.Fatalf("refused for another reason: %v", err)
	}
}

func TestAnOpenedActionPublishesExactlyOneRoute(t *testing.T) {
	raw := `{"operations":{` + openRead + `,
	 "wipe":{"method":"DELETE","path":"/e/{slug}","call":"Retire","action":"t.entry.delete","args":[{"from":"path","as":"slug"}],"returns":[{"body":true}]}}}`

	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("precondition: a well-formed spec must load: %v", err)
	}
	var routes []dispatch.Route
	for _, op := range s.Operations {
		routes = append(routes, dispatch.Route{Method: op.Method, Path: op.Path, Action: op.Action})
	}
	split := dispatch.Anonymous(routes, "t.entry.open")
	if len(split.Anonymous) != 1 || split.Anonymous[0].Method != "GET" {
		t.Fatalf("anonymous = %+v, want only the GET", split.Anonymous)
	}
}
