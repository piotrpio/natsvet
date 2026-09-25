## Context

See proposal.md (Why) for the field report. The state this design works from, observed in the code at `78f53a9` and reproduced on scratch modules:

**How steps are produced.** The planner simulates its steps on in-memory copies of the files (`sim` in `steps.go`):
- Each step's intents are resolved into the coordinates of the text the earlier steps left (`sim.cur`). They are applied as one batch per file, and logged, so later offsets map through them.
- Text the planner inserts carries marks: `markSibling` on each `<name>New` identifier, and span marks for `root:<site>` and `placeholder:<handle>`. `removeIntents` and `renameIntents` find what they delete and rename through these marks.
- The marks exist only inside one planner run.

**Re-planning mid-component.** The loaded tree then contains `jsNew` and `_ = js`, and the planner sees:
- `js` as an ordinary legacy handle, so it plans a second `add-handle` (bug 1);
- `_ = js` as a use it cannot thread, so it marks the component blocked, with no removal or rename.

A component with a guided site has to be re-planned mid-component: the hand edit breaks the hash chain, and `remove-legacy` carries no edits in the first plan (it `waits_on` the guided site). So such a component can never be finished.

**gofmt.**
- `importEdits` inserts `"context"` before the first import spec (bug 3).
- `declIntent` inserts `\tjsNew jetstream.JetStream\n` into a struct without realigning its neighbors.
- A struct-field component was gofmt-dirty after every step, including the last.

**Ids.** Ids are positions (`file:line:col`). `answerSet.lookup` falls back to the module answer when a scoped answer's id no longer matches anything, and `stale` only reports the unmatched answer.

**Already fixed in the working tree:**
- `collectHandles` ranged over the whole package's `info.Defs` once per file, so a handle was recorded in the package's last-sorted file. Removal then looked for its placeholder marks in the wrong file, and `declKind` walked the wrong syntax tree.
- `units.go` and `subscribe.go` ranged over maps while collecting facts.
- A fact printed an absolute position.

The testdata packages `migrate/multifile` and `migrate/order` cover these, together with `TestHandleInDeclaringFile` and `TestPlanDeterministic`.

## Goals / Non-Goals

**Goals:**
- Two paths give identical files:
  - apply the steps of one plan to the end;
  - apply some steps (or migrate some guided sites by hand), plan again, and apply the new plan to the end.
- Every intermediate tree a plan reaches compiles, is gofmt-clean if it started so, and can be planned again.
- The sibling sequence becomes invisible where it adds nothing.
- An agent applies steps with `natsvet migrate apply` and writes no applier code of its own.

**Non-Goals:**
- Recognizing partial migrations that do not follow the step shapes (a sibling under another name, a sibling in another scope).
- Recovering the migration after an agent deletes a placeholder by hand. The component then fails to compile, which the build step reports.

## Decisions

**Resume by seeding the simulation from the loaded tree.**
- *Recognition.* After the graph is built, the planner looks for the sibling of each threadable handle `h`: an object named `h.sibling` of type `h.newType`. Where it counts as a sibling:
  - for a local, declared later in the same function scope than the legacy declaration;
  - for a field, anywhere in the same struct;
  - for a parameter, anywhere in the same parameter list.

  The connection the sibling was created from is not checked. Exact adjacency was rejected in the grill: a log line inserted by hand, or a field moved while resolving a merge, would make the planner miss the sibling and plan a second `add-handle`, which is bug 1 again. Checking the connection would mean tracking connection expressions through flows, and would guard only against a coincidental `jsNew` of the right type on another connection.

  It also recognizes:
  - placeholders: `_ = <ident>` statements whose identifier is a handle or a recognized sibling;
  - threaded flows: a keyed element `jsNew: …` next to `js: …`, an argument after the legacy one, an assignment after the legacy one.
- *Seeding.* The recognized state is loaded into the simulation's marks exactly as the planner's own `add-handle` would have left them:
  - `markSibling` on every identifier whose object is the sibling, in every loaded file;
  - a `placeholder:` span over the placeholder lines;
  - a `root:` span over the sibling's root statement and its error check.
- *What changes.* `add-handle` plans intents only for handles and flows without a sibling. `finish` needs no change: it finds everything through the marks, in both cases.
- *The graph.* It treats placeholder statements as neither sites nor boundaries.
- *Alternatives:*
  - Marker comments on inserted code (`// natsvet:sibling`) — rejected. They are noise in reviewed commits and vanish when an agent edits the line, while name, type and adjacency are exactly what the steps guarantee.
  - Recording progress in `natsvet-migrate.json` — rejected. The planner never writes that file, and progress kept apart from the code drifts from it.
  - Forbidding re-plans inside a component — rejected. A guided site makes the re-plan unavoidable.

