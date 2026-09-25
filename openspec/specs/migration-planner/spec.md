# migration-planner Specification

## Purpose

`natsvet migrate plan` tells a team, or the agent working for them, exactly how to move a module off the legacy JetStream API (`nats.JetStreamContext`, `nats.KeyValue`, `nats.ObjectStore`) onto the `jetstream` package: every legacy site, what it becomes, which choices need a human, and an order of steps in which the code compiles after each one. Planning never edits code, and a plan can be made again at any point of the migration. `natsvet migrate apply` writes the plan's next machine step, type-checks it, and restores the files when it does not compile.

## Requirements

### Requirement: The plan command loads the module as one program and never edits code
`natsvet migrate plan [-decisions <file>] [-format json|markdown] [-tests=<bool>] <packages>` SHALL load the named packages, including their test files unless `-tests=false`, with full type information as a single program, and SHALL write the plan to standard output in the requested format (JSON by default). It SHALL NOT modify any file. When a package fails to load or type-check, it SHALL exit non-zero and list the errors, since a plan over broken types would misclassify sites. `natsvet migrate skill` SHALL print the agent skill embedded in the binary.

#### Scenario: Plan over a module with legacy use
- **WHEN** `natsvet migrate plan ./...` runs in a module that uses `nats.JetStreamContext`
- **THEN** it prints a JSON plan listing the sites and exits 0, and no file in the module changes

#### Scenario: No legacy use
- **WHEN** it runs over packages that use only core NATS and the `jetstream` package
- **THEN** it prints a plan with zero sites and no components, and exits 0

#### Scenario: Type errors
- **WHEN** one of the named packages does not type-check
- **THEN** it exits non-zero, prints the type errors, and prints no plan

#### Scenario: Skill
- **WHEN** `natsvet migrate skill` runs
- **THEN** it prints the skill, and the skill names the same schema version the binary's plans carry

### Requirement: Every legacy use belongs to exactly one site
The planner SHALL find every identifier whose object is a legacy JetStream symbol, as `legacyjs` does (`natsapi.LegacySymbol`), and every use of a `*nats.Msg` parameter inside a handler passed to a legacy subscribe call, and SHALL assign each to exactly one site: the smallest expression or declaration that is rewritten as a unit (a call with its arguments and options, a composite literal, a type in a declaration, a field access). The number of legacy identifiers in the plan SHALL equal the number of `legacyjs` diagnostics for the same packages. Constants of legacy types (`nats.AckExplicitPolicy`) and error values the jetstream package declares under the same name (`nats.ErrKeyNotFound`) are not legacy identifiers, but the planner SHALL rewrite them with the values they belong to: a constant with the values it is used with, an error value with the call whose error it is compared against; they are not counted as legacy identifiers.

#### Scenario: Agreement with legacyjs
- **WHEN** the plan and `natsvet -legacyjs.enable` run over the same packages
- **THEN** the plan's legacy identifier count equals the number of `legacyjs` diagnostics

#### Scenario: One call, one site
- **WHEN** code calls `js.Subscribe("orders.new", h, nats.Durable("w"), nats.DeliverNew())`
- **THEN** the call, its options and its handler binding form one site, not four

#### Scenario: Error value follows its call
- **WHEN** code has `e, err := kv.Get(k); if errors.Is(err, nats.ErrKeyNotFound) { … }`
- **THEN** `nats.ErrKeyNotFound` becomes `jetstream.ErrKeyNotFound` in the same step as the `Get` call

#### Scenario: Core NATS is not a site
- **WHEN** code calls `nc.Subscribe("x", func(m *nats.Msg) { _ = m.Data })`
- **THEN** neither the call nor the handler's `m.Data` is a site

### Requirement: The mapping table covers every legacy symbol and agrees with the pinned nats.go
The planner SHALL classify every symbol of the generated legacy table — JetStream, KeyValue and object store — through one mapping table: a target in the `jetstream` package with its shape change (context position, pointer to value, option to field, handle acquisition), a guided transformation, a decision pattern, or unmapped with a reason. A test SHALL fail when a legacy symbol has no entry, when an entry names a `jetstream` symbol the pinned nats.go does not export, or when a legacy-to-new row in nats.go's `jetstream/MIGRATION.md` mapping tables disagrees with the table.

