# Prompt: Drone MAVLink service — week-1 plan, cleanup, low-value items

Read `DRONE_MAVLINK_HANDOFF_2026-10-20.md` first (current state, rules, known issues, roadmap). Older handoff files are background only. Then read every `.py` file in the project (`.venv` has pymavlink; check its real source before calling anything LEARN or BORROW).

Context: I'm building a full Python drone service this week (core commands, FastAPI, SQLite, a thin React/JS dashboard), then porting it to Go next week. I learn by writing the high-value code myself (concurrency, sequencing, architecture) and want you to build the low-value pieces. **For every item below, talk it through with me before you change anything.**

## How to run this session

1. **Start with a short status check, not edits.** Tell me the 3–5 things you'd tackle first, in order, and ask me about anything that depends on a decision of mine. Don't edit files in this step.
2. **For each item: chat first, then do it.** One message from you with: the verdict (LEARN / BORROW / SKIP-MINIMAL), what you'd do and which files, any decisions you need from me. Wait for my answer, then do it, then summarize what changed. Don't batch several items into one go-ahead.
3. **High-value items stay mine** (help ladder below). Low-value items you may write, after the chat.
4. Before editing a file I might be working on, re-read it (I edit concurrently). Don't overwrite my comments or code.
5. No tests unless I ask.

## A. Cleanup of what exists (you do, after we chat)

From the handoff's known-issues list:
- `main.py`: missing `CommandRejected` import, stale comments / `pass` lines / the `time.sleep(5)` hiding the missing-vehicle case. Keep my `try/except` structure.
- Receiver hardening: wrap the loop body so one bad message can't kill the thread; log and continue. Ask me how I want errors surfaced.
- Logging: replace `print`s with `logging` configured in `logging_config.py`; simple `config.py` for the connection string, timeouts, poll interval.
- Remove unused code (`UnexpectedAck` if still unused, dead variables, stale docstrings).
- Do NOT move pymavlink out of `CommandHandler` yet (that's a design exercise of mine).

## B. Low-value items you build (after we chat about each)

- Set mode: BORROW (`mavutil.mode_mapping_byname` / `set_mode_apm`); fix the param bug.
- RTL and goto message construction.
- `STATUSTEXT` mapping (PreArm reasons), minimal.
- FastAPI layer: thin routes over a service layer, with error mapping (timeout, rejected, already-pending) and a stable contract (write an OpenAPI-friendly schema in `api/schemas.py`). The contract must stay stable so the React app survives the Go port.
- SQLite: command log with stage timestamps (registered / acked / achieved / failed) and maybe telemetry snapshots. Ask me about the schema before writing it; keep it portable to Go.
- Thin React/JS dashboard: vehicle state, arm/takeoff/RTL buttons, command history. Minimal styling.
- Battery mapping: SKIP/MINIMAL (a few fields only), if at all.

## C. High-value items I write (coach me with the ladder)

- Finish roadmap step 2 (wait for armed state; my decisions are in the handoff), the rejected-result handling in `wait_for_ack`.
- Takeoff sequencing in GUIDED with `IN_PROGRESS` ACKs.
- HealthManager (link health from heartbeat age).
- The CommandHandler → domain boundary (before the Go port).
- Where orchestration lives (`application/vehicle_service.py`), and the command lifecycle design in the handoff.

### Help ladder (resets to level 0 at the start of every new task)
Only go up one level when I ask or am clearly stuck after trying.
- **L0 Direction:** goal, concept, key questions, which files. No code.
- **L1 Review and nudges:** I say "check"; you point to exact lines with hints/questions, not fixes.
- **L2 Skeleton:** signatures, structure, `# TODO` steps, no logic. Write it into the file only if I ask.
- **L3 Partial:** fill in the hardest one or two pieces (in chat unless I ask); I finish.
- **L4 Full solution:** only if I explicitly say so, then explain it.

Always explain *why* (especially concurrency and architecture) and mention how it would look in Go when useful.

## Learn / Borrow / Skip verdict
Before every new task, a one-line verdict: **LEARN** (pymavlink doesn't solve it and the concept carries to other projects; coach me), **BORROW** (pymavlink or the spec already solves it; point me at the helper/source), **SKIP/MINIMAL** (tedious and low value; give the minimum version and offer to do it). Check the pymavlink source in `.venv` before calling something LEARN. Flag anything I'm about to build that's out of scope for the current step.

## Schedule and scope guard
Week 1 is Python + API + SQLite + thin React. Week 2 is the Go port. Keep a running list of what's done and what's cut, and tell me honestly when something won't fit. Towards the end of week 1, help me write a one-page port spec (design decisions that must carry into Go) from the handoff.

## Start
Do step 1 (status check, no edits), then wait for me.
