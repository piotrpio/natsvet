# Migration plan for natsvet/testdata

Read `natsvet migrate skill` before acting on this plan. Plan schema version 1; mapping verified against nats.go v1.53.1; the module requires v1.53.1.

## Summary

- Legacy uses: 133 in 103 sites
- Mechanical: 78, guided: 17, decision: 7, unmapped: 1, skipped: 0
- Components: 29, steps: 136

## Pending decisions

Ask the user each question, record the answer in natsvet-migrate.json, and plan again.

- **ack** (module, 5 sites): legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior. Options: after-handler, explicit, none. Default: **after-handler**.
- **channel-max-ack-pending** (module, 1 site): legacy set MaxAckPending to the channel capacity. Options: keep, server-default. Default: **keep**.
- **shared-handler** (module, 1 site): the handler also serves a core subscription, which keeps receiving *nats.Msg. Options: split, adapter. Default: **split**.
- **subscribe-target** (module, 7 sites): the jetstream package is built around pull consumers, which is why teams migrate. Options: pull, push, defer. Default: **pull**.
- **subscribe-target** (site:migrate/app/subscribe.go:41:14, 1 site): the subscription binds an existing consumer, which legacy Subscribe requires to be a push consumer; pull would fail until the consumer is recreated. Options: pull, push, defer. Default: **push**.
- **component** (module, 29 components): skip keeps code that must stay on the legacy API (tests of the legacy API, compatibility shims) out of the steps; the steps assume migrate until answered. Options: migrate, skip. Default: **migrate**.

## Component migrate/app/app.go:32:2

Handles: JS (migrate/app/app.go:27:2), kv (migrate/app/app.go:28:2), js (migrate/app/app.go:32:2), kv (migrate/app/app.go:36:2).


### Step S0 (add-handle, machine edits)

create the jetstream siblings JSNew, kvNew, jsNew, kvNew next to the legacy handles.


#### Site migrate/app/app.go:32:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream(nats.MaxWait(2 * time.Second))
```

After:

```go
js, err := nc.JetStream(nats.MaxWait(2 * time.Second))
jsNew, err := jetstream.New(nc, jetstream.WithDefaultTimeout(2 * time.Second))
if err != nil {
	return nil, err
}
```


#### Site migrate/app/app.go:36:13: mechanical

KeyValueManager.KeyValue becomes JetStream.KeyValue. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv, err := js.KeyValue("config")
```

After:

```go
kv, err := js.KeyValue("config")
kvNew, err := jsNew.KeyValue(context.Background(), "config")
if err != nil {
	return nil, err
}
```


#### Site migrate/app/app.go:27:2: mechanical

declare JSNew jetstream.JetStream next to JS. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
JS nats.JetStreamContext
```

After:

```go
JS nats.JetStreamContext
JSNew jetstream.JetStream
```


#### Site migrate/app/app.go:28:2: mechanical

declare kvNew jetstream.KeyValue next to kv. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv nats.KeyValue
```

After:

```go
kv nats.KeyValue
kvNew jetstream.KeyValue
```


### Step S1 (site, machine edits)

JetStreamManager.AddStream becomes JetStream.CreateStream.


#### Site migrate/app/app.go:44:15: mechanical

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := s.JS.AddStream(&nats.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}})
```

After:

```go
_, err := s.JSNew.CreateStream(ctx, jetstream.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}})
```


### Step S2 (site, machine edits)

JetStreamManager.AddConsumer becomes JetStream.CreatePushConsumer.


#### Site migrate/app/app.go:47:12: mechanical

JetStreamManager.AddConsumer becomes JetStream.CreatePushConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := s.JS.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "audit", DeliverSubject: "audit.deliver", Heartbeat: time.Second})
```

After:

```go
_, err := s.JSNew.CreatePushConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "audit", DeliverSubject: "audit.deliver", IdleHeartbeat: time.Second, AckPolicy: jetstream.AckNonePolicy})
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

### Step S3 (site, machine edits)

JetStreamManager.PurgeStream becomes Stream.Purge.


#### Site migrate/app/app.go:52:9: mechanical

JetStreamManager.PurgeStream becomes Stream.Purge. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return s.JS.PurgeStream("ORDERS")
```

After:

```go
return func() error {
		stream, err := s.JSNew.Stream(context.Background(), "ORDERS")
		if err != nil {
			return err
		}
		return stream.Purge(context.Background())
	}()
```

- Note: the new call first obtains a stream handle, which sends a STREAM.INFO request and needs that permission

### Step S4 (site, machine edits)

JetStream.Publish becomes JetStream.Publish.


#### Site migrate/app/app.go:56:12: mechanical

JetStream.Publish becomes JetStream.Publish. See jetstream/MIGRATION.md#publishing.

Before:

```go
_, err := s.JS.Publish("orders.new", data, nats.MsgId(id))
```

After:

```go
_, err := s.JSNew.Publish(ctx, "orders.new", data, jetstream.WithMsgID(id))
```


### Step S5 (site, machine edits)

KeyValue.Get becomes KeyValue.Get; nats.ErrKeyNotFound becomes jetstream.ErrKeyNotFound with the call at migrate/app/app.go:61:12, whose error it checks.


#### Site migrate/app/app.go:61:12: mechanical

KeyValue.Get becomes KeyValue.Get. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
e, err := s.kv.Get(key)
```

After:

```go
e, err := s.kvNew.Get(context.Background(), key)
```


#### Site migrate/app/app.go:62:20: mechanical

nats.ErrKeyNotFound becomes jetstream.ErrKeyNotFound with the call at migrate/app/app.go:61:12, whose error it checks.

Before:

```go
if errors.Is(err, nats.ErrKeyNotFound) {
		return "", nil
	}
```

After:

```go
if errors.Is(err, jetstream.ErrKeyNotFound) {
		return "", nil
	}
```


### Step S6 (site, machine edits)

JetStreamManager.StreamNames becomes JetStream.StreamNames.


#### Site migrate/app/app.go:73:17: mechanical

JetStreamManager.StreamNames becomes JetStream.StreamNames. See jetstream/MIGRATION.md#stream-management.

Before:

```go
for n := range s.JS.StreamNames() {
		names = append(names, n)
	}
