## Why

`streamconfig` stops at the top level of a stream config: nested `Mirror`, `Sources`, `SubjectTransform` and `RePublish` literals are only tested for presence (`docs/design.md` §8 item 10, a documented `config-rules` non-goal). Those literals carry the configuration nats-server validates most strictly and users get wrong most easily: subject transform destinations are a small language (`$1`, `{{wildcard(1)}}`, `{{partition(3,1,2)}}`, `{{split(1,-)}}`, …) whose errors surface only as `invalid mapping destination: wildcard index out of range in {{split(3,1)}}: [3]` from `CreateStream`. nats-server's own tests exercise these rejections over nats.go types (`server/jetstream_test.go`: invalid transform source `events.>.*`, bad `{{split(3,1)}}` destinations on stream, source and mirror, overlapping mirror filters, filter-plus-transforms conflicts, republish cycles), so the shapes are real and the server's wording is settled. The checks are deterministic on constants, like every other `streamconfig` check.

## What Changes

- `streamconfig` (default-on, extended) examines nested literals of `SubjectTransformConfig`, `RePublish` and `StreamSource` (jetstream and legacy twins) and reports what nats-server's `checkStreamCfg` or nats.go's `convertStreamConfigDomains` rejects:
  - **Checks on the nested literal alone**, reported wherever the literal appears (in a stream config, a KeyValue config, or bound to its own variable): a transform source that is not a valid subject; a transform destination `ValidateMapping` rejects; a republish mapping `NewSubjectTransform` rejects; a stream source that sets both `FilterSubject` and `SubjectTransforms`; transform sources within one stream source that overlap (subset match); `Domain` together with `External`; an invalid durable-source `Consumer` (name, deliver subject, start position, filter). The transform checks skip a source written in a legacy `nats.KeyValueConfig.Sources`: legacy `CreateKeyValue` overwrites its transforms before submission, so the server never sees them.
  - **Checks that need the enclosing `StreamConfig` literal**: invalid sourced or mirrored stream names (not applied under a KeyValue config, where nats.go prefixes `KV_`); mirror transform sources that collide (the mirror check is stricter than the sources check); a republish destination that collides with the stream's subjects (a cycle), including the implicit republish the server derives from a subject transform, and the stream's default subject `Name` when `Subjects` is absent.
- `kvconfig` (default-on, extended): a `KeyValueConfig` `RePublish` whose destination collides with the bucket's `$KV.<bucket>.>` subject forms a cycle when the bucket has no `Mirror`. This is the first server-side check in `kvconfig`; its Doc and spec purpose change accordingly.
- `internal/natsapi`: a port of nats-server's `ValidateMapping` and the error paths of `NewSubjectTransform` (with `indexPlaceHolders` and the mapping-function regexes), pinned by the server's `TestValidateDestinationSubject` and `TestSubjectTransforms` tables; a `Fields` accessor for nested composite literals in pointer and slice fields; and field masking extended to indexed assignments (`cfg.Sources[0] = src` masks `Sources`).
- Corpus run and triage. nats-server's tests that assert these rejections become `FP` lines (a test asserting an error); every other finding is triaged.
- `docs/rules.md` regenerated; `docs/design.md` §3.1 `streamconfig` gains the nested checks and §8 item 10 is closed. §8 item 8 (legacy `Bind` with a non-empty subject) is closed as won't-do in the same edit: nats.go fails only when the bound consumer has a single `FilterSubject` that differs (`processConsInfo`), which is server-side state no rule can see.

## Capabilities

### New Capabilities

None. The checks extend existing rules; no new rule id.

### Modified Capabilities

- `rule-streamconfig`: the scope requirement stops treating nested literals as presence-only and defines which checks run on a nested literal alone and which need the parent; one new requirement per nested check.
- `rule-kvconfig`: adds the republish-cycle requirement against the bucket subject.
- `analyzer-framework`: adds the requirement that subject transform validation matches nats-server's (verified by its test tables); extends composite-literal extraction to nested literals; extends config masking to indexed element assignments.

## Non-goals

- `Placement` and `ConsumerLimits` literals. Their constraints depend on the cluster and account, not only on the config.
- Checks that need other streams or server state: whether a sourced stream exists, `MaxMsgSize` against the origin, `MirrorDirect` inheritance, `External` prefix overlaps with other streams' subjects (`deliveryPrefixes`/`apiPrefixes`).
- The pedantic-only rejections (`republish source can not be empty`, `implicit republish based on subject transform`). The rule reports what a default create rejects.
- Duplicate source detection (`composeIName`) and duplicate durable-source consumers (`composeCName`). Both are copy-paste slips the server reports with a clear message (`duplicate source configuration detected`), they need their own normalization, and the corpus has no case outside tests asserting the rejection.
- KeyValue sources that nats.go rewrites. nats.go adds a `$KV.<src>.>` → `$KV.<bucket>.>` transform to every KeyValue source (`jetstream` only when the source has none, legacy always), so a KeyValue source with a `FilterSubject` can never work; reporting that needs a model of the rewrite, and no corpus literal does it. The filter-plus-transform check reports only transforms the literal itself sets, and not under legacy KeyValue sources.
- Fixes. Which half of a conflict the user meant is ambiguous in every case.
- `headerkey` near-miss keys (§8 item 9) and the `Bind` subject question (§8 item 8). Separate changes.

## Rule defaults

- `streamconfig`: default-on (unchanged).
- `kvconfig`: default-on (unchanged).

## Shared helpers

Added to `internal/natsapi`: `ValidateMapping` and `SubjectTransformErr` (the error paths of `NewSubjectTransform`) with the mapping-function regexes, and `Fields.Lits` (the composite literals of a nested pointer or slice field, with a completeness flag). Changed: `AssignedFields` treats `x.F[i] = ...` as assigning `F`, which also loosens the existing config rules. Reused: `CompositeFields`, `NewFields`, `Masker`, `IsValidSubject`, `SubjectIsLiteral`, `SubjectsCollide`, `SubjectIsSubsetMatch`, `BucketValid`.

## Impact

- More `streamconfig` diagnostics on existing users: a stream literal with nested constants now reports what the server rejects. No existing diagnostic changes wording or position.
- `internal/natsapi` grows a transform-parser port (about 250 lines at the server source). Its correctness is pinned to the server's own test tables, as the subject helpers are.
- `scripts/corpus.expected` gains `FP` lines for nats-server tests asserting these rejections (about 20 sites in `server/jetstream_test.go` build them with nats.go types). The masking change can only remove existing findings, never add any; the corpus diff shows whether it does.
- No new dependencies. The checks are identical in nats-server v2.14.7 (the corpus pin, `8d8b69a`) and `main` at `7cbac9c`.
