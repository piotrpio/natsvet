## MODIFIED Requirements

### Requirement: Miscased known header keys are reported
The rule SHALL report a constant string key that equals a known `Nats-*` header case-insensitively but not exactly, at every key site the key-site requirement defines. A known header is one in the generated nats.go header table or in the pinned nats-server header list. Reaching the header through `msg.Header`, `jetstream.Msg.Headers()` or `micro.Request.Headers()` SHALL make no difference. The message SHALL be `header key "<key>" does not match "<Canonical>"; nats.go header lookups are case-sensitive`.

#### Scenario: Lowercase key on nats.Header.Get
- **WHEN** code calls `msg.Header.Get("nats-msg-id")`
- **THEN** the rule reports `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`

#### Scenario: Key via jetstream message
- **WHEN** code calls `m.Headers().Get("NATS-STREAM")` on a `jetstream.Msg`
- **THEN** the rule reports the key against `Nats-Stream`

#### Scenario: Key via micro request
- **WHEN** code calls `req.Headers().Get("nats-service-error")` on a `micro.Request`
- **THEN** the rule reports the key against `Nats-Service-Error`

#### Scenario: Direct map index
- **WHEN** code evaluates `h["nats-msg-id"]` where `h` is a `nats.Header`
- **THEN** the rule reports the key

#### Scenario: Named constant key
- **WHEN** code declares `const id = "nats-msg-id"` and calls `h.Get(id)`
- **THEN** the rule reports the key at the call site

#### Scenario: Server-only header
- **WHEN** code calls `h.Get("nats-num-pending")`, a header nats-server sets and nats.go defines no constant for
- **THEN** the rule reports the key against `Nats-Num-Pending`

#### Scenario: Exact-case known header
- **WHEN** code calls `h.Get("Nats-Msg-Id")` or `h.Get(nats.MsgIdHdr)` or `h.Get("Nats-UpTo-Sequence")`
- **THEN** the rule reports nothing

#### Scenario: User-defined header
- **WHEN** code calls `h.Set("X-My-Header", v)` or `h.Get("x-request-id")`
- **THEN** the rule reports nothing

#### Scenario: Non-constant key
- **WHEN** code calls `h.Get(keyFromConfig)` where the argument is not a compile-time constant
- **THEN** the rule reports nothing

### Requirement: The fix replaces the key with a certain equivalent
The rule SHALL offer exactly one suggested fix per case diagnostic and none per near-miss diagnostic. The replacement SHALL be the qualified constant from a package that is already imported in the file and defines a constant for the header, preferring `jetstream`, then `nats`, then `micro`; when no imported package defines one (including every header only nats-server defines), the correctly cased string literal. The fix SHALL replace only the key expression and SHALL never add an import.

#### Scenario: jetstream imported
- **WHEN** the file imports `github.com/nats-io/nats.go/jetstream` and calls `h.Get("nats-msg-id")`
- **THEN** the fix rewrites the argument to `jetstream.MsgIDHeader`

#### Scenario: Only nats imported
- **WHEN** the file imports only `github.com/nats-io/nats.go` and calls `h.Get("nats-msg-id")`
- **THEN** the fix rewrites the argument to `nats.MsgIdHdr`

#### Scenario: Both imported
- **WHEN** the file imports both packages
- **THEN** the fix uses the `jetstream` constant

#### Scenario: Header has no constant in the imported package
- **WHEN** the file imports only `github.com/nats-io/nats.go` and calls `h.Get("nats-schedule")`, for which only `jetstream` defines a constant
- **THEN** the fix rewrites the argument to the literal `"Nats-Schedule"`

#### Scenario: Server-only header
- **WHEN** the file imports `jetstream` and calls `h.Get("nats-num-pending")`
- **THEN** the fix rewrites the argument to the literal `"Nats-Num-Pending"`

#### Scenario: micro header with micro imported
- **WHEN** the file imports `github.com/nats-io/nats.go/micro` and calls `req.Headers().Get("nats-service-error")`
- **THEN** the fix rewrites the argument to `micro.ErrorHeader`

#### Scenario: Named constant key is not rewritten at its declaration
- **WHEN** the key is a named constant declared elsewhere
- **THEN** the fix replaces the argument expression at the call site and leaves the constant declaration untouched

#### Scenario: Near-miss has no fix
- **WHEN** the rule reports `h.Get("Nats-UpTo-Sequnce")` as a near-miss
- **THEN** the diagnostic carries no suggested fix

