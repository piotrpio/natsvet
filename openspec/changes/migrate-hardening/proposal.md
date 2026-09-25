## Why

The first agent-driven run of `natsvet migrate` on a production repository (nats-surveyor, 2026-09-25: 37 legacy uses, 4 components, all test code) needed a hand-written applier, workarounds for three bugs, one hand edit and two wrong turns to land about 20 changed lines. Reproducing the report against the source showed the flow is worse than it looked. Following the shipped skill word for word breaks the build at step 2. A component with a guided site can never be finished: re-planning after the hand edit, which is required, emits a second `add-handle` and marks the component blocked by the planner's own `_ = js` placeholder. `migrate-apply` would automate a plan that cannot be resumed, so the plan has to be made resumable first.

## What Changes

- **Plans resume from any tree the plan's own steps leave behind.** The planner recognizes a legacy handle's `<name>New` sibling and its `_ =` placeholder lines in the loaded code, and continues from there: no second `add-handle`, placeholders are not uses that block a component, and the remaining steps finish the migration. Re-planning becomes safe at every point: after any step, after a guided hand edit, and after a hash mismatch.
- **Removal and rename become one step (`finish`)**, so no reachable intermediate tree has a sibling without its legacy handle. **BREAKING** for the step kinds `remove-legacy` and `rename`.
- **Steps are gofmt-stable.** When a file is gofmt-clean before a step, it is gofmt-clean after it: imports are inserted in sorted order, and the whitespace gofmt would change (struct field alignment around an inserted sibling field, for example) is part of the step's edits. Running gofmt between steps no longer breaks the hash chain.
- **Components safe to apply in one commit collapse into one step.** A component that is package-local, fully mechanical and free of pending decisions gets a single `component` step with the combined edits and sites showing their final text; the sibling sequence is kept for components that need it (guided or decided sites, several packages).
- **Step 0 pins the verified version**: `go get github.com/nats-io/nats.go@<table version>` instead of `@latest`, still only when the module requires an older nats.go.
- **Stable ids.** Component ids name the declaration and the handle (`surveyor.TestSurveyor_AccountJetStreamAssets#js`, `app.Service.js`); site ids name the enclosing function, the symbol and a hash of the site's source text. Answers in `natsvet-migrate.json` then survive edits elsewhere in the file. Position ids go stale on every edit above them, and a stale answer falls back to the module answer, so a `skip` stops protecting its component. Worse, a site can move onto a neighbor's old position and silently inherit its answer. Every site keeps its `position`. **BREAKING** for id formats and the decisions file version.
- **Sites carry their enclosing function**, so an agent can run `go test -run` on the tests a step touched instead of the whole package.
- **Plan shape fixes**:
  - Unmapped sites no longer appear in `steps`, where they looked exactly like guided steps.
  - The `component` decision lists the components it covers (files, functions, handles, test-only) instead of a count mislabeled `sites`.
  - The Markdown renders each step's own before and after instead of repeating the `add-handle` snippet under `finish`, shows the placeholder lines `add-handle` inserts, and prints the go-get step once.
- **Schema version 2**, with the skill rewritten to match how the plan actually works:
  - Steps of one plan chain: each step expects the tree the previous one leaves.
  - Re-plan after a hand edit, a hash mismatch, or between components.
  - gofmt is safe between steps.
  - Unmapped sites are reported, not rewritten.
  - Test at component boundaries, using the sites' functions to choose the tests.
- **Already fixed in the working tree** (from the same investigation, each with a regression test):
  - A handle was recorded in the last-sorted file of its package, not the file that declares it (`collectHandles` ranged over the whole package's `Defs` once per file). As a result, `remove-legacy` left the placeholders behind in any multi-file package, and parameters in earlier files were classified as locals.
  - Facts on units and on subscription uses depended on map order, which broke the spec's byte-identical output on eventing-natss (6 runs, 5 different plans).
  - One fact named an absolute path.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `migration-planner`:
  - Changed requirements: the nats.go version step (pinned), components and step order (`finish` step, one-step components), decisions (id formats, the `component` decision's listing), unmapped sites (not steps), and the JSON plan and Markdown contract (schema 2, step previews, the skill's loop).
  - New requirements: resuming from intermediate trees, gofmt-stable steps, and enclosing functions on sites.

## Non-goals

- `natsvet migrate apply`. It stays the next change (`docs/design.md` §3.5), together with threading handles through function results. This change makes the plan something an applier can follow and resume; exposing the applier is separate.
- Resuming from arbitrary hand-written partial migrations. The planner recognizes the trees its own steps produce (a sibling named `<name>New` of the new type, declared next to its legacy handle, and `_ =` placeholders), plus any hand edits that migrate sites onto the sibling. A sibling under another name is not recognized.
- Per-symbol minimum nats.go versions, so that step 0 could be skipped when only old API is used. The plan's behavior facts were verified on the table's version, not only its API.
- Formatting code the planner did not touch. A file that is not gofmt-clean before a step gets no formatting edits.
- Accepting position ids in the decisions file. Plans are unreleased (no tag), so version 1 files are rejected with a message naming the new id formats.

## Rule defaults

No analyzer is added or changed; `legacyjs` stays opt-in.

## Shared helpers

No `internal/natsapi` helper is added or changed; everything is in `internal/migrate`.

## Impact

- `internal/migrate`:
  - Loader: sibling and placeholder recognition.
  - Components: ids and the `finish` step.
  - Steps: resume seeding, an edit-composition helper shared by gofmt-stable steps and one-step components, pinned step 0.
  - Imports: sorted insertion.
  - Output: plan model (schema 2: `function` on sites, `test_only` on components, `components` on the component decision), Markdown template, `SKILL.md`.
  - Test applier: resume, gofmt and one-step properties.
- New testdata packages under `testdata/migrate/`: a component with a guided site and its hand-migrated fixture, a struct-field handle, a file with standard-library imports that sort after `context`, and neighboring subscribe sites for id stability. `multifile` and `order` were added with the fixes already made.
- `natsvet-migrate.json` goes to version 2. Plans and decision files made with the current binary must be regenerated. Only the nats-surveyor trial used them.
- `docs/design.md` §3.5 describes the resumable plan; `README`'s migrate section names schema 2.
