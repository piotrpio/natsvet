## ADDED Requirements

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
The framework SHALL decide, for a config literal, which fields may have changed before the value leaves the enclosing function, so that config rules never report a field that a later statement may rewrite before submission. A direct call argument or return value SHALL have no masked fields. A literal bound to a variable SHALL mask the fields assigned before the first statement that hands the variable off (by-value call argument, return, channel send, or pointer argument to a nats.go method). A pointer escape to another call, a method call on the variable, a closure capture, or the absence of a hand-off in the block SHALL fall back to masking every field assigned anywhere in the function.

#### Scenario: Inline literal
- **WHEN** a config literal is a direct call argument
- **THEN** no field is masked, regardless of other assignments in the function

#### Scenario: Hand-off before assignment
- **WHEN** `cfg := T{...}` is followed by `use(cfg)` and then `cfg.X = ...`
- **THEN** no field is masked

#### Scenario: Assignment before hand-off
- **WHEN** `cfg := T{...}` is followed by `cfg.X = ...` (directly or inside a nested block) and then `use(cfg)`
- **THEN** `X` is masked and other fields are not

#### Scenario: Pointer escape
- **WHEN** `cfg := T{...}` is followed by `helper(&cfg)` and later `cfg.X = ...`
- **THEN** every field assigned anywhere in the function is masked

#### Scenario: Value nested in a table
- **WHEN** the literal is an element of a composite literal bound to a variable that is only ranged over, with `elem.X = ...` in the loop
- **THEN** `X` is masked
