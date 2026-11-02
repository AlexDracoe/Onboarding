# DSETraining — Drone MAVLink Handoff

_Last updated: 2026-10-22 (week 1, day 3). Supersedes `DRONE_MAVLINK_HANDOFF_2026-10-20.md` for current state; older files are background only (SITL/MAVProxy/QGC topology, networking)._

## How to work with the user

- The user learns by writing design-heavy code (concurrency, sequencing, architecture) themselves. Claude coaches and writes the low-value pieces after a short chat about each.
- **Talk it through before changing anything**: for every item, one message with a verdict (LEARN / BORROW / SKIP-MINIMAL), what you'd do and which files, and decisions you need. Wait for the answer, do it, summarize. No batching several items into one go-ahead.
- Help ladder, resets to L0 per task: L0 direction → L1 review/nudges ("check": point to exact lines with hints, not fixes) → L2 skeleton (TODOs, no logic; into the file only if asked, the user likes it in the IDE) → L3 partial (hardest pieces, usually in chat so the user types it) → L4 full solution only on explicit request. The user asks for more help by saying so ("need more help", "skeleton").
- When the user types code from your explanation, they want "what you did vs what was needed" and the *why*. They also like syntax references (they know C++ well, Python little: explain Python idioms with C++/Go analogies).
- The user edits files concurrently: always re-read before editing, don't overwrite comments/code. Prefer testing in the scratchpad dir (a copy of the file, a throwaway DB) over touching their files. No tests in the repo unless asked.
- Review by running the code: import with `-W error`, drive it with fakes, report real output. Never claim something works without running it.
- Use they/them if pronouns are ever needed. Explain concurrency/architecture "why" and mention how it looks in Go.

## Plan and deadline

- **Week 1 (2026-10-20 → 10-26):** full Python service: core commands + health, FastAPI layer, SQLite persistence, thin React/JS dashboard.
- **Week 2:** port to Go. The design ports, not the lines. Keep the HTTP contract and SQLite schema stable so React and the DB carry over. Towards the end of week 1, write a one-page port spec from the decisions below.

## Architecture (working)

```
SITL -> MAVProxy -> udpin:127.0.0.1:14540 -> PymavlinkConnection
  -> Receiver (ONE inbound consumer, daemon thread "receiver", has stop_event)
       -> Mapper (pymavlink msg -> domain update)
       -> CommandAckUpdate -> AckTracker.handle_ack
       -> other updates    -> StateManager.apply_update
caller thread: VehicleService.execute(CommandRequest)
       -> spec lookup -> command_log.create(PENDING) -> AckTracker.register(system_id, spec.ack_name)
       -> connection.send(message) -> wait_for_ack -> wait_for_state(spec.predicate) -> command_log.update_status in finally
```

Layout: `domain/` (models, updates, interfaces: `Connection` and `CommandLog` Protocols), `application/` (ack_tracker, state_manager, command_handler, vehicle_service; health_manager.py empty), `infra/mavlink/` (connection, mapper, receiver), `infra/db/sqlite_command_log.py`, `api/` (routes.py, schemas.py both EMPTY), `config.py` (frozen `Config`, `CONFIG`), `logging_config.py` (`setup_logging`), `main.py` (`main()`), `tests/` (not maintained).