#### Scenario: nats.go adds a legacy symbol
- **WHEN** the pinned nats.go is bumped and the regenerated legacy table has a symbol the mapping table lacks
- **THEN** the test fails naming that symbol

#### Scenario: Target renamed upstream
- **WHEN** an entry maps to `jetstream.WithMsgID` and the pinned `jetstream` package no longer exports it
- **THEN** the test fails naming the entry

#### Scenario: Guide and table agree
- **WHEN** `MIGRATION.md` maps `nats.Durable("name")` to `Durable: "name"`
- **THEN** the table maps it the same way, and the test passes

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

### Requirement: The context argument follows a fixed rule
Every `jetstream` call that takes a `context.Context` SHALL receive, in order of preference: a `context.Context` variable or parameter in scope at the site; else the context passed to the legacy call through `nats.Context(ctx)`; else `context.Background()`. This mirrors nats.go: `jetstream` wraps a context without a deadline in its default timeout (`wrapContextWithoutDeadline`, `defaultAPITimeout` = 5s), which equals the legacy default wait (`defaultRequestWait` = 5s), so the replacement preserves the legacy timeout. A legacy handle option `nats.MaxWait(d)` SHALL map to `jetstream.WithDefaultTimeout(d)`; a per-call `nats.MaxWait(d)` SHALL make the site guided (a `context.WithTimeout` around the call).

#### Scenario: Context in scope
- **WHEN** `js.AddStream(&cfg)` is inside `func setup(ctx context.Context, js nats.JetStreamContext)`
- **THEN** the replacement is `jsNew.CreateStream(ctx, cfg)`

#### Scenario: Context from the legacy option
- **WHEN** code calls `js.AddStream(&cfg, nats.Context(reqCtx))` with no other context in scope
- **THEN** the replacement is `jsNew.CreateStream(reqCtx, cfg)`

#### Scenario: No context anywhere
- **WHEN** `js.DeleteStream("S")` is in a function with no context in scope and no `nats.Context` option
- **THEN** the replacement passes `context.Background()` and the site is still mechanical

### Requirement: Mechanical sites carry their exact replacement
A site whose every input is known SHALL be classified mechanical and carry its replacement, including at least: the handle (`nc.JetStream(opts...)` to `jetstream.New`, `NewWithDomain` for `nats.Domain`, `NewWithAPIPrefix` for `nats.APIPrefix`, `WithPublishAsyncErrHandler`, `WithPublishAsyncMaxPending`); stream and consumer management with pointer arguments made values (`AddStream` to `CreateStream`, `UpdateStream`, `DeleteStream`, `AddConsumer` to `CreateConsumer` — legacy `AddConsumer` returns the existing consumer for an identical config and fails with `ErrConsumerNameAlreadyInUse` otherwise, which is create semantics, not create-or-update — `UpdateConsumer`, `DeleteConsumer`, and `CreatePushConsumer`/`UpdatePushConsumer` for a config literal that sets `DeliverSubject`); a legacy `ConsumerConfig` literal without an `AckPolicy` given `AckPolicy: jetstream.AckNonePolicy`, since legacy's zero ack policy is none and jetstream's is explicit; calls that need a stream handle (`PurgeStream`, `StreamInfo`, `GetMsg`, `GetLastMsg`, `DeleteMsg` through `js.Stream(ctx, name)`); publish calls with their options (`nats.MsgId` to `jetstream.WithMsgID`, and the expectation options); KeyValue and object store handles and methods with the context added; config types, enum constants and error sentinels renamed, with `ConsumerConfig.Heartbeat` renamed `IdleHeartbeat`; message methods (`AckSync` to `DoubleAck(ctx)`); subscribe options folded into a `ConsumerConfig` literal together with `FilterSubject` set to the subscribe subject, as legacy subscribe does; and, for a subscription not bound to a stream (`nats.Bind`, `nats.BindStream`), the stream resolved at runtime with `StreamNameBySubject(ctx, subject)`, which is the request legacy `Subscribe` itself sends (`js.go`), so behavior and permissions are unchanged. A consumer a legacy subscription would create is created with `CreateOrUpdateConsumer`, as nats.go's `MIGRATION.md` does, and the site notes the one difference: legacy used an existing durable as it was when the options it set were compatible, while `CreateOrUpdateConsumer` applies the code's full configuration. A site that needs a stream handle SHALL carry a note that the new call sends a `STREAM.INFO` request and needs that permission.

