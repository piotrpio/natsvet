## ADDED Requirements

### Requirement: The binary reports its version
`natsvet -version` SHALL print the module version and, when available, the VCS revision and modification state from the binary's build information, and exit 0 without running any analysis.

#### Scenario: Installed at a tag or pseudo-version
- **WHEN** the binary was built by `go install ...@latest` (or at a tag once one exists) and `natsvet -version` is run
- **THEN** the output contains the module version Go recorded: the pseudo-version of `main`, or the tag

#### Scenario: Built from a checkout
- **WHEN** the binary was built with `go build` in a git checkout
- **THEN** the output contains the commit revision and `(modified)` when the tree was dirty

#### Scenario: Analysis flags untouched
- **WHEN** `natsvet -V=full` is run
- **THEN** the `go vet` cache-key output is unchanged
