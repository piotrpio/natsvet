## Context

All Tier 1 rules are implemented and archived; `Analyzers()` registers ten default-on rules, `OptIn()` one. The corpus covers five nats.go-adjacent repositories at 30 triaged findings. A parallel exploratory run (17 third-party users plus nats-server's tests as an oracle) reported zero false positives from the config rules, confirmed nats-server's `server` package as the richest source of deliberately-invalid configs, and found that x/tools v0.43 cannot load a Go 1.27 module. See proposal.md for motivation. Verified this session: x/tools v0.49.0 keeps the `go 1.25.0` directive and is in the local module cache; v0.50.0 requires `go 1.26.0`. Pins resolved: nats-server `8d8b69a` (tag v2.14.7), go-choria `7bda4fe`, synadia-io/connect `562f73a`, eventing-natss `d7e9d9f`.

## Goals / Non-Goals

**Goals:**
- A `v0.1.0` that installs, states its version, and has passed a corpus wide enough to back the README's claim.
- Keep the CI matrix (Go 1.26 and 1.25) intact through the x/tools bump.

**Non-Goals:**
- Any rule behavior change beyond a hook-table fix for a corpus false positive (proposal Non-goals).
- Changing the corpus mechanism; only its list and expectations grow.

## Decisions

**x/tools v0.49.0, not v0.50.0.** v0.50 moves the directive to `go 1.26`, which would either drop the Go 1.25 CI leg or force every user of `go vet -vettool` onto 1.26. v0.49 is the newest version that keeps 1.25 and includes the loader fix. Bump with `go get golang.org/x/tools@v0.49.0 && go mod tidy` from the module cache, then run the full suite: the `analysis`, `analysistest`, `inspector` and `typeutil` APIs the rules use are unchanged in that range, and the drift test guards `go/packages` loading in `tablegen`.

**Corpus entries.** Four lines in `corpus.txt`, each `./...` except nats-server, which is `. ./server/... ./test/...` (its root package and cmd are not interesting and `./...` would also pull tooling). Order in the file is by expected runtime so a failing early entry surfaces fast. Runtime budget: the corpus CI job today takes about four minutes; nats-server adds ~16s of analysis plus its module download, eventing-natss brings the Kubernetes dependency tree (large download, cached by `actions/cache` on the `corpus.txt` hash). If the job exceeds ten minutes, narrow go-choria and eventing-natss to the directories that import nats.go — decided by measurement in the first run, not now.

**Triage policy for the widened corpus.** nats-server's tests will produce findings that are all deliberate-invalid configs (the parallel run saw ~30); each is tagged `FP` with the test's purpose. A finding on go-choria, connect or eventing-natss is presumed a true positive until read; a `TP` gets an upstream issue or PR reference in its reason. A false positive attributable to a missing hook-table entry (the `Bind` shape) is fixed in this change with a spec scenario; anything needing new analysis is recorded as `FP` with reason and listed in `docs/design.md` §8.

**`-version` is a pre-check in `main`.** `multichecker.Main` owns the flag set, and its `-V` flag is the `go vet` cache protocol (it prints `version devel comments-go-here buildID=...`, which is by design). So `cmd/natsvet/main.go` checks `os.Args` for `-version`/`--version` before calling `Main`, prints `natsvet <Main.Version> (<vcs.revision>[, modified])` from `debug.ReadBuildInfo()`, and exits 0. `Main.Version` is `v0.1.0` for a `go install ...@v0.1.0` build and `(devel)` for a checkout build, where the revision carries the information. Alternative: `ldflags -X` at build time — rejected; `go install` users get no ldflags, and build info is what the toolchain already embeds.

**README shape.** Sections: what it is; install (`@v0.1.0` and `@latest`); use (three invocation modes, flags); rules — the table stays, followed by one subsection per rule with a before/after snippet lifted from the analyzer's `Doc` so the two never diverge; corpus — the repository list with a one-line reason each and the guarantee ("a release passes the corpus with every finding triaged; the expected file is the audit trail"); develop; license. No "found in the wild" section: the corpus has produced no true positives outside tests that assert the error, and the README should say that rather than invent examples.

**`docs/design.md` updates.** §4.3 corpus list; §8 gains: (a) legacy `js.Subscribe*(subj, nats.Bind(stream, consumer))` with a non-empty constant subject — the subject must equal the consumer's `FilterSubject` exactly, so a wildcard or unrelated constant is almost certainly wrong (`ErrSubjectMismatch`); (b) `headerkey` near-miss: an unknown `Nats-*` key within a small edit distance of a known header (`Nats-TTLSeconds` vs `Nats-TTL`, seen in go-choria); (c) config rules: checks inside nested `Mirror`/`Sources` subject transforms, `SubjectTransform` and `RePublish`, with the server test cases to derive from. A status line under the title records the tag.

**Tag.** Annotated `v0.1.0` on the commit where every gate passed, created and pushed by the maintainer; the change's last task verifies the gates and hands over the exact commands. The GitHub release notes are the README rule table; no CHANGELOG file in this release.

## Risks / Trade-offs

- [x/tools bump changes `analysistest` or diagnostic positions] → the full suite runs before anything else in this change; a behavior change shows up as a failing `// want` and is investigated, not papered over.
- [Corpus job time or download size becomes impractical] → measure on the first run; narrow patterns per the decision above; the pinned commits keep the result deterministic either way.
- [go-choria or eventing-natss fail to load (build tags, cgo, replace directives)] → `natsvet` prints the load error as a finding line, which fails the diff visibly; fix the entry (directory, patterns) or drop the repository with the reason recorded in `corpus.txt`.
- [A widened corpus finds a false positive that needs real analysis, not a table fix] → recorded as `FP` with reason and in §8; the tag is not blocked by a known, documented, accepted false positive.
- [Tagging under `piotrpio` makes the module path sticky] → accepted and stated in `docs/design.md`; the path question is now live for the next milestone.
