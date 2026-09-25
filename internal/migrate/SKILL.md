---
name: natsvet-migrate
description: Move a Go module off the legacy nats.go JetStream API (nats.JetStreamContext, nats.KeyValue, nats.ObjectStore) onto the jetstream package with natsvet's migration plan and its apply command, one compiling step at a time, with the user answering every decision.
---

# Migrating off the legacy JetStream API with natsvet

This skill understands plans with `schema_version: 2`. If a plan carries another version, stop and tell the user to install the natsvet release that matches it.

`natsvet migrate plan` reads the module and writes a plan; it never edits code. `natsvet migrate apply` applies the plan's machine steps, one at a time, and the module compiles after every step. The plan is exact where the answer is known, asks where it is not, and says so where there is no answer. Every run plans afresh from the code as it is, so you can plan and apply again at any point: after a step, after a hand edit, after gofmt.

## 1. Plan

```sh
natsvet migrate plan ./... > "$TMPDIR/natsvet-plan.json"
```

Write the plan outside the module. Add `-format markdown` for a guide the user can read. Add `-tests=false` to leave test files out (components that reach test code are then blocked). Plan only the packages you intend to migrate; the plan covers the named packages and nothing else, and `apply` takes the same packages.

Read `counts`, `pending_decisions`, `stale_answers`, `notes`, `steps`, `components`, `sites` and `follow_ups`.

Before changing anything, run `natsvet ./...` and keep its output: it is the baseline the checks in section 3 compare against.

## 2. Decisions: ask, never pick

Each entry of `pending_decisions` is a question for the user: its `pattern`, `scope`, the `options`, the `default` and the `reason` the default preserves behavior. Each site's `decisions` show the replacement every option leads to (`options[].after`), which helps the user choose. The `component` question lists the `components` it covers; show the user each one's files, functions, handles and whether it is test code only (`test_only`), from `components`.

- Ask the user every pending decision, with its default and reason. Never answer one yourself, not even with the default. `apply` refuses to touch a component whose `component` question is unanswered.
- Record the answers in `natsvet-migrate.json` at the module root:

  ```json
  {
    "version": 2,
    "answers": [
      {"pattern": "component", "scope": "module", "choice": "migrate"},
      {"pattern": "component", "scope": "component:legacytests.TestLegacyAddStream#js", "choice": "skip"},
      {"pattern": "subscribe-target", "scope": "module", "choice": "pull"},
      {"pattern": "subscribe-target", "scope": "site:app.Worker.Run#JetStreamContext.Subscribe@3fa2c1", "choice": "push"}
    ]
  }
  ```

  Scopes are `module`, `component:<id>` and `site:<id>`; the narrowest one wins. Ids are the `id` fields of the plan. They name code, not positions: a component by the declaration of its handle (`<pkg>.<Func>#<handle>`, `<pkg>.<Type>.<field>`), a site by its function, symbol and a hash of its text. Edits elsewhere leave them alone.
- Plan again. Answering one question can raise another (a push-only option is asked about only once pull is chosen), so repeat until nothing you can answer is pending.
- Commit `natsvet-migrate.json` with the first step that relies on its answers.
- `stale_answers` are answers whose site or component no longer exists. A site answer goes stale once its site is migrated; remove it. A stale `skip` makes `plan` and `apply` fail, because the skipped code changed (a renamed function or handle): tell the user, and record the skip under the component's new id.

