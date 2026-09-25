## ADDED Requirements

### Requirement: KeyValue republish does not form a cycle with the bucket subject
Mirrors the republish cycle check in nats-server's `checkStreamCfg` applied to the stream nats.go builds for a bucket: `prepareKeyValueConfig` in `jetstream/kv.go` and legacy `CreateKeyValue` in `kv.go` pass `RePublish` through unchanged and set the stream subjects to `$KV.<bucket>.>` unless the bucket has a `Mirror`. For a `jetstream.KeyValueConfig` or `nats.KeyValueConfig` literal with a constant `Bucket` that passes the bucket check, no `Mirror` (absent or `nil`), and a `RePublish` literal whose `Source` and `Destination` are constants and whose `Destination` is non-empty, the rule SHALL report a destination that collides with `$KV.<bucket>.>` at the `RePublish` literal with `republish destination "<destination>" forms a cycle with the bucket subject "$KV.<bucket>.>"; the server rejects the bucket`. Category `kvconfig`. No SuggestedFix. Fields assigned before the literal's variable is handed off SHALL be unknown, under the same rule as the other config rules.

#### Scenario: Republish back into the bucket
- **WHEN** a literal is `jetstream.KeyValueConfig{Bucket: "orders", RePublish: &jetstream.RePublish{Destination: ">"}}`
- **THEN** the rule reports `republish destination ">" forms a cycle with the bucket subject "$KV.orders.>"; the server rejects the bucket`

#### Scenario: Legacy twin
- **WHEN** a literal is `&nats.KeyValueConfig{Bucket: "orders", RePublish: &nats.RePublish{Source: "$KV.orders.>", Destination: "$KV.orders.copy.>"}}`
- **THEN** the rule reports the cycle message

#### Scenario: Change feed outside the bucket
- **WHEN** a literal is `jetstream.KeyValueConfig{Bucket: "orders", RePublish: &jetstream.RePublish{Source: "$KV.orders.>", Destination: "feed.orders.>"}}`
- **THEN** the rule reports nothing

#### Scenario: Mirrored bucket
- **WHEN** a literal has `Bucket: "orders"`, `Mirror: &jetstream.StreamSource{Name: "orders"}` and `RePublish: &jetstream.RePublish{Destination: ">"}`
- **THEN** the rule does not report the cycle message

#### Scenario: Bucket from a variable
- **WHEN** a literal has `Bucket: name` and `RePublish: &jetstream.RePublish{Destination: ">"}`
- **THEN** the rule reports nothing
