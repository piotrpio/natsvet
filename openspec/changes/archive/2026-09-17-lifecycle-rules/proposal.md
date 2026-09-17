## Why

Tier 1 is complete and `main` is release-ready. The next cluster of support cases is not a bad value but a lost handle: a `ConsumeContext` assigned to `_` that nobody can ever `Stop()`, a `KeyWatcher` whose subscription lives as long as the connection, a `PubAckFuture` thrown away so a rejected publish is never seen, a `for { msg, err := it.Next(); if err != nil { continue } }` that never exits once the iterator is stopped, a `Fetch` batch ranged over without `Error()` so a lost heartbeat or a deleted consumer looks like an empty batch, and a `nc.Drain()` in `main` right before the process exits, which drains nothing. Each is decidable from the statement that discards the value plus, at most, the enclosing function or package. This change ships the three Tier 2 rules from `docs/design.md` §3.2, settles the `Drain`-in-`main` question (§8 item 2) by shipping that check, and uses the corpus numbers, as the design requires, to confirm the defaults.

## What Changes

- Rule `handle` (default-on): a lifecycle handle returned by `Consumer.Consume`, `PushConsumer.Consume`, `Consumer.Messages`, `KeyValue.Watch`/`WatchAll`/`WatchFiltered`, `ObjectStore.Watch` or `micro.AddService` — and their legacy `nats.KeyValue`/`nats.ObjectStore` twins — that is assigned to `_` or dropped by an expression statement. Nothing can stop or drain it afterwards. A consume or messages call carrying a `jetstream.StopAfter` option is self-terminating and exempt. `main` is not exempt: it is the one place a signal handler would want to `Drain()` the context, and nats.go's own examples keep it.
- Rule `msgloop` (default-on): (1) inside a condition-less `for`, `msg, err := it.Next()` on a `jetstream.MessagesContext` followed by `if err != nil` whose body never leaves the loop (it logs, maybe sleeps, and `continue`s, or falls into an `else`) with no reference to `ErrMsgIteratorClosed` in the loop — after `Stop`/`Drain`, `Next` returns `ErrMsgIteratorClosed` on every call without blocking and the loop never exits; (2) a `MessageBatch` from `Fetch`/`FetchBytes`/`FetchNoWait` (or legacy `Subscription.FetchBatch`) whose `Messages()` is ranged over while `Error()` is never called on it in the function — the batch's terminal error is silently dropped.
- Rule `pubasync` (default-on, corpus-gated): a `PubAckFuture` from `PublishAsync`/`PublishMsgAsync` (`jetstream.JetStream` or legacy `nats.JetStreamContext`) assigned to `_` or dropped, in a package that neither sets an async error handler (`jetstream.WithPublishAsyncErrHandler`, `nats.PublishAsyncErrHandler`) nor awaits `PublishAsyncComplete` anywhere. The future's `Err()` channel and the error handler are the only two places an async publish error surfaces; `WithPublishAsyncAckHandler` is the callback replacement for the future's `Ok()` only and does not exempt a package on its own.
- Rule `drain` extension (default-on): `defer nc.Drain()` in `func main()`, or a `nc.Drain()` in `main` followed directly by process exit (`os.Exit`, `log.Fatal*`, `return`, or the end of `main`). `Drain` returns as soon as it has started the drain goroutine; the exit truncates it. Two of nats.go's own examples (`nats-qsub`, `nats-rply`) do this under a comment promising a drain. Public docs snippets that show `defer nc.Drain()` in `main` are to be corrected, not accommodated.
- Corpus run with the four rules; every new line triaged; per-rule finding/FP counts recorded in the design; `handle` and `pubasync` demoted to opt-in only if the corpus shows a false positive — a site where the message is untrue — on non-test code.
- README rule table and generated `docs/rules.md` extended; `docs/design.md` §3.2, §6 and §8 updated with the outcome.

## Capabilities

### New Capabilities
- `rule-handle`: the `handle` rule — the hook table per package, the discard shapes, the `StopAfter` exemption, messages.
- `rule-msgloop`: the `msgloop` rule — the never-exits loop shape and what disarms it, the unchecked batch shape and its single-definition requirement, messages.
- `rule-pubasync`: the `pubasync` rule — the discard shapes, the two package-wide exemptions, the ack handler's non-exemption, messages.