#### Scenario: Consumer creation
- **WHEN** code calls `js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", Heartbeat: time.Second})`
- **THEN** the replacement is `jsNew.CreatePushConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", IdleHeartbeat: time.Second, AckPolicy: jetstream.AckNonePolicy})`

#### Scenario: Purge needs a stream handle
- **WHEN** code calls `js.PurgeStream("ORDERS")`
- **THEN** the replacement obtains `js.Stream(ctx, "ORDERS")` and calls `Purge(ctx)` on it, and the site notes the extra `STREAM.INFO` request and permission

#### Scenario: Options fold into a consumer config
- **WHEN** a bound pull subscription passes `nats.Durable("w"), nats.DeliverNew(), nats.MaxAckPending(100)` for subject `orders.new`
- **THEN** the consumer config in the replacement has `Durable: "w"`, `DeliverPolicy: jetstream.DeliverNewPolicy`, `MaxAckPending: 100` and `FilterSubject: "orders.new"`

#### Scenario: Unbound subscription resolves its stream at runtime
- **WHEN** code calls `js.PullSubscribe("orders.new", "w")` with no bind option
- **THEN** the replacement obtains the stream name with `jsNew.StreamNameBySubject(ctx, "orders.new")` before creating the consumer, and the site is still mechanical

#### Scenario: Not everything is mechanical
- **WHEN** a site depends on an unanswered decision, such as a `Subscribe` whose target is not chosen
- **THEN** the site is not classified mechanical and carries no replacement

### Requirement: Guided sites carry the transformation and the facts it needs
A site whose transformation is determined but whose text depends on surrounding code the planner does not rewrite SHALL be classified guided and carry a template and the facts the agent needs. At least: a legacy `Fetch`/`FetchBatch` loop becomes a `MessageBatch` ranged over `Messages()` followed by an `Error()` check, with the fact that an empty pull is not an error in the new API (timeouts and no-message statuses are filtered in `pull.go`), so legacy `nats.ErrTimeout` checks on the batch are removed, while `Consumer.Next` still returns `nats.ErrTimeout`; and a per-call `nats.MaxWait`.

#### Scenario: Fetch loop
- **WHEN** code has `msgs, err := sub.Fetch(10); if errors.Is(err, nats.ErrTimeout) { continue }; for _, m := range msgs { … }`
- **THEN** the site is guided, and its template ranges over `batch.Messages()`, checks `batch.Error()`, and states that the `ErrTimeout` branch is dropped because an empty batch has no error

#### Scenario: Plain call is not guided
- **WHEN** code calls `js.StreamNameBySubject("orders.new")`
- **THEN** the site is mechanical, not guided

