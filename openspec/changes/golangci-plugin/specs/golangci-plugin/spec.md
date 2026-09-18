## Purpose

How natsvet runs inside golangci-lint as a module plugin: the plugin registers under the name natsvet, takes a settings block that turns opt-in rules on and default-on rules off, asks for type information, and reports exactly what the standalone binary reports with the rule id prefixed to each finding.

## ADDED Requirements

### Requirement: The plugin registers as natsvet
The module SHALL expose a package that registers a golangci-lint module plugin named `natsvet` at package initialization, so that a `.custom-gcl.yml` naming the module (and that package as its import path) builds a golangci-lint binary in which `linters.settings.custom.natsvet` with `type: module` is a valid linter. The plugin SHALL request the type-information load mode, since every rule resolves nats.go symbols through `go/types`.

#### Scenario: Custom binary lists the linter
- **WHEN** `golangci-lint custom` is run with a `.custom-gcl.yml` that names the natsvet module and the plugin import path, and the resulting binary is run with a config that enables `natsvet`
- **THEN** the run succeeds and natsvet findings are reported

#### Scenario: Wrong config key
- **WHEN** the config declares the plugin under a key other than `natsvet` (for example `custom.natslint`)
- **THEN** golangci-lint fails at startup reporting that no plugin of that name is registered

### Requirement: Settings turn opt-in rules on and default-on rules off
The plugin SHALL accept a settings block with two optional string lists, `enable` and `disable`. `BuildAnalyzers` SHALL return every default-on rule not named in `disable`, plus every opt-in rule named in `enable` with that rule's `enable` flag set; an opt-in rule not named in `enable` SHALL NOT run and SHALL have its flag cleared, so building twice with different settings is deterministic. A name in either list that is not a registered rule, or a key other than `enable` and `disable`, SHALL make plugin construction fail with a message naming the offending name or key, so a typo fails the run rather than silently running the default set.

#### Scenario: No settings
- **WHEN** the config declares `custom.natsvet` with `type: module` and no `settings`
- **THEN** every default-on rule runs and no opt-in rule runs — the same set as `natsvet ./...`

#### Scenario: Opt-in rule enabled
- **WHEN** `settings.enable` is `[legacyjs]`
- **THEN** `legacyjs` findings are reported alongside the default-on rules'

#### Scenario: Default-on rule disabled
- **WHEN** `settings.disable` is `[drain]`
- **THEN** no `drain` finding is reported and every other default-on rule still runs

#### Scenario: Unknown rule name
- **WHEN** `settings.enable` is `[legacyj]`
- **THEN** golangci-lint fails at startup with an error naming `legacyj`

#### Scenario: Unknown settings key
- **WHEN** `settings` contains `enabled: [legacyjs]`
- **THEN** golangci-lint fails at startup with an error naming the unknown key

#### Scenario: Redundant names are accepted
- **WHEN** `settings.enable` names a default-on rule or `settings.disable` names an opt-in rule
- **THEN** construction succeeds and the rule set is unchanged by that entry

### Requirement: Findings carry the rule id and match the standalone binary
Under golangci-lint each finding SHALL be reported at the same file, line and column and with the same message as `natsvet` reports for the same package with the same rules enabled; golangci-lint prefixes the analyzer name, so the text reads `<rule>: <message>`. A parity script SHALL build the custom binary from the repository's `.custom-gcl.yml`, run it over the `testdata` module with every opt-in rule enabled, run `natsvet` with the same rules over the same module, normalize both outputs (position, rule-id prefix, linter-name suffix, sort) and fail on any difference.

#### Scenario: Parity over testdata
- **WHEN** the parity script runs on a clean checkout
- **THEN** the two normalized outputs are identical and the script exits 0

#### Scenario: A rule missing from the plugin
- **WHEN** a rule is registered in `Analyzers()` but the plugin does not return it
- **THEN** the parity script fails, listing the findings only the standalone binary produced

#### Scenario: Finding text
- **WHEN** the custom binary reports the `drain` adjacency case
- **THEN** the issue text is `drain: Close immediately after Drain aborts the drain; wait for the ClosedHandler instead` with `natsvet` as the linter name

### Requirement: Build from a checkout and from a version
The repository SHALL carry a `.custom-gcl.yml` that pins the golangci-lint version and points at the checkout (`path: .`), so `golangci-lint custom` in a clone produces `bin/custom-gcl` without any published version; the README SHALL document the form users copy, which names the module, the plugin import path and a `version` (a tag once one exists, otherwise a branch or commit).

#### Scenario: Checkout build
- **WHEN** `golangci-lint custom` runs at the repository root
- **THEN** `bin/custom-gcl` exists and `bin/custom-gcl linters` lists `natsvet` when a config enables it

#### Scenario: Version build before a tag
- **WHEN** a user's `.custom-gcl.yml` names the module with `version: main`
- **THEN** `golangci-lint custom` resolves the pseudo-version of `main` and builds a binary containing the current rule set
