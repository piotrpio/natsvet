# Migration plan for natsvet/testdata

Read `natsvet migrate skill` before acting on this plan. Plan schema version 2; mapping verified against nats.go v1.53.1; the module requires v1.53.1.

## Summary

- Legacy uses: 155 in 118 sites
- Mechanical: 90, guided: 18, decision: 9, unmapped: 1, skipped: 0
- Components: 35, steps: 83

## Pending decisions

Ask the user each question, record the answer in natsvet-migrate.json, and plan again.

- **ack** (module, 5 sites): legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior. Options: after-handler, explicit, none. Default: **after-handler**.
- **channel-max-ack-pending** (module, 1 site): legacy set MaxAckPending to the channel capacity. Options: keep, server-default. Default: **keep**.
- **shared-handler** (module, 1 site): the handler also serves a core subscription, which keeps receiving *nats.Msg. Options: split, adapter. Default: **split**.
- **subscribe-target** (module, 9 sites): the jetstream package is built around pull consumers, which is why teams migrate. Options: pull, push, defer. Default: **pull**.
- **subscribe-target** (site:migrate/app.Service.Queue#JetStream.QueueSubscribe@47abb8, 1 site): the subscription binds an existing consumer, which legacy Subscribe requires to be a push consumer; pull would fail until the consumer is recreated. Options: pull, push, defer. Default: **push**.
- **component** (module, 35 components): skip keeps code that must stay on the legacy API (tests of the legacy API, compatibility shims) out of the steps; the steps assume migrate until answered. Options: migrate, skip. Default: **migrate**.
  - migrate/app.Service.JS: handles JS (migrate/app/app.go:27:2), kv (migrate/app/app.go:28:2), js (migrate/app/app.go:32:2), kv (migrate/app/app.go:36:2); files migrate/app/app.go, migrate/app/subscribe.go, migrate/worker/worker.go; functions migrate/app.New, migrate/app.Service.Batch, migrate/app.Service.Channel, migrate/app.Service.Limited, migrate/app.Service.Next, migrate/app.Service.Publish, migrate/app.Service.Purge, migrate/app.Service.Queue, migrate/app.Service.Run, migrate/app.Service.Setting, migrate/app.Service.Setup, migrate/app.Service.Shared, migrate/app.Service.StreamNames, migrate/app.Service.Tail, migrate/worker.Done
  - migrate/boundary.Hand#js: handles js (migrate/boundary/boundary.go:23:2); files migrate/boundary/boundary.go; functions migrate/boundary.Hand
  - migrate/fieldfmt.Server.JS: handles JS (migrate/fieldfmt/fieldfmt.go:22:2), js (migrate/fieldfmt/fieldfmt.go:27:2); files migrate/fieldfmt/fieldfmt.go, migrate/fieldfmt/use/use.go; functions migrate/fieldfmt.New, migrate/fieldfmt/use.Streams
  - migrate/fieldfmt.Report#js: handles js (migrate/fieldfmt/report.go:26:2); files migrate/fieldfmt/report.go; functions migrate/fieldfmt.Report
  - migrate/guided.Drain#js: handles js (migrate/guided/guided.go:21:2); files migrate/guided/guided.go; functions migrate/guided.Drain
  - migrate/independent.Cleanup#js: handles js (migrate/independent/independent.go:23:2); files migrate/independent/independent.go; functions migrate/independent.Cleanup
  - migrate/independent.Account#js: handles js (migrate/independent/independent.go:31:2); files migrate/independent/independent.go; functions migrate/independent.Account
  - migrate/legacytests.TestLegacyAddStream#js: handles js (migrate/legacytests/legacy_test.go:27:2); files migrate/legacytests/legacy_test.go; functions migrate/legacytests.TestLegacyAddStream; test code only
  - migrate/multifile.Streams#js: handles js (migrate/multifile/handle.go:21:2); files migrate/multifile/handle.go; functions migrate/multifile.Streams
  - migrate/neighbors.Legacy#js: handles js (migrate/neighbors/legacy.go:19:2); files migrate/neighbors/legacy.go; functions migrate/neighbors.Legacy
  - migrate/neighbors.Subscribe#js: handles js (migrate/neighbors/neighbors.go:26:16); files migrate/neighbors/neighbors.go; functions migrate/neighbors.Subscribe
  - migrate/objects.Store#js: handles js (migrate/objects/objects.go:23:2), obj (migrate/objects/objects.go:27:2); files migrate/objects/objects.go; functions migrate/objects.Store
  - migrate/order.Pull#js: handles js (migrate/order/order.go:26:11); files migrate/order/order.go; functions migrate/order.Pull
  - migrate/scenarios.Traced#js: handles js (migrate/scenarios/corpus.go:26:2); files migrate/scenarios/corpus.go; functions migrate/scenarios.Traced
  - migrate/scenarios.Retention#js: handles js (migrate/scenarios/corpus.go:35:37); files migrate/scenarios/corpus.go; functions migrate/scenarios.Retention
  - migrate/scenarios.Infos#js: handles js (migrate/scenarios/corpus.go:43:33); files migrate/scenarios/corpus.go; functions migrate/scenarios.Infos
  - migrate/scenarios.watcher.Watch#kv: handles kv (migrate/scenarios/corpus.go:61:24); files migrate/scenarios/corpus.go; functions migrate/scenarios.watcher.Watch
  - migrate/scenarios.Asserted#js: handles js (migrate/scenarios/corpus.go:65:2); files migrate/scenarios/corpus.go; functions migrate/scenarios.Asserted
  - migrate/scenarios.useHandle#js: handles js (migrate/scenarios/corpus.go:72:16); files migrate/scenarios/corpus.go; functions migrate/scenarios.useHandle
  - migrate/scenarios.framework.KV#js: handles js (migrate/scenarios/corpus.go:85:2); files migrate/scenarios/corpus.go; functions migrate/scenarios.framework.KV
  - migrate/scenarios.Helper#kv: handles kv (migrate/scenarios/corpus.go:94:2); files migrate/scenarios/corpus.go; functions migrate/scenarios.Helper
  - migrate/scenarios.Setup#js: handles js (migrate/scenarios/scenarios.go:25:33); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Setup
  - migrate/scenarios.request.Create#js: handles js (migrate/scenarios/scenarios.go:33:25); files migrate/scenarios/scenarios.go; functions migrate/scenarios.request.Create
  - migrate/scenarios.Drop#js: handles js (migrate/scenarios/scenarios.go:39:11); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Drop
  - migrate/scenarios.Timed#js: handles js (migrate/scenarios/scenarios.go:44:12); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Timed
  - migrate/scenarios.Consumer#js: handles js (migrate/scenarios/scenarios.go:50:36); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Consumer
  - migrate/scenarios.Name#js: handles js (migrate/scenarios/scenarios.go:56:11); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Name
  - migrate/scenarios.Bound#js: handles js (migrate/scenarios/scenarios.go:61:12); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Bound
  - migrate/scenarios.Pull#js: handles js (migrate/scenarios/scenarios.go:71:11); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Pull
  - migrate/scenarios.Billing#js: handles js (migrate/scenarios/scenarios.go:81:35); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Billing
  - migrate/scenarios.Buckets#js: handles js (migrate/scenarios/scenarios.go:88:2), kv (migrate/scenarios/scenarios.go:92:2); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Buckets
  - migrate/scenarios.Config#js: handles js (migrate/scenarios/scenarios.go:104:34); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Config
  - migrate/scenarios.Policy#js: handles js (migrate/scenarios/scenarios.go:112:34); files migrate/scenarios/scenarios.go; functions migrate/scenarios.Policy
  - migrate/scenarios.All#js: handles js (migrate/scenarios/scenarios.go:122:31); files migrate/scenarios/scenarios.go; functions migrate/scenarios.All
  - migrate/unrelated.Legacy#js: handles js (migrate/unrelated/unrelated.go:38:2); files migrate/unrelated/unrelated.go; functions migrate/unrelated.Legacy

## Component migrate/app.Service.JS

Handles: JS (migrate/app/app.go:27:2), kv (migrate/app/app.go:28:2), js (migrate/app/app.go:32:2), kv (migrate/app/app.go:36:2).


### Step S0 (add-handle, machine edits)

create the jetstream siblings JSNew, kvNew, jsNew, kvNew next to the legacy handles.

Before:

```go
// migrate/app/app.go
)
...
	JS nats.JetStreamContext
	kv nats.KeyValue
}
...
	kv, err := js.KeyValue("config")
...
	return &Service{JS: js, kv: kv}, nil
```

After:

```go
// migrate/app/app.go
	"github.com/nats-io/nats.go/jetstream"
)
...
	JS    nats.JetStreamContext
	JSNew jetstream.JetStream
	kv    nats.KeyValue
	kvNew jetstream.KeyValue
}
...
	jsNew, err := jetstream.New(nc, jetstream.WithDefaultTimeout(2*time.Second))
	if err != nil {
		return nil, err
	}
	_ = js
	kv, err := js.KeyValue("config")
...
	kvNew, err := jsNew.KeyValue(context.Background(), "config")
	if err != nil {
		return nil, err
	}
	_ = kv
	return &Service{JS: js, JSNew: jsNew, kv: kv, kvNew: kvNew}, nil
```

#### Site migrate/app#JetStreamContext@ddbd95: mechanical

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


#### Site migrate/app#KeyValue@fc1b43: mechanical

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


#### Site migrate/app.New#Conn.JetStream@52e530: mechanical in migrate/app.New

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


#### Site migrate/app.New#KeyValueManager.KeyValue@b79a66: mechanical in migrate/app.New

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


### Step S1 (site, machine edits)

JetStreamManager.AddStream becomes JetStream.CreateStream.

#### Site migrate/app.Service.Setup#JetStreamManager.AddStream@54d273: mechanical in migrate/app.Service.Setup

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

#### Site migrate/app.Service.Setup#JetStreamManager.AddConsumer@3f1ce3: mechanical in migrate/app.Service.Setup

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

#### Site migrate/app.Service.Purge#JetStreamManager.PurgeStream@947942: mechanical in migrate/app.Service.Purge

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

#### Site migrate/app.Service.Publish#JetStream.Publish@2237c7: mechanical in migrate/app.Service.Publish

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

KeyValue.Get becomes KeyValue.Get; nats.ErrKeyNotFound becomes jetstream.ErrKeyNotFound with the call at migrate/app.Service.Setting#KeyValue.Get@b6f3ac, whose error it checks.

#### Site migrate/app.Service.Setting#KeyValue.Get@b6f3ac: mechanical in migrate/app.Service.Setting

KeyValue.Get becomes KeyValue.Get. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
e, err := s.kv.Get(key)
```

After:

```go
e, err := s.kvNew.Get(context.Background(), key)
```


#### Site migrate/app.Service.Setting#@d92432: mechanical in migrate/app.Service.Setting

nats.ErrKeyNotFound becomes jetstream.ErrKeyNotFound with the call at migrate/app.Service.Setting#KeyValue.Get@b6f3ac, whose error it checks.

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

#### Site migrate/app.Service.StreamNames#JetStreamManager.StreamNames@4bf32c: mechanical in migrate/app.Service.StreamNames

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

#### Site migrate/app.Service.Tail#JetStream.Subscribe@032ec2: mechanical in migrate/app.Service.Tail

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

#### Site migrate/worker.Done#JetStream.Publish@7bd4c9: mechanical in migrate/worker.Done

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

#### Site migrate/app.Service.Batch#JetStream.PullSubscribe@80357c: guided in migrate/app.Service.Batch

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

#### Site migrate/app.Service.Next#JetStream.SubscribeSync@c4096c: guided in migrate/app.Service.Next

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

#### Site migrate/app.Service.Channel#JetStream.ChanSubscribe@b4a2a8: guided in migrate/app.Service.Channel

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

#### Site migrate/app.Service.Run#JetStream.Subscribe@138e9e: decision in migrate/app.Service.Run

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

#### Site migrate/app.Service.Queue#JetStream.QueueSubscribe@47abb8: decision in migrate/app.Service.Queue

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

#### Site migrate/app.Service.Queue#Msg.Ack@38ab3e: decision in migrate/app.Service.Queue

rewritten with the handler of migrate/app.Service.Queue#JetStream.QueueSubscribe@47abb8. See jetstream/MIGRATION.md#message-acknowledgement.

Before:

```go
m.Ack()
```

- Fact: the message is a handler parameter: the call migrates with the subscription that delivers it, once the handler takes a jetstream.Msg (becomes jetstream Msg.Ack)

### Step S14 (site)

Subscribe becomes a pull consumer's Consume.

#### Site migrate/app.Service.Limited#JetStream.Subscribe@ce9a15: decision in migrate/app.Service.Limited

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

#### Site migrate/app.Service.Shared#JetStream.Subscribe@1b480a: decision in migrate/app.Service.Shared

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

### Step S16 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: JSNew → JS, kvNew → kv, jsNew → js, kvNew → kv. Waits on: migrate/app.Service.Batch#JetStream.PullSubscribe@80357c, migrate/app.Service.Next#JetStream.SubscribeSync@c4096c, migrate/app.Service.Channel#JetStream.ChanSubscribe@b4a2a8, migrate/app.Service.Run#JetStream.Subscribe@138e9e, migrate/app.Service.Queue#JetStream.QueueSubscribe@47abb8, migrate/app.Service.Queue#Msg.Ack@38ab3e, migrate/app.Service.Limited#JetStream.Subscribe@ce9a15, migrate/app.Service.Shared#JetStream.Subscribe@1b480a.

## Component migrate/boundary.Hand#js

Handles: js (migrate/boundary/boundary.go:23:2).

- Blocked at migrate/boundary/boundary.go:27:10: passed to natsvet/testdata/migrateext/lib.Use, outside the loaded packages; the finish step is omitted.

### Step S17 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/boundary/boundary.go

...
	lib.Use(js)
```

After:

```go
// migrate/boundary/boundary.go
	"github.com/nats-io/nats.go/jetstream"

...
	jsNew, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	_ = js
	_ = jsNew
	lib.Use(js)
```

#### Site migrate/boundary.Hand#Conn.JetStream@dd7995: mechanical in migrate/boundary.Hand

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


### Step S18 (site, machine edits)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream.

#### Site migrate/boundary.Hand#JetStreamManager.DeleteStream@b938fa: mechanical in migrate/boundary.Hand

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("TMP")
```

After:

```go
return jsNew.DeleteStream(context.Background(), "TMP")
```


## Component migrate/fieldfmt.Server.JS

Handles: JS (migrate/fieldfmt/fieldfmt.go:22:2), js (migrate/fieldfmt/fieldfmt.go:27:2).


### Step S19 (add-handle, machine edits)

create the jetstream siblings JSNew, jsNew next to the legacy handles.

Before:

```go
// migrate/fieldfmt/fieldfmt.go
import "github.com/nats-io/nats.go"
...
	NC   *nats.Conn
	JS   nats.JetStreamContext
	Name string
...
	return &Server{NC: nc, JS: js, Name: "fieldfmt"}, nil
```

After:

```go
// migrate/fieldfmt/fieldfmt.go
import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)
...
	NC    *nats.Conn
	JS    nats.JetStreamContext
	JSNew jetstream.JetStream
	Name  string
