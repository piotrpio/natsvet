## Why

nats.go users hit the same deterministic runtime failures and lifecycle mistakes over and over; the maintainers see them in support and there is no tooling that catches them before the program runs. `docs/design.md` lays out a `go/analysis` linter to close that gap. Before any rule catalog can be built, three infrastructure bets have to be proven end to end on a real rule: compiling test packages against the real nats.go API in `analysistest` module mode, applying `SuggestedFix`es through golden files and the `-fix` path, and shipping one binary that serves as standalone tool, `go vet -vettool` and `go fix -fixtool`. This change makes those bets concrete and gives every later change a working skeleton to add rules to.

## What Changes

- New Go module `github.com/piotrpio/natsvet` with the only production dependency `golang.org/x/tools`, Apache 2.0 license, `Makefile`, CI workflow and README skeleton.
- Analyzer registry (`Analyzers()` for default-on rules, `OptIn()` for opt-in rules) and `cmd/natsvet` built on `multichecker`, so `natsvet ./...`, `natsvet -fix`, `go vet -vettool` and `go fix -fixtool` all work from the first release.
- `internal/natsapi` with the helpers these two rules need: package matching keyed by import path (`Core`, `JetStream`, `Micro`; orbit.go modules slot in later), callee and method matching, constant-string extraction, and two generated tables — every `Nats-` header constant and the legacy JetStream API symbol set — produced from the pinned nats.go through `go/packages` and guarded by a regenerate-and-diff test.
- `testdata` module requiring the pinned nats.go so rule tests compile against the real API; one test package per rule with `// want` comments and `.golden` files.
- Rule `headerkey` (default-on): flags a header key constant that matches a known `Nats-*` header case-insensitively but not exactly, and fixes it to the qualified constant or the correctly cased literal. Proves the fix path.
- Rule `legacyjs` (opt-in via `-legacyjs.enable`): reports every use of the legacy `nats.JetStreamContext` / `nats.KeyValue` / `nats.ObjectStore` API surface. Proves the opt-in flag path and is the migration inventory (`docs/design.md` §3.4).
- Corpus script `scripts/corpus.sh` with the initial pinned repository list and an empty triaged `scripts/corpus.expected`; wired as a separate CI job. Establishes the false-positive gate every later rule group closes with.

## Capabilities

### New Capabilities
- `analyzer-framework`: analyzer registration, opt-in flag convention, the `natsvet` binary and its three invocation modes, `internal/natsapi` helper contracts, generated symbol tables, and the diagnostic/message/license conventions every rule follows.
- `testing`: `analysistest` in module mode against the real nats.go, golden-file fix tests, the generated-table drift test, and the corpus false-positive gate (`corpus.txt`, `corpus.expected`, triage tags, CI job).
- `rule-headerkey`: the `headerkey` rule — detection, message, fix selection, negative cases.
- `rule-legacyjs`: the `legacyjs` rule — the legacy symbol set, what counts as a use, message, opt-in behavior.

### Modified Capabilities
None; there are no existing specs.

## Non-goals

- Any other rule from the catalog; they arrive in `config-rules` and `callsite-rules`.
- Migration rewrites (legacy → `jetstream`). `legacyjs` only reports.
- Helpers no rule in this change needs (`CompositeFields`, subject helpers, KV regexes, `SingleDefinition`, `EnclosingFunc`): built with the first rule that uses them.
- golangci-lint plugin configuration; a later change.
- Full corpus triage and the v0.1 tag; this change lands the script and the initial expected file, `corpus-gate-v0.1` does the triage.
- orbit.go rules of any kind. orbit.go appears only as a corpus target.

## Rule defaults

- `headerkey`: default-on.
- `legacyjs`: opt-in (`-legacyjs.enable`).

## Shared helpers

Added to `internal/natsapi` by this change: `Pkg` type and constants, `IsPkg`, `Callee`, `IsMethod`, `ConstString`, `Headers` table, `LegacySymbols` table, and the generator that produces the two tables. Nothing is reused; this is the first change.

## Impact

- New repository content only; nothing depends on it yet.
- Test-time dependency on the pinned nats.go through `testdata/go.mod`; downloaded by `make test` before `go test`. Note for CI and sandboxed environments: `analysistest` runs with `GOPROXY=off`, so the download step must run with network access first.
- Bumping the pinned nats.go fails the build until the generated tables are regenerated; that is the intended drift signal.
- Corpus job clones pinned commits of nats.go, natscli, nack, nex and orbit.go; network access in that CI job only.
