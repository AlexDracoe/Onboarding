# DSETraining — Drone MAVLink Handoff

_Last updated: 2026-10-20. Supersedes `DRONE_MAVLINK_HANDOFF_2026-10-16.md` for current state; the older files still hold useful background (SITL/MAVProxy/QGC topology, networking notes)._

## How to work with the user

- The user learns by writing design-heavy code themselves. Coach (direction, review, hints) rather than dumping solutions.
- Help ladder: L0 direction → L1 review/nudges (point to exact lines, hints not fixes) → L2 skeleton → L3 partial → L4 full solution (only on explicit request). Resets per task. The user asks for more help by saying so ("skeleton", "need more hand holding").
- Before each task give a one-line verdict: **LEARN** (coach), **BORROW** (pymavlink/spec already solves it; check real source in `.venv`), **SKIP/MINIMAL** (tedious; offer to do it).
- Answer reviews in the terminal. If the user asks for inline comment threads in a file, do that, but beware merge clashes: the user edits files concurrently, so re-read before editing.
- No tests/test framework unless asked.
- Explain the *why*, especially concurrency and architecture, and mention how it looks in Go.
- Use they/them if pronouns are ever needed.

## Plan and deadline (decided 2026-10-20)

- **Week 1:** a fully fledged Python service: core commands + health, FastAPI layer, SQLite persistence, thin React/JS dashboard.
- **Week 2:** port to Go.
- User is not worried about API/GUI learning value ("straight forward"); most value is in command integration on the vehicle. So: Claude may write most of the low-value pieces (after a short interactive chat about each), the user writes the high-value ones.

## Architecture (current, working)

```
SITL -> MAVProxy -> udpin:127.0.0.1:14540 -> PymavlinkConnection
  -> Receiver (ONE inbound consumer, background daemon thread)
       -> Mapper (pymavlink msg -> domain update)
       -> CommandAckUpdate -> AckTracker.handle_ack
       -> other updates    -> StateManager.apply_update
main thread: CommandHandler builds message; AckTracker.register -> connection.send -> wait_for_ack
```

Layout: `domain/` (models.py, updates.py, interfaces.py), `application/` (ack_tracker, state_manager, command_handler; health_manager.py and vehicle_service.py are empty), `infra/mavlink/` (connection, mapper, receiver), `api/` (empty), `tests/fakes/fake_connection.py`.

Rules decided:
- Exactly one inbound consumer (Receiver). Never call `recv_match`/`wait_heartbeat` from elsewhere (note: pymavlink's `motors_armed_wait()` violates this, so don't use it).
- ACK ≠ done. ACCEPTED means the autopilot took the command; confirm from telemetry (HEARTBEAT for armed). ACK -> AckTracker; telemetry -> StateManager. Keep them separate.
- AckTracker key: `(system_id, command_name)` where the name is what `Mapper.enum_name` produces (prefix stripped, e.g. `COMPONENT_ARM_DISARM`). A COMMAND_ACK has no timestamp/sequence, so the key cannot contain one; put timestamps in the `PendingCommand` value.
- One outstanding command per `(system_id, command)`; a second `register` raises `CommandAlreadyPending`.
- Register BEFORE send. `wait_for_ack` owns cleanup via try/finally (`discard`, removes the entry only if it `is` the same object). A failed send discards explicitly and re-raises.
- Never hold the lock while blocking in `Event.wait`. Write `result` before `event.set()`.
- Timeout = exception (`AckTimeout`); a rejected ACK should also raise (`CommandRejected` carrying the result).
- Multi-vehicle: shared services, per-vehicle state keyed by `system_id`.

## Milestone reached

ARM in SITL works end to end: `ARM result: ACCEPTED`, ACK delivered by the Receiver thread to the waiting main thread, entry cleaned up.

## Known issues / TODO (current code)

