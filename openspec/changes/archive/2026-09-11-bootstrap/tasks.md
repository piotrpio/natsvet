## 1. Module and skeleton

- [x] 1.1 Create `go.mod` (`github.com/piotrpio/natsvet`, `go 1.25`, `golang.org/x/tools v0.43.0`), `LICENSE` (Apache 2.0), `.gitignore`; verify `go build ./...` succeeds on an empty module
- [x] 1.2 Create `testdata/go.mod` (module `natsvet/testdata`, nats.go v1.53.1) and a `Makefile` with `build`, `test` (runs `cd testdata && go mod download` first), `lint`, `generate`, `corpus` targets; verify `make test` passes with no packages yet and the nats.go module is present in the cache afterwards (run the download outside the sandbox)
- [x] 1.3 Create `natsvet.go` with `Analyzers()` and `OptIn()` returning empty slices, and `cmd/natsvet/main.go` on `multichecker`; verify `go run ./cmd/natsvet -h` lists no analyzers and exits cleanly
- [x] 1.4 Verify the `go fix -fixtool` path: run `go fix -fixtool=$(go env GOPATH)/bin/natsvet ./cmd/...` on the empty analyzer set and record in design.md whether it delegates correctly on x/tools v0.43.0 (see Risks)

## 2. internal/natsapi helpers

- [x] 2.1 Implement `Pkg`, `Core`/`JetStream`/`Micro`, `IsPkg` with vendored-suffix matching; verify table tests cover exact path, `/vendor/` suffix, unrelated package, nil package
- [x] 2.2 Implement `Callee` and `IsMethod` (pointer receiver deref, interface receivers); verify with a `go/types`-driven table test over a small in-test source that defines named, pointer and interface receivers
- [x] 2.3 Implement `ConstString`; verify table test covers literal, named constant, constant concatenation, non-constant expression

## 3. Generated tables

- [x] 3.1 Implement `internal/tablegen.Generate(testdataDir)` loading the three nats.go packages via `go/packages` with `Dir` = testdata; verify a unit test against the cached nats.go finds `Nats-Msg-Id` in both `nats` and `jetstream` and `Nats-Service-Error` in `micro`
- [x] 3.2 Implement the legacy symbol collection over `js.go`, `jsm.go`, `jserrors.go`, `kv.go`, `object.go`; verify a unit test finds `JetStreamContext`, `Subscription.Fetch`, `Msg.Ack`, `Conn.JetStream`, and does not find `MsgIdHdr` or `Conn.Publish`
- [x] 3.3 Add `internal/tablegen/cmd` and a `//go:generate` directive in `natsapi`; run it and commit `headers_table.go` and `legacy_table.go`; verify `go generate ./internal/natsapi && git diff --exit-code` is clean
- [x] 3.4 Add the drift test `TestTablesUpToDate` in `natsapi` (skips with a message when the testdata module is not downloaded); verify it passes, then temporarily edit one table entry and confirm it fails naming the difference, then revert

## 4. Rule headerkey

- [x] 4.1 Write `testdata/headerkey/` packages with `// want` comments covering every scenario in `rule-headerkey/spec.md` (nats.Header methods, map index, jetstream.Msg.Headers, micro.Request.Headers, named constant, exact-case, user header, non-constant) plus `.golden` files for the four fix-selection cases (jetstream only, nats only, both, micro only, no constant available); verify `go test ./analyzers/headerkey` fails because the analyzer does not exist yet
- [x] 4.2 Implement `analyzers/headerkey` detection with the message from the spec; verify `analysistest.Run` expectations pass
- [x] 4.3 Implement fix selection per file and the single `TextEdit`; verify `analysistest.RunWithSuggestedFixes` matches all golden files
- [x] 4.4 Register `headerkey` in `Analyzers()`; verify `go run ./cmd/natsvet ./testdata/headerkey/...` from the `testdata` module directory reports the expected diagnostics, `-diff` prints the fixes, and `-fix` on a scratch copy applies them

## 5. Rule legacyjs

- [x] 5.1 Write `testdata/legacyjs/` packages with `// want` comments covering every scenario in `rule-legacyjs/spec.md` (entry point, interface method, type in declaration and field, option constructor, `Msg.Ack`, `Subscription.Fetch`, config literal, jetstream twin, header constant, core API); verify the test fails before the analyzer exists
- [x] 5.2 Implement `analyzers/legacyjs` with the `enable` flag and ident walk over `TypesInfo.Uses`; verify the test (which sets `enable`) passes and that running without the flag reports nothing
- [x] 5.3 Register `legacyjs` in `OptIn()`; verify `go run ./cmd/natsvet -legacyjs.enable ./testdata/legacyjs/...` reports and the same command without the flag reports nothing

## 6. Corpus gate

- [x] 6.1 Write `scripts/corpus.txt` with pinned commits for nats.go (`examples/`, `test/`), natscli, nack, nex, and each orbit.go module `test/` directory; verify every line resolves with `git ls-remote`
- [x] 6.2 Write `scripts/corpus.sh` (shallow fetch by commit, `go mod download`, run all rules except `legacyjs`, normalize, diff against `corpus.expected` ignoring `# TP|FP` tags, separate `legacyjs` count pass) and an empty `scripts/corpus.expected` with a header comment describing the line format; verify `make corpus` runs end to end and, with only `headerkey` active, reports its findings or none
- [x] 6.3 Triage any `headerkey` findings from the run into `corpus.expected` with `TP`/`FP` tags; verify a second `make corpus` passes

## 7. CI, README, conventions

- [x] 7.1 Add `.github/workflows/ci.yml` with the lint/test job on the two newest Go releases (testdata download step first) and a separate corpus job; verify the workflow file passes `actionlint` or a dry parse and the Makefile targets it calls exist
- [x] 7.2 Add the orbit.go license header (`// Copyright 2026 Synadia Communications Inc.` + Apache boilerplate) to every `.go` file (including generated and testdata) and a `make lint` check for its first line; verify `make lint` passes
- [x] 7.3 Write `README.md` skeleton: purpose, install, the three invocation modes, rule table with `headerkey` and `legacyjs` (summary, default, fix), link to `docs/design.md`; verify the rule table matches `Analyzers()`/`OptIn()`
- [x] 7.4 Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, `misspell -locale US .`, and `make test` (with the testdata download step); verify all pass with no output from gofmt and misspell
