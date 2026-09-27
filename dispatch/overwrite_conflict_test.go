package dispatch_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

const callerThenPathOverwriteSpec = `{
  "operations": {
    "grant": {"method":"PUT","path":"/records/{slug}","call":"Upsert","action":"t.record.grant",
               "args":[
                 {"from":"body","whole":true},
                 {"from":"caller","as":"actor","into":"Name"},
                 {"from":"path","as":"slug","into":"Name"}
               ],
               "returns":[{"body":true}]}
  }
}`

func TestS24_OpenBug_DuplicateOverwriteTargetIsRefusedAtLoad(t *testing.T) {
	if _, err := dispatch.Parse([]byte(callerThenPathOverwriteSpec)); err == nil {
		t.Fatalf("S24: dispatch.Parse accepted an operation where a caller-sourced overwrite "+
			"and a path-sourced overwrite both target the field %q; at runtime the one declared "+
			"last silently wins, so a request-controlled path segment can replace a value that "+
			"was supposed to come only from the caller. Want a validation error naming the "+
			"duplicate target, got none", "Name")
	}
}
