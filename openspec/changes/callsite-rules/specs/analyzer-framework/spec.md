## ADDED Requirements

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
