## Context

Empty repository; `docs/design.md` §2 fixes the architecture (`go/analysis`, no nats.go import, one package per rule, `internal/natsapi` helpers, module-mode `testdata`). See proposal.md for motivation. Verified on this machine: x/tools v0.43.0 (`go 1.25.0` directive; `analysistest` treats a `go.mod` in the test directory as a module root), Go 1.26 with `go fix -fixtool`, nats.go v1.53.1 in the module cache. The module proxy is not reachable from the sandboxed shell, so downloads run outside it.

## Goals / Non-Goals

**Goals:**
- Prove the three infrastructure paths on real rules: module-mode tests, golden-file fixes, one binary for `natsvet`/`go vet`/`go fix`.
- Make adding a rule a copy-the-shape exercise: registry entry, `analyzers/<rule>`, `testdata/<rule>`, spec.
- Make the two nats.go-derived tables impossible to leave stale.

**Non-Goals:**
- Helpers for rules not in this change (see proposal Non-goals).
- Corpus triage; only the machinery and an initially empty expected file.

## Decisions

**Versions.** `go.mod`: `go 1.25`, `golang.org/x/tools v0.43.0` as the only requirement. `testdata/go.mod`: module `natsvet/testdata`, `github.com/nats-io/nats.go v1.53.1`. Both bump independently; bumping nats.go is what the drift test guards. Alternative: a `go.work` joining both modules — rejected, it would make nats.go a dependency of the linter module in tooling that reads workspaces.

**Registry and binary.** `natsvet.go` exports `Analyzers()` (default-on) and `OptIn()`; `cmd/natsvet/main.go` is `multichecker.Main(append(Analyzers(), OptIn()...)...)`. `multichecker` already delegates to `unitchecker` when invoked by `go vet`/`go fix`, so no second entry point. golangci-lint's module plugin API consumes the same two functions later.

**Opt-in mechanism.** An opt-in analyzer declares `Flags.Bool("enable", false, ...)` and returns early from `Run` when unset. Alternative: register opt-in analyzers only under a global `-optin` flag — rejected, `multichecker` has no global flags and per-analyzer flags surface uniformly as `-legacyjs.enable` in every driver, including `go vet -vettool`.

**Package matching.** `natsapi.Pkg` is a string type whose value is the import path; `IsPkg(obj, pkg)` is true when `obj.Pkg().Path()` equals it or ends in `/vendor/<path>` (GOPATH-style vendoring; module-mode `-mod=vendor` keeps the canonical path). Keyed by path so orbit.go modules become additional constants. `Callee(pass, call)` wraps `typeutil.Callee`; `IsMethod(fn, pkg, recv, name)` derefs the receiver type and compares the named type or interface name, which is what makes interface-typed receivers (`jetstream.Msg`, `micro.Request`) work. `ConstString(pass, expr)` reads `TypesInfo.Types[expr].Value`, so named constants and constant concatenations qualify.

**Generated tables.** One library package `internal/tablegen` with `Generate(testdataDir) (headers, legacy []byte, error)` that loads `github.com/nats-io/nats.go`, `.../jetstream`, `.../micro` through `go/packages` with `Dir` set to the `testdata` module, so the pinned version resolves without network once cached. A thin `internal/tablegen/cmd` is the `go:generate` target and writes `internal/natsapi/headers_table.go` and `legacy_table.go`. The drift test in `natsapi` calls `Generate` and compares bytes. Header table: exported string constants whose value has prefix `Nats-`, mapped to `[]struct{Pkg, Name}` in package order `jetstream`, `nats`, `micro` — the fix's preference order falls out of the slice order. Legacy table: exported `*ast.TypeSpec`, `*ast.FuncDecl` (function or method, receiver pointer stripped) whose position is in `js.go`, `jsm.go`, `jserrors.go`, `kv.go`, `object.go` of the `nats` package, as a set of `Type`, `Func`, `Type.Method` strings. Constants and variables are skipped by construction (only those two decl kinds are visited). Alternative: hand-maintained tables — rejected; 35 headers and ~250 legacy symbols today, and the point of the drift test is that a nats.go bump cannot silently miss additions.

