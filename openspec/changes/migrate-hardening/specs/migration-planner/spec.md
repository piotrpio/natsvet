## ADDED Requirements

### Requirement: Plans resume from the trees their own steps produce
The planner SHALL recognize the state its own steps leave behind:
- a legacy handle with a sibling of the new type named `<name>New` declared next to it (a local defined after the legacy root and its error check, or a field or parameter right after the legacy one);
- the values threaded into the siblings;
- `_ = <handle>` and `_ = <sibling>` placeholder lines.

Hand edits that move a site onto the sibling are part of that state. For a component in that state, the planner SHALL NOT plan a second `add-handle` for handles that already have siblings, and SHALL NOT treat placeholder lines as uses that block the component. It SHALL plan the remaining site steps and the `finish` step against the tree as it is. Applying a plan's steps up to any step, planning again, and applying the new plan's machine steps SHALL produce the same files as applying the first plan's machine steps to the end.

#### Scenario: Plan again after add-handle
- **WHEN** the `add-handle` step of a component whose handle `js` is a local is applied, and the planner runs again
- **THEN** the new plan has no `add-handle` step for `js`: only the remaining site steps and the `finish` step. Applying them gives the same file as applying the first plan to the end.

#### Scenario: Plan again after a guided hand edit
- **WHEN** a component's machine steps are applied, then its guided `PullSubscribe` site is rewritten by hand onto `jsNew` as the site's template shows, and the planner runs again
- **THEN** the component is not blocked, and its `finish` step is a machine step. After it is applied, the module type-checks and `legacyjs` reports no use in the component.

#### Scenario: Placeholder does not block
- **WHEN** the only remaining use of a legacy handle `js` is the placeholder line `_ = js`
- **THEN** the component is not marked blocked, and its `finish` step deletes the line

#### Scenario: Unrelated jetstream variable
- **WHEN** a function declares `jsNew, err := jetstream.New(nc)`, and no legacy handle named `js` is declared next to it
- **THEN** the planner treats `jsNew` as ordinary jetstream code: no step renames or removes it

### Requirement: Steps keep gofmt-clean files gofmt-clean
When a file is gofmt-clean before a machine step, it SHALL be gofmt-clean after the step. Imports a step adds SHALL be inserted in sorted position within their group. Whitespace that gofmt would change around the step's edits, such as the alignment of struct fields or composite literal values, SHALL be part of the step's edits. A file that is not gofmt-clean before a step SHALL get no formatting edits.

#### Scenario: Standard-library import in order
- **WHEN** a step needs `context` in a file whose import block starts with `"bufio"`
- **THEN** `"context"` is inserted after `"bufio"`, and gofmt leaves the file unchanged

#### Scenario: Sibling struct field
- **WHEN** a component's handle is the field `js` of a struct whose field types are aligned
- **THEN** after `add-handle`, and again after `finish`, the struct's field types are aligned and gofmt leaves the file unchanged

#### Scenario: Unformatted file
- **WHEN** a file is not gofmt-clean before a step
- **THEN** the step's edits change only the text its sites and handles need

### Requirement: Sites name their enclosing function
Every site SHALL name the function or method that encloses it, as `<pkg>.<Func>` or `<pkg>.<Type>.<Method>`. `<pkg>` is the package's import path relative to the module path, or the package name for the module's root package. A site in a package-level declaration SHALL name no function. Every component SHALL list the functions of its sites, and whether all its sites are in test files, so that an agent can run only the tests a step touched with `go test -run`.

#### Scenario: Site in a test
- **WHEN** a site is in `func TestSurveyor_AccountJetStreamAssets(t *testing.T)` of package `surveyor`
- **THEN** the site's function is `surveyor.TestSurveyor_AccountJetStreamAssets`, and its component is marked test-only when all its sites are in test files

#### Scenario: Site in a method
- **WHEN** a site is in `func (w *Worker) Run(ctx context.Context)` of package `app`
- **THEN** the site's function is `app.Worker.Run`

