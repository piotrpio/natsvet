# rule-streamconfig Specification

## Purpose

streamconfig reports stream configurations that nats-server rejects at create or update time. Every check mirrors one in the server's checkStreamCfgLocked (or, for a stream source with both Domain and External, nats.go's convertDomain) and fires only when the involved fields are constants, so a report is a guaranteed runtime error, reported at the literal instead of as a JetStreamError from CreateStream. SubjectTransformConfig, RePublish and StreamSource literals are checked wherever they are written, including inside KeyValue configs: transform sources that are not valid subjects, transform destinations the server's ValidateMapping rejects, republish mappings it cannot build, a filter subject combined with subject transforms, overlapping transform sources, and invalid durable-source consumers. Inside a stream config the sourced and mirrored stream names and republish cycles are checked as well.

## Requirements

### Requirement: Only constant top-level fields of stream config literals are examined
The rule SHALL examine composite literals whose type is `jetstream.StreamConfig` or legacy `nats.StreamConfig`, directly, through `&T{...}`, and as elements of slices or maps of those types. A field SHALL take part in a check only when it is present with a compile-time constant value (or a slice literal of constants, or `nil`); a field set from a variable or call makes every check that involves it inapplicable. A literal that is a direct call argument or return value is checked as written. A literal bound to a variable is checked as written only up to the point where the variable is handed off — passed by value to a concretely typed parameter, returned, sent on a channel, or passed by pointer to a nats.go method (a by-value pass to an interface parameter such as a logger's `...any` is not a hand-off) — and any field assigned (`x.<Field> = ...` or `x.<Field>[i] = ...`, on any receiver) before that point SHALL be unknown, whether or not the literal sets it. If before the hand-off a pointer to the variable reaches any other call, the variable is a method receiver, or it is captured by a closure, or no hand-off is found in the block, every field assigned anywhere in the function SHALL be unknown instead. Absent enum fields SHALL take the server default: `Retention` `LimitsPolicy`, `Discard` `DiscardOld`, `Storage` `FileStorage`, `Replicas` `1`, `PersistMode` default. Nested `Placement` and `ConsumerLimits` literals SHALL only be tested for presence (non-`nil`); nested `Mirror`, `Sources`, `SubjectTransform` and `RePublish` literals SHALL be examined as the nested-literal requirements below define. Each top-level diagnostic SHALL be reported at the literal, carry category `streamconfig`, and read `stream config: <server wording>`.

#### Scenario: Field from a variable disables the check
- **WHEN** a literal has `Name: name` from a variable and `Replicas: 7`
- **THEN** the rule reports only the replicas diagnostic

#### Scenario: Legacy twin
- **WHEN** code writes `&nats.StreamConfig{Name: "a.b"}`
- **THEN** the rule reports the same diagnostic as for `jetstream.StreamConfig`

#### Scenario: Unrelated struct
- **WHEN** code writes a literal of a user-defined struct with `Name` and `Replicas` fields
- **THEN** the rule reports nothing

#### Scenario: Absent field completed later in the function
- **WHEN** a function has `cfg := jetstream.StreamConfig{Name: "s", DiscardNewPerSubject: true, MaxMsgsPerSubject: 10}` followed by `cfg.Discard = jetstream.DiscardNew`
- **THEN** the rule reports nothing for that literal

#### Scenario: Unrelated field assigned later
- **WHEN** a function has `cfg := jetstream.StreamConfig{Name: "s", Replicas: 7}` followed by `cfg.Subjects = []string{"x"}`
- **THEN** the rule still reports the replicas message

#### Scenario: Indexed assignment masks the slice field
- **WHEN** a function has `cfg := jetstream.StreamConfig{Name: "s", Subjects: []string{"a", "a"}}` followed by `cfg.Subjects[1] = "b"` and then `js.CreateStream(ctx, cfg)`
- **THEN** the rule reports nothing for that literal

### Requirement: Stream name is a valid asset name within the length limit
Mirrors `checkStreamCfgLocked` via `isValidAssetName` and `JSMaxNameLen` = 255. The rule SHALL report `Name` present and empty, or containing any of `.`, `*`, `>`, `\`, `/` or whitespace, with `stream config: stream name is required and can not contain '.', '*', '>', '\', '/' or whitespace`; and a `Name` longer than 255 bytes with `stream config: stream name is too long, maximum allowed is 255`.

#### Scenario: Dotted name
- **WHEN** a literal has `Name: "orders.v1"`
- **THEN** the rule reports the name message

#### Scenario: Empty name
- **WHEN** a literal has `Name: ""`
- **THEN** the rule reports the name message

#### Scenario: Name absent
- **WHEN** a literal has no `Name` field
- **THEN** the rule reports nothing for the name (it may be set later)

#### Scenario: Valid name
- **WHEN** a literal has `Name: "ORDERS_v1-a"`
- **THEN** the rule reports nothing

### Requirement: Description length limit
Mirrors `checkStreamCfgLocked` (`JSMaxDescriptionLen` = 4096). The rule SHALL report a constant `Description` longer than 4096 bytes with `stream config: stream description is too long, maximum allowed is 4096`.

#### Scenario: Long description
- **WHEN** a literal's `Description` constant is 4097 bytes
- **THEN** the rule reports the message

#### Scenario: Limit
- **WHEN** a literal's `Description` constant is 4096 bytes
- **THEN** the rule reports nothing

### Requirement: Replicas within range
Mirrors `checkStreamCfgLocked` (`StreamMaxReplicas` = 5). The rule SHALL report `Replicas > 5` with `stream config: maximum replicas is 5` and `Replicas < 0` with `stream config: replicas count cannot be negative`.

#### Scenario: Too many replicas
- **WHEN** a literal has `Replicas: 7`
- **THEN** the rule reports the maximum message

#### Scenario: Valid replicas
- **WHEN** a literal has `Replicas: 3` or no `Replicas`
- **THEN** the rule reports nothing

### Requirement: MaxAge and Duplicates windows are sane
Mirrors `checkStreamCfgLocked`. The rule SHALL report `MaxAge < 0` with `stream config: max age can not be negative`; `MaxAge` in `(0, 100ms)` with `stream config: max age needs to be >= 100ms`; `Duplicates < 0` with `stream config: duplicates window can not be negative`; `Duplicates` in `(0, 100ms)` with `stream config: duplicates window needs to be >= 100ms`; `Duplicates > MaxAge` when both are constants and `MaxAge > 0` with `stream config: duplicates window can not be larger then max age`. `Duplicates` absent or `0` is defaulted by the server and SHALL never be reported.

#### Scenario: Tiny max age
- **WHEN** a literal has `MaxAge: 50 * time.Millisecond`
- **THEN** the rule reports the max age floor message

#### Scenario: Duplicates larger than max age
- **WHEN** a literal has `MaxAge: time.Minute` and `Duplicates: time.Hour`
- **THEN** the rule reports the larger-than message

#### Scenario: Duplicates defaulted
- **WHEN** a literal has `MaxAge: time.Minute` and no `Duplicates`
- **THEN** the rule reports nothing

#### Scenario: Valid windows
- **WHEN** a literal has `MaxAge: 24 * time.Hour` and `Duplicates: 2 * time.Minute`
- **THEN** the rule reports nothing

### Requirement: Rollup requires purge
Mirrors `checkStreamCfgLocked`. The rule SHALL report `DenyPurge: true` together with `AllowRollup: true` with `stream config: roll-ups require the purge permission`.

#### Scenario: Both set
- **WHEN** a literal has `DenyPurge: true` and `AllowRollup: true`
- **THEN** the rule reports the message

#### Scenario: Only one set
- **WHEN** a literal has `AllowRollup: true` and no `DenyPurge`
- **THEN** the rule reports nothing

### Requirement: Counter streams
Mirrors `checkStreamCfgLocked`. When `AllowMsgCounter: true` the rule SHALL report `Discard: DiscardNew` with `stream config: counter stream cannot use discard new`; `AllowMsgTTL: true` with `stream config: counter stream cannot use message TTLs`; `AllowMsgSchedules: true` with `stream config: counter stream cannot use message schedules`; `Retention` present and not `LimitsPolicy` with `stream config: counter stream can only use limits retention`.

#### Scenario: Counter with work queue retention
- **WHEN** a literal has `AllowMsgCounter: true` and `Retention: jetstream.WorkQueuePolicy`
- **THEN** the rule reports the retention message

#### Scenario: Valid counter stream
- **WHEN** a literal has `AllowMsgCounter: true` and no `Discard`, `Retention`, `AllowMsgTTL` or `AllowMsgSchedules`
- **THEN** the rule reports nothing

### Requirement: Discard new per subject
Mirrors `checkStreamCfgLocked`. When `DiscardNewPerSubject: true` the rule SHALL report `Discard` absent or not `DiscardNew` with `stream config: discard new per subject requires discard new policy to be set`, and `MaxMsgsPerSubject` absent or `<= 0` with `stream config: discard new per subject requires max msgs per subject > 0`.

#### Scenario: Missing discard policy
- **WHEN** a literal has `DiscardNewPerSubject: true`, `MaxMsgsPerSubject: 10` and no `Discard`
- **THEN** the rule reports the discard policy message

#### Scenario: Valid
- **WHEN** a literal has `DiscardNewPerSubject: true`, `Discard: jetstream.DiscardNew`, `MaxMsgsPerSubject: 10`
- **THEN** the rule reports nothing

### Requirement: Subject delete marker TTL
Mirrors `checkStreamCfgLocked`. The rule SHALL report `SubjectDeleteMarkerTTL < 0` with `stream config: subject delete marker TTL must not be negative` and `SubjectDeleteMarkerTTL` in `(0, 1s)` with `stream config: subject delete marker TTL must be at least 1 second`.

#### Scenario: Sub-second marker TTL
- **WHEN** a literal has `SubjectDeleteMarkerTTL: 500 * time.Millisecond`
- **THEN** the rule reports the at-least message

#### Scenario: Valid marker TTL
- **WHEN** a literal has `SubjectDeleteMarkerTTL: time.Minute` and `AllowMsgTTL: true`
- **THEN** the rule reports nothing

### Requirement: Message scheduling constraints
Mirrors `checkStreamCfgLocked`. When `AllowMsgSchedules: true` the rule SHALL report `Discard: DiscardNew` with `stream config: message scheduling cannot use discard new` and a non-empty `Sources` with `stream config: stream source can not also schedule messages`.

#### Scenario: Scheduling with discard new
- **WHEN** a literal has `AllowMsgSchedules: true` and `Discard: jetstream.DiscardNew`
- **THEN** the rule reports the discard message

#### Scenario: Valid scheduling
- **WHEN** a literal has `AllowMsgSchedules: true` and `AllowRollup: true`
- **THEN** the rule reports nothing

### Requirement: Async persist mode constraints
Mirrors `checkStreamCfgLocked`. When `PersistMode` is `AsyncPersistMode` the rule SHALL report `Storage: MemoryStorage` with `stream config: async persist mode is only supported on file storage`, `Replicas > 1` with `stream config: async persist mode is not supported on replicated streams`, and `AllowAtomicPublish: true` with `stream config: async persist mode is not supported with atomic batch publish`. Legacy `nats.StreamConfig` has no `PersistMode` field and is exempt.

#### Scenario: Async on a replicated stream
- **WHEN** a literal has `PersistMode: jetstream.AsyncPersistMode` and `Replicas: 3`
- **THEN** the rule reports the replicated message

#### Scenario: Valid async
- **WHEN** a literal has `PersistMode: jetstream.AsyncPersistMode` and no `Storage`, `Replicas` or `AllowAtomicPublish`
- **THEN** the rule reports nothing

### Requirement: Mirror streams exclude conflicting settings
Mirrors `checkStreamCfgLocked`. When `Mirror` is present and not `nil` the rule SHALL report: `FirstSeq > 0` with `stream config: stream mirrors can not have first sequence configured`; non-empty `Subjects` with `stream config: stream mirrors can not contain subjects`; non-empty `Sources` with `stream config: stream mirrors can not also contain other sources`; `AllowMsgCounter: true` with `stream config: stream mirrors can not also calculate counters`; `AllowAtomicPublish: true` with `stream config: stream mirrors can not also use atomic publishing`; `AllowBatchPublish: true` with `stream config: stream mirrors can not also use batch publishing`; `AllowMsgSchedules: true` with `stream config: stream mirrors can not also schedule messages`; `SubjectDeleteMarkerTTL > 0` with `stream config: subject delete markers forbidden on mirrors`.

#### Scenario: Mirror with subjects
- **WHEN** a literal has `Mirror: &jetstream.StreamSource{Name: "ORDERS"}` and `Subjects: []string{"orders.>"}`
- **THEN** the rule reports the subjects message

#### Scenario: Mirror with sources
- **WHEN** a literal has `Mirror: &jetstream.StreamSource{Name: "A"}` and `Sources: []*jetstream.StreamSource{{Name: "B"}}`
- **THEN** the rule reports the sources message

#### Scenario: Plain mirror
- **WHEN** a literal has `Name: "M"` and `Mirror: &jetstream.StreamSource{Name: "ORDERS"}` and nothing else
- **THEN** the rule reports nothing

### Requirement: Subjects are valid, distinct and non-overlapping
Mirrors the `Subjects` loop of `checkStreamCfgLocked`. For a `Subjects` slice literal the rule SHALL report: an element failing `IsValidSubject` with `stream config: invalid subject "<subject>"`; two equal elements with `stream config: duplicate subjects detected`; two elements where `SubjectsCollide` with `stream config: subject "<a>" overlaps with "<b>"`; an element equal to `>` without `NoAck: true` with `stream config: capturing all subjects requires no-ack to be true`, or with `Replicas` present and not `1` with `stream config: capturing all subjects requires replicas of 1`; an element other than `>` itself colliding with `$JS.>`, `$JSC.>` or `$NRG.>` (unless it is a subset of `$JS.EVENT.>`) or with `$SYS.>` (unless it is a subset of `$SYS.ACCOUNT.>`) without `NoAck: true`, with `stream config: subjects that overlap with jetstream api require no-ack to be true` or `... system api ...` respectively. Non-constant elements SHALL be skipped and SHALL not disable the checks on the constant elements.

#### Scenario: Overlapping subjects
- **WHEN** a literal has `Subjects: []string{"orders.>", "orders.new"}`
- **THEN** the rule reports `stream config: subject "orders.>" overlaps with "orders.new"`

#### Scenario: Duplicate subjects
- **WHEN** a literal has `Subjects: []string{"a", "a"}`
- **THEN** the rule reports the duplicate message

#### Scenario: Invalid subject
- **WHEN** a literal has `Subjects: []string{"orders. new"}`
- **THEN** the rule reports the invalid subject message

#### Scenario: Capture-all without no-ack
- **WHEN** a literal has `Subjects: []string{">"}`
- **THEN** the rule reports the no-ack message

#### Scenario: JetStream API overlap
- **WHEN** a literal has `Subjects: []string{"$JS.API.>"}`
- **THEN** the rule reports the jetstream api message

#### Scenario: Event subjects are allowed
- **WHEN** a literal has `Subjects: []string{"$JS.EVENT.ADVISORY.>"}`
- **THEN** the rule reports nothing

#### Scenario: Disjoint subjects
- **WHEN** a literal has `Subjects: []string{"orders.new", "orders.paid", "shipments.*"}`
- **THEN** the rule reports nothing

#### Scenario: Mixed constant and variable elements
- **WHEN** a literal has `Subjects: []string{"a", "a", prefix + ".x"}` where `prefix` is a variable
- **THEN** the rule reports the duplicate message for the two constant elements

### Requirement: Nested source, transform and republish literals are examined
The rule SHALL examine composite literals of `jetstream.SubjectTransformConfig`, `jetstream.RePublish`, `jetstream.StreamSource` and `jetstream.StreamConsumerSource` and their legacy `nats` twins (which have no `StreamConsumerSource`), wherever they are written: as a field value or slice element inside a stream or KeyValue config literal, or on their own (bound to a variable, passed, returned). Checks on the nested literal alone SHALL run for every such literal. Checks that need the enclosing stream config SHALL run only when the nested literal is written inside a `jetstream.StreamConfig` or `nats.StreamConfig` literal as its `Mirror`, an element of its `Sources`, its `SubjectTransform` or its `RePublish`; a nested field given by a variable or call (`Mirror: m`) makes every parent check involving it inapplicable. A `StreamSource` literal written as an element of the `Sources` of a legacy `nats.KeyValueConfig` literal SHALL NOT get the transform-source, transform-mapping, filter-and-transforms or overlap checks, and neither SHALL the `SubjectTransformConfig` literals inside it, because legacy `CreateKeyValue` replaces its transforms before submission; its other checks still apply. A nested literal's fields SHALL be unknown under the same hand-off rule as the outermost literal that contains it, applied by field name. Absent string fields SHALL be empty. Each diagnostic about a nested literal SHALL be reported at that nested literal, carry category `streamconfig`, and read `stream config: <wording>`. No check offers a SuggestedFix.

#### Scenario: Republish bound to its own variable
- **WHEN** a function has `rp := &jetstream.RePublish{Source: "orders.*", Destination: "repub.$2"}` and later passes `rp` in a stream config
- **THEN** the rule reports the republish mapping message at the `RePublish` literal

#### Scenario: Nested literal inside a KeyValue config
- **WHEN** a `jetstream.KeyValueConfig` literal has `RePublish: &jetstream.RePublish{Source: "orders.*", Destination: "repub.$2"}`
- **THEN** the rule reports the republish mapping message

#### Scenario: Legacy KeyValue source transforms are replaced by nats.go
- **WHEN** a `nats.KeyValueConfig` literal has `Sources: []*nats.StreamSource{{Name: "other", SubjectTransforms: []nats.SubjectTransformConfig{{Source: "events.>.*", Destination: "x.>"}}}}`
- **THEN** the rule reports nothing

#### Scenario: jetstream KeyValue source transforms are submitted as written
- **WHEN** a `jetstream.KeyValueConfig` literal has `Sources: []*jetstream.StreamSource{{Name: "other", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "events.>.*", Destination: "x.>"}}}}`
- **THEN** the rule reports the invalid transform source message

#### Scenario: Mirror from a variable
- **WHEN** a stream config literal has `Mirror: m` where `m` is a variable
- **THEN** no parent check involving the mirror is applied

#### Scenario: Nested field completed later
- **WHEN** a function has `cfg := jetstream.StreamConfig{Name: "s", Sources: []*jetstream.StreamSource{{Name: "A", FilterSubject: "a.>", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "a.b", Destination: "c.b"}}}}}` followed by `cfg.Sources[0].FilterSubject = ""` and then `js.CreateStream(ctx, cfg)`
- **THEN** the rule does not report the filter-and-transforms message

### Requirement: Subject transform sources are valid subjects
Mirrors the `tr.Source != "" && !IsValidSubject(tr.Source)` checks in nats-server's `checkStreamCfg` for the stream `SubjectTransform`, each `Mirror.SubjectTransforms` element and each `Sources[i].SubjectTransforms` element (`JSStreamTransformInvalidSource`, `JSMirrorInvalidSubjectFilter`, `JSSourceInvalidSubjectFilter`). The rule SHALL report a `SubjectTransformConfig` literal with a constant, non-empty `Source` that is not a valid subject with `stream config: subject transform source: invalid subject "<source>"`.

#### Scenario: Full wildcard before a token
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Source: "events.>.*", Destination: "x.>"}`
- **THEN** the rule reports `stream config: subject transform source: invalid subject "events.>.*"`

#### Scenario: Legacy twin
- **WHEN** a literal is `nats.SubjectTransformConfig{Source: "events..a", Destination: "x"}`
- **THEN** the rule reports the invalid subject message

#### Scenario: Empty source
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Destination: "x.>"}`
- **THEN** the rule does not report the source message

