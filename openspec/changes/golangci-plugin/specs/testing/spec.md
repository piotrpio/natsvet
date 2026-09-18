## MODIFIED Requirements

### Requirement: Continuous integration runs the full check set
CI SHALL run `gofmt -l`, `go vet`, `staticcheck`, `misspell -locale US`, and `go test ./...` (after the `testdata` download) on the two newest Go releases, SHALL run the corpus script as a separate job, and SHALL run the golangci-lint plugin parity script as a separate job that installs the golangci-lint version pinned in `.custom-gcl.yml`.

#### Scenario: Formatting drift
- **WHEN** a committed `.go` file is not gofmt-formatted
- **THEN** the CI check fails naming the file

#### Scenario: Corpus job isolation
- **WHEN** the corpus job fails
- **THEN** the unit-test job's result is unaffected and reported independently

#### Scenario: Plugin parity job
- **WHEN** a change makes the custom golangci-lint binary report differently from `natsvet` over the `testdata` module
- **THEN** the plugin job fails independently of the unit-test and corpus jobs, printing the differing lines
