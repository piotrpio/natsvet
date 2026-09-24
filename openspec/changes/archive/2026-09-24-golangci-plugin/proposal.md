## Why

Every rule ships, but the only ways to run them are the standalone binary and `go vet -vettool`. The distribution channel the design names as "the real reach" (`docs/design.md` §5) is golangci-lint, and the upstream built-in-linter submission cannot happen before the repository move settles the module path. golangci-lint's module plugin system closes that gap now: a `natsvet` package that registers with `plugin-module-register`, a `.custom-gcl.yml`, and `golangci-lint custom` give Synadia a `custom-gcl` binary with natsvet inside it today — which is the internal dogfooding the move is waiting on.

## What Changes

- Package `plugin` in the main module: `register.Plugin("natsvet", New)` in `init()`, `New` decoding a settings block of two string lists, `enable` (opt-in rules to turn on) and `disable` (default-on rules to turn off), rejecting unknown keys and unknown rule names; `BuildAnalyzers` returning the registry's default-on set minus `disable` plus `enable` with each opt-in rule's `enable` flag set; `GetLoadMode` returning type-info mode.
- `.custom-gcl.yml` at the repository root pointing at the checkout (`path: .`), so `golangci-lint custom` in a clone builds `bin/custom-gcl`; the README documents the `module`/`version` form users copy.
- `scripts/plugin.sh` and `make plugin`: build the custom binary, run it over the `testdata` module with a `testdata/.golangci.yml` that enables only `natsvet` with every opt-in rule on, run `bin/natsvet` with the same rules over the same module, normalize both outputs and diff them. Parity is the proof that registration, settings, load mode and every rule work through golangci-lint.
- A `plugin` CI job that installs the pinned golangci-lint and runs `make plugin`.
- README section for golangci-lint use; `docs/design.md` §2.1 amended for the second production dependency, §5 and §6 updated.

## Capabilities

### New Capabilities
- `golangci-plugin`: how natsvet plugs into golangci-lint — the plugin name, the settings block and its validation, the load mode, how findings are labeled, and output parity with the standalone binary.

### Modified Capabilities
- `testing`: adds the plugin parity check to the CI check set.
- `release`: a tag also yields the plugin — a `.custom-gcl.yml` pinning `version: vX.Y.Z` builds a custom binary whose rule set is the README's.

## Non-goals

- The upstream golangci-lint PR (built-in linter). It needs the final module path and the linter-name decision (`docs/design.md` §8 item 4); the design records what that PR consists of so it can be cut the day the path is final.
- Per-rule `//nolint` addressing. golangci-lint's `//nolint:natsvet` covers the whole linter, as `//nolint:govet` does; a rule is turned off for a module with `disable`, and the rule id is visible in every finding because golangci-lint prefixes the analyzer name when it differs from the linter name.
- A `.golangci.yml` for natsvet itself: natsvet does not use nats.go, so there is nothing to dogfood in this repository beyond the parity run.
- Exposing analyzer flags other than opt-in `enable` through settings. No rule has any today.
- Nested module for the plugin (a second tag and a pseudo-version bump on every rule change) — decided with the maintainer: the plugin lives in the main module and `docs/design.md` §2.1 gains a second, golangci-owned dependency whose own requirement is only `golang.org/x/tools`.

## Rule defaults

No rule is added or changed. The plugin preserves the registry: `Analyzers()` run unless listed in `disable`; `OptIn()` rules run only when listed in `enable`. Under golangci-lint the linter as a whole is enabled the usual way (`linters.enable: [natsvet]`).

## Shared helpers

None added to `internal/natsapi`. The plugin relies on `natsvet.Analyzers()` and `natsvet.OptIn()` and on each opt-in analyzer's `enable` flag (analyzer-framework: "Rules are registered as default-on or opt-in").

## Impact

- `go.mod` gains `github.com/golangci/plugin-module-register` (requires only `golang.org/x/tools`, at a version below ours). `cmd/natsvet` does not import it; `go install ...@latest` users see it in the module graph only.
- CI gains a third job that downloads golangci-lint (a binary via the official action) and builds a custom binary — roughly two minutes, on a public repository.
- Findings under golangci-lint read `<rule>: <message> (natsvet)`, e.g. `drain: Close immediately after Drain aborts the drain; wait for the ClosedHandler instead (natsvet)`; the standalone binary's output is unchanged.
- No behavior change for any existing invocation mode.