### Modified Capabilities
- `rule-drain`: adds the `Drain`-in-`main`-then-exit requirement and rewords the deferred-Close requirement's `main` exemption to point at it.
- `analyzer-framework`: adds a package-wide symbol-reference lookup (does any file of the analyzed package use function or method `X` of nats.go) that `pubasync` relies on; the first rule to reason about the package rather than the function.

## Non-goals

- `*nats.Subscription` discards (`_, err := nc.Subscribe(...)`). The design doc parked these behind a `-handle.subscriptions` flag; the corpus has about 480 such sites, 50 of them in production code (natscli, nats.go's examples, go-choria), every one a lifetime-of-connection subscription. A check whose real-world hits are all legitimate ships neither on nor off; the numbers go to `docs/design.md` §8.
- Test and benchmark functions in the `Drain`-then-exit check. A test function's return does not exit the process, so the drain proceeds; only `main` in `package main` is reported.
- Dataflow beyond one definition or one function: a handle stored in a struct field and never stopped, a future passed to a helper that ignores it, a `MessagesContext` parameter whose loop lives in a different function.
- A discarded `MessageBatch` from `Fetch` (`_, err := c.Fetch(10)` sends a pull request whose messages are delivered to nobody and redelivered after `AckWait`). The corpus has 57 such sites, all in tests, half of them deliberately poking a pull; recorded in `docs/design.md` §8 for a later change.
- Loops that fall through to use `msg` after a non-terminating `if err != nil { log }`: that is a nil dereference, not a spin, and is a different rule.
- Special-casing `_test.go` files in any rule. Tests that discard a handle to assert an error, or range a batch to count messages, are triaged as `FP` with that reason; the corpus keeps measuring every rule on the densest maintainer-written usage available.
- Fixes. `handle` cannot know where to `Stop()`; `msgloop` cannot know whether the loop should `break` or `return`; `pubasync` cannot choose between a future and a handler; `drain` needs a handler the user must write.
- Tier 3 rules (`connopts`, `microhandler`), the golangci-lint plugin, the tag.

## Rule defaults

- `msgloop` and the `drain` extension: default-on.
- `handle`, `pubasync`: registered default-on; the corpus task in this change confirms or demotes them to opt-in. A grep over the cached corpus before this proposal counts roughly 28 discarded handles (one in a `main`, the rest in tests that assert an error from the call) and 54 discarded futures (about 8 without a completion wait nearby), so both are expected to stay default-on.

## Shared helpers

Added to `internal/natsapi`: `PackageUses(info, pkg, recv, name) bool` (whether any file of the analyzed package uses the named nats.go function or method), `Discarded(stack) bool` (whether the call at the top of the inspector stack has its first result assigned to `_` or is an expression statement — shared by `handle` and `pubasync`), `IsNamedType(t, pkg, name) bool` (the `StopAfter` test), and `ExitsProcess(info, call) bool` (`panic`, `os.Exit`, `Fatal*` — the process-exit test `msgloop` and `drain` share). Reused: `Callee`, `IsMethod`, `IsFunc`, `IsPkg`, `SingleDefinition`, `EnclosingFuncBody`, and inside `drain` its existing `main` detection and trailing-`Drain` walk. Not needed: the `Masker` (no config literals here) — the design doc's §6 list is corrected.

## Impact

- `Analyzers()` grows from ten to thirteen default-on rules; `drain` reports one more shape.
- `handle` and `pubasync` fire on the discard statement, so `errcheck`-clean code (`_, err := ...; if err != nil`) is exactly what they target; users who consider a lifetime-of-process consume in `main` fine disable the rule per the design's accepted cost.
- Corpus: `handle` and `msgloop` (batch half) will fire mostly on nats.go's own tests, which discard handles to assert an error and range over batches to count messages; each is triaged as `FP` with that reason and `corpus.expected` roughly doubles. One nats.go example (`js-ordered-consume`) discards a `ConsumeContext`, one docs example (`jetstream-basic`) ranges a batch without `Error()`, and two examples (`nats-qsub`, `nats-rply`) drain and exit; all four are `TP` candidates for upstream.
- No new dependencies.