### Requirement: Decision sites name the choice, the options, a behavior-preserving default and the reason
A site that needs the user SHALL be classified decision and name its pattern, its options, the recommended default and the reason, and SHALL list the replacement that follows each option. Every default SHALL preserve the legacy behavior, except `subscribe-target`, whose default is a pull consumer. The patterns SHALL be:
- `subscribe-target` for `Subscribe`, `QueueSubscribe`, `SubscribeSync`, `QueueSubscribeSync`, `ChanSubscribe`, `ChanQueueSubscribe`: a pull consumer (`Consume` for callbacks, `Messages` for sync and channel forms; for queue forms, instances share one pull consumer) as the default; a push consumer for `Subscribe` and `QueueSubscribe` only (`CreateOrUpdatePushConsumer` with a `DeliverSubject`, and `DeliverGroup` for queues), since `PushConsumer` offers only `Consume`; or defer the site. A subscription bound to an existing consumer with `nats.Bind` defaults to the push consumer (`js.PushConsumer(ctx, stream, consumer)`): legacy `Subscribe` and `QueueSubscribe` bind only push consumers (`processConsInfo` fails with `ErrPullSubscribeRequired` otherwise), so pull would fail at runtime until the consumer is recreated.
- `ack` for callback subscriptions that auto-acked (no `nats.ManualAck`, ack policy not `AckNone`, not ordered; legacy wraps the callback as `ocb(m); m.Ack()`): ack after the handler returns (the default), explicit acks chosen per handler path, or `AckNone`.
- `push-only-option`, only once pull is chosen, for `nats.DeliverSubject`, `nats.EnableFlowControl`, `nats.IdleHeartbeat` and `nats.RateLimit`: drop with a note (the default, since pull has no equivalent) or choose the push target instead.
- `channel-max-ack-pending` for `ChanSubscribe` without `nats.MaxAckPending`: legacy sets `MaxAckPending` to the channel capacity; keep that value explicitly (the default) or take the server default.
- `shared-handler` for a handler bound to both a core and a legacy JetStream subscription: split it in two (the default) or wrap it in an adapter.
- `component` for every component: `migrate` (the default) or `skip`.

#### Scenario: Unbound callback subscription
- **WHEN** code calls `js.Subscribe("orders.new", h, nats.Durable("w"))`
- **THEN** the site has decisions `subscribe-target` (default pull) and `ack` (default ack after the handler), a durable note, and a stream-name follow-up

#### Scenario: Bound push subscription
- **WHEN** code calls `js.QueueSubscribe("orders.new", "q", h, nats.Bind("ORDERS", "workers"), nats.ManualAck())`
- **THEN** the site's `subscribe-target` defaults to push through `js.PushConsumer(ctx, "ORDERS", "workers")`, and it has no `ack` decision

#### Scenario: Manual ack
- **WHEN** the same call also passes `nats.ManualAck()`
- **THEN** the site has no `ack` decision

#### Scenario: Bound pull subscription
- **WHEN** code calls `js.PullSubscribe("orders.new", "w", nats.BindStream("ORDERS"))`
- **THEN** the site has no `subscribe-target` decision and no stream-name follow-up

#### Scenario: Push-only option under push
- **WHEN** a subscription passes `nats.RateLimit(1024)` and `subscribe-target` is answered `push`
- **THEN** the site has no `push-only-option` decision and the rate limit becomes the consumer config's `RateLimit`

### Requirement: Durables are kept and consumers are never deleted
The planner SHALL NOT propose `DeleteConsumer` for any site. A subscription with an explicit durable (`nats.Durable`, or `Durable` in a config) SHALL carry a note that legacy `Unsubscribe`/`Drain` deleted a durable the library had created (`jsi.dc`) while the new API keeps it. A consumer without a durable name, including one with only a `Name` or a `Name` and `InactiveThreshold`, SHALL carry no note: the server treats it as not durable (`isDurable` is `Durable != ""`) and removes it after its inactive threshold (5s by default).

#### Scenario: Explicit durable
- **WHEN** code calls `js.Subscribe("orders.new", h, nats.Durable("w"))`
- **THEN** the site carries the durable note and no step deletes the consumer

#### Scenario: Named ephemeral
- **WHEN** code calls `js.Subscribe("orders.new", h, nats.ConsumerName("w"))`
- **THEN** the site carries no durable note

### Requirement: Follow-ups list improvements the migration makes possible
The plan SHALL list follow-ups after the steps: changes that are not needed to finish the migration. For each unbound subscription it SHALL include a follow-up to replace the runtime stream lookup with a static name; when the loaded code creates exactly one stream (a legacy or `jetstream` stream config literal with constant `Subjects`) whose subjects cover the subscription subject, the follow-up SHALL name that stream and the position where it is created.

#### Scenario: Stream created in the module
- **WHEN** the module calls `js.AddStream(&nats.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}})` and elsewhere `js.PullSubscribe("orders.new", "w")`
- **THEN** the follow-up for the subscription names `ORDERS` and the position of the `AddStream` call

