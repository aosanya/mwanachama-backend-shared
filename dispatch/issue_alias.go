package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	issueKey   = "taskID"
	issueAlias = "id"
)

func takesIssueAlias(known map[string]bool) bool {
	return known[issueKey] && !known[issueAlias]
}

func foldIssueAlias(tool string, known map[string]bool, in map[string]json.RawMessage) error {
	if !takesIssueAlias(known) {
		return nil
	}
	alias, given := in[issueAlias]
	if !given {
		return nil
	}
	if own, both := in[issueKey]; both && !bytes.Equal(bytes.TrimSpace(own), bytes.TrimSpace(alias)) {
		return fmt.Errorf("%s got %q and %q with different values; they name the same issue, send one", tool, issueKey, issueAlias)
	}
	in[issueKey] = alias
	delete(in, issueAlias)
	return nil
}

func unknownArgument(tool string, key string, known map[string]bool) error {
	if takesIssueAlias(known) {
		return fmt.Errorf("%s takes no argument %q; the issue is %q and %q is accepted for it", tool, key, issueKey, issueAlias)
	}
	return fmt.Errorf("%s takes no argument %q", tool, key)
}
