# rule-ctxdeadline Specification

## Purpose

ctxdeadline reports context.Background() or context.TODO() passed directly to the nats.go core methods that need a deadline. FlushWithContext returns ErrNoDeadlineContext for such a context every time; RequestWithContext, RequestMsgWithContext and NextMsgWithContext accept it and then block forever whenever a responder exists but never replies, because no-responders detection covers only the zero-responder case.

## Requirements

### Requirement: Deadline-less contexts to FlushWithContext
Mirrors `FlushWithContext` in nats.go `context.go`, which returns `ErrNoDeadlineContext` when `ctx.Deadline()` is unset. The rule SHALL report a context argument that is a direct call to `context.Background()` or `context.TODO()` with `FlushWithContext requires a context with a deadline (returns ErrNoDeadlineContext); use context.WithTimeout`. Category `ctxdeadline`. No fix.

#### Scenario: Background
- **WHEN** code calls `nc.FlushWithContext(context.Background())`
- **THEN** the rule reports the message

#### Scenario: TODO
- **WHEN** code calls `nc.FlushWithContext(context.TODO())`
- **THEN** the rule reports the message

#### Scenario: Context variable
- **WHEN** code calls `nc.FlushWithContext(ctx)` where `ctx` is any variable, even one holding `context.Background()`
- **THEN** the rule reports nothing

### Requirement: Deadline-less contexts to request and next-message calls
The rule SHALL report a context argument that is a direct call to `context.Background()` or `context.TODO()` to `nats.Conn.RequestWithContext`, `nats.Conn.RequestMsgWithContext` or `nats.Subscription.NextMsgWithContext` with `<Method> with a context that has no deadline blocks forever if a responder exists but never replies; use context.WithTimeout`.

#### Scenario: Request
- **WHEN** code calls `nc.RequestWithContext(context.Background(), "s", nil)`
- **THEN** the rule reports `RequestWithContext with a context that has no deadline blocks forever if a responder exists but never replies; use context.WithTimeout`

#### Scenario: NextMsg
- **WHEN** code calls `sub.NextMsgWithContext(context.TODO())`
- **THEN** the rule reports the `NextMsgWithContext` message

#### Scenario: Derived context
- **WHEN** code calls `nc.RequestWithContext(context.WithValue(context.Background(), k, v), "s", nil)`
- **THEN** the rule reports nothing (only a direct `Background()`/`TODO()` argument is reported)

#### Scenario: jetstream methods are exempt
- **WHEN** code calls `js.Publish(context.Background(), "s", nil)` or `consumer.Fetch` with a background context
- **THEN** the rule reports nothing (the `jetstream` package applies its own default timeout)

### Requirement: Legacy Fetch with a background context option
Mirrors `Subscription.Fetch` and `FetchBatch` in nats.go `js.go`, which return `ErrNoDeadlineContext` when the `nats.Context` option carries `context.Background()` (a `context.TODO()` passes the check and blocks). The rule SHALL report a `Fetch` or `FetchBatch` call on `*nats.Subscription` whose arguments include a direct call `nats.Context(context.Background())` or `nats.Context(context.TODO())`, with `<Method> with nats.Context(context.Background()) returns ErrNoDeadlineContext; use nats.MaxWait or a context with a deadline` (naming `TODO` when that is what was passed).

#### Scenario: Fetch with background
- **WHEN** code calls `sub.Fetch(10, nats.Context(context.Background()))`
- **THEN** the rule reports the `Fetch` message

#### Scenario: FetchBatch with TODO
- **WHEN** code calls `sub.FetchBatch(10, nats.Context(context.TODO()))`
- **THEN** the rule reports the `FetchBatch` message naming `context.TODO()`

#### Scenario: Subscribe with background
- **WHEN** code calls `js.Subscribe("s", h, nats.Context(context.Background()))`
- **THEN** the rule reports nothing (the option only carries cancellation there)
