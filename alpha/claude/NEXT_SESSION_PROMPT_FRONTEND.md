# Prompt: Drone MAVLink service — frontend design, then thin dashboard

Read `DRONE_MAVLINK_HANDOFF_2026-10-22.md` first (rules, architecture, how to work with me; its "Known issues" list is partly stale, see "State" below). Then read `api/app.py`, `api/routes.py`, `api/schemas.py`, `application/vehicle_service.py`. Older handoff files are background only. Do not read the whole project again unless a step needs it.

Context: week 1, day 4 of a 1-week Python build then a 1-week Go port. The backend for arm/disarm is finished and verified against real SITL. This session: design the dashboard with me, then build it. I learn by writing high-value code (concurrency, sequencing, architecture) myself and want you to build low-value pieces. **For every item, talk it through with me before you change anything.**

## State at the end of the previous session (2026-10-22)

Done and verified on real SITL (arm and disarm both `ACHIEVED`, ~0.2–0.8 s):
- `VehicleService.submit(request) -> command_id`: validates the command name (raises `UnknownCommand` before any row exists), creates the PENDING row, starts one daemon thread per command, returns the id. I wrote this split.
- `VehicleService.execute(request, spec, command_id)`: the background body. Registers the ACK, sends, waits for ACK and for the state predicate, and always writes the final status in `finally`. It never raises (nobody to catch it); failures are rows.
- Decisions: 202 + poll; thread per command (a failed start just leaves PENDING); `register` happens in the background thread, so a second click while one is pending ends as a row with status `ALREADY_PENDING`, not an HTTP 409.
- `api/app.py`: `create_app()` + lifespan (ExitStack: connection, SQLite log, Receiver thread, service; stops the Receiver and joins before disconnecting, then closes the DB). The API starts even with no vehicle. Run it with `.venv/bin/uvicorn api.app:app` from the project root (`db_path` is relative).
- `PymavlinkConnection.receive()` now uses `recv_match(timeout=CONFIG.receive_timeout_s)` (1 s) so shutdown works.
- `main.py` is a stale scratch runner (delete or keep for offline runs, my call).

HTTP contract (must stay stable for the Go port; times are Unix seconds, wall clock):
- `POST /vehicles/{system_id}/commands/{name}` -> `202 {"command_id": str, "status": "PENDING"}`; `404 {"error":"UNKNOWN_COMMAND","detail":...}`. Names today: `arm`, `disarm`.
- `GET /vehicles/{system_id}` -> `VehicleStateOut` (system_id, component_id, vehicle_type, autopilot, is_armed, flight_mode, system_status, heartbeat_count, position{latitude, longitude, altitude, relative_altitude}, velocity{velocity_x, velocity_y, velocity_z}, heading); `404 {"error":"NO_VEHICLE",...}` until the first heartbeat.
- `GET /commands?system_id=&limit=` -> newest first list of `CommandRecord` (id, system_id, cmd{name}, status, error, created_at, updated_at); `limit` 1–500, default 50.
- `GET /commands/{command_id}` -> one record; `404 {"error":"UNKNOWN_COMMAND_ID",...}`.
- Record statuses so far (strings, my enum is still to come): PENDING (in flight), ACHIEVED, ACK_TIMEOUT, COMMAND_REJECTED, STATE_TIMEOUT, ALREADY_PENDING, FAILED. Everything except PENDING is terminal. A command can stay PENDING forever if the server dies mid-flight.
- Malformed input gives FastAPI's default 422 body (`{"detail":[...]}`), not `ErrorBody`.
- There is no CORS configuration yet. Node v26 is installed; no frontend exists. Swagger UI is at `/docs`.

Known gaps (not blocking): no startup sweep of stale PENDING rows; `PymavlinkConnection.send` is not locked while several worker threads may call it; takeoff/RTL/set-mode do not exist; `batteries` is never populated; `StateManager` timestamps are monotonic and deliberately not exposed.

## How to run this session

