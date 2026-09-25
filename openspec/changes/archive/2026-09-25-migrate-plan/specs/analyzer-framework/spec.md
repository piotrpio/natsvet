## MODIFIED Requirements

### Requirement: One binary serves every invocation mode
The same binary SHALL report identical diagnostics when run directly, through `go vet -vettool`, and through `go fix -fixtool`, and SHALL apply suggested fixes with `-fix` and print them with `-diff`. It SHALL also run the migration planner when its first argument is `migrate`, without changing how any of the other modes parse their arguments or what they report, and SHALL name the `migrate` subcommand in every help it prints, since agents learn what a tool does from its help.

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

#### Scenario: Help names the migrate subcommand
- **WHEN** `natsvet -h`, `natsvet help` or `natsvet` without arguments is run
- **THEN** the output lists `natsvet migrate plan` and `natsvet migrate skill` next to the analyzer modes, `-h` still lists every analyzer flag, and `natsvet help migrate` describes the planner and its flags
