# rule-handle Specification

## Purpose

handle reports a lifecycle handle that is thrown away at the call that created it: a ConsumeContext from Consume, a MessagesContext from Messages, a KeyWatcher or ObjectWatcher from Watch, or a micro.Service from AddService assigned to the blank identifier or dropped as an expression statement. The handle is the only way to Stop or Drain what the call started; without it the consumer, watcher or service runs until the connection closes and in-flight work cannot be finished cleanly.

## Requirements

### Requirement: Discarded consume and messages context
Mirrors `Consumer.Consume`, `PushConsumer.Consume` and `Consumer.Messages` in nats.go `jetstream`, whose returned `ConsumeContext`/`MessagesContext` carry the only `Stop`/`Drain`. The rule SHALL report a call to `Consume` on `jetstream.Consumer` or `jetstream.PushConsumer`, or to `Messages` on `jetstream.Consumer`, whose first result is assigned to `_` (in any assignment or `if`/`for`/`switch` initializer) or whose results are dropped by an expression statement. The message SHALL be `<Type> from <Method> discarded; the consumer can never be stopped or drained`, with `<Type>` `ConsumeContext` or `MessagesContext`, reported at the call. Category `handle`. No fix.

#### Scenario: Consume assigned to blank
- **WHEN** code has `_, err := cons.Consume(handler)`
- **THEN** the rule reports `ConsumeContext from Consume discarded; the consumer can never be stopped or drained`

#### Scenario: Consume as an expression statement
- **WHEN** code has `cons.Consume(handler)` as a statement on its own
- **THEN** the rule reports the `ConsumeContext` message

#### Scenario: Push consumer
- **WHEN** code has `_, err = pc.Consume(handler)` on a `jetstream.PushConsumer`
- **THEN** the rule reports the `ConsumeContext` message

#### Scenario: Messages assigned to blank
- **WHEN** code has `if _, err := cons.Messages(); err != nil {`
- **THEN** the rule reports `MessagesContext from Messages discarded; the consumer can never be stopped or drained`

#### Scenario: Handle kept
- **WHEN** code has `cc, err := cons.Consume(handler)` followed by `defer cc.Stop()`
- **THEN** the rule reports nothing

#### Scenario: Handle returned
- **WHEN** a function ends with `return cons.Consume(handler)`
- **THEN** the rule reports nothing

### Requirement: Self-terminating consume is exempt
Mirrors `jetstream.StopAfter`, which makes `Consume` and `Messages` stop themselves after the given number of messages. The rule SHALL NOT report a `Consume` or `Messages` call when any argument has type `jetstream.StopAfter`.

#### Scenario: StopAfter option
- **WHEN** code has `_, err := cons.Consume(handler, jetstream.StopAfter(10))`
- **THEN** the rule reports nothing

#### Scenario: StopAfter through a variable
- **WHEN** code has `limit := jetstream.StopAfter(n)` and `_, err := cons.Messages(limit)`
- **THEN** the rule reports nothing

#### Scenario: Other options only
- **WHEN** code has `_, err := cons.Consume(handler, jetstream.PullMaxMessages(100))`
- **THEN** the rule reports the `ConsumeContext` message

### Requirement: Discarded watcher
Mirrors `KeyValue.Watch`, `WatchAll`, `WatchFiltered` and `ObjectStore.Watch` in nats.go `jetstream`, and their legacy twins on `nats.KeyValue` and `nats.ObjectStore`, whose watcher's `Stop` is the only way to unsubscribe the ordered consumer subscription they create. The rule SHALL report a call to any of those methods whose first result is assigned to `_` or dropped by an expression statement, with `<Type> from <Method> discarded; the watcher can never be stopped and its subscription lives as long as the connection`, `<Type>` being `KeyWatcher` or `ObjectWatcher`. No fix.

#### Scenario: KV watch assigned to blank
- **WHEN** code has `_, err = kv.Watch(ctx, "orders.*")` on a `jetstream.KeyValue`
- **THEN** the rule reports `KeyWatcher from Watch discarded; the watcher can never be stopped and its subscription lives as long as the connection`

#### Scenario: Legacy WatchAll
- **WHEN** code has `_, err := kv.WatchAll()` on a legacy `nats.KeyValue`
- **THEN** the rule reports the `KeyWatcher` message naming `WatchAll`

#### Scenario: Object store watch
- **WHEN** code has `_, err := os.Watch(ctx)` on a `jetstream.ObjectStore`
- **THEN** the rule reports `ObjectWatcher from Watch discarded; the watcher can never be stopped and its subscription lives as long as the connection`

#### Scenario: Watcher kept
- **WHEN** code has `w, err := kv.Watch(ctx, "orders.*")` followed by `defer w.Stop()`
- **THEN** the rule reports nothing

#### Scenario: Key lister is not a watcher
- **WHEN** code has `_, err := kv.ListKeys(ctx)`
- **THEN** the rule reports nothing

### Requirement: Discarded micro service
Mirrors `micro.AddService`, whose returned `Service` is the only way to `Stop` the endpoint subscriptions it registers. The rule SHALL report a call to `micro.AddService` whose first result is assigned to `_` or dropped by an expression statement, with `Service from AddService discarded; the service can never be stopped and keeps answering until the connection closes`. No fix.

#### Scenario: AddService assigned to blank
- **WHEN** code has `_, err := micro.AddService(nc, cfg)`
- **THEN** the rule reports `Service from AddService discarded; the service can never be stopped and keeps answering until the connection closes`

#### Scenario: Service kept
- **WHEN** code has `svc, err := micro.AddService(nc, cfg)` followed by `defer svc.Stop()`
- **THEN** the rule reports nothing

### Requirement: Only direct discards are examined
The rule SHALL NOT report a handle that is assigned to a named variable, a struct field, a map entry or a channel send, regardless of whether that variable is later used, and SHALL NOT report calls reached through a function value or an interface the analyzed package defines itself.

#### Scenario: Stored in a field
- **WHEN** code has `s.cc, err = cons.Consume(handler)`
- **THEN** the rule reports nothing

#### Scenario: Same-named method on a user type
- **WHEN** the analyzed package defines its own `Consume` method returning a `Stopper` and calls it as `_, err := w.Consume(h)`
- **THEN** the rule reports nothing
