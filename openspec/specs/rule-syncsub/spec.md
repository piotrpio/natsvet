# rule-syncsub Specification

## Purpose

syncsub reports NextMsg or NextMsgWithContext on a subscription that was not created as a synchronous one. nats.go's validateNextMsgState returns ErrSyncSubRequired when the subscription has a callback, ErrTypeSubscription when it is a legacy pull subscription, and for a channel subscription NextMsg silently reads from the caller's own channel and competes with it.

## Requirements

### Requirement: NextMsg on a callback subscription
Mirrors `validateNextMsgState` (`mcb != nil`). When the receiver of `NextMsg` or `NextMsgWithContext` is a local variable with a single definition in the enclosing function (see analyzer-framework: single definition) whose value is a call to `Subscribe` or `QueueSubscribe` on `nats.Conn`, or to `Subscribe`, `QueueSubscribe` on legacy `nats.JetStream`, the rule SHALL report `NextMsg on a subscription created with <Method> (callback) always returns ErrSyncSubRequired; use <Method>Sync`. Category `syncsub`. No fix.

#### Scenario: Callback subscription
- **WHEN** code has `sub, _ := nc.Subscribe("s", handler)` followed by `sub.NextMsg(time.Second)`
- **THEN** the rule reports `NextMsg on a subscription created with Subscribe (callback) always returns ErrSyncSubRequired; use SubscribeSync`

#### Scenario: Queue callback subscription through JetStream
- **WHEN** code has `sub, _ := js.QueueSubscribe("s", "q", handler)` on a legacy `nats.JetStreamContext` followed by `sub.NextMsgWithContext(ctx)`
- **THEN** the rule reports the message naming `QueueSubscribe`

#### Scenario: Sync subscription
- **WHEN** code has `sub, _ := nc.SubscribeSync("s")` followed by `sub.NextMsg(time.Second)`
- **THEN** the rule reports nothing

### Requirement: NextMsg on a channel subscription
Mirrors `NextMsg` reading from `s.mch`, which for `ChanSubscribe`, `ChanQueueSubscribe` and `QueueSubscribeSyncWithChan` is the caller's channel. Under the same single-definition condition, the rule SHALL report `NextMsg on a subscription created with <Method> steals messages from the channel; read the channel or use SubscribeSync`.

#### Scenario: Channel subscription
- **WHEN** code has `sub, _ := nc.ChanSubscribe("s", ch)` followed by `sub.NextMsg(time.Second)`
- **THEN** the rule reports the channel message naming `ChanSubscribe`

### Requirement: NextMsg on a legacy pull subscription
Mirrors `validateNextMsgState` (`jsi.pull` → `ErrTypeSubscription`). Under the same condition, when the value is a call to `PullSubscribe` on legacy `nats.JetStream`, the rule SHALL report `NextMsg on a pull subscription returns ErrTypeSubscription; use Fetch`.

#### Scenario: Pull subscription
- **WHEN** code has `sub, _ := js.PullSubscribe("s", "d")` followed by `sub.NextMsg(time.Second)`
- **THEN** the rule reports the pull message

### Requirement: Only single-definition receivers are examined
The rule SHALL report nothing when the receiver is a parameter, a struct field, a package-level variable, a variable assigned more than once in the function, or a variable whose address is taken.

#### Scenario: Reassigned subscription
- **WHEN** code has `sub, _ := nc.Subscribe("s", h)`, later `sub, _ = nc.SubscribeSync("s")`, then `sub.NextMsg(time.Second)`
- **THEN** the rule reports nothing

#### Scenario: Parameter
- **WHEN** `NextMsg` is called on a `*nats.Subscription` parameter
- **THEN** the rule reports nothing
