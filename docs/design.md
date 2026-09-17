# natsvet — a `go/analysis` linter for nats.go

Status: living design. Work is planned and tracked as OpenSpec changes under `openspec/`;
this document is the rationale they refer back to. As of 2026-09-17 `main` is
release-ready: every Tier 1 and Tier 2 rule is implemented and specified, the corpus (§4.3)
passes with every finding triaged, and `go install ...@latest` works. The first tag follows
the repository move (§8, item 1), which follows internal dogfooding; a tag under the current
personal path would make it sticky for `go install` users. Rule lists in §3 are derived from the
nats.go / nats-server source at the commits named in §3 and are re-derived when a rule is
implemented.

## 1. Summary

`natsvet` is a static analyzer suite for programs that use `github.com/nats-io/nats.go`
(core, `jetstream`, `micro`). It turns runtime failures and lifecycle mistakes that the
nats.go maintainers see repeatedly in support into compile-time diagnostics, with
automatic fixes where the fix is unambiguous.

`github.com/synadia-io/orbit.go` (`natscontext`, `natsext`, `jetstreamext`, `kvcodec`,
`counters`, `pcgroups`, `natssysclient`) is in scope in three ways, in this order:

1. As a corpus target (§4.3) from v0.1: its per-module `test/` directories are dense,
   maintainer-written usage of the `jetstream` API, which the rest of the corpus lacks.
2. As the subject of opt-in "use orbit instead" rules (Tier 3, §3.3), e.g. a hand-rolled
   scatter-gather that `natsext.RequestMany` replaces.
3. As an API to lint in its own right (misuse of orbit functions and configs). No concrete
   rules yet; surveyed after the corpus run. The `natsapi` package matching (§2.4) is
   keyed by import path so orbit modules can be added without touching existing rules.

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
  `SuggestedFix`es). Nothing in this phase should make that harder. The one exception is
  `legacyjs` (§3.4): an opt-in, report-only inventory of legacy API use that costs almost
  nothing on top of `natsapi` and is a prerequisite for any migration anyway. nats.go
  does not mark the legacy API `// Deprecated:` (only "NOTE: ... is part of legacy API"
  comments), so staticcheck SA1019 never sees it; the rule is not redundant.
- Anything needing cross-package facts or whole-program analysis.
- Anything needing runtime configuration knowledge (e.g. "`Ack()` called on a consumer
  whose ack policy is `AckNone`").
- Re-implementing what generic linters already do: unchecked errors from `Ack()`
  (`errcheck`), `err == jetstream.ErrX` on possibly-wrapped errors (`errorlint`),
  use of `// Deprecated:` APIs (`staticcheck` SA1019).
- Style opinions. The only opinionated rules are in Tier 3 and are off by default.

## 2. Architecture

### 2.1 Repository and module

- Separate repository and module. Initial module path `github.com/piotrpio/natsvet`;
  intended final home `github.com/nats-io/natsvet` (fallback: a module under
  `github.com/synadia-io/orbit.go`, which already hosts per-directory modules and tooling).
  Renaming the module path is free until someone depends on it, and must happen before
  any announcement or golangci-lint submission.
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
  LICENSE                    // Apache 2.0
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
    nilheader/
    headerkey/
    drain/
    handle/
    msgloop/
    pubasync/
    legacyjs/                // opt-in, migration inventory
    connopts/                // opt-in
    microhandler/            // opt-in
  testdata/
    go.mod                   // module natsvet/testdata; requires nats.go (real API, see §4)
    go.sum
    <rule>/                  // one package per rule, named after the rule
  scripts/
    corpus.sh                // runs the binary over pinned real-world repos, see §4.3
    corpus.txt               // repo, commit, optional path filter
    corpus.expected          // triaged findings
  docs/design.md             // this document
  openspec/                  // change proposals, specs, tasks
  .github/workflows/ci.yml
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
  `nats.KeyValueConfig`). Same pitfall, same rule; this is not migration work. Field
  names differ in places (`jetstream.ConsumerConfig.IdleHeartbeat` is
  `nats.ConsumerConfig.Heartbeat`); the rule maps them, the spec lists both.
- Every check that mirrors a server or client validation names the function it mirrors
  (`checkConsumerCfg`, `checkStreamCfgLocked`, `keyValid`, ...) in the rule's spec. When
  a rule is implemented its check list is re-derived from that function line by line,
  not copied from this document.
- Config rules check what reaches the server. A literal passed directly to a call or
  returned is checked as written. A literal bound to a variable is checked as written
  up to the first hand-off (by-value argument to a concretely typed parameter, return,
  channel send, or pointer argument to a nats.go method; a logger's `...any` is not a
  hand-off); fields assigned before that point are unknown. A
  pointer escaping to another call, a method call on the variable, a closure capture,
  or no hand-off at all falls back to treating every field assigned anywhere in the
  function as unknown. Mutation inside another function is a known, accepted hole.
