# Prompt: Drone MAVLink service — API layer, then dashboard design

Read `DRONE_MAVLINK_HANDOFF_2026-10-22.md` first (current state, rules, known issues, API design notes, roadmap, how to work with me). Older handoff files are background only. Then read every `.py` file in the project (`.venv` has pymavlink; check its real source before calling anything LEARN or BORROW).

Context: I'm building a full Python drone service this week (it's week 1, day 3): core commands (done for arm/disarm in `VehicleService.execute`), SQLite command log (done), now the FastAPI layer and a thin React/JS dashboard, then a Go port next week. I learn by writing high-value code (concurrency, sequencing, architecture) myself and want you to build low-value pieces. **For every item, talk it through with me before you change anything.**

## How to run this session

1. **Start with a short status check, no edits.** Run the imports with `-W error`, confirm what works, and tell me the 3–5 things you'd tackle first, in order, plus any decisions that are mine.
2. **Per item: chat first, then do it.** One message: verdict (LEARN / BORROW / SKIP-MINIMAL), what you'd do and which files, decisions you need. Wait for my answer, do it, summarize. Don't batch.
3. Re-read any file before editing it (I edit concurrently). Don't overwrite my comments or code. Test in the scratchpad dir, not in my files. No tests in the repo unless I ask.
4. When you show me code to type, explain what was wrong/needed and why.

## Today's work

1. **Wire `main.py` to the service** (small; first so we have something real to call from the API): `SqliteCommandLog`, `VehicleService`, `execute(CommandRequest("arm", 1))`, shutdown. Then try it against SITL if I have it running. Talk through: should `main.py` become the app factory/lifespan that FastAPI uses?
2. **API design chat (LEARN-ish, design is mine, plumbing is yours):** routes and request/response shapes for arm/disarm, state, command history; error mapping table; sync request vs 202 + polling (the open `command_id` question in the handoff); what goes in `api/schemas.py`. The contract must stay stable for the Go port.
3. **Build the FastAPI layer** (you, after the chat): thin routes over `VehicleService`, schemas in `api/schemas.py`, dependency wiring, lifespan (Receiver thread, service, log, clean shutdown). Add FastAPI/uvicorn to the venv and `requirements.txt` (ask me first).
4. **Frontend design chat**, then build the thin React/JS dashboard: vehicle state, arm/disarm (takeoff/RTL when the backend has them), command history. Minimal styling; contract-driven.
5. If time remains: the status enum (mine), STATUSTEXT minimal, set mode (BORROW).

## Help ladder (resets to level 0 at the start of every new task)
- **L0 Direction:** goal, concept, key questions, which files. No code.
- **L1 Review and nudges:** I say "check"; you point to exact lines with hints/questions, not fixes (and run the code to see what actually happens).
- **L2 Skeleton:** signatures, structure, `# TODO` steps, no logic. Into the file only if I ask.
- **L3 Partial:** fill in the hardest one or two pieces (in chat unless I ask); I finish.
- **L4 Full solution:** only if I explicitly say so, then explain it.
Only go up a level when I ask or am clearly stuck. Always explain *why* (especially concurrency and architecture) and mention how it would look in Go when useful.

## Learn / Borrow / Skip verdict
Before every new task, a one-line verdict: **LEARN** (coach me), **BORROW** (pymavlink or a library already solves it; point me at it), **SKIP/MINIMAL** (tedious and low value; give the minimum, offer to do it). Flag anything out of scope for the current step.

## Schedule and scope guard
Week 1 is Python + API + SQLite + thin React; week 2 is the Go port. Keep a running list of what's done and cut, and tell me honestly when something won't fit. Cut order if short: battery, STATUSTEXT, goto/RTL, HealthManager. Towards the end of week 1, help me write the one-page port spec from the handoff.

## Start
Do step 1 (status check, no edits), then wait for me.
