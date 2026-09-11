## Why

A stream or consumer config the server rejects, or a KV bucket name or key nats.go rejects, is the most common class of "it worked in the docs, fails in my program" report: the error surfaces only at `CreateStream`/`CreateConsumer`/`KeyValue.Put` time, often in production, with a server message that names the field but not the line. Every one of these validations runs on values that are usually compile-time constants in the user's composite literal, so they can be reported at the literal, before the program runs. With the framework, testdata module and corpus gate in place from `bootstrap`, this change adds the three config rules of Tier 1 — the largest and, on the corpus, the likeliest to find real misconfigurations.

## What Changes

- Rule `consumerconfig` (default-on): mirrors every check in nats-server `checkConsumerCfg` that depends only on the consumer config itself, on composite literals of `jetstream.ConsumerConfig`, `jetstream.OrderedConsumerConfig` and legacy `nats.ConsumerConfig`.
- Rule `streamconfig` (default-on): mirrors the top-level checks of nats-server `checkStreamCfgLocked` on composite literals of `jetstream.StreamConfig` and legacy `nats.StreamConfig`, including pairwise subject overlap.
- Rule `kvconfig` (default-on): mirrors nats.go's client-side `bucketValid`, the `KeyValueMaxHistory` limit, `keyValid` and `searchKeyValid` on `jetstream.KeyValueConfig` / `nats.KeyValueConfig` / `jetstream.ObjectStoreConfig` / `nats.ObjectStoreConfig` literals and on constant key arguments to `KeyValue` methods.
- New `internal/natsapi` helpers: `CompositeFields` (keyed fields of a composite literal of a given nats.go struct type, through `&` and pointer forms), `ConstInt`, `ConstDuration`, `ConstBool` and enum-constant resolution, `SliceConstStrings` (constant elements of a slice literal), and the subject functions ported from nats-server `server/sublist.go` — `IsValidSubject`, `SubjectIsLiteral`, `SubjectsCollide`, `SubjectIsSubsetMatch` — with the server's own test tables, plus the KV regexes and `KeyValid`/`SearchKeyValid`.
- Corpus run after the three rules land; new findings triaged into `scripts/corpus.expected`.
- README rule table extended.

## Capabilities

### New Capabilities
- `rule-consumerconfig`: the `consumerconfig` rule — every server check it mirrors, cited, with positive and negative scenarios, on both `jetstream` and legacy types.
- `rule-streamconfig`: the `streamconfig` rule — the same for stream configs.
- `rule-kvconfig`: the `kvconfig` rule — bucket names, history limit, keys.

### Modified Capabilities
- `analyzer-framework`: adds the requirement that the shared subject helpers match nats-server's semantics (verified by its test tables) and that composite-literal field extraction sees through `&T{}`, pointer element types and legacy twins.

## Non-goals

- Checks that need the stream config the consumer attaches to (`Replicas` vs stream, retention policy, `ConsumerLimits`), server or account limits, or other streams (mirror/source targets). The rule cannot see them.
- Checks inside nested literals: `Mirror`/`Sources` (`StreamSource`), `SubjectTransform`, `RePublish`, `Placement`, `ConsumerLimits`. Listed as a follow-up; `CompositeFields` is designed so they can be added without restructuring.
- Configs built field by field (`cfg.Name = ...`) or from variables. A field that is not a constant in the literal is unknown and disables every check that involves it.
- Fixes. Which field the user meant is ambiguous for every check here.
- Any change to `headerkey` or `legacyjs`.

## Rule defaults

- `consumerconfig`: default-on.
- `streamconfig`: default-on.
- `kvconfig`: default-on.

## Shared helpers

Added to `internal/natsapi`: `CompositeFields`, `ConstInt`, `ConstDuration`, `ConstBool`, `ConstEnum` (named-constant identity for policy enums), `SliceConstStrings`, `IsValidSubject`, `SubjectIsLiteral`, `SubjectsCollide`, `SubjectIsSubsetMatch`, `BucketValid`, `KeyValid`, `SearchKeyValid`. Reused: `IsPkg`, `Callee`, `IsMethod`, `ConstString`.

## Impact

- Three new default-on rules; `Analyzers()` grows from one to four. Users of `natsvet ./...` and `go vet -vettool` see them immediately.
- `internal/natsapi` grows a subject package worth of code; its correctness is pinned to nats-server's own test table rather than to our reading of the algorithm.
- Corpus expectations may gain lines (natscli and nack build many stream and consumer configs); each is triaged before it is accepted.
- No new dependencies. The server-derived constants (`StreamMaxReplicas = 5`, `JSMaxNameLen = 255`, `JSMaxDescriptionLen = 4096`, 100ms floors, 1ms `MaxRequestExpires` floor) are long-standing and are encoded as constants with the server function cited in the rule spec.