...
	jsNew, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}
	_ = js
	return &Server{NC: nc, JS: js, JSNew: jsNew, Name: "fieldfmt"}, nil
```

#### Site migrate/fieldfmt#JetStreamContext@ddbd95: mechanical

declare JSNew jetstream.JetStream next to JS. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
JS   nats.JetStreamContext
```

After:

```go
JS   nats.JetStreamContext
JSNew jetstream.JetStream
```


#### Site migrate/fieldfmt.New#Conn.JetStream@dd7995: mechanical in migrate/fieldfmt.New

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


### Step S20 (site, machine edits)

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

#### Site migrate/fieldfmt/use.Streams#JetStreamManager.AccountInfo@803d6e: mechanical in migrate/fieldfmt/use.Streams

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := s.JS.AccountInfo()
```

After:

```go
info, err := s.JSNew.AccountInfo(context.Background())
```


### Step S21 (finish, machine edits)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: JSNew → JS, jsNew → js.

Before:

```go
// migrate/fieldfmt/fieldfmt.go
	NC    *nats.Conn
	JS    nats.JetStreamContext
	JSNew jetstream.JetStream
	Name  string
...
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	jsNew, err := jetstream.New(nc)
...
	_ = js
	return &Server{NC: nc, JS: js, JSNew: jsNew, Name: "fieldfmt"}, nil
