# QA

Test coverage for `mwanachama-backend-shared`. The tests live beside the code
they cover; there is no separate test corpus.

```sh
go test ./...        # in-memory SQLite and in-process handlers, no database needed
```

This line used to say the folder was empty at bootstrap, waiting on S8. That
stopped being true long ago: every package with behaviour carries its own
suite, and `go test ./...` was green across all of them on 2026-09-27.

## What is guarded, and where

- **`spec/`** — the declared-domain loader: identifier alphabets, the
  63-byte budget, blueprint merge refusals, idempotent migration
  (`spec_test.go`, `blueprint_test.go`).
- **`specstore/`** — the codec between declared columns and Go fields
  (`store_test.go`).
- **`dispatch/`** — the operations engine every declared module mounts
  through. Its security-relevant properties each have a test:
  - the authorizer runs before arguments are read, on routes and tools
    alike, and its refusal does not name the action (`authorize_test.go`);
  - anything not named in an anonymous allowlist, or carrying no action, is
    gated (`gate_test.go`);
  - a path value overwrites whatever the body claims (`overwrite_test.go`);
  - **a `from: caller` argument is never read from the request**
    (`caller_security_test.go`, added by the 2026-09-27 security sweep): over
    HTTP a same-named query parameter and body key are both ignored and the
    mount's `Deps.Caller` is bound — `""` when the mount supplies none; the
    MCP tool schema does not offer the argument, a tool call that sends it is
    refused as an unknown argument, and a spec marking it `required` is
    refused at load. The design is in
    [dispatcher.md](../2.%20design/dispatcher.md); the first consumer is
    `mwanachama-backend-catalog`'s share-link issuer (its CAT12).

## Red by design

None. No test in this repo is failing on purpose.