**headerkey.** Inspect `*ast.CallExpr` whose callee is `Get`/`Set`/`Add`/`Values`/`Del` on `nats.Header` or `Get`/`Values` on `micro.Headers`, and `*ast.IndexExpr` whose operand type is one of those; take the key with `ConstString`; look it up case-insensitively; report when the canonical form differs. Fix selection is per file: walk `file.Imports`, resolve each spec's local name with `TypesInfo.PkgNameOf`, skip dot and blank imports, pick the first table entry whose package is imported, else emit the canonical literal. Replacement is a single `TextEdit` over the key expression; the fix never adds imports because an added import can collide with a local name.

**legacyjs.** Walk `*ast.Ident` through `TypesInfo.Uses`. `*types.TypeName` in the `nats` package whose name is in the table → report at the ident. `*types.Func`: with a receiver, key `Recv.Method`; without, key `Name`; report when in the table. Interface method calls resolve to the interface's `*types.Func`, so `js.Publish` keys as `JetStreamContext.Publish`. One diagnostic per ident, so a composite literal type reports once and an option constructor passed to a legacy call reports separately from the call, as the spec requires.

**Test layout.** `analyzers/<rule>/<rule>_test.go` calls `analysistest.RunWithSuggestedFixes(t, testdataDir, Analyzer, "natsvet/testdata/<rule>")` (or `Run` for fix-less rules) where `testdataDir` is `../../testdata`; opt-in rule tests set the analyzer's `enable` flag before running. `make test` runs `cd testdata && go mod download` first; the Makefile is the documented entry point because `analysistest` runs with `GOPROXY=off`.

**Corpus script.** `scripts/corpus.sh`: builds the binary once; for each `corpus.txt` line does a shallow fetch of the pinned commit into `${CORPUS_DIR:-$HOME/.cache/natsvet-corpus}/<repo>`, runs `go mod download` there, then runs the binary with every default-on rule and every opt-in rule except `legacyjs` over `./...` (or the path filter), strips the clone prefix, sorts, and diffs against `corpus.expected` with the trailing `# TP|FP: ...` tag removed from expected lines. A second pass with only `-legacyjs.enable` prints a count per repository. Bash rather than Go: it is glue around `git`, `go` and `diff`, and the Makefile already assumes a POSIX shell.

**CI.** GitHub Actions, matrix of the two newest Go releases; steps: download testdata module, gofmt check, `go vet`, `staticcheck` (pinned version), `misspell -locale US`, `go test ./...`. Corpus is a separate job on the newest Go only, so its network use and runtime never block the unit-test signal.

**License header.** nats-io style, `Copyright 2026 The NATS Authors`, on every `.go` file including generated ones and `testdata`. CI greps for the first line.

**Helpers new vs reused.** All new: `Pkg`/`IsPkg`, `Callee`, `IsMethod`, `ConstString`, the two tables. `EnclosingFunc`, `SingleDefinition`, `CompositeFields`, subject and KV helpers are deliberately absent; each is promoted to `natsapi` by the first rule that needs it, with its own table test.

## Risks / Trade-offs

- [`multichecker` may not yet support being invoked as `go fix -fixtool`] → verify on the pinned x/tools in the first task; if it does not, `go vet` and `natsvet -fix` still cover the spec's fix path and the `go fix` scenario is downgraded to documentation until x/tools catches up.
- [Module downloads blocked in sandboxed shells] → `make test` documents the download step; the step is run once outside the sandbox and everything after is offline.
- [`go/packages` in the generator needs the full nats.go dependency closure cached] → `go mod download` in `testdata` covers it; the drift test skips with a clear message when the module is absent rather than failing confusingly.
- [Header table carries nats.go's own typo `jetstream.TimeStampHeaer`] → the fix emits it verbatim; correct, if ugly. Not worked around.
- [Legacy table keyed by source file names could break if nats.go reorganizes files] → the drift test fails loudly; the file list is one constant in the generator.
- [Corpus repositories need their own `go mod download`, which can be slow] → clones and module cache are reused across runs via `CORPUS_DIR`; CI caches it.

## Open Questions

- Whether the copyright line should read `The NATS Authors` from day one while the module lives under a personal account. Assumed yes, since the intended home is nats-io and changing it later touches every file.