// migrate/fieldfmt/use/use.go
	info, err := s.JSNew.AccountInfo(context.Background())
```

After:

```go
// migrate/fieldfmt/fieldfmt.go
	NC   *nats.Conn
	JS   jetstream.JetStream
	Name string
...
	js, err := jetstream.New(nc)
...
	return &Server{NC: nc, JS: js, Name: "fieldfmt"}, nil
// migrate/fieldfmt/use/use.go
	info, err := s.JS.AccountInfo(context.Background())
```

## Component migrate/fieldfmt.Report#js

Handles: js (migrate/fieldfmt/report.go:26:2). Safe to apply in one commit.


### Step S22 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
// migrate/fieldfmt/report.go
	"os"
...
)
...
	js, err := nc.JetStream()
...
	info, err := js.AccountInfo()
```

After:

```go
// migrate/fieldfmt/report.go
	"context"
	"os"
...
	"github.com/nats-io/nats.go/jetstream"
)
...
	js, err := jetstream.New(nc)
...
	info, err := js.AccountInfo(context.Background())
```

#### Site migrate/fieldfmt.Report#Conn.JetStream@dd7995: mechanical in migrate/fieldfmt.Report

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/fieldfmt.Report#JetStreamManager.AccountInfo@3e7dd1: mechanical in migrate/fieldfmt.Report

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := js.AccountInfo(context.Background())
```


## Component migrate/guided.Drain#js

Handles: js (migrate/guided/guided.go:21:2).


### Step S23 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/guided/guided.go
import "github.com/nats-io/nats.go"
...
	info, err := js.AccountInfo()
```

After:

```go
// migrate/guided/guided.go
import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)
...
	jsNew, err := jetstream.New(nc)
	if err != nil {
		return 0, err
	}
	_ = js
	_ = jsNew
	info, err := js.AccountInfo()
```

#### Site migrate/guided.Drain#Conn.JetStream@dd7995: mechanical in migrate/guided.Drain

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


### Step S24 (site, machine edits)

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

#### Site migrate/guided.Drain#JetStreamManager.AccountInfo@3e7dd1: mechanical in migrate/guided.Drain

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := jsNew.AccountInfo(context.Background())
```


### Step S25 (site)

PullSubscribe becomes a jetstream.Consumer.

#### Site migrate/guided.Drain#JetStream.PullSubscribe@f84679: guided in migrate/guided.Drain

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := js.PullSubscribe("orders.new", "worker", nats.BindStream("ORDERS"))
```

Template:

```go
sub, err := jsNew.CreateOrUpdateConsumer(context.Background(), "ORDERS", jetstream.ConsumerConfig{
		Durable: "worker",
		FilterSubject: "orders.new",
	})

batch, err := sub.Fetch(10)
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

### Step S26 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/guided.Drain#JetStream.PullSubscribe@f84679.

## Component migrate/independent.Cleanup#js

Handles: js (migrate/independent/independent.go:23:2). Safe to apply in one commit.


### Step S27 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.DeleteStream becomes JetStream.DeleteStream.

Before:

```go
// migrate/independent/independent.go
)
...
	js, err := nc.JetStream()
...
	return js.DeleteStream("OLD")
```

After:

```go
// migrate/independent/independent.go
	"github.com/nats-io/nats.go/jetstream"
)
...
	js, err := jetstream.New(nc)
...
	return js.DeleteStream(context.Background(), "OLD")
```

#### Site migrate/independent.Cleanup#Conn.JetStream@dd7995: mechanical in migrate/independent.Cleanup

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/independent.Cleanup#JetStreamManager.DeleteStream@fd4335: mechanical in migrate/independent.Cleanup

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("OLD")
```

After:

```go
return js.DeleteStream(context.Background(), "OLD")
```


## Component migrate/independent.Account#js

Handles: js (migrate/independent/independent.go:31:2). Safe to apply in one commit.


### Step S28 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.DeleteConsumer becomes JetStream.DeleteConsumer; JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
// migrate/independent/independent.go
	js, err := nc.JetStream()
...
	if err := js.DeleteConsumer("ORDERS", "stale", nats.Context(ctx)); err != nil {
...
	info, err := js.AccountInfo()
```

After:

```go
// migrate/independent/independent.go
	js, err := jetstream.New(nc)
...
	if err := js.DeleteConsumer(ctx, "ORDERS", "stale"); err != nil {
...
	info, err := js.AccountInfo(ctx)
```

#### Site migrate/independent.Account#Conn.JetStream@dd7995: mechanical in migrate/independent.Account

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/independent.Account#JetStreamManager.DeleteConsumer@8c832d: mechanical in migrate/independent.Account

JetStreamManager.DeleteConsumer becomes JetStream.DeleteConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
err := js.DeleteConsumer("ORDERS", "stale", nats.Context(ctx))
```

After:

```go
err := js.DeleteConsumer(ctx, "ORDERS", "stale")
```


#### Site migrate/independent.Account#JetStreamManager.AccountInfo@3e7dd1: mechanical in migrate/independent.Account

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := js.AccountInfo(ctx)
```


## Component migrate/legacytests.TestLegacyAddStream#js

Handles: js (migrate/legacytests/legacy_test.go:27:2). Safe to apply in one commit. Test code only.


### Step S29 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.AddStream becomes JetStream.CreateStream.

Before:

```go
// migrate/legacytests/legacy_test.go
	"testing"
...
)
...
	js, err := nc.JetStream()
...
	if _, err := js.AddStream(&nats.StreamConfig{Name: "LEGACY"}); err != nil {
```

After:

```go
// migrate/legacytests/legacy_test.go
	"context"
	"testing"
...
	"github.com/nats-io/nats.go/jetstream"
)
...
	js, err := jetstream.New(nc)
...
	if _, err := js.CreateStream(context.Background(), jetstream.StreamConfig{Name: "LEGACY"}); err != nil {
```

#### Site migrate/legacytests.TestLegacyAddStream#Conn.JetStream@dd7995: mechanical in migrate/legacytests.TestLegacyAddStream

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/legacytests.TestLegacyAddStream#JetStreamManager.AddStream@81af35: mechanical in migrate/legacytests.TestLegacyAddStream

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&nats.StreamConfig{Name: "LEGACY"})
```

After:

```go
_, err := js.CreateStream(context.Background(), jetstream.StreamConfig{Name: "LEGACY"})
```


## Component migrate/multifile.Streams#js

Handles: js (migrate/multifile/handle.go:21:2). Safe to apply in one commit.


### Step S30 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
// migrate/multifile/handle.go
import "github.com/nats-io/nats.go"
...
	js, err := nc.JetStream()
...
	info, err := js.AccountInfo()
```

After:

```go
// migrate/multifile/handle.go
import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)
...
	js, err := jetstream.New(nc)
...
	info, err := js.AccountInfo(context.Background())
```

#### Site migrate/multifile.Streams#Conn.JetStream@dd7995: mechanical in migrate/multifile.Streams

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/multifile.Streams#JetStreamManager.AccountInfo@3e7dd1: mechanical in migrate/multifile.Streams

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := js.AccountInfo(context.Background())
```


## Component migrate/neighbors.Legacy#js

Handles: js (migrate/neighbors/legacy.go:19:2). Safe to apply in one commit.


### Step S31 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
// migrate/neighbors/legacy.go
import "github.com/nats-io/nats.go"
...
	js, err := nc.JetStream()
...
	info, err := js.AccountInfo()
```

After:

```go
// migrate/neighbors/legacy.go
import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)
...
	js, err := jetstream.New(nc)