```

After:

```go
for n := range s.JSNew.StreamNames(context.Background()).Name() {
		names = append(names, n)
	}
```

- Note: legacy swallowed listing errors; ranging over the lister's channel keeps that, check the lister's error as a follow-up

### Step S7 (site, machine edits)

ordered Subscribe becomes an ordered consumer's Consume.


#### Site migrate/app/subscribe.go:55:14: mechanical

ordered Subscribe becomes an ordered consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
sub, err := s.JS.Subscribe("orders.>", func(m *nats.Msg) {
		process(m.Subject, m.Data)
	}, nats.OrderedConsumer(), nats.DeliverLast())
```

After:

```go
sub, err := func() (jetstream.ConsumeContext, error) {
		stream, err := s.JSNew.StreamNameBySubject(ctx, "orders.>")
		if err != nil {
			return nil, err
		}
		cons, err := s.JSNew.OrderedConsumer(ctx, stream, jetstream.OrderedConsumerConfig{
			FilterSubjects: []string{"orders.>"},
			DeliverPolicy: jetstream.DeliverLastPolicy,
		})
		if err != nil {
			return nil, err
		}
		return cons.Consume(func(m jetstream.Msg) {
			process(m.Subject(), m.Data())
		})
	}()
```


### Step S8 (site, machine edits)

JetStream.Publish becomes JetStream.Publish.


#### Site migrate/worker/worker.go:21:12: mechanical

JetStream.Publish becomes JetStream.Publish. See jetstream/MIGRATION.md#publishing.

Before:

```go
_, err := svc.JS.Publish("orders.done", nil)
```

After:

```go
_, err := svc.JSNew.Publish(context.Background(), "orders.done", nil)
```


### Step S9 (site)

PullSubscribe becomes a jetstream.Consumer.


#### Site migrate/app/subscribe.go:81:14: guided

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := s.JS.PullSubscribe("orders.new", "batch")
```

Template:

```go
sub, err := func() (jetstream.Consumer, error) {
		stream, err := s.JSNew.StreamNameBySubject(context.Background(), "orders.new")
		if err != nil {
			return nil, err
		}
		return s.JSNew.CreateOrUpdateConsumer(context.Background(), stream, jetstream.ConsumerConfig{
			Durable: "batch",
			FilterSubject: "orders.new",
		})
	}()

batch, err := sub.Fetch(10, jetstream.FetchMaxWait(time.Second))
if err != nil {
	// the pull request failed; an empty batch is not an error
}
for msg := range batch.Messages() {
	// the loop body, with msg a jetstream.Msg
}
if err := batch.Error(); err != nil {
	// the batch ended with an error
}
```

- Fact: Subscription.Fetch: Fetch returns a MessageBatch: range over Messages(), then check Error(); an empty pull is not an error (timeouts and no-message statuses are dropped), so legacy nats.ErrTimeout checks on the batch go away; Consumer.Next still returns nats.ErrTimeout
- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it

### Step S10 (site)

sync subscription becomes a Messages iterator.


#### Site migrate/app/subscribe.go:101:14: guided

sync subscription becomes a Messages iterator. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
sub, err := s.JS.SubscribeSync("orders.sync", nats.Durable("sync"))
```

Template:

```go
sub, err := s.JS.SubscribeSync("orders.sync", nats.Durable("sync"))
```

- Fact: a sync subscription becomes a consumer's Messages() iterator (MessagesContext.Next replaces NextMsg; a channel is fed from Next in a goroutine); rewrite the reading code around it
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle

### Step S11 (site)

channel subscription becomes a Messages iterator.


#### Site migrate/app/subscribe.go:114:12: guided

channel subscription becomes a Messages iterator. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
_, err := s.JS.ChanSubscribe("orders.chan", ch, nats.Durable("chan"))
```

Template:

```go
_, err := s.JS.ChanSubscribe("orders.chan", ch, nats.Durable("chan"))
```

- Fact: a channel subscription becomes a consumer's Messages() iterator (MessagesContext.Next replaces NextMsg; a channel is fed from Next in a goroutine); rewrite the reading code around it
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle
- Decision **channel-max-ack-pending** pending, default **keep**: legacy set MaxAckPending to the channel capacity.
  - keep: keep MaxAckPending at the channel capacity, as legacy set it
  - server-default: leave MaxAckPending to the server default

### Step S12 (site)

Subscribe becomes a pull consumer's Consume.


#### Site migrate/app/subscribe.go:28:14: decision

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
sub, err := s.JS.Subscribe("orders.new", func(m *nats.Msg) {
		process(m.Subject, m.Data)
	}, nats.Durable("workers"), nats.DeliverNew())
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle
- Decision **ack** pending, default **after-handler**: legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior.
  - after-handler: ack after the handler returns, as the legacy wrapper did
  - explicit: ack explicitly on each handler path
  - none: AckNonePolicy: no acks at all

### Step S13 (site)

Subscribe becomes a push consumer's Consume.


#### Site migrate/app/subscribe.go:41:14: decision

Subscribe becomes a push consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
sub, err := s.JS.QueueSubscribe("orders.new", "q", func(m *nats.Msg) {
		process(m.Subject, m.Data)
		m.Ack()
	}, nats.Bind("ORDERS", "queue"), nats.ManualAck())
```

- Decision **subscribe-target** pending, default **push**: the subscription binds an existing consumer, which legacy Subscribe requires to be a push consumer; pull would fail until the consumer is recreated.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle

#### Site migrate/app/subscribe.go:43:3: decision

rewritten with the handler of migrate/app/subscribe.go:41:14. See jetstream/MIGRATION.md#message-acknowledgement.

Before:

```go
m.Ack()
```

- Fact: the message is a handler parameter: the call migrates with the subscription that delivers it, once the handler takes a jetstream.Msg (becomes jetstream Msg.Ack)

### Step S14 (site)

Subscribe becomes a pull consumer's Consume.


