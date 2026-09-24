## 1. Plugin package

- [x] 1.1 Write `plugin/plugin_test.go`: registration via `register.GetPlugin("natsvet")`, a table over settings (`nil`, `enable: [legacyjs]`, `disable: [drain]`, both, unknown name in `enable`, unknown name in `disable`, unknown key `enabled`, a default-on rule in `enable`, an opt-in rule in `disable`) checked against returned analyzer names and each opt-in analyzer's `enable` flag, and `GetLoadMode() == register.LoadModeTypesInfo`; verify the test fails to build before the package exists
- [x] 1.2 Add `github.com/golangci/plugin-module-register` to `go.mod` (`go get` from the module cache, `go mod tidy`) and implement `plugin/plugin.go` per design.md (strict settings decode, name validation, explicit flag set on every opt-in analyzer, `Analyzers()` minus `disable` plus enabled `OptIn()`); verify `go test ./plugin/` passes and `go build ./cmd/natsvet` does not link the register package (`go version -m bin/natsvet | grep -c plugin-module-register` is 0)

## 2. Build configuration and parity

- [x] 2.1 Add `.custom-gcl.yml` at the repository root (`version: v2.13.2`, `name: custom-gcl`, `destination: ./bin`, the module with `import: github.com/piotrpio/natsvet/plugin` and `path: .`); verify `golangci-lint custom` (installed at v2.13.2, outside the sandbox) produces `bin/custom-gcl` and `bin/custom-gcl version` runs
- [x] 2.2 Add `testdata/.golangci.yml` per design.md (natsvet only, every opt-in rule in `settings.enable`, no issue caps, no uniq-by-line, plain text output without linter name or issued lines, no stats, `relative-path-mode: cfg`); verify `bin/custom-gcl config verify` accepts it from `testdata` and `bin/custom-gcl run ./...` there reports `handle: ...`, `legacyjs: ...` and `drain: ...` lines
- [x] 2.3 Write `scripts/plugin.sh` (version check against the pin, custom build, both runs, normalization, sorted diff) and the Makefile target `plugin: build`; verify `make plugin` exits 0 on the clean tree, then temporarily remove one rule from the plugin's returned list and verify it exits non-zero listing that rule's findings, then restore
- [x] 2.4 Add the `plugin` CI job (`actions/setup-go` 1.26, `golangci/golangci-lint-action` with `version: v2.13.2` and `install-only: true`, `make plugin`); verify the workflow parses (`actionlint` if available, otherwise a push to a branch) and the job is independent of `test` and `corpus`

## 3. Docs

- [x] 3.1 Add the golangci-lint subsection to the README Use section (user `.custom-gcl.yml` with `version`, `golangci-lint custom`, the `.golangci.yml` block, a sample `<rule>: <message> (natsvet)` line, the `//nolint:natsvet` note, `main` until a tag exists); verify the YAML in the README is byte-identical to what `bin/custom-gcl config verify` accepts
- [x] 3.2 Update `docs/design.md`: §2.1 second production dependency and why, §5 item 4 plugin shipped / upstream waiting on the move, §6 item 6 done; verify the status line and §6 progress paragraph name this change
- [x] 3.3 Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, `misspell -locale US .`, `make lint`, `make test` (with the testdata download step), `make corpus` and `make plugin`; verify all pass with no output from gofmt and misspell