#### Scenario: Valid wildcard source
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Source: "events.*", Destination: "x.{{wildcard(1)}}"}`
- **THEN** the rule reports nothing

### Requirement: Subject transform destinations are valid mappings
Mirrors the `ValidateMapping(tr.Source, tr.Destination)` calls in nats-server's `checkStreamCfg` (`JSStreamTransformInvalidDestination`, `JSMirrorInvalidTransformDestination`, `JSSourceInvalidTransformDestination`), including the `NewSubjectTransform` validation it ends with, where an empty source means `>` and an empty destination is valid. For a `SubjectTransformConfig` literal whose `Source` and `Destination` are constants and whose `Source` is empty or a valid subject, the rule SHALL report a mapping the server rejects with `stream config: subject transform from "<source>" to "<destination>": <server error>`, where `<server error>` is nats-server's error text (for example `invalid mapping destination: wildcard index out of range in {{split(3,1)}}: [3]`).

#### Scenario: Index beyond the source wildcards
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Source: "events.*", Destination: "events.{{split(3,1)}}"}`
- **THEN** the rule reports `stream config: subject transform from "events.*" to "events.{{split(3,1)}}": invalid mapping destination: wildcard index out of range in {{split(3,1)}}: [3]`

#### Scenario: Two functions in one token
- **WHEN** a literal has `Source: "events.*.*"` and `Destination: "events.{{wildcard(1)}}{{split(3,1)}}"`
- **THEN** the rule reports the message ending `invalid mapping destination: too many arguments passed to the function in {{wildcard(1)}}{{split(3,1)}}`

