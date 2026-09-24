# release Specification

## Purpose

What a tagged natsvet release guarantees, once tags exist: the checks that must pass before a tag is created, what the binary says about itself, and what a user gets from go install at the tag. Until the first tag, the same gates apply to main.

## Requirements

### Requirement: A tag is only created when every gate passes
A release tag `vX.Y.Z` SHALL be created only from a commit on which `make lint`, `make test`, the generated-table drift test, the corpus run, and the README-matches-registry check all pass, and on which CI is green for the two newest Go releases.

#### Scenario: Corpus failing
- **WHEN** the corpus run reports an untriaged finding on the candidate commit
- **THEN** no tag is created until the finding is triaged into `corpus.expected` or the rule is fixed

#### Scenario: All gates green
- **WHEN** every gate passes on the candidate commit and CI for that commit is green
- **THEN** the maintainer tags it `vX.Y.Z` and pushes the tag

### Requirement: Installing at a tag yields the tagged rules
`go install github.com/piotrpio/natsvet/cmd/natsvet@vX.Y.Z` SHALL produce a binary whose `-version` output names `vX.Y.Z` and whose rule set is exactly the README rule table at that tag.

#### Scenario: Install at a tag
- **WHEN** a user runs `go install github.com/piotrpio/natsvet/cmd/natsvet@vX.Y.Z` and then `natsvet -version`
- **THEN** the output contains `vX.Y.Z`, and `natsvet -h` lists exactly the rules in the README at that tag

#### Scenario: Install from main before any tag
- **WHEN** a user runs `go install github.com/piotrpio/natsvet/cmd/natsvet@latest` and then `natsvet -version`
- **THEN** the output contains the pseudo-version and commit of `main`, and `natsvet -h` lists exactly the rules in the current README

### Requirement: Rule documentation is generated from the analyzers
`docs/rules.md` SHALL be generated from the registered analyzers' `Doc` strings — for every rule its name, default state, whether it offers fixes, and the `Doc` text with its before/after snippet — and a test SHALL fail when the committed file differs from a fresh generation. The README SHALL keep a one-line-per-rule table that links to `docs/rules.md`, name the corpus repositories, and state that `main` passes the corpus with every finding triaged.

#### Scenario: Doc string edited without regeneration
- **WHEN** an analyzer's `Doc` changes and `docs/rules.md` is not regenerated
- **THEN** the drift test fails naming the rule

#### Scenario: Rule table matches the registry
- **WHEN** the README rule table and `docs/rules.md` are compared with `Analyzers()` and `OptIn()`
- **THEN** the sets of rule names are identical in both

### Requirement: A tag also yields the golangci-lint plugin
A `.custom-gcl.yml` that names the natsvet module and plugin import path with `version: vX.Y.Z` SHALL build, with the golangci-lint version pinned in the repository's own `.custom-gcl.yml` at that tag, a custom binary whose `natsvet` rule set is exactly the README rule table at that tag. The README SHALL document that form next to `go install`.

#### Scenario: Plugin at a tag
- **WHEN** a user builds a custom golangci-lint binary with `version: vX.Y.Z` and enables `natsvet` with `settings.enable` naming every opt-in rule
- **THEN** the findings over a package match `natsvet` at that tag with the same rules enabled

#### Scenario: Plugin from main before any tag
- **WHEN** a user builds with `version: main`
- **THEN** the binary contains the rule set of the current README