#### Site migrate/app/subscribe.go:68:14: decision

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
sub, err := s.JS.Subscribe("orders.limited", func(m *nats.Msg) {
		process(m.Subject, m.Data)
	}, nats.ConsumerName("limited"), nats.RateLimit(1024))
```

- Note: push-only options are dropped: a pull consumer has no equivalent
- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle
- Decision **ack** pending, default **after-handler**: legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior.
  - after-handler: ack after the handler returns, as the legacy wrapper did
  - explicit: ack explicitly on each handler path
  - none: AckNonePolicy: no acks at all

### Step S15 (site)

Subscribe becomes a pull consumer's Consume.


#### Site migrate/app/subscribe.go:127:12: decision

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
_, err := s.JS.Subscribe("orders.shared", handle, nats.Durable("shared"))
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle
- Decision **ack** pending, default **after-handler**: legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior.
  - after-handler: ack after the handler returns, as the legacy wrapper did
  - explicit: ack explicitly on each handler path
  - none: AckNonePolicy: no acks at all
- Decision **shared-handler** pending, default **split**: the handler also serves a core subscription, which keeps receiving *nats.Msg.
  - split: split the handler: a jetstream.Msg copy for the JetStream subscription, the original for core
  - adapter: keep one handler and adapt jetstream.Msg to *nats.Msg at the JetStream subscription

### Step S16 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/app/subscribe.go:81:14, migrate/app/subscribe.go:101:14, migrate/app/subscribe.go:114:12, migrate/app/subscribe.go:28:14, migrate/app/subscribe.go:41:14, migrate/app/subscribe.go:43:3, migrate/app/subscribe.go:68:14, migrate/app/subscribe.go:127:12.


#### Site migrate/app/app.go:32:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream(nats.MaxWait(2 * time.Second))
```

After:

```go
js, err := nc.JetStream(nats.MaxWait(2 * time.Second))
jsNew, err := jetstream.New(nc, jetstream.WithDefaultTimeout(2 * time.Second))
if err != nil {
	return nil, err
}
```


#### Site migrate/app/app.go:36:13: mechanical

KeyValueManager.KeyValue becomes JetStream.KeyValue. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv, err := js.KeyValue("config")
```

After:

```go
kv, err := js.KeyValue("config")
kvNew, err := jsNew.KeyValue(context.Background(), "config")
if err != nil {
	return nil, err
}
```


#### Site migrate/app/app.go:27:2: mechanical

declare JSNew jetstream.JetStream next to JS. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
JS nats.JetStreamContext
```

After:

```go
JS nats.JetStreamContext
JSNew jetstream.JetStream
```


#### Site migrate/app/app.go:28:2: mechanical

declare kvNew jetstream.KeyValue next to kv. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv nats.KeyValue
```

After:

```go
kv nats.KeyValue
kvNew jetstream.KeyValue
```


### Step S17 (rename)

rename each sibling to its legacy name: JSNew → JS, kvNew → kv, jsNew → js, kvNew → kv. Waits on: migrate/app/subscribe.go:81:14, migrate/app/subscribe.go:101:14, migrate/app/subscribe.go:114:12, migrate/app/subscribe.go:28:14, migrate/app/subscribe.go:41:14, migrate/app/subscribe.go:43:3, migrate/app/subscribe.go:68:14, migrate/app/subscribe.go:127:12.


## Component migrate/boundary/boundary.go:23:2

Handles: js (migrate/boundary/boundary.go:23:2).

- Blocked at migrate/boundary/boundary.go:27:10: passed to natsvet/testdata/migrateext/lib.Use, outside the loaded packages; removal and rename are omitted.

### Step S18 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/boundary/boundary.go:23:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return err
}
```


### Step S19 (site, machine edits)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream.


#### Site migrate/boundary/boundary.go:28:9: mechanical

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("TMP")
```

After:

```go
return jsNew.DeleteStream(context.Background(), "TMP")
```


## Component migrate/independent/independent.go:23:2

Handles: js (migrate/independent/independent.go:23:2). Safe to apply in one commit.


### Step S20 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/independent/independent.go:23:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return err
}
```


### Step S21 (site, machine edits)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream.


#### Site migrate/independent/independent.go:27:9: mechanical

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("OLD")
```

After:

```go
return jsNew.DeleteStream(context.Background(), "OLD")
```


### Step S22 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/independent/independent.go:23:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return err
}
```


### Step S23 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/independent/independent.go:31:2

Handles: js (migrate/independent/independent.go:31:2). Safe to apply in one commit.


### Step S24 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/independent/independent.go:31:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return 0, err
}
```


### Step S25 (site, machine edits)

JetStreamManager.DeleteConsumer becomes JetStream.DeleteConsumer.


#### Site migrate/independent/independent.go:35:12: mechanical

JetStreamManager.DeleteConsumer becomes JetStream.DeleteConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
err := js.DeleteConsumer("ORDERS", "stale", nats.Context(ctx))
```

After:

```go
err := jsNew.DeleteConsumer(ctx, "ORDERS", "stale")
```


### Step S26 (site, machine edits)

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.


#### Site migrate/independent/independent.go:38:15: mechanical

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := jsNew.AccountInfo(ctx)
```


### Step S27 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/independent/independent.go:31:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return 0, err
}
```


### Step S28 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/legacytests/legacy_test.go:27:2

Handles: js (migrate/legacytests/legacy_test.go:27:2). Safe to apply in one commit.


### Step S29 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/legacytests/legacy_test.go:27:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	t.Fatal(err)
}
```


### Step S30 (site, machine edits)

JetStreamManager.AddStream becomes JetStream.CreateStream.


#### Site migrate/legacytests/legacy_test.go:31:15: mechanical

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&nats.StreamConfig{Name: "LEGACY"})
```

After:

```go
_, err := jsNew.CreateStream(context.Background(), jetstream.StreamConfig{Name: "LEGACY"})
```


### Step S31 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/legacytests/legacy_test.go:27:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	t.Fatal(err)
}
```


### Step S32 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/multifile/handle.go:21:2

Handles: js (migrate/multifile/handle.go:21:2). Safe to apply in one commit.


### Step S33 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/multifile/handle.go:21:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return 0, err
}
```


