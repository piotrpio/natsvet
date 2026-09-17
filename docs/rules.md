# natsvet rules

Generated from the analyzers' documentation by `go generate`; do not edit.
Default-on rules can be turned off with `-<rule>=false`; opt-in rules are
enabled with `-<rule>.enable`.

| Rule | Default | Fix | Summary |
|------|---------|-----|---------|
| [`consumerconfig`](#consumerconfig) | on | no | report consumer configurations the server rejects |
| [`ctxdeadline`](#ctxdeadline) | on | no | report context.Background() or context.TODO() passed where a deadline is needed |
| [`drain`](#drain) | on | no | report a Drain that Close or process exit cuts short |
| [`duration`](#duration) | on | no | report untyped constants passed where nats.go expects a time.Duration |
| [`handle`](#handle) | on | no | report a discarded ConsumeContext, MessagesContext, watcher or micro.Service |
| [`headerkey`](#headerkey) | on | yes | report header keys that differ only in case from a NATS header |
| [`kvconfig`](#kvconfig) | on | no | report KV bucket names, history limits and keys nats.go rejects |
| [`msgloop`](#msgloop) | on | no | report a Next loop that never exits and a Fetch batch ranged without Error |
| [`nilheader`](#nilheader) | on | no | report header writes on a nats.Msg literal that has no Header |
| [`pubasync`](#pubasync) | on | no | report a discarded PubAckFuture with no async error path in the package |
| [`streamconfig`](#streamconfig) | on | no | report stream configurations the server rejects |
| [`subject`](#subject) | on | no | report invalid subjects, wildcard publishes and bad queue names |
| [`syncsub`](#syncsub) | on | no | report NextMsg on a subscription that is not synchronous |
| [`legacyjs`](#legacyjs) | opt-in (`-legacyjs.enable`) | no | report uses of the legacy JetStream API (opt-in) |

## consumerconfig

report consumer configurations the server rejects

Default: on. Fix: no.

Every check mirrors one in nats-server's checkConsumerCfg and fires only
when the involved fields are constants in the same composite literal, so a
report is a guaranteed runtime error, reported at the literal instead of as
a JetStreamError from CreateConsumer.

```go
jetstream.ConsumerConfig{
	FilterSubject:  "orders.new",
	FilterSubjects: []string{"orders.paid"}, // both set: rejected
}
```

## ctxdeadline

report context.Background() or context.TODO() passed where a deadline is needed

Default: on. Fix: no.

FlushWithContext returns ErrNoDeadlineContext for such a context every
time; RequestWithContext, RequestMsgWithContext and NextMsgWithContext
accept it and then block forever whenever a responder exists but never
replies; legacy Fetch and FetchBatch return ErrNoDeadlineContext for
nats.Context(context.Background()). Only a direct Background()/TODO()
argument is reported.

```go
nc.RequestWithContext(context.Background(), "s", data)    // may block forever
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
```

## drain

report a Drain that Close or process exit cuts short

Default: on. Fix: no.

Drain returns as soon as it has started draining in a goroutine; the
ClosedHandler reports when it finishes. A Close in the next statement, or a
deferred Close that runs when the function returns from a trailing Drain,
closes the connection and discards the drain in progress. In main, a
deferred Drain or a Drain followed by process exit (os.Exit, log.Fatal, the
end of main) drains nothing at all: the process is gone before the goroutine
has done anything. Tests are exempt from the deferred forms, where the
function's return does not end the process.

```go
nc.Drain()
nc.Close()          // aborts the drain

defer nc.Close()
return nc.Drain()   // same

func main() {
	defer nc.Drain() // drains nothing; wait for the ClosedHandler
}
```

## duration

report untyped constants passed where nats.go expects a time.Duration

Default: on. Fix: no.

Go converts an untyped integer constant to time.Duration silently, so
nc.Request("s", nil, 5) waits five nanoseconds and AckWait: 30 acknowledges
in thirty nanoseconds; the intended unit was almost certainly seconds or
milliseconds. Arguments, struct literal fields, field assignments and
conversions to nats.go's Duration-based option types are covered, for every
API in the nats, jetstream and micro packages.

```go
nc.Request("s", nil, 5)            // 5ns
nc.Request("s", nil, 5*time.Second)
```

## handle

report a discarded ConsumeContext, MessagesContext, watcher or micro.Service

Default: on. Fix: no.

The handle returned by Consume, Messages, Watch or AddService is the only way
to Stop or Drain what the call started; assigned to the blank identifier or
dropped as an expression statement, the consumer, watcher or service runs
until the connection closes and in-flight work cannot be finished cleanly. A
Consume or Messages call with a jetstream.StopAfter option stops itself and
is not reported.

```go
_, err := cons.Consume(handler)   // nothing can ever stop this consumer
cc, err := cons.Consume(handler)
defer cc.Drain()
```

## headerkey

report header keys that differ only in case from a NATS header

Default: on. Fix: yes.

nats.go headers are case-preserving and lookups are exact map lookups, unlike
net/http which canonicalizes keys, so msg.Header.Get("nats-msg-id") returns ""
on a message that carries Nats-Msg-Id, and Set("nats-msg-id", v) publishes a
header the server does not recognize.

```go
msg.Header.Get("nats-msg-id")      // always ""
msg.Header.Get(jetstream.MsgIDHeader)
```

The fix replaces the key with the constant from the nats.go package the file
already imports, or with the correctly cased literal.

## kvconfig

report KV bucket names, history limits and keys nats.go rejects

Default: on. Fix: no.

The checks are the ones in nats.go's jetstream/kv.go and jetstream/object.go
(bucketValid, keyValid, searchKeyValid, KeyValueMaxHistory) and fire only on
constants, so a report is a guaranteed ErrInvalidBucketName,
ErrInvalidStoreName, ErrHistoryTooLarge or ErrInvalidKey at runtime.

```go
js.KeyValue(ctx, "my.bucket")   // ErrInvalidBucketName
kv.Put(ctx, "user name", data)  // ErrInvalidKey
kv.Watch(ctx, "users.*")        // fine: filters may use wildcards
```

## msgloop

report a Next loop that never exits and a Fetch batch ranged without Error

Default: on. Fix: no.

A for loop that calls MessagesContext.Next and continues on every error never
exits once the iterator is stopped or drained, because Next then returns
ErrMsgIteratorClosed on every call without blocking. A Fetch result whose
Messages channel is ranged over without checking Error afterwards drops the
batch's terminal error, so a missed heartbeat or a deleted consumer looks
like an empty batch.

```go
for {
	msg, err := it.Next()
	if err != nil {
		log.Println(err)
		continue           // forever, once it.Stop() has run
	}
	msg.Ack()
}

msgs, _ := cons.Fetch(10)
for msg := range msgs.Messages() {
	msg.Ack()
}
if err := msgs.Error(); err != nil { // the part that is missing
	return err
}
```

## nilheader

report header writes on a nats.Msg literal that has no Header

Default: on. Fix: no.

nats.Header.Set, Add and a direct Header[key] = assignment are plain map
writes; nats.NewMsg allocates the map and a composite literal does not, so
the write panics with an assignment to a nil map. The rule follows the
message variable back to its single definition in the same function.

```go
m := &nats.Msg{Subject: "s"}
m.Header.Set("X-Id", "1")   // panic: assignment to entry in nil map
m := nats.NewMsg("s")       // allocates Header
```

## pubasync

report a discarded PubAckFuture with no async error path in the package

Default: on. Fix: no.

The future's Err channel and the handler installed by
WithPublishAsyncErrHandler are the only two places a rejected or timed-out
async publish is ever reported; when the future is thrown away and the
package neither installs the handler nor waits on PublishAsyncComplete, the
publish fails silently and the caller believes the message was stored. An
ack handler (WithPublishAsyncAckHandler) runs only for successful publishes
and does not replace the error handler.

```go
_, err := js.PublishAsync("orders.new", data)   // a NACK is never seen
f, err := js.PublishAsync("orders.new", data)
select {
case <-f.Ok():
case err := <-f.Err():
}
```

## streamconfig

report stream configurations the server rejects

Default: on. Fix: no.

Every check mirrors one in nats-server's checkStreamCfgLocked and fires only
when the involved fields are constants in the same composite literal, so a
report is a guaranteed runtime error, reported at the literal instead of as
a JetStreamError from CreateStream.

```go
jetstream.StreamConfig{
	Name:     "ORDERS",
	Subjects: []string{"orders.>", "orders.new"}, // overlap: rejected
}
```

## subject

report invalid subjects, wildcard publishes and bad queue names

Default: on. Fix: no.

nats.go returns ErrBadSubject for an empty subject or one containing
whitespace, the server rejects a subscription with an empty token or a
misplaced '>', and a wildcard token in a publish subject is sent literally,
so the message reaches only subscriptions that spell out that literal
token. A queue group containing whitespace returns ErrBadQueueName. Only
constant subjects and queue names are examined.

```go
nc.Publish("orders.*", data)      // reaches nobody who subscribed to orders.*
nc.Subscribe("foo..bar", handler) // ErrBadSubject
```

## syncsub

report NextMsg on a subscription that is not synchronous

Default: on. Fix: no.

nats.go's validateNextMsgState returns ErrSyncSubRequired when the
subscription has a callback and ErrTypeSubscription when it is a legacy pull
subscription; for a channel subscription NextMsg silently reads from the
caller's own channel and competes with it. The rule follows the receiver
back to the single subscribe call that defined it in the same function.

```go
sub, _ := nc.Subscribe("s", handler)
msg, err := sub.NextMsg(time.Second)   // always ErrSyncSubRequired
```

## legacyjs

report uses of the legacy JetStream API (opt-in)

Default: opt-in (`-legacyjs.enable`). Fix: no.

Inventories every use of nats.JetStreamContext, nats.KeyValue,
nats.ObjectStore and their options, configs and message methods, so a
codebase can see what a migration to the jetstream package has to touch.
nats.go does not mark this API deprecated, so no generic deprecation check
sees it. The rule reports facts and never fixes; enable it with
-legacyjs.enable.

```go
js, _ := nc.JetStream()        // legacy JetStream API: nats.Conn.JetStream
js.Publish("orders", data)      // legacy JetStream API: nats.JetStream.Publish
```
