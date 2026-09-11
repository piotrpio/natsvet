## Purpose

headerkey reports header keys that differ only in case from a header nats.go defines. nats.go headers are case-preserving and lookups are exact map lookups, unlike net/http which canonicalizes keys, so `msg.Header.Get("nats-msg-id")` returns "" on a message that carries `Nats-Msg-Id`, and `Set("nats-msg-id", v)` publishes a header the server does not recognize.

## ADDED Requirements

### Requirement: Miscased known header keys are reported
The rule SHALL report a constant string key that equals a known `Nats-*` header (per the generated header table) case-insensitively but not exactly, when the key is passed to `Get`, `Set`, `Add`, `Values` or `Del` on `nats.Header`, to `Get` or `Values` on `micro.Headers`, or used to index a `nats.Header` value directly. Reaching the header through `msg.Header`, `jetstream.Msg.Headers()` or `micro.Request.Headers()` SHALL make no difference. The message SHALL be `header key "<key>" does not match "<Canonical>"; nats.go header lookups are case-sensitive`.

#### Scenario: Lowercase key on nats.Header.Get
- **WHEN** code calls `msg.Header.Get("nats-msg-id")`
- **THEN** the rule reports `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`

#### Scenario: Key via jetstream message
- **WHEN** code calls `m.Headers().Get("NATS-STREAM")` on a `jetstream.Msg`
- **THEN** the rule reports the key against `Nats-Stream`

#### Scenario: Key via micro request
- **WHEN** code calls `req.Headers().Get("nats-service-error")` on a `micro.Request`
- **THEN** the rule reports the key against `Nats-Service-Error`

#### Scenario: Direct map index
- **WHEN** code evaluates `h["nats-msg-id"]` where `h` is a `nats.Header`
- **THEN** the rule reports the key

#### Scenario: Named constant key
- **WHEN** code declares `const id = "nats-msg-id"` and calls `h.Get(id)`
- **THEN** the rule reports the key at the call site

#### Scenario: Exact-case known header
- **WHEN** code calls `h.Get("Nats-Msg-Id")` or `h.Get(nats.MsgIdHdr)`
- **THEN** the rule reports nothing

#### Scenario: User-defined header
- **WHEN** code calls `h.Set("X-My-Header", v)` or `h.Get("x-request-id")`
- **THEN** the rule reports nothing

#### Scenario: Non-constant key
- **WHEN** code calls `h.Get(keyFromConfig)` where the argument is not a compile-time constant
- **THEN** the rule reports nothing

### Requirement: The fix replaces the key with a certain equivalent
The rule SHALL offer exactly one suggested fix per diagnostic. The replacement SHALL be the qualified constant from a package that is already imported in the file and defines a constant for the header, preferring `jetstream`, then `nats`, then `micro`; when no imported package defines one, the correctly cased string literal. The fix SHALL replace only the key expression and SHALL never add an import.

#### Scenario: jetstream imported
- **WHEN** the file imports `github.com/nats-io/nats.go/jetstream` and calls `h.Get("nats-msg-id")`
- **THEN** the fix rewrites the argument to `jetstream.MsgIDHeader`

#### Scenario: Only nats imported
- **WHEN** the file imports only `github.com/nats-io/nats.go` and calls `h.Get("nats-msg-id")`
- **THEN** the fix rewrites the argument to `nats.MsgIdHdr`

#### Scenario: Both imported
- **WHEN** the file imports both packages
- **THEN** the fix uses the `jetstream` constant

#### Scenario: Header has no constant in the imported package
- **WHEN** the file imports only `github.com/nats-io/nats.go` and calls `h.Get("nats-schedule")`, for which only `jetstream` defines a constant
- **THEN** the fix rewrites the argument to the literal `"Nats-Schedule"`

#### Scenario: micro header with micro imported
- **WHEN** the file imports `github.com/nats-io/nats.go/micro` and calls `req.Headers().Get("nats-service-error")`
- **THEN** the fix rewrites the argument to `micro.ErrorHeader`

#### Scenario: Named constant key is not rewritten at its declaration
- **WHEN** the key is a named constant declared elsewhere
- **THEN** the fix replaces the argument expression at the call site and leaves the constant declaration untouched