## ADDED Requirements

### Requirement: Header keys are examined at every site that names a header
The rule SHALL examine a constant key at each of these sites: the first argument of `Get`, `Set`, `Add`, `Values` or `Del` on `nats.Header` and of `Get` or `Values` on `micro.Headers`; the index of an index expression on a `nats.Header` value; a key of a `nats.Header{...}` or `micro.Headers{...}` composite literal; and the other operand of `==` or `!=`, or an expression in a `case` clause of a `switch` on, the key variable of a `for k := range h` (or `for k, v := range h`) where `h` is a `nats.Header` or `micro.Headers` and the loop body never assigns `k`. The diagnostic SHALL be reported at the key expression. A comparison against the range value variable, or against a key variable the loop body reassigns, SHALL NOT be examined.

#### Scenario: Range key comparison
- **WHEN** code has `for k := range msg.Header { if k == "nats-msg-id" { … } }`
- **THEN** the rule reports the case diagnostic at `"nats-msg-id"` with its fix

#### Scenario: Reversed operands and inequality
- **WHEN** code has `for k, vs := range h { if "nats-stream" != k { … } }` with `h` a `nats.Header`
- **THEN** the rule reports the case diagnostic

#### Scenario: Switch on the range key
- **WHEN** code has `for k := range h { switch k { case "Nats-Subject", "nats-stream": … } }`
- **THEN** the rule reports only `"nats-stream"`

#### Scenario: Header literal key
- **WHEN** code writes `nats.Header{"nats-msg-id": []string{"1"}}`
- **THEN** the rule reports the case diagnostic at the key with its fix

#### Scenario: Reassigned key variable
- **WHEN** code has `for k := range h { k = strings.ToLower(k); if k == "nats-msg-id" { … } }`
- **THEN** the rule reports nothing

#### Scenario: Plain map range
- **WHEN** code ranges over a `map[string][]string` that is not a `nats.Header` and compares the key with `"nats-msg-id"`
- **THEN** the rule reports nothing

### Requirement: Keys within edit distance 2 of a NATS header are reported
The rule SHALL report a constant key at a key site that starts with `Nats-` in any letter case, is not a known header in any letter case, and whose case-insensitive Levenshtein distance to some known header is at most 2. The diagnostic SHALL name the known header at the smallest distance, the lexically first on a tie, with `header key "<key>" is not a header NATS sets or reads; the closest NATS header is "<Known>"`. No SuggestedFix is offered.

#### Scenario: Missing letter
- **WHEN** code has `for k := range msg.Header { if k == "Nats-UpTo-Sequnce" { … } }`
- **THEN** the rule reports `header key "Nats-UpTo-Sequnce" is not a header NATS sets or reads; the closest NATS header is "Nats-UpTo-Sequence"`

#### Scenario: Missing separator
- **WHEN** code calls `h.Set("Nats-MsgId", id)`
- **THEN** the rule reports the key with closest header `Nats-Msg-Id`

#### Scenario: Custom header far from every known one
- **WHEN** code calls `h.Set("Nats-Has-More", "1")`
- **THEN** the rule reports nothing

#### Scenario: Short key at distance 3
- **WHEN** code calls `h.Set("Nats-X", "x")`
- **THEN** the rule reports nothing

#### Scenario: Close key without the prefix
- **WHEN** code calls `h.Set("Data-TTL", "1")`
- **THEN** the rule reports nothing

### Requirement: A NATS header with letters or digits appended is reported
The rule SHALL report a constant key at a key site that starts with `Nats-` in any letter case, is not a known header in any letter case, is not reported by the edit-distance requirement, and begins, case-insensitively, with a known header followed directly by an ASCII letter or digit. The diagnostic SHALL name the longest such known header, with the edit-distance requirement's message. No SuggestedFix is offered.

#### Scenario: Unit glued onto a header
- **WHEN** code calls `msg.Header.Add("Nats-TTLSeconds", "30s")`
- **THEN** the rule reports `header key "Nats-TTLSeconds" is not a header NATS sets or reads; the closest NATS header is "Nats-TTL"`

#### Scenario: Dash-separated extension
- **WHEN** code calls `h.Set("Nats-TTL-Seconds", "30")`
- **THEN** the rule reports nothing

#### Scenario: Known header that extends another
- **WHEN** code calls `h.Get("Nats-Scheduler")`
- **THEN** the rule reports nothing
