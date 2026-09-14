## Purpose

subject reports constant subjects and queue group names that nats.go or the server rejects, and publish subjects that contain wildcards. nats.go's validateSubject returns ErrBadSubject for an empty subject or one with whitespace, the server rejects a subscription with an empty token or a misplaced >, and a wildcard token in a publish subject is sent literally, so the message matches only subscriptions that spell out the same literal token.

## ADDED Requirements

### Requirement: Subject positions
The rule SHALL examine constant string expressions in these positions: the subject (and reply) arguments of `nats.Conn` methods `Publish`, `PublishRequest`, `Request`, `RequestWithContext`, `Subscribe`, `SubscribeSync`, `QueueSubscribe`, `QueueSubscribeSync`, `ChanSubscribe`, `ChanQueueSubscribe`, `QueueSubscribeSyncWithChan`; the argument of `nats.NewMsg`; the `Subject` and `Reply` fields of a `nats.Msg` literal; the subject argument of `jetstream.Publisher` `Publish` and `PublishAsync` and of legacy `nats.JetStream` `Publish`, `PublishAsync`, `Subscribe`, `SubscribeSync`, `QueueSubscribe`, `QueueSubscribeSync`, `ChanSubscribe`, `ChanQueueSubscribe`, `PullSubscribe`; the `Subject` field of `micro.EndpointConfig` and the argument of `micro.WithEndpointSubject`. Non-constant expressions SHALL be skipped, and so SHALL the subject of a legacy `nats.JetStream` subscribe call that passes `nats.Bind(stream, consumer)` among its arguments. Category `subject`. No fix.

#### Scenario: Concatenated constants
- **WHEN** code declares `const prefix = "orders"` and calls `nc.Publish(prefix+"..new", nil)`
- **THEN** the rule reports the argument

#### Scenario: Non-constant subject
- **WHEN** code calls `nc.Publish(fmt.Sprintf("orders.%s", id), nil)` or `nc.Publish(subj, nil)`
- **THEN** the rule reports nothing

### Requirement: Invalid subjects are reported everywhere
Mirrors nats.go `validateSubject`/`badSubject` and nats-server `IsValidSubject`. The rule SHALL report a constant subject that is empty (except on the legacy `nats.JetStream` subscribe methods, where an empty subject is valid with `Bind`/`BindStream`), has an empty token, contains whitespace, or has a `>` token that is not last, with `subject "<s>" is invalid: <reason>` where reason is one of `empty subject`, `empty token`, `contains whitespace`, `'>' must be the last token`.

#### Scenario: Empty token
- **WHEN** code calls `nc.Subscribe("foo..bar", handler)`
- **THEN** the rule reports `subject "foo..bar" is invalid: empty token`

#### Scenario: Whitespace
- **WHEN** code calls `nc.Publish("foo. bar", nil)`
- **THEN** the rule reports `subject "foo. bar" is invalid: contains whitespace`

#### Scenario: Misplaced full wildcard
- **WHEN** code calls `nc.SubscribeSync("foo.>.bar")`
- **THEN** the rule reports `subject "foo.>.bar" is invalid: '>' must be the last token`

#### Scenario: Empty subject
- **WHEN** code writes `nats.Msg{Subject: ""}`
- **THEN** the rule reports `subject "" is invalid: empty subject`

#### Scenario: Empty subject on a legacy JetStream subscribe
- **WHEN** code calls `js.SubscribeSync("", nats.BindStream("ORDERS"))` or `js.PullSubscribe("", "d", nats.Bind("ORDERS", "d"))` on a legacy `nats.JetStreamContext`
- **THEN** the rule reports nothing: nats.go accepts an empty subject when a stream is bound (`js.go`: "subject required" only without a stream), and the binding option is often behind a variadic `opts...`

