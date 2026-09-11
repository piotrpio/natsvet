## Purpose

consumerconfig reports consumer configurations that nats-server rejects at create or update time. Every check mirrors one in the server's checkConsumerCfg and fires only when the involved fields are constants in the same composite literal, so a report is a guaranteed runtime error, reported at the literal instead of as a JetStreamError from CreateConsumer.

## ADDED Requirements

### Requirement: Only constant fields of consumer config literals are examined
The rule SHALL examine composite literals whose type is `jetstream.ConsumerConfig`, `jetstream.OrderedConsumerConfig` or legacy `nats.ConsumerConfig`, directly, through `&T{...}`, and as elements of slices or maps of those types. A field SHALL take part in a check only when it is present in the literal with a compile-time constant value (or a slice literal of constants, or `nil`); a field set from a variable or call makes every check that involves it inapplicable. An absent field has the type's zero value. A field that any assignment or increment statement in the enclosing function writes (`x.<Field> = ...`, on any receiver) SHALL be unknown for every literal in that function, whether or not the literal sets it: a literal stored in a variable may be completed or changed before use. Field names below are the `jetstream` ones; the legacy twin `nats.ConsumerConfig.Heartbeat` corresponds to `IdleHeartbeat`. Each diagnostic SHALL be reported at the literal, carry category `consumerconfig`, and read `consumer config: <server wording>`.

#### Scenario: Field from a variable disables the check
- **WHEN** a literal has `FilterSubject: subj` and `FilterSubjects: []string{"a"}` where `subj` is a variable
- **THEN** the rule reports nothing for that literal

#### Scenario: Pointer literal
- **WHEN** code writes `&jetstream.ConsumerConfig{Name: "a.b"}`
- **THEN** the rule reports the same diagnostic as for the value literal

#### Scenario: Legacy twin
- **WHEN** code writes `nats.ConsumerConfig{Durable: "a.b"}`
- **THEN** the rule reports the same diagnostic as for `jetstream.ConsumerConfig`

#### Scenario: Unrelated struct with the same field names
- **WHEN** code writes a literal of a user-defined struct with a `FilterSubject` and `FilterSubjects` field
- **THEN** the rule reports nothing

#### Scenario: Absent field completed later in the function
- **WHEN** a function has `cfg := jetstream.ConsumerConfig{DeliverPolicy: jetstream.DeliverByStartSequencePolicy}` followed by `cfg.OptStartSeq = 10`
- **THEN** the rule reports nothing for that literal

#### Scenario: Unrelated field assigned later
- **WHEN** a function has `cfg := jetstream.ConsumerConfig{Durable: "a.b"}` followed by `cfg.FilterSubject = "x"`
- **THEN** the rule still reports the durable-name message

### Requirement: Consumer names are valid asset names
Mirrors `checkConsumerCfg` via `isValidAssetName`. The rule SHALL report `Name` or `Durable` when the constant is non-empty and contains any of `.`, `*`, `>`, `\`, `/`, or whitespace (space, tab, CR, LF, form feed). Messages: `consumer config: consumer name can not contain '.', '*', '>', '\', '/' or whitespace` and `consumer config: consumer durable name can not contain '.', '*', '>', '\', '/' or whitespace`.

#### Scenario: Dotted durable
- **WHEN** a literal has `Durable: "orders.worker"`
- **THEN** the rule reports the durable-name message

#### Scenario: Name with a space
- **WHEN** a literal has `Name: "my worker"`
- **THEN** the rule reports the name message

#### Scenario: Valid names
- **WHEN** a literal has `Name: "orders-worker_1"` or `Durable: ""`
- **THEN** the rule reports nothing for those fields

### Requirement: Replicas, AckWait and BackOff are not negative
Mirrors `checkConsumerCfg`. The rule SHALL report `Replicas < 0` with `consumer config: replicas count cannot be negative`; `AckWait < 0` with `consumer config: consumer ack wait needs to be positive`; any constant element of a `BackOff` slice literal `< 0` with `consumer config: consumer backoff needs to be positive`.