- No cross-package facts. Each rule reasons only about the package being analyzed.
- `pass.Module` (module path/version of the analyzed package) is available in recent
  x/tools drivers but is not needed: if an API does not exist in the user's nats.go
  version, their code does not compile and the rule never sees it. Server-side
  validations encoded by rules are long-standing; rules must not reference server or
  client versions.
- Opt-in rules (`legacyjs` and Tier 3) are registered but disabled: each declares a
  boolean flag `enable` (default false) and returns early from `Run` when it is not set.
  `multichecker` exposes it as `-connopts.enable`. Default-on rules can be disabled the
  standard way (`-drain=false`).

### 2.4 `internal/natsapi` helpers

Shared, tested independently. A helper is added when the first rule needs it, not
speculatively; the list below is the expected end state.

- `type Pkg string` with constants `Core = "github.com/nats-io/nats.go"`,
  `JetStream = ".../jetstream"`, `Micro = ".../micro"`; orbit modules
  (`github.com/synadia-io/orbit.go/<mod>`) are added as further constants when a rule
  needs them. `IsPkg(obj types.Object, pkg Pkg) bool` matches on the object's package
  path, also when vendored (compare on the path suffix after `vendor/`).
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
- Subject helpers, ported from nats-server `server/sublist.go` with its test tables:
  `IsValidSubject(s) bool` (no empty token, no whitespace, `>` only last),
  `SubjectIsLiteral(s) bool` (no `*`/`>` token), `SubjectsCollide(a, b) bool`,
  `SubjectIsSubsetMatch(subject, filter string) bool`. nats.go's own `badSubject` is
  a subset of `!IsValidSubject`.
- KV name validation from nats.go `jetstream/kv.go`: `validBucketRe = ^[a-zA-Z0-9_-]+$`,
  `validKeyRe = ^[-/_=\.a-zA-Z0-9]+$`, `validSearchKeyRe = ^[-/_=\.a-zA-Z0-9*]*[>]?$`,
  and `keyValid`/`searchKeyValid` (non-empty, no leading or trailing `.`, no `..`).
  `validBucketRe` also governs `ObjectStoreConfig.Bucket` (`ErrInvalidStoreName`).
- Known header table: every exported string constant in the `nats`, `jetstream` and
  `micro` packages whose value starts with `Nats-`, mapped to the qualified constant
  name(s). Currently 12, 21 and 2 constants respectively; the same header often has a
  constant in both `nats` (`MsgIdHdr`) and `jetstream` (`MsgIDHeader`). Generated by a
  `go generate` program that loads the nats.go packages through `go/packages` from the
  `testdata` module; a test regenerates and diffs, so bumping the pinned nats.go fails
  the build until the table is regenerated.
- `EnclosingFunc(pass, node) (ast.Node, *ast.FuncType)` — nearest `FuncDecl`/`FuncLit`.
- `SingleDefinition(pass, ident) (ast.Expr, bool)` — if the identifier's object is
  assigned exactly once in its enclosing function (`:=`, `=`, or `var x = `), return the
  RHS expression; otherwise false. Used by `syncsub`, `nilheader` and `msgloop`.
- `Discarded(stack) bool` — the call at the top of the inspector stack is an expression
  statement or the single RHS of an assignment whose first LHS is `_`. `handle`, `pubasync`.
- `PackageUses(info, pkg, recv, name) bool` — any file of the analyzed package uses the
  function or method; the one package-scope question so far. `pubasync`.
- `IsNamedType(t, pkg, name) bool`, `ExitsProcess(info, call) bool` (builtin `panic`,
  `os.Exit`, declared `Fatal*`). `handle`; `msgloop` and `drain`.

## 3. Rule catalog

Derived from nats.go `272f938` and nats-server `c16afd1` (both 2026-06-15).

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
are constant in the same composite literal. Checks that depend on the stream config
(`Replicas` vs stream, retention policy, `ConsumerLimits`) or on server/account limits
are out of scope: the rule cannot see them.

- **Hooks**: composite literals of `jetstream.ConsumerConfig`, `jetstream.OrderedConsumerConfig`
  (subset of fields), `nats.ConsumerConfig`. Field names below are the `jetstream` ones;
  `nats.ConsumerConfig.Heartbeat` is `jetstream.ConsumerConfig.IdleHeartbeat`.
