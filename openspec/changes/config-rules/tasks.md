## 1. Shared helpers

- [x] 1.1 Port `IsValidSubject`, `SubjectIsLiteral`, `SubjectsCollide`, `SubjectIsSubsetMatch` and their tokenizer helpers from nats-server `server/sublist.go` into `internal/natsapi/subject.go`; verify table tests copied from `TestSublistValidSubjects`, `TestSubjectIsLiteral`, `TestIsSubsetMatch` and `TestSublistSubjectCollide` pass unchanged
- [x] 1.2 Add `BucketValid`, `KeyValid`, `SearchKeyValid` in `internal/natsapi/kv.go` with the regexes and `.`/`..` rules from `jetstream/kv.go`; verify a table test covers empty, leading/trailing dot, `..`, space, wildcard in key (bad) vs in search key (ok), `>` trailing (ok) vs mid (bad)
- [x] 1.3 Add `ConstInt`, `ConstDuration`, `ConstBool`, `SliceConstStrings` (with the `complete` flag) and `ConstEnum` to `internal/natsapi`; verify a `go/types`-driven table test covers literals, named constants, `50 * time.Millisecond`, a mixed slice literal, `nil`, and enum identity across two packages declaring the same constant name
- [x] 1.4 Add `TypeRef` and `CompositeFields` to `internal/natsapi`; verify a table test covers `T{}`, `&T{}`, elements of `[]T{{}}` and `[]*T{{}}`, a positional literal (returns not-ok), a same-named type from another package (not-ok), and that the matched `TypeRef` distinguishes two accepted types

## 2. Rule consumerconfig

- [x] 2.1 Write `testdata/consumerconfig/{jetstream,ordered,legacy,ok}.go` with one `// want` line (or `ok.go` line) per scenario in `rule-consumerconfig/spec.md`, messages copied from the spec; verify `go test ./analyzers/consumerconfig` fails because the analyzer does not exist
- [x] 2.2 Implement `analyzers/consumerconfig` with the `fields` alias table (`IdleHeartbeat`↔`Heartbeat`), the `cfg` view, and checks 1–13 in `checkConsumerCfg` order (names, negatives, backoff vs max deliver, description, push, pull, filters, deliver policy, sampling, flow control, durable/name, priority, flow-control ack); verify the analysistest passes
- [x] 2.3 Register `consumerconfig` in `Analyzers()`; verify `bin/natsvet ./consumerconfig/` from `testdata` prints the expected diagnostics and `ok.go` yields none

## 3. Rule streamconfig

- [x] 3.1 Write `testdata/streamconfig/{jetstream,legacy,ok}.go` with one `// want` line per scenario in `rule-streamconfig/spec.md`; verify the test fails before the analyzer exists
- [x] 3.2 Implement `analyzers/streamconfig` with checks in `checkStreamCfgLocked` order (name, description, replicas, max age/duplicates, rollup, counters, discard-new-per-subject, delete marker TTL, scheduling, async persist, mirror exclusions, subjects); verify the analysistest passes, including the mixed constant/variable `Subjects` scenario
- [x] 3.3 Register `streamconfig` in `Analyzers()`; verify `bin/natsvet ./streamconfig/` from `testdata` prints the expected diagnostics

## 4. Rule kvconfig

- [ ] 4.1 Write `testdata/kvconfig/{jetstream,legacy,ok}.go` with one `// want` line per scenario in `rule-kvconfig/spec.md` (config literals, bucket lookups, key methods, watch filters, object store); verify the test fails before the analyzer exists
- [ ] 4.2 Implement `analyzers/kvconfig`: config-literal checks via `CompositeFields`, method-argument checks via the `{pkg, recv, method, argIndex, search}` table; verify the analysistest passes
- [ ] 4.3 Register `kvconfig` in `Analyzers()`; verify `bin/natsvet ./kvconfig/` from `testdata` prints the expected diagnostics

## 5. Corpus and docs

- [ ] 5.1 Run `make corpus`; triage every new line into `scripts/corpus.expected` with `TP` or `FP` and a reason (open an upstream issue or PR for each `TP` and note it in the reason); verify a second `make corpus` passes
- [ ] 5.2 Extend the README rule table with the three rules and refresh `docs/design.md` §3.1 where the specs corrected it (`MaxAckPending`+`AckNone` push-only; KV `History` negative accepted); verify the table matches `Analyzers()`
- [ ] 5.3 Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, `misspell -locale US .`, `make lint` and `make test` (with the testdata download step); verify all pass with no output from gofmt and misspell
