# Drone MAVLink Project — Chat Handoff

## Purpose of this file

This file captures the current project state and the important architectural/learning decisions from the previous chat so a new chat can continue without reconstructing the context.

The user prefers to **learn by writing the code themselves**. The assistant should primarily coach, review, explain, and point out issues rather than dumping turnkey implementations unless explicitly requested.

---

# 1. Overall Project Goal

Build a service that communicates with an ArduPilot drone using MAVLink.

Target architecture:

```text
API Client
    |
    | HTTP
    v
Drone Control Service
    |
    | MAVLink
    v
ArduPilot
    |
    v
Drone
```

Planned service capabilities:

- Telemetry
- Vehicle state
- Arm / disarm
- Flight mode changes
- Takeoff
- Goto
- RTL (Return to Launch)
- Simple missions
- Communication-loss detection

The user eventually may port the project from Python to Go, so the application should avoid being tightly coupled to pymavlink outside the MAVLink infrastructure boundary.

---

# 2. Week 1 Learning Goal

The original Week 1 focus was:

- Linux basics
- Networking basics
- ArduPilot SITL
- MAVLink fundamentals
- Understanding how SITL reaches another application

Linux commands of interest:

```text
ps
kill
grep
find
ssh
curl
ip
ping
ss
tcpdump
```

Networking topics:

- IP addresses and interfaces
- localhost
- ports and sockets
- TCP vs UDP
- client/server communication
- routing tables
- basic firewalls

The user has already completed the main TCP and UDP networking experiments and now understands the basic packet/process/socket relationship.

A useful mental model established:

```text
Application
    -> destination IP
    -> routing table
    -> network interface
    -> packet
```

---

# 3. Host Environment

OS:

- Omarchy Linux
- Uses `pacman`

Project working directory:

```text
~/DSETraining
```

Username observed in shell examples:

```text
alex
```

Important: this is just environment context for continuing commands; do not unnecessarily repeat personal identifiers in user-facing prose.

ArduPilot SITL is installed and working.
QGroundControl is installed and working.

QGroundControl was built from source and successfully opened and connected to MAVProxy.

QGroundControl package/build artifacts included a version around 5.2.0.

---

# 4. Current SITL / MAVProxy / QGroundControl Topology

Observed commands/processes:

### ArduPilot SITL

```bash
python Tools/autotest/sim_vehicle.py -v ArduCopter --console --map -w
```

The simulated ArduCopter listens on:

```text
tcp:0.0.0.0:5760
```

MAVProxy connects to it using:

```text
tcp:127.0.0.1:5760
```

### MAVProxy

Observed invocation:

```bash
/home/alex/DSETraining/ardupilot/.venv/bin/python \
/home/alex/DSETraining/ardupilot/.venv/bin/mavproxy.py \
--retries 5 \
--out 127.0.0.1:14550 \
--master tcp:127.0.0.1:5760 \
--sitl 127.0.0.1:5501 \
--map \
--console
```

Relevant endpoints:

```text
master: tcp:127.0.0.1:5760
SITL-related: 127.0.0.1:5501
output to QGC: 127.0.0.1:14550
```

MAVProxy's local UDP source port was observed as an ephemeral port:

```text
127.0.0.1:40996
```

This is an important distinction:

- `40996` is MAVProxy's local/source UDP port in the observed socket.
- It is **not** the stable port the application should bind to.
- The drone control service should get its own stable destination port, e.g. `14540`.

### QGroundControl

QGC was observed listening on:

```text
0.0.0.0:14550/UDP
```

Therefore the intended separation is:

```text
ArduPilot SITL
      |
      v
   MAVProxy
    /    \
   /      \
14550    14540
  |         |
 QGC     Drone Control Service
```

The next practical step is to configure MAVProxy with another output such as:

```text
--out 127.0.0.1:14540
```

or an explicitly expressed UDP form such as:

```text
--out=udp:127.0.0.1:14540
```

Pymavlink can then listen with:

```python
mavutil.mavlink_connection('udpin:127.0.0.1:14540')
```

Official pymavlink examples use the `udpin:127.0.0.1:14540` style.

---

# 5. Network Interfaces Observed

`ip addr` showed roughly:

```text
lo
  127.0.0.1/8
  ::1/128

wlo1
  172.20.10.155/24
  state UP
```

A Docker bridge also existed:

```text
docker0
  172.17.0.1/16
```

Routing included:

