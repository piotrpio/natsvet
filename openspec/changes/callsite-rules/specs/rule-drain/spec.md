## Purpose

drain reports Close called right after Drain on the same connection, including a deferred Close that runs when the function returns from a trailing Drain. Drain returns as soon as it has started the drain in a goroutine and the documentation says to use the ClosedHandler to learn when it finishes; a Close immediately after it closes the connection and discards the in-progress drain.

## ADDED Requirements

### Requirement: Close immediately after Drain
Mirrors `Conn.Drain` (`go nc.drainConnection(); return nil`) and `drainConnection` (returns once the connection is closed). The rule SHALL report an expression or assignment statement whose call is `<x>.Drain()` on a `*nats.Conn`, when the next statement in the same block — allowing one intervening `if` statement that does not call a method on `<x>` — is an expression statement `<x>.Close()` on the same variable. The message SHALL be `Close immediately after Drain aborts the drain; wait for the ClosedHandler instead`, reported at the `Close` call. Category `drain`. No fix.

#### Scenario: Adjacent
- **WHEN** code has `nc.Drain()` followed by `nc.Close()`
- **THEN** the rule reports the message at `nc.Close()`

#### Scenario: Error check between
- **WHEN** code has `if err := nc.Drain(); err != nil { return err }` or `err := nc.Drain()` + `if err != nil { log.Fatal(err) }`, followed by `nc.Close()`
- **THEN** the rule reports the message

#### Scenario: Wait between
- **WHEN** code has `nc.Drain()`, then `<-done`, then `nc.Close()`
- **THEN** the rule reports nothing

#### Scenario: Different connections
- **WHEN** code has `a.Drain()` followed by `b.Close()`
- **THEN** the rule reports nothing

### Requirement: Deferred Close after a trailing Drain
The rule SHALL report a `<x>.Drain()` call that is the last statement of a function body, the expression of a `return` statement, or immediately followed by a `return`, when the same function body contains `defer <x>.Close()` on the same variable, with `deferred Close runs as soon as Drain returns and aborts the drain; wait for the ClosedHandler before returning`, reported at the `Drain` call. Functions named `main` in package `main`, and `Test*`, `Benchmark*` and `Fuzz*` functions in `_test.go` files, SHALL be exempt: there the process exit dominates, which is a separate question.

#### Scenario: Deferred Close with trailing Drain
- **WHEN** a function has `defer nc.Close()` and ends with `return nc.Drain()`
- **THEN** the rule reports the deferred message at the `Drain` call

#### Scenario: Deferred Close with Drain then wait
- **WHEN** a function has `defer nc.Close()`, then `nc.Drain()`, then `<-done`, then returns
- **THEN** the rule reports nothing

#### Scenario: main is exempt
- **WHEN** `func main()` has `defer nc.Close()` and ends with `nc.Drain()`
- **THEN** the rule reports nothing
