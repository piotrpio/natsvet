# natsvet — a `go/analysis` linter for nats.go

Status: draft, pitfalls-first scope agreed. Rule list to be honed during implementation.

## 1. Summary

`natsvet` is a static analyzer suite for programs that use `github.com/nats-io/nats.go`
(core, `jetstream`, `micro`). It turns runtime failures and lifecycle mistakes that the
nats.go maintainers see repeatedly in support into compile-time diagnostics, with
automatic fixes where the fix is unambiguous.

It is built on `golang.org/x/tools/go/analysis`, so the same rules run as a standalone
binary, as a `go vet -vettool`, as a `go fix -fixtool`, and — the eventual distribution
channel — as a golangci-lint linter.

### Goals

- Catch mistakes that are deterministic runtime errors or silent misbehavior in nats.go.
- Zero-configuration default: every default-on rule has a false-positive rate low enough
  that a maintainer would accept it upstream in golangci-lint without argument.
- Every rule is small, independently testable, and documented well enough to be the
  golangci-lint description verbatim.
- Autofix only when the rewrite is semantically certain.

### Non-goals (for this phase)

- Migration from the legacy `nats.JetStreamContext` / `nats.KeyValue` / `nats.ObjectStore`
  API to the `jetstream` package. That is a separate, later rule family (opt-in, big
  `SuggestedFix`es). Nothing in this phase should make that harder.