- **Detect**, in `checkConsumerCfg` order (each is its own diagnostic; "push" =
  `DeliverSubject` constant non-empty, "pull" = field absent or constant `""`):
  1. `Name` or `Durable` non-empty and failing `isValidAssetName`: contains any of
     `.`, `*`, `>`, `\`, `/` or whitespace (` \t\r\n\f`).
  2. `Replicas < 0`.
  3. Any `BackOff` element `< 0`; `AckWait < 0`.
  4. `len(BackOff) > MaxDeliver` when `BackOff` is a slice literal and `MaxDeliver` is a
     constant `> 0`. (Server defaults `MaxDeliver` `0` to `-1` = unlimited before this
     check; skip when unset, `0`, or `-1`.)
  5. `len(Description) > 4096` (`JSMaxDescriptionLen`).
  6. push: `DeliverSubject` not a literal subject (contains a `*`/`>` token) or failing
     `IsValidSubject`; `MaxWaiting != 0`; `MaxAckPending > 0 && AckPolicy == AckNonePolicy`
     (push only); `IdleHeartbeat` in `(0, 100ms)`.
  7. pull (`DeliverSubject` absent or `""`): `RateLimit > 0`; `MaxWaiting < 0`;
     `IdleHeartbeat > 0` (heartbeat is a pull-request option, not a config field);
     `FlowControl`; `MaxRequestBatch < 0`; `MaxRequestExpires` in `(0, 1ms)`.
  8. `FilterSubject` non-empty and `FilterSubjects` non-empty.
  9. `FilterSubject` failing `IsValidSubject`; a `FilterSubjects` element that is `""` or
     fails `IsValidSubject`.
  10. Two elements of `FilterSubjects` (or `FilterSubject` and an element) where one is a
      subset match of the other (`SubjectIsSubsetMatch`, both directions).
  11. `DeliverPolicy` vs start options (server `badStart`/`notSet`):
      `DeliverAll` (also when the field is absent), `DeliverLast`, `DeliverNew`,
      `DeliverLastPerSubject` with `OptStartSeq > 0` or `OptStartTime` present and not
      `nil`; `DeliverByStartSequence` with `OptStartSeq` absent or `0`, or with
      `OptStartTime`; `DeliverByStartTime` with `OptStartTime` absent or `nil`, or with
      `OptStartSeq != 0`.
  12. `DeliverLastPerSubject` with neither `FilterSubject` nor `FilterSubjects`.
  13. `SampleFrequency` constant that, after trimming a trailing `%`, is not a
      non-negative integer.
  14. `FlowControl` and `IdleHeartbeat` absent or `0`.
  15. `Durable` and `Name` both non-empty constants and different.
  16. `PriorityPolicy != PriorityNone` with `DeliverSubject` set, or with `PriorityGroups`
      empty, or with an element that is `""` or fails the server's `validGroupName`;
      `PriorityPolicy` absent or `PriorityNone` with `PriorityGroups` non-empty or
      `PinnedTTL > 0`.
  17. `AckPolicy == AckFlowControlPolicy` without push, without `FlowControl`, with
      `IdleHeartbeat` other than exactly 1s, with `MaxAckPending <= 0`, with `AckWait` or
      `BackOff` set, or with `MaxDeliver > 0`.
- **Message**: `consumer config: <server's wording>`, e.g.
  `consumer config: FilterSubject and FilterSubjects cannot both be set`.
- **Fix**: none. (Which field the user meant is ambiguous.)
- **FP**: none by construction; only constant fields are considered. A field set from a
  variable is treated as unknown, which disables any check involving it.
- **Tests**: one positive and one negative case per check; a literal with all fields
  from variables produces nothing; pointer literal `&jetstream.ConsumerConfig{...}`;
  legacy `nats.ConsumerConfig` twin; `OrderedConsumerConfig` for checks 9-12.

#### `streamconfig`

Mirrors nats-server `checkStreamCfgLocked` (`server/stream.go`). Same constant-only
policy. Checks inside nested `Mirror`/`Sources`/`SubjectTransform`/`RePublish` literals
and checks against account limits or other streams are out of scope for now.

- **Hooks**: composite literals of `jetstream.StreamConfig`, `nats.StreamConfig`. Field
  names below are the `jetstream` ones (`MaxMsgsPerSubject`, `DiscardNewPerSubject`).
