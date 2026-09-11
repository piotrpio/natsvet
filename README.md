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
