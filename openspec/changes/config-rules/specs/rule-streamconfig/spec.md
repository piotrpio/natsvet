## Purpose

streamconfig reports stream configurations that nats-server rejects at create or update time. Every check mirrors one in the server's checkStreamCfgLocked and fires only when the involved fields are constants in the same composite literal, so a report is a guaranteed runtime error, reported at the literal instead of as a JetStreamError from CreateStream.

## ADDED Requirements

### Requirement: Only constant top-level fields of stream config literals are examined
The rule SHALL examine composite literals whose type is `jetstream.StreamConfig` or legacy `nats.StreamConfig`, directly, through `&T{...}`, and as elements of slices or maps of those types. A field SHALL take part in a check only when it is present with a compile-time constant value (or a slice literal of constants, or `nil`); a field set from a variable or call makes every check that involves it inapplicable. A literal that is a direct call argument or return value is checked as written. A literal bound to a variable is checked as written only up to the point where the variable is handed off — passed to a call by value, returned, sent on a channel, or passed by pointer to a nats.go method — and any field assigned (`x.<Field> = ...`, on any receiver) before that point SHALL be unknown, whether or not the literal sets it. If before the hand-off a pointer to the variable reaches any other call, the variable is a method receiver, or it is captured by a closure, or no hand-off is found in the block, every field assigned anywhere in the function SHALL be unknown instead. Absent enum fields SHALL take the server default: `Retention` `LimitsPolicy`, `Discard` `DiscardOld`, `Storage` `FileStorage`, `Replicas` `1`, `PersistMode` default. Nested `Mirror`, `Sources`, `SubjectTransform`, `RePublish`, `Placement` and `ConsumerLimits` literals SHALL only be tested for presence (non-`nil`). Each diagnostic SHALL be reported at the literal, carry category `streamconfig`, and read `stream config: <server wording>`.

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