- **Detect**, in server order (absent enum fields take the server default: `Retention`
  `LimitsPolicy`, `Discard` `DiscardOld`, `Storage` `FileStorage`, `Replicas` `1`):
  1. `Name` empty, failing `isValidAssetName` (see `consumerconfig` 1), or longer than
     255 (`JSMaxNameLen`).
  2. `len(Description) > 4096`.
  3. `Replicas > 5` (`StreamMaxReplicas`) or `< 0`.
  4. `MaxAge < 0`, or in `(0, 100ms)`.
  5. `Duplicates < 0`; in `(0, 100ms)`; or `> MaxAge` when both constant and
     `MaxAge > 0`. (`Duplicates` absent or `0` is defaulted, never an error.)
  6. `DenyPurge` and `AllowRollup`.
  7. `AllowMsgCounter` with `Discard == DiscardNew`, `AllowMsgTTL`, `AllowMsgSchedules`,
     or `Retention != LimitsPolicy`.
  8. `DiscardNewPerSubject` with `Discard != DiscardNew`, or with `MaxMsgsPerSubject`
     absent or `<= 0`.
  9. `SubjectDeleteMarkerTTL < 0`, or in `(0, 1s)`.
  10. `AllowMsgSchedules` with `Discard == DiscardNew`, or with `Sources` non-empty.
  11. `PersistMode == AsyncPersistMode` with `Storage != FileStorage`, `Replicas > 1`, or
      `AllowAtomicPublish`.
  12. `Mirror` present (non-nil) with any of: `FirstSeq > 0`, `Subjects` non-empty,
      `Sources` non-empty, `AllowMsgCounter`, `AllowAtomicPublish`, `AllowBatchPublish`,
      `AllowMsgSchedules`, `SubjectDeleteMarkerTTL > 0`.
  13. `Subjects` slice literal: an element failing `IsValidSubject`; two equal elements;
      two elements where `SubjectsCollide`; an element equal to `>` without `NoAck` or
      with `Replicas != 1`; an element colliding with `$JS.>`, `$JSC.>`, `$NRG.>` (unless
      a subset of `$JS.EVENT.>`) or `$SYS.>` (unless a subset of `$SYS.ACCOUNT.>`)
      without `NoAck`.
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
- **Detect** (mirrors `jetstream/kv.go` `CreateKeyValue`, `keyValid`, `searchKeyValid`
  and `jetstream/object.go` `CreateObjectStore`):
  1. `Bucket` constant not matching `validBucketRe` (`ErrInvalidBucketName`,
     `ErrInvalidStoreName`).
  2. `History` constant `> 64` (`jetstream.KeyValueMaxHistory`). Values `<= 0` default to 1
     and are accepted.
  3. Key constant that is empty, starts or ends with `.`, contains `..`, or fails
     `validKeyRe` (`validSearchKeyRe` for `Watch*`/`ListKeysFiltered`).
- **Message**: `invalid KV key "foo bar": keys may only contain [-/_=.a-zA-Z0-9]`;
  `KV history 100 exceeds the maximum of 64`; `invalid bucket name "my.bucket"`.
- **Fix**: none.
- **FP**: none by construction.
- **Tests**: valid/invalid bucket; history 64 (ok) and 65; keys with space, leading dot,
  `a..b`, wildcard in `Put` (bad) vs wildcard in `Watch` (ok).

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
  1. Any constant subject failing `IsValidSubject`: empty or whitespace token, or `>`
     not in last position (nats.go returns `ErrBadSubject` at call time for the token
     cases; the server rejects the `>` case).
  2. Publish-type calls (`Publish*`, `Request*`, `Msg.Subject` used in a publish,
     jetstream `Publish*`) whose subject is not `SubjectIsLiteral` — the server rejects
     it only in pedantic mode; otherwise the `*`/`>` are literal tokens and the message
     reaches no one the user intended.
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
  `MaxRequestExpires`, `IdleHeartbeat`, `MaxAge`, `Duplicates`, `TTL`, elements of `BackOff`, and the
  `nats.Options` struct — without enumerating them.
- **Detect**: the argument, field value or assigned value is an untyped integer constant
  (a `BasicLit`, or an identifier resolving to an untyped constant, or an arithmetic
  expression of those with no typed constant, conversion or call anywhere in its AST)
  with value `v > 0`. `0` and negatives are excluded (they commonly mean
  "default"/"none"). No upper bound: a millisecond-mindset `86400000` is as unit-less
  as a `5`. Field assignments (`opts.Timeout = 5`) are covered as well as literals.
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
  function whose RHS is one of the calls below. Three cases, three messages, per
  `validateNextMsgState` in nats.go:
  1. Callback constructors `Subscribe`, `QueueSubscribe` on `*nats.Conn`: `mcb != nil`, so
     `NextMsg` returns `ErrSyncSubRequired` on every call.
  2. Channel constructors `ChanSubscribe`, `ChanQueueSubscribe`, `QueueSubscribeSyncWithChan`:
     nothing rejects the call. `NextMsg` reads from the user's own channel and silently
     competes with the code ranging over it; delivery accounting for `AutoUnsubscribe`
     is also double-counted on that path.
  3. Legacy `nats.JetStreamContext.PullSubscribe`: `NextMsg` returns
     `ErrTypeSubscription`; `Fetch` is the only way to read.
  (`SubscribeSync` and `QueueSubscribeSync` are the sync constructors.)
- **Message**: `NextMsg on a subscription created with Subscribe (callback) always returns
  ErrSyncSubRequired; use SubscribeSync`; `NextMsg on a subscription created with
  ChanSubscribe steals messages from the channel; read the channel or use SubscribeSync`;
  `NextMsg on a pull subscription returns ErrTypeSubscription; use Fetch`.