#### Scenario: Package-level declaration
- **WHEN** a site is the type of a package-level `var js nats.JetStreamContext`
- **THEN** the site names no function

## MODIFIED Requirements

### Requirement: The nats.go version is raised first when it is older than the table's
The plan SHALL record the nats.go version its mapping table was verified against. When the loaded module requires an older nats.go, the plan SHALL begin with a command step 0, `go get github.com/nats-io/nats.go@<table version>`. It is pinned to the version the table and its behavior facts were verified against, so its result does not depend on when it runs, and it raises nats.go and its requirements no further than that. Later v1 releases only add API, so every target exists at that version. When the module requires the table's version or a newer one, the plan SHALL have no step 0. When it requires a newer one, the plan SHALL note that the behavior facts were verified on the table's version.

#### Scenario: Old nats.go
- **WHEN** the module requires nats.go v1.31.0 and the table was verified against v1.53.1
- **THEN** the plan's first step is `go get github.com/nats-io/nats.go@v1.53.1`

#### Scenario: Current nats.go
- **WHEN** the module requires the table's version
- **THEN** the plan has no step 0

#### Scenario: Newer nats.go
- **WHEN** the module requires nats.go v1.54.0 and the table was verified against v1.53.1
- **THEN** the plan has no step 0, and it notes that the behavior facts were verified on v1.53.1

### Requirement: Decisions are asked per pattern and recorded in natsvet-migrate.json
The plan SHALL group pending decisions by pattern and scope (module by default), each with its options and its default. A site pattern SHALL carry the number of sites it covers. The `component` pattern SHALL carry the ids of the components it covers, and each of those components in the plan names its handles, files and functions and whether it is test-only. When the sites of one pattern have different defaults, the most common default is asked at module scope and each other site at its own scope.

Answers SHALL be read from `natsvet-migrate.json` (version 2) at the module root when it exists, or from the file named by `-decisions`. Each answer is a pattern, a scope (`module`, `component:<id>` or `site:<id>`) and a choice, and the narrowest scope wins.

Ids SHALL NOT be positions:
- A component id names the declaration that holds its first handle, and the handle: `<pkg>.<Func>#<handle>` for a local variable or parameter, and `<pkg>.<Type>.<field>` for a struct field.
- A site id names its enclosing function (or `<pkg>` at package level), its legacy symbol, and a short hash of its source text: `<pkg>.<Func>#<Symbol>@<hash>`. An ordinal is added only to tell apart sites of identical text in one function.

An edit outside a site or component therefore leaves its id unchanged, and a site's id changes only when its own text does.

The planner SHALL NOT write the file. A site whose decisions are all answered is reclassified by the chosen option (mechanical or guided, with its replacement or template), and answered patterns no longer appear as pending. A component answered `skip` SHALL keep its sites out of the steps, be counted separately, and block the removal of any declaration it shares with a migrated component. An answer naming an unknown pattern, option or scope SHALL be an error, and so SHALL a file of another version, with a message naming the id formats. An answer whose scope no longer exists SHALL be reported. Given the same packages and the same answers, the plan SHALL be byte-identical, including the order of every list of facts and notes.

#### Scenario: One answer covers many sites
- **WHEN** twelve `Subscribe` sites are pending and `natsvet-migrate.json` answers `subscribe-target` with `pull` for the module
- **THEN** the plan no longer lists `subscribe-target` as pending, and each of the twelve sites carries the pull replacement

#### Scenario: Site override
- **WHEN** the file also answers `subscribe-target` with `push` for one site
- **THEN** that site carries the push replacement and the other eleven the pull one

#### Scenario: Skipped component
- **WHEN** the file answers `component` with `skip` for a component of legacy-API tests
- **THEN** its sites are counted as skipped and appear in no step

#### Scenario: Answer survives an edit above it
- **WHEN** the file answers `component` with `skip` for `component:app.Run#js`, and an import and a function are then added above `Run` in its file
- **THEN** the next plan still skips that component and reports no stale answer