```text
default via 172.20.10.1 dev wlo1
172.20.10.0/24 dev wlo1
172.17.0.0/16 dev docker0
```

Docker bridge is not central to the current work yet.

---

# 6. Networking Experiments Already Completed

The user completed basic TCP and UDP experiments.

### TCP

Used:

```bash
python3 -m http.server 8000
```

and:

```bash
curl ...
```

Observed TCP connection setup and teardown, including:

```text
SYN
SYN-ACK
ACK
HTTP data
FIN
```

### Process / socket relationship

A temporary HTTP server had a PID and a socket file descriptor under `/proc/<pid>/fd`.

Example:

```text
python3 -m http.server 8000
PID 115834
```

`/proc/115834/fd` showed a socket such as:

```text
fd 3 -> socket:[319889]
```

This helped establish the connection between:

```text
process -> file descriptor -> socket -> port
```

### UDP

The user completed the UDP experiment and understands that UDP has no TCP-style connection handshake.

---

# 7. tcpdump / MAVLink Observation

`tcpdump` on loopback showed MAVLink UDP traffic approximately like:

```text
127.0.0.1:40996 -> 127.0.0.1:14550
127.0.0.1:14550 -> 127.0.0.1:40996
```

Observed packets had UDP payload lengths such as:

```text
41
36
34
32
53
```

The user manually inspected raw packet bytes and has already decoded simple MAVLink 2 frames.

---

# 8. MAVLink Fundamentals Learned

The user understands:

- MAVLink is the protocol.
- TCP/UDP are transports.
- MAVLink messages are structured packets carried over a transport.
- `HEARTBEAT` is central to discovering a vehicle and monitoring connection/vehicle state.
- A MAVLink **system ID** identifies a system/vehicle.
- A MAVLink **component ID** identifies a component inside that system.
- Multiple components can share the same system ID (autopilot, camera, gimbal, companion computer, etc.).
- Component ID is **not** the vehicle type.
- For current application state, vehicles can be indexed by `system_id`.

### MAVLink 2 framing

MAVLink 2 starts with:

```text
fd
```

A previously observed frame looked like:

```text
fd 09 00 00 11 ff be 00 00 00 ...
```

It was correctly understood as:

- MAVLink 2
- payload length = 9
- sequence = 17
- system ID = 255
- component ID = 190
- message ID = 0
- message = `HEARTBEAT`

Another 32-byte payload frame was identified as `VIBRATION`, message ID 241, system 1, component 1.

The user now has enough protocol understanding to proceed with application work rather than spending more time manually decoding raw frames for every message.

---

# 9. Pymavlink Decision

Project instructions require **pymavlink**.

Important distinction:

```text
pymavlink != MAVSDK
```

Pymavlink is the selected library.

Pymavlink is allowed in the infrastructure/MAVLink boundary only.

The goal is for application/domain code not to know that pymavlink exists.

FastAPI will eventually provide HTTP.

---

# 10. Pymavlink Basics Already Learned

Basic import:

```python
from pymavlink import mavutil
```

Connection factory:

```python
mavutil.mavlink_connection(...)
```

UDP receive endpoint example:

```python
mavutil.mavlink_connection('udpin:127.0.0.1:14540')
```

Important concepts:

- `mavlink_connection()` creates a pymavlink connection object.
- `wait_heartbeat()` blocks until a heartbeat is received.
- `recv_match()` receives the next matching parsed MAVLink message.
- `recv_match()` supports things such as message type, condition, blocking, and timeout.
- Baudrate matters primarily for serial links; it is generally not relevant to the localhost UDP setup being used here.

The user successfully used the ArduPilot virtualenv Python:

```text
/home/alex/DSETraining/ardupilot/.venv/bin/python
```

---

# 11. Current High-Level Software Architecture

Current intended package structure:

```text
project/
├── api/
│   ├── routes
│   └── schemas
├── application/
│   ├── vehicle_service
│   ├── command_handler
│   ├── state_manager
│   ├── ack_tracker
│   └── health_manager
├── domain/
│   ├── models
│   ├── updates
│   ├── enums
│   └── errors
├── infrastructure/
│   └── mavlink/
│       ├── pymavlink_connection
│       ├── receiver
│       └── mapper
├── config.py
├── logging_config.py
├── main.py
└── tests/
    ├── unit
    ├── integration
    └── fixtures
```

Tests exist conceptually and some StateManager tests were started, but the user explicitly decided to **skip the testing work for now** and continue with functionality.

