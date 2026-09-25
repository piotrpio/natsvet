---
name: natsvet-migrate
description: Move a Go module off the legacy nats.go JetStream API (nats.JetStreamContext, nats.KeyValue, nats.ObjectStore) onto the jetstream package by following a natsvet migration plan, one compiling step at a time, with the user answering every decision.
---

# Migrating off the legacy JetStream API with natsvet

This skill understands plans with `schema_version: 1`. If a plan carries another version, stop and tell the user to install the natsvet release that matches it.

`natsvet migrate plan` reads the module and writes a plan; it never edits code. You apply the plan, one step at a time, and the module compiles after every step. The plan is exact where the answer is known, asks where it is not, and says so where there is no answer.

## 1. Plan

```sh
natsvet migrate plan ./... > /tmp/natsvet-plan.json
```

Add `-format markdown` for a guide the user can read. Add `-tests=false` to leave test files out (components that reach test code are then blocked). Plan only the packages you intend to migrate; the plan covers the named packages and nothing else.

Read `counts`, `pending_decisions`, `stale_answers`, `notes`, `steps`, `components`, `sites` and `follow_ups`.

## 2. Decisions: ask, never pick

Each entry of `pending_decisions` is a question for the user: its `pattern`, `scope`, the `options`, the `default` and the `reason` the default preserves behavior. Each site's `decisions` show the replacement every option leads to (`options[].after`), which helps the user choose.

- Ask the user every pending decision, with its default and reason. Never answer one yourself, not even with the default.
- Record the answers in `natsvet-migrate.json` at the module root:

  ```json
  {
    "version": 1,
    "answers": [
      {"pattern": "subscribe-target", "scope": "module", "choice": "pull"},
      {"pattern": "subscribe-target", "scope": "site:app/sub.go:41:14", "choice": "push"},
      {"pattern": "component", "scope": "component:legacytests/legacy_test.go:27:2", "choice": "skip"}
    ]
  }
  ```

  Scopes are `module`, `component:<id>` and `site:<id>`; the narrowest one wins. Ids are the `id` fields of the plan.
- Plan again. Answering one question can raise another (a push-only option is asked about only once pull is chosen), so repeat until nothing you can answer is pending.
- Commit `natsvet-migrate.json` with the first step that relies on its answers.
- If `stale_answers` is not empty, tell the user: those answers name a site or component that no longer exists.

Patterns: `subscribe-target` (pull, push or defer a legacy subscription), `ack` (ack after the handler returns, as legacy did; explicit acks; or none), `push-only-option` (drop an option pull consumers lack, or use push), `channel-max-ack-pending` (keep legacy's channel-capacity limit or not), `shared-handler` (split a handler shared with a core subscription, or adapt the message), `component` (migrate a component, or skip it and keep it on the legacy API).

## 3. Steps

Take the steps in order. Every step lists its `sites`; every site has a `summary`, `before`, and `after` (mechanical) or `template` and `facts` (guided), `notes` and a `reference` into nats.go's `jetstream/MIGRATION.md`.

- **Command step** (`command` set, step 0): run the command (`go get github.com/nats-io/nats.go@latest`), build, and plan again before anything else.
- **Machine step** (`machine: true`): for each file in `expect`, check that its SHA-256 matches; if one does not, the tree changed since the plan was made: plan again. Then apply the step's `edits`, each file's from the highest `start` down (`start` and `end` are byte offsets; replace `[start, end)` with `new`). Apply exactly these edits; do not retype them.
- **Guided step** (`machine: false`, no `waits_on`): the change is determined but depends on code around the site. Rewrite the code by hand to the site's `template`, keeping behavior; the `facts` are what you need (for example: an empty `Fetch` batch is not an error in the jetstream package, so legacy `nats.ErrTimeout` checks on it go away).
- **Waiting step** (`waits_on` set): it cannot apply until the listed sites are migrated. Do those first; after planning again it becomes a machine step.
- **Unmapped sites** have no jetstream counterpart; the site's notes say why. Tell the user; the code stays on the legacy API until they decide otherwise.

After every step:

1. `gofmt -w` the files it touched.
2. `go build ./...` and `go vet ./...`.
3. `natsvet ./...` (the default rules must report nothing new).
4. Run the tests of the packages it touched.
5. Commit, one step per commit, unless the component is marked `one_commit`: then its steps may share one commit.
6. Plan again. Offsets and hashes are valid only for the tree the plan was made from, so plan again after any edit you make by hand as well.

## 4. What the steps do

A component is a set of legacy handles that values flow between (a handle stored in a struct field, passed to a function, returned). Its steps keep the code compiling throughout:

1. **add-handle** creates a jetstream sibling named `<name>New` next to each legacy handle, declaration and assignment, from the same connection and with the same error handling. It adds `_ = js` lines that keep local handles used until the end; do not remove them by hand.
2. **site** steps rewrite each use onto the sibling.
3. **remove-legacy** deletes the legacy handles, their creation and the values threaded into them, and the `_ =` lines.
4. **rename** renames every `<name>New` back to `<name>`.

A component whose handle leaves the loaded packages is `blocked`: it gets no remove or rename step; tell the user where it is blocked. Removal and rename wait while guided or undecided sites still use the legacy handle.

## 5. Finish

The migration is done when the plan has no steps left, only skipped components and the unmapped sites the user chose to keep. Confirm with `natsvet -legacyjs.enable ./...`, which reports every remaining legacy identifier. Then offer the `follow_ups`: improvements the migration makes possible but does not need, such as naming a stream instead of looking it up at runtime with `StreamNameBySubject`, or checking a lister's error.
