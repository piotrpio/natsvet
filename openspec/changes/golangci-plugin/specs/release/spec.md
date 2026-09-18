## ADDED Requirements

### Requirement: A tag also yields the golangci-lint plugin
A `.custom-gcl.yml` that names the natsvet module and plugin import path with `version: vX.Y.Z` SHALL build, with the golangci-lint version pinned in the repository's own `.custom-gcl.yml` at that tag, a custom binary whose `natsvet` rule set is exactly the README rule table at that tag. The README SHALL document that form next to `go install`.

#### Scenario: Plugin at a tag
- **WHEN** a user builds a custom golangci-lint binary with `version: vX.Y.Z` and enables `natsvet` with `settings.enable` naming every opt-in rule
- **THEN** the findings over a package match `natsvet` at that tag with the same rules enabled

#### Scenario: Plugin from main before any tag
- **WHEN** a user builds with `version: main`
- **THEN** the binary contains the rule set of the current README
