# Changelog

## Unreleased

- JWT and Casbin documentation-path allowlists now match exact paths or path-segment prefixes; empty custom paths no longer allow every request.
- Legacy plugin callbacks are isolated so one panic is reported without preventing other plugins from being checked or closed.
- Job shutdown is bounded through `StopWithContext`; handlers receive cancellation, and a timeout does not falsely mark the manager stopped.
- Plugin shutdown follows the set that initialized successfully, even if a plugin is disabled later.
- Invalid utility inputs (nil RSA keys, malformed DES ciphertext, invalid slice indexes, nil reflection inputs, uncomparable cache keys, and corrupt ring-buffer indexes) now return safely instead of panicking.
- `Values.GetAll` returns a synchronized snapshot; linked-list callbacks run outside the list lock; limit acquisition observes request cancellation.
- SQLite closes its underlying SQL connection even when the GORM handle carries an earlier operation error. OTel typed-nil providers use the no-op fallback.
- App startup rejects repeated `Run` calls, and SafeGo reports panics from before-hooks through structured logging.
- App shutdown no longer creates an orphan health-check waiter, and listener startup failures now enter the shutdown path instead of waiting indefinitely for a signal.
- Etcd deregistration waits for its lease-renewal worker before deleting the instance key; Etcd/Nacos/Consul renewal panic paths are reported and supervised.
- `CmdRunWithTimeout` rejects nil or unstarted commands safely and no longer needs a second waiter goroutine after timeout.
- Logsink queue capacity is consistently documented as a default/maximum of 1024; oversized configuration falls back to that limit.
- Job store initialization performs database I/O outside its coordination mutex while concurrent callers share one initialization result.
