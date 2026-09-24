## ADDED Requirements

### Requirement: Subject transform validation matches nats-server
The shared helpers SHALL implement nats-server's `ValidateMapping` from `server/sublist.go` and the error paths of the non-strict `NewSubjectTransform` from `server/subject_transform.go` (including `indexPlaceHolders` and the mapping-function expressions), returning an error exactly when the server does and with the server's error text, where an empty source means `>` and an empty destination is valid. They SHALL be verified by running the server's own test tables for those functions.

#### Scenario: Server test tables
- **WHEN** the helper tests run
- **THEN** every case of nats-server's `TestValidateDestinationSubject` and every non-strict `shouldErr`/`shouldBeOK` case of `TestSubjectTransforms` pass unchanged

#### Scenario: Error text
- **WHEN** the mapping from `events.*` to `events.{{split(3,1)}}` is validated
- **THEN** the error text is `invalid mapping destination: wildcard index out of range in {{split(3,1)}}: [3]`

#### Scenario: Valid mapping
- **WHEN** the mapping from `orders.*` to `archive.{{wildcard(1)}}` is validated
- **THEN** no error is returned

## MODIFIED Requirements

### Requirement: Composite-literal field extraction sees through pointers and twins
The framework SHALL provide a way to obtain the keyed fields of a composite literal when its type is one of a set of nats.go struct types, treating `T{...}`, `&T{...}`, and unkeyed element literals inside `[]T{...}`, `[]*T{...}` and `map[K]T{...}` alike, and SHALL expose which of the named types matched so a rule can map legacy field names to their `jetstream` equivalents. For a field of such a literal, it SHALL also provide the composite literals the field holds — the literal of a struct or pointer field written as `T{...}` or `&T{...}`, or the element literals of a slice literal — together with whether every element is a literal.

#### Scenario: Address-of literal
- **WHEN** a rule asks for the fields of `&jetstream.StreamConfig{Name: "x"}`
- **THEN** it receives `Name` mapped to the `"x"` expression and the matched type `jetstream.StreamConfig`

#### Scenario: Slice element literal
- **WHEN** a rule inspects `[]jetstream.ConsumerConfig{{Durable: "a.b"}}`
- **THEN** the inner element literal is reported as a `jetstream.ConsumerConfig` literal

#### Scenario: Positional literal
- **WHEN** a literal uses positional (unkeyed) fields
- **THEN** the extraction returns no fields, and rules report nothing for it

#### Scenario: Nested literals of a slice field
- **WHEN** a rule asks for the literals held by `Sources` in `jetstream.StreamConfig{Sources: []*jetstream.StreamSource{{Name: "A"}, src}}`
- **THEN** it receives the one element literal and is told that not every element is a literal

#### Scenario: Nested literal of a pointer field
- **WHEN** a rule asks for the literals held by `Mirror` in `jetstream.StreamConfig{Mirror: m}`
- **THEN** it receives no literal and is told that not every value is a literal

### Requirement: Config literals are checked as they leave the function
The framework SHALL decide, for a config literal, which fields may have changed before the value leaves the enclosing function, so that config rules never report a field that a later statement may rewrite before submission. A direct call argument or return value SHALL have no masked fields. A literal bound to a variable SHALL mask the fields assigned before the first statement that hands the variable off (by-value argument to a concretely typed parameter, return, channel send, or pointer argument to a nats.go method); a by-value argument to an interface-typed parameter SHALL NOT count as a hand-off. A pointer escape to another call, a method call on the variable, a closure capture, or the absence of a hand-off in the block SHALL fall back to masking every field assigned anywhere in the function. An assignment to an element of a field (`x.F[i] = ...`) SHALL count as assigning `F`.

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

#### Scenario: Indexed element assignment
- **WHEN** `cfg := T{...}` is followed by `cfg.Sources[0] = src` and then `use(cfg)`
- **THEN** `Sources` is masked
