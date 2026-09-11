## Purpose

kvconfig reports KeyValue and ObjectStore bucket names, history limits and keys that nats.go rejects client-side with ErrInvalidBucketName, ErrInvalidStoreName, ErrHistoryTooLarge or ErrInvalidKey. The checks are the ones in nats.go's jetstream/kv.go and jetstream/object.go (bucketValid, keyValid, searchKeyValid, KeyValueMaxHistory) and fire only on constants, so a report is a guaranteed runtime error.

## ADDED Requirements

### Requirement: Bucket names match the client's bucket regex
Mirrors `bucketValid` in `jetstream/kv.go` (`^[a-zA-Z0-9_-]+$`) and the same check in `jetstream/object.go` (`ErrInvalidStoreName`). The rule SHALL report a constant `Bucket` field of a `jetstream.KeyValueConfig`, `nats.KeyValueConfig`, `jetstream.ObjectStoreConfig` or `nats.ObjectStoreConfig` literal that is empty or contains a character outside `[a-zA-Z0-9_-]`, and a constant bucket argument to `KeyValue`, `ObjectStore`, `DeleteKeyValue` and `DeleteObjectStore` on `jetstream.JetStream` or legacy `nats.JetStreamContext`, with `invalid bucket name "<bucket>": bucket names may only contain [a-zA-Z0-9_-]`. Category `kvconfig`.

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
