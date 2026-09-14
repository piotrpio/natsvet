## Why

The config rules cover what users *declare*; the remaining Tier 1 pitfalls are in what they *call*: a `Request` timeout written as `5` (five nanoseconds), a publish to `orders.*` that reaches nobody, `FlushWithContext(context.Background())` that returns `ErrNoDeadlineContext` every time, `NextMsg` on a callback subscription that returns `ErrSyncSubRequired` every time, `Header.Set` on a `nats.Msg` literal that panics on a nil map, and `Close()` right after `Drain()` that aborts the drain it was meant to wait for. Each is deterministic — a guaranteed error, panic or silent misdelivery — and each is decidable from the call site plus, at most, the single definition of the receiver in the same function. This change completes Tier 1 so `corpus-gate-v0.1` can tag a release.

## What Changes

- Rule `duration` (default-on): an untyped integer constant between 1 and 999999 passed where nats.go, `jetstream` or `micro` declares a `time.Duration` parameter or struct field is almost certainly a missing unit. Discovered from the callee's signature and the struct's field types, not from a list, so every present and future Duration-typed API is covered.
- Rule `subject` (default-on): a constant subject argument or `Subject`/`Reply` field that nats.go rejects (`validateSubject`: empty, whitespace) or the server rejects for subscriptions (`IsValidSubject`: empty token, `>` not last), and a publish-side subject containing a wildcard token, which is delivered as a literal and matches nothing the author intended.
- Rule `ctxdeadline` (default-on): `context.Background()` or `context.TODO()` passed directly to `FlushWithContext` (deterministic `ErrNoDeadlineContext`) or to `RequestWithContext`/`RequestMsgWithContext`/`NextMsgWithContext` (blocks forever when a responder exists but never replies).
- Rule `syncsub` (default-on): `NextMsg`/`NextMsgWithContext` on a subscription whose single definition in the function is a callback subscribe (`ErrSyncSubRequired`), a channel subscribe (silently competes with the channel), or a legacy pull subscribe (`ErrTypeSubscription`).
- Rule `nilheader` (default-on): `Header.Set`/`Add` through a variable whose single definition is a `nats.Msg` literal without a `Header` key and whose `Header` is never assigned — a nil-map write panic.
- Rule `drain` (default-on): `x.Close()` as the statement after `x.Drain()` (allowing one intervening error check); `Drain` returns before draining completes, so the `Close` discards it.
- New `internal/natsapi` helper `SingleDefinition` (the one assignment of a local variable in its function, or none), plus a `Duration` type test and a struct-field type lookup.
- Corpus run with all six rules; new findings triaged; README rule table extended.

## Capabilities

### New Capabilities
- `rule-duration`: the `duration` rule — what counts as an untyped constant, the value window, discovery of Duration-typed parameters and fields, negative cases.
- `rule-subject`: the `subject` rule — hooks per package, the two client/server validity checks, the publish-side wildcard check.
- `rule-ctxdeadline`: the `ctxdeadline` rule — the four hooks and their two distinct messages.
- `rule-syncsub`: the `syncsub` rule — the three subscription kinds, single-definition requirement, messages.
- `rule-nilheader`: the `nilheader` rule — the literal shapes, the later-assignment escape, negative cases.
- `rule-drain`: the `drain` rule — adjacency, the tolerated error check, negative cases.

### Modified Capabilities
- `analyzer-framework`: adds the single-definition helper contract (exactly one assignment of a local variable in the enclosing function, no address taken) that `syncsub` and `nilheader` rely on.

## Non-goals

- The `defer nc.Drain()`-in-`main` check from Tier 2 (`docs/design.md` §3.2, open question 2). Not in this change.
- Tracking subjects or contexts through variables: `ctx := context.Background(); nc.FlushWithContext(ctx)` and `subj := "a..b"; nc.Publish(subj, nil)` are not reported. Constants and direct calls only.
- `duration` on user-defined Duration parameters or on `time.Sleep`; only nats.go, `jetstream` and `micro` declarations. `durationcheck` and `staticcheck` cover the generic cases they cover.
- Subjects built by `fmt.Sprintf` or concatenation with non-constants; a concatenation of constants is a constant and is checked.
- Fixes. `duration` does not know the unit; `syncsub` would orphan the callback; `subject` cannot know the intended subject; `nilheader` has two equally good fixes; `drain` needs a handler the user must write.
- `msgloop`, `pubasync`, `handle` (Tier 2, `lifecycle-rules`).

## Rule defaults

- `duration`, `subject`, `ctxdeadline`, `syncsub`, `nilheader`, `drain`: all default-on.

## Shared helpers

Added to `internal/natsapi`: `SingleDefinition(info, body, ident) (ast.Expr, bool)`, `IsDurationType(t types.Type) bool`, `StructField(t types.Type, name string) *types.Var`. Reused: `IsPkg`, `Callee`, `IsMethod`, `IsFunc`, `ConstString`, `ConstInt`, `IsValidSubject`, `SubjectIsLiteral`, `CompositeFields`, `EnclosingFuncBody`, `AssignedFields`.

## Impact

- `Analyzers()` grows from four to ten default-on rules; every `natsvet`/`go vet -vettool` user sees them at once.
- `duration` is the first rule that keys on a *type* (`time.Duration`) rather than a nats.go symbol; it depends on the callee being resolvable, which excludes calls through function values.
- Corpus: `subject` and `duration` are the likeliest to fire on real code (nats.go's `examples/` and `test/` in particular); each finding is triaged before acceptance.
- No new dependencies.
