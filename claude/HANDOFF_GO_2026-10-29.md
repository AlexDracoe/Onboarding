# Handoff 2026-10-29: Go port of the drone service (week 2, day 1)

Python reference implementation: `/home/alex/DSETraining/alpha` (frozen MVP, do not add features).
Go port: `/home/alex/DSETraining/beta` (module `beta`, Go 1.27.3 via mise).
React dashboard: `/home/alex/DSETraining/web`. Compose: `/home/alex/DSETraining/docker-compose.yml`.
Older handoffs and the coaching prompt: `/home/alex/DSETraining/alpha/claude/`.

## How to work with the user (read this first)

Follow `/home/alex/DSETraining/alpha/claude/COACHING_PROMPT.md`, **Phase 2 (coaching mode) only**; its Phase 1 cleanup was
done long ago. On top of that:

- **Go syntax first (new for the port).** The user has zero Go experience but strong CS fundamentals and knows C++ well.
  At the start of every task: (1) one-line verdict LEARN / BORROW / SKIP-MINIMAL, (2) Level 0 (goal, concept, questions,
  files, what "done" looks like), (3) a **Go syntax primer** in chat: small standalone snippets (not their solution)
  explaining every construct they have not seen yet, with C++/Python analogies. Only explain items not in the
  "Introduced so far" list below, and update that list in the next handoff.
- Help ladder resets to L0 per task; go up only when asked. "Give me the code to type" = L4 shown in chat: verify it
  first on a scratch copy (scratchpad dir: `go run`, `go vet ./...`, `gofmt -l .`), then show it with "what's new" notes.
- **Never edit the user's files in `beta/`** unless they say "do it for me" / "just make". They type the code.
- "check" = actually run their code (`go run`, `go vet ./...`, `gofmt -l .`, plus drive it with fakes or a scratch DB),
  report the real output, and give "you have / needed / why" blocks for anything wrong.
- Low-value pieces (SQLite store, HTTP plumbing, Docker) Claude may build after a short chat; high-value ones
  (StateManager, AckTracker, VehicleService, later HealthManager, status enum, takeoff sequencing) the user writes.
- Check real library source before calling something BORROW (e.g. gomavlib in `~/go/pkg/mod` after `go get`).
- Use they/them if pronouns are ever needed.
- Note: Claude's auto-memory is stored per working directory. These rules are written here so they survive a chat
  started in `beta/`.

## Status

- **Python eval (2026-10-29):** all five commands verified on real SITL (Docker DB: arm 17, set_mode 12, takeoff 6,
  goto 5 ACHIEVED). Goto ACK name `DO_REPOSITION` confirmed; ArduPilot rejected DO_REPOSITION in COMMAND_LONG with
  `COMMAND_INT_ONLY`, so goto must be sent as **COMMAND_INT**.
- **Go Phase 0 DONE** (checked by running it): `go.mod` (`module beta`, `go 1.27.3`),
  `internal/config/config.go` (`Config` struct with `time.Duration` fields, `Load()` returns by value, private
  `getEnv` via `os.LookupEnv`), `cmd/droned/main.go` prints `%+v`. `go vet` and `gofmt -l` clean. Env overrides
  verified. Only nit: no doc comments on exported names (optional).
- `beta/` is not a git repo yet (suggested `git init` + first commit).

## Introduced so far (do not re-explain)

go mod init / go.mod, go run ./path, go build, go vet ./..., gofmt, gopls; `./...`; package clause, folder = package,
`package main` + `func main()`; import blocks (stdlib group, blank line, own module paths like `beta/internal/config`);
exported = capital first letter; `internal/` is compiler-enforced; unused imports/variables are errors; `func` syntax
(types after names, shared types `a, b int`), multiple return values, `error` last + `nil`; `var` + zero values, `:=` vs
`=`, `const`; structs, keyed composite literals, trailing comma rule, `New`/`Load` constructor convention, return by
value = copy; `if v, ok := ...; ok {}` and the comma-ok idiom; `os.LookupEnv` vs `os.Getenv`; `time.Duration`,
untyped constants, no implicit numeric conversion (`time.Duration(n) * time.Second`); `fmt.Printf` `%v` `%+v` `%s`
`%d` `%.2f`; doc comments.