#### Scenario: Negative ack wait
- **WHEN** a literal has `AckWait: -time.Second`
- **THEN** the rule reports the ack wait message

#### Scenario: Negative backoff element
- **WHEN** a literal has `BackOff: []time.Duration{time.Second, -1}`
- **THEN** the rule reports the backoff message

#### Scenario: Zero and positive values
- **WHEN** a literal has `Replicas: 0`, `AckWait: 0` and `BackOff: []time.Duration{time.Second}`
- **THEN** the rule reports nothing for those fields

### Requirement: BackOff is not longer than MaxDeliver
Mirrors `checkConsumerCfg`, after `setConsumerConfigDefaults` maps `MaxDeliver` `0` to `-1` (unlimited). The rule SHALL report a `BackOff` slice literal with more elements than a constant `MaxDeliver > 0`, with `consumer config: max deliver is required to be > length of backoff values`. `MaxDeliver` absent, `0` or `-1` SHALL disable the check.

#### Scenario: Three backoffs, two deliveries
- **WHEN** a literal has `MaxDeliver: 2` and `BackOff: []time.Duration{a, b, c}` with three elements
- **THEN** the rule reports the message

#### Scenario: Unlimited deliveries
- **WHEN** a literal has `BackOff` with three elements and no `MaxDeliver`, or `MaxDeliver: -1`
- **THEN** the rule reports nothing

#### Scenario: Enough deliveries
- **WHEN** a literal has `MaxDeliver: 3` and three `BackOff` elements
- **THEN** the rule reports nothing

### Requirement: Description length limit
Mirrors `checkConsumerCfg` (`JSMaxDescriptionLen` = 4096). The rule SHALL report a constant `Description` longer than 4096 bytes with `consumer config: consumer description is too long, maximum allowed is 4096`.

#### Scenario: Long description
- **WHEN** a literal's `Description` constant is 4097 bytes
- **THEN** the rule reports the message

#### Scenario: Limit
- **WHEN** a literal's `Description` constant is 4096 bytes
- **THEN** the rule reports nothing

### Requirement: Push-only constraints
Mirrors the `DeliverSubject != ""` branch of `checkConsumerCfg`. When `DeliverSubject` is a non-empty constant the rule SHALL report: a `DeliverSubject` containing a `*` or `>` token with `consumer config: consumer deliver subject has wildcards`; a `DeliverSubject` failing `IsValidSubject` with `consumer config: invalid push consumer deliver subject`; `MaxWaiting != 0` with `consumer config: consumer in push mode can not set max waiting`; `MaxAckPending > 0` together with `AckPolicy: AckNonePolicy` with `consumer config: consumer requires ack policy for max ack pending`; `IdleHeartbeat` in `(0, 100ms)` with `consumer config: consumer idle heartbeat needs to be >= 100ms`.

#### Scenario: Wildcard deliver subject
- **WHEN** a literal has `DeliverSubject: "deliver.*"`
- **THEN** the rule reports the wildcard message

#### Scenario: Push with max waiting
- **WHEN** a literal has `DeliverSubject: "deliver.x"` and `MaxWaiting: 10`
- **THEN** the rule reports the max waiting message

#### Scenario: Push with AckNone and MaxAckPending
- **WHEN** a literal has `DeliverSubject: "d"`, `AckPolicy: jetstream.AckNonePolicy`, `MaxAckPending: 100`
- **THEN** the rule reports the ack policy message

#### Scenario: Pull with AckNone and MaxAckPending
- **WHEN** a literal has `AckPolicy: jetstream.AckNonePolicy` and `MaxAckPending: 100` and no `DeliverSubject`
- **THEN** the rule reports nothing (the server only checks this for push consumers)

#### Scenario: Small push heartbeat
- **WHEN** a literal has `DeliverSubject: "d"` and `IdleHeartbeat: 50 * time.Millisecond`
- **THEN** the rule reports the heartbeat message

