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