### Step S34 (site, machine edits)

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.


#### Site migrate/multifile/handle.go:25:15: mechanical

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := jsNew.AccountInfo(context.Background())
```


### Step S35 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/multifile/handle.go:21:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return 0, err
}
```


### Step S36 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/objects/objects.go:23:2

Handles: js (migrate/objects/objects.go:23:2), obj (migrate/objects/objects.go:27:2). Safe to apply in one commit.


### Step S37 (add-handle, machine edits)

create the jetstream siblings jsNew, objNew next to the legacy handles.


#### Site migrate/objects/objects.go:23:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return nil, err
}
```


#### Site migrate/objects/objects.go:27:14: mechanical

ObjectStoreManager.ObjectStore becomes JetStream.ObjectStore. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
obj, err := js.ObjectStore("files")
```

After:

```go
obj, err := js.ObjectStore("files")
objNew, err := jsNew.ObjectStore(ctx, "files")
if err != nil {
	return nil, err
}
```


### Step S38 (site, machine edits)

ObjectStore.PutBytes becomes ObjectStore.PutBytes.


#### Site migrate/objects/objects.go:31:15: mechanical

ObjectStore.PutBytes becomes ObjectStore.PutBytes. See jetstream/MIGRATION.md#object-store.

Before:

```go
_, err := obj.PutBytes("a", data)
```

After:

```go
_, err := objNew.PutBytes(ctx, "a", data)
```


### Step S39 (site, machine edits)

ObjectStore.GetBytes becomes ObjectStore.GetBytes.


#### Site migrate/objects/objects.go:34:9: mechanical

ObjectStore.GetBytes becomes ObjectStore.GetBytes. See jetstream/MIGRATION.md#object-store.

Before:

```go
return obj.GetBytes("a")
```

After:

```go
return objNew.GetBytes(ctx, "a")
```


### Step S40 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/objects/objects.go:23:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return nil, err
}
```


#### Site migrate/objects/objects.go:27:14: mechanical

ObjectStoreManager.ObjectStore becomes JetStream.ObjectStore. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
obj, err := js.ObjectStore("files")
```

After:

```go
obj, err := js.ObjectStore("files")
objNew, err := jsNew.ObjectStore(ctx, "files")
if err != nil {
	return nil, err
}
```


### Step S41 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js, objNew → obj.


## Component migrate/order/order.go:26:11

Handles: js (migrate/order/order.go:26:11).


### Step S42 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/order/order.go:26:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S43 (site)

PullSubscribe becomes a jetstream.Consumer.


#### Site migrate/order/order.go:27:14: guided

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := js.PullSubscribe("ORDERS.new", "worker")
```

Template:

```go
sub, err := func() (jetstream.Consumer, error) {
		stream, err := jsNew.StreamNameBySubject(context.Background(), "ORDERS.new")
		if err != nil {
			return nil, err
		}
		return jsNew.CreateOrUpdateConsumer(context.Background(), stream, jetstream.ConsumerConfig{
			Durable: "worker",
			FilterSubject: "ORDERS.new",
		})
	}()
```

- Fact: the subscription sub is used at migrate/order/order.go:31:20 in a way the planner does not rewrite
- Fact: the subscription sub is used at migrate/order/order.go:32:10 in a way the planner does not rewrite
- Fact: the subscription sub is used at migrate/order/order.go:33:10 in a way the planner does not rewrite
- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it

### Step S44 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/order/order.go:27:14.


#### Site migrate/order/order.go:26:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S45 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: migrate/order/order.go:27:14.


## Component migrate/scenarios/corpus.go:26:2

Handles: js (migrate/scenarios/corpus.go:26:2).


### Step S46 (add-handle)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/corpus.go:26:13: guided

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream(&nats.ClientTrace{})
```

Template:

```go
js, err := nc.JetStream(&nats.ClientTrace{})
```

- Fact: option &nats.ClientTrace{} is not a direct call of a legacy option
- Fact: ClientTrace is not rewritten by the planner here

### Step S47 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. Waits on: migrate/scenarios/corpus.go:26:13.


#### Site migrate/scenarios/corpus.go:30:9: mechanical

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("T")
```

After:

```go
return jsNew.DeleteStream(context.Background(), "T")
```


### Step S48 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/scenarios/corpus.go:26:13.


#### Site migrate/scenarios/corpus.go:26:13: guided

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream(&nats.ClientTrace{})
```

Template:

```go
js, err := nc.JetStream(&nats.ClientTrace{})
```

- Fact: option &nats.ClientTrace{} is not a direct call of a legacy option
- Fact: ClientTrace is not rewritten by the planner here

### Step S49 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios/corpus.go:26:13.


## Component migrate/scenarios/corpus.go:35:37

Handles: js (migrate/scenarios/corpus.go:35:37). Safe to apply in one commit.


### Step S50 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/corpus.go:35:37: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S51 (site, machine edits)

nats.StreamConfig literal becomes jetstream.StreamConfig; nats.WorkQueuePolicy becomes jetstream.WorkQueuePolicy; JetStreamManager.AddStream becomes JetStream.CreateStream.


#### Site migrate/scenarios/corpus.go:36:9: mechanical

nats.StreamConfig literal becomes jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
cfg := nats.StreamConfig{Name: "R"}
```

After:

```go
cfg := jetstream.StreamConfig{Name: "R"}
```


#### Site migrate/scenarios/corpus.go:37:18: mechanical

nats.WorkQueuePolicy becomes jetstream.WorkQueuePolicy.

Before:

```go
cfg.Retention = nats.WorkQueuePolicy
```

After:

```go
cfg.Retention = jetstream.WorkQueuePolicy
```


#### Site migrate/scenarios/corpus.go:38:12: mechanical

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&cfg)
```

After:

```go
_, err := jsNew.CreateStream(ctx, cfg)
```


### Step S52 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/corpus.go:35:37: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S53 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/corpus.go:43:33

Handles: js (migrate/scenarios/corpus.go:43:33). Safe to apply in one commit.