#### Scenario: Valid push consumer
- **WHEN** a literal has `DeliverSubject: "deliver.x"`, `IdleHeartbeat: time.Second`, `FlowControl: true`
- **THEN** the rule reports nothing

### Requirement: Pull-only constraints
Mirrors the `DeliverSubject == ""` branch of `checkConsumerCfg`. When `DeliverSubject` is absent or `""` the rule SHALL report: `RateLimit > 0` with `consumer config: consumer in pull mode can not have rate limit set`; `MaxWaiting < 0` with `consumer config: consumer max waiting needs to be positive`; `IdleHeartbeat > 0` with `consumer config: consumer idle heartbeat requires a push based consumer`; `FlowControl: true` with `consumer config: consumer flow control requires a push based consumer`; `MaxRequestBatch < 0` with `consumer config: consumer max request batch needs to be > 0`; `MaxRequestExpires` in `(0, 1ms)` with `consumer config: consumer max request expires needs to be >= 1ms`.

#### Scenario: Heartbeat on a pull consumer
- **WHEN** a literal has `Durable: "w"` and `IdleHeartbeat: 5 * time.Second` and no `DeliverSubject`
- **THEN** the rule reports the heartbeat message

#### Scenario: Legacy field name
- **WHEN** a `nats.ConsumerConfig` literal has `Heartbeat: 5 * time.Second` and no `DeliverSubject`
- **THEN** the rule reports the heartbeat message

#### Scenario: Rate limit on a pull consumer
- **WHEN** a literal has `RateLimit: 1000` and no `DeliverSubject`
- **THEN** the rule reports the rate limit message

#### Scenario: Tiny request expiry
- **WHEN** a literal has `MaxRequestExpires: 500 * time.Microsecond`
- **THEN** the rule reports the expires message

#### Scenario: Valid pull consumer
- **WHEN** a literal has `Durable: "w"`, `MaxWaiting: 512`, `MaxRequestBatch: 100`, `MaxRequestExpires: time.Second`
- **THEN** the rule reports nothing

#### Scenario: DeliverSubject assigned after the literal
- **WHEN** a function has `cc := nats.ConsumerConfig{Heartbeat: 5 * time.Second}` followed by `cc.DeliverSubject = "d"`
- **THEN** the rule reports nothing for the pull-only checks (the mode is unknown)

#### Scenario: Literal stored without later assignment
- **WHEN** a function has `cc := nats.ConsumerConfig{Heartbeat: 5 * time.Second}` and never assigns a `DeliverSubject`
- **THEN** the rule reports the heartbeat message

### Requirement: Filter subjects are consistent
Mirrors `checkConsumerCfg`. The rule SHALL report: `FilterSubject` non-empty together with a non-empty `FilterSubjects` slice literal with `consumer config: consumer cannot have both FilterSubject and FilterSubjects specified`; a `FilterSubjects` element that is `""` with `consumer config: consumer filter in FilterSubjects cannot be empty`; `FilterSubject` or a `FilterSubjects` element failing `IsValidSubject` with `consumer config: invalid filter subject "<subject>"`; two constant filters (from `FilterSubjects`, or `FilterSubject` combined with them) where either is a subset match of the other (`subjectIsSubsetMatch`) with `consumer config: consumer subject filters cannot overlap`. These checks SHALL also apply to `jetstream.OrderedConsumerConfig.FilterSubjects`.

#### Scenario: Both filter fields
- **WHEN** a literal has `FilterSubject: "a"` and `FilterSubjects: []string{"b"}`
- **THEN** the rule reports the both-specified message

#### Scenario: Empty filter element
- **WHEN** a literal has `FilterSubjects: []string{"orders.*", ""}`
- **THEN** the rule reports the empty filter message

#### Scenario: Overlapping filters
- **WHEN** a literal has `FilterSubjects: []string{"orders.*", "orders.new"}`
- **THEN** the rule reports the overlap message

#### Scenario: Ordered consumer overlap
- **WHEN** a `jetstream.OrderedConsumerConfig` literal has `FilterSubjects: []string{"a.>", "a.b.c"}`
- **THEN** the rule reports the overlap message