#### Scenario: Stream created elsewhere
- **WHEN** no stream config in the loaded code covers `orders.new`
- **THEN** the follow-up asks for a static name without naming a candidate

### Requirement: Decisions are asked per pattern and recorded in natsvet-migrate.json
The plan SHALL group pending decisions by pattern and scope (module by default), each with its options and its default. A site pattern SHALL carry the number of sites it covers. The `component` pattern SHALL carry the ids of the components it covers, and each of those components in the plan names its handles, files and functions and whether it is test-only. When the sites of one pattern have different defaults, the most common default is asked at module scope and each other site at its own scope.

Answers SHALL be read from `natsvet-migrate.json` (version 2) at the module root when it exists, or from the file named by `-decisions`. Each answer is a pattern, a scope (`module`, `component:<id>` or `site:<id>`) and a choice, and the narrowest scope wins.

Ids SHALL NOT be positions:
- A component id names the declaration that holds its first handle, and the handle: `<pkg>.<Func>#<handle>` for a local variable or parameter, and `<pkg>.<Type>.<field>` for a struct field.
- A site id names its enclosing function (or `<pkg>` at package level), its legacy symbol, and a short hash of its source text: `<pkg>.<Func>#<Symbol>@<hash>`. An ordinal is added only to tell apart sites of identical text in one function.

An edit outside a site or component therefore leaves its id unchanged, and a site's id changes only when its own text does.

The planner SHALL NOT write the file. A site whose decisions are all answered is reclassified by the chosen option (mechanical or guided, with its replacement or template), and answered patterns no longer appear as pending. A component answered `skip` SHALL keep its sites out of the steps, be counted separately, and block the removal of any declaration it shares with a migrated component. An answer naming an unknown pattern, option or scope SHALL be an error, and so SHALL a file of another version, with a message naming the id formats. An answer whose scope no longer exists is stale:
- A stale `component` answer choosing `skip` SHALL be an error naming the answer. A skipped component never migrates, so its id can only disappear because its code changed, and falling back to the module answer would migrate code the user chose to keep.
- Every other stale answer SHALL be reported, and the plan continues. Typically it names a site that has already migrated.

Given the same packages and the same answers, the plan SHALL be byte-identical, including the order of every list of facts and notes.

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

#### Scenario: Stale skip
- **WHEN** the file answers `component` with `skip` for `component:app.Run#js`, and `Run` has since been renamed `Start`
- **THEN** the command exits non-zero, naming the answer, and prints no plan

#### Scenario: Stale site answer after migration
- **WHEN** a site answered `push` by its site id has been migrated
- **THEN** the answer is reported as stale, and the plan is produced

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

### Requirement: Plans resume from the trees their own steps produce
The planner SHALL recognize the state its own steps leave behind:
- a legacy handle with a sibling: a declaration named `<name>New`, of the new type, in the legacy handle's scope. For a local, it is declared later in the same function scope. For a field, it is anywhere in the same struct. For a parameter, it is anywhere in the same parameter list. The connection the sibling was created from is not checked.
- the values threaded into the siblings;
- `_ = <handle>` and `_ = <sibling>` placeholder lines.

Hand edits that move a site onto the sibling are part of that state. For a component in that state, the planner SHALL NOT plan a second `add-handle` for handles that already have siblings, and SHALL NOT treat placeholder lines as uses that block the component. It SHALL plan the remaining site steps and the `finish` step against the tree as it is. Applying a plan's steps up to any step, planning again, and applying the new plan's machine steps SHALL produce the same files as applying the first plan's machine steps to the end.

#### Scenario: Plan again after add-handle
- **WHEN** the `add-handle` step of a component whose handle `js` is a local is applied, and the planner runs again
- **THEN** the new plan has no `add-handle` step for `js`: only the remaining site steps and the `finish` step. Applying them gives the same file as applying the first plan to the end.

#### Scenario: Plan again after a guided hand edit
- **WHEN** a component's machine steps are applied, then its guided `PullSubscribe` site is rewritten by hand onto `jsNew` as the site's template shows, and the planner runs again
- **THEN** the component is not blocked, and its `finish` step is a machine step. After it is applied, the module type-checks and `legacyjs` reports no use in the component.