#### Scenario: Unknown function
- **WHEN** a literal has `Source: "a.*"` and `Destination: "b.{{unknown(1)}}"`
- **THEN** the rule reports the message ending `invalid mapping destination: unknown function in {{unknown(1)}}`

#### Scenario: Empty source requires a full wildcard destination
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Destination: "archive.orders"}`
- **THEN** the rule reports `stream config: subject transform from "" to "archive.orders": invalid subject`

#### Scenario: Valid mapping
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Source: "orders.*.*", Destination: "archive.{{wildcard(2)}}.{{partition(3,1)}}"}`
- **THEN** the rule reports nothing

#### Scenario: Empty destination
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Source: "orders.>"}`
- **THEN** the rule reports nothing

#### Scenario: Invalid source is reported once
- **WHEN** a literal is `jetstream.SubjectTransformConfig{Source: "events.>.*", Destination: "x.$1"}`
- **THEN** the rule reports only the invalid source message

### Requirement: Republish mappings are valid transforms
Mirrors the `NewSubjectTransform(cfg.RePublish.Source, cfg.RePublish.Destination)` check in nats-server's `checkStreamCfg`, after the empty source defaults to `>` (`JSStreamInvalidConfig`, "stream configuration for republish with transform from … not valid"). For a `RePublish` literal whose `Source` and `Destination` are constants and whose `Destination` is non-empty, the rule SHALL report a pair the server rejects with `stream config: republish with transform from "<source>" to "<destination>" not valid`, with `<source>` after defaulting.

#### Scenario: Wildcard index out of range
- **WHEN** a literal is `jetstream.RePublish{Source: "orders.*", Destination: "repub.$2"}`
- **THEN** the rule reports `stream config: republish with transform from "orders.*" to "repub.$2" not valid`

#### Scenario: Empty source without a full wildcard destination
- **WHEN** a literal is `jetstream.RePublish{Destination: "repub.orders"}`
- **THEN** the rule reports `stream config: republish with transform from ">" to "repub.orders" not valid`

#### Scenario: Prefixing republish
- **WHEN** a literal is `jetstream.RePublish{Source: "orders.>", Destination: "repub.orders.>"}`
- **THEN** the rule reports nothing

#### Scenario: Legacy twin with default source
- **WHEN** a literal is `nats.RePublish{Destination: "repub.>"}`
- **THEN** the rule reports nothing

### Requirement: A stream source cannot combine a filter subject with subject transforms
Mirrors `JSMirrorMultipleFiltersNotAllowed` and `JSSourceMultipleFiltersNotAllowed` in nats-server's `checkStreamCfg`. The rule SHALL report a `StreamSource` literal with a constant non-empty `FilterSubject` and a non-empty `SubjectTransforms` slice literal with `stream config: a source or mirror with subject transforms cannot also have a single subject filter`.

#### Scenario: Both set
- **WHEN** a literal is `&jetstream.StreamSource{Name: "A", FilterSubject: "a.>", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "a.b", Destination: "c.b"}}}`
- **THEN** the rule reports the filter-and-transforms message

#### Scenario: Filter only
- **WHEN** a literal is `&jetstream.StreamSource{Name: "A", FilterSubject: "a.>"}`
- **THEN** the rule reports nothing

#### Scenario: Empty filter with transforms
- **WHEN** a literal has `FilterSubject: ""` and one subject transform
- **THEN** the rule reports nothing

### Requirement: Subject transform sources within one stream source do not overlap
Mirrors the overlap loops in nats-server's `checkStreamCfg`: for `Sources` elements two transform sources are rejected when either is a subset match of the other (`subjectIsSubsetMatch`, `JSSourceOverlappingSubjectFilters`); for the `Mirror` they are rejected when they collide (`SubjectsCollide`, `JSMirrorOverlappingSubjectFilters`). The rule SHALL apply the subset test to every `StreamSource` literal and SHALL apply the collision test only to a literal written as the `Mirror` of a stream config literal or of a KeyValue config literal (nats.go submits a KeyValue mirror as the stream's mirror). Only transform sources that are constants and empty or valid subjects take part. A pair SHALL be reported once, with `stream config: subject transform sources "<a>" and "<b>" can not overlap`.

#### Scenario: Subset in a source
- **WHEN** a `Sources` element has transforms with sources `orders.>` and `orders.new`
- **THEN** the rule reports `stream config: subject transform sources "orders.>" and "orders.new" can not overlap`

#### Scenario: Collision in a mirror
- **WHEN** a stream config literal has a `Mirror` whose transforms have sources `orders.*.new` and `orders.eu.*`
- **THEN** the rule reports the overlap message

#### Scenario: Collision in a source is allowed
- **WHEN** a `Sources` element has transforms with sources `orders.*.new` and `orders.eu.*`
- **THEN** the rule reports nothing

#### Scenario: Disjoint sources
- **WHEN** a `StreamSource` literal has transforms with sources `orders.>` and `returns.>`
- **THEN** the rule reports nothing

### Requirement: Stream sources do not set both Domain and External
Mirrors nats.go's `(*StreamSource).convertDomain`, which `CreateStream`, `UpdateStream` and legacy `AddStream`/`UpdateStream` run on the mirror and every source before submission and which fails with `nats: domain and external are both set`. The rule SHALL report a `StreamSource` literal with a constant non-empty `Domain` and a non-`nil` `External` with `stream config: domain and external are both set`.

#### Scenario: Both set
- **WHEN** a literal is `&jetstream.StreamSource{Name: "A", Domain: "hub", External: &jetstream.ExternalStream{APIPrefix: "$JS.hub.API"}}`
- **THEN** the rule reports the domain message

#### Scenario: Domain only
- **WHEN** a literal is `&jetstream.StreamSource{Name: "A", Domain: "hub"}`
- **THEN** the rule reports nothing

### Requirement: Durable source consumers are valid
Mirrors the `Consumer` checks for the mirror and for each source in nats-server's `checkStreamCfg` (`JSMirrorDurableConsumerCfgInvalid`, `JSSourceDurableConsumerCfgInvalid`). For a `jetstream.StreamSource` literal whose `Consumer` is a `StreamConsumerSource` literal, the rule SHALL report, with `stream config: stream source consumer config is invalid: <reason>`: a constant consumer `Name` that is empty or contains `.`, `*`, `>`, `\`, `/` or whitespace (`consumer name is required and can not contain '.', '*', '>', '\', '/' or whitespace`); a constant `DeliverSubject` that is empty, not a valid subject, or contains a wildcard (`deliver subject must be a valid literal subject`); a constant non-zero `OptStartSeq` or a non-`nil` `OptStartTime` on the stream source (`a start sequence or start time can not be set`); a constant non-empty `FilterSubject` on the stream source (`a filter subject can not be set`).

