## ADDED Requirements

### Requirement: Package-wide symbol use
The framework SHALL provide a way to ask whether any file of the analyzed package uses a given nats.go function or method (identified by package path, receiver and name, as for the call-site helpers), so that a rule can treat a handler installed or a completion awaited in another function of the same package as evidence the author handles a lifecycle. The answer SHALL cover every file the driver hands the rule for that package, including `_test.go` files when the package under analysis is a test variant, and SHALL NOT cover other packages.

#### Scenario: Function used elsewhere in the package
- **WHEN** one file of the package calls `jetstream.WithPublishAsyncErrHandler(h)` and the lookup asks for that function
- **THEN** it answers true from any file of the package

#### Scenario: Method used through an interface
- **WHEN** one file calls `js.PublishAsyncComplete()` on a `jetstream.JetStream` value and the lookup asks for `Publisher.PublishAsyncComplete` (the interface `JetStream` embeds that declares it)
- **THEN** it answers true

#### Scenario: Not used
- **WHEN** no file of the package refers to the symbol
- **THEN** it answers false

#### Scenario: Used only in a dependency
- **WHEN** a package the analyzed one imports uses the symbol, and the analyzed package does not
- **THEN** it answers false
