## MODIFIED Requirements

### Requirement: One binary serves every invocation mode
The same binary SHALL report identical diagnostics when run directly, through `go vet -vettool`, and through `go fix -fixtool`, and SHALL apply suggested fixes with `-fix` and print them with `-diff`. It SHALL also run the migration planner when its first argument is `migrate`, without changing how any of the other modes parse their arguments or what they report.

#### Scenario: go vet delegation
- **WHEN** `go vet -vettool=$(which natsvet) ./...` is run on a package with a known finding
- **THEN** the same diagnostic text and position are reported as with `natsvet ./...`

#### Scenario: Fix application
- **WHEN** `natsvet -fix ./...` is run on a package where a rule offers a suggested fix
- **THEN** the source file is rewritten with the fix and a subsequent run reports nothing for that site

#### Scenario: Migrate subcommand
- **WHEN** `natsvet migrate plan ./...` or `natsvet migrate skill` is run
- **THEN** the migration planner runs, or the skill is printed, and no analyzer runs

#### Scenario: Analyzer modes unaffected
- **WHEN** `natsvet ./...`, `natsvet -legacyjs.enable ./...` or `go vet -vettool=$(which natsvet) ./...` is run after the subcommand is added
- **THEN** each reports exactly what it reported before