#### Scenario: Missing deliver subject
- **WHEN** a literal is `&jetstream.StreamSource{Name: "O", Consumer: &jetstream.StreamConsumerSource{Name: "C"}}`
- **THEN** the rule reports the message ending `deliver subject must be a valid literal subject`

#### Scenario: Consumer with a filter subject
- **WHEN** a literal has `FilterSubject: "o.>"` and `Consumer: &jetstream.StreamConsumerSource{Name: "C", DeliverSubject: "deliver.c"}`
- **THEN** the rule reports the message ending `a filter subject can not be set`

#### Scenario: Valid durable source
- **WHEN** a literal is `&jetstream.StreamSource{Name: "O", Consumer: &jetstream.StreamConsumerSource{Name: "C", DeliverSubject: "deliver.c"}}`
- **THEN** the rule reports nothing

#### Scenario: No consumer
- **WHEN** a literal is `&jetstream.StreamSource{Name: "O", OptStartSeq: 10}`
- **THEN** the rule reports nothing

### Requirement: Sourced and mirrored stream names are valid
Mirrors `isValidAssetName` on each source name (`JSSourceInvalidStreamName`) and, when the mirror has no `External`, on the mirror name (`JSMirrorInvalidStreamName`) in nats-server's `checkStreamCfg`. Within a stream config literal, the rule SHALL report a `Sources` element that is `nil` or a literal with a constant `Name` that is empty or contains `.`, `*`, `>`, `\`, `/` or whitespace with `stream config: sourced stream name is invalid`, and a `Mirror` literal with such a `Name`, no `External` and an empty `Domain` with `stream config: mirrored stream name is invalid`. The check SHALL NOT apply to sources and mirrors of KeyValue configs, whose names nats.go rewrites.

