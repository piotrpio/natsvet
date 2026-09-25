## 1. Fixes already made

- [x] 1.1 Record each handle in the file that declares it: `collectHandles` skips `Defs` outside the file's syntax tree. Verify: `testdata/migrate/multifile` (handle in `handle.go`, then `other.go` and an internal `other_test.go`) fails `TestHandleInDeclaringFile` and `TestApplyTestdata` before the fix (`undefined: js` after removal) and passes after. Outside the new package, the golden plan changes only in the "After" text of the three parameter handles in `scenarios/corpus.go`.
- [x] 1.2 Make facts independent of map order, and name positions relative to the module: unit facts in first-seen order (`units.go`), subscription uses in source order via `posID` (`subscribe.go`). Verify: `testdata/migrate/order` fails `TestPlanDeterministic` before and passes after, and 8 plans of eventing-natss are byte-identical, with no absolute path.

## 2. Failing tests first

- [x] 2.1 Add a gofmt property to `applyAll`: a file that was gofmt-clean before a step must be gofmt-clean after it. Add `testdata/migrate/fieldfmt`: a handle in an aligned struct field, and a file importing `bufio` that needs `context`. Verify: `TestApplyTestdata` fails on both, naming the step and the file.
- [x] 2.2 Add `testdata/migrate/guided`, a component whose `PullSubscribe` site is guided, and its hand-migrated text as a fixture under `internal/migrate/testdata/`. Add a resume test that applies the machine steps, writes the fixture, plans again with the same answers, and applies the new plan. Verify: it fails today (a second `add-handle` and a blocked component).
- [x] 2.3 Add the resume property over the testdata plan: after each `add-handle` step, plan a copy again and apply the new plan to the end. The result must equal the files of the uninterrupted chain. Verify: it fails today.
- [x] 2.4 Add the id tests:
  - a `component: skip` answer and a site-scoped `subscribe-target: push` answer on the lower of two stacked `Subscribe` calls (new `testdata/migrate/neighbors`);
  - then insert an import and a function at the top of the file and plan again;
  - separately, rename the skipped component's function and plan again.

  Verify: today the skip goes stale and the push answer moves to the other call, and the rename falls back to `migrate` instead of failing.

## 3. gofmt-stable steps

- [x] 3.1 Insert new imports in sorted position within their group in `importEdits`. Verify: the `bufio`/`context` case of 2.1 passes, and no golden import block is out of order.
- [x] 3.2 Add the edit-composition helper: map each batch's edits back to the coordinates before the step, merge overlapping and touching ranges, and replace each with the final text. The simulation checks every composed step against the sequential batches. Verify with unit tests: an insertion inside an earlier insertion, a rename inside inserted text, a deletion next to an insertion.
- [x] 3.3 Let span marks survive edits inside them, adjusting their length (`applyBatch`); drop them only when an edit crosses their boundary. Verify with a unit test on a placeholder span with an inner deletion.
- [x] 3.4 Add the formatting batch: for a sim file that was gofmt-clean before a step, compute the whitespace difference to `format.Source` of the result by aligning `go/scanner` tokens, log it as a second batch, and compose it into the step's edits. Verify: the struct-field case of 2.1 passes, and so does the gofmt property over all of `./migrate/...`.

## 4. The finish step and resuming

- [x] 4.1 Merge `remove-legacy` and `rename` into one `finish` step: the step kind, `waits_on`, `expectedResidue` in the test applier, and the goldens. Verify: `TestApplyTestdata` and `TestApplyRefusesChangedFile` pass, and the reviewed golden diff shows only merged steps.
- [x] 4.2 Recognize siblings, threaded flows and placeholders in the loaded tree, and seed the simulation's sibling, `root:` and `placeholder:` marks from them. A sibling has the same name and type as the one `add-handle` would declare: for a local, declared later in the same function scope; for a field, anywhere in the same struct; for a parameter, anywhere in the same parameter list. The connection is not checked. `add-handle` then plans only for handles and flows without a sibling. Verify: tests 2.2 and 2.3 pass, and so does the "Sibling after an inserted line" scenario (a log line inserted after the error check, and a moved field).
- [x] 4.3 Stop treating placeholder statements as sites or boundaries in the graph. Add the "Unrelated jetstream variable" case (a `jsNew` with no legacy `js` next to it) to testdata. Verify: the "Placeholder does not block" and "Unrelated jetstream variable" scenarios pass.

## 5. One-step components

- [x] 5.1 Emit one `component` step, composing the `add-handle`, site and `finish` edits, for components marked `oneCommit`. Their sites' `after` becomes the final text of their original line span. Verify: for `migrate/independent` and `migrate/multifile`, the "One-commit hint" scenario holds (one step, type-checks, no placeholder, final `after`). The gofmt and resume properties still pass.

## 6. Plan shape

