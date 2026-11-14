# CLAUDE.md

Querra is a lightweight desktop database client (PostgreSQL, MySQL/MariaDB, SQLite) for macOS 12+ and Windows 10/11, built with **Wails v3** (Go backend + OS webview) and React/TypeScript. It's an internal tool for personal use and the owner's business (balikaryadigital.com), shared with a few close friends. It isn't sold, so there is no licensing, no feature gating, and no paid code signing.

- Identity: Go module `github.com/cokagung30/querra`, bundle ID `com.balikaryadigital.querra`.
- Requirements: `docs/PRD Querra – Database Management Client.md` (Indonesian)
- Architecture, milestones, decisions: `PLAN.md` (Indonesian). Section refs like "PLAN §3.7" point there.
- Status: Phase 0 (M0.1–M0.5) is in place, and Windows ships through the Microsoft Store; PLAN.md tracks what's done and what still awaits manual checks. Folders under Layout that don't exist yet get created by the milestone that needs them.

## Working agreement

- Work milestone by milestone (`M0.1`, `M1.2`, …) in PLAN.md order. Don't build a later milestone's features unless asked.
- When a milestone task lands, tick its checkbox in PLAN.md. If the implementation deviates from PLAN.md's architecture, update PLAN.md (and this file) in the same change.
- Wails v3 only went stable in October 2026 (pinned at v3.1.0), and most examples online are for v2 or v3 alphas. Before writing Wails-specific code, check the current docs (Context7 library `/llmstxt/v3_wails_io_llms_txt`) or the module source under `$(go env GOPATH)/pkg/mod/github.com/wailsapp/wails/v3@v3.1.0`. Never use Wails v2 APIs (`wails.Run`, `options.App`, `runtime.EventsEmit`, `wails.json`).
- Language: PLAN.md and the PRD are Indonesian; code, comments, commit messages, and UI strings are English.
- Commit messages: Conventional Commits with the milestone code, e.g. `feat(grid): M1.7 server-side pagination`.
- Manual and cross-platform checks live in `docs/test/<milestone>.md` (Indonesian, one file per milestone, a result column per platform; the flow is in `docs/test/README.md`). The owner develops on both Windows and macOS and continues work on the other machine through these files. When they say they're testing or continuing on another platform, read the milestone's file first, especially its "Konteks untuk Claude" section. Record reported results there, and tick PLAN.md only once every platform passes.
- Plugin skills (Superpowers, mattpocock-skills, …) run inside this workflow. PLAN.md and the PRD are the spec and the plan: design decisions and milestone plans go there, never into separate files such as `docs/superpowers/`. Work in the main checkout on `main`, and commit only when the owner asks. Brainstorm only what PLAN.md leaves open, and put those questions to the owner.

## Stack