#### Scenario: Dotted source name
- **WHEN** a stream config literal has `Sources: []*jetstream.StreamSource{{Name: "orders.v1"}}`
- **THEN** the rule reports the sourced stream name message at the source literal

#### Scenario: Unnamed mirror
- **WHEN** a stream config literal has `Mirror: &jetstream.StreamSource{}`
- **THEN** the rule reports the mirrored stream name message

#### Scenario: External mirror
- **WHEN** a stream config literal has `Mirror: &jetstream.StreamSource{External: &jetstream.ExternalStream{APIPrefix: "$JS.hub.API"}}`
- **THEN** the rule does not report the mirrored stream name message

#### Scenario: KeyValue source
- **WHEN** a `jetstream.KeyValueConfig` literal has `Sources: []*jetstream.StreamSource{{Name: ""}}`
- **THEN** the rule reports nothing

### Requirement: Republish destination does not form a cycle
Mirrors the republish cycle check in nats-server's `checkStreamCfg` (`JSStreamInvalidConfig`, "stream configuration for republish destination forms a cycle"). Within a stream config literal with a `RePublish` literal whose `Source` and `Destination` are constants, the rule SHALL take the republish source as `>` when empty, and the stream subjects as the `Subjects` elements, or the constant `Name` when `Subjects`, `Mirror` and `Sources` are all absent (the server's default subject). When source and destination are both `>` and `SubjectTransform` is present, the destination SHALL be the transform destination if there is exactly one stream subject and it equals the transform source (the server's implicit republish); when that cannot be decided because the transform or the single subject is not constant, the rule SHALL report nothing. A destination that collides with any constant stream subject SHALL be reported at the `RePublish` literal with `stream config: republish destination "<destination>" forms a cycle with subject "<subject>"`, once per literal, naming the first such subject.

#### Scenario: Destination under the stream's own subjects
- **WHEN** a literal has `Subjects: []string{"orders.>"}` and `RePublish: &jetstream.RePublish{Source: "orders.>", Destination: "orders.copy.>"}`
- **THEN** the rule reports `stream config: republish destination "orders.copy.>" forms a cycle with subject "orders.>"`

#### Scenario: Default subject
- **WHEN** a literal has `Name: "ORDERS"`, no `Subjects`, and `RePublish: &jetstream.RePublish{Destination: ">"}`
- **THEN** the rule reports the cycle message naming subject `ORDERS`

#### Scenario: Implicit republish through the transform
- **WHEN** a literal has `Subjects: []string{"in.>"}`, `SubjectTransform: &jetstream.SubjectTransformConfig{Source: "in.>", Destination: "out.>"}` and `RePublish: &jetstream.RePublish{Source: ">", Destination: ">"}`
- **THEN** the rule reports nothing

#### Scenario: Disjoint destination
- **WHEN** a literal has `Subjects: []string{"orders.>"}` and `RePublish: &jetstream.RePublish{Source: "orders.>", Destination: "repub.orders.>"}`
- **THEN** the rule reports nothing

#### Scenario: Sourced stream without subjects
- **WHEN** a literal has `Sources` set, no `Subjects`, and `RePublish: &jetstream.RePublish{Destination: ">"}`
- **THEN** the rule does not report the cycle message