## Plan (phases)

| # | Phase | Verdict | New Go concepts |
|---|---|---|---|
| 0 | Toolchain, layout, config | done | (see above) |
| 1 | Domain types (`Position`, `Velocity`, `VehicleState`, updates) + `geo.Distance` (haversine, R=6371000) | LEARN (light) | pointer fields for missing values (`*float64` ↔ JSON null), JSON struct tags, `math`, maybe a first table-driven `go test` (ask; user didn't want tests in Python) |
| 2 | StateManager | LEARN | maps, methods + receivers, `sync.Mutex`, `defer`, copying structs that contain pointers (deep copy is manual) |
| 3 | AckTracker | **LEARN (core)** | channels, buffered `chan` per pending command, `select` + `time.After`/context, custom error types, `errors.As` |
| 4 | MAVLink connection + Receiver + Mapper | BORROW (gomavlib; verify the module path, version, COMMAND_INT, write thread-safety in its source) | interfaces (implicit), goroutines, type switch |
| 5 | VehicleService (command table, submit/execute, wait for state) | **LEARN (core)** | function fields, closures, `context.WithTimeout`, `time.Ticker`, goroutine per command |
| 6 | SQLite command log | BORROW, Claude builds | `database/sql`, `modernc.org/sqlite` (pure Go, no cgo) |
| 7 | HTTP API | Claude builds after a chat | `net/http` pattern routing (`POST /vehicles/{id}/commands/{name}`), `encoding/json`, hand validation |
| 8 | Docker multi-stage + compose profiles (`backend-py` / `backend-go`), graceful shutdown | Claude builds | `signal.NotifyContext`, static binary |

**Milestone 1:** the unchanged React dashboard drives arm → set_mode → takeoff → goto on SITL through the Go backend.
**Milestone 2 (well-rounded MVP):** status enum (typed string consts), HealthManager (heartbeat age), startup sweep of
stale PENDING rows, a send lock / single writer, mode validation at submit (422), maybe STATUSTEXT.

Suggested layout: `cmd/droned/`, `internal/{config,domain,app,mavlink,store,api}`.

## Design that must carry over (port the design, not the lines)

- Exactly one inbound consumer (Receiver goroutine). ACK → AckTracker; telemetry → StateManager.
- AckTracker key `(system_id, command_name)`, name = MAV_CMD enum name without prefix (`COMPONENT_ARM_DISARM`,
  `DO_SET_MODE`, `NAV_TAKEOFF`, `DO_REPOSITION`). One outstanding command per key (second register → already pending).
  Register BEFORE send; wait owns cleanup on every path (`defer`); a failed send discards. Remove an entry only if it is
  the same object (`==` pointer). Success = {ACCEPTED, IN_PROGRESS}; TEMPORARILY_REJECTED is retryable; others not.
  Known limitation: a late ACK can match a retry (COMMAND_ACK has no sequence number).
- ACK ≠ done: completion is a predicate over vehicle state, polled (`StatePollInterval` 100 ms) until a deadline,
  with one final check after the deadline. Three distinct failures: ACK_TIMEOUT, COMMAND_REJECTED, STATE_TIMEOUT.
- StateManager: one mutex, every public method locks, `Get` returns a copy (nil/false if unknown), never call another
  locking method while holding the lock.
- Submit is fast: validate params → state check hook → create PENDING row → start a goroutine → return id (HTTP 202).
  Execute always writes the final status (its own error handling, so a DB error can't hide the real one).
- Wall clock for stored/shown times (Unix seconds float), monotonic only for deadlines (Go's `time.Now()` carries both).
- Python gaps to fix in Go, not in Python: concurrent `send` from several command threads is unlocked; unknown mode
  fails late (FAILED after 202) instead of 422; command log stores only `{"name"}` not params; no stale-PENDING sweep;
  `mode_mapping()` is the plane table before the first heartbeat (Go: pick the table from the HEARTBEAT `type`).

## HTTP contract (React depends on it; keep identical)

- `POST /vehicles/{system_id}/commands/{name}` with optional JSON object body → **202** `{"command_id": str, "status": "PENDING"}`;
  404 `UNKNOWN_COMMAND`; 422 `INVALID_PARAMS` (detail like `"altitude: Input should be ...;lat: ..."`, fields joined by `;`).
- `GET /vehicles` → list of vehicle states. `GET /vehicles/{system_id}` → state; 404 `NO_VEHICLE` until telemetry arrives.
- `GET /commands?system_id=&limit=` (limit 1..500, default 50) → newest first. `GET /commands/{id}` → record; 404 `UNKNOWN_COMMAND_ID`.
- Error body is exactly `{"error": CODE, "detail": str}`.
- Vehicle state JSON: `system_id, component_id, vehicle_type, autopilot, is_armed, flight_mode, system_status`
  (all nullable), `heartbeat_count` (int, 0), `position {latitude, longitude, altitude, relative_altitude}` or null,
  `velocity {velocity_x, velocity_y, velocity_z}` or null, `heading` or null. No batteries, no monotonic timestamps.
- Command record JSON: `id, system_id, cmd (object), status, error (nullable), created_at, updated_at`.
- Statuses: PENDING, ACHIEVED, ACK_TIMEOUT, COMMAND_REJECTED, STATE_TIMEOUT, ALREADY_PENDING, FAILED.
- Port 8000 (`HTTP_ADDR`); env `MAVLINK_CONNECTION`, `DB_PATH` (Docker: `udpin:0.0.0.0:14540`, `/data/drone.db`).

## Commands (params, message, predicate, state timeout)

Unknown body keys are rejected (422). Default state timeout = 10 s.

| name | body | message | predicate | timeout |
|---|---|---|---|---|
| arm | none | COMMAND_LONG ARM_DISARM p1=1, comp AUTOPILOT1 | is_armed == true | 10 s |
| disarm | none | same, p1=0 | is_armed == false | 10 s |
| set_mode | `{mode: non-empty str}` | COMMAND_LONG DO_SET_MODE p1=MAV_MODE_FLAG_CUSTOM_MODE_ENABLED (1), p2=mode number | flight_mode == mode | 10 s |
| takeoff | `{altitude: 2..100, finite}` | COMMAND_LONG NAV_TAKEOFF p4=NaN, p7=altitude | relative_altitude >= 0.9 × altitude | 30 s |
| goto | `{lat: -90..90, long: -180..180, altitude: 2..400, finite}` | **COMMAND_INT** DO_REPOSITION frame GLOBAL_RELATIVE_ALT_INT, p1=-1, p2=0, p3=0, p4=NaN, x=lat·1e7, y=long·1e7, z=altitude | horizontal distance ≤ 7 m and abs(rel_alt − altitude) ≤ 5 m | 200 s |

goto submit-time check: vehicle needs a position fix and the target must be ≤ 500 m away, else 422.
Mapper units: GLOBAL_POSITION_INT lat/lon ÷1e7, alt/relative_alt ÷1000 (mm→m), vx/vy/vz ÷100, hdg ÷100 (65535 → null).
armed = `base_mode & MAV_MODE_FLAG_SAFETY_ARMED`.

## SQLite schema (keep identical)

```sql
CREATE TABLE IF NOT EXISTS commands (
    id TEXT PRIMARY KEY, created_at REAL NOT NULL, updated_at REAL NOT NULL,
    system_id INTEGER NOT NULL, cmd TEXT NOT NULL, status TEXT NOT NULL, error TEXT);
CREATE INDEX IF NOT EXISTS idx_commands_system_created ON commands (system_id, created_at DESC);
```
WAL mode, uuid4 ids, `cmd` is a JSON object.

## Open decisions (ask the user)

1. Commit + tag `python-mvp` in `alpha` (everything since "Init commit" is uncommitted; add `__pycache__/` to `.gitignore`).
2. `cmd` column: store `{name, params}` in Go (and the one-line Python fix at `alpha/application/vehicle_service.py:151`)?
3. `git init` in `beta/`.

## Environment

Omarchy Linux. Go 1.27.3 via mise (global `go@1.27.3`), GOPATH `~/go`. ArduPilot SITL + MAVProxy forwards UDP to
`127.0.0.1:14540`; QGC on 14550. Python venv `alpha/.venv` (pymavlink 2.4.41). The Python backend runs with
`.venv/bin/uvicorn api.app:app` or via compose.
