## Purpose

`natsvet migrate plan` tells a team, or the agent working for them, exactly how to move a module off the legacy JetStream API (`nats.JetStreamContext`, `nats.KeyValue`, `nats.ObjectStore`) onto the `jetstream` package: every legacy site, what it becomes, which choices need a human, and an order of steps in which the code compiles after each one. It plans; it never edits code.

## ADDED Requirements

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
The plan SHALL record the nats.go version its mapping table was verified against. When the loaded module requires an older nats.go, the plan SHALL begin with a mechanical step 0, `go get github.com/nats-io/nats.go@latest`, since later v1 releases only add API and every target then exists. When the module requires a newer nats.go, the plan SHALL note that behavior facts were verified on the table's version.

#### Scenario: Old nats.go
- **WHEN** the module requires nats.go v1.31.0 and the table was verified against v1.53.1
- **THEN** the plan's first step is `go get github.com/nats-io/nats.go@latest`

#### Scenario: Current nats.go
- **WHEN** the module requires the table's version
- **THEN** the plan has no step 0

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
The plan SHALL group pending decisions by pattern and scope (module by default), each with the number of sites it covers, its options and its default; when sites of one pattern have different defaults, the most common default is asked at module scope and each other site at its own scope. Answers SHALL be read from `natsvet-migrate.json` at the module root when it exists, or from the file named by `-decisions`; each answer is a pattern, a scope (`module`, `component:<id>` or `site:<id>`) and a choice, and the narrowest scope wins. The planner SHALL NOT write the file. A site whose decisions are all answered is reclassified by the chosen option (mechanical or guided, with its replacement or template), and answered patterns no longer appear as pending. A component answered `skip` SHALL keep its sites out of the steps, be counted separately, and block the removal of any declaration it shares with a migrated component. An answer naming an unknown pattern, option or scope SHALL be an error; an answer whose scope no longer exists SHALL be reported. Given the same packages and the same answers, the plan SHALL be byte-identical.

#### Scenario: One answer covers many sites
- **WHEN** twelve `Subscribe` sites are pending and `natsvet-migrate.json` answers `subscribe-target` with `pull` for the module
- **THEN** the plan no longer lists `subscribe-target` as pending, and each of the twelve sites carries the pull replacement

#### Scenario: Site override
- **WHEN** the file also answers `subscribe-target` with `push` for one site
- **THEN** that site carries the push replacement and the other eleven the pull one

#### Scenario: Skipped component
- **WHEN** the file answers `component` with `skip` for a component of legacy-API tests
- **THEN** its sites are counted as skipped and appear in no step

#### Scenario: Unknown option
- **WHEN** the file answers `subscribe-target` with `poll`
- **THEN** the command exits non-zero naming the pattern and the option

#### Scenario: Deterministic output
- **WHEN** the plan runs twice over the same packages with the same answers
- **THEN** the two outputs are byte-identical

### Requirement: Unmapped sites are reported, never guessed
A legacy symbol without a `jetstream` equivalent SHALL produce an unmapped site with the reason from the mapping table, and the plan SHALL count unmapped sites separately.

#### Scenario: Legacy-only option
- **WHEN** code passes `nats.UseLegacyDurableConsumers()` to `nc.JetStream`
- **THEN** the site is unmapped with a reason, and no replacement is proposed

### Requirement: Components and a dual-handle step order
The planner SHALL group sites into components: a legacy handle's creation sites (roots) and every declaration the handle flows through — variables, struct fields, parameters, results — connected by assignment, call argument and return, across all loaded packages. For each migrated component the plan SHALL order steps so the code compiles after each one: create a `jetstream` handle named `<name>New` next to each legacy root from the same connection; add a sibling declaration of the new type, named `<name>New`, next to each variable, struct field and parameter that carries the legacy handle; migrate the sites (mechanical, then guided, then decided); remove the legacy handle and its declarations; rename each `<name>New` to `<name>`. While a component has guided sites, its removal and rename steps SHALL be marked as waiting on them, since only those sites still use the legacy handle. The plan SHALL mark a component that is package-local and has no pending decisions and no guided sites as safe to apply in one commit. When a legacy value is passed to a package outside the loaded set, or reaches test code while `-tests=false`, the component SHALL be marked blocked at that point and the removal and rename steps SHALL be omitted. A handle the planner cannot thread — returned from a function, received from a call it does not rewrite (a helper, a type assertion), or a parameter fed something other than a handle variable — SHALL get no sibling: its sites are guided, and its component's removal and rename wait on them.

