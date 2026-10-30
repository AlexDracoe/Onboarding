# DSETraining — Drone MAVLink Handoff

_Last updated: 2026-10-16_

## How to work with the user

- The user is learning by implementing the code themselves.
- Prefer coaching, review, and small incremental steps over turnkey implementations.
- Do not dump a complete implementation unless explicitly requested.
- The user wants to focus on functionality for now and skip testing frameworks/tests.
- Explain architectural reasoning clearly and keep scope focused.

## Project goal

Build a Python drone control service for ArduPilot SITL using `pymavlink`, with a clean architecture that keeps `pymavlink` at the infrastructure boundary. Domain/application code should not depend directly on `pymavlink`, partly to preserve the option of porting the service to Go later.

High-level flow:

API client → Drone Control Service → MAVLink → ArduPilot

## Working setup

- ArduPilot SITL, MAVProxy, and QGroundControl setup works.
- MAVProxy forwards UDP to `127.0.0.1:14540` for the service.
- `pymavlink` connects using `udpin:127.0.0.1:14540`.
- Keep one persistent MAVLink connection for the service lifetime.
- There must be exactly one inbound message consumer: `Receiver`.
- `CommandHandler` must not independently call `recv_match()`; ACK messages and telemetry are consumed by the same `Receiver`.
- The user successfully armed the SITL vehicle by constructing a MAVLink command message and sending it through the connection wrapper.

## Architecture decisions so far

### Connection and receiver

- A shared connection is appropriate because it represents the MAVLink stream.
- A single `Receiver` reads inbound messages and routes mapped domain updates.
- `PymavlinkConnection.send(message)` should send an already-constructed message using `self.connection.mav.send(message)`.
- Receiver runs an infinite blocking loop and should run in a background thread so the main thread can do other work.
- During development, `thread.join()` can keep the process alive; graceful shutdown is a later concern.

### Mapper and update routing

Mapper currently handles:
- `HEARTBEAT`
- `GLOBAL_POSITION_INT`
- `COMMAND_ACK`
- battery mapping is currently commented out.

The mapper converts MAVLink messages into domain updates. Keep the domain/application layers independent of `pymavlink`.

For `COMMAND_ACK`, the mapper needs to use `message.result` when mapping `MAV_RESULT`; check for typos such as `messsage` vs `message`.

`enum_name()` strips enum prefixes, producing names such as:
- `"COMPONENT_ARM_DISARM"` from `MAV_CMD_COMPONENT_ARM_DISARM`
- `"ACCEPTED"` from `MAV_RESULT_ACCEPTED`

`CommandAckUpdate` currently uses a `result` field (not an older `response` field).

Receiver routing should follow this idea:

```python
if isinstance(update, CommandAckUpdate):
    ack_tracker.handle_ack(update)
elif update is not None:
    state_manager.apply_update(update)
else:
    # Log unknown/unmapped message as appropriate.
```

Do not use `update.get_type` to classify domain updates; `update` is a Python dataclass/domain object, not a pymavlink message. Receiver should depend on the `Connection` protocol rather than a concrete `FakeConnection` type.

Unmapped MAVLink telemetry messages may be logged while debugging; this is not necessarily an error. Avoid duplicate logs from both Mapper and Receiver if it becomes noisy, but don't silently hide ACK-related problems during development.

### State vs command result

Keep these concerns separate:

- `COMMAND_ACK` → `AckTracker`: whether a command was accepted/rejected/reported in progress.
- `HEARTBEAT` and other telemetry → `StateManager`: what the vehicle is actually reporting.
- `VehicleState` is observed current state, not pending command state.
- `VehicleState.is_armed` should be updated from HEARTBEAT, not set directly by a command ACK.
- An accepted ACK does not prove the vehicle is already armed. HEARTBEAT confirms observed armed state.
- If a higher-level API operation eventually needs to wait until the vehicle is actually armed, add a separate state-wait/notification mechanism rather than making AckTracker own vehicle state.
- ArduPilot pre-arm failures may be explained through `STATUSTEXT` (e.g. `"PreArm: ..."`); consider mapping this later for diagnostics.

## Multi-vehicle design discussion

Current recommendation: keep shared application services and per-vehicle state keyed by MAVLink `system_id`.

- `StateManager` owns `dict[int, VehicleState]`, keyed by `system_id`.
- One shared connection and one receiver handle the common MAVLink stream.
- `AckTracker` can track pending commands for multiple vehicles.
- `CommandHandler` can serve commands for multiple vehicles.
- For v1, enforce at most one outstanding command per vehicle.
- A per-vehicle facade can be introduced later for a convenient API such as `vehicle.arm()` or `vehicle.get_state()`, while still using the shared connection/receiver internally.
- Avoid separate full service stacks per vehicle until there is a concrete reason.

