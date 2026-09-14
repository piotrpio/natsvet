## Purpose

subject reports constant subjects that nats.go or the server rejects, and publish subjects that contain wildcards. nats.go's validateSubject returns ErrBadSubject for an empty subject or one with whitespace, the server rejects a subscription with an empty token or a misplaced >, and a wildcard token in a publish subject is sent literally, so the message matches only subscriptions that spell out the same literal token.

## ADDED Requirements

### Requirement: Subject positions
The rule SHALL examine constant string expressions in these positions: the subject (and reply) arguments of `nats.Conn` methods `Publish`, `PublishRequest`, `Request`, `RequestWithContext`, `Subscribe`, `SubscribeSync`, `QueueSubscribe`, `QueueSubscribeSync`, `ChanSubscribe`, `ChanQueueSubscribe`, `QueueSubscribeSyncWithChan`; the argument of `nats.NewMsg`; the `Subject` and `Reply` fields of a `nats.Msg` literal; the subject argument of `jetstream.Publisher` `Publish` and `PublishAsync` and of legacy `nats.JetStream` `Publish`, `PublishAsync`, `Subscribe`, `SubscribeSync`, `QueueSubscribe`, `QueueSubscribeSync`, `ChanSubscribe`, `ChanQueueSubscribe`, `PullSubscribe`; the `Subject` field of `micro.EndpointConfig` and the argument of `micro.WithEndpointSubject`. Non-constant expressions SHALL be skipped. Category `subject`. No fix.

#### Scenario: Concatenated constants
- **WHEN** code declares `const prefix = "orders"` and calls `nc.Publish(prefix+"..new", nil)`
- **THEN** the rule reports the argument

#### Scenario: Non-constant subject
- **WHEN** code calls `nc.Publish(fmt.Sprintf("orders.%s", id), nil)` or `nc.Publish(subj, nil)`
- **THEN** the rule reports nothing

### Requirement: Invalid subjects are reported everywhere
Mirrors nats.go `validateSubject`/`badSubject` and nats-server `IsValidSubject`. The rule SHALL report a constant subject that is empty, has an empty token, contains whitespace, or has a `>` token that is not last, with `subject "<s>" is invalid: <reason>` where reason is one of `empty subject`, `empty token`, `contains whitespace`, `'>' must be the last token`.

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