Do not force a test-first workflow at this point.

---

# 12. Dependency Inversion / Boundary Decision

The receiver should depend on a **behavioral connection interface/protocol**, not on concrete pymavlink implementation classes.

Conceptually:

```text
Receiver
   |
   | depends on Connection behavior
   v
Connection protocol
   ^
   |
   +-------------------------+
   |                         |
PymavlinkConnection       FakeConnection
```

However, production `Receiver` should not need to know about `FakeConnection`.

`main.py` should construct and wire concrete objects.

Python `Protocol` is appropriate because structural typing means a class can satisfy an interface based on its methods/behavior without needing explicit inheritance.

---

# 13. Current Connection Abstraction

Current conceptual `PymavlinkConnection` implementation:

```python
from pymavlink import mavutil

class PymavlinkConnection(Connection):
    def __init__(self):
        self.connection = None

    def connect(self, connection_string):
        self.connection = mavutil.mavlink_connection(connection_string)

    def wait_for_heartbeat(self):
        if self.connection:
            self.connection.wait_heartbeat()
            print(
                "Heartbeat from system (system %u component %u)"
                % (
                    self.connection.target_system,
                    self.connection.target_component,
                )
            )
        else:
            raise Exception("Not connected")

    def disconnect(self):
        if self.connection:
            self.connection.close()
            self.connection = None

    def receive(self):
        if self.connection:
            return self.connection.recv_match(blocking=True)
        else:
            raise Exception("Not connected")
```

This is a conceptual snapshot, not necessarily the exact final code.

Important decision:

`connect()` and `wait_for_heartbeat()` should remain separate responsibilities.

---

# 14. FakeConnection / Testing Decision

A simple fake connection was created for experimentation:

```python
class FakeConnection(Connection):
    def __init__(self):
        self.connected = True

    def connect(self, ...):
        self.connection = None

    def wait_for_heartbeat(self, ...):
        print("Simulated heartbeat received.")

    def receive(self, ...):
        return "Fake message"
```

It is only a development/test double. It is not important to the next immediate task.

The user chose to skip expanding tests for now.

---

# 15. Receiver Design

Current conceptual receiver:

```python
class Receiver:
    def __init__(self, connection, mapper):
        self.connection = connection
        self.mapper = mapper

    def start(self):
        while True:
            message = self.connection.receive()
            self.mapper.map(message)
```

The intended next evolution is:

```text
connection.receive()
        |
        v
      Mapper
        |
        v
   Domain Update
        |
        +--> StateManager
        |
        +--> ACK tracker / event handling
```

Important concurrency rule:

> **One persistent MAVLink connection should exist for the service lifetime, and one receiver should be the sole consumer of inbound MAVLink messages.**

Do not have the command handler independently consume from the socket with `recv_match()`.

Why:

- Multiple readers competing for the same socket can consume each other's messages.
- Telemetry is latest-value-oriented.
- `COMMAND_ACK` is event-oriented and needs routing/correlation.

The connection should be application-scoped rather than a global singleton so it remains testable and replaceable.

A clean shutdown mechanism can be added later (event/flag rather than `while True`).

---

# 16. Mapper Design

The Mapper translates pymavlink messages into domain/application update dataclasses.

The mapper **is allowed to know pymavlink**.

Domain classes should not import pymavlink.

Current rough dispatch shape:

```python
def map(self, message):
    if message.get_type() == 'HEARTBEAT':
        return self.map_heartbeat(message)
    elif message.get_type() == 'GLOBAL_POSITION_INT':
        return self.map_global_position_int(message)
    elif message.get_type() == 'BATTERY_STATUS':
        return self.map_battery_status(message)
    else:
        print(...)
        return None
```

Unsupported messages should not crash the system.

The receiver can ignore `None` mappings and optionally debug-log the unsupported message type.

Use explicit imports rather than wildcard imports.

---

# 17. HEARTBEAT Mapping

`HeartbeatUpdate` conceptually contains:

```text
system_id: int
component_id: int
vehicle_type: str
autopilot: str
is_armed: bool
flight_mode: str
system_status: str
```

Important mapping details:

- `system_id` comes from the message source.
- `component_id` comes from the message source.
- MAV_TYPE is mapped to an application-friendly name such as `QUADROTOR`.
- MAV_AUTOPILOT is mapped to a name such as `ARDUPILOTMEGA`.
- Armed state uses:

