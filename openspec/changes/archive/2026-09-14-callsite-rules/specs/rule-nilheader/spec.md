## Purpose

nilheader reports Header.Set, Header.Add or a direct Header[key] = assignment on a nats.Msg built as a composite literal without a Header. nats.Header.Set and Add are plain map writes, nats.NewMsg allocates the map and a literal does not, so the call panics with an assignment to a nil map.

## ADDED Requirements

### Requirement: Header writes on a header-less message literal
Mirrors `Header.Set`/`Header.Add` in nats.go (`h[key] = ...`, no nil check) and `NewMsg` (allocates `Header`). The rule SHALL report a call to `Set` or `Add` on `<v>.Header` where `<v>` is a local variable with a single definition in the enclosing function whose value is a `nats.Msg` composite literal (value or `&`) with no `Header` key, and no statement in the function assigns `<v>.Header`. The message SHALL be `Header.<Method> on a nats.Msg literal without Header panics (nil map); use nats.NewMsg or set Header: nats.Header{}`. Category `nilheader`. No fix.

#### Scenario: Literal then Set
- **WHEN** code has `m := &nats.Msg{Subject: "s"}` followed by `m.Header.Set("X-Id", "1")`
- **THEN** the rule reports `Header.Set on a nats.Msg literal without Header panics (nil map); use nats.NewMsg or set Header: nats.Header{}`

#### Scenario: Value literal then Add
- **WHEN** code has `m := nats.Msg{Subject: "s"}` followed by `m.Header.Add("X-Id", "1")`
- **THEN** the rule reports the `Add` message

#### Scenario: NewMsg
- **WHEN** code has `m := nats.NewMsg("s")` followed by `m.Header.Set("X-Id", "1")`
- **THEN** the rule reports nothing

#### Scenario: Literal with a header
- **WHEN** code has `m := &nats.Msg{Subject: "s", Header: nats.Header{}}` followed by `m.Header.Set("X-Id", "1")`
- **THEN** the rule reports nothing

#### Scenario: Header assigned later
- **WHEN** code has `m := &nats.Msg{Subject: "s"}`, then `m.Header = nats.Header{}`, then `m.Header.Set("X-Id", "1")`
- **THEN** the rule reports nothing

#### Scenario: Nil-safe methods
- **WHEN** code has `m := &nats.Msg{Subject: "s"}` followed by `m.Header.Get("X-Id")`, `m.Header.Values("X-Id")` or `m.Header.Del("X-Id")`
- **THEN** the rule reports nothing

#### Scenario: Not a single definition
- **WHEN** `m` is a parameter, or is assigned twice in the function
- **THEN** the rule reports nothing

### Requirement: Direct map writes on a header-less message literal
Under the same conditions, the rule SHALL report an assignment statement whose left-hand side is `<v>.Header[<key>]`, with `assignment to <v>.Header on a nats.Msg literal without Header panics (nil map); use nats.NewMsg or set Header: nats.Header{}`. Reads (`<v>.Header[<key>]` as a value), `len` and `delete` SHALL NOT be reported.

#### Scenario: Index assignment
- **WHEN** code has `m := &nats.Msg{Subject: "s"}` followed by `m.Header["X-Id"] = []string{"1"}`
- **THEN** the rule reports the assignment message

#### Scenario: Index read and delete
- **WHEN** code has `m := &nats.Msg{Subject: "s"}` followed by `_ = m.Header["X-Id"]` and `delete(m.Header, "X-Id")`
- **THEN** the rule reports nothing
