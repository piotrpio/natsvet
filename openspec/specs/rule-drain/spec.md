# rule-drain Specification

## Purpose

drain reports a Drain that Close or process exit cuts short: Close called right after Drain on the same connection, a deferred Close that runs when the function returns from a trailing Drain, and, in main, a deferred Drain or a Drain followed by process exit. Drain returns as soon as it has started the drain in a goroutine and the documentation says to use the ClosedHandler to learn when it finishes; a Close immediately after it closes the connection and discards the in-progress drain, and an exit right after it ends the process before the goroutine has drained anything.

## Requirements

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
The rule SHALL report a `<x>.Drain()` call that is the last statement of a function body, the expression of a `return` statement, or immediately followed by a `return`, when the same function body contains `defer <x>.Close()` on the same variable, with `deferred Close runs as soon as Drain returns and aborts the drain; wait for the ClosedHandler before returning`, reported at the `Drain` call. Functions named `main` in package `main`, and `Test*`, `Benchmark*` and `Fuzz*` functions in `_test.go` files, SHALL be exempt from this requirement: in `main` the trailing `Drain` is reported by the process-exit requirement instead, and in a test the deferred `Close` is ordinary cleanup.

#### Scenario: Deferred Close with trailing Drain
- **WHEN** a function has `defer nc.Close()` and ends with `return nc.Drain()`
- **THEN** the rule reports the deferred message at the `Drain` call

#### Scenario: Deferred Close with Drain then wait
- **WHEN** a function has `defer nc.Close()`, then `nc.Drain()`, then `<-done`, then returns
- **THEN** the rule reports nothing

#### Scenario: main is exempt
- **WHEN** `func main()` has `defer nc.Close()` and ends with `nc.Drain()`
- **THEN** the rule reports no deferred-Close message there; the process-exit requirement reports its own message at that `Drain`

### Requirement: Drain in main followed by process exit
Mirrors `Conn.Drain` (`go nc.drainConnection(); return nil`): the drain runs in a goroutine that process exit kills. In a function named `main` in package `main`, the rule SHALL report a `<x>.Drain()` call on a `*nats.Conn` that is deferred (`defer <x>.Drain()`), that is the last statement of `main`'s body, that is immediately followed by a `return`, or that is immediately followed by an expression statement calling `os.Exit` or a function whose name starts with `Fatal` (`log.Fatal`, `log.Fatalf`, `log.Fatalln`). The message SHALL be `Drain in main followed by process exit drains nothing; Drain returns immediately, wait for the ClosedHandler before exiting`, reported at the `Drain` call. Category `drain`. No fix. `Test*`, `Benchmark*` and `Fuzz*` functions SHALL NOT be reported: their return does not exit the process.

#### Scenario: Deferred Drain in main
- **WHEN** `func main()` in `package main` has `defer nc.Drain()`
- **THEN** the rule reports the message at the `Drain` call

#### Scenario: Drain then log.Fatalf
- **WHEN** `func main()` has `nc.Drain()` followed by `log.Fatalf("Exiting")`
- **THEN** the rule reports the message

#### Scenario: Drain as the last statement of main
- **WHEN** `func main()` ends with `nc.Drain()`
- **THEN** the rule reports the message

#### Scenario: Drain then os.Exit
- **WHEN** `func main()` has `nc.Drain()` followed by `os.Exit(0)`
- **THEN** the rule reports the message

#### Scenario: Drain then wait in main
- **WHEN** `func main()` has `nc.Drain()` followed by `<-done` (closed by a `ClosedHandler`) and then returns
- **THEN** the rule reports nothing

#### Scenario: Drain in a goroutine in main
- **WHEN** `func main()` has `go func() { <-sig; nc.Drain() }()` and then blocks
- **THEN** the rule reports nothing

#### Scenario: Deferred Drain in a test
- **WHEN** a `TestXxx` function has `defer nc.Drain()`
- **THEN** the rule reports nothing

#### Scenario: Deferred Drain in a helper
- **WHEN** a function other than `main` has `defer nc.Drain()`
- **THEN** the rule reports nothing

#### Scenario: Not package main
- **WHEN** a function named `main` in a package other than `main` has `defer nc.Drain()`
- **THEN** the rule reports nothing