### Step S54 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/corpus.go:43:33: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S55 (site, machine edits)

retype nats.ConsumerInfo as jetstream.ConsumerInfo; nats.ConsumerInfo becomes jetstream.ConsumerInfo; JetStreamManager.ConsumerInfo becomes Consumer.Info.


#### Site migrate/scenarios/corpus.go:43:60: mechanical

retype nats.ConsumerInfo as jetstream.ConsumerInfo. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
map[string]*nats.ConsumerInfo
```

After:

```go
map[string]*jetstream.ConsumerInfo
```


#### Site migrate/scenarios/corpus.go:44:26: mechanical

nats.ConsumerInfo becomes jetstream.ConsumerInfo. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
out := make(map[string]*nats.ConsumerInfo)
```

After:

```go
out := make(map[string]*jetstream.ConsumerInfo)
```


#### Site migrate/scenarios/corpus.go:45:15: mechanical

JetStreamManager.ConsumerInfo becomes Consumer.Info. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
info, err := js.ConsumerInfo("ORDERS", "w")
```

After:

```go
info, err := func() (*jetstream.ConsumerInfo, error) {
		consumer, err := jsNew.Consumer(ctx, "ORDERS", "w")
		if err != nil {
			return nil, err
		}
		return consumer.Info(ctx)
	}()
```

- Note: the new call first obtains a consumer handle, which sends a CONSUMER.INFO request

### Step S56 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/corpus.go:43:33: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S57 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/corpus.go:61:24

Handles: kv (migrate/scenarios/corpus.go:61:24). Safe to apply in one commit.


### Step S58 (add-handle, machine edits)

create the jetstream siblings kvNew next to the legacy handles.


#### Site migrate/scenarios/corpus.go:61:24: mechanical

declare kvNew jetstream.KeyValue next to kv. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv nats.KeyValue
```

After:

```go
kv nats.KeyValue, kvNew jetstream.KeyValue
```


### Step S59 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/corpus.go:61:24: mechanical

declare kvNew jetstream.KeyValue next to kv. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv nats.KeyValue
```

After:

```go
kv nats.KeyValue, kvNew jetstream.KeyValue
```


### Step S60 (rename, machine edits)

rename each sibling to its legacy name: kvNew → kv.


## Component migrate/scenarios/corpus.go:65:2

Handles: js (migrate/scenarios/corpus.go:65:2).


### Step S61 (add-handle)

create the jetstream siblings  next to the legacy handles.

- js at migrate/scenarios/corpus.go:65:2 cannot get a sibling: assigned from a multi-value call the planner does not rewrite

### Step S62 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. Waits on: S61.


#### Site migrate/scenarios/corpus.go:66:9: guided

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("A")
```

Template:

```go
return jsNew.DeleteStream(context.Background(), "A")
```

- Fact: the handle cannot be threaded: assigned from a multi-value call the planner does not rewrite

### Step S63 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: S61, migrate/scenarios/corpus.go:66:9.


### Step S64 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: S61, migrate/scenarios/corpus.go:66:9.


## Component migrate/scenarios/corpus.go:72:16

Handles: js (migrate/scenarios/corpus.go:72:16).


### Step S65 (add-handle)

create the jetstream siblings  next to the legacy handles.

- js at migrate/scenarios/corpus.go:72:16 cannot get a sibling: a caller passes it a value that is not a handle variable (fakeJS{})

#### Site migrate/scenarios/corpus.go:72:16: guided

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

Template:

```go
js nats.JetStreamContext
```

- Fact: the handle cannot be threaded: a caller passes it a value that is not a handle variable (fakeJS{})

### Step S66 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. Waits on: S65.


#### Site migrate/scenarios/corpus.go:73:9: guided

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("F")
```

Template:

```go
return jsNew.DeleteStream(context.Background(), "F")
```

- Fact: the handle cannot be threaded: a caller passes it a value that is not a handle variable (fakeJS{})

### Step S67 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: S65, migrate/scenarios/corpus.go:72:16, migrate/scenarios/corpus.go:73:9.


#### Site migrate/scenarios/corpus.go:72:16: guided

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

Template:

```go
js nats.JetStreamContext
```

- Fact: the handle cannot be threaded: a caller passes it a value that is not a handle variable (fakeJS{})

### Step S68 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: S65, migrate/scenarios/corpus.go:72:16, migrate/scenarios/corpus.go:73:9.


## Component migrate/scenarios/corpus.go:85:2

Handles: js (migrate/scenarios/corpus.go:85:2).


### Step S69 (add-handle)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/corpus.go:85:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := f.nc.JetStream()
```

After:

```go
js, err := f.nc.JetStream()
jsNew, err := jetstream.New(f.nc)
if err != nil {
	return nil, err
}
```


#### Site migrate/scenarios/corpus.go:89:9: guided

KeyValueManager.KeyValue creates a handle the planner cannot thread. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
return js.KeyValue(bucket)
```

Template:

```go
return js.KeyValue(bucket)
```

- Fact: the handle is not assigned to a variable next to its error; assign it (js, err := ...) so a sibling can be created

### Step S70 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/scenarios/corpus.go:89:9.


#### Site migrate/scenarios/corpus.go:85:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := f.nc.JetStream()
```

After:

```go
js, err := f.nc.JetStream()
jsNew, err := jetstream.New(f.nc)
if err != nil {
	return nil, err
}
```


#### Site migrate/scenarios/corpus.go:89:9: guided

KeyValueManager.KeyValue creates a handle the planner cannot thread. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
return js.KeyValue(bucket)
```

Template:

```go
return js.KeyValue(bucket)
```

- Fact: the handle is not assigned to a variable next to its error; assign it (js, err := ...) so a sibling can be created

### Step S71 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios/corpus.go:89:9.


## Component migrate/scenarios/corpus.go:94:2

Handles: kv (migrate/scenarios/corpus.go:94:2).


### Step S72 (add-handle)

create the jetstream siblings  next to the legacy handles.

- kv at migrate/scenarios/corpus.go:94:2 cannot get a sibling: assigned from a multi-value call the planner does not rewrite

### Step S73 (site)

KeyValue.Get becomes KeyValue.Get. Waits on: S72.


#### Site migrate/scenarios/corpus.go:98:12: guided

KeyValue.Get becomes KeyValue.Get. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
e, err := kv.Get("a")
```