- **Fix**: none. (Rewriting to `SubscribeSync` would orphan the callback or channel.)
- **FP**: identifiers with more than one assignment are skipped.
- **Tests**: callback sub (bad), chan sub (bad), pull sub (bad), sync sub (ok), sub
  reassigned twice (ok), sub passed in as a parameter (ok).

#### `nilheader`

`nats.Header.Set` and `Add` are plain map writes with no nil check (`nats.go`
`Header.Set`); `Get`, `Values` and `Del` tolerate a nil map. `nats.NewMsg` allocates the
header, a `nats.Msg` composite literal does not.

- **Hooks**: `Set` and `Add` on `nats.Header` reached through `<ident>.Header`.
- **Detect**: `<ident>` has a `SingleDefinition` in the enclosing function whose RHS is a
  `nats.Msg` composite literal (value or `&`) with no `Header` key, and no assignment to
  `<ident>.Header` occurs in the function before the call.
- **Message**: `Header.Set on a nats.Msg literal without Header panics (nil map); use
  nats.NewMsg or set Header: nats.Header{}`.
- **Fix**: none. (Both fixes are reasonable; the user picks.)
- **FP**: none by construction; any `Header:` key or later `.Header =` disables it.
- **Tests**: literal without header + `Set` (bad), `NewMsg` (ok), literal with
  `Header: nats.Header{}` (ok), literal then `m.Header = ...` then `Set` (ok), `Get` on
  literal without header (ok).

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
  `if err != nil { return ... }` on Drain's error); and `defer x.Close()` in a function
  whose last action is `x.Drain()`, excluding `main` and test functions (there the
  process exit dominates, see §3.2). `Close` aborts the in-progress drain.
- **Message**: `Close immediately after Drain aborts the drain; wait for the
  ClosedHandler instead`.
- **Fix**: none.
- **FP**: none.
- **Tests**: adjacent (bad), adjacent with error check between (bad), `Drain` then wait on
  a channel then `Close` (ok).

### 3.2 Tier 2 — lifecycle mistakes (default on, v0.2, some judgment)

#### `handle` (implemented)

- **Hooks**: calls returning `jetstream.ConsumeContext` (`Consumer.Consume`,
  `PushConsumer.Consume`), `jetstream.MessagesContext` (`Consumer.Messages`),
  `jetstream.KeyWatcher` (`KeyValue.Watch`, `WatchAll`, `WatchFiltered`),
  `jetstream.ObjectWatcher` (`ObjectStore.Watch`), `micro.Service` (`micro.AddService`).
  Legacy twins (`nats.KeyWatcher`, `nats.ObjectWatcher`) included. `*nats.Subscription`
  from `Conn.Subscribe*` is not hooked and there is no flag for it: the corpus has about
  480 such discards, 50 in production code, every one a subscription meant to live as
  long as the connection (§8, item 12).
- **Detect**: the first result is assigned to `_` (in any assignment, including an
  `if`/`for`/`switch` initializer) or dropped by an expression statement. A `Consume` or
  `Messages` call with a `jetstream.StopAfter` argument stops itself and is exempt.
- **Message**: `<Type> from <Method> discarded; the consumer can never be stopped or
  drained` / `...the watcher can never be stopped and its subscription lives as long as
  the connection` / `...the service can never be stopped and keeps answering until the
  connection closes`.
- **Fix**: none.
- **FP**: `main` is not exempt — it is the one place a signal handler would `Drain()` the
  context, and nats.go's own examples keep it. Corpus: 3 TP (an example, two natscli
  commands), 8 FP (tests asserting an error before any watcher exists); default-on.
- **Tests**: each hook with `_` and as a bare statement (bad), kept, returned, `StopAfter`
  literal and variable (ok), a user type with its own `Consume` (ok).

#### `drain` (extension, implemented, default-on)

- **Detect**: in `func main()` of `package main` only, `defer x.Drain()`, or an
  `x.Drain()` statement that is the last statement of `main`, or is followed by a
  `return` or by a call that exits the process (`os.Exit`, `log.Fatal*`, `panic`).
  `Drain` returns after `go nc.drainConnection()`; the exit kills that goroutine. Test
  and benchmark functions are not reported: their return does not end the process. The
  deferred-`Close` check keeps its `main` exemption so a trailing `Drain` in `main` gets
  this diagnostic alone.
- **Message**: `Drain in main followed by process exit drains nothing; Drain returns
  immediately, wait for the ClosedHandler before exiting`.