**`remove-legacy` and `rename` become one `finish` step.**
- *Why merge.* Between the two old steps the tree held a sibling with no legacy handle. Only a `New` suffix tells such a tree apart from jetstream code someone named `kvNew`, so a re-plan there would have had to guess. The split mirrored a gopls rename done by hand, and has no value once both are machine edits: the deletions and the renames never overlap.
- *Alternative:* keep two steps and treat any `<x>New` of a jetstream handle type with no `x` in scope as a sibling to rename — rejected. It renames code the planner did not write.

**One edit-composition helper, used by gofmt-stable steps and one-step components.**
- `sim` keeps logging fine-grained batches, so `cur` and marks work as today. What changes is the emitted `Step.Edits`, which may cover several logged batches.
- To compose them:
  1. Map every edit of every batch back to original coordinates. An edit inside text inserted by an earlier batch maps to that insertion's point.
  2. Merge overlapping and touching ranges.
  3. Replace each merged range with the final text between its mapped ends.
- The result is non-overlapping by construction.
- The simulation checks every composed step: applying the composed edits to the text before the step must give the text after the sequential batches. A mismatch is a planner error, not a silently wrong plan.
- *Alternative:* diff the text before and after (common prefix and suffix per file) — rejected. One replacement spanning a whole file region swallows the positions of other components' sites, and their later steps then fail to map.

**gofmt-stable steps: sorted imports, plus a formatting batch.**
- *Imports.* `importEdits` inserts each new import in sorted position within its group: standard library (`context`) among the standard imports, `nats.go` and `jetstream` among the others. A new standard group is started before the first group when the block has only third-party imports.
- *Formatting batch.* When a sim file was gofmt-clean before a step (`format.Source(text) == text`), the planner formats the text after the step's code batch.
  - Its whitespace-only difference is computed by walking both texts with `go/scanner`: the token sequences must be equal, and each inter-token gap that differs becomes a minimal insertion or deletion.
  - That difference is logged as a second batch and composed into the step's edits.
- *Marks.* Identifier marks never sit in whitespace. `applyBatch` is changed so that span marks survive a deletion inside them, with their length adjusted, and are dropped only when an edit crosses their boundary.
- *Unequal token sequences* (an import the planner misplaced): the step ships without formatting edits, and the test's gofmt property fails.
- *Alternatives:*
  - The applier formats — rejected. The next step's hash then depends on the applier's formatter.
  - Agents format only at the end of a component — rejected. Commits between the steps of a multi-commit component would fail a gofmt CI check.

**One step for components safe in one commit.**
- *Which components.* Those `planComponent` already marks `oneCommit`: no blockers (no guided site, no pending decision, every handle threadable), one package. The simulation runs their `add-handle`, site and `finish` steps as today; the plan emits one `component` step whose edits compose all of them.
- *Site text.* Each site's `after` becomes the final text of its original line span (`cur` through the component's batches), so reviewers see `js, err := jetstream.New(nc)` rather than the sibling.
- *Fit with migrate-plan.* The earlier change rejected atomic components because service-sized components span packages and would be one uncompilable diff. That argument does not apply to a package-local component whose steps are all mechanical: its combined diff is the migration.

**Step 0 pins `tableVersion`.**
- The command becomes `go get github.com/nats-io/nats.go@` + `tableVersion`, under the same `older(module, table)` condition.
- `@latest` made plans depend on the day they ran, and pulled unrelated requirement bumps (`x/crypto` 0.49 to 0.57 in the trial).
- *Alternative:* per-target minimum versions, skipping step 0 when the used API already exists — rejected, as in migrate-plan. The facts that make sites mechanical are behavioral (default timeouts, empty `Fetch`, create semantics) and were verified on the table's version.

**Ids from declarations and source text.**
- `<pkg>` is the package's import path relative to the module path (`surveyor`, `internal/store`). The module's root package uses its name. An external test package keeps its `_test` suffix.
- *Component id.* From the declaration of its first handle, in position order:
  - `<pkg>.<Func>#<name>` for a local or parameter (`<Type>.<Method>` for methods, without `*` or type parameters);
  - `<pkg>.<Type>.<field>` for a field;
  - `<pkg>#<name>` at package level.
  - A second handle of the same name in the same function gets `.2`.