#### Scenario: Any subject with a bound consumer
- **WHEN** code calls `js.PullSubscribe(".>", "d", nats.Bind("ORDERS", "d"))` or `js.Subscribe(".>", handler, nats.Bind("ORDERS", "d"))`
- **THEN** the rule reports nothing: with `Bind(stream, consumer)` the subject is never used as a subscription subject, only compared to the consumer's `FilterSubject` (`js.go` `processConsInfo`), which is not decidable statically

#### Scenario: Invalid subject with only a bound stream
- **WHEN** code calls `js.PullSubscribe(".>", "d", nats.BindStream("ORDERS"))` or `js.PullSubscribe(".>", "d")`
- **THEN** the rule reports `subject ".>" is invalid: empty token`: the subject becomes the new consumer's `FilterSubject`, which the server validates

#### Scenario: Reply subject
- **WHEN** code calls `nc.PublishRequest("req", "reply..x", nil)`
- **THEN** the rule reports the reply argument

#### Scenario: Micro endpoint
- **WHEN** code writes `micro.EndpointConfig{Subject: "svc. echo"}` or calls `micro.WithEndpointSubject("svc..echo")`
- **THEN** the rule reports the value

#### Scenario: Valid subjects
- **WHEN** code calls `nc.Subscribe("foo.>", h)`, `nc.Subscribe("*.bar", h)` and `nc.Publish("foo.bar", nil)`
- **THEN** the rule reports nothing

### Requirement: Publish subjects must be literal
The rule SHALL report a constant subject containing a `*` or `>` token (nats-server `subjectIsLiteral` false) in a publish position — `Publish`, `PublishRequest` (subject and reply), `Request`, `RequestWithContext`, `NewMsg`, `nats.Msg.Subject`/`Reply`, jetstream `Publish`/`PublishAsync`, legacy `Publish`/`PublishAsync` — with `publish subject "<s>" contains a wildcard; wildcards only match in subscriptions`. Subscribe positions SHALL NOT be subject to this check.

#### Scenario: Wildcard publish
- **WHEN** code calls `nc.Publish("orders.*", data)` or `js.Publish(ctx, "orders.>", data)`
- **THEN** the rule reports the wildcard message

#### Scenario: Wildcard in a message literal
- **WHEN** code writes `nats.Msg{Subject: "orders.*"}`
- **THEN** the rule reports the wildcard message

#### Scenario: Wildcard subscribe
- **WHEN** code calls `nc.Subscribe("orders.*", h)` or `js.SubscribeSync("orders.>")`
- **THEN** the rule reports nothing

#### Scenario: Literal token that only looks like a wildcard
- **WHEN** code calls `nc.Publish("foo*bar", nil)` or `nc.Publish("a.b>", nil)`
- **THEN** the rule reports nothing (not a wildcard token)

### Requirement: Queue group names have no whitespace
Mirrors nats.go `badQueue` (`ErrBadQueueName`) and micro's endpoint validation. The rule SHALL report a constant queue group argument of `QueueSubscribe`, `QueueSubscribeSync`, `ChanQueueSubscribe` and `QueueSubscribeSyncWithChan` on `nats.Conn`, of `QueueSubscribe`, `QueueSubscribeSync` and `ChanQueueSubscribe` on legacy `nats.JetStream`, and a constant `QueueGroup` field of `micro.EndpointConfig`, that contains whitespace, with `queue group "<q>" contains whitespace (ErrBadQueueName)`.

#### Scenario: Queue with a space
- **WHEN** code calls `nc.QueueSubscribe("orders", "order workers", h)`
- **THEN** the rule reports `queue group "order workers" contains whitespace (ErrBadQueueName)` at the queue argument

#### Scenario: Micro queue group
- **WHEN** code writes `micro.EndpointConfig{Subject: "svc.echo", QueueGroup: "q 1"}`
- **THEN** the rule reports the message

#### Scenario: Valid and empty queue names
- **WHEN** code calls `nc.QueueSubscribe("orders", "workers", h)` or `js.QueueSubscribe("orders", "", h)`
- **THEN** the rule reports nothing