```python
message.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED
```

- Flight mode can be obtained with:

```python
mavutil.mode_string_v10(message)
```

- MAV_STATE is converted to an application-friendly string such as `STANDBY`.

---

# 18. GLOBAL_POSITION_INT Mapping

`GlobalPositionIntUpdate` conceptually contains:

```text
system_id: int
time_boot_ms: int
position: Position
velocity: Velocity
heading: float | None
```

MAVLink scaling handled in mapper:

```text
lat / 1e7
lon / 1e7
alt / 1e3
relative_alt / 1e3
vx / 1e2
vy / 1e2
vz / 1e2
```

Heading:

```text
65535 = unknown
otherwise / 100
```

`time_boot_ms` is elapsed time since boot, not wall-clock time. Retain it in the update if useful to the application.

---

# 19. Battery Mapping / Domain Direction

Battery support was being designed when the conversation paused.

Recommended conceptual approach:

```text
BatteryStatusUpdate
    -> system_id
    -> BatteryState
```

rather than copying every MAVLink field directly onto VehicleState.

MAVLink supports multiple battery instances, so the domain should preferably support:

```python
VehicleState.batteries: dict[int, BatteryState]
```

where the key is `battery_id`.

Recommended application-friendly fields include:

```text
battery_id: int
temperature_c: float | None
total_voltage_v: float | None
current_a: float | None
cell_voltages_v: list[float]
battery_remaining_percent: int | None
time_remaining_s: int | None
fault_bitmask: int
```

`fault_bitmask` is conceptually an integer corresponding to the MAVLink `uint32_t` field.

Potential mapper issues already noticed in the user's rough battery code:

- `getsrcComponent()` is wrong; pymavlink method is `get_srcComponent()`.
- `msg.battery_function` was referenced incorrectly / undefined in the rough code.
- Some type annotations were malformed, e.g. `float:` / `float[]` / `None`.
- Battery voltage/cell voltage fields may contain sentinel values and should be handled consciously.

---

# 20. Domain Models

The user moved toward explicit domain dataclasses.

### Position

```text
latitude: float | None
longitude: float | None
altitude: float | None
relative_altitude: float | None
```

### Velocity

```text
velocity_x: float | None
velocity_y: float | None
velocity_z: float | None
```

### VehicleState

Conceptually:

```text
system_id: int | None
component_id: int | None
vehicle_type: str | None
autopilot: str | None
is_armed: bool | None
flight_mode: str | None
system_status: str | None
heartbeat_count: int = 0
position: Position | None
velocity: Velocity | None
heading: float | None
last_heartbeat_update_time: float | None
last_gpiu_update_time: float | None
batteries / battery state
```

Important design decision:

> `VehicleState` is the current aggregate state, not a storage object for update messages.

So don't simply do:

```python
self.heartbeat = heartbeat_update
```

as the main design.

Instead, explicit fields in the aggregate are updated by `StateManager`.

The user prefers telemetry fields to be `None` until actually received, rather than initializing unknown telemetry to `0.0`.

Counters can legitimately start at `0`.

`VehicleState` should be a dataclass primarily for clear typed data storage, generated init/repr/eq behavior, and explicit defaults.

Avoid combining `@dataclass` with a manually written `__init__` unless there is a strong reason, because doing so defeats much of the benefit of the dataclass.

Nested dataclasses for `Position` and `Velocity` are considered a good model.

When a position update arrives, replacing the nested object is preferred:

```python
vehicle.position = update.position
vehicle.velocity = update.velocity
```

while the top-level VehicleState is mutated incrementally by StateManager.

---

# 21. StateManager

StateManager owns the dictionary of current vehicle state:

```python
self.vehicles: dict[int, VehicleState]
```

Vehicles are indexed by `system_id`.

Component ID does not need to be part of the primary vehicle key because many components can belong to the same system.

Current conceptual logic:

