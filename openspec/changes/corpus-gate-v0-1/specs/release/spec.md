## Purpose

What a tagged natsvet release guarantees: the checks that must pass before a tag exists, what the binary says about itself, and what a user gets from go install at the tag.

## ADDED Requirements

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

#### Scenario: Install at v0.1.0
- **WHEN** a user runs `go install github.com/piotrpio/natsvet/cmd/natsvet@v0.1.0` and then `natsvet -version`
- **THEN** the output contains `v0.1.0`, and `natsvet -h` lists the eleven rules of the v0.1.0 README

### Requirement: The README documents every rule with an example
The README SHALL contain, for every registered rule, its default state, whether it fixes, and a before/after Go snippet, and SHALL name the corpus repositories and state that a release passes the corpus with every finding triaged.

#### Scenario: Rule table matches the registry
- **WHEN** the README rule table is compared with `Analyzers()` and `OptIn()`
- **THEN** the sets of rule names are identical
