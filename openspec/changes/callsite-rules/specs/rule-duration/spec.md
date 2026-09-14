## Purpose

duration reports an untyped integer constant passed where nats.go, jetstream or micro expects a time.Duration. Go converts it silently, so nc.Request("s", nil, 5) waits five nanoseconds and AckWait: 30 acknowledges in thirty nanoseconds; the intended unit was almost certainly seconds or milliseconds.

## ADDED Requirements

### Requirement: Untyped constants in Duration positions are reported
The rule SHALL report an expression whose value is an integer constant `v` with `0 < v < 1000000` (one millisecond) and whose expression contains no typed constant, conversion or call — a literal, an identifier resolving to an untyped constant, or arithmetic over those — when it is passed as an argument whose parameter type is `time.Duration` to a function or method declared in `github.com/nats-io/nats.go`, its `jetstream` or `micro` package, or assigned to a `time.Duration` field (or element of a `[]time.Duration` field) in a composite literal of a struct type declared in one of those packages. The message SHALL be `duration <v> for <name> is <d>; multiply by a time unit such as time.Second or time.Millisecond`, where `<name>` is the parameter's field or option name when known (the field name for a literal, the function name for an option constructor like `nats.Timeout`, the method name otherwise) and `<d>` is Go's duration formatting of `<v>` nanoseconds. Category `duration`. No fix.

#### Scenario: Request timeout
- **WHEN** code calls `nc.Request("s", nil, 5)`
- **THEN** the rule reports `duration 5 for Request is 5ns; multiply by a time unit such as time.Second or time.Millisecond` at the argument

#### Scenario: Option constructor
- **WHEN** code calls `nats.Connect(url, nats.Timeout(10))`
- **THEN** the rule reports the argument to `nats.Timeout`

#### Scenario: Config field
- **WHEN** code writes `jetstream.ConsumerConfig{AckWait: 30}`
- **THEN** the rule reports `duration 30 for AckWait is 30ns; ...` at the value

#### Scenario: Legacy and micro surfaces
- **WHEN** code writes `nats.StreamConfig{MaxAge: 3600}` or calls `sub.NextMsg(500)`
- **THEN** the rule reports each value

#### Scenario: Slice element
- **WHEN** code writes `jetstream.ConsumerConfig{BackOff: []time.Duration{1, 2}}`
- **THEN** the rule reports each element

#### Scenario: Named untyped constant
- **WHEN** code declares `const timeout = 5` and calls `nc.Request("s", nil, timeout)`
- **THEN** the rule reports the argument

#### Scenario: Arithmetic over untyped constants
- **WHEN** code calls `nc.Request("s", nil, 2*5)`
- **THEN** the rule reports the argument as `duration 10 for Request is 10ns; ...`

#### Scenario: Typed unit
- **WHEN** code calls `nc.Request("s", nil, 500*time.Microsecond)` or writes `AckWait: 500 * time.Millisecond`
- **THEN** the rule reports nothing, although the value is below 1ms

#### Scenario: Zero and negative
- **WHEN** code writes `MaxAge: 0` or `MaxAge: -1`
- **THEN** the rule reports nothing

#### Scenario: Large raw value
- **WHEN** code writes `AckWait: 30000000000`
- **THEN** the rule reports nothing (at or above 1ms the value is taken as deliberate nanoseconds)

#### Scenario: Explicit conversion
- **WHEN** code calls `nc.Request("s", nil, time.Duration(n))`
- **THEN** the rule reports nothing

#### Scenario: Non-nats callee
- **WHEN** code calls `time.Sleep(5)` or a user function with a `time.Duration` parameter with `5`
- **THEN** the rule reports nothing

#### Scenario: Non-Duration parameter
- **WHEN** code calls `sub.AutoUnsubscribe(5)` (an `int` parameter)
- **THEN** the rule reports nothing
