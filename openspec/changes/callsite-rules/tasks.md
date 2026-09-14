## 1. Shared helpers

- [x] 1.1 Add `IsDurationType` and `StructField` to `internal/natsapi`; verify a table test covers `time.Duration`, a named type with `time.Duration` underlying (not a match), `int64`, and field lookup on a struct with and without the field
- [x] 1.2 Add `SingleDefinition` to `internal/natsapi`; verify a `go/types`-driven table test covers `:=` single value, `:=` multi-value call, `var x = ...`, `=` reassignment (twice → none), range variable (none), parameter (none), package-level (none), address taken (none), and a definition inside a closure (counted)

## 2. Rule duration

- [x] 2.1 Write `testdata/duration/{jetstream,legacy}.go` (micro exposes no Duration inputs) with one `// want` line per scenario in `rule-duration/spec.md` (Request, Timeout option, AckWait field, MaxAge legacy, NextMsg, BackOff elements, named constant, arithmetic, typed unit, zero/negative, millisecond-mindset value, conversion, time.Sleep, int parameter, `opts.Timeout = 5` and `cfg.AckWait = 30 * time.Second` assignments); verify the test fails before the analyzer exists
- [x] 2.2 Implement `analyzers/duration` with type-driven parameter, field and field-assignment discovery and the untyped-constant AST test; verify the analysistest passes
- [x] 2.3 Register `duration` in `Analyzers()`; verify `bin/natsvet ./duration/` from `testdata` prints the expected diagnostics

## 3. Rule subject

- [x] 3.1 Write `testdata/subject/{core,jetstream,legacy,micro}.go` with one `// want` line per scenario in `rule-subject/spec.md` (each invalidity reason, reply argument, NewMsg, Msg literal fields, micro endpoint, wildcard publish vs subscribe, look-alike tokens, concatenated constants, non-constant, queue group with whitespace on core, legacy and micro, valid and empty queue names); verify the test fails before the analyzer exists
- [x] 3.2 Implement `analyzers/subject` with the hook table (subject and queue indexes), `subjectInvalid` reasons in nats.go/server order, the publish-literal check and the queue whitespace check; verify the analysistest passes
- [x] 3.3 Register `subject` in `Analyzers()`; verify the binary prints the expected diagnostics on `testdata/subject`

## 4. Rule ctxdeadline

- [x] 4.1 Write `testdata/ctxdeadline/ctxdeadline.go` with one `// want` line per scenario (Background/TODO to each of the four methods, variable, derived context, jetstream exempt, legacy `Fetch`/`FetchBatch` with `nats.Context(Background/TODO)`, `Subscribe` with the option exempt); verify the test fails before the analyzer exists
- [x] 4.2 Implement `analyzers/ctxdeadline`; verify the analysistest passes and register it in `Analyzers()`

## 5. Rule syncsub

- [ ] 5.1 Write `testdata/syncsub/{core,legacy}.go` with one `// want` line per scenario (callback, queue callback via JetStream, sync, channel kinds, pull, reassigned, parameter); verify the test fails before the analyzer exists
- [ ] 5.2 Implement `analyzers/syncsub` on `SingleDefinition` with the kinds table; verify the analysistest passes and register it in `Analyzers()`

## 6. Rule nilheader

- [ ] 6.1 Write `testdata/nilheader/nilheader.go` with one `// want` line per scenario (pointer and value literal with Set/Add, `m.Header[k] = v` assignment, index read and delete, NewMsg, literal with Header, Header assigned later, nil-safe methods, parameter, reassigned); verify the test fails before the analyzer exists
- [ ] 6.2 Implement `analyzers/nilheader` on `SingleDefinition` and `CompositeFields`, hooking `Set`/`Add` calls and index assignments; verify the analysistest passes and register it in `Analyzers()`

## 7. Rule drain

- [ ] 7.1 Write `testdata/drain/drain.go` with one `// want` line per scenario (adjacent, `if err := Drain()` form, `err := Drain()` + check form, wait between, different connections; deferred Close with `return nc.Drain()`, with trailing `nc.Drain()`, with a wait before returning, and in `main`/`TestXxx` (exempt)); verify the test fails before the analyzer exists
- [ ] 7.2 Implement `analyzers/drain` as a statement-list walk with the single tolerated `if`, plus the per-function deferred-`Close` pass with the `main`/test exemption; verify the analysistest passes and register it in `Analyzers()`

## 8. Corpus and docs

- [ ] 8.1 Run `make corpus`; triage every new line into `scripts/corpus.expected` with `TP` or `FP` and a reason; revisit the `subject` field hook and the `ctxdeadline` request half per the design's risk notes if the numbers say so; verify a second `make corpus` passes
- [ ] 8.2 Extend the README rule table with the six rules; verify the table matches `Analyzers()`
- [ ] 8.3 Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, `misspell -locale US .`, `make lint` and `make test` (with the testdata download step); verify all pass with no output from gofmt and misspell