#### Scenario: Neighboring sites keep their own answers
- **WHEN** two `Subscribe` calls on consecutive lines of one function have different text, the lower one is answered `push` by its site id, and a line is inserted above both
- **THEN** the lower call still carries the push replacement, and the upper one the module answer

#### Scenario: Component decision lists its components
- **WHEN** the `component` decision is pending for four components
- **THEN** the pending decision names the four component ids, and the Markdown question lists each one's files, functions, handles and whether it is test-only

#### Scenario: Unknown option
- **WHEN** the file answers `subscribe-target` with `poll`
- **THEN** the command exits non-zero naming the pattern and the option

#### Scenario: Decisions file of version 1
- **WHEN** `natsvet-migrate.json` has `"version": 1`
- **THEN** the command exits non-zero, naming version 2 and the component and site id formats

#### Scenario: Deterministic output
- **WHEN** the plan runs twice over the same packages with the same answers
- **THEN** the two outputs are byte-identical

#### Scenario: Facts from several uses
- **WHEN** a subscription is used in three places the planner does not rewrite, and a table-test struct has three fields of legacy enum types
- **THEN** repeated plans list the facts in the same order, source order, and name positions relative to the module root

### Requirement: Unmapped sites are reported, never guessed
A legacy symbol without a `jetstream` equivalent SHALL produce an unmapped site with the reason from the mapping table. The plan SHALL count unmapped sites separately and SHALL NOT list them in any step, since no step can migrate them.

#### Scenario: Legacy-only option
- **WHEN** code passes `nats.UseLegacyDurableConsumers()` to `nc.JetStream`
- **THEN** the site is unmapped with a reason, no replacement is proposed, and no step lists the site

### Requirement: Components and a dual-handle step order
The planner SHALL group sites into components. A component is a legacy handle's creation sites (roots) and every declaration the handle flows through (variables, struct fields, parameters, results), connected by assignment, call argument and return, across all loaded packages.

For each migrated component, the plan SHALL order steps so that the code compiles after each one:
1. `add-handle`: creates a `jetstream` handle named `<name>New` next to each legacy root, from the same connection. Adds a sibling declaration of the new type named `<name>New` next to each variable, struct field and parameter that carries the legacy handle. Keeps local handles used with `_ =` placeholder lines until the `finish` step.
2. `site` steps: migrate the sites onto the siblings, mechanical first, then guided, then decided.
3. `finish`: removes the legacy handles, their roots, the values threaded into them and the placeholders, and renames each `<name>New` to `<name>`, in one step.

While a component has guided sites, its `finish` step SHALL be marked as waiting on them.

A component SHALL be marked safe to apply in one commit when it is package-local, has no guided sites and no pending decisions, and all its handles can get siblings. Such a component SHALL get a single `component` step, which carries the combined edits of the sequence above, and its sites SHALL show their final text.

When a legacy value is passed to a package outside the loaded set, or reaches test code while `-tests=false`, the component SHALL be marked blocked at that point, and its `finish` step SHALL be omitted.

Some handles cannot be threaded: a handle returned from a function, received from a call the planner does not rewrite (a helper, a type assertion), or a parameter fed something other than a handle variable. Such a handle SHALL get no sibling: its sites are guided, and its component's `finish` step waits on them.

Every step SHALL type-check wherever the component's declarations sit, including in a package with further files and an internal test file.

#### Scenario: Handle stored in a struct across packages
- **WHEN** `package app` creates `nc.JetStream()`, stores it in `Service.js`, and `package worker` calls `svc.js.Publish(...)`
- **THEN** one component spans both packages, and its steps include adding a `jsNew jetstream.JetStream` field next to `Service.js`

#### Scenario: Independent handles
- **WHEN** two functions each create their own `nc.JetStream()` and never share it
- **THEN** the plan has two components

#### Scenario: One-commit hint
- **WHEN** a component lives in one package, and none of its sites has a pending decision or is guided
- **THEN** the plan marks it safe to apply in one commit and gives it one `component` step. After that step the module type-checks, the handle is a `jetstream.JetStream` under its legacy name, no placeholder is left, and each site's `after` shows the final text (`js, err := jetstream.New(nc)`, not a sibling).

