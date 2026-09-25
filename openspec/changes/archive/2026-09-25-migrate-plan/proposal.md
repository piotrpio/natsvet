## Why

Moving a codebase off the legacy JetStream API (`nats.JetStreamContext`, `nats.KeyValue`, `nats.ObjectStore`) onto the `jetstream` package is the migration nats.go users ask about most, and it cannot be fully automatic: the new API is context-first, object-based (streams and consumers are handles), pull-oriented (`PushConsumer` offers only `Consume`), and never acks for the user. It is also too large and too uniform to hand to an agent unaided — the corpus has 207 legacy uses in go-choria, 227 in eventing-natss, 8421 in nats-server's tests — and an unaided agent misses sites, invents option names, and drops the auto-ack. `legacyjs` already finds every site; nothing yet says what each site becomes, which ones need a human, and in what order to change them so the tree compiles after every step. `docs/design.md` §8 item 6 set this aside for "a separate `natsvet migrate` driver with a report"; nats.go's own `jetstream/MIGRATION.md` is the human-readable half of the mapping this change makes machine-usable.

## What Changes

- New subcommand `natsvet migrate plan [flags] <packages>`: loads the named packages (tests included) with `go/packages` as one program, and writes a migration plan. It never edits code. `natsvet migrate skill` prints the agent skill embedded in the binary.
- Every legacy use is a **site**, classified as:
  - **mechanical** — one correct replacement, given as exact text (`js.AddConsumer("S", &cfg)` → `jsNew.CreateConsumer(ctx, "S", cfg)`; subscribe options folded into `ConsumerConfig` fields; the stream of an unbound subscription resolved with `StreamNameBySubject`, the same request legacy `Subscribe` sends; enum, error and config-type renames; `ctx` chosen by a fixed rule);
  - **guided** — the transformation is determined but its text depends on code the planner does not rewrite (a `Fetch` loop becomes `Messages()` + `Error()`);
  - **decision** — needs the user: push or pull for a subscription, how a handler acks once auto-ack is gone, push-only options on a pull target, a channel subscription's `MaxAckPending`, handlers shared with core subscriptions, and whether a component is migrated or skipped. Each decision names its options, a recommended default and the reason. **Defaults preserve legacy behavior**, with one exception: subscriptions default to pull consumers, because that is the point of migrating;
  - **unmapped** — no `jetstream` equivalent; reported, never guessed.
- **Follow-ups**: improvements the migration makes possible but does not require, listed after the steps — chiefly replacing a runtime stream lookup with a static name, naming the candidate stream when the module's own code creates one whose subjects cover the subscription. Explicit durables are kept (the new API never deletes a consumer on `Stop`; legacy `Unsubscribe` deleted durables the library had created), with a note on the site; consumers without a durable name are removed by the server after their inactive threshold, so no `DeleteConsumer` is ever proposed.
- Sites are grouped into **components**: the declarations a legacy handle flows through, across packages. The plan orders the work as a **dual-handle migration**: create a `jetstream` handle (`<name>New`) next to each legacy one on the same `*nats.Conn`, add it next to every declaration that carries the legacy handle, migrate sites one at a time, delete the legacy handle, rename the new one back. Every step compiles. A package-local component with no open decisions is marked as safe to apply in one commit. A component that reaches a package outside the loaded set, or test code excluded with `-tests=false`, keeps its legacy handle.
- When the module requires a nats.go older than the version the mapping table was verified against, the plan starts with a mechanical step 0, `go get github.com/nats-io/nats.go@latest` (v1 only adds API, so every target exists afterwards).
- Decisions are asked **per pattern, not per site** ("12 `Subscribe` sites: pull or push?") and recorded in `natsvet-migrate.json` at the module root, read automatically, committed with the code it shaped, and edited by the user or the agent; `plan` never writes it. Re-running the planner with the same answers gives a byte-identical plan.
- Output: a versioned JSON plan and a Markdown guide rendered from it by template, not by a model. Every site and step carries human-readable `before`/`after` text **and** machine edits (file, byte offsets, new text) against a recorded hash of each file, so a later `apply` can refuse stale files; each site cites the matching section of nats.go's `jetstream/MIGRATION.md`. The plan header tells agents to run `natsvet migrate skill`, whose loop is: answer decisions with the user, apply one step, `go build`, `natsvet ./...` (whose `handle`, `msgloop`, `pubasync` and `consumerconfig` rules catch the typical translation mistakes), tests, commit, re-plan.
- A legacy → `jetstream` mapping table covering every one of the 324 symbols in the generated legacy table — JetStream, KeyValue and object store — with a test that every legacy symbol is covered, every target exists in the pinned `jetstream` package, and every row of `MIGRATION.md`'s mapping tables agrees.
- Behavior facts the classification rests on, re-verified against nats.go v1.53.1 and nats-server v2.14.7 and corrected where earlier notes were wrong: a context without a deadline gets `jetstream`'s 5s default timeout, equal to the legacy default wait, so `context.Background()` preserves behavior; an empty `Fetch` is not an error; legacy `AddConsumer` is create-only (`CreateConsumer`, not `CreateOrUpdateConsumer`); stream purge, info and message calls need a `Stream` handle, which costs a `STREAM.INFO` request and permission.
- Proof: a test applies the plan to the testdata module step by step and type-checks after each step; an environment-gated test does the same on natscli from the corpus cache. go-choria and eventing-natss are planned and their components reviewed, without applying.
- `docs/design.md`: §3.5 names the migration family and this change, §6 records it, §8 item 6 is closed.