#### Scenario: Disjoint filters
- **WHEN** a literal has `FilterSubjects: []string{"orders.new", "orders.paid"}`
- **THEN** the rule reports nothing

#### Scenario: Invalid filter subject
- **WHEN** a literal has `FilterSubject: "orders..new"`
- **THEN** the rule reports the invalid filter message

### Requirement: Deliver policy agrees with start options
Mirrors the `DeliverPolicy` switch in `checkConsumerCfg` (`badStart`, `notSet`). With `DeliverPolicy` absent meaning `DeliverAllPolicy`, the rule SHALL report: `DeliverAllPolicy`, `DeliverLastPolicy`, `DeliverNewPolicy` or `DeliverLastPerSubjectPolicy` with `OptStartSeq > 0` or with `OptStartTime` present and not `nil`, with `consumer config: consumer delivery policy is deliver <all|last|new|last per subject>, but optional start <sequence|time> is also set`; `DeliverByStartSequencePolicy` with `OptStartSeq` absent or `0` with `consumer config: consumer delivery policy is deliver by start sequence, but optional start sequence is not set`, or with `OptStartTime` set with the `badStart` wording; `DeliverByStartTimePolicy` with `OptStartTime` absent or `nil` with `consumer config: consumer delivery policy is deliver by start time, but optional start time is not set`, or with `OptStartSeq != 0` with the `badStart` wording (verbatim from the server, including its doubled word: `... but optional start start sequence is also set`); `DeliverLastPerSubjectPolicy` with neither `FilterSubject` nor `FilterSubjects` with `consumer config: consumer delivery policy is deliver last per subject, but optional filter subject is not set`. These checks SHALL also apply to `jetstream.OrderedConsumerConfig`.

#### Scenario: Start sequence without the policy
- **WHEN** a literal has `OptStartSeq: 10` and no `DeliverPolicy`
- **THEN** the rule reports `consumer config: consumer delivery policy is deliver all, but optional start sequence is also set`

#### Scenario: Policy without the start sequence
- **WHEN** a literal has `DeliverPolicy: jetstream.DeliverByStartSequencePolicy` and no `OptStartSeq`
- **THEN** the rule reports the not-set message

#### Scenario: Start time policy with a time
- **WHEN** a literal has `DeliverPolicy: jetstream.DeliverByStartTimePolicy` and `OptStartTime: &t`
- **THEN** the rule reports nothing

#### Scenario: Last per subject without a filter
- **WHEN** a literal has `DeliverPolicy: jetstream.DeliverLastPerSubjectPolicy` and no filter fields
- **THEN** the rule reports the filter not-set message

#### Scenario: Ordered consumer start sequence
- **WHEN** a `jetstream.OrderedConsumerConfig` literal has `DeliverPolicy: jetstream.DeliverNewPolicy` and `OptStartSeq: 5`
- **THEN** the rule reports the deliver-new badStart message

### Requirement: Sample frequency parses
Mirrors `checkConsumerCfg`. The rule SHALL report a constant `SampleFrequency` that, after trimming one trailing `%`, is not a non-negative decimal integer, with `consumer config: failed to parse consumer sampling configuration`.

#### Scenario: Non-numeric sampling
- **WHEN** a literal has `SampleFrequency: "half"`
- **THEN** the rule reports the message

#### Scenario: Percentage forms
- **WHEN** a literal has `SampleFrequency: "50%"` or `SampleFrequency: "50"`
- **THEN** the rule reports nothing

### Requirement: Flow control requires heartbeats
Mirrors `checkConsumerCfg`. The rule SHALL report `FlowControl: true` with `IdleHeartbeat` absent or `0` with `consumer config: consumer with flow control also needs heartbeats`.

#### Scenario: Flow control without heartbeat
- **WHEN** a literal has `DeliverSubject: "d"` and `FlowControl: true` and no `IdleHeartbeat`
- **THEN** the rule reports the message