Go 1.26 · Wails v3.1.0 · pgx v5 via `database/sql` · go-sql-driver/mysql · modernc.org/sqlite (pure Go, also used for the app's local store) · golang.org/x/crypto/ssh · zalando/go-keyring · log/slog
React 19 + TypeScript + Vite 8 · Tailwind CSS v4 + shadcn/ui (Radix, Nova preset) · Zustand · TanStack Query · TanStack Table + TanStack Virtual · CodeMirror 6 + `@codemirror/lang-sql` · react-resizable-panels · i18next · Vitest · npm

Libraries are added by the milestone that first needs them, so not all of these are installed yet.

## Commands

```bash
# one-time setup (keep the CLI version in sync with go.mod)
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
export PATH="$PATH:$(go env GOPATH)/bin"
go env -w GOTOOLCHAIN=go1.26.2+auto  # go.mod's toolchain line is only a minimum; a newer local Go would win
# Windows packaging needs NSIS: winget install NSIS.NSIS, then add "C:\Program Files (x86)\NSIS" to PATH
# Windows db:* tasks run dev/testdb.sh with bash: add "C:\Program Files\Git\bin" to PATH (or run them from Git Bash)
wails3 doctor

wails3 dev                       # run the app with hot reload
wails3 build                     # production build for the current OS
wails3 task test                 # Go unit tests + frontend Vitest
wails3 task lint                 # golangci-lint (pinned, via go run), eslint, prettier --check, tsc
wails3 task db:reset             # recreate + seed querra_test* on the local servers in dev/test.env (no Docker)
wails3 task test:integration     # Go integration tests (needs db:reset)
wails3 task db:drop              # remove querra_test* from the local servers
wails3 task bindings             # regenerate frontend/bindings after changing bound methods/structs
wails3 task darwin:package:dmg   # macOS .dmg; run `wails3 task --list` for the other packaging tasks

# focused runs
go test ./internal/sqlscan -run 'TestSplit/dollar_quote' -v
go test -tags=integration ./internal/driver/postgres -run TestIntrospect -v
cd frontend && npx vitest run src/features/grid
```

## Layout

```text
main.go               Wails bootstrap: services, windows, menus
internal/app/         Wails services (the only package besides main that imports Wails)
internal/driver/      Driver/Conn/Session/Introspector/Dialect interfaces + postgres/ mysql/ sqlite/
internal/<domain>/    session, query, sqlscan, resultset, changeset, export, importer,
                      connection, store (+ migrations/), secrets, settings, sshtunnel, apperr
frontend/bindings/    GENERATED by wails3. Never edit by hand
frontend/src/         app/ (shell), features/<feature>/, components/ui/ (shadcn), lib/, stores/, i18n/
dev/                  seed SQL, testdb.sh, test.env.example for the integration-test databases
dev/perf/             startup/memory/grid measurement scripts (macOS Python, Windows PowerShell)
docs/test/            manual test checklists per milestone, with results per platform
internal/perf/, internal/app/perfservice.go, frontend/src/features/perf/
                      dev/perf autorun hooks (QUERRA_PERF_AUTORUN=startup|idle|grid); inert without
                      the env var, so release builds are what gets measured. Keep them working
build/                Wails build config, icons, Info.plist, NSIS, signing
.github/workflows/    CI: test + build on macOS and Windows; lint + stale generated-files check on macOS only;
                      the Windows job uploads exe + NSIS installer as artifact `querra-windows`, which
                      is how Windows manual checks get a build (docs/test/README.md)
```

## Architecture rules

Dependencies point one way: frontend → `frontend/bindings` → `internal/app` → domain packages → `internal/driver/<engine>`.

1. **Wails stays at the edge.** Only `main.go` and `internal/app/` import `github.com/wailsapp/wails/v3`. Domain packages are plain Go, so `go test ./internal/...` runs anywhere, including Linux CI.
2. **Services are thin.** `internal/app` validates input, calls domain code, and maps errors. No SQL and no business logic there.
3. **Engine-specific code lives only in `internal/driver/<engine>`.** If you need `if engine == postgres` anywhere else, extend `Dialect`, `Introspector`, or `Capabilities` instead. The UI branches on `Capabilities`, never on engine name.
4. **SQL we generate is parameterized.** Identifiers go through `Dialect.QuoteIdent`; values always go in as bind parameters. Never `fmt.Sprintf` a value into SQL. (SQL the user types in the editor runs as-is, which is the product, but it always passes through `sqlscan` safety checks.)
5. **Context everywhere.** Every bound method that touches a database takes `ctx context.Context` first and passes it all the way down. That ctx is how the frontend cancels a call (`CancellablePromise.cancel()`). No `context.Background()` on a request path.
6. **Results never cross the bridge whole.** `Execute` returns summaries plus a `resultID`; the grid pulls windows with `FetchRows(resultID, offset, limit)`. Table browsing uses server-side LIMIT/OFFSET pages. Don't add an API that returns an unbounded row set.
7. **Cell values cross the bridge as `string | null`** (`*string` in Go), never as JS numbers, to protect int64, unsigned BIGINT, and DECIMAL precision. Type info travels in column metadata (`kind`, native DB type). See PLAN §3.5.
8. **Safety is enforced in Go; the UI only renders it.** That covers read-only mode (applied at the DB session level and checked in Go), destructive-statement detection in `sqlscan`, and the Production `confirmation_required` → `confirmToken` flow (PLAN §3.7). Never add a frontend-only bypass. When the scanner is unsure, treat the statement as destructive.
9. **Sessions.** Each SQL editor tab owns one pinned `driver.Session`. The grid, explorer, and export use the pool. After a dropped connection, never re-run a statement automatically; surface that session state was lost.
10. **Errors.** Return `*apperr.Error` (exported fields `Code`, `Message`, `Detail`, `Hint`, `Position`); the frontend receives it as `error.cause`. When you add a code, add it to `internal/apperr` and the frontend i18n in the same change. Goroutines started by services must `recover()`. A failed query must never crash the app.

## Secrets and privacy

- Passwords, SSH passwords, and key passphrases go to the OS keychain only, via `internal/secrets` (service `com.balikaryadigital.querra`, account `conn/<id>/<kind>`). Never put them in SQLite, logs, exports, error messages, or test fixtures.
- After saving, a secret is never sent back to the frontend; it only gets `hasPassword`-style flags.
- Store SSH private key **paths**, not key contents (Windows Credential Manager has a small size limit).
- Logs must not contain secrets, DSNs with passwords, or row data.
- `PRIVACY.md` is the privacy policy the Microsoft Store listing links to. When a change affects what Querra stores, logs or sends (query history, updater, export), update it in the same change.
- No network traffic except to the DB/SSH hosts the user configured, plus the update check (Wails v3 updater). The update check sends only version and platform, and the user can turn it off. No telemetry.
- The updater signing key (`updater.key`) lives only in CI secrets and an offline backup. Never commit it; only `updater.key.pub` is embedded in the app.
- Tests use the in-memory fake from `internal/secrets` (`secrets.NewMemory()`), never the real keychain. The one exception is the opt-in smoke test (`go test -tags keychain ./internal/secrets -run TestKeychainSmoke -v`). It writes dummy values under the separate service `com.balikaryadigital.querra.smoke-test` and deletes them, and it's run by hand once per platform.
- `dev/test.env` holds local admin credentials for the test database servers. It's git-ignored: never commit it, print it, or copy its values anywhere else.

## Go conventions

- gofmt/goimports clean and golangci-lint clean.
- Wrap errors with context (`fmt.Errorf("introspect tables: %w", err)`); convert to `apperr` at the service boundary, not deeper.
- Prefer table-driven tests. Integration tests carry `//go:build integration` and read DSNs from the env vars set up for `dev/`.
- No package-level mutable state except the driver registry.
- Wails service lifecycle: `ServiceStartup(ctx context.Context, opts application.ServiceOptions) error` and `ServiceShutdown() error`. `ServiceShutdown` takes **no** ctx; a version with a ctx parameter is silently never called.
- Bound structs use camelCase JSON tags. After changing a bound method signature or struct, run `wails3 task bindings` and commit the regenerated files.

## Frontend conventions

- Feature code lives in `src/features/<feature>/`; shared primitives go in `src/components/ui/` (add them with `npx shadcn add <name>`, which uses the pinned local CLI). Merge classes with `cn` from `@/lib/utils`.
- Components never import `frontend/bindings` directly; they go through `api.ts` wrappers (`features/<feature>/api.ts`, or `app/api.ts` for the shell). That keeps generated paths in one place and lets tests mock the backend. ESLint enforces this with `no-restricted-imports`.
- State: TanStack Query for data that comes from Go (connections, schema metadata); Zustand for UI, tab, and pending-change state. Don't copy Query data into Zustand.
- Every user-facing string goes through i18next `t('…')`, with keys in `src/i18n/en.json`. Keys are type-checked (`src/i18n/i18next.d.ts`), so a missing key fails `tsc`.
- Every user action is registered as a command in `src/lib/commands` with an id, a `titleKey`, and a default keybinding (`Mod` = Cmd on macOS, Ctrl on Windows); buttons run commands through `commands.execute(id)` or `CommandButton`. No ad-hoc `keydown` listeners: `installKeybindings` (installed once in `main.tsx`) is the only one, and keybindings must stay user-remappable. Shell commands live in `src/app/commands.ts`, whose test rejects duplicate defaults and keys the OS or the macOS menu takes.
- The macOS native menu (`internal/app/menu.go`) holds only App, Edit and Window. Don't add native menu items with accelerators for app actions: a native accelerator wins over the webview, so the key could no longer be remapped.
- Colors come from theme tokens (CSS variables / Tailwind theme), including the environment colors. Anywhere a connection appears, a Production connection must be visibly marked.
- Lazy-load heavy modules (CodeMirror, grid) to protect startup time.

## Before saying a task is done

- `wails3 task lint` and `wails3 task test` pass.
- On a Windows machine with Smart App Control on, Windows can refuse to run unsigned Go-built binaries: test binaries, and also `wails3` and golangci-lint ("An Application Control policy has blocked this file"). That is not a test result, and rebuilding with other flags to slip past it is not a fix. Run what does start (`go vet`, `go build`, npm scripts), leave the rest to CI or the Mac, and say which ran where. Keep Smart App Control on: it can't be re-enabled without reinstalling Windows, and that PC is how releases get checked against it (PLAN R10).
- If you touched drivers, session, query, changeset, export, or importer, run `wails3 task db:reset` and then `wails3 task test:integration`. Engines without a HOST in `dev/test.env` are skipped (locally there's no MariaDB). Say which engines actually ran, and leave the rest to CI's `integration` job; don't claim an engine was tested when it was skipped.
- Integration tests get their connection from `QUERRA_DEV_<ENGINE>_HOST`/`_PORT` and log in as `querra_test`/`querra_test` to database `querra_test`. They must `t.Skip` when the HOST is empty, and must never create or modify anything outside `querra_test*`.
- If you changed the UI, run `wails3 dev` and exercise the change. Say explicitly which platform wasn't verified, and put the steps for it in `docs/test/<milestone>.md` (copy `docs/test/_template.md` for a new milestone).
- New `sqlscan` rules need test cases, including the tricky ones: keywords inside strings or comments, `WITH … DELETE`, dollar-quoted bodies, MySQL `#` comments.

## Performance budgets (PRD §7)

Startup < 2 s · idle memory < 250 MB (including webview processes) · smooth scrolling at 100k rows · installer < 60 MB. Call out any change likely to hurt these, such as a heavy new dependency, eager loading, or an unbounded fetch.

## Gotchas

- WKWebView (macOS) and WebView2 (Windows) behave differently, so avoid experimental web APIs. Tailwind v4 targets Safari 16.4+, which matters for macOS 12 (PLAN R3).
- MySQL: cancelling the ctx in go-sql-driver closes the connection, but the query keeps running on the server. The adapter cancels with `KILL QUERY` from another connection instead (PLAN §3.8). `mysql.RegisterDialContext` and `mysql.RegisterTLSConfig` are global registries, so use a unique name per connection.
- PostgreSQL: one pool per database. Switching databases means opening a new pool.
- PostgreSQL adapter (`internal/driver/postgres`): the password goes on the pgx config, never into a connection string. Queries run in `QueryExecModeDescribeExec` (no statement cache, so it works behind PgBouncer and after DDL, and string arguments are typed by the server). Each editor session has its own one-connection pool, so a session's connection never goes back to the shared pool with its `SET`s or open transaction. Cell text comes from `Codec`; pgx already returns most types as PostgreSQL's own text, so keep that text rather than reformatting it.
- PostgreSQL cancellation: pgx's default context handler kills the connection on cancel. Always set `BuildContextWatcherHandler` to `pgconn.CancelRequestContextWatcherHandler` (with a `DeadlineDelay` fallback) so editor sessions survive a cancelled query (PLAN §3.8, measured in `docs/spikes/fase-0.md`).
- Connection errors: an adapter's `Open` marks login failures with `driver.ErrAuthFailed`, `ErrDatabaseNotFound`, `ErrPermissionDenied` and `ErrTLSRefused` (`driver.Mark`), and `apperr.FromConnect` maps those, plus the standard DNS, network, timeout and TLS errors, to PLAN §3.9 codes. A new adapter (MySQL, SQLite) that skips the marks makes Test Connection report a wrong password as `internal`. On Windows a refused connection is WSAECONNREFUSED (10061), not `syscall.ECONNREFUSED`.
- `go-keyring` on macOS shells out to `/usr/bin/security`. There are no keychain prompts after updates, but any process of the same user can read the secrets. That trade-off is deliberate for v1 (`docs/spikes/fase-0.md` §7); don't swap in a native keychain library without revisiting that decision.
- Node 25+ defines its own `localStorage` global, which is undefined unless Node runs with `--localstorage-file`, and it hides jsdom's in Vitest. Reach webview storage through `uiStorage` (`src/lib/storage.ts`), which falls back to memory. Keep it to UI state such as panel sizes; real settings go to the Go store.
- Keep the build cgo-free on Windows: modernc.org/sqlite is pure Go, so don't introduce cgo-based drivers.
- Local store (`internal/store`): never edit a shipped migration; add `migrations/NNNN_name.sql` with the next number. A table belongs to the migration of the milestone that first needs it. `PRAGMA user_version` is the schema version, and `Open` refuses a database from a newer Querra (`ErrNewerSchema`). Pass the path to modernc.org/sqlite as a plain file name, not a `file:` URI, because Windows user folders often contain spaces.
- Release builds are ad-hoc signed on macOS (not notarized), so Gatekeeper warnings are expected. Windows releases go through the Microsoft Store as MSIX, which Microsoft signs for free (PLAN §8.1, spike M0.5): unsigned Windows builds get blocked outright by Smart App Control, even after they once ran. The unsigned NSIS installer remains only as a CI artifact. Don't add build or release steps that assume a paid signing identity unless PLAN §8 changes.
- `build/windows/msix/app_manifest.xml` is hand-maintained and carries the Store identity from Partner Center (`BaliDigitalKarya.Querra`, its `CN=…` publisher, "Bali Digital Karya"); it must match Partner Center exactly. `common:update:build-assets` overwrites it, so restore it from git afterwards. Each Store submission needs a higher `Identity/@Version` whose last part stays 0.
- In the Store (MSIX) build, Windows redirects writes under `%AppData%` to `%LocalAppData%\Packages\BaliDigitalKarya.Querra_m596njqrn6kpj\LocalCache\Roaming\`, so `querra.db` and the WebView2 profile live there, and uninstall deletes them. To confirm the Store app really launched or was blocked without taking screenshots, read the `Microsoft-Windows-AppModel-Runtime/Admin` log (event 201 "Created process … Querra") and `Microsoft-Windows-CodeIntegrity/Operational`.
- Pin Wails and the Go toolchain. New Go releases can raise the minimum macOS version, and the PRD requires macOS 12.
- `go.mod` has `ignore ./frontend/node_modules`, because some npm packages ship Go code. Keep it, or `./...` will test, lint, and `go mod tidy` code inside node_modules.
- `main.go` embeds `frontend/dist`, so Go builds, tests, and lint fail on a fresh clone until the frontend has been built. The `lint` and `test` tasks build it first; if you call `go test` directly, run `wails3 task common:build:frontend` once.
- `frontend/bindings` is generated with `-ts -i` (TypeScript interfaces) and committed. CI fails if `frontend/bindings`, `go.mod`, or `go.sum` change during a build.
- After editing `build/config.yml`, `wails3 task common:update:build-assets` regenerates the platform assets but also recreates `build/ios` and `build/linux`. Delete those again, since we only ship macOS and Windows, and check `git diff build/` for icon or DMG assets it may have reset.
- App icons come from `build/icon-src/logo-original.png`. To change them, run `python3 build/icon-src/make_icons.py` (needs Pillow), then `wails3 task common:generate:icons`; the script's docstring lists every output. macOS gets the logo on a squircle (`appicon.png`, `appicon.icon`); Windows gets the bare logo (`icon-src/logo.png`). With `wails3 generate icons`, pass `-windowsfilename ""` or `-macfilename ""` to skip an output, because both default to real paths and will silently overwrite `build/windows/icon.ico` or the `.icns`.
- After `wails3 task darwin:package:universal`, `bin/querra` is a universal (lipo) binary and the next native `go build` refuses to overwrite it. Delete `bin/querra` first.
