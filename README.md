# Querra

A lightweight desktop database client for PostgreSQL, MySQL/MariaDB and SQLite on macOS and Windows, built with [Wails v3](https://v3.wails.io) (Go) and React.

> **Status:** early development (milestone M0). Not usable yet.

Querra is a personal tool, also used at balikaryadigital.com. The source is public for reference, but it is **not open source**: all rights reserved, and it is not for sale.

## Documents

- [`docs/PRD Querra – Database Management Client.md`](docs/) – product requirements (Indonesian)
- [`PLAN.md`](PLAN.md) – architecture and milestones (Indonesian)
- [`CLAUDE.md`](CLAUDE.md) – development conventions

## Development

Requirements: Go 1.26, Node.js 24+, and the Wails v3 CLI at the version pinned in `go.mod`. Integration tests use PostgreSQL/MySQL/MariaDB servers you already run (no Docker): copy `dev/test.env.example` to `dev/test.env` and fill it in.

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0   # make sure $(go env GOPATH)/bin is on PATH
wails3 doctor                # check system dependencies
wails3 dev                   # run with hot reload
wails3 task lint             # golangci-lint, ESLint, Prettier, tsc
wails3 task test             # Go + frontend unit tests
wails3 task db:reset         # create + seed the querra_test databases
wails3 task test:integration
wails3 task db:drop          # remove them again
wails3 build                 # production build in bin/
```