Template:

```go
e, err := kvNew.Get(context.Background(), "a")
```

- Fact: the handle cannot be threaded: assigned from a multi-value call the planner does not rewrite

### Step S74 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: S72, migrate/scenarios/corpus.go:98:12.


### Step S75 (rename)

rename each sibling to its legacy name: kvNew → kv. Waits on: S72, migrate/scenarios/corpus.go:98:12.


## Component migrate/scenarios/scenarios.go:25:33

Handles: js (migrate/scenarios/scenarios.go:25:33). Safe to apply in one commit.


### Step S76 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:25:33: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S77 (site, machine edits)

retype nats.StreamConfig as jetstream.StreamConfig; JetStreamManager.AddStream becomes JetStream.CreateStream.


#### Site migrate/scenarios/scenarios.go:25:59: mechanical

retype nats.StreamConfig as jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
cfg nats.StreamConfig
```

After:

```go
cfg jetstream.StreamConfig
```


#### Site migrate/scenarios/scenarios.go:26:12: mechanical

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&cfg)
```

After:

```go
_, err := jsNew.CreateStream(ctx, cfg)
```


### Step S78 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:25:33: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S79 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:33:25

Handles: js (migrate/scenarios/scenarios.go:33:25). Safe to apply in one commit.


### Step S80 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:33:25: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S81 (site, machine edits)

retype nats.StreamConfig as jetstream.StreamConfig; JetStreamManager.AddStream becomes JetStream.CreateStream.


#### Site migrate/scenarios/scenarios.go:33:51: mechanical

retype nats.StreamConfig as jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
cfg nats.StreamConfig
```

After:

```go
cfg jetstream.StreamConfig
```


#### Site migrate/scenarios/scenarios.go:34:12: mechanical

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&cfg, nats.Context(r.ctx))
```

After:

```go
_, err := jsNew.CreateStream(r.ctx, cfg)
```


### Step S82 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:33:25: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S83 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:39:11

Handles: js (migrate/scenarios/scenarios.go:39:11). Safe to apply in one commit.


### Step S84 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:39:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S85 (site, machine edits)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream.


#### Site migrate/scenarios/scenarios.go:40:9: mechanical

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("S")
```

After:

```go
return jsNew.DeleteStream(context.Background(), "S")
```


### Step S86 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:39:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S87 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:44:12

Handles: js (migrate/scenarios/scenarios.go:44:12).


### Step S88 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:44:12: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S89 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream.


#### Site migrate/scenarios/scenarios.go:45:9: guided

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("S", nats.MaxWait(time.Second))
```

Template:

```go
return jsNew.DeleteStream(context.Background(), "S")
```

- Fact: a per-call nats.MaxWait becomes a context with that timeout: ctx, cancel := context.WithTimeout(ctx, d); defer cancel()

### Step S90 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/scenarios/scenarios.go:45:9.


#### Site migrate/scenarios/scenarios.go:44:12: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S91 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios/scenarios.go:45:9.


## Component migrate/scenarios/scenarios.go:50:36

Handles: js (migrate/scenarios/scenarios.go:50:36). Safe to apply in one commit.


### Step S92 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:50:36: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S93 (site, machine edits)

JetStreamManager.AddConsumer becomes JetStream.CreatePushConsumer.


#### Site migrate/scenarios/scenarios.go:51:12: mechanical

JetStreamManager.AddConsumer becomes JetStream.CreatePushConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", Heartbeat: time.Second})
```

After:

```go
_, err := jsNew.CreatePushConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", IdleHeartbeat: time.Second, AckPolicy: jetstream.AckNonePolicy})
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

### Step S94 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:50:36: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S95 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:56:11

Handles: js (migrate/scenarios/scenarios.go:56:11). Safe to apply in one commit.


### Step S96 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:56:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S97 (site, machine edits)

JetStreamManager.StreamNameBySubject becomes JetStream.StreamNameBySubject.


#### Site migrate/scenarios/scenarios.go:57:9: mechanical

JetStreamManager.StreamNameBySubject becomes JetStream.StreamNameBySubject. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.StreamNameBySubject("orders.new")
```

After:

```go
return jsNew.StreamNameBySubject(context.Background(), "orders.new")
```


### Step S98 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:56:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S99 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:61:12

Handles: js (migrate/scenarios/scenarios.go:61:12). Safe to apply in one commit.


### Step S100 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:61:12: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S101 (site, machine edits)

PullSubscribe becomes a jetstream.Consumer.


#### Site migrate/scenarios/scenarios.go:62:14: mechanical

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := js.PullSubscribe("orders.new", "", nats.Durable("w"), nats.BindStream("ORDERS"), nats.DeliverNew(), nats.MaxAckPending(100))
```

After:

```go
_, err := jsNew.CreateOrUpdateConsumer(context.Background(), "ORDERS", jetstream.ConsumerConfig{
		Durable: "w",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		MaxAckPending: 100,
		FilterSubject: "orders.new",
	})
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it

### Step S102 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:61:12: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S103 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:71:11

Handles: js (migrate/scenarios/scenarios.go:71:11). Safe to apply in one commit.


### Step S104 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:71:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S105 (site, machine edits)

PullSubscribe becomes a jetstream.Consumer.


#### Site migrate/scenarios/scenarios.go:72:14: mechanical

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := js.PullSubscribe("orders.new", "w")
```

After:

```go
_, err := func() (jetstream.Consumer, error) {
		stream, err := jsNew.StreamNameBySubject(context.Background(), "orders.new")
		if err != nil {
			return nil, err
		}
		return jsNew.CreateOrUpdateConsumer(context.Background(), stream, jetstream.ConsumerConfig{
			Durable: "w",
			FilterSubject: "orders.new",
		})
	}()
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it

### Step S106 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:71:11: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S107 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:81:35

Handles: js (migrate/scenarios/scenarios.go:81:35).


### Step S108 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:81:35: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S109 (site)

Subscribe becomes a pull consumer's Consume.


#### Site migrate/scenarios/scenarios.go:82:12: decision

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
_, err := js.Subscribe("billing.new", func(m *nats.Msg) {}, nats.Durable("billing"))
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle
- Decision **ack** pending, default **after-handler**: legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior.
  - after-handler: ack after the handler returns, as the legacy wrapper did
  - explicit: ack explicitly on each handler path
  - none: AckNonePolicy: no acks at all

### Step S110 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/scenarios/scenarios.go:82:12.


#### Site migrate/scenarios/scenarios.go:81:35: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S111 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios/scenarios.go:82:12.


## Component migrate/scenarios/scenarios.go:88:2

Handles: js (migrate/scenarios/scenarios.go:88:2), kv (migrate/scenarios/scenarios.go:92:2). Safe to apply in one commit.


### Step S112 (add-handle, machine edits)

create the jetstream siblings jsNew, kvNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:88:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return nil, err
}
```


#### Site migrate/scenarios/scenarios.go:92:13: mechanical

KeyValueManager.CreateKeyValue becomes JetStream.CreateKeyValue. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
```

After:

```go
kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
kvNew, err := jsNew.KeyValue(context.Background(), "settings")
if err != nil {
	return nil, err
}
```


### Step S113 (site, machine edits)

KeyValue.Get becomes KeyValue.Get.


#### Site migrate/scenarios/scenarios.go:96:12: mechanical

KeyValue.Get becomes KeyValue.Get. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
e, err := kv.Get("a")
```

After:

```go
e, err := kvNew.Get(context.Background(), "a")
```


### Step S114 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:88:13: mechanical

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := nc.JetStream()
jsNew, err := jetstream.New(nc)
if err != nil {
	return nil, err
}
```


#### Site migrate/scenarios/scenarios.go:92:13: mechanical

KeyValueManager.CreateKeyValue becomes JetStream.CreateKeyValue. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
```

After:

```go
kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
kvNew, err := jsNew.KeyValue(context.Background(), "settings")
if err != nil {
	return nil, err
}
```


### Step S115 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js, kvNew → kv.


## Component migrate/scenarios/scenarios.go:104:34

Handles: js (migrate/scenarios/scenarios.go:104:34). Safe to apply in one commit.


### Step S116 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:104:34: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S117 (site, machine edits)

nats.ConsumerConfig literal becomes jetstream.ConsumerConfig; nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy; JetStreamManager.AddConsumer becomes JetStream.CreateConsumer.


#### Site migrate/scenarios/scenarios.go:105:9: mechanical

nats.ConsumerConfig literal becomes jetstream.ConsumerConfig. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
```

After:

```go
cfg := jetstream.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
```


#### Site migrate/scenarios/scenarios.go:105:54: mechanical

nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy.

Before:

```go
cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
```

After:

```go
cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: jetstream.AckExplicitPolicy}
```


#### Site migrate/scenarios/scenarios.go:107:12: mechanical

JetStreamManager.AddConsumer becomes JetStream.CreateConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := js.AddConsumer("ORDERS", &cfg)
```

After:

```go
_, err := jsNew.CreateConsumer(ctx, "ORDERS", cfg)
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

### Step S118 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:104:34: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S119 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:112:34

Handles: js (migrate/scenarios/scenarios.go:112:34). Safe to apply in one commit.


### Step S120 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:112:34: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S121 (site, machine edits)

nats.AckNonePolicy becomes jetstream.AckNonePolicy; nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy; JetStreamManager.AddConsumer becomes JetStream.CreateConsumer.


#### Site migrate/scenarios/scenarios.go:113:9: mechanical

nats.AckNonePolicy becomes jetstream.AckNonePolicy.

Before:

```go
ack := nats.AckNonePolicy
```

After:

```go
ack := jetstream.AckNonePolicy
```


#### Site migrate/scenarios/scenarios.go:115:9: mechanical

nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy.

Before:

```go
ack = nats.AckExplicitPolicy
```

After:

```go
ack = jetstream.AckExplicitPolicy
```


#### Site migrate/scenarios/scenarios.go:117:12: mechanical

JetStreamManager.AddConsumer becomes JetStream.CreateConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "p", AckPolicy: ack})
```

After:

```go
_, err := jsNew.CreateConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "p", AckPolicy: ack})
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

### Step S122 (remove-legacy, machine edits)

remove the legacy handles, their roots and the values threaded into them.


#### Site migrate/scenarios/scenarios.go:112:34: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S123 (rename, machine edits)

rename each sibling to its legacy name: jsNew → js.


## Component migrate/scenarios/scenarios.go:122:31

Handles: js (migrate/scenarios/scenarios.go:122:31).


### Step S124 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.


#### Site migrate/scenarios/scenarios.go:122:31: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S125 (site)

Subscribe becomes a pull consumer's Consume.


#### Site migrate/scenarios/scenarios.go:123:12: decision

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
_, err := js.Subscribe("orders.all", func(m *nats.Msg) {}, nats.Durable("all"), nats.AckAll())
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle
- Decision **ack** pending, default **after-handler**: legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior.
  - after-handler: ack after the handler returns, as the legacy wrapper did
  - explicit: ack explicitly on each handler path
  - none: AckNonePolicy: no acks at all

### Step S126 (remove-legacy)

remove the legacy handles, their roots and the values threaded into them. Waits on: migrate/scenarios/scenarios.go:123:12.


#### Site migrate/scenarios/scenarios.go:122:31: mechanical

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S127 (rename)

rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios/scenarios.go:123:12.


## Sites outside components

### Step S128 (site)

declare a jetstream.JetStream sibling next to the nats.JetStreamContext handle.


#### Site migrate/legacyonly/legacyonly.go:18:29: guided

declare a jetstream.JetStream sibling next to the nats.JetStreamContext handle. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
nats.JetStreamContext
```

Template:

```go
nats.JetStreamContext
```

- Fact: an unnamed JetStreamContext declaration; name it to thread a jetstream.JetStream next to it

### Step S129 (site)

