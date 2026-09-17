# rule-msgloop Specification

## Purpose

msgloop reports two loops over JetStream messages that mishandle the iterator's terminal state. A for loop that calls MessagesContext.Next and continues on every error never exits once the iterator is stopped or drained, because Next then returns ErrMsgIteratorClosed on every call without blocking. A Fetch result whose Messages channel is ranged over without checking Error afterwards drops the batch's terminal error, so a missed heartbeat or a deleted consumer looks like an empty batch.

## Requirements

### Requirement: Loop never exits on a closed iterator
Mirrors `pullSubscription.Next` in nats.go `jetstream/pull.go`, which returns `ErrMsgIteratorClosed` immediately when the subscription is closed and not draining, and `Stop`/`Drain`, which close it. The rule SHALL report a `for` statement with no condition, initializer or post statement whose body contains a statement `<msg>, <err> := <it>.Next(...)` (or `=`) where the callee is `Next` on `jetstream.MessagesContext`, immediately followed by an `if` — or an `if` whose own initializer is that statement — whose condition is `<err> != nil` (or `nil != <err>`) and whose then-branch contains no `return`, `break`, `goto`, `panic`, call to `os.Exit`, or call to a function or method whose name starts with `Fatal`, and either ends in `continue` or is paired with an `else` branch — provided no expression in the loop body refers to `jetstream.ErrMsgIteratorClosed`. The message SHALL be `loop continues on every Next error; after Stop or Drain, Next returns ErrMsgIteratorClosed on every call and the loop never exits`, reported at the `if`. Category `msgloop`. No fix.

#### Scenario: Log and continue
- **WHEN** code has `for { msg, err := it.Next(); if err != nil { log.Println(err); continue }; msg.Ack() }`
- **THEN** the rule reports the never-exits message at the `if`

#### Scenario: Else branch
- **WHEN** code has `for { msg, err := it.Next(); if err != nil { log.Println(err) } else { msg.Ack() } }`
- **THEN** the rule reports the never-exits message

#### Scenario: Next in the if initializer
- **WHEN** code has `for { if msg, err := it.Next(); err != nil { log.Println(err); continue } else { msg.Ack() } }`
- **THEN** the rule reports the never-exits message

#### Scenario: Backoff in the branch
- **WHEN** the then-branch is `log.Println(err); time.Sleep(time.Second); continue`
- **THEN** the rule reports the never-exits message (the loop no longer burns CPU, but it still cannot observe that the iterator is gone)

#### Scenario: Labeled continue to an outer loop
- **WHEN** the then-branch ends in `continue outer` where `outer` labels a loop enclosing this one
- **THEN** the rule reports nothing

#### Scenario: Break on closed iterator
- **WHEN** the then-branch is `if errors.Is(err, jetstream.ErrMsgIteratorClosed) { break }; continue`
- **THEN** the rule reports nothing

#### Scenario: Closed iterator handled earlier in the loop
- **WHEN** the loop has `if errors.Is(err, jetstream.ErrMsgIteratorClosed) { return }` before `if err != nil { continue }`
- **THEN** the rule reports nothing

#### Scenario: Return on error
- **WHEN** the then-branch is `return err`
- **THEN** the rule reports nothing

#### Scenario: Fatal on error
- **WHEN** the then-branch is `log.Fatal(err)`
- **THEN** the rule reports nothing

#### Scenario: Conditional loop
- **WHEN** the loop is `for ctx.Err() == nil { ... }` or a `for range` with the same body
- **THEN** the rule reports nothing

#### Scenario: One-shot Next
- **WHEN** the call is `cons.Next()` on a `jetstream.Consumer` (a single fetch with a timeout) with the same loop shape
- **THEN** the rule reports nothing

#### Scenario: Timeout-only error handling
- **WHEN** the then-branch is `if errors.Is(err, nats.ErrTimeout) { continue }; return err`
- **THEN** the rule reports nothing

### Requirement: Fetch result ranged without Error
Mirrors `fetchResult.Error` in nats.go `jetstream/pull.go`, which holds the error that ended the batch (`ErrNoHeartbeat`, a terminal status such as consumer deleted, the context's error) after `Messages()` is closed, and its legacy twin `nats.MessageBatch` from `Subscription.FetchBatch`. The rule SHALL report a `range` statement over `<r>.Messages()` when `<r>` is a local variable with a single definition in the enclosing function (see analyzer-framework: single definition) whose value is a call to `Fetch`, `FetchBytes` or `FetchNoWait` on `jetstream.Consumer` or to `FetchBatch` on `*nats.Subscription`, every use of `<r>` in the function is the receiver of `Messages` or `Error`, and no call to `<r>.Error()` exists anywhere in the function. The message SHALL be `<Method> result ranged without checking Error(); a failed fetch looks like an empty batch`, reported at the `range` statement. No fix.

#### Scenario: Range without Error
- **WHEN** code has `msgs, err := cons.Fetch(10)` and `for msg := range msgs.Messages() { msg.Ack() }` with no `msgs.Error()` in the function
- **THEN** the rule reports `Fetch result ranged without checking Error(); a failed fetch looks like an empty batch`

#### Scenario: Legacy FetchBatch
- **WHEN** code has `msgs, err := sub.FetchBatch(10)` on a `*nats.Subscription` and ranges `msgs.Messages()` without `msgs.Error()`
- **THEN** the rule reports the message naming `FetchBatch`

#### Scenario: Error checked after the range
- **WHEN** the range is followed by `if err := msgs.Error(); err != nil {`
- **THEN** the rule reports nothing

#### Scenario: Error checked inside a closure
- **WHEN** `msgs.Error()` is called inside a `defer func() { ... }()` in the same function
- **THEN** the rule reports nothing

#### Scenario: Batch passed elsewhere
- **WHEN** code has `msgs, err := cons.Fetch(10)`, `process(msgs)` and then ranges `msgs.Messages()`
- **THEN** the rule reports nothing

#### Scenario: Batch is a parameter
- **WHEN** a function ranges over `batch.Messages()` where `batch` is a `jetstream.MessageBatch` parameter
- **THEN** the rule reports nothing

#### Scenario: Channel drained without a range
- **WHEN** code reads `<-msgs.Messages()` in a `select` and never ranges
- **THEN** the rule reports nothing
