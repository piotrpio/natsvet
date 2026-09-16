## 1. Toolchain

- [x] 1.1 Bump `golang.org/x/tools` to v0.49.0 (`go get` + `go mod tidy` from the module cache) and verify `make test` and `make lint` pass unchanged, including the table drift test
- [x] 1.2 Add the `-version` pre-check to `cmd/natsvet/main.go` printing module version and VCS revision from build info; verify `go run ./cmd/natsvet -version` prints `(devel)` with the checkout revision, `-V=full` output is unchanged, and `-version` does not appear in analysis flags

## 2. Corpus

- [x] 2.1 Add nats-server (v2.14.7, `. ./server/... ./test/...`), go-choria, synadia-io/connect and eventing-natss with their pinned commits to `scripts/corpus.txt`; verify every commit resolves with `git ls-remote` and `make corpus` loads each entry without a package-load error line
- [x] 2.2 Triage every new finding into `scripts/corpus.expected` (`FP` with the test's purpose for nats-server; `TP` with what is wrong plus an issue draft in the summary, or `FP` with reason, for the others; fix a hook-table false positive in this change only if it is of the `Bind` kind, with a spec scenario); verify a second `make corpus` passes and record the job's wall time
- [x] 2.3 If the corpus run exceeds ten minutes, narrow go-choria and eventing-natss to the directories that import nats.go and note it in `corpus.txt`; verify the run stays under budget with identical findings (not needed: the cold run of the full widened list took 75s)

## 3. Documentation

- [x] 3.0 Add `internal/docgen` rendering `docs/rules.md` from `Analyzers()`/`OptIn()` `Doc` strings with a `go:generate` directive in `natsvet.go` and `TestRulesDocUpToDate`; verify the generated file lists all eleven rules with default and fix state and that editing a `Doc` without regenerating fails the test
- [ ] 3.1 Rewrite README: install via `@latest` with a note that the module path may move before the first tag, rule table linking to `docs/rules.md`, corpus section with the repository list and the guarantee; verify the rule table matches `Analyzers()`/`OptIn()`
- [ ] 3.2 Update `docs/design.md`: status line (main release-ready, first tag after the repository move, move after dogfooding), §4.3 corpus list, §8 follow-ups (Bind-mismatch heuristic, `headerkey` near-miss, nested-literal config checks); verify `misspell -locale US` passes on the doc

## 4. Release gate

- [ ] 4.1 Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, `misspell -locale US .`, `make lint`, `make test` and `make corpus` on the candidate commit; verify all pass with no output from gofmt and misspell
- [ ] 4.2 Confirm CI is green on the pushed candidate commit for both Go versions; verify with `gh run view`
- [ ] 4.3 Verify the install path without a tag: `go install github.com/piotrpio/natsvet/cmd/natsvet@latest && natsvet -version` prints the pseudo-version of the pushed `main` and `natsvet -h` lists the eleven rules
