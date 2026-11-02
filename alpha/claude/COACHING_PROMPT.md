# Prompt: Drone MAVLink service — cleanup + coaching mode

Read `DRONE_MAVLINK_HANDOFF_2026-10-16.md` and `DRONE_MAVLINK_HANDOFF (1).md` for background, then read every `.py` file in this project. The project is a Python drone control service for ArduPilot SITL using pymavlink (installed in `.venv`). I'm building it to learn, so you have two jobs: **first a one-time cleanup pass, then coaching mode for everything after that.**

---

## Phase 1 — Cleanup pass (you do this yourself)

Fix the mechanical problems so `main.py` runs again. **Don't make design decisions for me.** If a fix needs one, leave a `# TODO(learn):` comment that explains the question, and add it to the list you report back.

### 1. Fix typos and syntax errors. Known ones (check for more):
- `infra/mavlink/mapper.py`: a stray `""` in the handlers dict; a junk line `addds to the list of what gets scp`; `messsage` → `message`
- `application/ack_tracker.py`: `from dataclass` → `dataclasses`; `PendingCommnad`; `new_commmand`/`new_command`; `command_id` → `command`; `pending` → `self.pending`. Fix only the names. **Leave the logic alone**: the list-per-key, `self.lock.set()`, and the `raise NotImplementedError`s are my exercise. Mark them with `# TODO(learn):`.
- `infra/mavlink/receiver.py`: `awaitACK` → `handle_ack`; type-hint `connection` as `Connection`, not `FakeConnection`
- `main.py`: `reciever_thread` → `receiver_thread`
- `application/command_handler.py`: `disarm()` is missing `target_component` (use `mavutil.mavlink.MAV_COMP_ID_AUTOPILOT1`, the same value `arm()` uses)
- `domain/updates.py`: imports for `Position`, `Velocity`, `BatteryState`
- `loggin_config.py` → `logging_config.py`

### 2. Move files where they make more sense, and update the imports:
- Domain models (`Position`, `Velocity`, `BatteryState`, `VehicleState`) belong in `domain/` (e.g. `domain/models.py`), not `application/`. Right now the infra mapper imports from application, so the dependency points the wrong way.
- `tests/FakeConnection.py` → a test-support location (e.g. `tests/fakes/fake_connection.py`). `main.py` shouldn't import from `tests/`; leave a commented-out alternative instead.
- Add `send()` to the `Connection` protocol in `domain/interfaces.py`.
- Add the missing `__init__.py` files if imports need them.
- **Don't** move pymavlink message construction out of `CommandHandler`. That boundary fix is a design exercise for me, so just flag it.

### 3. Small cleanups
- Remove the duplicate "unknown message" logging between Mapper and Receiver (keep one).
- `StateManager` prints the whole vehicle on every update, which floods the terminal. Make that opt-in or throttle it.
- Don't add tests or a test framework.

### 4. Verify
Run `.venv/bin/python -c "import main"`, or compile the files with `py_compile`, so that at least there are no import or syntax errors. Don't require SITL to be running.

### 5. Report back
Give me a short list of: what you fixed, what you moved and why, and every `TODO(learn)` you left. Then switch to Phase 2.

---

## Phase 2 — Coaching mode (for the rest of the session and later sessions)

### Rules
- **I write the design-heavy code.** Don't edit files I'm working on unless I explicitly ask. Mechanical fixes (typos, imports, renames) are fine; ask first if they touch code I'm actively writing.
- **Answer in the terminal.** When I say "check", read my code, comments, and pseudocode and respond there.
- **Don't add tests** unless I ask.

### Roadmap
Give me the next step at a general level: the goal, the concept it teaches, and what "done" looks like (ideally something I can watch happen in SITL). Current order:
1. AckTracker (`register` / `handle_ack` / `wait_for_ack`), then wire it into ARM: register → send → wait → clean up on every path
2. Wait for the confirmed state (HEARTBEAT shows armed) as a separate mechanism from the ACK
3. Set mode (fix the param bug: ArduPilot wants `param1 = MAV_MODE_FLAG_CUSTOM_MODE_ENABLED`, `param2 = mode number`)
4. Takeoff in GUIDED (sequence: mode → arm → takeoff → watch altitude; handle `IN_PROGRESS` ACKs)
5. HealthManager (link health from heartbeat age)
6. RTL / goto
7. Moving pymavlink out of the application layer (the CommandHandler boundary)
8. Mission upload handshake (`MISSION_COUNT` / `REQUEST_INT` / `ITEM_INT` / `ACK`)
9. FastAPI layer, graceful shutdown, config/logging

### Help ladder (resets to level 0 at the start of every new task)
Only go up one level when I ask for more help or am clearly stuck after trying.
- **Level 0 — Direction:** the goal, the concept, the key questions to think about, and which file(s) to change. No code.
- **Level 1 — Review and nudges:** I write code, comments, or pseudocode in the file and say "check". You point out what's wrong or missing as a hint or question, not the fix. Point to the exact line.
- **Level 2 — Skeleton:** signatures, structure, and `# TODO` steps, with no logic filled in. You can write it into the file if I ask.
- **Level 3 — Partial:** fill in the hardest one or two pieces, or show a minimal example of the concept outside my code, and I finish the rest.
- **Level 4 — Full solution:** only if I explicitly say so. Explain it afterward.

At every level, explain *why*, especially concurrency and architecture reasoning, and mention how it would look in Go when that's useful.

### Learn / Borrow / Skip verdict
Before every new task, start with a one-line verdict:
- **LEARN:** pymavlink doesn't solve it and the concept carries over to other projects. Coach me.
- **BORROW:** pymavlink or the MAVLink spec already solves it (e.g. `mavutil.mode_mapping_byname()`, command param layouts in `arducopter_arm()` / `set_mode_apm()` in `.venv/.../pymavlink/mavutil.py`). Point me at the helper or source, and I'll wrap it in a few minutes.
- **SKIP / MINIMAL:** tedious and low-value (e.g. mapping every BATTERY_STATUS field, hand-written enum tables). Tell me the minimum version and move on; offer to do the tedious part yourself.

Before calling something LEARN, check the actual pymavlink source in `.venv`; don't go from memory. Also flag it if something I'm about to build is out of scope for the current step.

### Start
Do Phase 1, report back, then give me the Level 0 for roadmap step 1.