...
	info, err := js.AccountInfo(context.Background())
```

#### Site migrate/neighbors.Legacy#Conn.JetStream@dd7995: mechanical in migrate/neighbors.Legacy

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/neighbors.Legacy#JetStreamManager.AccountInfo@3e7dd1: mechanical in migrate/neighbors.Legacy

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := js.AccountInfo(context.Background())
```


## Component migrate/neighbors.Subscribe#js

Handles: js (migrate/neighbors/neighbors.go:26:16).


### Step S32 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/neighbors/neighbors.go
)
...
func Subscribe(js nats.JetStreamContext) error {
```

After:

```go
// migrate/neighbors/neighbors.go
	"github.com/nats-io/nats.go/jetstream"
)
...
func Subscribe(js nats.JetStreamContext, jsNew jetstream.JetStream) error {
```

#### Site migrate/neighbors.Subscribe#JetStreamContext@b9d014: mechanical in migrate/neighbors.Subscribe

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S33 (site)

Subscribe becomes a pull consumer's Consume.

#### Site migrate/neighbors.Subscribe#JetStream.Subscribe@c20042: decision in migrate/neighbors.Subscribe

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
_, errNew := js.Subscribe("orders.new", Handle, nats.Durable("new"), nats.ManualAck())
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle

#### Site migrate/neighbors.Subscribe#JetStream.Subscribe@5102a5: decision in migrate/neighbors.Subscribe

Subscribe becomes a pull consumer's Consume. See jetstream/MIGRATION.md#replacing-jssubscribe.

Before:

```go
_, errOld := js.Subscribe("orders.old", Handle, nats.Durable("old"), nats.ManualAck())
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it
- Decision **subscribe-target** pending, default **pull**: the jetstream package is built around pull consumers, which is why teams migrate.
  - pull: a pull consumer: Consume for callbacks, Messages for sync and channel forms
  - push: a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound
  - defer: keep the legacy subscription for now; the component keeps its legacy handle

### Step S34 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/neighbors.Subscribe#JetStream.Subscribe@c20042, migrate/neighbors.Subscribe#JetStream.Subscribe@5102a5.

## Component migrate/objects.Store#js

Handles: js (migrate/objects/objects.go:23:2), obj (migrate/objects/objects.go:27:2). Safe to apply in one commit.


### Step S35 (component, machine edits)

migrate js, obj to the jetstream package in one step: ObjectStore.PutBytes becomes ObjectStore.PutBytes; ObjectStore.GetBytes becomes ObjectStore.GetBytes.

Before:

```go
// migrate/objects/objects.go
)
...
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	obj, err := js.ObjectStore("files")
...
	if _, err := obj.PutBytes("a", data); err != nil {
...
	return obj.GetBytes("a")
```

After:

```go
// migrate/objects/objects.go
	"github.com/nats-io/nats.go/jetstream"
)
...
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}
	obj, err := js.ObjectStore(ctx, "files")
...
	if _, err := obj.PutBytes(ctx, "a", data); err != nil {
...
	return obj.GetBytes(ctx, "a")
```

#### Site migrate/objects.Store#Conn.JetStream@dd7995: mechanical in migrate/objects.Store

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
if err != nil {
	return nil, err
}
obj, err := js.ObjectStore(ctx, "files")
```


#### Site migrate/objects.Store#ObjectStoreManager.ObjectStore@1cdfd8: mechanical in migrate/objects.Store

ObjectStoreManager.ObjectStore becomes JetStream.ObjectStore. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
obj, err := js.ObjectStore("files")
```

After:

```go
js, err := jetstream.New(nc)
if err != nil {
	return nil, err
}
obj, err := js.ObjectStore(ctx, "files")
```


#### Site migrate/objects.Store#ObjectStore.PutBytes@31f3e3: mechanical in migrate/objects.Store

ObjectStore.PutBytes becomes ObjectStore.PutBytes. See jetstream/MIGRATION.md#object-store.

Before:

```go
_, err := obj.PutBytes("a", data)
```

After:

```go
_, err := obj.PutBytes(ctx, "a", data)
```


#### Site migrate/objects.Store#ObjectStore.GetBytes@7a81d5: mechanical in migrate/objects.Store

ObjectStore.GetBytes becomes ObjectStore.GetBytes. See jetstream/MIGRATION.md#object-store.

Before:

```go
return obj.GetBytes("a")
```

After:

```go
return obj.GetBytes(ctx, "a")
```


## Component migrate/order.Pull#js

Handles: js (migrate/order/order.go:26:11).


### Step S36 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/order/order.go
import "github.com/nats-io/nats.go"
...
func Pull(js nats.JetStreamContext) (*holder, error) {
```

After:

```go
// migrate/order/order.go
import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)
...
func Pull(js nats.JetStreamContext, jsNew jetstream.JetStream) (*holder, error) {
```

#### Site migrate/order.Pull#JetStreamContext@b9d014: mechanical in migrate/order.Pull

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S37 (site)

PullSubscribe becomes a jetstream.Consumer.

#### Site migrate/order.Pull#JetStream.PullSubscribe@7f715c: guided in migrate/order.Pull

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

### Step S38 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/order.Pull#JetStream.PullSubscribe@7f715c.

## Component migrate/scenarios.Traced#js

Handles: js (migrate/scenarios/corpus.go:26:2).


### Step S39 (add-handle)

create the jetstream siblings jsNew next to the legacy handles.

#### Site migrate/scenarios.Traced#Conn.JetStream@cf5652: guided in migrate/scenarios.Traced

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

### Step S40 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. Waits on: migrate/scenarios.Traced#Conn.JetStream@cf5652.

#### Site migrate/scenarios.Traced#JetStreamManager.DeleteStream@26bc3e: mechanical in migrate/scenarios.Traced

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("T")
```

After:

```go
return jsNew.DeleteStream(context.Background(), "T")
```


### Step S41 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios.Traced#Conn.JetStream@cf5652.

## Component migrate/scenarios.Retention#js

Handles: js (migrate/scenarios/corpus.go:35:37). Safe to apply in one commit.


### Step S42 (component, machine edits)

migrate js to the jetstream package in one step: nats.StreamConfig literal becomes jetstream.StreamConfig; nats.WorkQueuePolicy becomes jetstream.WorkQueuePolicy; JetStreamManager.AddStream becomes JetStream.CreateStream.

Before:

```go
// migrate/scenarios/corpus.go
)
...
func Retention(ctx context.Context, js nats.JetStreamContext) error {
	cfg := nats.StreamConfig{Name: "R"}
	cfg.Retention = nats.WorkQueuePolicy
	_, err := js.AddStream(&cfg)
```

After:

```go
// migrate/scenarios/corpus.go
	"github.com/nats-io/nats.go/jetstream"
)
...
func Retention(ctx context.Context, js jetstream.JetStream) error {
	cfg := jetstream.StreamConfig{Name: "R"}
	cfg.Retention = jetstream.WorkQueuePolicy
	_, err := js.CreateStream(ctx, cfg)
```

#### Site migrate/scenarios.Retention#JetStreamContext@b9d014: mechanical in migrate/scenarios.Retention

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Retention#StreamConfig@8b8feb: mechanical in migrate/scenarios.Retention

nats.StreamConfig literal becomes jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
cfg := nats.StreamConfig{Name: "R"}
```

After:

```go
cfg := jetstream.StreamConfig{Name: "R"}
```


#### Site migrate/scenarios.Retention#@861f0d: mechanical in migrate/scenarios.Retention

nats.WorkQueuePolicy becomes jetstream.WorkQueuePolicy.

Before:

```go
cfg.Retention = nats.WorkQueuePolicy
```

After:

```go
cfg.Retention = jetstream.WorkQueuePolicy
```


#### Site migrate/scenarios.Retention#JetStreamManager.AddStream@6269be: mechanical in migrate/scenarios.Retention

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&cfg)
```

After:

```go
_, err := js.CreateStream(ctx, cfg)
```


## Component migrate/scenarios.Infos#js

Handles: js (migrate/scenarios/corpus.go:43:33). Safe to apply in one commit.


### Step S43 (component, machine edits)

migrate js to the jetstream package in one step: retype nats.ConsumerInfo as jetstream.ConsumerInfo; nats.ConsumerInfo becomes jetstream.ConsumerInfo; JetStreamManager.ConsumerInfo becomes Consumer.Info.

Before:

```go
// migrate/scenarios/corpus.go
func Infos(ctx context.Context, js nats.JetStreamContext) (map[string]*nats.ConsumerInfo, error) {
	out := make(map[string]*nats.ConsumerInfo)
	info, err := js.ConsumerInfo("ORDERS", "w")
```

After:

```go
// migrate/scenarios/corpus.go
func Infos(ctx context.Context, js jetstream.JetStream) (map[string]*jetstream.ConsumerInfo, error) {
	out := make(map[string]*jetstream.ConsumerInfo)
	info, err := func() (*jetstream.ConsumerInfo, error) {
		consumer, err := js.Consumer(ctx, "ORDERS", "w")
		if err != nil {
			return nil, err
		}
		return consumer.Info(ctx)
	}()
```

#### Site migrate/scenarios.Infos#JetStreamContext@b9d014: mechanical in migrate/scenarios.Infos

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Infos#ConsumerInfo@d0895d: mechanical in migrate/scenarios.Infos

retype nats.ConsumerInfo as jetstream.ConsumerInfo. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
map[string]*nats.ConsumerInfo
```

After:

```go
map[string]*jetstream.ConsumerInfo
```


#### Site migrate/scenarios.Infos#ConsumerInfo@c0a96f: mechanical in migrate/scenarios.Infos

nats.ConsumerInfo becomes jetstream.ConsumerInfo. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
out := make(map[string]*nats.ConsumerInfo)
```

After:

```go
out := make(map[string]*jetstream.ConsumerInfo)
```


#### Site migrate/scenarios.Infos#JetStreamManager.ConsumerInfo@29fe56: mechanical in migrate/scenarios.Infos

JetStreamManager.ConsumerInfo becomes Consumer.Info. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
info, err := js.ConsumerInfo("ORDERS", "w")
```

After:

```go
info, err := func() (*jetstream.ConsumerInfo, error) {
	consumer, err := js.Consumer(ctx, "ORDERS", "w")
	if err != nil {
		return nil, err
	}
	return consumer.Info(ctx)
}()
```

- Note: the new call first obtains a consumer handle, which sends a CONSUMER.INFO request

## Component migrate/scenarios.watcher.Watch#kv

Handles: kv (migrate/scenarios/corpus.go:61:24). Safe to apply in one commit.


### Step S44 (component, machine edits)

migrate kv to the jetstream package in one step.

Before:

```go
// migrate/scenarios/corpus.go
func (watcher) Context() context.Context                 { return nil }
func (watcher) Updates() <-chan nats.KeyValueEntry       { return nil }
func (watcher) Stop() error                              { return nil }
func (watcher) Error() <-chan error                      { return nil }
func (w watcher) Watch(kv nats.KeyValue) nats.KeyWatcher { return w }
```

After:

```go
// migrate/scenarios/corpus.go
func (watcher) Context() context.Context                      { return nil }
func (watcher) Updates() <-chan nats.KeyValueEntry            { return nil }
func (watcher) Stop() error                                   { return nil }
func (watcher) Error() <-chan error                           { return nil }
func (w watcher) Watch(kv jetstream.KeyValue) nats.KeyWatcher { return w }
```

#### Site migrate/scenarios.watcher.Watch#KeyValue@fc1b43: mechanical in migrate/scenarios.watcher.Watch

declare kvNew jetstream.KeyValue next to kv. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv nats.KeyValue
```

After:

```go
kv jetstream.KeyValue
```


## Component migrate/scenarios.Asserted#js

Handles: js (migrate/scenarios/corpus.go:65:2).


### Step S45 (add-handle)

create the jetstream siblings  next to the legacy handles.

- js at migrate/scenarios/corpus.go:65:2 cannot get a sibling: assigned from a multi-value call the planner does not rewrite
### Step S46 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. Waits on: S45.

#### Site migrate/scenarios.Asserted#JetStreamManager.DeleteStream@282f4c: guided in migrate/scenarios.Asserted

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

### Step S47 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: S45, migrate/scenarios.Asserted#JetStreamManager.DeleteStream@282f4c.

## Component migrate/scenarios.useHandle#js

Handles: js (migrate/scenarios/corpus.go:72:16).


### Step S48 (add-handle)

create the jetstream siblings  next to the legacy handles.

- js at migrate/scenarios/corpus.go:72:16 cannot get a sibling: a caller passes it a value that is not a handle variable (fakeJS{})
#### Site migrate/scenarios.useHandle#JetStreamContext@b9d014: guided in migrate/scenarios.useHandle

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

### Step S49 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. Waits on: S48.

#### Site migrate/scenarios.useHandle#JetStreamManager.DeleteStream@580322: guided in migrate/scenarios.useHandle

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

### Step S50 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: S48, migrate/scenarios.useHandle#JetStreamContext@b9d014, migrate/scenarios.useHandle#JetStreamManager.DeleteStream@580322.

## Component migrate/scenarios.framework.KV#js

Handles: js (migrate/scenarios/corpus.go:85:2).


### Step S51 (add-handle)

create the jetstream siblings jsNew next to the legacy handles.

#### Site migrate/scenarios.framework.KV#Conn.JetStream@280266: mechanical in migrate/scenarios.framework.KV

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


#### Site migrate/scenarios.framework.KV#KeyValueManager.KeyValue@6b007d: guided in migrate/scenarios.framework.KV

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

### Step S52 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios.framework.KV#KeyValueManager.KeyValue@6b007d.

## Component migrate/scenarios.Helper#kv

Handles: kv (migrate/scenarios/corpus.go:94:2).


### Step S53 (add-handle)

create the jetstream siblings  next to the legacy handles.

- kv at migrate/scenarios/corpus.go:94:2 cannot get a sibling: assigned from a multi-value call the planner does not rewrite
### Step S54 (site)

KeyValue.Get becomes KeyValue.Get. Waits on: S53.

#### Site migrate/scenarios.Helper#KeyValue.Get@7a5fa0: guided in migrate/scenarios.Helper

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

### Step S55 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: kvNew → kv. Waits on: S53, migrate/scenarios.Helper#KeyValue.Get@7a5fa0.

## Component migrate/scenarios.Setup#js

Handles: js (migrate/scenarios/scenarios.go:25:33). Safe to apply in one commit.


### Step S56 (component, machine edits)

migrate js to the jetstream package in one step: retype nats.StreamConfig as jetstream.StreamConfig; JetStreamManager.AddStream becomes JetStream.CreateStream.

Before:

```go
// migrate/scenarios/scenarios.go
)
...
func Setup(ctx context.Context, js nats.JetStreamContext, cfg nats.StreamConfig) error {
	_, err := js.AddStream(&cfg)
```

After:

```go
// migrate/scenarios/scenarios.go
	"github.com/nats-io/nats.go/jetstream"
)
...
func Setup(ctx context.Context, js jetstream.JetStream, cfg jetstream.StreamConfig) error {
	_, err := js.CreateStream(ctx, cfg)
```

#### Site migrate/scenarios.Setup#JetStreamContext@b9d014: mechanical in migrate/scenarios.Setup

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Setup#StreamConfig@240044: mechanical in migrate/scenarios.Setup

retype nats.StreamConfig as jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
cfg nats.StreamConfig
```

After:

```go
cfg jetstream.StreamConfig
```


#### Site migrate/scenarios.Setup#JetStreamManager.AddStream@6269be: mechanical in migrate/scenarios.Setup

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&cfg)
```

After:

```go
_, err := js.CreateStream(ctx, cfg)
```


## Component migrate/scenarios.request.Create#js

Handles: js (migrate/scenarios/scenarios.go:33:25). Safe to apply in one commit.


### Step S57 (component, machine edits)

migrate js to the jetstream package in one step: retype nats.StreamConfig as jetstream.StreamConfig; JetStreamManager.AddStream becomes JetStream.CreateStream.

Before:

```go
// migrate/scenarios/scenarios.go
func (r request) Create(js nats.JetStreamContext, cfg nats.StreamConfig) error {
	_, err := js.AddStream(&cfg, nats.Context(r.ctx))
```

After:

```go
// migrate/scenarios/scenarios.go
func (r request) Create(js jetstream.JetStream, cfg jetstream.StreamConfig) error {
	_, err := js.CreateStream(r.ctx, cfg)
```

#### Site migrate/scenarios.request.Create#JetStreamContext@b9d014: mechanical in migrate/scenarios.request.Create

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.request.Create#StreamConfig@240044: mechanical in migrate/scenarios.request.Create

retype nats.StreamConfig as jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
cfg nats.StreamConfig
```

After:

```go
cfg jetstream.StreamConfig
```


#### Site migrate/scenarios.request.Create#JetStreamManager.AddStream@0e8ebd: mechanical in migrate/scenarios.request.Create

JetStreamManager.AddStream becomes JetStream.CreateStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
_, err := js.AddStream(&cfg, nats.Context(r.ctx))
```

After:

```go
_, err := js.CreateStream(r.ctx, cfg)
```


## Component migrate/scenarios.Drop#js

Handles: js (migrate/scenarios/scenarios.go:39:11). Safe to apply in one commit.


### Step S58 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.DeleteStream becomes JetStream.DeleteStream.

Before:

```go
// migrate/scenarios/scenarios.go
func Drop(js nats.JetStreamContext) error {
	return js.DeleteStream("S")
```

After:

```go
// migrate/scenarios/scenarios.go
func Drop(js jetstream.JetStream) error {
	return js.DeleteStream(context.Background(), "S")
```

#### Site migrate/scenarios.Drop#JetStreamContext@b9d014: mechanical in migrate/scenarios.Drop

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Drop#JetStreamManager.DeleteStream@6b0fc9: mechanical in migrate/scenarios.Drop

JetStreamManager.DeleteStream becomes JetStream.DeleteStream. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.DeleteStream("S")
```

After:

```go
return js.DeleteStream(context.Background(), "S")
```


## Component migrate/scenarios.Timed#js

Handles: js (migrate/scenarios/scenarios.go:44:12).


### Step S59 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/scenarios/scenarios.go
func Timed(js nats.JetStreamContext) error {
```

After:

```go
// migrate/scenarios/scenarios.go
func Timed(js nats.JetStreamContext, jsNew jetstream.JetStream) error {
```

#### Site migrate/scenarios.Timed#JetStreamContext@b9d014: mechanical in migrate/scenarios.Timed

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S60 (site)

JetStreamManager.DeleteStream becomes JetStream.DeleteStream.

#### Site migrate/scenarios.Timed#JetStreamManager.DeleteStream@4379c2: guided in migrate/scenarios.Timed

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

### Step S61 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios.Timed#JetStreamManager.DeleteStream@4379c2.

## Component migrate/scenarios.Consumer#js

Handles: js (migrate/scenarios/scenarios.go:50:36). Safe to apply in one commit.


### Step S62 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.AddConsumer becomes JetStream.CreatePushConsumer.

Before:

```go
// migrate/scenarios/scenarios.go
func Consumer(ctx context.Context, js nats.JetStreamContext) error {
	_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", Heartbeat: time.Second})
```

After:

```go
// migrate/scenarios/scenarios.go
func Consumer(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreatePushConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", IdleHeartbeat: time.Second, AckPolicy: jetstream.AckNonePolicy})
```

#### Site migrate/scenarios.Consumer#JetStreamContext@b9d014: mechanical in migrate/scenarios.Consumer

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Consumer#JetStreamManager.AddConsumer@1b7945: mechanical in migrate/scenarios.Consumer

JetStreamManager.AddConsumer becomes JetStream.CreatePushConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", Heartbeat: time.Second})
```

After:

```go
_, err := js.CreatePushConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", IdleHeartbeat: time.Second, AckPolicy: jetstream.AckNonePolicy})
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

## Component migrate/scenarios.Name#js

Handles: js (migrate/scenarios/scenarios.go:56:11). Safe to apply in one commit.


### Step S63 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.StreamNameBySubject becomes JetStream.StreamNameBySubject.

Before:

```go
// migrate/scenarios/scenarios.go
func Name(js nats.JetStreamContext) (string, error) {
	return js.StreamNameBySubject("orders.new")
```

After:

```go
// migrate/scenarios/scenarios.go
func Name(js jetstream.JetStream) (string, error) {
	return js.StreamNameBySubject(context.Background(), "orders.new")
```

#### Site migrate/scenarios.Name#JetStreamContext@b9d014: mechanical in migrate/scenarios.Name

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Name#JetStreamManager.StreamNameBySubject@543075: mechanical in migrate/scenarios.Name

JetStreamManager.StreamNameBySubject becomes JetStream.StreamNameBySubject. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return js.StreamNameBySubject("orders.new")
```

After:

```go
return js.StreamNameBySubject(context.Background(), "orders.new")
```


## Component migrate/scenarios.Bound#js

Handles: js (migrate/scenarios/scenarios.go:61:12). Safe to apply in one commit.


### Step S64 (component, machine edits)

migrate js to the jetstream package in one step: PullSubscribe becomes a jetstream.Consumer.

Before:

```go
// migrate/scenarios/scenarios.go
func Bound(js nats.JetStreamContext) error {
	sub, err := js.PullSubscribe("orders.new", "", nats.Durable("w"), nats.BindStream("ORDERS"), nats.DeliverNew(), nats.MaxAckPending(100))
...
	defer sub.Drain()
```

After:

```go
// migrate/scenarios/scenarios.go
func Bound(js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateConsumer(context.Background(), "ORDERS", jetstream.ConsumerConfig{
		Durable:       "w",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		MaxAckPending: 100,
		FilterSubject: "orders.new",
	})
...
```

#### Site migrate/scenarios.Bound#JetStreamContext@b9d014: mechanical in migrate/scenarios.Bound

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Bound#JetStream.PullSubscribe@aed306: mechanical in migrate/scenarios.Bound

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := js.PullSubscribe("orders.new", "", nats.Durable("w"), nats.BindStream("ORDERS"), nats.DeliverNew(), nats.MaxAckPending(100))
```

After:

```go
_, err := js.CreateOrUpdateConsumer(context.Background(), "ORDERS", jetstream.ConsumerConfig{
	Durable:       "w",
	DeliverPolicy: jetstream.DeliverNewPolicy,
	MaxAckPending: 100,
	FilterSubject: "orders.new",
})
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it

## Component migrate/scenarios.Pull#js

Handles: js (migrate/scenarios/scenarios.go:71:11). Safe to apply in one commit.


### Step S65 (component, machine edits)

migrate js to the jetstream package in one step: PullSubscribe becomes a jetstream.Consumer.

Before:

```go
// migrate/scenarios/scenarios.go
func Pull(js nats.JetStreamContext) error {
	sub, err := js.PullSubscribe("orders.new", "w")
...
	defer sub.Unsubscribe()
```

After:

```go
// migrate/scenarios/scenarios.go
func Pull(js jetstream.JetStream) error {
	_, err := func() (jetstream.Consumer, error) {
		stream, err := js.StreamNameBySubject(context.Background(), "orders.new")
		if err != nil {
			return nil, err
		}
		return js.CreateOrUpdateConsumer(context.Background(), stream, jetstream.ConsumerConfig{
			Durable:       "w",
			FilterSubject: "orders.new",
		})
	}()
...
```

#### Site migrate/scenarios.Pull#JetStreamContext@b9d014: mechanical in migrate/scenarios.Pull

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Pull#JetStream.PullSubscribe@c77dab: mechanical in migrate/scenarios.Pull

PullSubscribe becomes a jetstream.Consumer. See jetstream/MIGRATION.md#replacing-jspullsubscribe.

Before:

```go
sub, err := js.PullSubscribe("orders.new", "w")
```

After:

```go
_, err := func() (jetstream.Consumer, error) {
	stream, err := js.StreamNameBySubject(context.Background(), "orders.new")
	if err != nil {
		return nil, err
	}
	return js.CreateOrUpdateConsumer(context.Background(), stream, jetstream.ConsumerConfig{
		Durable:       "w",
		FilterSubject: "orders.new",
	})
}()
```

- Note: the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration
- Note: legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it

## Component migrate/scenarios.Billing#js

Handles: js (migrate/scenarios/scenarios.go:81:35).


### Step S66 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/scenarios/scenarios.go
func Billing(ctx context.Context, js nats.JetStreamContext) error {
```

After:

```go
// migrate/scenarios/scenarios.go
func Billing(ctx context.Context, js nats.JetStreamContext, jsNew jetstream.JetStream) error {
```

#### Site migrate/scenarios.Billing#JetStreamContext@b9d014: mechanical in migrate/scenarios.Billing

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S67 (site)

Subscribe becomes a pull consumer's Consume.

#### Site migrate/scenarios.Billing#JetStream.Subscribe@a7bfbf: decision in migrate/scenarios.Billing

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

### Step S68 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios.Billing#JetStream.Subscribe@a7bfbf.

## Component migrate/scenarios.Buckets#js

Handles: js (migrate/scenarios/scenarios.go:88:2), kv (migrate/scenarios/scenarios.go:92:2). Safe to apply in one commit.


### Step S69 (component, machine edits)

migrate js, kv to the jetstream package in one step: KeyValue.Get becomes KeyValue.Get.

Before:

```go
// migrate/scenarios/scenarios.go
	js, err := nc.JetStream()
...
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
...
	e, err := kv.Get("a")
```

After:

```go
// migrate/scenarios/scenarios.go
	js, err := jetstream.New(nc)
...
	kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{Bucket: "settings"})
...
	e, err := kv.Get(context.Background(), "a")
```

#### Site migrate/scenarios.Buckets#Conn.JetStream@dd7995: mechanical in migrate/scenarios.Buckets

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/scenarios.Buckets#KeyValueManager.CreateKeyValue@f33482: mechanical in migrate/scenarios.Buckets

KeyValueManager.CreateKeyValue becomes JetStream.CreateKeyValue. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
```

After:

```go
kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{Bucket: "settings"})
```


#### Site migrate/scenarios.Buckets#KeyValue.Get@7a5fa0: mechanical in migrate/scenarios.Buckets

KeyValue.Get becomes KeyValue.Get. See jetstream/MIGRATION.md#keyvalue-store.

Before:

```go
e, err := kv.Get("a")
```

After:

```go
e, err := kv.Get(context.Background(), "a")
```


## Component migrate/scenarios.Config#js

Handles: js (migrate/scenarios/scenarios.go:104:34). Safe to apply in one commit.


### Step S70 (component, machine edits)

migrate js to the jetstream package in one step: nats.ConsumerConfig literal becomes jetstream.ConsumerConfig; nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy; JetStreamManager.AddConsumer becomes JetStream.CreateConsumer.

Before:

```go
// migrate/scenarios/scenarios.go
func Config(ctx context.Context, js nats.JetStreamContext) error {
	cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
...
	_, err := js.AddConsumer("ORDERS", &cfg)
```

After:

```go
// migrate/scenarios/scenarios.go
func Config(ctx context.Context, js jetstream.JetStream) error {
	cfg := jetstream.ConsumerConfig{Durable: "c", AckPolicy: jetstream.AckExplicitPolicy}
...
	_, err := js.CreateConsumer(ctx, "ORDERS", cfg)
```

#### Site migrate/scenarios.Config#JetStreamContext@b9d014: mechanical in migrate/scenarios.Config

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Config#ConsumerConfig@469c18: mechanical in migrate/scenarios.Config

nats.ConsumerConfig literal becomes jetstream.ConsumerConfig. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
```

After:

```go
cfg := jetstream.ConsumerConfig{Durable: "c", AckPolicy: jetstream.AckExplicitPolicy}
```


#### Site migrate/scenarios.Config#@39b2be: mechanical in migrate/scenarios.Config

nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy.

Before:

```go
cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
```

After:

```go
cfg := jetstream.ConsumerConfig{Durable: "c", AckPolicy: jetstream.AckExplicitPolicy}
```


#### Site migrate/scenarios.Config#JetStreamManager.AddConsumer@261f10: mechanical in migrate/scenarios.Config

JetStreamManager.AddConsumer becomes JetStream.CreateConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := js.AddConsumer("ORDERS", &cfg)
```

After:

```go
_, err := js.CreateConsumer(ctx, "ORDERS", cfg)
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

## Component migrate/scenarios.Policy#js

Handles: js (migrate/scenarios/scenarios.go:112:34). Safe to apply in one commit.


### Step S71 (component, machine edits)

migrate js to the jetstream package in one step: nats.AckNonePolicy becomes jetstream.AckNonePolicy; nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy; JetStreamManager.AddConsumer becomes JetStream.CreateConsumer.

Before:

```go
// migrate/scenarios/scenarios.go
func Policy(ctx context.Context, js nats.JetStreamContext, explicit bool) error {
	ack := nats.AckNonePolicy
...
		ack = nats.AckExplicitPolicy
...
	_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "p", AckPolicy: ack})
```

After:

```go
// migrate/scenarios/scenarios.go
func Policy(ctx context.Context, js jetstream.JetStream, explicit bool) error {
	ack := jetstream.AckNonePolicy
...
		ack = jetstream.AckExplicitPolicy
...
	_, err := js.CreateConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "p", AckPolicy: ack})
```

#### Site migrate/scenarios.Policy#JetStreamContext@b9d014: mechanical in migrate/scenarios.Policy

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js jetstream.JetStream
```


#### Site migrate/scenarios.Policy#@e233e5: mechanical in migrate/scenarios.Policy

nats.AckNonePolicy becomes jetstream.AckNonePolicy.

Before:

```go
ack := nats.AckNonePolicy
```

After:

```go
ack := jetstream.AckNonePolicy
```


#### Site migrate/scenarios.Policy#@39b2be: mechanical in migrate/scenarios.Policy

nats.AckExplicitPolicy becomes jetstream.AckExplicitPolicy.

Before:

```go
ack = nats.AckExplicitPolicy
```

After:

```go
ack = jetstream.AckExplicitPolicy
```


#### Site migrate/scenarios.Policy#JetStreamManager.AddConsumer@c3609f: mechanical in migrate/scenarios.Policy

JetStreamManager.AddConsumer becomes JetStream.CreateConsumer. See jetstream/MIGRATION.md#consumer-management.

Before:

```go
_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "p", AckPolicy: ack})
```

After:

```go
_, err := js.CreateConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "p", AckPolicy: ack})
```

- Note: legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise

## Component migrate/scenarios.All#js

Handles: js (migrate/scenarios/scenarios.go:122:31).


### Step S72 (add-handle, machine edits)

create the jetstream siblings jsNew next to the legacy handles.

Before:

```go
// migrate/scenarios/scenarios.go
func All(ctx context.Context, js nats.JetStreamContext) error {
```

After:

```go
// migrate/scenarios/scenarios.go
func All(ctx context.Context, js nats.JetStreamContext, jsNew jetstream.JetStream) error {
```

#### Site migrate/scenarios.All#JetStreamContext@b9d014: mechanical in migrate/scenarios.All

declare jsNew jetstream.JetStream next to js. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js nats.JetStreamContext
```

After:

```go
js nats.JetStreamContext, jsNew jetstream.JetStream
```


### Step S73 (site)

Subscribe becomes a pull consumer's Consume.

#### Site migrate/scenarios.All#JetStream.Subscribe@62b31a: decision in migrate/scenarios.All

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

### Step S74 (finish)

remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: jsNew → js. Waits on: migrate/scenarios.All#JetStream.Subscribe@62b31a.

## Component migrate/unrelated.Legacy#js

Handles: js (migrate/unrelated/unrelated.go:38:2). Safe to apply in one commit.


### Step S75 (component, machine edits)

migrate js to the jetstream package in one step: JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
// migrate/unrelated/unrelated.go
	js, err := nc.JetStream()
...
	info, err := js.AccountInfo()
```

After:

```go
// migrate/unrelated/unrelated.go
	js, err := jetstream.New(nc)
...
	info, err := js.AccountInfo(context.Background())
```

#### Site migrate/unrelated.Legacy#Conn.JetStream@dd7995: mechanical in migrate/unrelated.Legacy

nc.JetStream becomes jetstream.New. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
js, err := nc.JetStream()
```

After:

```go
js, err := jetstream.New(nc)
```


#### Site migrate/unrelated.Legacy#JetStreamManager.AccountInfo@3e7dd1: mechanical in migrate/unrelated.Legacy

JetStreamManager.AccountInfo becomes JetStream.AccountInfo.

Before:

```go
info, err := js.AccountInfo()
```

After:

```go
info, err := js.AccountInfo(context.Background())
```


## Sites outside components

### Step S76 (site)

declare a jetstream.JetStream sibling next to the nats.JetStreamContext handle.

#### Site migrate/legacyonly.Handle#JetStreamContext@346850: guided in migrate/legacyonly.Handle

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

### Step S77 (site)

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
#### Site migrate/order.config#StreamConfig@ac861b: mechanical in migrate/order.config

retype nats.StreamConfig as jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
nats.StreamConfig
```

After:

```go
jetstream.StreamConfig
```


#### Site migrate/order.config#StreamConfig@717110: mechanical in migrate/order.config

nats.StreamConfig literal becomes jetstream.StreamConfig. See jetstream/MIGRATION.md#stream-management.

Before:

```go
return nats.StreamConfig{}
```

After:

```go
return jetstream.StreamConfig{}
```


#### Site migrate/order.Defaults#RetentionPolicy@74bfaa: mechanical in migrate/order.Defaults

retype nats.RetentionPolicy as jetstream.RetentionPolicy.

Before:

```go
retention nats.RetentionPolicy
```

After:

```go
retention jetstream.RetentionPolicy
```


#### Site migrate/order.Defaults#StorageType@a31f77: mechanical in migrate/order.Defaults

retype nats.StorageType as jetstream.StorageType.

Before:

```go
storage   nats.StorageType
```

After:

```go
storage   jetstream.StorageType
```


#### Site migrate/order.Defaults#DiscardPolicy@c6210f: mechanical in migrate/order.Defaults

retype nats.DiscardPolicy as jetstream.DiscardPolicy.

Before:

```go
discard   nats.DiscardPolicy
```

After:

```go
discard   jetstream.DiscardPolicy
```


#### Site migrate/order.Defaults#@b95341: mechanical in migrate/order.Defaults

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


#### Site migrate/order.Defaults#@7e27c0: mechanical in migrate/order.Defaults

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


#### Site migrate/order.Defaults#@23fcba: mechanical in migrate/order.Defaults

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


### Step S78 (site)

retype nats.KeyValueEntry as jetstream.KeyValueEntry.

#### Site migrate/scenarios.watcher.Updates#KeyValueEntry@97eedd: guided in migrate/scenarios.watcher.Updates

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

### Step S79 (site)

retype nats.KeyWatcher as jetstream.KeyWatcher.

#### Site migrate/scenarios.watcher.Watch#KeyWatcher@45c668: guided in migrate/scenarios.watcher.Watch

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

### Step S80 (site)

nats.JetStreamContext becomes jetstream.JetStream.

#### Site migrate/scenarios.Asserted#JetStreamContext@346850: guided in migrate/scenarios.Asserted

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

### Step S81 (site)

declare a jetstream.JetStream sibling next to the nats.JetStreamContext handle.

#### Site migrate/scenarios#JetStreamContext@346850: guided

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

### Step S82 (site)

declare a jetstream.KeyValue sibling next to the nats.KeyValue handle.

#### Site migrate/scenarios.framework.KV#KeyValue@acad29: guided in migrate/scenarios.framework.KV

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

## Unmapped sites

No jetstream counterpart; the code stays on the legacy API until the user decides otherwise.

#### Site migrate/legacyonly.Handle#Conn.JetStream@293c78: unmapped in migrate/legacyonly.Handle

Conn.JetStream creates a handle the planner cannot thread. See jetstream/MIGRATION.md#initialization-options.

Before:

```go
return nc.JetStream(nats.UseLegacyDurableConsumers())
```

- Fact: the handle is not assigned to a variable next to its error; assign it (js, err := ...) so a sibling can be created
- Note: UseLegacyDurableConsumers: the jetstream package always uses the consumer create API; there is no legacy mode

## Follow-ups

Not needed to finish the migration:

- migrate/app.Service.StreamNames#JetStreamManager.StreamNames@4bf32c (lister-error): check the error of the StreamNames lister after ranging over Name()
- migrate/app.Service.Run#JetStream.Subscribe@138e9e (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app.Service.Tail#JetStream.Subscribe@032ec2 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app.Service.Limited#JetStream.Subscribe@ce9a15 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app.Service.Batch#JetStream.PullSubscribe@80357c (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app.Service.Next#JetStream.SubscribeSync@c4096c (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app.Service.Channel#JetStream.ChanSubscribe@b4a2a8 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/app.Service.Shared#JetStream.Subscribe@1b480a (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/neighbors.Subscribe#JetStream.Subscribe@c20042 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/neighbors.Subscribe#JetStream.Subscribe@5102a5 (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/order.Pull#JetStream.PullSubscribe@7f715c (stream-name): replace the runtime StreamNameBySubject lookup with the stream's name
- migrate/scenarios.Pull#JetStream.PullSubscribe@c77dab (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
- migrate/scenarios.Billing#JetStream.Subscribe@a7bfbf (stream-name): replace the runtime StreamNameBySubject lookup with the stream's name
- migrate/scenarios.All#JetStream.Subscribe@62b31a (stream-name): the stream is likely "ORDERS", created at migrate/app/app.go:44; name it instead of looking it up
