# The dispatcher

`dispatch/` turns a declared route table into `[]httpwire.Route`. It is the
runtime half of the org-wide spec-driven shape designed in
[architecture-spec-driven-modules.md](../../../developer/documentation/2.%20design/architecture-spec-driven-modules.md)
(the website's W14); the store half lives in each module's own `spec` package.

It is plumbing. Every rule about who may see what stays in the manager, and a
spec may only narrow the surface, never widen it.

## Why it is here and not in the module

`httpwire`'s own package doc names the duplication this exists to remove: the
same route-table shape "copied near-verbatim across actor/assetmanager/comm/
forms/git", and catalog made a seventh copy. A dispatcher that lived in one
module would be the eighth.

It takes the manager as `any` and binds by reflection, so it depends on no
module. That is also what lets a process mount *a spec* rather than *a named
module* — the property the owner asked for of `mwanachama-wakala-api` on
2026-09-23.

## Deny by default, twice

**An operation absent from the spec is not routed.** The dispatcher never
walks the manager looking for methods to publish. Adding a route is an edit
to a file whose whole purpose is to be read. `TestAnUndeclaredMethodIsNotRouted`
pins this.

**An action nobody named is gated.** `Anonymous(routes, actions...)` splits a
table into what may be served without a caller and everything else. The
allowlist is of what is *public*, never of what is protected, so the failure
direction is safe: an operation added upstream arrives gated and shows up as a
401, rather than as 634 pending entries on the public internet. Four tests pin
it, including the case of a route carrying no action at all.

This matters because it is invisible to a route-table test. `catalog`'s
`routes_test.go` mounts every route into one gateless mux, so a table that
cannot be mounted safely passes it green. Raised by the session wiring
wakala-api, 2026-09-23, and it is a gap in W14's stated acceptance criterion
rather than only in an implementation.

`Unmatched` exists because the other half of that failure is silent: misspell
an action in an allowlist and the endpoint you meant to publish simply stays
gated. A mount should fail its own startup on a non-empty `Unmatched`.

## Binding

`args` is positional and matches the method signature after `ctx`.

| Form | Means |
| ---- | ----- |
| `{from: path\|query\|body, as: x}` | one named value |
| `{from: query, as: x, repeated: true}` | binds a variadic, one call argument per occurrence |
| `{from: body\|query, whole: true}` | the entire request becomes one parameter |
| `{from: path, as: x, into: Field}` | overwrites a field on the whole parameter |

`whole` exists because managers take domain structs — `UpsertEntry(ctx, Entry)`,
`ListEntries(ctx, ListFilter)` — not loose scalars. A whole body decodes with
`DisallowUnknownFields`, so a misspelled field is a 400 rather than a silently
dropped value. A whole query assembles a struct from query parameters by
`query:"..."` tag, falling back to the lowercased field name, and `query:"-"`
makes a field unbindable.

**`into` is a security rule, not a convenience.** The hand-written handler it
replaces ended `in.Slug = r.PathValue("slug")`: the address outranks the body,
so a caller cannot `PUT /entries/a` with a body claiming slug `b` and write to
`b`. Overwrites therefore apply *after* the body is decoded, and only `path`
may overwrite — a spec saying `from: query` or `from: body` with `into` is
refused at load. Pinned by `TestTheAddressOutranksAContradictingBody`.

## What the spec is refused for

All of these fail at load rather than at the first request, because a spec is
read once and a request arrives forever:

- an operation with no `action` — an ungated operation is the one nobody
  notices, so the id a role is granted is not optional
- two operations claiming one action, or answering one address
- a `once: true` value on a read, or on more than one operation
- a path parameter no argument binds, or an argument from a path segment that
  does not exist
- an operation rendering nothing, or mixing a whole-body return with named ones
- more than one whole request, or an overwrite with no whole request to land on

And at `Dispatch` rather than at load, because they need the manager: a method
the manager does not have, a method that does not take a context first or
return an error last, an arity mismatch, and a spec naming an error with no
sentinel supplied behind it.

## Rendering

A single return with `body: true` is the whole response body; two or more are
an object keyed by each `as`. A method returning only an error renders no body.

Rendering goes through the value's own `json` tags, so a field the type hides
stays hidden however the dispatcher renders it — which is what keeps a stored
key hash out of a response. `once: true` marks a value the store will never
produce again; it is declared because `IssueShareLink` returns
`(ShareLink, string, error)` and no convention can infer that the second value
is a secret.

## The same spec as MCP tools

`Tools(spec, deps)` returns one `Tool` per operation from the same file the
routes come from: a name, prose, a JSON Schema, and an `Invoke` that takes the
call's arguments as raw JSON and returns the manager's own value. It exists
because the alternative was a hand-written `mcp` package per module, which is
the duplication this package was created to remove — and because a module that
declares its objects and its operations cannot hand-write one anyway: a
domain-agnostic module may not carry an agency's words, and
`mwanachama-backend-catalog` carries no Go handler left to hang a tool on.

**It does not depend on the MCP SDK.** A `Tool` carries `InputSchema` as
`json.RawMessage` and `Invoke` as a plain function, so the twenty lines that
turn one into an SDK tool live in the process that already imports the SDK
(`mwanachama-wakala-api`'s `mcp_tools_catalog.go`). A library that only wants
its acts callable does not acquire a protocol dependency to get them.

### One flat object of arguments

HTTP spreads a call across an address, a query string and a body; a tool gets
one JSON object. So every argument becomes one property:

- a named argument is its own property, typed from the manager's parameter
- a `whole` body or query is **spread** — one property per field of the struct,
  by `json` tag for a body and by `query` key for a query, reassembled before
  the call
- an `into` argument replaces the field it overwrites, so `slug` appears once,
  and the address-outranks-the-body rule holds by construction rather than by
  ordering

An argument the operation does not declare is refused rather than ignored,
which is the tool-side equivalent of `DisallowUnknownFields` and the thing that
tells a model it used a word this module does not have.

### Prose is declared, never inferred

An operation must carry a `description`, or `Tools` refuses the whole spec: a
tool a model cannot read is a tool it cannot call correctly, and the moment to
notice is startup. `title` is the human label, `description` the instruction.

Per-argument prose comes from `Deps.Fields`, a map of `Type.field` to a
description, whether a value is required, whether the store owns it, and the
values an enum permits. A module that declares its objects already has all four
— `mwanachama-backend-catalog` builds the map from its blueprint — so the
schema a model reads and the column the write lands in cannot drift apart. An
argument may also name a field directly (`"field": "Entry.visibility"`) to
borrow its prose and its enum, which is how `set_visibility`'s scalar argument
offers the three states without repeating them.

A field the store owns — a primary key, a stamped time, a review state a
different operation sets — is marked read-only and never becomes an argument.
That is what keeps `catalog_entry_upsert` from offering a model a `visibility`
it would be entitled to expect the store to honour, when `UpsertEntry`
overwrites it.

### Errors keep the HTTP surface's answer

A sentinel the spec maps to a status reaches the caller with its own text; one
it does not map is `internal error`, exactly as the route table renders a 500.
The two surfaces cannot disagree about what a caller is told, because they read
one table.