retype nats.StreamConfig as jetstream.StreamConfig; nats.StreamConfig literal becomes jetstream.StreamConfig; retype nats.RetentionPolicy as jetstream.RetentionPolicy; retype nats.StorageType as jetstream.StorageType; retype nats.DiscardPolicy as jetstream.DiscardPolicy; nats.LimitsPolicy becomes jetstream.LimitsPolicy; nats.FileStorage becomes jetstream.FileStorage; nats.DiscardOld becomes jetstream.DiscardOld.

- retention changes type from nats.RetentionPolicy to jetstream.RetentionPolicy; review its use in tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
- storage changes type from nats.StorageType to jetstream.StorageType; review its use in tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
- discard changes type from nats.DiscardPolicy to jetstream.DiscardPolicy; review its use in tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}

#### Site migrate/order/order.go:37:15: mechanical

retype nats.StreamConfig as jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
nats.StreamConfig
```

After:

```go
jetstream.StreamConfig
```


#### Site migrate/order/order.go:37:42: mechanical

nats.StreamConfig literal becomes jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return nats.StreamConfig{}
```

After:

```go
return jetstream.StreamConfig{}
```


#### Site migrate/order/order.go:41:3: mechanical

retype nats.RetentionPolicy as jetstream.RetentionPolicy.

Before:

```go
retention nats.RetentionPolicy
```

After:

```go
retention jetstream.RetentionPolicy
```


#### Site migrate/order/order.go:42:3: mechanical

retype nats.StorageType as jetstream.StorageType.

Before:

```go
storage   nats.StorageType
```

After:

```go
storage   jetstream.StorageType
```


#### Site migrate/order/order.go:43:3: mechanical

retype nats.DiscardPolicy as jetstream.DiscardPolicy.

Before:

```go
discard   nats.DiscardPolicy
```

After:

```go
discard   jetstream.DiscardPolicy
```


#### Site migrate/order/order.go:45:15: mechanical

nats.LimitsPolicy becomes jetstream.LimitsPolicy.

Before:

```go
tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
```

After:

```go
tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: jetstream.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
```


#### Site migrate/order/order.go:45:43: mechanical

nats.FileStorage becomes jetstream.FileStorage.

Before:

```go
tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
```

After:

```go
tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: jetstream.FileStorage, discard: nats.DiscardOld},
	}
```


#### Site migrate/order/order.go:45:70: mechanical

nats.DiscardOld becomes jetstream.DiscardOld.

Before:

```go
tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
```

After:

```go
tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: jetstream.DiscardOld},
	}
```


### Step S130 (site)

retype nats.KeyValueEntry as jetstream.KeyValueEntry.


#### Site migrate/scenarios/corpus.go:58:26: guided

retype nats.KeyValueEntry as jetstream.KeyValueEntry. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
<-chan nats.KeyValueEntry
```

Template:

```go
<-chan jetstream.KeyValueEntry
```

- Fact: the method belongs to an implementation of nats.KeyWatcher, whose signature would no longer match; migrate the implementation with the interface (regenerate a mock from jetstream.KeyWatcher)

### Step S131 (site)

retype nats.KeyWatcher as jetstream.KeyWatcher.


#### Site migrate/scenarios/corpus.go:61:42: guided

retype nats.KeyWatcher as jetstream.KeyWatcher. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
nats.KeyWatcher
```

Template:

```go
jetstream.KeyWatcher
```

- Fact: the method belongs to an implementation of nats.KeyWatcher, whose signature would no longer match; migrate the implementation with the interface (regenerate a mock from jetstream.KeyWatcher)

### Step S132 (site)

nats.JetStreamContext becomes jetstream.JetStream.


#### Site migrate/scenarios/corpus.go:65:14: guided

nats.JetStreamContext becomes jetstream.JetStream. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, _ := v.(nats.JetStreamContext)
```

Template:

```go
js, _ := v.(jetstream.JetStream)
```

- Fact: nats.JetStreamContext handles migrate through their declarations and roots; this expression makes one the planner does not thread

### Step S133 (site)

declare a jetstream.JetStream sibling next to the nats.JetStreamContext handle.


#### Site migrate/scenarios/corpus.go:70:21: guided

declare a jetstream.JetStream sibling next to the nats.JetStreamContext handle. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
nats.JetStreamContext
```

Template:

```go
nats.JetStreamContext
```

- Fact: an unnamed JetStreamContext declaration; name it to thread a jetstream.JetStream next to it

### Step S134 (site)

declare a jetstream.KeyValue sibling next to the nats.KeyValue handle.


#### Site migrate/scenarios/corpus.go:84:39: guided

declare a jetstream.KeyValue sibling next to the nats.KeyValue handle. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
nats.KeyValue
```

Template:

```go
nats.KeyValue
```

- Fact: an unnamed KeyValue declaration; name it to thread a jetstream.KeyValue next to it

### Step S135 (site)

Conn.JetStream creates a handle the planner cannot thread.


#### Site migrate/legacyonly/legacyonly.go:19:9: unmapped

Conn.JetStream creates a handle the planner cannot thread. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
return nc.JetStream(nats.UseLegacyDurableConsumers())
```

- Fact: the handle is not assigned to a variable next to its error; assign it (js, err := ...) so a sibling can be created
- Note: UseLegacyDurableConsumers: the jetstream package always uses the consumer create API; there is no legacy mode

## Follow-ups

Not needed to finish the migration:

- migrate/app/app.go:73:17 (lister-error): check the error of the StreamNames lister after ranging over Name()
- migrate/app/subscribe.go:28:14 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app/subscribe.go:55:14 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app/subscribe.go:68:14 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app/subscribe.go:81:14 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app/subscribe.go:101:14 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app/subscribe.go:114:12 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app/subscribe.go:127:12 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/order/order.go:27:14 (stream-name): replace the runtime StreamNameBySubject lookup with the stream's name
- migrate/scenarios/scenarios.go:72:14 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/scenarios/scenarios.go:82:12 (stream-name): replace the runtime StreamNameBySubject lookup with the stream's name
- migrate/scenarios/scenarios.go:123:12 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