Patterns: `subscribe-target` (pull, push or defer a legacy subscription), `ack` (ack after the handler returns, as legacy did; explicit acks; or none), `push-only-option` (drop an option pull consumers lack, or use push), `channel-max-ack-pending` (keep legacy's channel-capacity limit or not), `shared-handler` (split a handler shared with a core subscription, or adapt the message), `component` (migrate a component, or skip it and keep it on the legacy API).

## 3. Apply

```sh
natsvet migrate apply ./...                              # the next machine step
natsvet migrate apply -component 'app.Worker.Run#js' ./...  # a component's machine steps, up to its first one that is not
natsvet migrate apply -dry-run ./...                     # the same, as a unified diff; writes nothing
```

Quote ids on the command line: they contain `#`.

`apply` plans, writes the step, type-checks the packages it touched, and restores the files if they do not compile (exit 1: report it to the user as a natsvet bug). It prints the steps it applied with their component, their files and the functions of their sites, and the next step. Step ids (`S0`, `S1`) number the current plan only: every `apply` plans again, so the next step is often `S0` again. Refer to work by component id. Exit 3 means the next step needs a person, and nothing was written:

- **Step 0** (`go-get`): the module's nats.go is older than the one the plan was verified against. Tell the user, run the printed `go get github.com/nats-io/nats.go@<version>`, build, and apply again.
- **Guided step** (`machine: false`, sites with a `template`): the change is determined but depends on code around the site. Rewrite the code by hand to the site's `template`, keeping behavior; the `facts` are what you need (for example: an empty `Fetch` batch is not an error in the jetstream package, so legacy `nats.ErrTimeout` checks on it go away). Use the component's `<name>New` sibling. Then apply again: the plan picks up from your edit.
- **Waiting step** (`waits_on` set): it applies once the listed sites are migrated.

Do not splice the plan's `edits` by hand, and do not edit or delete the `_ = x` lines `add-handle` writes: the `finish` step removes them. Unmapped sites have no step: they have no jetstream counterpart, and the site's notes say why. Tell the user; the code stays on the legacy API until they decide otherwise.

After every `apply`:

1. `gofmt -l` the touched files (the steps keep gofmt-clean files clean), `go build ./...` and `go vet ./...`.
2. `natsvet ./...`: the default rules must report nothing beyond the baseline.
3. Tests. After a step that leaves its component unfinished (`add-handle`, `site`), run the functions `apply` printed that are tests (`go test -run '^(TestA|TestB)$' ./pkg`); a printed function that is not a test (a helper) is exercised by the tests that call it, so run the packages that use it. When a component is done (after a `component` or `finish` step), run the tests of every package that imports the touched ones, or `go test ./...`.
4. Commit, naming the component id `apply` printed. A component marked `one_commit` has one `component` step; the steps of another component may be committed one by one or together once it is done.

## 4. What the steps do

A component is a set of legacy handles that values flow between (a handle stored in a struct field, passed to a function). Its steps keep the code compiling throughout:

1. **add-handle** creates a jetstream sibling named `<name>New` next to each legacy handle, declaration and assignment, from the same connection and with the same error handling. It adds `_ = js` lines that keep local handles used until the end.
2. **site** steps rewrite each use onto the sibling.
3. **finish** deletes the legacy handles, their creation, the values threaded into them and the `_ =` lines, and renames every `<name>New` to `<name>`.

A component that lives in one package and has only mechanical sites gets one **component** step instead, which does all of that at once. A component whose handle leaves the loaded packages is `blocked`: it has no finish step; tell the user where it is blocked. `finish` waits while guided or undecided sites still use the legacy handle.

## 5. Finish

The migration is done when `apply` says there is nothing left to apply and the plan holds only skipped components and the unmapped sites the user chose to keep. Confirm with `natsvet -legacyjs.enable ./...`, which reports every remaining legacy identifier; like `go vet`, it exits 3 when it reports anything, which is expected while unmapped or skipped sites remain. Keep `natsvet-migrate.json` while skipped components remain on the legacy API, since later plans need their answers; otherwise ask the user whether to delete it. Then offer the `follow_ups`: improvements the migration makes possible but does not need, such as naming a stream instead of looking it up at runtime with `StreamNameBySubject`, or checking a lister's error.

Other tools can apply the plan without `apply`: each machine step's `edits` are byte offsets against the files as the plan's earlier steps leave them, and `expect` holds the SHA-256 each file must have first.
