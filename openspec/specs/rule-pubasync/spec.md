# rule-pubasync Specification

## Purpose

pubasync reports a PublishAsync or PublishMsgAsync whose PubAckFuture is discarded in a package that neither installs an async publish error handler nor waits on PublishAsyncComplete. The future's Err channel and the error handler are the only two places a rejected or timed-out async publish is ever reported; when both are absent the publish fails silently and the caller believes the message was stored. An ack handler (WithPublishAsyncAckHandler) runs only for successful publishes and does not replace the error handler.

## Requirements

### Requirement: Discarded future with no error path in the package
Mirrors `handleAsyncReply` in nats.go `jetstream/publish.go` (and `js.go` for the legacy API), which delivers a failed async publish only to the future's `Err` channel and to the handler installed by `WithPublishAsyncErrHandler` (`nats.PublishAsyncErrHandler` for the legacy API), and a successful one only to the future's `Ok` channel and the handler installed by `WithPublishAsyncAckHandler`; `PublishAsyncComplete` reports completion, not errors. The rule SHALL report a call to `PublishAsync` or `PublishMsgAsync` on `jetstream.JetStream` or legacy `nats.JetStream` whose first result is assigned to `_` (in any assignment or `if`/`for`/`switch` initializer) or whose results are dropped by an expression statement, unless any file of the analyzed package (see analyzer-framework: package-wide symbol use) refers to `jetstream.WithPublishAsyncErrHandler`, `nats.PublishAsyncErrHandler`, or calls `PublishAsyncComplete` on either interface. The message SHALL be `PubAckFuture from <Method> discarded and this package never sets an async error handler or awaits PublishAsyncComplete; publish errors are lost`, reported at the call. Category `pubasync`. No fix.

#### Scenario: Discard with no error path
- **WHEN** a package has `_, err := js.PublishAsync("orders.new", data)` and no reference to `WithPublishAsyncErrHandler` or `PublishAsyncComplete` in any of its files
- **THEN** the rule reports `PubAckFuture from PublishAsync discarded and this package never sets an async error handler or awaits PublishAsyncComplete; publish errors are lost`

#### Scenario: Expression statement
- **WHEN** a package has `js.PublishMsgAsync(msg)` as a statement on its own under the same conditions
- **THEN** the rule reports the message naming `PublishMsgAsync`

#### Scenario: Legacy JetStreamContext
- **WHEN** a package has `_, err := js.PublishAsync("orders.new", data)` on a `nats.JetStreamContext` under the same conditions
- **THEN** the rule reports the message

#### Scenario: Future kept
- **WHEN** code has `f, err := js.PublishAsync("orders.new", data)` followed by `select { case <-f.Ok(): case err := <-f.Err(): }`
- **THEN** the rule reports nothing

#### Scenario: Handler installed in another function of the package
- **WHEN** a file of the package has `jetstream.New(nc, jetstream.WithPublishAsyncErrHandler(onErr))` and another function has `_, err := js.PublishAsync("orders.new", data)`
- **THEN** the rule reports nothing

#### Scenario: Legacy handler
- **WHEN** a file of the package has `nc.JetStream(nats.PublishAsyncErrHandler(onErr))`
- **THEN** the rule reports nothing for any discard in the package

#### Scenario: Ack handler paired with error handler
- **WHEN** a file of the package has `jetstream.New(nc, jetstream.WithPublishAsyncAckHandler(onAck), jetstream.WithPublishAsyncErrHandler(onErr))` and futures are discarded elsewhere in the package
- **THEN** the rule reports nothing

#### Scenario: Ack handler only
- **WHEN** a file of the package has `jetstream.New(nc, jetstream.WithPublishAsyncAckHandler(onAck))` with no error handler and no `PublishAsyncComplete`, and another function has `_, err := js.PublishAsync("orders.new", data)`
- **THEN** the rule reports the message (the ack handler runs only for successful publishes)

#### Scenario: Completion awaited in the same function
- **WHEN** a loop discards futures and the function ends with `select { case <-js.PublishAsyncComplete(): case <-time.After(5 * time.Second): }`
- **THEN** the rule reports nothing

#### Scenario: Completion awaited in another function of the package
- **WHEN** a method discards futures and a different method of the same package calls `js.PublishAsyncComplete()`
- **THEN** the rule reports nothing

#### Scenario: Synchronous publish
- **WHEN** code has `_, err := js.Publish(ctx, "orders.new", data)`
- **THEN** the rule reports nothing