```python
class StateManager:
    def __init__(self):
        self.vehicles: dict[int, VehicleState] = {}

    def apply_update(self, update):
        if update.system_id not in self.vehicles:
            self.vehicles[update.system_id] = VehicleState()

        vehicle = self.vehicles[update.system_id]

        if isinstance(update, HeartbeatUpdate):
            vehicle.system_id = update.system_id
            vehicle.component_id = update.component_id
            vehicle.vehicle_type = update.vehicle_type
            vehicle.autopilot = update.autopilot
            vehicle.is_armed = update.is_armed
            vehicle.flight_mode = update.flight_mode
            vehicle.system_status = update.system_status
            vehicle.heartbeat_count += 1
            vehicle.last_heartbeat_update_time = time.monotonic()

        elif isinstance(update, GlobalPositionIntUpdate):
            vehicle.system_id = update.system_id
            vehicle.position = update.position
            vehicle.velocity = update.velocity
            vehicle.heading = update.heading
            vehicle.last_gpiu_update_time = time.monotonic()

        elif isinstance(update, BatteryStatusUpdate):
            # apply battery state
            pass

        else:
            print(...)
```

One refinement previously discussed:

When creating a new VehicleState, set its `system_id` immediately so each update handler doesn't have to repeat that assignment.

Also preserve unrelated state when applying a specific update.

Example:

- Apply heartbeat.
- Apply position update.
- Heartbeat information remains intact.

The user was also deciding whether this style is somewhat MVC-like. The answer: yes, there is a controller/model resemblance, but the whole architecture is not strict MVC. `VehicleState` is model-like, `StateManager` is controller-ish, and command handling is separate behavior/control logic.

Do not replace StateManager with generic `dataclasses.asdict()` + `setattr()` merging as the primary design. Explicit update-type handling is preferred because it keeps domain rules, validation, and stale-data behavior visible.

`asdict()` is still fine for debugging/JSON printing.

Example debugging approach:

```python
json.dumps(asdict(vehicle), indent=2)
```

Later FastAPI/Pydantic should handle actual HTTP JSON serialization rather than putting `to_json()` methods into every domain model.

---

# 22. Current End-to-End Data Pipeline

This pipeline was successfully demonstrated against real SITL:

```text
MAVProxy
   |
   | UDP 127.0.0.1:14540
   v
pymavlink
   |
   v
Receiver
   |
   v
Mapper
   |
   v
Domain Updates
   |
   v
StateManager
   |
   v
VehicleState
   |
   v
JSON / console output
```

Example state output demonstrated:

```json
{
  "system_id": 1,
  "component_id": 1,
  "vehicle_type": "QUADROTOR",
  "autopilot": "ARDUPILOTMEGA",
  "is_armed": false,
  "flight_mode": "STABILIZE",
  "system_status": "STANDBY",
  "heartbeat_count": 2,
  "position": {
    "latitude": -35.363262,
    "longitude": 149.1652373,
    "altitude": 584.08,
    "relative_altitude": -0.006
  },
  "velocity": {
    "velocity_x": 0.01,
    "velocity_y": -0.01,
    "velocity_z": 0.0
  },
  "heading": 352.44
}
```

The exact live values will of course change on later runs.

Unsupported messages seen included things like:

```text
LOCAL_POSITION_NED
VIBRATION
ESC_TELEMETRY_1_TO_4
```

These were ignored/logged without crashing.

---

# 23. Connection Health Design

A key conceptual decision:

`connected` should **not** simply mean the UDP socket is open.

Instead, connection health should be based primarily on recent valid `HEARTBEAT` messages.

Use:

```python
time.monotonic()
```

for elapsed-time / timeout logic.

Possible high-level health model:

```text
HEALTHY
DEGRADED
DISCONNECTED
```

with heartbeat-age thresholds.

The exact thresholds are not fixed yet.

A command timeout does not automatically mean the whole connection is dead. Repeated failures can be evidence of degraded/lost communication, but command-specific success/retry behavior should be treated carefully.

Avoid a blind policy of retrying every command three times.

---

# 24. Important Concurrency / ACK Design

This is the major area to build next.

Desired architecture:

```text
                   +----------------------+
                   |   MAVLink Connection |
                   +----------+-----------+
                              |
                    inbound messages
                              |
                              v
                         Receiver
                              |
                    +---------+---------+
                    |                   |
                    v                   v
              StateManager         AckTracker

CommandHandler
      |
      | send command through connection
      v
MAVLink Connection
```

Critical rule:

> **Receiver is the sole inbound reader.**

The CommandHandler should send commands through the persistent connection, but should not independently call `recv_match()` to wait for acknowledgments.

Instead, Receiver sees `COMMAND_ACK` and routes it to an `AckTracker` / `CommandTracker`.

This keeps one consumer of incoming messages and prevents telemetry and ACK messages from stealing each other's reads.

For v1, it is reasonable to simplify concurrency by allowing at most one outstanding command for the same vehicle and waiting for its ACK before issuing another.

