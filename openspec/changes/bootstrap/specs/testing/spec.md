## Purpose

How natsvet proves its rules: unit tests that compile against the real nats.go API, golden-file tests for suggested fixes, a drift test for generated tables, and a corpus run over pinned real-world repositories that gates on false positives.

## ADDED Requirements

### Requirement: Rule tests compile against the real nats.go API
Every rule SHALL have a test package under `testdata/<rule>` that imports the pinned nats.go and is checked with `// want` comments. Test packages SHALL resolve nats.go from a `testdata` module, not from hand-written stubs, so that a rule test that references a nonexistent field or method fails to compile.

#### Scenario: Rule test runs in module mode
- **WHEN** `make test` is run
- **THEN** the pinned nats.go is downloaded for the `testdata` module first and every rule's `// want` expectations are checked against diagnostics produced on real nats.go types

#### Scenario: Stale API reference in a test package
- **WHEN** a test package references a nats.go symbol that does not exist in the pinned version
- **THEN** the rule test fails with a compile error rather than silently reporting nothing

### Requirement: Suggested fixes are verified against golden files
A rule that offers suggested fixes SHALL have a golden file per test source file, and the test SHALL fail when applying the fixes does not reproduce the golden file exactly.

#### Scenario: Fix output matches golden
- **WHEN** the `headerkey` test applies its fixes to `testdata/headerkey/*.go`
- **THEN** the result equals the corresponding `.golden` file byte for byte

### Requirement: Generated tables cannot drift from the pinned nats.go
A test SHALL regenerate the header and legacy symbol tables from the nats.go version pinned in the `testdata` module and fail when the committed tables differ.

#### Scenario: nats.go bumped without regeneration
- **WHEN** the `testdata` module's nats.go requirement is raised to a version that adds a `Nats-` constant and the tables are not regenerated
- **THEN** the drift test fails and names the added constant

### Requirement: Corpus run gates on unreviewed findings
A corpus script SHALL clone the repositories and commits listed in `scripts/corpus.txt` (one entry per line: repository URL, commit, optional path filter), run natsvet with every default-on and opt-in rule enabled over each except inventory rules (`legacyjs`), normalize the output by stripping the clone location and sorting, and diff it against `scripts/corpus.expected`. The run SHALL fail when the diff is non-empty. Every line in the expected file SHALL be tagged `TP` (a real defect in that repository) or `FP` with a one-line reason (an accepted false positive). Inventory rules SHALL be run separately and only their per-repository diagnostic counts printed, never diffed.

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

#### Scenario: Inventory rule counts
- **WHEN** the corpus run completes
- **THEN** it prints the number of `legacyjs` diagnostics per repository and those lines are absent from both the diff and `corpus.expected`

### Requirement: Continuous integration runs the full check set
CI SHALL run `gofmt -l`, `go vet`, `staticcheck`, `misspell -locale US`, and `go test ./...` (after the `testdata` download) on the two newest Go releases, and SHALL run the corpus script as a separate job.

#### Scenario: Formatting drift
- **WHEN** a committed `.go` file is not gofmt-formatted
- **THEN** the CI check fails naming the file

#### Scenario: Corpus job isolation
- **WHEN** the corpus job fails
- **THEN** the unit-test job's result is unaffected and reported independently
