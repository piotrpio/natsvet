# rule-kvconfig Specification

## Purpose

kvconfig reports KeyValue and ObjectStore bucket names, history limits and keys that nats.go rejects client-side with ErrInvalidBucketName, ErrInvalidStoreName, ErrHistoryTooLarge or ErrInvalidKey. The checks are the ones in nats.go's jetstream/kv.go and jetstream/object.go (bucketValid, keyValid, searchKeyValid, KeyValueMaxHistory) and fire only on constants, so a report is a guaranteed runtime error. One check is the server's: nats.go gives a bucket's stream the subject $KV.<bucket>.> and passes RePublish through, so a republish destination that overlaps that subject forms a cycle and CreateKeyValue fails.

## Requirements

### Requirement: Bucket names match the client's bucket regex
Mirrors `bucketValid` in `jetstream/kv.go` (`^[a-zA-Z0-9_-]+$`) and the same check in `jetstream/object.go` (`ErrInvalidStoreName`). The rule SHALL report a constant `Bucket` field of a `jetstream.KeyValueConfig`, `nats.KeyValueConfig`, `jetstream.ObjectStoreConfig` or `nats.ObjectStoreConfig` literal that is empty or contains a character outside `[a-zA-Z0-9_-]`, and a constant bucket argument to `KeyValue`, `ObjectStore`, `DeleteKeyValue` and `DeleteObjectStore` on `jetstream.JetStream` or legacy `nats.JetStreamContext`, with `invalid bucket name "<bucket>": bucket names may only contain [a-zA-Z0-9_-]`. Category `kvconfig`. For config literals, fields assigned before the literal's variable is handed off SHALL be unknown, under the same rule as the other config rules.

#### Scenario: Dotted bucket in config
- **WHEN** code writes `jetstream.KeyValueConfig{Bucket: "my.bucket"}`
- **THEN** the rule reports `invalid bucket name "my.bucket": bucket names may only contain [a-zA-Z0-9_-]`

#### Scenario: Bucket argument to lookup
- **WHEN** code calls `js.KeyValue(ctx, "my bucket")`
- **THEN** the rule reports the message

#### Scenario: Object store config
- **WHEN** code writes `nats.ObjectStoreConfig{Bucket: "files/2024"}`
- **THEN** the rule reports the message

#### Scenario: Valid bucket
- **WHEN** code writes `jetstream.KeyValueConfig{Bucket: "user-profiles_v2"}`
- **THEN** the rule reports nothing

#### Scenario: Non-constant bucket
- **WHEN** code writes `jetstream.KeyValueConfig{Bucket: name}` with `name` a variable
- **THEN** the rule reports nothing

### Requirement: History within the maximum
Mirrors `prepareKeyValueConfig` in `jetstream/kv.go` (`KeyValueMaxHistory` = 64; values `<= 0` default to 1 and are accepted). The rule SHALL report a constant `History > 64` in a `jetstream.KeyValueConfig` or `nats.KeyValueConfig` literal with `KV history <n> exceeds the maximum of 64`.

#### Scenario: Too much history
- **WHEN** code writes `jetstream.KeyValueConfig{Bucket: "b", History: 100}`
- **THEN** the rule reports `KV history 100 exceeds the maximum of 64`

#### Scenario: Maximum and default
- **WHEN** code writes `History: 64`, `History: 0`, or omits `History`
- **THEN** the rule reports nothing

### Requirement: Keys are valid
Mirrors `keyValid` and `searchKeyValid` in `jetstream/kv.go` (`^[-/_=\.a-zA-Z0-9]+$` and `^[-/_=\.a-zA-Z0-9*]*[>]?$`; both reject empty keys, a leading or trailing `.`, and `..`). The rule SHALL report a constant key argument to `Get`, `GetRevision`, `Put`, `PutString`, `Create`, `Update`, `Delete` and `Purge` on `jetstream.KeyValue` or legacy `nats.KeyValue` that fails `keyValid` with `invalid KV key "<key>": keys may only contain [-/_=.a-zA-Z0-9]`; and a constant key argument to `Watch` and `History`, or a constant element of the slice literal passed to `WatchFiltered` or the variadic arguments of `ListKeysFiltered`, that fails `searchKeyValid` with `invalid KV key filter "<key>": filters may only contain [-/_=.a-zA-Z0-9*] and a trailing >`. Non-constant keys SHALL be skipped.

#### Scenario: Key with a space
- **WHEN** code calls `kv.Put(ctx, "user name", v)`
- **THEN** the rule reports `invalid KV key "user name": keys may only contain [-/_=.a-zA-Z0-9]`

#### Scenario: Leading dot
- **WHEN** code calls `kv.Get(ctx, ".hidden")`
- **THEN** the rule reports the key message

#### Scenario: Double dot
- **WHEN** code calls `kv.Delete(ctx, "a..b")`
- **THEN** the rule reports the key message

#### Scenario: Wildcard in a plain key
- **WHEN** code calls `kv.Put(ctx, "users.*", v)`
- **THEN** the rule reports the key message

#### Scenario: Wildcard in a watch
- **WHEN** code calls `kv.Watch(ctx, "users.*")` or `kv.WatchFiltered(ctx, []string{"users.>", "orders.*"})`
- **THEN** the rule reports nothing

#### Scenario: Invalid watch filter
- **WHEN** code calls `kv.Watch(ctx, "users.>.x")`
- **THEN** the rule reports the filter message

#### Scenario: Legacy KeyValue
- **WHEN** code calls `kv.PutString("bad key", "v")` on a `nats.KeyValue`
- **THEN** the rule reports the key message

#### Scenario: Valid keys
- **WHEN** code calls `kv.Put(ctx, "users/42=profile.v1", v)` and `kv.Watch(ctx, ">")`
- **THEN** the rule reports nothing

#### Scenario: Non-constant key
- **WHEN** code calls `kv.Put(ctx, key, v)` with `key` a variable
- **THEN** the rule reports nothing

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
