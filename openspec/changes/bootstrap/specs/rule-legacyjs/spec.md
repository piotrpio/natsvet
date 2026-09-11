## Purpose

legacyjs reports every use of the legacy JetStream API in the nats package (nats.JetStreamContext, nats.KeyValue, nats.ObjectStore and their options, configs and message methods) so that a codebase can inventory what a migration to the jetstream package has to touch. nats.go does not mark this API deprecated, so no generic deprecation check sees it. The rule reports facts, never fixes, and is off by default.

## ADDED Requirements

### Requirement: Rule is opt-in
The rule SHALL report nothing unless enabled with `-legacyjs.enable`.

#### Scenario: Not enabled
- **WHEN** `natsvet ./...` is run on a package that calls `nc.JetStream()`
- **THEN** no `legacyjs` diagnostic is reported

#### Scenario: Enabled
- **WHEN** `natsvet -legacyjs.enable ./...` is run on the same package
- **THEN** the call is reported

### Requirement: Every use site of a legacy symbol is reported
When enabled, the rule SHALL report one diagnostic per use of a symbol in the legacy symbol table: a reference to a legacy type in a declaration, parameter, field, conversion or assertion; a call of a legacy function or option constructor; and a call of a legacy method, identified by the type that declares the method (for an interface method reached through an embedding interface, the embedded interface that declares it). The message SHALL be `legacy JetStream API: <qualified symbol>; see the jetstream package`, where the qualified symbol is `nats.<Type>`, `nats.<Func>` or `nats.<Type>.<Method>`.

#### Scenario: Entry point
- **WHEN** code calls `nc.JetStream()`
- **THEN** the rule reports `legacy JetStream API: nats.Conn.JetStream; see the jetstream package`

#### Scenario: Method on legacy interface
- **WHEN** code calls `js.Publish("orders", data)` where `js` is a `nats.JetStreamContext`
- **THEN** the rule reports `legacy JetStream API: nats.JetStream.Publish; see the jetstream package`, naming the interface that declares `Publish`

#### Scenario: Type in a declaration
- **WHEN** code declares `var kv nats.KeyValue` or a struct field of type `nats.JetStreamContext`
- **THEN** the rule reports the type at that position

#### Scenario: Option constructor
- **WHEN** code passes `nats.Durable("worker")` or `nats.MaxWait(time.Second)` to a legacy call
- **THEN** the rule reports the option constructor in addition to the call

#### Scenario: Legacy message method
- **WHEN** code calls `msg.Ack()` on a `*nats.Msg`
- **THEN** the rule reports `legacy JetStream API: nats.Msg.Ack; see the jetstream package`

#### Scenario: Pull fetch
- **WHEN** code calls `sub.Fetch(10)` on a `*nats.Subscription`
- **THEN** the rule reports `nats.Subscription.Fetch`

#### Scenario: Legacy config struct
- **WHEN** code builds a `nats.StreamConfig{...}` literal
- **THEN** the rule reports the type once, at the literal

### Requirement: The jetstream package and shared symbols are not reported
The rule SHALL NOT report symbols from the `jetstream` package, core NATS symbols, or constants and variables of the nats package (header names, error sentinels), even when those are declared in the legacy source files.

#### Scenario: jetstream twin
- **WHEN** code calls `jetstream.New(nc)` and `js.Publish(ctx, "orders", data)` on a `jetstream.JetStream`
- **THEN** the rule reports nothing

#### Scenario: Header constant from a legacy file
- **WHEN** code uses `nats.MsgIdHdr`
- **THEN** the rule reports nothing

#### Scenario: Core API
- **WHEN** code calls `nc.Publish`, `nc.SubscribeSync` or `sub.NextMsg`
- **THEN** the rule reports nothing