#### Scenario: Guided sites hold the legacy handle
- **WHEN** a component's only remaining legacy use is a guided `Fetch` loop
- **THEN** its `finish` step is marked as waiting on that site

#### Scenario: Handle returned from a helper
- **WHEN** code calls `kv, err := fw.KV(ctx, "config")`, a function that returns `nats.KeyValue`, and then `kv.Get("a")`
- **THEN** `kv` gets no sibling, the `Get` site is guided with the reason, and the component's `finish` step waits on it

#### Scenario: External boundary
- **WHEN** a legacy handle is passed to a function of a module outside the loaded packages
- **THEN** the component is marked blocked at that call, and it has no `finish` step

#### Scenario: Handle in a package with more files
- **WHEN** a component that spans several packages declares its handle in `handle.go`, and its package also has `other.go` and an internal `other_test.go`
- **THEN** every step type-checks, and the `finish` step deletes the placeholder lines in `handle.go`

### Requirement: The JSON plan is a versioned contract with machine edits, and the Markdown guide is rendered from it
The JSON plan SHALL carry, all in a stable order:
- a schema version (2);
- the nats.go version the table was verified against;
- the loaded packages;
- a SHA-256 hash of every file it edits;
- counts: sites by class, skipped sites, legacy identifiers;
- components with their steps, sites and follow-ups;
- pending decisions.

Every site with a replacement SHALL carry human-readable `before` and `after` text. Every machine step SHALL carry machine edits, each a file, a start and end byte offset, and the new text. A step's offsets SHALL be against each file as it stands after all earlier steps of the same plan. The step SHALL record the SHA-256 each file it edits must have before it is applied. Edits within a step SHALL NOT overlap. The plan header SHALL tell agents to read `natsvet migrate skill`.

The Markdown format SHALL be rendered from the same plan by template, in this order:
1. a summary;
2. the pending decisions as questions with their defaults, the `component` question listing its components;
3. each component's steps;
4. the unmapped sites;
5. the follow-ups.

Each step SHALL be shown once, with the before and after of its sites. The `add-handle` step's after SHALL include the placeholder lines. A step without sites of its own (`finish`) SHALL show the before and after of the lines its edits change.

The skill SHALL name the schema version it understands, and SHALL describe the agent loop:
1. Answer the pending decisions with the user, and record them in `natsvet-migrate.json`.
2. Apply the steps of one plan in order, since each step expects the tree the previous one leaves.
3. Run gofmt and build, run `natsvet ./...`, and run the tests of the functions the step's sites name.
4. Commit.
5. Plan again after a hand edit, when a step's recorded hash does not match, or between components.

#### Scenario: Schema version in both
- **WHEN** a plan is produced
- **THEN** its schema version is 2 and equals the version the embedded skill names

#### Scenario: Stale file
- **WHEN** a file changes after the plan was produced
- **THEN** its current hash differs from the hash the next step records for it, so tooling that applies the edits can refuse that file

#### Scenario: Steps apply in order
- **WHEN** the steps of a plan are applied in order, each checking its recorded hashes
- **THEN** every recorded hash matches, including the `finish` step that renames identifiers inserted by earlier steps

#### Scenario: Markdown mirrors JSON
- **WHEN** the same plan is written in both formats
- **THEN** every site, pending decision, step and follow-up in the JSON appears in the Markdown exactly once, and nothing else does

#### Scenario: Finish step preview
- **WHEN** a plan has a `finish` step for a local handle `js`
- **THEN** the Markdown shows its before with `js, err := nc.JetStream()` and the placeholder lines, and its after with `js, err := jetstream.New(nc)` and no placeholders

#### Scenario: Reference to the upstream guide
- **WHEN** a site replaces `js.Subscribe`
- **THEN** it references `MIGRATION.md`'s section on replacing `js.Subscribe()`
