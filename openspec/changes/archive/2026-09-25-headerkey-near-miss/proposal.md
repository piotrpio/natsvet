## Why

`headerkey` catches `nats-msg-id` for `Nats-Msg-Id`, but not a key that is *almost* a NATS header. `docs/design.md` §8 item 9 recorded one such bug (go-choria sets `Nats-TTLSeconds`; the server header is `Nats-TTL`, so its gossip messages never expire) and proposed a Levenshtein threshold of 2–3. Measured against the corpus, that proposal misses its own motivating case: `Nats-TTLSeconds` is edit distance 7 from `Nats-TTL`. A grep of every `"Nats-…"` literal in the nine pinned corpus repositories, nats-server's tests included, against the headers nats-server, nats.go and orbit.go define leaves exactly four unknown keys:

| Key | Where | What it is |
|---|---|---|
| `Nats-UpTo-Sequnce` | natscli `cli/stream_command.go:1291` | typo of `Nats-UpTo-Sequence` (distance 1); `stream view` prints the header it meant to hide |
| `Nats-TTLSeconds` | go-choria `aagent/watchers/gossipwatcher/gossip.go:197` | `Nats-TTL` with `Seconds` glued on; the TTL is never applied |
| `Nats-Has-More` | synadia-io/connect `client/transport.go:13` | connect's own protocol header, legitimate (distance 6 from any known header) |
| `Nats-X` | go-choria `submission/disk_spool_test.go:185` | test fixture for go-choria's reserved-prefix check, not a nats.go header site (distance 3) |

Two shapes catch both bugs and neither legitimate key: edit distance ≤ 2 from a known header, and a known header followed directly by more letters or digits. No ADR reserves the `Nats-` prefix, and connect uses it, so "any unknown `Nats-*` key" is not a finding. The natscli bug also shows a key site `headerkey` does not see: a comparison against the key variable of `for k, v := range msg.Header`.

## What Changes

- `headerkey` (default-on, extended) reports a constant key that starts with `Nats-` (in any case), is not a known header in any case, and is either within edit distance 2 of a known header or a known header followed directly by a letter or digit. The message names the closest known header. No fix: which header was meant is a guess, however likely.
- The known-header set grows from the generated nats.go table to that table plus every `Nats-*` header name in the pinned nats-server's non-test source (46 at the v2.14.7 pin; `Nats-UpTo-Sequence`, `Nats-Num-Pending`, `Nats-Trace-Dest`, … have no nats.go constant). The existing case check uses the same set, so `h.Get("nats-num-pending")` is now reported too; its fix is the correctly cased literal, as for any header without a constant in an imported package.
- New key sites for both checks: a constant compared with `==`/`!=` or listed in a `switch` case against the key variable of a `range` over a `nats.Header` or `micro.Headers`, and the keys of a `nats.Header{…}` / `micro.Headers{…}` literal. The case fix applies at these sites too.
- The nats-server header list is a generated table, `internal/natsapi/server_headers_table.go`, written by `tablegen -server <nats-server-dir>` (from `$NATS_SERVER_DIR`, else the corpus clone; skipped with a message when neither exists). The corpus job regenerates it from the pinned nats-server clone into a temporary directory and fails on any difference, the way `TestTablesUpToDate` pins the nats.go tables to the testdata module.
- Corpus run and triage: the natscli and go-choria sites become `TP` lines with issue drafts.
- `docs/rules.md` regenerated; `docs/design.md` §3.1 `headerkey` gains the near-miss check and §8 item 9 is closed with the corrected analysis.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `rule-headerkey`: the case requirement uses the combined known set and the new key sites; the fix requirement distinguishes case diagnostics (one fix) from near-miss diagnostics (none); new requirements for the two near-miss shapes and for the key sites.
- `analyzer-framework`: new requirement that the nats-server header list is derived from the pinned nats-server source and checked by the corpus job.

## Non-goals

- Keys that do not start with `Nats-`. `Data-TTL` is distance 2 from `Nats-TTL` and is somebody's header.
- A known header with a dash-separated extension (`Nats-TTL-Seconds`). That is how custom headers extend a namespace, and nothing in the corpus does it by mistake.
- Keys built at run time, including `fmt.Sprintf("Nats-%s", …)`.
- `strings.EqualFold` and other case-insensitive comparisons. They already match any case, and they are rare enough that the near-miss check can wait for a real one.
- `docs/design.md` §8 item 5 (a `Get` literal that differs in case from a `Set` literal elsewhere in the package). It still has no evidence.
- A fix for near-miss keys.

## Rule defaults

- `headerkey`: default-on (unchanged).

## Shared helpers

Added to `internal/tablegen`: a server mode that parses nats-server's non-test sources for header literals. Added to `internal/natsapi`: the generated nats-server header table and a lookup that merges it with the generated nats.go table (`Header` keeps its signature; a server-only header returns no constants). Reused: `ConstString`, `Callee`, `IsMethod`, `IsPkg`. Range-key detection (the identifier is the key of an enclosing `for k := range h` over a header type and the loop body never assigns it) and the edit-distance function stay in `analyzers/headerkey`; no other rule needs them. `SingleDefinition` does not fit: it deliberately treats a range clause as an assignment.

## Impact

- New `headerkey` diagnostics where a key is near a NATS header, and case diagnostics for server-only headers. Existing diagnostics and fixes are unchanged.
- A generated `internal/natsapi/server_headers_table.go` and a `-server` input to `tablegen`; `scripts/corpus.sh` gains a check that runs against the nats-server clone it already fetches.
- Corpus: two new `TP` lines expected (natscli, go-choria), no `FP`.
- No new dependencies (`go/parser` is in the standard library).