- Anything needing cross-package facts or whole-program analysis.
- Anything needing runtime configuration knowledge (e.g. "`Ack()` called on a consumer
  whose ack policy is `AckNone`").
- Re-implementing what generic linters already do: unchecked errors from `Ack()`
  (`errcheck`), `err == jetstream.ErrX` on possibly-wrapped errors (`errorlint`),
  use of `// Deprecated:` APIs (`staticcheck` SA1019).
- Style opinions. The only opinionated rules are in Tier 3 and are off by default.

## 2. Architecture

### 2.1 Repository and module

- Separate repository and module. Working name `github.com/<owner>/natsvet`; intended
  final home `github.com/nats-io/natsvet` (fallback: a module under
  `github.com/synadia-io/orbit.go`, which already hosts per-directory modules and tooling).
  Start under a personal account; renaming the module path is free until someone depends
  on it, and must happen before any announcement or golangci-lint submission.
- Not inside nats.go: nats.go's production `go.mod` allows no dependencies beyond
  `klauspost/compress`, `nkeys`, `nuid`; the linter does not import nats.go at all
  (analyzers match on package path and identifier through `go/types`); release cadences
  differ; nats.go CI is minutes of integration tests, this is seconds of unit tests.
- Only production dependency: `golang.org/x/tools`. Test-only: nats.go (see §4).
- Go version: whatever the pinned `golang.org/x/tools` requires. `go fix -fixtool` needs
  Go 1.26 on the user's side, not the tool's.

### 2.2 Layout

```
natsvet/
  go.mod
  natsvet.go                 // Analyzers() []*analysis.Analyzer, OptIn() []*analysis.Analyzer
  cmd/natsvet/main.go        // multichecker.Main(append(Analyzers(), OptIn()...)...)
  internal/natsapi/          // shared helpers, see 2.4
  analyzers/
    consumerconfig/          // one package per rule
      consumerconfig.go      //   var Analyzer = &analysis.Analyzer{Name: "consumerconfig", ...}
      consumerconfig_test.go //   analysistest against ../../testdata
    streamconfig/
    kvconfig/
    subject/
    duration/
    ctxdeadline/
    syncsub/
    headerkey/
    drain/
    handle/
    msgloop/
    pubasync/
    connopts/                // opt-in
    microhandler/            // opt-in
  testdata/
    go.mod                   // module natsvet/testdata; requires nats.go (real API, see §4)
    go.sum
    src/... or flat packages // one package per rule, named after the rule
  scripts/corpus.sh          // runs the binary over pinned real-world repos, see §4.3
  Makefile
  README.md
```

### 2.3 Analyzer conventions

- `Name` is the rule id (lowercase, no separators). `Doc` is the golangci-lint
  description: first line is the one-sentence summary, then a paragraph explaining the
  runtime consequence, then a before/after snippet.
- `Requires: []*analysis.Analyzer{inspect.Analyzer}`. No `FactTypes`. No `ResultType`
  unless two rules genuinely share a computation (then put it in `internal/natsapi` as
  its own analyzer).
- `Diagnostic.Category` = rule id. Messages are one sentence, lowercase, no trailing
  period, and state the consequence, not just the pattern:
  `Request timeout 5 is 5ns; use a time.Duration unit`.
- `SuggestedFixes` only when the replacement is certain. A fix must never change
  semantics in a way the user might not intend (so `duration` reports but does not fix:
  the intended unit is unknown).
- Every rule handles both the `jetstream` package types and, where the same field
  exists, the legacy `nats` package types (`nats.ConsumerConfig`, `nats.StreamConfig`,
  `nats.KeyValueConfig`). Same pitfall, same rule; this is not migration work.
- No cross-package facts. Each rule reasons only about the package being analyzed.
- `pass.Module` (module path/version of the analyzed package) is available in recent
  x/tools drivers but is not needed: if an API does not exist in the user's nats.go
  version, their code does not compile and the rule never sees it. Server-side
  validations encoded by rules are long-standing; rules must not reference server or
  client versions.
- Tier 3 rules are registered but disabled: each declares a boolean flag `enable`
  (default false) and returns early from `Run` when it is not set. `multichecker`
  exposes it as `-connopts.enable`. Default-on rules can be disabled the standard way
  (`-drain=false`).

### 2.4 `internal/natsapi` helpers

Shared, tested independently:

- `IsPkg(obj types.Object, pkg Pkg) bool` for `Pkg ∈ {Core, JetStream, Micro}` — matches
  `github.com/nats-io/nats.go`, `.../jetstream`, `.../micro`. Also matches when the
  package is vendored (compare on path suffix after `vendor/`).
- `Callee(pass, call) (*types.Func, ok)` — `typeutil.Callee` wrapper; works for
  interface methods (`jetstream.Consumer` etc. are interfaces).
- `IsMethod(fn *types.Func, pkg Pkg, recv, name string) bool` — recv is the named type
  or interface name (`Conn`, `Subscription`, `Consumer`, `KeyValue`, ...).
- `ConstString(pass, expr) (string, bool)`, `ConstInt(pass, expr) (int64, bool)` —
  via `pass.TypesInfo.Types[expr].Value`; resolves named constants and constant
  expressions, not just literals.
- `CompositeFields(pass, lit *ast.CompositeLit, pkg Pkg, typeName string) map[string]ast.Expr`
  — returns keyed fields when the literal's type is exactly the named struct (including
  pointer/address-of forms and the legacy twin types).
- Subject helpers, ported from nats.go and nats-server (`server/sublist.go`):
  `BadSubject(s) bool` (whitespace or empty token — nats.go `badSubject`),
  `HasWildcard(s) bool` (a token equal to `*` or `>`), `ValidSubscribeSubject(s) bool`
  (`>` only as last token), `SubjectIsSubsetMatch(subject, filter string) bool`
  (used for overlap checks; port `subjectIsSubsetMatch` from nats-server).
- KV name validation: `validBucketRe = ^[a-zA-Z0-9_-]+$`,
  `validKeyRe = ^[-/_=\.a-zA-Z0-9]+$`, `validSearchKeyRe = ^[-/_=\.a-zA-Z0-9*]*[>]?$`
  (copied from nats.go `jetstream/kv.go`), plus the leading/trailing-`.` rule.
- Known header table: every exported string constant in the `nats` and `jetstream`
  packages whose value starts with `Nats-`, mapped to the qualified constant name.
  Generate with a small `go generate` script over the nats.go module in the module
  cache, or maintain by hand; either way the table lives in one file.
