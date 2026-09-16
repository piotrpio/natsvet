## Why

Tier 1 is complete: ten default-on rules, `legacyjs`, 68 spec requirements, and a corpus that has caught two real false positives before users could. What is missing is everything that makes the repository something a maintainer can dogfood and a colleague can install and trust: a corpus wide enough that "zero false positives" means something (today it is five nats.go-adjacent repositories; a parallel run over 17 third-party users found more), a toolchain dependency that can load projects on the newest Go (x/tools v0.43 fails on Go 1.27 modules with an internal error), a README that shows each rule's before/after and states the corpus guarantee, and a way to ask the binary what build it is. The tag itself waits: a tag under `github.com/piotrpio/natsvet` makes the module path sticky, and the path is decided by the repository move, which follows internal dogfooding. This change makes the repository release-ready; the release is a later, small change.

## What Changes

- Bump `golang.org/x/tools` to v0.49.0 (keeps the `go 1.25` directive; v0.50 would require Go 1.26 and drop a CI leg). This is the version that turns "internal error: package context without types" on a newer-Go project into a clear "requires newer Go" error.
- Widen the corpus with four pinned repositories: `nats-io/nats-server` (`server/` and `test/` at v2.14.7 — the best correctness oracle, its tests build every invalid config the server rejects), `choria-io/go-choria` (the densest third-party nats.go user found), `synadia-io/connect` (new `jetstream` API usage), and `knative-extensions/eventing-natss` (the repository whose code exposed the `Bind` false positive). Triage every finding.
- `natsvet -version` prints the module version and VCS revision from build info, so a bug report can name the build. `multichecker`'s `-V` is for the `go vet` cache and prints nothing useful to a person.
- `docs/rules.md` generated from the analyzers' `Doc` strings (`go generate` + a drift test, like the nats.go tables), so the rule documentation users read is exactly what golangci-lint will show; README keeps the short table, links to it, adds the corpus list and what passing it means, and install instructions via `@latest`.
- `docs/design.md` §8 gains the follow-ups the runs surfaced: the `Bind`+subject mismatch heuristic, unknown `Nats-*` keys near a known header (`Nats-TTLSeconds`), nested-literal checks for the config rules, and the `main`/test `Drain` question already there.
- Record in `docs/design.md` that the first tag follows the repository move, and that the move follows internal dogfooding.

## Capabilities

### New Capabilities
- `release`: what a tagged release will guarantee when one is made — the checks that must pass before a tag, the version the binary reports, and what `go install` at a tag yields. Defined now so the gates are exercised on `main` before any tag exists.

### Modified Capabilities
- `testing`: the corpus requirement's repository list grows to the widened set, and the expected-file format gains the repository-count summary line the script already prints.
- `analyzer-framework`: adds the `-version` flag requirement.

## Non-goals

- New rules or rule changes. If triage of the widened corpus finds a false positive, it is fixed in this change only when the fix is a hook-table adjustment of the kind `callsite-rules` already made; anything larger becomes its own change and the finding is recorded as `FP` with reason until then.
- The golangci-lint module plugin and the upstream submission (`golangci-plugin`).
- Tier 2 rules (`lifecycle-rules`).
- The nested-literal config checks, the `headerkey` near-miss extension, and the `Bind`-mismatch heuristic themselves — recorded as follow-ups, not built.
- Tagging. No `v0.1.0` in this change; the tag follows the repository move.
- Moving the module path; that is the maintainer's decision after dogfooding.
- A CHANGELOG file.

## Rule defaults

No rules are added or changed; all eleven keep their defaults (ten on, `legacyjs` opt-in).

## Shared helpers

None added to `internal/natsapi`. New `internal/docgen` (rule documentation from `Doc` strings) with its own drift test.

## Impact

- `go.mod`/`go.sum`: x/tools v0.43.0 → v0.49.0 and its indirect requirements. Analyzer API used by the rules is stable across that range; tests confirm.
- `scripts/corpus.txt` and `scripts/corpus.expected` grow; the corpus job runs longer (nats-server's `server` package is the largest single package analyzed, about 16 seconds) and clones more, cached across runs.
- `cmd/natsvet`: a `-version` pre-check before `multichecker.Main`.
- README and `docs/design.md`: documentation only. Install instructions use `@latest`, which resolves to a pseudo-version of `main` and works without a tag.