1. `main.py` has `except CommandRejected` but **does not import it** (line 8 imports `AckTracker, UnexpectedAck, AckTimeout, CommandAlreadyPending`). It would raise NameError if an exception ever reaches that clause.
2. `wait_for_ack` **never raises `CommandRejected`**; a DENIED/FAILED ACK is returned as a normal result. Decide success set (ACCEPTED only? what about IN_PROGRESS?) and make `CommandRejected` carry `result`. Also: `UnexpectedAck` is now unused (handle_ack just prints).
3. `Receiver.start()` has **no try/except**: any exception from the mapper or state manager (e.g. an enum lookup in `map_heartbeat`) kills the thread silently. Highest-priority hardening item.
4. `main.py` has stale comments, `pass` lines, leftover thoughts about timestamp keys, and `time.sleep(5)` hiding that `StateManager` has no vehicle yet when the main thread starts.
5. `StateManager` has no lock; fine for single-bool polling in CPython, unsafe for multi-field reads (and a data race in Go). `vehicles[...]` lookup from the main thread can KeyError before the first update; `is_armed` starts as `None`.
6. `CommandHandler` builds pymavlink messages directly (application layer depends on pymavlink). `ARM_NAME` constant lives there as a v1 compromise. Roadmap step 7 fixes the boundary; do it before the Go port because Go's domain interface will mirror it.
7. `set_mode()` is broken: uses undefined `connection`/`master`. Needs `param1 = MAV_MODE_FLAG_CUSTOM_MODE_ENABLED`, `param2 = mode number`.
8. `map_battery_status` is disabled and has an undefined `cell_voltages_mv`; `BATTERY_STATUS` is deliberately unmapped (SKIP/MINIMAL).
9. `STATUSTEXT` is unmapped; it carries ArduPilot PreArm failure reasons. Needed for "failed to arm {reason}".
10. `config.py` is empty; logging uses `print`; there is no graceful shutdown or reconnect; `main.py` ends with `receiver_thread.join()`.
11. Existing tests are not maintained: `tests/unit/test_mapper.py` has wrong imports/stubs, `test_state_manager.py` has a stray `ls` (syntax error). User asked for no tests for now.
12. Known limitation: a late ACK from a timed-out attempt could match a retry (no sequence in COMMAND_ACK).

## Roadmap status

1. AckTracker + wire into ARM: **done** (except items 1–2 above).
2. Wait for confirmed armed state (HEARTBEAT) as a separate mechanism: **in progress, user's design decisions made**: minimal polling (not condition variable); check the *current* value first (already armed returns immediately); deadline via `time.monotonic()`; short sleep between polls; treat `None` as not-armed; handle missing vehicle; timeout error is its own exception (ACK timeout and "ACCEPTED but never armed" mean different things); reason text needs STATUSTEXT (later). Open: where the wait lives (StateManager method vs service layer; `application/vehicle_service.py` is the natural home for orchestration: register → send → wait ACK → wait state).
3. Set mode: **BORROW** (`mavutil.mode_mapping_byname` / `set_mode_apm` in `.venv/.../pymavlink/mavutil.py`), fix the param bug.
4. Takeoff in GUIDED (mode → arm → takeoff → watch altitude; handle `IN_PROGRESS` ACKs): **LEARN, user writes**.
5. HealthManager (link health from heartbeat age): **LEARN, user writes** (small, safety-relevant).
6. RTL / goto: **SKIP/MINIMAL**, Claude can write the message construction.
7. Move pymavlink out of the application layer (CommandHandler boundary): **LEARN, user designs**; do before the Go port.
8. Mission upload handshake: only if missions are required for the demo.
9. FastAPI layer, SQLite, React dashboard, graceful shutdown, config/logging: **mostly Claude writes**, user reviews.

Design direction discussed and parked: a command lifecycle (REGISTERED → ACKED → ACHIEVED | DENIED | TIMED_OUT, with per-stage timestamps). The user wants to track when a command was registered/acked/achieved. Guidance given: states are alternatives at the end, not an ordered line; do not make `StateManager` scan commands for expected outcomes (it would have to know what ARM/TAKEOFF/SET_MODE expect, mirroring the AckTracker-owning-vehicle-state mistake). Put per-command expected-outcome predicates in a service layer; `StateManager` only exposes state. SQLite is the natural home for the lifecycle/timestamps. Do the minimal version first.

## Go port notes

Do not port line by line. What ports is the design: single consumer, ACK matching by key, register-before-send, state-confirmed completion, timeouts distinguishing "no answer" from "answered no", cleanup on every path. Go equivalents: mutex + `chan Result` per pending command with `select`/`time.After`; `defer` for cleanup; verify a Go MAVLink library (e.g. `gomavlib`) before relying on it. Keep the HTTP API contract and SQLite schema stable so the React frontend and DB carry over unchanged. Write a one-page "port spec" from the decisions above.

## Environment

Omarchy Linux, `.venv` in the project, ArduPilot SITL + MAVProxy + QGroundControl work. MAVProxy forwards UDP to `127.0.0.1:14540`; the service uses `udpin:127.0.0.1:14540`; QGC listens on 14550.