## Capabilities

### New Capabilities

- `migration-planner`: the `natsvet migrate plan` and `natsvet migrate skill` commands — inventory, site classification, follow-ups, components and step order, the nats.go version step, the decisions file, JSON plan with machine edits and Markdown guide, the mapping table and its consistency checks, and the embedded agent skill.

### Modified Capabilities

- `analyzer-framework`: the "one binary" requirement gains the `migrate` subcommand, which must not change any analyzer invocation mode.

## Non-goals

- Rewriting code in this change. No `SuggestedFix`, no `go fix` path. `natsvet migrate apply` is the planned next change; this change's plan format (machine edits, file hashes, ordered steps) and its test-only applier are built so that it can be promoted, and the applier is not exposed as a command until then.
- Modules outside the loaded package set. A legacy value handed to a third-party package blocks its component's removal step at that boundary; the planner does not reach into dependencies.
- Choosing for the user. Defaults are recommendations recorded in the plan; nothing is applied silently.
- An MCP server or other interactive interface. The JSON plan and the skill work with any agent, in CI and for humans; an interactive layer can sit on top later.
- Changes to nats.go (such as marking the legacy API `// Deprecated:`). That is the nats.go maintainers' decision, independent of this tool.

## Rule defaults

No analyzer is added or changed; `legacyjs` stays opt-in and remains the progress counter.

## Shared helpers

Reused from `internal/natsapi`: `LegacySymbol` and the generated legacy table, `IsPkg`, `Callee`, `IsMethod`, `ConstString`, `CompositeFields`. New: `internal/migrate` (loader, inventory, mapping table, classification, components, plan model, JSON and Markdown writers, embedded skill). Nothing new is promoted to `natsapi`: the planner is its only user.

## Impact

- `cmd/natsvet` dispatches `natsvet migrate …` before `multichecker.Main`, as it already does for `-version`; `go vet -vettool` and `go fix -fixtool` never pass `migrate` as the first argument, so every existing mode is unchanged.
- New package `internal/migrate`; `golang.org/x/tools/go/packages` is already a dependency (used by `internal/tablegen`). No new dependencies: the decisions file and the plan are JSON, the skill is embedded with `embed` (standard library).
- The golangci-lint plugin does not import `internal/migrate`.
- `docs/migrate/SKILL.md` (embedded in the binary) and the plan schema are new, versioned artifacts; the schema version is in every plan so an agent can refuse a plan it does not understand.
- Modules that adopt the planner gain a committed `natsvet-migrate.json`.
