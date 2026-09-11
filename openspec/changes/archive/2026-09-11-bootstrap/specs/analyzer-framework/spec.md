## Purpose

The framework every natsvet rule plugs into: how rules are registered and enabled, how the single binary is invoked, how rules recognize nats.go symbols, and the diagnostic conventions and generated symbol tables all rules share.

## ADDED Requirements

### Requirement: Rules are registered as default-on or opt-in
The tool SHALL expose two rule sets: default-on rules, which run unless disabled, and opt-in rules, which run only when enabled. Every rule SHALL have a unique lowercase identifier with no separators that serves as its analyzer name, flag prefix and diagnostic category.

#### Scenario: Default invocation runs default-on rules only
- **WHEN** `natsvet ./...` is run on a package that triggers both a default-on rule and an opt-in rule
- **THEN** only the default-on rule's diagnostics are reported

#### Scenario: Default-on rule disabled by flag
- **WHEN** `natsvet -<rule>=false ./...` is run on a package that triggers `<rule>`
- **THEN** no diagnostics from `<rule>` are reported

#### Scenario: Opt-in rule enabled by flag
- **WHEN** `natsvet -<rule>.enable ./...` is run on a package that triggers opt-in rule `<rule>`
- **THEN** its diagnostics are reported

### Requirement: One binary serves every invocation mode
The same binary SHALL report identical diagnostics when run directly, through `go vet -vettool`, and through `go fix -fixtool`, and SHALL apply suggested fixes with `-fix` and print them with `-diff`.

#### Scenario: go vet delegation
- **WHEN** `go vet -vettool=$(which natsvet) ./...` is run on a package with a known finding
- **THEN** the same diagnostic text and position are reported as with `natsvet ./...`

#### Scenario: Fix application
- **WHEN** `natsvet -fix ./...` is run on a package where a rule offers a suggested fix
- **THEN** the source file is rewritten with the fix and a subsequent run reports nothing for that site

### Requirement: nats.go symbols are matched by import path
Rules SHALL identify nats.go types, functions and methods by the declaring package's import path (`github.com/nats-io/nats.go`, its `jetstream` and `micro` subpackages), including when the package is vendored, and SHALL never match same-named symbols from other packages.

#### Scenario: Vendored nats.go
- **WHEN** a package uses `vendor/github.com/nats-io/nats.go` and triggers a rule
- **THEN** the diagnostic is reported as for a module-cache import

#### Scenario: Unrelated package with same identifiers
- **WHEN** a package defines its own `Header` type with a `Get` method and calls `h.Get("nats-msg-id")`
- **THEN** no diagnostic is reported

### Requirement: Diagnostics follow one message convention
Each diagnostic SHALL carry the rule identifier as its category and a single-sentence message that is lowercase, has no trailing period, and states the runtime consequence, not only the matched pattern.

#### Scenario: Message shape
- **WHEN** any rule reports a diagnostic
- **THEN** the message has no trailing period, starts with a lowercase letter or a quoted/identifier token, and names the runtime effect (an error value, a panic, a silent misbehavior)

### Requirement: Known NATS header table is generated from the pinned nats.go
The tool SHALL maintain a table of every exported string constant whose value starts with `Nats-` in the `nats`, `jetstream` and `micro` packages of the pinned nats.go, mapping each header value to the qualified constant names that define it. The table SHALL be generated from the nats.go source, not maintained by hand.

#### Scenario: Header with constants in two packages
- **WHEN** the table is queried for `Nats-Msg-Id`
- **THEN** it returns both `nats.MsgIdHdr` and `jetstream.MsgIDHeader`

#### Scenario: Header with a constant in one package
- **WHEN** the table is queried for `Nats-Schedule`
- **THEN** it returns only the `jetstream` constant

#### Scenario: Unknown header
- **WHEN** the table is queried for `X-Request-Id`
- **THEN** it returns no match

### Requirement: Legacy JetStream symbol table is generated from the pinned nats.go
The tool SHALL maintain a table of the legacy JetStream API surface: every exported type, function and method declared in the `nats` package files `js.go`, `jsm.go`, `jserrors.go`, `kv.go` and `object.go` of the pinned nats.go, keyed by qualified name (`nats.JetStreamContext`, `nats.KeyValue.Put`, `nats.Conn.JetStream`). Exported constants and variables from those files SHALL NOT be in the table, since header constants and error sentinels are valid with either API.

#### Scenario: Legacy type and method
- **WHEN** the table is queried for `nats.JetStreamContext` and for `nats.Subscription.Fetch`
- **THEN** both are present

#### Scenario: Header constant declared in a legacy file
- **WHEN** the table is queried for `nats.MsgIdHdr`
- **THEN** it is absent

#### Scenario: Core API
- **WHEN** the table is queried for `nats.Conn.Publish`
- **THEN** it is absent
