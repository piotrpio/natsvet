## MODIFIED Requirements

### Requirement: Corpus run gates on unreviewed findings
A corpus script SHALL clone the repositories and commits listed in `scripts/corpus.txt` (one entry per line: optional `modfile=<file>` option, repository URL, commit, optional directory, optional package patterns), run natsvet with every default-on and opt-in rule enabled over each except inventory rules (`legacyjs`), normalize the output by stripping the clone location and sorting, and diff it against `scripts/corpus.expected`. The run SHALL fail when the diff is non-empty. Every line in the expected file SHALL be tagged `TP` (a real defect in that repository) or `FP` with a one-line reason (an accepted false positive). Inventory rules SHALL be run separately and only their per-repository diagnostic counts printed, never diffed.

#### Scenario: New finding after a rule change
- **WHEN** a rule change produces a diagnostic on a corpus repository that is not in `corpus.expected`
- **THEN** the corpus run fails and prints the new line

#### Scenario: Triaged finding
- **WHEN** a line in `corpus.expected` is tagged `FP` with a reason and the tool still reports it
- **THEN** the corpus run passes

#### Scenario: Finding disappears
- **WHEN** a rule change stops reporting a line present in `corpus.expected`
- **THEN** the corpus run fails so the expected file is updated deliberately

#### Scenario: Initial corpus
- **WHEN** the corpus list is first committed
- **THEN** it contains pinned commits of nats.go (`examples/` and `test/`), natscli, nack, nex, and every per-module `test/` directory of orbit.go

#### Scenario: Release corpus
- **WHEN** the corpus list is at or after the `v0.1.0` tag
- **THEN** it additionally contains pinned commits of nats-server (`server/` and `test/`), go-choria, synadia-io/connect and eventing-natss, and every finding on them is triaged

#### Scenario: Inventory rule counts
- **WHEN** the corpus run completes
- **THEN** it prints the number of `legacyjs` diagnostics per repository and those lines are absent from both the diff and `corpus.expected`
