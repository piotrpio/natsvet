## ADDED Requirements

### Requirement: Known nats-server header list is pinned to the corpus nats-server
The tool SHALL carry a list of the header names nats-server uses: every string literal of the form `Nats-<letters, digits and dashes>` that does not end in `-`, found in the non-test Go files of the `server` directory of the nats-server commit pinned in the corpus. The list SHALL be generated from a nats-server checkout, not maintained by hand, and checked by the corpus run, which SHALL regenerate it from the pinned clone and fail on any difference. Regenerating the tables without a nats-server checkout SHALL leave the committed list unchanged and say so. Header lookups SHALL merge this list with the generated nats.go header table; a header only in the list SHALL have no constants.

#### Scenario: Server-only header
- **WHEN** the lookup is queried for `nats-upto-sequence`
- **THEN** it returns the canonical `Nats-UpTo-Sequence` with no constants

#### Scenario: Prefix literal excluded
- **WHEN** the server source contains the prefix literal `"Nats-Expected-"`
- **THEN** it is not in the list

#### Scenario: Pinned server adds a header
- **WHEN** the corpus pin moves to a nats-server commit that uses a new `Nats-*` header and the list is not regenerated
- **THEN** the corpus run fails naming the missing header

#### Scenario: No nats-server checkout
- **WHEN** the tables are regenerated on a machine with no nats-server checkout
- **THEN** the nats.go tables are regenerated, the committed nats-server list is left unchanged, and the generator reports that it skipped it

#### Scenario: Header in both sources
- **WHEN** the lookup is queried for `nats-msg-id`
- **THEN** it returns `Nats-Msg-Id` with both nats.go constants, as before