Rules decided (keep):
- Exactly one inbound consumer (Receiver). Never call `recv_match`/`wait_heartbeat` elsewhere after start.
- ACK ≠ done. ACK → AckTracker; telemetry → StateManager; "achieved" is confirmed from state via a predicate.
- AckTracker key `(system_id, command_name)` where the name is what `Mapper.enum_name` produces (`COMPONENT_ARM_DISARM`). One outstanding command per key; second `register` raises `CommandAlreadyPending`. Register BEFORE send; `wait_for_ack` owns cleanup (`finally: discard`); a failed send discards explicitly. Never hold a lock in `Event.wait`; write `result` before `event.set()`.
- `wait_for_ack` returns only for `SUCCESS_RESULTS` = {ACCEPTED, IN_PROGRESS}; everything else raises `CommandRejected(system_id, command, result, retryable)` (`RETRYABLE_RESULTS` = {TEMPORARILY_REJECTED}; DENIED/FAILED/UNSUPPORTED/UNKNOWN non-retryable). Timeout → `AckTimeout`. IN_PROGRESS multi-ACK handling deferred to takeoff.
- StateManager has one `threading.Lock`; `apply_update` and `get()` hold it; `get()` returns a deepcopy (None if unknown). Public methods lock, never call another locking method while holding the lock (verbose print is outside the `with`). Go: same, with a mutex.
- `wait_for_state(system_id, predicate, description, timeout)` polls `state_manager.get` every `CONFIG.state_poll_interval_s`, deadline via `time.monotonic()`, one final check after the loop, raises `StateTimeout(system_id, description, timeout)`. It only waits; side effects (DB/API) belong to the caller.
- Time: wall clock (`time.time()`) for anything stored/shown; monotonic only for deadlines/elapsed.
- Receiver never dies from one bad message (per-message try/except with logging); `receive()` errors back off `CONFIG.receive_backoff_s`.
- Command table in `VehicleService._commands`: name → `CommandSpec(build, ack_name, predicate, description)`; `arm`/`disarm` exist (same ack_name, predicates `is_armed is True/False`). `CommandRequest(name, system_id)`.
- `execute` always writes the final status in `finally` (own try so a DB error can't hide the real one), then re-raises. Statuses are strings for now: PENDING, ACHIEVED, ACK_TIMEOUT, COMMAND_REJECTED, STATE_TIMEOUT, ALREADY_PENDING, FAILED. `UnknownCommand` raised before any row exists.
- SQLite: `commands(id TEXT PK uuid4, created_at REAL, updated_at REAL, system_id INTEGER, cmd TEXT json, status TEXT, error TEXT)` + index (system_id, created_at DESC). One connection, `check_same_thread=False`, one Lock, WAL. Methods return plain dicts (`cmd` decoded). Only this class touches the DB. Go: `database/sql` is already a safe pool.

## Verified so far

- ARM works end to end in SITL (earlier session, via inline code in main.py).
- `execute` verified against fakes (real AckTracker/StateManager + fake connection and in-memory log): arm/disarm success, rejected ACK, ACK timeout, state timeout, send failure, unknown command, already pending; pending dict empty after every path.
- `SqliteCommandLog` verified on a temp DB: update (and KeyError on unknown id), get, missing get, newest-first list, system_id filter, limit, empty list, close. All code imports with `-W error`.
- NOT yet run: `execute` against real SITL, `main.py` with the service.

## Known issues / TODO

1. **`main.py` is not wired to the service**: it still has the old inline ARM block. Needs: `SqliteCommandLog(CONFIG.db_path)`, `VehicleService(state_manager, command_handler, ack_tracker, connection, command_log)`, `execute(CommandRequest("arm", 1))`, `command_log.close()` in finally. Eventually becomes the app factory/lifespan for FastAPI.
2. Status enum (user writes) replacing the strings; maybe per-stage timestamp columns (`acked_at`, `finished_at`); the schema stays portable.
3. Open design question: on failure the route doesn't learn `command_id` (exceptions carry no id); `execute` returns the state snapshot, not a record. A `return` inside `try` evaluates before `finally`, so returning the DB record would show a stale status. Decide with the API design.
4. `CommandHandler` still builds pymavlink messages (`ARM_NAME` constant; `connection`/`ack_tracker` args unused). Roadmap: move pymavlink out before the Go port (user designs).
5. `set_mode()` broken (undefined `connection`/`master`, needs `MAV_MODE_FLAG_CUSTOM_MODE_ENABLED` + mode number). BORROW `mavutil.mode_mapping_byname` / `set_mode_apm`; the wrapper `PymavlinkConnection` has no `mode_mapping()`.
6. `STATUSTEXT` unmapped (PreArm reasons). `BATTERY_STATUS` unmapped, `map_battery_status` has an undefined `cell_voltages_mv` (SKIP/MINIMAL).
7. `PymavlinkConnection.receive()` uses `recv_match(blocking=True)` with no timeout, so `Receiver.stop()` only takes effect on the next message; fix = `timeout=1` and return None (Receiver already skips None).
8. HealthManager (link health from heartbeat age) empty; takeoff sequencing not started; no reconnect.
9. `requirements.txt` has only pymavlink, lxml, pytest: FastAPI/uvicorn (and pydantic if used) must be added to `.venv`.
10. Existing tests are stale (`tests/unit/*`); user asked for none for now.
11. Known limitation: a late ACK from a timed-out attempt could match a retry (no sequence number in COMMAND_ACK).

## Roadmap / ownership

- **Done (this week):** cleanup + logging + config; Receiver hardening + stop flag; rejected-ACK handling; locked StateManager + `get`; `wait_for_state`; `VehicleService.execute` for arm/disarm; SqliteCommandLog.
- **Next (user, high value):** wire main.py + status enum; takeoff sequencing (mode → arm → takeoff → altitude wait, IN_PROGRESS ACKs); HealthManager; CommandHandler boundary.
- **Next (Claude builds after a chat each, low value):** FastAPI layer (thin routes over `VehicleService`, error mapping, stable contract in `api/schemas.py`), thin React/JS dashboard (vehicle state, arm/takeoff/RTL buttons, command history), set mode (BORROW), RTL/goto message construction, minimal STATUSTEXT.
- **Cut order if time runs short:** battery, STATUSTEXT, goto/RTL, then HealthManager. Takeoff and the pymavlink boundary matter more.

## API design notes (to settle in the API session)

- Contract must be stable across the Go port: React and the DB are shared.
- Candidate routes: `POST /vehicles/{system_id}/commands/{name}` (arm/disarm → `CommandRequest`), `GET /vehicles/{system_id}` (state snapshot), `GET /commands` (`list_recent`, optional `system_id`, `limit`), `GET /commands/{id}`. The user's mental model: GUI calls `sys/cmd/arm`.
- Error mapping to decide: `UnknownCommand` (404/422), `CommandAlreadyPending` (409), `CommandRejected` (retryable vs not; 409/422), `AckTimeout`/`StateTimeout` (504), anything else (500).
- `execute` blocks up to ack timeout + state timeout (~20 s default): decide sync request vs 202 + poll by command id (ties to open issue 3).
- FastAPI runs sync handlers in a thread pool: that is why StateManager and SqliteCommandLog are locked. The Receiver thread, service and log must be created once (lifespan/app factory), shut down cleanly (`receiver.stop()`, `connection.disconnect()`, `command_log.close()`).
- Responses are built from plain dicts / dataclasses; keep pymavlink types out of the schemas.

## Go port notes

Do not port line by line. What ports is the design: single consumer, ACK matching by key, register-before-send, state-confirmed completion via predicate, timeouts that distinguish "no answer" from "answered no" from "accepted but never happened", cleanup on every path (`defer`), per-vehicle state behind a mutex. Go equivalents: mutex + `chan Result` per pending command with `select`/`time.After`; `context.WithTimeout` + `time.Ticker` for the state poll; `errors.As` with typed errors for CommandRejected; `database/sql` pool for SQLite; verify a Go MAVLink library (e.g. `gomavlib`) before relying on it. Write the one-page port spec from this file.

## Environment

Omarchy Linux, `.venv` in the project (Python, pymavlink 2.4.41), ArduPilot SITL + MAVProxy + QGroundControl work. MAVProxy forwards UDP to `127.0.0.1:14540`; the service uses `udpin:127.0.0.1:14540`; QGC listens on 14550. DB file `drone.db` (gitignored with `-wal`/`-shm`).
