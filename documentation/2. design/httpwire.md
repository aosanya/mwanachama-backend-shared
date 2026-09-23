# `httpwire` — JSON wire helpers and the route-table shape

`httpwire/` holds the JSON wire helpers and route-table shape that every
GORM-backed repo's `routes/` package used to rebuild by hand — the same
functions, copied near-verbatim across `actor`, `assetmanager`, `comm`,
`forms`, and `git`. It was extracted as S13; see
[gorm-shared-infrastructure.md](gorm-shared-infrastructure.md) for the audit
that motivated the extraction and for the sibling packages (`gormtest`,
`gormutil`, `vocab`) that came out of the same pass.

## What stays with the caller

The package speaks bytes and addresses only. It never decides whether a
request is allowed. Three things deliberately stay on the consumer side:

- **Handlers.** `httpwire` carries an `http.HandlerFunc` around; it never
  writes one.
- **Error-to-status tables.** Each repo's sentinel errors are its own domain
  vocabulary. `StatusFor(err, table, fallback)` shares only the
  `errors.Is`-lookup-and-fallback mechanics, not the mapping.
- **Auth.** `Bearer` pulls a token out of an `Authorization` header and stops
  there. Validating it, and gating on the result, belongs to the mounting
  process.

## `Route`

A `Route` is one address a `routes/` package answers, relative to wherever
the mounting process prefixes it. `Pattern(prefix)` renders it as an
`http.ServeMux` registration pattern.

```go
type Route struct {
    Method  string
    Path    string
    Handler http.HandlerFunc
    Action  string
}
```

### `Action`

`Action` is the stable id of the operation a route performs, formatted
`<module>.<resource>.<verb>` — three or more lowercase `[a-z][a-z0-9_]*`
segments, as enforced by `dispatch.ActionPattern`.

It exists so a mounting process can gate by action rather than by knowing a
module's own group names. A hand-written `Route` may leave it empty; a route
produced by `dispatch/` always carries one, and that package's spec
validation additionally requires each action to be unique across a module's
operations.

An empty `Action` is never treated as a match. A route without one is gated
rather than allowed, so adding a route cannot silently widen what a mounting
process exposes.

## `ReadJSON` vs `ReadJSONFields`

Both decode a request body with `DisallowUnknownFields`. `ReadJSONFields`
additionally reports which top-level keys the body actually carried, for a
caller that must tell an absent field apart from an explicit `null` — a PATCH
handler, typically.