---

# 25. Immediate Next Step

The user explicitly said they want to **skip the testing stuff for now**.

Next target:

## Build COMMAND_ACK + Arm/Disarm

Start with direct application functionality before adding a full FastAPI surface.

Desired flow:

```text
CommandHandler
    |
    | send ARM/DISARM command
    v
pymavlink / MAVLink connection
    |
    v
ArduPilot SITL
    |
    | COMMAND_ACK
    v
Receiver
    |
    v
AckTracker
    |
    v
CommandHandler receives result
```

Then verify the new state through heartbeat messages:

```text
HEARTBEAT
   -> is_armed changes
```

This teaches the important distinction between:

- command request
- command acknowledgment
- resulting vehicle state

That distinction will matter for every later command.

Suggested progression after ARM/DISARM:

```text
1. COMMAND_ACK infrastructure
2. Arm / disarm
3. Mode change
4. Takeoff
5. RTL
6. Goto
7. Simple missions
8. Connection-loss handling
9. FastAPI HTTP layer
10. Config / logging / error handling
11. Docker / packaging
```

Do not jump into missions before the command/ACK/state pattern is clear.

---

# 26. Suggested Arm/Disarm Concept

The command will likely use MAVLink's command-long mechanism with:

```text
MAV_CMD_COMPONENT_ARM_DISARM
```

Conceptually:

```text
ARM  -> param1 indicates arm
DISARM -> param1 indicates disarm
```

The exact pymavlink call syntax should be checked from the installed/generated MAVLink API rather than assumed from memory if there is any uncertainty.

After sending:

1. Track the expected command.
2. Receiver processes incoming `COMMAND_ACK`.
3. AckTracker resolves success/failure.
4. CommandHandler returns the command result.
5. Heartbeat confirms the resulting `is_armed` state.

Acknowledge status and resulting state are related but not identical concepts.

---

# 27. HTTP/API Direction

The user does not want to focus heavily on HTTP yet.

A v1 API can be simple and synchronous-ish, for example:

```text
POST /vehicles/{system_id}/arm
POST /vehicles/{system_id}/disarm
POST /vehicles/{system_id}/mode
```

The important part is the internal architecture, not elaborate REST design.

Later, long-running actions may require distinguishing:

```text
accepted
vs
completed
```

For now, direct command execution against SITL is preferable before exposing it over FastAPI.

---

# 28. QGroundControl Build / Docker Notes

QGroundControl source was cloned under:

```text
~/DSETraining/qgroundcontrol
```

from the official GitHub repository.

Build artifacts included:

```text
/project/build/qgroundcontrol_5.2.0-95.gf2f163196_amd64.deb
```

QGC runtime was installed under:

```text
/opt/QGroundControl
```

The user initially hit:

```text
error while loading shared libraries: libQt6Bluetooth.so.6
```

but later confirmed:

```text
it opened and connected to mavproxy
```

QGroundControl repository contains Docker deployment material under:

```text
qgroundcontrol/deploy/docker/docker-compose.yml
```

The repository root did not contain a compose file, so:

```bash
cd ~/DSETraining
 docker compose up
```

failed because there was no compose config there.

The Docker compose file found in QGC has a service such as `ubuntu`, mounts source/build paths, and is intended primarily as a build environment.

Docker/containerization is a later concern. Do not let it distract from the command path yet.

---

# 29. Style / Coaching Preferences

The user learns best when:

- Concepts are explained clearly.
- They write the code.
- The assistant reviews their code and points out bugs/design problems.
- The assistant does not immediately provide a giant finished implementation.
- The next task is small enough to reason about.

Avoid over-focusing on tests right now because the user explicitly chose to defer them.

Avoid asking unnecessary clarification questions; make reasonable assumptions and move forward.

---

# 30. Best Starting Prompt for the New Chat

Paste this file into a new chat, then say something like:

> Continue from this handoff. I skipped testing for now. I want to build the COMMAND_ACK infrastructure and then ARM/DISARM against ArduPilot SITL. Coach me while I write the code instead of giving me the whole solution at once.

That should be enough to resume directly at the correct point.

---

# 31. Current Status in One Sentence

The telemetry/state pipeline is working end-to-end against ArduPilot SITL, the user understands the underlying Linux/networking/MAVLink concepts, testing is intentionally deferred, and the next implementation target is a single-reader `COMMAND_ACK` path supporting ARM/DISARM.