- **Decision** (2026-09-17): ship it; public docs snippets that show `defer nc.Drain()`
  in `main` are to be corrected, not accommodated. The corpus has no deferred `Drain` in
  any `main`, but nats.go's `nats-qsub` and `nats-rply` examples do `nc.Drain();
  log.Fatalf(...)` under a comment promising a drain — the "then exit" shape is the one
  with evidence.

#### `msgloop` (implemented)

- **Hooks**: `jetstream.MessagesContext.Next`; `jetstream.Consumer.Fetch`, `FetchBytes`,
  `FetchNoWait` and legacy `Subscription.FetchBatch` results (`MessageBatch`).
- **Detect**:
  1. Inside a condition-less `for` (no init, condition or post), `msg, err := it.Next()`
     followed by an `if err != nil` — or an `if` with that assignment as its initializer
     — whose then-branch contains no `return`, `break`, `goto`, continue of an outer
     loop, or process-exiting call, and either ends in `continue` or has an `else`; and
     nothing in the loop body refers to `jetstream.ErrMsgIteratorClosed`. After `Stop`
     or `Drain`, `Next` returns `ErrMsgIteratorClosed` on every call without blocking
     and the loop never exits (a `time.Sleep` in the branch turns the spin into a leaked
     goroutine, same bug, same fix). Conditional loops have an exit the author chose and
     are not reported; `Consumer.Next` is a one-shot fetch with a timeout and cannot spin.
  2. A `range r.Messages()` where `r` is a local with a single definition from one of
     the fetch calls, every use of `r` in the function is `r.Messages()` or `r.Error()`,
     and `r.Error()` is never called. A batch passed to a helper or a batch parameter is
     the other function's business. `fetchResult.err` carries `ErrNoHeartbeat`, a
     terminal status such as consumer deleted, and the context's error.
- **Message**: `loop continues on every Next error; after Stop or Drain, Next returns
  ErrMsgIteratorClosed on every call and the loop never exits`; `<Method> result ranged
  without checking Error(); a failed fetch looks like an empty batch`.
- **Fix**: none.
- **FP**: corpus 1 TP (the `jetstream-basic` docs example), 0 FP; nats.go's
  `jetstream/test/` — where the batch-counting tests live — is not in the corpus.
- **Tests**: log-and-continue, `else`, `if`-initializer, backoff (bad); labeled continue,
  closed check in either place, return, `Fatal`, `os.Exit`, conditional and range loops,
  `Consumer.Next`, timeout-only branch (ok); range without `Error` on each fetch method
  (bad); `Error` after the range or in a deferred closure, helper, parameter, reassigned,
  `select` receive (ok).

#### `pubasync` (implemented)

- **Hooks**: `PublishAsync`, `PublishMsgAsync` on `jetstream.JetStream` (declared in the
  embedded `Publisher` interface) and legacy `nats.JetStream`.
- **Detect**: the `PubAckFuture` result is assigned to `_` or dropped by an expression
  statement, in a package none of whose files refers to `WithPublishAsyncErrHandler` /
  `nats.PublishAsyncErrHandler` or calls `PublishAsyncComplete`. The exemptions are
  package-wide, not function-wide: the handler is installed once per `JetStream`
  instance, typically in a constructor, and the completion wait belongs to a `Close`;
  nats.go's own `ObjectStore.Put` is the reference for the split, and a default-on rule
  must not flag it. `WithPublishAsyncAckHandler` is not an exemption: `handleAsyncReply`
  calls it only after a valid `PubAck`; errors go only to the future's `Err` and to the
  error handler, so an ack handler without an error handler is the half-wired case the
  rule exists for.
- **Message**: `PubAckFuture from <Method> discarded and this package never sets an
  async error handler or awaits PublishAsyncComplete; publish errors are lost`.
- **Fix**: none.
- **FP**: corpus 0 findings — every corpus package that discards a future also awaits
  completion or installs a handler somewhere; the rule's precision rests on its seven
  testdata packages. Default-on.
- **Tests**: one testdata package per exemption state: discard shapes (bad), error
  handler elsewhere, completion in another method, both handlers, legacy handler (ok),
  ack handler only, legacy discard (bad).

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

#### orbit.go recommendations (future)

Rules that flag a hand-rolled pattern for which an orbit.go module exists, e.g. a
`Subscribe` on an inbox followed by a timed collection loop → `natsext.RequestMany`;
manual `nats.Connect` from `~/.config/nats/context` files → `natscontext.Connect`. No
concrete rules until the corpus run shows the patterns; listed so the tier has a home.

### 3.4 Migration inventory (opt-in via `-legacyjs.enable`)

#### `legacyjs`

Step 0 of the migration family: report, never fix. Answers "how much legacy JetStream API
does this module use, and where" so the later rewrite family can be sized and so users
can track their own progress.

- **Hooks**: any use of a type, method, function or option from the legacy API surface:
  `nats.JetStreamContext`, `nats.JetStream`, `nats.JetStreamManager`, `nats.KeyValue`,
  `nats.KeyValueManager`, `nats.ObjectStore`, `nats.ObjectStoreManager`,
  `(*nats.Conn).JetStream`, `nats.SubOpt`, `nats.PubOpt`, `nats.PullOpt`,
  `nats.JSOpt`, the `nats.ConsumerConfig` / `nats.StreamConfig` / `nats.KeyValueConfig`
  / `nats.ObjectStoreConfig` types, and `(*nats.Subscription).Fetch` / `FetchBatch`.
- **Detect**: any identifier whose object is in that set; one diagnostic per use site.
- **Message**: `legacy JetStream API: nats.JetStreamContext; see the jetstream package`.
- **Fix**: none.
- **FP**: none; it reports facts. Off by default because it is an inventory, not a
  finding.
- **Tests**: one use of each listed symbol (bad), the `jetstream` twin of each (ok).

## 4. Testing

### 4.1 Unit tests per rule

`analysistest.Run` / `RunWithSuggestedFixes` against a shared `testdata` directory with
`// want "..."` comments and `.golden` files for fixes.

