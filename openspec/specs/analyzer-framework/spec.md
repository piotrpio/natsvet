# analyzer-framework Specification

## Purpose

The framework every natsvet rule plugs into: how rules are registered and enabled, how the single binary is invoked, how rules recognize nats.go symbols, and the diagnostic conventions and generated symbol tables all rules share.

## Requirements

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

### Requirement: Subject helpers match nats-server semantics
The shared subject helpers SHALL implement nats-server's `IsValidSubject`, `subjectIsLiteral`, `SubjectsCollide` and `subjectIsSubsetMatch` from `server/sublist.go` with identical results, verified by running the server's own test tables for those functions.

#### Scenario: Server test table
- **WHEN** the helper tests run
- **THEN** every case from nats-server's `TestSublistValidSubjects`, `TestSubjectIsLiteral`, `TestIsSubsetMatch` and `TestSublistSubjectCollide` tables passes unchanged

#### Scenario: Collision semantics
- **WHEN** `SubjectsCollide("orders.>", "orders.new")` and `SubjectsCollide("a.*.c", "a.b.>")` are evaluated
- **THEN** both are true, while `SubjectsCollide("orders.new", "orders.paid")` is false

### Requirement: Composite-literal field extraction sees through pointers and twins
The framework SHALL provide a way to obtain the keyed fields of a composite literal when its type is one of a set of nats.go struct types, treating `T{...}`, `&T{...}`, and unkeyed element literals inside `[]T{...}`, `[]*T{...}` and `map[K]T{...}` alike, and SHALL expose which of the named types matched so a rule can map legacy field names to their `jetstream` equivalents.

#### Scenario: Address-of literal
- **WHEN** a rule asks for the fields of `&jetstream.StreamConfig{Name: "x"}`
- **THEN** it receives `Name` mapped to the `"x"` expression and the matched type `jetstream.StreamConfig`

#### Scenario: Slice element literal
- **WHEN** a rule inspects `[]jetstream.ConsumerConfig{{Durable: "a.b"}}`
- **THEN** the inner element literal is reported as a `jetstream.ConsumerConfig` literal

#### Scenario: Positional literal
- **WHEN** a literal uses positional (unkeyed) fields
- **THEN** the extraction returns no fields, and rules report nothing for it

### Requirement: Typed constant extraction
The framework SHALL provide constant extraction for integer, duration and boolean expressions and for the string elements of a slice literal, resolving named constants and constant arithmetic, and SHALL identify which named enum constant of a nats.go package (for example `jetstream.AckNonePolicy` or its legacy twin `nats.AckNonePolicy`) a constant expression denotes.

#### Scenario: Duration arithmetic
- **WHEN** the expression is `50 * time.Millisecond`
- **THEN** the duration extractor returns 50ms

#### Scenario: Enum identity across packages
- **WHEN** the expression is `nats.AckNonePolicy` or `jetstream.AckNonePolicy`
- **THEN** the enum extractor identifies both as the `AckNonePolicy` value

#### Scenario: Non-constant
- **WHEN** the expression is a variable or a call
- **THEN** every extractor reports not-constant

### Requirement: Config literals are checked as they leave the function
The framework SHALL decide, for a config literal, which fields may have changed before the value leaves the enclosing function, so that config rules never report a field that a later statement may rewrite before submission. A direct call argument or return value SHALL have no masked fields. A literal bound to a variable SHALL mask the fields assigned before the first statement that hands the variable off (by-value argument to a concretely typed parameter, return, channel send, or pointer argument to a nats.go method); a by-value argument to an interface-typed parameter SHALL NOT count as a hand-off. A pointer escape to another call, a method call on the variable, a closure capture, or the absence of a hand-off in the block SHALL fall back to masking every field assigned anywhere in the function.

#### Scenario: Inline literal
- **WHEN** a config literal is a direct call argument
- **THEN** no field is masked, regardless of other assignments in the function

#### Scenario: Hand-off before assignment
- **WHEN** `cfg := T{...}` is followed by `use(cfg)` and then `cfg.X = ...`
- **THEN** no field is masked

#### Scenario: Assignment before hand-off
- **WHEN** `cfg := T{...}` is followed by `cfg.X = ...` (directly or inside a nested block) and then `use(cfg)`
- **THEN** `X` is masked and other fields are not

#### Scenario: Interface parameter is not a hand-off
- **WHEN** `cfg := T{...}` is followed by `log.Printf("%v", cfg)`, then `cfg.X = ...`, then `use(cfg)`
- **THEN** `X` is masked

#### Scenario: Pointer escape
- **WHEN** `cfg := T{...}` is followed by `helper(&cfg)` and later `cfg.X = ...`
- **THEN** every field assigned anywhere in the function is masked

#### Scenario: Value nested in a table
- **WHEN** the literal is an element of a composite literal bound to a variable that is only ranged over, with `elem.X = ...` in the loop
- **THEN** `X` is masked

### Requirement: Single-definition lookup
The framework SHALL provide a way to find, for an identifier that refers to a local variable of the enclosing function, the one expression the variable is defined from — the right-hand side of its only `:=`, `=` or `var` assignment in the function, with a multi-value call counted as the definition of every left-hand variable — and SHALL report no definition when the variable is a parameter, a package-level variable, is assigned more than once, or has its address taken anywhere in the function.

#### Scenario: Multi-value definition
- **WHEN** a function has `sub, err := nc.Subscribe("s", h)` and nothing else assigns `sub`
- **THEN** the lookup for `sub` returns the `nc.Subscribe(...)` call

#### Scenario: Reassigned
- **WHEN** a function assigns `sub` twice
- **THEN** the lookup for `sub` reports no definition

#### Scenario: Address taken
- **WHEN** a function has `m := &nats.Msg{}` and later `p := &m`
- **THEN** the lookup for `m` reports no definition

#### Scenario: Parameter
- **WHEN** the identifier is a function parameter
- **THEN** the lookup reports no definition

### Requirement: The binary reports its version
`natsvet -version` SHALL print the module version and, when available, the VCS revision and modification state from the binary's build information, and exit 0 without running any analysis.

#### Scenario: Installed at a tag or pseudo-version
- **WHEN** the binary was built by `go install ...@latest` (or at a tag once one exists) and `natsvet -version` is run
- **THEN** the output contains the module version Go recorded: the pseudo-version of `main`, or the tag

#### Scenario: Built from a checkout
- **WHEN** the binary was built with `go build` in a git checkout
- **THEN** the output contains the commit revision and `(modified)` when the tree was dirty

#### Scenario: Analysis flags untouched
- **WHEN** `natsvet -V=full` is run
- **THEN** the `go vet` cache-key output is unchanged