1. **Start with a short status check, no edits.** Confirm the backend still imports with `-W error`, check whether SITL/uvicorn are running (I'll tell you), and tell me the 3–5 things you'd tackle first plus any decisions that are mine.
2. **Per item: chat first, then do it.** One message: verdict (LEARN / BORROW / SKIP-MINIMAL), what you'd do and which files, decisions you need. Wait for my answer, do it, summarize. Don't batch.
3. Re-read any file before editing it (I edit concurrently). Don't overwrite my comments or code. Test in the scratchpad dir, not in my files. No tests in the repo unless I ask.
4. When you show me code to type, explain what was wrong/needed and why. I know C++ well and little Python/JS: give syntax references and C++/Go analogies.

## Today's work

1. **Frontend design chat (LEARN-ish; the design is mine, the plumbing is yours).** Questions to settle:
   - What is on screen: vehicle state panel, arm/disarm buttons, command history with status. What does "no vehicle" look like (404 from the state route)?
   - Polling: state every ~0.5–1 s; after a POST, poll `GET /commands/{id}` until a terminal status; history list refresh. Fixed interval vs backoff? What happens when the server is down or the network flaps?
   - Buttons: disable while a command is PENDING? (The backend allows a second click and records `ALREADY_PENDING`.) Show the `error` text from the record?
   - Stack: "simple React" = Vite + React, plain `fetch`, no router, no state library (confirm). Where it lives (`web/`?). TypeScript or plain JS?
   - CORS vs a dev proxy: a Vite proxy to `localhost:8000` needs no backend change and no contract change; `CORSMiddleware` is the alternative. My pick.
   - Optional 3D view (three.js, probably `@react-three/fiber`): a drone marker driven by `position`/`heading`, only after the basics work. Decide whether it is in or out for this week.
2. **Build the thin dashboard (you, after the chat).** Contract-driven, minimal styling. Verify against a real running backend (SITL if I have it up, otherwise a scratchpad app with a fake connection like last session's) and tell me what you actually saw.
3. **Backend leftovers, only if I ask:** startup sweep of stale PENDING rows (new `SqliteCommandLog` method), a lock around `PymavlinkConnection.send`, unified 422 body.
4. **Mine, if time remains:** the status enum, takeoff sequencing (mode → arm → takeoff → altitude wait, IN_PROGRESS ACKs), HealthManager (link health from heartbeat age), the CommandHandler/pymavlink boundary. Set mode (BORROW `mavutil.mode_mapping_byname`/`set_mode_apm`) and minimal STATUSTEXT are optional.

## Parked ideas (NOT part of week 1; discuss, do not build unless I say so)

My long-term goal: a follow-me tracker that keeps the drone about X metres from a detected object and follows it. Rough idea: a YOLO model doing the tracking on local hardware (a Raspberry Pi or similar companion computer), video from a RunCam with an OpenIPC encoder, plus a video feed in the dashboard. Be honest about scope and unknowns; verify each claim against real docs before relying on it.

- **Where it fits in the architecture:** vision runs in its own process on the companion computer and only outputs "target offset/size in frame". The MAVLink service owns the vehicle link (single inbound consumer rule stays). Frames never go through the Python API.
- **New kind of control, not a command:** following needs a continuous stream of velocity setpoints in GUIDED mode (for example `SET_POSITION_TARGET_LOCAL_NED` or body-frame velocity), at a steady rate, with no ACK-per-message. That does not fit the `_commands` table (request, ACK, state predicate). It needs its own design (a rate-controlled sender, start/stop, who owns the connection write). LEARN-level concurrency work: I should do it.
- **Safety is the real design:** max speed, geofence/min altitude, behaviour when the target is lost (hold? RTL?), when the vision link drops, an always-available operator abort and RC override. Test only in SITL with a fake or simulated target first.
- **Keeping a distance from a single camera is hard.** Estimating range from bounding-box size needs a known object size or a rangefinder/gimbal geometry. Discuss options (box-size heuristic, depth sensor, keep angle/size constant instead of metres). My idea: PnP (`cv2.solvePnP`) for relative position. It needs a known 3D shape with at least 4 matched image points plus a calibrated camera (intrinsics and lens distortion for the RunCam). A fiducial marker (AprilTag/ArUco) on the target fits well; a plain YOLO bounding box gives no point correspondences, so for an arbitrary object it would need a keypoint model or a size heuristic.
- **Video path (BORROW, verify):** OpenIPC/RunCam typically outputs RTSP or RTP (H.264/H.265); browsers do not play RTSP, so a relay such as MediaMTX or go2rtc to WebRTC/HLS is the usual route. Latency matters for control: the tracker should read the stream directly, not through the browser.
- **Compute (verify):** nano-size YOLO models via ONNX/NCNN on a Pi CPU give modest frame rates; an accelerator (for example a Hailo module) or a Jetson-class board may be needed. Measure on the real board before committing.
- **Schedule honesty:** with 4 days left in week 1 and the Go port in week 2, the tracker and video are a separate project. If I want a taste, the realistic slice is a SITL-only demo: a fake target position feeding a simple velocity controller. Tell me plainly whether it fits.

## Help ladder (resets to level 0 at the start of every new task)
- **L0 Direction:** goal, concept, key questions, which files. No code.
- **L1 Review and nudges:** I say "check"; you point to exact lines with hints/questions, not fixes (and run the code to see what actually happens).
- **L2 Skeleton:** signatures, structure, `# TODO` steps, no logic. Into the file only if I ask.
- **L3 Partial:** fill in the hardest one or two pieces (in chat unless I ask); I finish.
- **L4 Full solution:** only if I explicitly say so, then explain it.
Only go up a level when I ask or am clearly stuck. Always explain *why* (especially concurrency and architecture) and mention how it would look in Go when useful.

## Learn / Borrow / Skip verdict
Before every new task, a one-line verdict: **LEARN** (coach me), **BORROW** (a library already solves it; point me at it), **SKIP/MINIMAL** (tedious and low value; give the minimum, offer to do it). Flag anything out of scope for the current step. Expected: dashboard = SKIP-MINIMAL (you build), three.js = BORROW, video relay and YOLO = BORROW, follow-me controller = LEARN.

## Schedule and scope guard
Week 1 (2026-10-20 → 10-26) is Python + API + SQLite + thin React; week 2 is the Go port. Keep a running list of what's done and cut, and tell me honestly when something won't fit. Cut order if short: 3D view, battery, STATUSTEXT, goto/RTL, HealthManager. Towards the end of week 1, help me write the one-page port spec from the handoff (design decisions: single consumer, ACK matching by key, register-before-send, state-confirmed completion, submit/execute split, status rows instead of HTTP errors after the 202, lifespan cleanup order).

## Start
Do step 1 (status check, no edits), then wait for me.