Use **module mode** so tests compile against the real nats.go API rather than stubs.
`analysistest.Run` treats a directory containing `go.mod` as a module root (documented in
x/tools v0.43 `analysistest.go`; a `go.work` there is honored too). It runs with
`GOPROXY=off`, so the module must already be in the module cache: the Makefile `test`
target and CI run `cd testdata && go mod download` first.

Each rule's test package is `testdata/<rule>` and is exercised with the pattern
`natsvet/testdata/<rule>`.

### 4.2 Helper tests

`internal/natsapi` has table-driven tests for subject helpers (port nats-server's own
subset-match test table), KV regexes, and the header table (must contain every `Nats-`
constant exported by the pinned nats.go — a test that fails when nats.go adds one).

### 4.3 Corpus run (false-positive gate)

The only measurement of false-positive rate is running the tool over code we did not
write to trigger it. `scripts/corpus.sh` clones the pinned commits listed in
`scripts/corpus.txt` (`<repo url> <commit> [<path filter>]`, cached under a local
directory), runs `natsvet ./...` with every opt-in rule enabled over each, normalizes
the output (strips the clone prefix, sorts) and diffs it against
`scripts/corpus.expected`. Every line in the expected file was triaged by hand and is
tagged `TP` (a real bug in that repo — README material and an upstream PR) or `FP`
(an accepted false positive with a one-line reason). Any line not in the file fails the
job.

Corpus: nats.go's own `examples/` and `test/`; nats-server's `server/` and `test/`
(its tests build every invalid config the server rejects — the best oracle for the
config rules); `nats-io/natscli`, `nats-io/nack`, `synadia-io/nex`; every per-module
`test/` directory of `synadia-io/orbit.go`; and three third-party users,
`synadia-io/connect`, `choria-io/go-choria` and `knative-extensions/eventing-natss`.
The whole list lints in about 75 seconds cold.

The script lands in the bootstrap change and runs after every rule group, not only
before a tag: the per-rule finding/FP counts decide Tier 2 defaults (§3.2), can demote
a Tier 1 rule to opt-in, and tell us which rules are worth building next. Pinned commits
are bumped deliberately: rerun, triage the delta, commit both files.

### 4.4 CI

`go vet`, `staticcheck`, `go test ./...` (with the testdata download step), `misspell
-locale US`, and the corpus job. `gofmt` check. Run on the two newest Go versions.

## 5. Distribution

1. `go install github.com/piotrpio/natsvet/cmd/natsvet@latest`, then `natsvet ./...`,
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

## 6. Delivery

Work is cut into OpenSpec changes grouped by the helpers the rules share, so each change
is one reviewable unit and helpers are built once. Specs are one per rule (each Detect
item a requirement, each test case a scenario, the spec text doubling as the rule `Doc`)
plus `analyzer-framework` and `testing`.

Progress (2026-09-17): 1–3, the release-readiness part of 4, and 5 are archived under
`openspec/changes/archive/`; nothing is in flight. The next change to propose is
`golangci-plugin` (6). The tag waits for the repository move.
Read the archived change's `design.md` before extending a rule: that is where the
corpus-driven corrections live.

1. **`bootstrap`** (done): module, license, `cmd/natsvet`, `internal/natsapi` with only what
   the two rules below need, testdata module, CI, README skeleton, corpus script with
   the initial repo list. `headerkey` end-to-end (default-on, `SuggestedFix`, golden
   file) proves the fix path; `legacyjs` (opt-in) proves the `enable` flag path and is
   the migration inventory.
2. **`config-rules`** (done): `consumerconfig`, `streamconfig`, `kvconfig` — share
   `CompositeFields`, the subject helpers and the KV regexes. Corpus run and triage
   close the change.
3. **`callsite-rules`** (done): `duration`, `subject`, `ctxdeadline`, `syncsub`, `nilheader`,
   `drain` — share `Callee`/`IsMethod`/`SingleDefinition`. `duration` and `subject`
   first: they are the likeliest to fire on the corpus. Corpus run closes the change.
4. **`release-readiness`** (done) and the tag (waiting): full triage over nine corpus
   sources, generated `docs/rules.md`, `-version`, README. The `v0.1.0` tag follows the
   repository move.
