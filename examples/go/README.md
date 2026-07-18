# Serify Go Example

Shows how to write a serify worker in Go using `Suite` + `Type`.
Field mapping (schema ↔ struct) is automatic via `serify:` tags or snake_case;
binary encoding is explicit.

Go is the `--ref` language for the example suite, so the bytes these types
produce define the layout every other worker has to reproduce.

| File              | What it is                                                                        |
| ----------------- | --------------------------------------------------------------------------------- |
| `main.go`         | The worker: the type/format registration, and nothing else.                         |
| `customer.go`     | `CustomerRecord`, `Address`, `Money` — plus their binary and JSON layouts.           |
| `ledger.go`       | `LedgerEntry`, whose two int128 fields need `math/big`.                             |
| `notification.go` | `NotificationRecord` and the `Channel` sum type, bridged by a `serify.Converter`.    |
| `wire.go`         | The layout conventions every worker in every language reproduces, and the helpers for them. |
