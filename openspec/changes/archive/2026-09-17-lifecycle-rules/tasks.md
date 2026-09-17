## 1. Shared helpers

- [x] 1.1 Add `Discarded(stack []ast.Node) bool` to `internal/natsapi`; verify a `go/types`-driven table test covers an expression statement, `_, err :=`, `_, err =`, `_, _ =`, the `Init` of `if`/`for`/`switch`, `var _ =`, a named first variable (false), a `go`/`defer` statement (false), a call nested inside another call's argument (false), and `return call()` (false)
- [x] 1.2 Add `IsNamedType(t types.Type, pkg Pkg, name string) bool` to `internal/natsapi`; verify a table test covers the named type, a pointer to it (false), a same-named type from another package (false) and an untyped constant (false)
- [x] 1.3 Add `PackageUses(info *types.Info, pkg Pkg, recv, name string) bool` to `internal/natsapi`; verify a table test over a two-file package covers a function used in the other file, an interface method used through an embedding interface, a same-named function from another package (false), and no use (false)

## 2. Rule handle

- [x] 2.1 Write `testdata/handle/{jetstream,legacy,micro}.go` with one `// want` line per scenario in `rule-handle/spec.md` (Consume blank, Consume expression statement, PushConsumer, Messages in an `if` initializer, handle kept, handle returned, `StopAfter` literal, `StopAfter` variable, other options only, KV `Watch`/`WatchAll`/`WatchFiltered` on both APIs, object store `Watch` on both APIs, watcher kept, `ListKeys`, `AddService` blank and kept, field assignment, a user type with its own `Consume`) and an unannotated `_, err := nc.Subscribe(...)` discard; verify the test fails before the analyzer exists
- [x] 2.2 Implement `analyzers/handle` with the hook table, `Discarded` and the `StopAfter` argument test; verify `analysistest.Run` passes on `natsvet/testdata/handle`
- [x] 2.3 Register `handle` in `Analyzers()`; verify `bin/natsvet ./handle/` from `testdata` prints the expected diagnostics

## 3. Rule msgloop

- [x] 3.1 Write `testdata/msgloop/{iterator,batch,legacy}.go` with one `// want` line per scenario in `rule-msgloop/spec.md` (log-and-continue, else branch, `if` initializer, backoff with `time.Sleep`, labeled continue to an outer loop, break on `ErrMsgIteratorClosed`, closed check earlier in the loop, return, `log.Fatal`, conditional and range loops, `Consumer.Next`, timeout-only branch; range without `Error`, legacy `FetchBatch`, `Error` after the range, `Error` in a deferred closure, batch passed to a helper, batch parameter, `select` receive); verify the test fails before the analyzer exists
- [x] 3.2 Implement `analyzers/msgloop`: the condition-less `ForStmt` statement-list match with the then-branch terminator test and the loop-wide `ErrMsgIteratorClosed` scan, and the `RangeStmt` match on `SingleDefinition` with the uses walk; verify the analysistest passes
- [x] 3.3 Register `msgloop` in `Analyzers()`; verify the binary prints the expected diagnostics on `testdata/msgloop`

## 4. Rule pubasync

- [x] 4.1 Write `testdata/pubasync/pubasync.go` (discard, expression statement, `if` initializer, future kept, synchronous `Publish`), `testdata/pubasync/handler/`, `testdata/pubasync/complete/` (the exemption in a different function of the package, no `// want`), `testdata/pubasync/ackhandler/` (`WithPublishAsyncAckHandler` only, `// want` on the discard), `testdata/pubasync/bothhandlers/` (ack and error handler, no `// want`), `testdata/pubasync/legacy/` (report) and `testdata/pubasync/legacyhandler/` (no `// want`); verify the test with pattern `natsvet/testdata/pubasync/...` fails before the analyzer exists
- [x] 4.2 Implement `analyzers/pubasync` with the four `PackageUses` checks at the top of `Run` and the `Discarded` walk over `PublishAsync`/`PublishMsgAsync` on both `JetStream` interfaces; verify the analysistest passes on every subpackage
- [x] 4.3 Register `pubasync` in `Analyzers()`; verify the binary reports on `testdata/pubasync` and `testdata/pubasync/ackhandler`, and nothing on `testdata/pubasync/handler`, `testdata/pubasync/complete` and `testdata/pubasync/bothhandlers`

## 5. Rule drain extension

- [x] 5.1 Extend `testdata/drain/cmd/main.go` with one `// want` line per scenario in the `rule-drain` delta (deferred `Drain` in `main`, `Drain` then `log.Fatalf`, `Drain` as the last statement, `Drain` then `os.Exit`; `Drain` then wait, `Drain` in a goroutine, a non-`main` helper with `defer nc.Drain()` — all unannotated) and flip the existing `defer nc.Close(); nc.Drain()` `main` to the process-exit `// want`; add `defer nc.Drain()` in a `TestXxx` to `testdata/drain/drain_test.go` and a `func main()` in a non-`main` package, both unannotated; verify the test fails before the analyzer changes
- [x] 5.2 Add the `main` process-exit pass to `analyzers/drain` and update its `Doc`; verify the analysistest passes and the deferred-Close pass still reports nothing in `main`

## 6. Corpus, defaults and docs

- [x] 6.1 Run `make corpus`; triage every new line into `scripts/corpus.expected` with `TP` or `FP` and a reason (a test that discards a handle to assert an error, or ranges a batch to count messages, is `FP` with that reason; the `js-ordered-consume`, `jetstream-basic`, `nats-qsub` and `nats-rply` examples are `TP`); verify a second `make corpus` passes
- [x] 6.2 Apply the demotion rule from design.md (an `FP` on non-test code where the message is untrue): if `handle` or `pubasync` qualifies, move it to `OptIn()` with an `enable` flag and update its spec's purpose and the README; record per-rule finding/FP counts and the decision in design.md Context; verify `make corpus` still passes after any move
- [x] 6.3 Add the three rules to the README rule table, update the `drain` row for the new shape, and regenerate `docs/rules.md` (`make generate`); verify `TestRulesDocUpToDate` and the README-matches-registry check pass
- [x] 6.4 Update `docs/design.md`: §3.2 corrections (`PushConsumer.Consume`, `StopAfter`, the subscriptions flag dropped with its numbers, the never-exits loop shapes, package-wide `pubasync` exemptions and the ack handler, the `drain` extension as shipped), §6 item 5 marked done with the Masker dropped, §8 items 2 and 3 answered and new items for the discarded `Fetch` batch, `Subscription` discards and `go`/`defer` discards; verify the document's status line and §6 progress paragraph name this change
- [x] 6.5 Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, `misspell -locale US .`, `make lint` and `make test` (with the testdata download step); verify all pass with no output from gofmt and misspell
