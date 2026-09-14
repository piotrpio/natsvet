# natsvet

Static analysis for Go programs that use [nats.go](https://github.com/nats-io/nats.go).
natsvet turns runtime failures and lifecycle mistakes that nats.go users hit repeatedly —
server-rejected configs, misused subscriptions, miscased headers — into diagnostics at
build time, with automatic fixes where the rewrite is certain.

It is built on `golang.org/x/tools/go/analysis`, so one binary works standalone, as a
`go vet` tool, and as a `go fix` tool.

## Install

```sh
go install github.com/piotrpio/natsvet/cmd/natsvet@latest
```

## Use

```sh
natsvet ./...                 # report
natsvet -fix ./...            # apply suggested fixes
natsvet -fix -diff ./...      # show the fixes as a diff

go vet -vettool=$(which natsvet) ./...
go fix -fixtool=$(which natsvet) ./...      # Go 1.26+
```

Default-on rules can be turned off with `-<rule>=false`; opt-in rules are enabled with
`-<rule>.enable`.

## Rules

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
| `headerkey` | on | yes | a header key that differs only in case from a NATS header (`nats-msg-id` vs `Nats-Msg-Id`); nats.go header lookups are case-sensitive, so the lookup silently fails |
| `legacyjs` | opt-in | no | every use of the legacy `nats.JetStreamContext` / `nats.KeyValue` / `nats.ObjectStore` API, as an inventory for migrating to the `jetstream` package |

Every default-on rule only fires on constant expressions and aims for a false-positive
rate low enough to run unconfigured in CI. Rules are checked against a corpus of
real-world nats.go users before release (`make corpus`).

## Develop

```sh
make test      # downloads the pinned nats.go for testdata, then go test ./...
make lint      # gofmt, go vet, staticcheck, misspell, license headers
make generate  # regenerate the nats.go-derived tables after bumping testdata/go.mod
make corpus    # lint pinned real-world repositories and diff against scripts/corpus.expected
```

Rule tests compile against the real nats.go API through the `testdata` module. Design
rationale and the rule catalog are in [`docs/design.md`](docs/design.md); work is planned
as OpenSpec changes under `openspec/`.

## License

Apache 2.0, see [LICENSE](LICENSE).