- *Site id.* `<function or pkg>#<Symbol>@<hash>`:
  - `<Symbol>` is the site's first legacy symbol without the `nats.` prefix (`JetStreamManager.AccountInfo`).
  - `<hash>` is the first 6 hex digits of the SHA-256 of the site's source text, with whitespace runs collapsed, so gofmt realignment does not change it.
  - Sites of identical text in one function are numbered `.2`, `.3` in source order.
- *Why a text hash, not an ordinal.* Ordinals shift when an earlier site with the same symbol migrates, and a site-scoped answer would then silently move to the next site. A text hash changes only when the site itself changes, which is when its answer should be asked again.
- *Other ids.* Step ids stay per-plan sequence numbers (`S0`…). The skill tells agents to quote component ids in commit messages.
- *Decisions file.* It goes to version 2, and a version 1 file is rejected with the new id formats in the message. Converting v1 files was rejected: position ids cannot be matched once the tree has changed, and the only v1 files come from the nats-surveyor trial.
- *Stale answers.* A stale `component:<id>` answer choosing `skip` makes `plan` and `apply` fail, naming the answer. A skipped component never migrates, so its id disappears only when its code changed (a renamed handle or function), and falling back to the module answer would migrate what the user chose to keep. Every other stale answer is reported as today. Most often it is a site-scoped answer whose site has already migrated, which is routine progress. Failing on every stale answer would make each such migration break the next plan.
- *Alternative:* keep position ids and warn louder about stale answers — rejected. It leaves the neighbor capture: a line inserted above two stacked subscribe calls moves the lower call onto the upper call's old position, and it inherits that call's answer with nothing reported.

**Enclosing functions and test-only components.**
- The site's function comes from its anchor's enclosing `FuncDecl`. A function literal inside a `FuncDecl` counts as that `FuncDecl`.
- `Component` gains `functions` (sorted, unique) and `test_only` (every site is in a `_test.go` file).
- The `component` pending decision carries `components` (ids) instead of a site count. Other patterns keep `sites`.

**Unmapped sites leave the steps.** A unit's step drops its unmapped sites, and a step left with no sites is not emitted. Unmapped sites keep an empty `step`, and the Markdown lists them in their own section after the components.

**Step previews in the plan, rendered by the Markdown.**
- `Step` gains `before` and `after` for the `add-handle`, `finish` and `component` steps: the full lines each edit touches, before and after the step, grouped per file, with context lines.
- They live in the JSON so that the Markdown still renders only the plan (the "Markdown mirrors JSON" scenario).
- `add-handle`'s preview therefore shows the placeholder lines.
- The go-get step is rendered once, in its own section, not again under "Sites outside components".

**A minimal `apply` that plans in-process.**
- *Selection.* `natsvet migrate apply` builds the plan exactly as `plan` does, with the same flags and `natsvet-migrate.json`, then selects its work:
  - without `-component`, the first machine step in plan order;
  - with `-component <id>`, that component's machine steps in order, up to its first step that is not a machine step.
- *Writing.* It writes each step's edits from the in-memory plan, checking each file's recorded hash against the bytes it is about to replace. That check guards only against a file changing between the load and the write. Because every invocation plans afresh, hashes never go stale between invocations, and no plan file exists to manage.
- *Why re-planning is enough.* That is possible only because of the resume decision above. After a step is written, the next invocation's plan starts where the tree now stands, including after a guided site the agent migrated by hand.
- *Stops.*
  - The go-get command step is printed, not run: it needs the module proxy and edits `go.mod`, which is the agent's and the user's call.
  - A guided, waiting or decision step ends a `-component` run, with the step and its sites named.
  - A step of a component whose `component` decision is unanswered (neither component nor module scope) is refused. Machine steps never contain undecided sites, but the `component` default of `migrate` is exactly the assumption the skill forbids an agent to make on the user's behalf.
- *Rollback.* After writing, `apply` loads the packages that contain the touched files, tests included, and type-checks them. On an error it writes back the original bytes it held in memory, and exits non-zero with the step id and the errors. A step that does not compile is a planner bug, and the agent then gets a refused step and a report, not a broken tree.
- *Dry run.* `-dry-run` prints a unified diff of the selected edits and writes nothing.
- *Output.* It names the step, the files, the sites' functions (for `go test -run`) and the next step. When nothing is left, it names the remaining guided, decision and unmapped sites.
- *Code.* The test applier's splice (`applyStep`) moves into the package and is shared by `apply` and the tests. The rest is flag parsing and the type-check.
- *Alternatives:*
  - An `apply` that reads a plan file and step ids, the test applier promoted as is — rejected. The agent would still manage plan files and re-plan itself, and the hash chain would still break on any hand edit in between.
  - Leaving `apply` to the next change — rejected in the grill. The applier each agent writes for itself was the least deterministic part of the trial, and after this change `apply` is small.
  - Running `go get`, go vet or tests inside `apply` — rejected. They need network access or take minutes, and the agent's loop already runs them with the user's permissions.