5. **`lifecycle-rules`** (done): `handle`, `msgloop`, `pubasync` and the `drain`-in-`main`
   check, all default-on; the corpus confirmed the defaults (6 TP, 8 FP, none on
   production code). Added `Discarded`, `PackageUses`, `IsNamedType` and `ExitsProcess`
   to `natsapi`; reused `SingleDefinition` and `EnclosingFuncBody` (not the `Masker`).
6. **`golangci-plugin`**: module plugin config in the repo, then the upstream PR once
   the module path is final.
7. Later, separate designs: migration rewrites (legacy → `jetstream`), orbit.go
   recommendation rules, orbit.go API rules.

## 7. Code conventions for the implementing repository

- Apache 2.0 license header on every `.go` file, the orbit.go form:
  `// Copyright 2026 Synadia Communications Inc.` then the standard boilerplate.
- Comments only where they carry information the code does not; rule `Doc` strings are
  the exception and should be complete. No version numbers, links, or issue references in
  code comments.
- `for i := range n` over the three-clause form.
- Table-driven tests; no comments that narrate the next statement.
- US English (`misspell -locale US`).
- Commits signed off (`git commit -s`), single-line subjects.

## 8. Open questions

1. Final repository home: `nats-io/natsvet` vs a module in `synadia-io/orbit.go`.
   Decided before any announcement; irrelevant until then.
2. ~~Whether to ship the `defer nc.Drain()`-in-`main` check at all (§3.2).~~ Shipped
   default-on in `lifecycle-rules`; the docs snippets are the thing to fix.
3. ~~Tier 2 defaults (`handle`, `pubasync`) — decided by the corpus run, not now.~~ Both
   default-on: no false positive on non-test code in the corpus (§3.2).
4. Name: `natsvet` for the binary; golangci-lint linter names conventionally end in
   `lint`/`check` — may register there as `natslint` or `natscheck` while the module stays
   `natsvet`.
5. Whether `headerkey` should also flag a `Get` literal that differs only in case from a
   `Set` literal elsewhere in the same package (needs a package-wide pre-pass; cheap, but
   deferred until there is evidence it happens).
6. Migration rewrites: every `jetstream` method takes a `context.Context` and no legacy
   method does, so a rewrite is never a local `SuggestedFix`; it needs per-function
   reasoning about where a context comes from. Likely a separate `natsvet migrate`
   driver with a report, not `go fix`. Design when `legacyjs` numbers exist.
7. Which orbit.go APIs warrant rules of their own (§1, item 3). Survey after the corpus
   run.
8. Legacy `js.Subscribe*(subj, nats.Bind(stream, consumer))` with a non-empty constant
   subject: the subject is never used as a subscription subject, only compared for
   equality with the consumer's `FilterSubject` (`js.go` `processConsInfo`), so a
   wildcard or unrelated constant fails with `ErrSubjectMismatch` whenever a filter is
   set. eventing-natss ships `PullSubscribe(".>", ..., nats.Bind(...))`. A rule "pass
   `""` with `Bind`" would catch it; `subject` deliberately skips these calls.
9. `headerkey` near-miss: an unknown `Nats-*` key within a small edit distance of a
   known header (`Nats-TTLSeconds` for `Nats-TTL`, seen in go-choria). The table lookup
   is exact today; a Levenshtein threshold of 2-3 over the known set would flag it
   without touching user headers that share only the prefix.
10. Config rules inside nested literals: `Mirror`/`Sources` subject transforms,
    `SubjectTransform` and `RePublish` (source validity, destination mapping via the
    server's `ValidateMapping`, republish cycles). nats-server test cases to derive
    from live in `server/jetstream_test.go` (overlapping transform sources, invalid
    transform source `events.>.*`, bad `{{split(3,1)}}` destination, republish cycle).
    `CompositeFields` already descends one level; the port of `ValidateMapping` is the
    work.
11. Discarded `Fetch` batch: `_, err := c.Fetch(10)` sends a pull request whose messages
    are delivered to nobody and redelivered after `AckWait`. Same shape as `handle`,
    different consequence. The corpus has 57 such sites, all in tests, about half of them
    `_, _ = sub.Fetch(1, nats.MaxWait(time.Microsecond))` to poke a pull; no production
    evidence yet.
12. Discarded `*nats.Subscription` (`_, err := nc.Subscribe(...)`): about 480 corpus
    sites, 50 in production code (natscli, nats.go's examples, go-choria), every one a
    subscription meant to live as long as the connection. Neither a default-on `handle`
    row nor an off-by-default flag has evidence behind it; revisit only with a real case.
13. `handle`/`pubasync` on `go`/`defer` statements (`go cons.Consume(h)`): not a discard
    shape today; nobody writes it. Add to `Discarded` if it ever shows up.