## CommandHandler current shape

The user's current design has `CommandHandler` methods construct and return pymavlink message objects; caller sends them separately:

```python
command = handler.arm(system_id=1)
connection.send(command)
```

This has worked for ARM in SITL. The user prefers this separation for now.

Potential future flow once ACK tracking is implemented:
1. Register a pending ACK before sending.
2. Send the message.
3. Wait for matching ACK with a timeout.
4. Return/raise an appropriate outcome.
5. Clean up pending state on every path, including send failure.

Do not force this integration before the user has implemented the tracker.

Known CommandHandler issues from earlier code:
- `set_mode()` was broken because it called `self.connection.mode_mapping()` on the wrapper, and referenced undefined `master`. Revisit when requested.
- For ARM/DISARM, `MAV_COMP_ID_AUTOPILOT1` is generally the sensible target component.
- Command message creation uses `MAVLink_command_long_message(...)`; sending an already-constructed object should use generic `.mav.send(message)`, not `command_long_send(...)`.
- TAKEOFF/GOTO command parameters and target component should be revisited if those commands become the focus. Avoid scope creep.

## AckTracker: current task

The user asked:
1. Should HEARTBEAT updates also be routed to AckTracker?
2. Should AckTracker be a state machine?
3. Should every vehicle own its own services or should services be shared?
4. Asked for an AckTracker skeleton.

Decisions/explanations:
- Do **not** route HEARTBEAT directly to AckTracker. Route ACK updates to AckTracker and heartbeat/state updates to StateManager.
- AckTracker is best started as a small pending-command tracker, not a general vehicle state machine.
- Suggested minimal pending lifecycle:
  - registered / waiting for ACK
  - ACK received (store result and signal waiter)
  - timeout (wake caller with timeout outcome)
- `MAV_RESULT_IN_PROGRESS` may not be terminal; later decide whether to keep waiting for a terminal ACK or apply a timeout.
- Match by `(system_id, command)` for v1, given at most one outstanding command per vehicle.
- Ensure command identifier representation is consistent between `CommandHandler` and `CommandAckUpdate`: either use normalized strings such as `"COMPONENT_ARM_DISARM"` on both sides or use numeric MAVLink command IDs consistently.
- Register before sending to avoid a race where ACK arrives before registration.
- Use a `threading.Event` per pending command and a `threading.Lock` for the shared pending dictionary. Do not hold the lock while blocking on `Event.wait(timeout)`.
- Ensure pending entries are cleaned up on ACK, timeout, and send failure.
- Receiver calls `ack_tracker.handle_ack(update)`.
- The caller of the command waits via `ack_tracker.wait_for_ack(...)`.
- No dedicated timeout thread is required; `Event.wait(timeout)` handles the waiting caller's timeout.

Skeleton previously provided (intentionally incomplete; user is expected to implement it):

```python
from dataclasses import dataclass, field
from threading import Event, Lock


@dataclass
class PendingCommand:
    system_id: int
    command: str
    event: Event = field(default_factory=Event)
    result: str | None = None


class AckTracker:
    def __init__(self):
        # Key: (system_id, command)
        # Value: PendingCommand
        self.pending = {}
        self.lock = Lock()

    def register(self, system_id: int, command: str):
        # TODO:
        # 1. Build the key.
        # 2. Check whether a command with that key is pending.
        # 3. Create and store a PendingCommand.
        # 4. Return the pending command to the caller.
        raise NotImplementedError

    def handle_ack(self, update):
        # Called by Receiver when a CommandAckUpdate arrives.
        # TODO:
        # 1. Build the matching key from the update.
        # 2. Find the pending command.
        # 3. Store the ACK result.
        # 4. Signal the pending command's event.
        raise NotImplementedError

    def wait_for_ack(self, pending: PendingCommand, timeout: float):
        # TODO:
        # 1. Wait on pending.event with the timeout.
        # 2. Distinguish timeout from a received result.
        # 3. Return or raise an appropriate outcome.
        # 4. Remove the pending entry safely.
        raise NotImplementedError
```

The last prompt for the user was to implement `register()` and `handle_ack()` first, and reason about this matching question:

> If system 1 and system 2 both have a pending `COMPONENT_ARM_DISARM` command, what key will let `handle_ack()` find the correct `PendingCommand` without waking the wrong caller?

## How to continue

- Continue from the AckTracker exercise if the user provides code or asks a follow-up.
- Be supportive, practical, and incremental.
- Avoid adding a full implementation unless asked explicitly.
- User asked to skip tests for now, so don't introduce a testing framework unless they change their mind.