**Schema 2 and the skill.**
- `schemaVersion` becomes 2. `SKILL.md` is rewritten around `apply`:
  - answer the decisions with the user;
  - run `natsvet migrate apply ./...` for the next step, or `-component <id>` for a whole component;
  - migrate guided sites by hand to their templates, and plan again to see what is left;
  - gofmt freely, since steps are gofmt-stable;
  - never type an unmapped site;
  - test with `go test -run` over the functions `apply` names, and run the package's full tests at component boundaries;
  - commit a `component` step, or a component's steps, with its component id in the message.
- The skill no longer teaches splicing byte offsets. The JSON's edits and hashes stay the documented contract, in the plan's help, for other tools.

**Verification is by properties on the test applier (`apply_test.go`), on top of its per-step type-check:**
- *gofmt:* a file that was gofmt-clean before a step is gofmt-clean after it.
- *Resume:* after each `add-handle` step and after each hand-migration fixture, a copy is planned again with the same answers, and applying the new plan to the end must give the same files as the uninterrupted chain.
  - Testdata gains `migrate/guided`, a `PullSubscribe` component. Its hand-migrated text is a fixture the test writes between plans, as an agent would.
  - Per-step resume over the whole testdata plan would re-plan about 130 times, so it runs only in the env-gated corpus test.
- *Ids:* the test inserts lines at the top of a testdata file, plans again, and expects every id to be unchanged.
- *Neighbors:* two stacked subscribe calls, one answered by site id, keep their answers after the insertion.
- *`apply`:* a testdata component is migrated end to end by calling `Main([]string{"apply", ...})` repeatedly, with the guided fixture written by the test in between. It must reach the same files as the test applier's chain.
  - Each stop (the go-get step, a guided step, an unanswered `component` decision) has its own case.
  - Rollback is exercised through a test hook that corrupts one edit of the selected step. The test checks that every file is back to its original bytes.

## Risks / Trade-offs

- [Recognition misfires on hand-written code shaped like a sibling (a `jsNew` of the new type next to `js`, from the same connection)] → The only effect is that `finish` removes `js` and renames `jsNew`, which is the migration the plan was about to do anyway. The "Unrelated jetstream variable" scenario pins the case with no legacy counterpart.
- [An agent deletes a placeholder by hand] → The local becomes unused and the build fails at once, so the skill's build step surfaces it. The skill keeps saying not to remove them.
- [A composition bug emits edits that type-check but differ from the sequential batches] → The simulation's check compares composed edits with the sequential result on every step and fails the plan. The applier tests type-check every step.
- [gofmt output changes between Go releases] → The formatting batch is computed with the `go/format` of the Go that built natsvet, so a tool applying a plan's JSON edits after running a different gofmt sees a hash mismatch and has to plan again. `apply` plans afresh on every run, so it is unaffected.
- [A text-hash site id changes when an agent touches the site without migrating it] → The answer is reported stale, which is right: it was given for different code.
- [Merging remove and rename makes `finish` a larger diff] → It is one mechanical rename, and the step preview shows it.
- [`apply`'s type-check passes but go vet, `natsvet` or the tests would fail] → The type-check catches what the planner can get wrong mechanically. Behavior and vet findings are the agent loop's to check, and the skill keeps those steps after every `apply`.
- [`apply` runs while the user has unrelated uncommitted edits] → It plans from the files as they are, so those edits are neither overwritten nor reverted. It writes only the step's files, and a rollback restores exactly the bytes it read.
- [Schema 2 invalidates the nats-surveyor trial's plan and answers] → That trial is the only known user, and the rejection message names the new id formats.

## Migration Plan

- The binary emits schema 2 only, and reads decisions files of version 2 only. Plans are unreleased (no tag), so there is no compatibility window.
- `docs/design.md` §3.5 describes the resumable plan, the `finish` and `component` steps and `apply`. The next change in the migration family becomes threading handles through function results only.
- `README` shows `natsvet migrate apply` and names schema 2.
- At archive, the main spec's Purpose ("it plans; it never edits code") is revised to cover `apply`.
- Rollback is reverting the change: schema 1 plans come back with it.

## Open Questions

- Length of the site-id hash: 6 hex digits make a collision between two different sites in one function about one in 16 million. This can be lengthened later without touching the specs, since the format is `@<hash>`.
