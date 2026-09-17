# natsvet

Static analysis for Go programs that use [nats.go](https://github.com/nats-io/nats.go).
natsvet turns runtime failures and lifecycle mistakes that nats.go users hit repeatedly —
server-rejected stream and consumer configs, invalid subjects, missing duration units,
misused subscriptions, miscased headers, aborted drains — into diagnostics at build time,
with automatic fixes where the rewrite is certain.

It is built on `golang.org/x/tools/go/analysis`, so one binary works standalone, as a
`go vet` tool, and as a `go fix` tool.

## Install

```sh
go install github.com/piotrpio/natsvet/cmd/natsvet@latest
natsvet -version
```

The module path is provisional: it moves to its final home before the first tagged
release, and installs from the new path will replace the above.

## Use

```sh
natsvet ./...                 # report
natsvet -fix ./...            # apply suggested fixes
natsvet -fix -diff ./...      # show the fixes as a diff

go vet -vettool=$(which natsvet) ./...
go fix -fixtool=$(which natsvet) ./...      # Go 1.26+
```

Default-on rules can be turned off with `-<rule>=false`; opt-in rules are enabled with
`-<rule>.enable`. A `go vet` run caches results per package, so rebuild-and-rerun after
changing the binary.

## Rules

Every rule's documentation, with a before/after snippet, is in
[`docs/rules.md`](docs/rules.md) — generated from the analyzers themselves, so it
cannot drift from what the tool does.

| Rule | Default | Fix | Reports |
|------|---------|-----|---------|
| `consumerconfig` | on | no | a `ConsumerConfig` literal the server rejects in `checkConsumerCfg`: overlapping or empty filters, deliver policy vs start options, heartbeat or flow control on a pull consumer, priority policy without groups, and more |
| `streamconfig` | on | no | a `StreamConfig` literal the server rejects in `checkStreamCfgLocked`: invalid or overlapping subjects, replicas > 5, windows under 100ms, mirror with subjects or sources, and more |
| `kvconfig` | on | no | a KV or object store bucket name, history, or key that nats.go rejects client-side (`ErrInvalidBucketName`, `ErrHistoryTooLarge`, `ErrInvalidKey`) |
| `duration` | on | no | an untyped constant where nats.go expects a `time.Duration` (`nc.Request("s", nil, 5)` waits 5ns, `AckWait: 30` acks in 30ns); arguments, fields, field assignments and option types like `nats.MaxWait` |
| `subject` | on | no | a constant subject nats.go or the server rejects (empty, whitespace, empty token, misplaced `>`), a publish to a subject with a wildcard token (delivered literally), or a queue group with whitespace |
| `ctxdeadline` | on | no | `context.Background()`/`context.TODO()` passed directly to `FlushWithContext` (`ErrNoDeadlineContext`), `RequestWithContext`/`NextMsgWithContext` (may block forever), or `nats.Context(...)` on legacy `Fetch` |
| `syncsub` | on | no | `NextMsg` on a subscription created with a callback (`ErrSyncSubRequired`), a channel (steals from it) or a legacy pull subscribe (`ErrTypeSubscription`) |
| `nilheader` | on | no | `Header.Set`/`Add` or `Header[k] =` on a `nats.Msg` literal without a `Header` — a nil-map panic; `nats.NewMsg` allocates it |
| `drain` | on | no | `Close()` right after `Drain()`, or `defer nc.Close()` in a function that ends with `Drain()`; `Drain` returns immediately and the `Close` aborts it |
| `handle` | on | no | a `ConsumeContext`, `MessagesContext`, `KeyWatcher`, `ObjectWatcher` or `micro.Service` assigned to `_` or dropped; nothing can ever `Stop` or `Drain` it |
| `msgloop` | on | no | a `for { msg, err := it.Next(); if err != nil { continue } }` that never exits once the iterator is stopped, and a `Fetch` batch ranged over without `Error()` |
| `pubasync` | on | no | a `PublishAsync` future assigned to `_` in a package that neither installs `WithPublishAsyncErrHandler` nor awaits `PublishAsyncComplete`; publish errors are lost |
| `headerkey` | on | yes | a header key that differs only in case from a NATS header (`nats-msg-id` vs `Nats-Msg-Id`); nats.go header lookups are case-sensitive, so the lookup silently fails |
| `legacyjs` | opt-in | no | every use of the legacy `nats.JetStreamContext` / `nats.KeyValue` / `nats.ObjectStore` API, as an inventory for migrating to the `jetstream` package |

Every default-on rule fires only on constants and direct calls, and every rule that
mirrors a server or client validation cites the function it mirrors in its spec under
`openspec/specs/`. Each rule handles both the `jetstream` types and their legacy
`nats` twins.

## Corpus

Unit tests prove a rule fires on code written to trigger it. The corpus proves it stays
quiet on code written for other reasons. `make corpus` lints pinned commits of:

- **nats.go** (`examples/`, `test/`) — the client's own examples and test suite
- **nats-server** (`server/`, `test/`) — its tests build every invalid config the server
  rejects, which makes it the best oracle for the config rules
- **natscli**, **nack**, **nex** — real tooling built on nats.go
- **orbit.go** (every module's `test/`) — dense use of the new `jetstream` API
- **synadia-io/connect**, **go-choria**, **knative eventing-natss** — third-party users
  of both APIs

`main` passes the corpus with every finding triaged into `scripts/corpus.expected`, each
tagged `TP` (a real defect) or `FP` with the reason it is accepted. At the time of
writing every finding is a test that deliberately builds the invalid thing to assert its
rejection; no application code in the corpus produces a diagnostic. Two false positives
were found by the corpus and fixed before release. New findings fail the CI job until
triaged.

## Develop

```sh
make test      # downloads the pinned nats.go for testdata, then go test ./...
make lint      # gofmt, go vet, staticcheck, misspell, license headers
make generate  # regenerate the nats.go-derived tables and docs/rules.md
make corpus    # lint the pinned repositories and diff against scripts/corpus.expected
```

Rule tests compile against the real nats.go API through the `testdata` module. Design
rationale and the rule catalog are in [`docs/design.md`](docs/design.md); work is planned
as OpenSpec changes under `openspec/`, with each rule's behavior specified in
`openspec/specs/rule-<name>/spec.md`.

## License

Apache 2.0, see [LICENSE](LICENSE).