- `EnclosingFunc(pass, node) (ast.Node, *ast.FuncType)` — nearest `FuncDecl`/`FuncLit`.
- `SingleDefinition(pass, ident) (ast.Expr, bool)` — if the identifier's object is
  assigned exactly once in its enclosing function (`:=`, `=`, or `var x = `), return the
  RHS expression; otherwise false. Used by `syncsub` and `handle`-style rules.

## 3. Rule catalog

Conventions used below:

- **Hooks**: the API surface the rule inspects.
- **Detect**: how, precisely enough to implement.
- **Message**: template.
- **Fix**: `none` or the rewrite.
- **FP**: known false-positive scenarios and how they are avoided.
- **Tests**: minimum cases in `testdata`.

### 3.1 Tier 1 — deterministic failures made static (default on, v0.1)

#### `consumerconfig`

Every check here mirrors a validation in nats-server `checkConsumerCfg`
(`server/consumer.go`); the server rejects the consumer at create/update time, so the
program fails at runtime with a `JetStreamError`. Fires only when the involved fields
are constant in the same composite literal.

- **Hooks**: composite literals of `jetstream.ConsumerConfig`, `jetstream.OrderedConsumerConfig`
  (subset of fields), `nats.ConsumerConfig`.
- **Detect** (each is its own diagnostic; "pull" = no `DeliverSubject` field or it is `""`):
  1. `FilterSubject` non-empty and `FilterSubjects` non-empty → both set.
  2. `FilterSubjects` slice literal containing a constant `""`.
  3. `FilterSubjects` slice literal with two constant elements where either is a subset
     match of the other (`SubjectIsSubsetMatch` both directions) → overlapping filters.
  4. `MaxAckPending > 0` and `AckPolicy == AckNonePolicy`.
  5. pull and `Heartbeat > 0` → heartbeat is a pull-request option, not a config field.
  6. pull and `RateLimit > 0`.
  7. push (`DeliverSubject` constant non-empty) and `MaxWaiting != 0`.
  8. `BackOff` slice literal with `len > MaxDeliver` when `MaxDeliver` is a constant `> 0`.
     (Server treats `MaxDeliver == 0` as unlimited; skip when unset, 0, or -1.)
  9. `DeliverPolicy` vs start options (server `badStart`/`notSet`):
     `DeliverAll`/`DeliverLast`/`DeliverNew`/`DeliverLastPerSubject` with `OptStartSeq > 0`
     or `OptStartTime` present and not `nil`; `DeliverByStartSequence` without
     `OptStartSeq > 0` or with `OptStartTime`; `DeliverByStartTime` without `OptStartTime`
     or with `OptStartSeq != 0`. When `DeliverPolicy` is absent it is `DeliverAll`.
  10. `DeliverLastPerSubject` with neither `FilterSubject` nor `FilterSubjects`.
  11. `Name` or `Durable` constant containing any of `.`, `*`, `>` (server: "durable name
      can not contain '.', '*', '>'"), or whitespace.
  12. `MaxRequestExpires` constant in `(0, 1ms)`.
- **Message**: `consumer config: <server's wording>`, e.g.
  `consumer config: FilterSubject and FilterSubjects cannot both be set`.
- **Fix**: none. (Which field the user meant is ambiguous.)
- **FP**: none by construction; only constant fields are considered. A field set from a
  variable is treated as unknown, which disables any check involving it.
- **Tests**: one positive and one negative case per check; a literal with all fields
  from variables produces nothing; pointer literal `&jetstream.ConsumerConfig{...}`;
  legacy `nats.ConsumerConfig` twin.

#### `streamconfig`

Mirrors nats-server `checkStreamCfg` (`server/stream.go`). Same constant-only policy.

- **Hooks**: composite literals of `jetstream.StreamConfig`, `nats.StreamConfig`.
- **Detect**:
  1. `Name` empty, or containing any of `.`, `*`, `>`, `\`, `/`, or whitespace.
  2. `Replicas` constant `> 5`.
  3. `MaxAge` constant negative, or in `(0, 100ms)`.
  4. `Duplicates` constant negative, in `(0, 100ms)`, or `> MaxAge` when both constant
     and `MaxAge > 0`.
  5. `Mirror` present (non-nil) and `Subjects` non-empty; `Mirror` present and `Sources`
     non-empty.
  6. `Subjects` slice literal with two equal constant elements, or an element that
     fails `BadSubject`, or a `>` token not in last position.
- **Message**: `stream config: <server's wording>`.
- **Fix**: none.
- **FP**: none by construction.
- **Tests**: as for `consumerconfig`.

#### `kvconfig`

Client-side validations in nats.go `jetstream/kv.go` that return `ErrInvalidBucketName`,
`ErrHistoryTooLarge`, `ErrInvalidKey` at runtime.

- **Hooks**: composite literals of `jetstream.KeyValueConfig`, `nats.KeyValueConfig`,
  `jetstream.ObjectStoreConfig`, `nats.ObjectStoreConfig`; calls to methods on
  `jetstream.KeyValue` / `nats.KeyValue` that take a key: `Get`, `GetRevision`, `Put`,
  `PutString`, `Create`, `Update`, `Delete`, `Purge`, `History`, `Watch`,
  `WatchFiltered` (each element), and `ListKeysFiltered`.
- **Detect**:
  1. `Bucket` constant not matching `validBucketRe`.
  2. `History` constant `> 64` (`jetstream.KeyValueMaxHistory`) or `< 0`.
  3. Key constant failing `validKeyRe` (or `validSearchKeyRe` for `Watch*`/`ListKeysFiltered`),
     or starting/ending with `.`.
- **Message**: `invalid KV key "foo bar": keys may only contain [-/_=.a-zA-Z0-9]`;
  `KV history 100 exceeds the maximum of 64`; `invalid bucket name "my.bucket"`.
- **Fix**: none.
- **FP**: none by construction.
- **Tests**: valid/invalid bucket; history 64 (ok) and 65; keys with space, leading dot,
  wildcard in `Put` (bad) vs wildcard in `Watch` (ok).

#### `subject`

- **Hooks**: constant subject arguments to `nats.Conn` methods `Publish`, `PublishRequest`,
  `Request`, `RequestWithContext`, `Subscribe`, `SubscribeSync`, `QueueSubscribe`,
  `QueueSubscribeSync`, `ChanSubscribe`, `ChanQueueSubscribe`, `QueueSubscribeSyncWithChan`;
  `Subject`/`Reply` fields of `nats.Msg` composite literals; `jetstream.JetStream` /
  `nats.JetStreamContext` methods `Publish`, `PublishAsync`; `micro.Config.Subject`,
  `micro.EndpointConfig.Subject`, and the subject argument of `micro.Service.AddEndpoint`
  / `Group.AddEndpoint`; `StreamConfig.Subjects` elements and `ConsumerConfig.FilterSubject(s)`
  are covered by their config rules, not here.
- **Detect**:
  1. Any constant subject failing `BadSubject` (nats.go returns `ErrBadSubject` at call time).
  2. Publish-type calls (`Publish*`, `Request*`, `Msg.Subject` used in a publish,
     jetstream `Publish*`) whose subject `HasWildcard` — the server rejects it only in
     pedantic mode; otherwise the `*`/`>` are literal tokens and the message reaches no
     one the user intended.
  3. Subscribe-type subjects with `>` not in last position.
- **Message**: `subject "foo..bar" has an empty token`; `publish subject "orders.*"
  contains a wildcard; wildcards only match in subscriptions`.
- **Fix**: none.
- **FP**: none by construction. Subjects built with `fmt.Sprintf` or concatenation with
  non-constants are not constant and are ignored. A concatenation of constants is a
  constant and is checked.
- **Tests**: `"foo..bar"`, `"foo. bar"`, `"foo.>"` in `Publish` (bad) and `Subscribe`
  (ok), `"foo.>.bar"` in `Subscribe` (bad), constant built from `const prefix + ".x"`.

#### `duration`

- **Hooks**: any call argument or composite-literal field whose declared type is
  `time.Duration` and whose declaring function/struct belongs to the `nats`, `jetstream`,
  or `micro` package. This covers `Conn.Request`, `Subscription.NextMsg`,
  `Conn.FlushTimeout`, options like `nats.Timeout`, `nats.ReconnectWait`,
  `nats.PingInterval`, `jetstream.FetchMaxWait`, `jetstream.PullExpiry`,
  `jetstream.PullHeartbeat`, fields `AckWait`, `Heartbeat`, `InactiveThreshold`,
  `MaxRequestExpires`, `MaxAge`, `Duplicates`, `TTL`, elements of `BackOff`, and the
  `nats.Options` struct — without enumerating them.
- **Detect**: the argument expression is an untyped integer constant (a `BasicLit`, or an
  identifier resolving to an untyped constant, or an arithmetic expression of those with
  no selector `time.X` anywhere in its AST) with value `0 < v < 1_000_000` (1ms).
  `0` and negatives are excluded (they commonly mean "default"/"none").
- **Message**: `duration 5 is 5ns; use a time.Duration unit such as 5*time.Second`.
- **Fix**: none — the unit is unknown.
- **FP**: `500*time.Microsecond` contains a `time.` selector and is skipped.
  A constant declared as `const timeout = 5` and passed by name is flagged, which is
  correct.
- **Tests**: `nc.Request("s", nil, 5)`, `AckWait: 30`, `MaxAge: 0` (ok),
  `AckWait: 500 * time.Millisecond` (ok), named untyped constant.

#### `ctxdeadline`

- **Hooks**: `nats.Conn.FlushWithContext`, `nats.Conn.RequestWithContext`,
  `nats.Conn.RequestMsgWithContext`, `nats.Subscription.NextMsgWithContext`.
- **Detect**: the context argument is a direct call to `context.Background()` or
  `context.TODO()`. Do not track variables.
- **Message**:
  `FlushWithContext requires a context with a deadline (returns ErrNoDeadlineContext)` —
  this one is a deterministic error, see nats.go `context.go`;
  `RequestWithContext with context.Background() blocks forever if a responder exists but
  never replies; use context.WithTimeout` — no-responders detection only covers the
  zero-responder case.
- **Fix**: none.
- **FP**: none. Deliberately excludes the `jetstream` package: its API methods wrap a
  deadline-less context with a default timeout (`wrapContextWithoutDeadline`).
- **Tests**: each hook with `Background()`, `TODO()`, and a `ctx` variable (ok).

#### `syncsub`

- **Hooks**: `nats.Subscription.NextMsg`, `NextMsgWithContext`.
- **Detect**: the receiver is an identifier with a `SingleDefinition` in the enclosing
  function whose RHS is a call to `Subscribe`, `QueueSubscribe`, `ChanSubscribe`,
  `ChanQueueSubscribe`, or `QueueSubscribeSyncWithChan` on a `*nats.Conn`. (`SubscribeSync`
  and `QueueSubscribeSync` are the sync constructors.) At runtime this returns
  `ErrSyncSubRequired` on every call.
- **Message**: `NextMsg on a subscription created with Subscribe (callback) always returns
  ErrSyncSubRequired; use SubscribeSync`.
- **Fix**: none. (Rewriting to `SubscribeSync` would orphan the callback.)
- **FP**: identifiers with more than one assignment are skipped.
- **Tests**: callback sub + `NextMsg` (bad), chan sub + `NextMsg` (bad), sync sub (ok),
  sub reassigned twice (ok), sub passed in as a parameter (ok).

#### `headerkey`

nats.go headers are case-preserving and `Header.Get` is an exact map lookup, unlike
`net/http` which canonicalizes (`readMIMEHeader` in nats.go, "preserves the original
case"). `msg.Header.Get("nats-msg-id")` silently returns `""`.

- **Hooks**: methods `Get`, `Set`, `Add`, `Values`, `Del` on `nats.Header` (also reached
  via `jetstream.Msg.Headers()`), and direct map indexing `h["..."]` where `h` is a
  `nats.Header`.
- **Detect**: the key is a constant that equals a known NATS header (the `Nats-` table
  from §2.4) case-insensitively but not exactly.
- **Message**: `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header
  lookups are case-sensitive`.
- **Fix**: replace the literal with the qualified constant (`jetstream.MsgIDHeader`,
  `nats.MsgIdHdr`) when that package is already imported in the file; otherwise replace
  it with the correctly cased string literal. Prefer the constant from the package the
  file already uses; when both are imported prefer `jetstream`.
- **FP**: user-defined headers are not in the table and are never touched.
- **Tests**: `Get("nats-msg-id")` with `jetstream` imported (fix → constant), without
  (fix → literal), `Set("X-My-Header", v)` (ok), map index form; golden files for fixes.

#### `drain`

`Conn.Drain()` returns immediately after starting `drainConnection` in a goroutine; the
docs say to use `ClosedHandler` to learn when it finishes.

- **Hooks**: `nats.Conn.Drain`, `nats.Conn.Close`.
- **Detect** (Tier 1 part): an expression statement `x.Drain()` immediately followed in
  the same block by `x.Close()` on the same identifier (ignoring an intervening
  `if err != nil { return ... }` on Drain's error). `Close` aborts the in-progress drain.
- **Message**: `Close immediately after Drain aborts the drain; wait for the
  ClosedHandler instead`.
- **Fix**: none.
- **FP**: none.
- **Tests**: adjacent (bad), adjacent with error check between (bad), `Drain` then wait on
  a channel then `Close` (ok).

### 3.2 Tier 2 — lifecycle mistakes (default on, v0.2, some judgment)

#### `handle`

- **Hooks**: calls returning `jetstream.ConsumeContext` (`Consumer.Consume`),
  `jetstream.MessagesContext` (`Consumer.Messages`), `jetstream.KeyWatcher`
  (`KeyValue.Watch`, `WatchAll`, `WatchFiltered`), `jetstream.ObjectWatcher`
  (`ObjectStore.Watch`), `micro.Service` (`micro.AddService`). Legacy twins
  (`nats.KeyWatcher`, `nats.ObjectWatcher`) included. `*nats.Subscription` from
  `Conn.Subscribe*` is behind the flag `-handle.subscriptions` (default off): a
  subscription that lives as long as the connection is a common, legitimate pattern.
- **Detect**: the handle result is assigned to `_`.
- **Message**: `ConsumeContext discarded; the consumer can never be stopped or drained`;
  for watchers add `and its server-side consumer lingers until its inactive threshold`.
- **Fix**: none.
- **FP**: legitimate in short-lived `main`s; that is the accepted cost, users can disable.
  Revisit after the corpus run (§4.3) — if the corpus is mostly `_, err := c.Consume(...)`
  in `main`, downgrade this to opt-in.
- **Tests**: each hook with `_` (bad) and with a named variable (ok).

#### `drain` (extension)

- **Detect**: `defer x.Drain()` where the enclosing function is `func main()` in
  `package main`, or a `TestXxx(*testing.T)` / `BenchmarkXxx` function. Process exit
  truncates the drain, so the deferred call protects nothing.
- **Message**: `deferred Drain in main does nothing useful: Drain returns immediately and
  the process exits; wait for the ClosedHandler`.
- **Open question**: this contradicts snippets in public NATS docs. Ship it only after
  deciding that argument is worth having; keep it in `drain` behind a flag until then.

#### `msgloop`

- **Hooks**: `jetstream.MessagesContext.Next`; `jetstream.Consumer.Fetch`, `FetchBytes`,
  `FetchNoWait` results (`jetstream.MessageBatch`).
- **Detect**:
  1. Inside a `for` statement, `msg, err := it.Next()` followed by an `if err != nil`
     whose body contains `continue` and contains no `return`, `break`, `goto`, `panic`,
     `os.Exit`, and no reference to `jetstream.ErrMsgIteratorClosed`. After `Stop()`,
     `Next` returns `ErrMsgIteratorClosed` forever and the loop busy-spins.
  2. A `MessageBatch` value `r` whose `r.Messages()` is ranged over in a function that
     never calls `r.Error()`. Errors such as `ErrNoHeartbeat` are lost.
- **Message**: `loop continues on every Next error; after Stop this spins forever —
  break on ErrMsgIteratorClosed`; `Fetch result ranged without checking Error()`.
- **Fix**: none.
- **FP**: medium. A `continue` guarded by a `select` on a done channel elsewhere in the
  loop is still flagged; acceptable, disabling is one flag.
- **Tests**: spin loop (bad), loop with `errors.Is(err, jetstream.ErrMsgIteratorClosed)`
  branch (ok), fetch without `Error()` (bad), with (ok).

#### `pubasync`

- **Hooks**: `jetstream.JetStream.PublishAsync`, `PublishMsgAsync`.
- **Detect**: the `PubAckFuture` result is assigned to `_`, and the enclosing function
  contains no call to `PublishAsyncComplete` and no reference to
  `WithPublishAsyncErrHandler`. Acks and errors are never observed.
- **Message**: `PublishAsync result discarded and PublishAsyncComplete never awaited;
  publish errors are lost`.
- **Fix**: none.
- **FP**: medium-high; the error handler may be configured in another function. Ship
  default-on only if the corpus run shows it is quiet; otherwise opt-in.
- **Tests**: discard without completion (bad); discard with `<-js.PublishAsyncComplete()`
  (ok); future used with `select` on `Ok()`/`Err()` (ok).

### 3.3 Tier 3 — opinionated advice (opt-in via `-<rule>.enable`)

#### `connopts`

- **Detect**: a `nats.Connect(...)` call whose option arguments do not include a call to
  `nats.MaxReconnects` (default gives up after 60 attempts × 2s and closes the connection
  for good) or `nats.ErrorHandler` (async errors such as slow consumer go to stderr via
  the default handler). Skip calls with a variadic spread `opts...`.
- **Message**: `Connect without MaxReconnects: the client stops reconnecting after ~2
  minutes; consider nats.MaxReconnects(-1)`.
- Two sub-flags: `-connopts.reconnects`, `-connopts.errorhandler`.

#### `microhandler`

- **Detect**: a `func(micro.Request)` literal (passed as `micro.HandlerFunc`, assigned
  to `Handler:` in `micro.Config`/`EndpointConfig`, or passed to `AddEndpoint`) with a
  terminating path that calls none of `Respond`, `RespondJSON`, `Error` on the request
  and does not pass the request to another function. The caller times out.
- **Message**: `handler path never responds; the requester will time out`.
- Heuristic; keep opt-in.

## 4. Testing

### 4.1 Unit tests per rule

`analysistest.Run` / `RunWithSuggestedFixes` against a shared `testdata` directory with
`// want "..."` comments and `.golden` files for fixes.

Use **module mode** so tests compile against the real nats.go API rather than stubs:
`analysistest` switches to module mode when `testdata/go.mod` exists (this mode is
present but documented as provisional in x/tools; verify on the pinned x/tools version).
It runs with `GOPROXY=off`, so the module must already be in the module cache:
the Makefile `test` target and CI run `cd testdata && go mod download` first. If module
mode proves unreliable, fall back to GOPATH-style stubs under `testdata/src/github.com/nats-io/nats.go/...`
containing only the signatures the rules need — accept that stubs can drift from the
real API and add a CI job that compiles the stubs' usages against real nats.go.

Each rule's test package is `testdata/<rule>` (module mode) and is exercised with the
pattern `natsvet/testdata/<rule>` (or `<rule>` in GOPATH mode).

### 4.2 Helper tests

`internal/natsapi` has table-driven tests for subject helpers (port nats-server's own
subset-match test table), KV regexes, and the header table (must contain every `Nats-`
constant exported by the pinned nats.go — a test that fails when nats.go adds one).

### 4.3 Corpus run (false-positive gate)

`scripts/corpus.sh` clones pinned commits of real-world users of nats.go — at minimum
nats.go's own `examples/` and `test/`, `nats-io/natscli`, `nats-io/nack`,
`synadia-io/nex`, `synadia-io/orbit.go` — and runs `natsvet ./...` over each. Output is
compared against `scripts/corpus.expected`; every line there was triaged by hand and is
either a true positive or a documented, accepted false positive. New findings fail the
job. This runs before every tag and decides Tier 2 defaults (§3.2).

### 4.4 CI

`go vet`, `staticcheck`, `go test ./...` (with the testdata download step), `misspell
-locale US`, and the corpus job. `gofmt` check. Run on the two newest Go versions.

## 5. Distribution

1. `go install github.com/<owner>/natsvet/cmd/natsvet@latest`, then `natsvet ./...`,
   `natsvet -fix ./...`, `natsvet -diff ./...` (`multichecker` provides these).
2. `go vet -vettool=$(which natsvet) ./...` — `multichecker.Main` delegates to
   `unitchecker` when `go vet` invokes it, so the same binary works.
3. `go fix -fixtool=$(which natsvet) ./...` on Go 1.26+ applies `SuggestedFixes` through
   the standard toolchain. Same binary.
4. golangci-lint: the real reach. Interim: module plugin (`golangci-lint custom` with
   `.custom-gcl.yml`) for Synadia-internal dogfooding. Goal: upstream as a built-in
   linter; library-specific linters with low-FP rules are accepted there (`testifylint`,
   `ginkgolinter`, `zerologlint`, `sloglint`, `protogetter` are precedents). The rule
   `Doc` strings are the linter documentation. Upstreaming requires the final module
   path, so it waits for the repository move.
5. gopls cannot load third-party analyzers; in-editor diagnostics come from golangci-lint
   integrations (VS Code Go extension lint tool, GoLand, neovim).

## 6. Delivery phases

1. **Skeleton**: module, `cmd/natsvet`, `internal/natsapi` with tests, testdata module,
   CI, README. One rule end-to-end with a fix (`headerkey`) to prove the golden-file and
   `-fix` paths.
2. **Tier 1**: remaining eight rules. Run on nats.go's `examples/` and fix whatever it
   finds there (also a demo beat).
3. **Corpus gate + v0.1 tag**: `scripts/corpus.sh`, triage, README rule table with
   before/after snippets.
4. **Tier 2** behind the corpus gate; decide `handle`/`pubasync` defaults from data.
5. **golangci-lint**: module plugin config in the repo, then the upstream PR once the
   module path is final.
6. Later, separate design: migration rule family (legacy → `jetstream`).

## 7. Code conventions for the implementing repository

- Apache 2.0 license header on every `.go` file, nats-io style.
- Comments only where they carry information the code does not; rule `Doc` strings are
  the exception and should be complete. No version numbers, links, or issue references in
  code comments.
- `for i := range n` over the three-clause form.
- Table-driven tests; no comments that narrate the next statement.
- US English (`misspell -locale US`).
- Commits signed off (`git commit -s`), single-line subjects.

## 8. Open questions

1. Final repository home: `nats-io/natsvet` vs a module in `synadia-io/orbit.go`.
2. Whether to ship the `defer nc.Drain()`-in-`main` check at all (§3.2).
3. Tier 2 defaults (`handle`, `pubasync`) — decided by the corpus run, not now.
4. Name: `natsvet` for the binary; golangci-lint linter names conventionally end in
   `lint`/`check` — may register there as `natslint` or `natscheck` while the module stays
   `natsvet`.
5. Whether `headerkey` should also flag a `Get` literal that differs only in case from a
   `Set` literal elsewhere in the same package (needs a package-wide pre-pass; cheap, but
   deferred until there is evidence it happens).