#### Scenario: Handle stored in a struct across packages
- **WHEN** `package app` creates `nc.JetStream()`, stores it in `Service.js`, and `package worker` calls `svc.js.Publish(...)`
- **THEN** one component spans both packages and its steps include adding a `jsNew jetstream.JetStream` field next to `Service.js`

#### Scenario: Independent handles
- **WHEN** two functions each create their own `nc.JetStream()` and never share it
- **THEN** the plan has two components

#### Scenario: One-commit hint
- **WHEN** a component lives in one package and none of its sites has a pending decision or is guided
- **THEN** the plan marks it safe to apply in one commit, and its steps are unchanged

#### Scenario: Guided sites hold the legacy handle
- **WHEN** a component's only remaining legacy use is a guided `Fetch` loop
- **THEN** its removal and rename steps are marked as waiting on that site

#### Scenario: Handle returned from a helper
- **WHEN** code calls `kv, err := fw.KV(ctx, "config")`, a function that returns `nats.KeyValue`, and then `kv.Get("a")`
- **THEN** `kv` gets no sibling, the `Get` site is guided with the reason, and the component's removal waits on it

#### Scenario: External boundary
- **WHEN** a legacy handle is passed to a function of a module outside the loaded packages
- **THEN** the component is marked blocked at that call and its steps omit removing and renaming the handle

### Requirement: The JSON plan is a versioned contract with machine edits, and the Markdown guide is rendered from it
The JSON plan SHALL carry a schema version, the nats.go version the table was verified against, the loaded packages, a SHA-256 hash of every file it edits, counts (sites by class, skipped sites, legacy identifiers), components with their steps, sites and follow-ups, and pending decisions, all in a stable order. Every site and step with a replacement SHALL carry human-readable `before` and `after` text and machine edits, each a file, a start and end byte offset, and the new text. A step's offsets SHALL be against each file as it stands after all earlier steps, and the step SHALL record the SHA-256 each file it edits must have before it is applied; edits within a step SHALL NOT overlap. The plan header SHALL tell agents to read `natsvet migrate skill`. The Markdown format SHALL be rendered from the same plan by template: a summary, the pending decisions as questions with their defaults, each component's steps with their sites, then the follow-ups. The skill SHALL describe the agent loop — answer the pending decisions with the user and record them in `natsvet-migrate.json`, apply one step, build, run `natsvet ./...` and the tests, commit, re-plan — and SHALL name the schema version it understands.

#### Scenario: Schema version in both
- **WHEN** a plan is produced
- **THEN** its schema version equals the version the embedded skill names

#### Scenario: Stale file
- **WHEN** a file changes after the plan was produced
- **THEN** its current hash differs from the hash the next step records for it, so tooling that applies the edits can refuse that file

#### Scenario: Steps apply in order
- **WHEN** the steps of a plan are applied in order, each checking its recorded hashes
- **THEN** every recorded hash matches, including the rename step that edits identifiers inserted by earlier steps

#### Scenario: Markdown mirrors JSON
- **WHEN** the same plan is written in both formats
- **THEN** every site, pending decision, step and follow-up in the JSON appears in the Markdown, and nothing else does

#### Scenario: Reference to the upstream guide
- **WHEN** a site replaces `js.Subscribe`
- **THEN** it references `MIGRATION.md`'s section on replacing `js.Subscribe()`