- [x] 6.1 Pin step 0 to `go get github.com/nats-io/nats.go@<tableVersion>`. Verify: `TestStepZero` expects `@v1.53.1` for `testdata/migrate-old`. A new case for a newer nats.go expects no step 0 and the behavior-facts note.
- [x] 6.2 Leave unmapped sites out of every step, and drop steps left without sites. Verify: the golden has no step listing `migrate/legacyonly/legacyonly.go`'s unmapped site, and the site's `step` is empty.
- [x] 6.3 Record each site's enclosing function (`<pkg>.<Func>`, `<pkg>.<Type>.<Method>`, none at package level), and give components `functions` and `test_only`. Verify: tests for the three "Sites name their enclosing function" scenarios, and `migrate/legacytests` is test-only.
- [x] 6.4 Switch to the new ids (component: declaration and handle; site: function, symbol and text hash, with an ordinal only for identical texts). Move `natsvet-migrate.json` to version 2, and reject version 1 with a message naming the id formats. Make a stale `component: skip` answer an error that names it; other stale answers stay reported. Verify: test 2.4 passes (including the renamed function now failing with the answer named), the decisions tests use the new ids, and there are tests for the version 1 rejection and the "Stale site answer after migration" scenario.
- [x] 6.5 List the covered component ids on the `component` pending decision, in place of the site count. Verify: a test for the "Component decision lists its components" scenario.
- [x] 6.6 Add step previews (`before`/`after` on `add-handle`, `finish` and `component` steps).
  - Markdown: each step once, the go-get step in its own section only, unmapped sites in their own section, the `component` question listing its components.
  - Extend `TestMarkdownMirrorsJSON` to require every step exactly once.
  - Verify: that test and the "Finish step preview" scenario pass.
- [x] 6.7 Set `schemaVersion` to 2. Verify: `TestSkillVersion` fails until 8.1, then passes.

## 7. apply

- [x] 7.1 Write the `apply` tests first, driving `Main([]string{"apply", ...})` on copies of testdata modules:
  - next machine step;
  - `-component` stopping at a guided step;
  - `-dry-run` writing nothing;
  - the go-get step printed for `testdata/migrate-old`;
  - an unanswered `component` decision refused;
  - rollback through a test hook that corrupts one edit of the selected step;
  - nothing left to apply.

  Verify: each fails with "unknown command" before 7.2.
- [x] 7.2 Move the test applier's splice and hash check (`applyStep`) into the package, and add the `apply` subcommand:
  - plan flags plus `-component` and `-dry-run`;
  - step selection and stops;
  - the `component` decision check;
  - the type-check of the touched packages with byte-for-byte rollback;
  - the unified diff for `-dry-run`;
  - the output (step, files, functions, next step).

  Verify: the 7.1 tests pass, and `TestApplyTestdata` uses the shared splice.
- [x] 7.3 Migrate `testdata/migrate/guided` end to end with `apply` alone, the test writing the guided fixture between invocations. Verify: the final files equal those of the test applier's chain (task 2.2).
- [x] 7.4 Add `apply` to the migrate usage text and `natsvet help migrate`. Change the package doc and usage text so that `plan` never edits code and `apply` writes one step. Verify: the analyzer-framework help test still passes, and the usage lists `apply` with its flags.

## 8. Skill and docs

- [x] 8.1 Rewrite `internal/migrate/SKILL.md` for schema 2 around `apply`:
  - answer decisions with the user;
  - `natsvet migrate apply ./...` for the next step, `-component <id>` for a component;
  - hand-migrate guided sites to their templates, then plan again to see what is left;
  - gofmt is safe between steps;
  - unmapped sites are reported, not rewritten;
  - `finish` and `component` steps;
  - test with `go test -run` on the functions `apply` names, and the full package at component boundaries;
  - component ids in commit messages;
  - no teaching of byte-offset splicing.

  Verify: `TestSkillVersion` passes, and a reading of the skill against a fresh plan of `testdata/migrate` finds no instruction the plan or `apply` contradicts.
- [x] 8.2 Update `docs/design.md`:
  - §3.5: the resumable plan, `finish` and `component` steps, `apply`, schema 2; the next migration change covers threading through function results only;
  - §6.

  Show `natsvet migrate apply` and name schema 2 in the `README` migrate section. Verify: `misspell` and a read-through.

## 9. Corpus and trial

- [x] 9.1 Run `scripts/migrate-corpus.sh` on go-choria, eventing-natss and natscli, and record counts next to the migrate-plan table in design.md, explaining every change in machine steps or components. Run `TestApplyCorpus` on natscli, with a re-plan after every step, in the env-gated test only. Verify: both pass, and 5 runs of each plan are byte-identical.
- [x] 9.2 Repeat the nats-surveyor trial end to end, with an agent following `natsvet migrate skill` and `natsvet migrate apply` only (outside the sandbox: the module proxy). Record the `apply` runs, the plans and every hand edit in design.md under "Resolved during implementation". Verify: the agent writes no applier code, the build never breaks between steps, and `natsvet -legacyjs.enable ./...` reports only the unmapped sites.

## 10. Final checks

- [x] 10.1 Run `make testdata-deps`, `make lint` (gofmt, go vet, staticcheck, misspell, license headers) and `go test ./...`, all clean.
- [ ] 10.2 At archive, revise the Purpose of `openspec/specs/migration-planner/spec.md` so that `plan` never edits code and `apply` writes one step at a time. Verify: `openspec validate --strict` passes after the archive.
