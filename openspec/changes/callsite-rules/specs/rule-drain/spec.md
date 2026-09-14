## Purpose

drain reports Close called right after Drain on the same connection. Drain returns as soon as it has started the drain in a goroutine and the documentation says to use the ClosedHandler to learn when it finishes; a Close immediately after it closes the connection and discards the in-progress drain.

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

#### Scenario: Deferred Close
- **WHEN** code has `defer nc.Close()` earlier and `nc.Drain()` as the last statement
- **THEN** the rule reports nothing (out of scope for this rule)