#### Scenario: Sibling after an inserted line
- **WHEN** a log statement has been added between the legacy root's error check and `jsNew, err := jetstream.New(nc)`, and a struct's `jsNew` field has been moved to the end of the struct
- **THEN** both siblings are recognized, and the plan has no `add-handle` step for them

#### Scenario: Placeholder does not block
- **WHEN** the only remaining use of a legacy handle `js` is the placeholder line `_ = js`
- **THEN** the component is not marked blocked, and its `finish` step deletes the line

#### Scenario: Unrelated jetstream variable
- **WHEN** a function declares `jsNew, err := jetstream.New(nc)`, and no legacy handle named `js` is declared in its scope
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
2. Apply the next machine step with `natsvet migrate apply`, rather than splicing the plan's edits by hand.
3. Rewrite guided sites by hand, following their templates.
4. Run gofmt and go vet, run `natsvet ./...`, and run the tests of the functions the step's sites name.
5. Commit.
6. Plan again after a hand edit, to see what is left.

The JSON edits and hashes remain the documented contract for any other tool that applies them.

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

### Requirement: The apply command applies one machine step and never leaves a broken tree
`natsvet migrate apply [-component <id>] [-dry-run] [-decisions <file>] [-tests=<bool>] <packages>` SHALL plan the named packages in-process, exactly as `plan` would with the same flags, and then apply machine steps to the files:
- Without `-component`, it applies the plan's next machine step.
- With `-component`, it applies that component's machine steps in order, up to its first step that is not a machine step.

It SHALL stop without writing, and say why, in these cases:
- When the next step is the go-get command step, it prints the command and applies nothing.
- When the next step of the component is guided, waits on sites, or carries decisions, it names the step and its sites.
- When the step belongs to a component whose `component` decision is not answered `migrate` at component or module scope, it exits non-zero naming the pending question, since the steps' `migrate` assumption is a default the user has not confirmed.

After writing, it SHALL load and type-check the packages that contain the touched files, tests included. On a type error, it SHALL restore every file it wrote byte for byte, and exit non-zero with the step id and the errors, since a step that does not compile is a planner bug.

On success, it SHALL print:
- the step id, its kind and its summary;
- the files it changed;
- the functions of the step's sites;
- what the next step is.

With `-dry-run`, it SHALL print the edits as a unified diff and write nothing. It SHALL NOT run `go get`, go vet, `natsvet` or tests, and SHALL NOT commit. `natsvet help migrate` SHALL list `apply` with its flags, next to `plan` and `skill`.

#### Scenario: Next machine step
- **WHEN** the first step of a plan is a machine `component` step, the `component` decision is answered `migrate` for the module, and `natsvet migrate apply ./...` runs
- **THEN** exactly that step's edits are written, the module type-checks, and the output names the step, its files, its sites' functions and the next step

#### Scenario: One component
- **WHEN** `natsvet migrate apply -component app.Run#js ./...` runs on a component whose steps are `add-handle`, two mechanical site steps, a guided site step and `finish`
- **THEN** `add-handle` and the two site steps are applied, and the output names the guided step and its site as the reason for stopping

#### Scenario: Dry run
- **WHEN** `natsvet migrate apply -dry-run ./...` runs
- **THEN** it prints a unified diff of the next machine step, and no file changes

#### Scenario: Step 0
- **WHEN** the module requires an older nats.go
- **THEN** apply prints `go get github.com/nats-io/nats.go@v1.53.1`, and applies nothing

#### Scenario: Unanswered component decision
- **WHEN** the next machine step belongs to a component, and `natsvet-migrate.json` answers `component` neither for it nor for the module
- **THEN** apply exits non-zero, naming the pending `component` question, and no file changes

#### Scenario: A step that does not compile
- **WHEN** the planner emits a machine step whose edits do not type-check
- **THEN** apply restores every file it wrote to its previous bytes, and exits non-zero, printing the step id and the type errors

#### Scenario: Nothing left to apply
- **WHEN** the plan has no machine step left
- **THEN** apply exits 0 saying so, and names the remaining guided, decision and unmapped sites