#### Scenario: Flow control with heartbeat
- **WHEN** a literal has `DeliverSubject: "d"`, `FlowControl: true`, `IdleHeartbeat: time.Second`
- **THEN** the rule reports nothing for this check

### Requirement: Durable and Name agree
Mirrors `checkConsumerCfg`. The rule SHALL report `Durable` and `Name` both non-empty constants and different, with `consumer config: Consumer Durable and Name have to be equal if both are provided`.

#### Scenario: Mismatch
- **WHEN** a literal has `Durable: "a"` and `Name: "b"`
- **THEN** the rule reports the message

#### Scenario: Equal or one empty
- **WHEN** a literal has `Durable: "a"` and `Name: "a"`, or only one of them
- **THEN** the rule reports nothing

### Requirement: Priority policy and groups agree
Mirrors `checkConsumerCfg` (`validGroupName` = `^[a-zA-Z0-9/_=-]{1,16}$`). With `PriorityPolicy` absent meaning `PriorityPolicyNone`, the rule SHALL report: a non-none `PriorityPolicy` with a non-empty `DeliverSubject` with `consumer config: priority groups can not be used with push consumers`; a non-none `PriorityPolicy` with `PriorityGroups` absent or empty with `consumer config: Setting PriorityPolicy requires at least one PriorityGroup to be set`; a `PriorityGroups` element that is `""` with `consumer config: Group name cannot be an empty string`; an element not matching `validGroupName` with `consumer config: Valid priority group name must match A-Z, a-z, 0-9, -_/=)+ and may not exceed 16 characters`; `PriorityPolicyNone` with `PriorityGroups` non-empty with `consumer config: consumer can not have priority groups when policy is none`; `PriorityPolicyNone` with `PinnedTTL > 0` with `consumer config: PinnedTTL cannot be set when PriorityPolicy is none`.

#### Scenario: Policy without groups
- **WHEN** a literal has `PriorityPolicy: jetstream.PriorityPolicyOverflow` and no `PriorityGroups`
- **THEN** the rule reports the requires-group message

#### Scenario: Groups without policy
- **WHEN** a literal has `PriorityGroups: []string{"a"}` and no `PriorityPolicy`
- **THEN** the rule reports the policy-none message

#### Scenario: Invalid group name
- **WHEN** a literal has `PriorityPolicy: jetstream.PriorityPolicyPinned` and `PriorityGroups: []string{"this-name-is-way-too-long"}`
- **THEN** the rule reports the group name message

#### Scenario: Valid priority config
- **WHEN** a literal has `PriorityPolicy: jetstream.PriorityPolicyPinned`, `PriorityGroups: []string{"gold", "silver"}`, `PinnedTTL: time.Minute`
- **THEN** the rule reports nothing

### Requirement: Flow-control ack policy constraints
Mirrors the `AckPolicy == AckFlowControl` block of `checkConsumerCfg`. When `AckPolicy` is `AckFlowControlPolicy` the rule SHALL report: `DeliverSubject` absent or `""` with `consumer config: flow control ack policy requires a push based consumer`; `FlowControl` absent or `false` with `consumer config: flow control ack policy requires flow control`; `IdleHeartbeat` absent or not exactly `1s` with `consumer config: flow control ack policy heartbeat needs to be 1s`; `MaxAckPending` absent or `<= 0` with `consumer config: flow control ack policy requires max ack pending`; `AckWait != 0` or `BackOff` non-empty with `consumer config: flow control ack policy requires unset ack wait`; `MaxDeliver > 0` with `consumer config: flow control ack policy requires unset max deliver`.

#### Scenario: Flow-control ack on a pull consumer
- **WHEN** a literal has `AckPolicy: jetstream.AckFlowControlPolicy` and no `DeliverSubject`
- **THEN** the rule reports the push-required message

#### Scenario: Valid flow-control ack consumer
- **WHEN** a literal has `AckPolicy: jetstream.AckFlowControlPolicy`, `DeliverSubject: "d"`, `FlowControl: true`, `IdleHeartbeat: time.Second`, `MaxAckPending: 1000`
- **THEN** the rule reports nothing
