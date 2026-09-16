## Why

Tier 1 is complete: ten default-on rules, `legacyjs`, 68 spec requirements, and a corpus that has caught two real false positives before users could. What is missing is everything that turns a repository into a release someone can install and trust: a corpus wide enough that "zero false positives" means something (today it is five nats.go-adjacent repositories; a parallel run over 17 third-party users found more), a toolchain dependency that can load projects on the newest Go (x/tools v0.43 fails on Go 1.27 modules with an internal error), a README that shows each rule's before/after and states the corpus guarantee, a way to ask the binary what version it is, and a tag. This change is the `v0.1.0` gate.

## What Changes

- Bump `golang.org/x/tools` to v0.49.0 (keeps the `go 1.25` directive; v0.50 would require Go 1.26 and drop a CI leg). This is the version that turns "internal error: package context without types" on a newer-Go project into a clear "requires newer Go" error.
- Widen the corpus with four pinned repositories: `nats-io/nats-server` (`server/` and `test/` at v2.14.7 — the best correctness oracle, its tests build every invalid config the server rejects), `choria-io/go-choria` (the densest third-party nats.go user found), `synadia-io/connect` (new `jetstream` API usage), and `knative-extensions/eventing-natss` (the repository whose code exposed the `Bind` false positive). Triage every finding.
- `natsvet -version` prints the module version and VCS revision from build info, so a bug report can name the build. `multichecker`'s `-V` is for the `go vet` cache and prints nothing useful to a person.
- README: a rule section with a before/after snippet per rule, the corpus list and what passing it means, install instructions pinned to the tag.
- `docs/design.md` §8 gains the follow-ups the runs surfaced: the `Bind`+subject mismatch heuristic, unknown `Nats-*` keys near a known header (`Nats-TTLSeconds`), nested-literal checks for the config rules, and the `main`/test `Drain` question already there.
- Tag `v0.1.0` after the corpus passes on the widened list; the push and the GitHub release are the maintainer's actions.

## Capabilities

### New Capabilities
- `release`: what a tagged release guarantees — the checks that must pass before a tag, the version the binary reports, and what `go install` at the tag yields.

### Modified Capabilities
- `testing`: the corpus requirement's repository list grows to the widened set, and the expected-file format gains the repository-count summary line the script already prints.
- `analyzer-framework`: adds the `-version` flag requirement.

## Non-goals

- New rules or rule changes. If triage of the widened corpus finds a false positive, it is fixed in this change only when the fix is a hook-table adjustment of the kind `callsite-rules` already made; anything larger becomes its own change and the finding is recorded as `FP` with reason until then.
- The golangci-lint module plugin and the upstream submission (`golangci-plugin`).
- Tier 2 rules (`lifecycle-rules`).
- The nested-literal config checks, the `headerkey` near-miss extension, and the `Bind`-mismatch heuristic themselves — recorded as follow-ups, not built.
- Moving the module path; the tag is under `github.com/piotrpio/natsvet` as the design doc allows before any announcement.
- A CHANGELOG file; the tag's GitHub release notes are written by the maintainer from the README rule table.

## Rule defaults

No rules are added or changed; all eleven keep their defaults (ten on, `legacyjs` opt-in).

## Shared helpers

None added. Reused as is.

## Impact

- `go.mod`/`go.sum`: x/tools v0.43.0 → v0.49.0 and its indirect requirements. Analyzer API used by the rules is stable across that range; tests confirm.
- `scripts/corpus.txt` and `scripts/corpus.expected` grow; the corpus job runs longer (nats-server's `server` package is the largest single package analyzed, about 16 seconds) and clones more, cached across runs.
- `cmd/natsvet`: a `-version` pre-check before `multichecker.Main`.
- README and `docs/design.md`: documentation only.
- The `v0.1.0` tag is the first thing anyone can depend on; after it, changing the module path costs consumers, so the path question in `docs/design.md` §8 becomes live at the next milestone.
