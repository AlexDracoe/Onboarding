"""HTTP contract. Plain JSON shapes only: no pymavlink types, no Python-specific encodings,
so the React dashboard and the Go port can rely on the same fields.

Times are Unix seconds (wall clock, float), as stored in the DB.
"""
from pydantic import BaseModel


class PositionOut(BaseModel):
    latitude: float | None = None
    longitude: float | None = None
    altitude: float | None = None
    relative_altitude: float | None = None


class VelocityOut(BaseModel):
    velocity_x: float | None = None
    velocity_y: float | None = None
    velocity_z: float | None = None


class VehicleStateOut(BaseModel):
    system_id: int | None = None
    component_id: int | None = None
    vehicle_type: str | None = None
    autopilot: str | None = None
    is_armed: bool | None = None
    flight_mode: str | None = None
    system_status: str | None = None
    heartbeat_count: int = 0
    position: PositionOut | None = None
    velocity: VelocityOut | None = None
    heading: float | None = None
    # Left out on purpose: batteries (not mapped yet) and the monotonic *_update_time fields
    # (not wall-clock, meaningless to a client).


class CommandAccepted(BaseModel):
    """Body of the 202: the command exists and is running; poll GET /commands/{command_id}."""
    command_id: str
    status: str


class CommandRecord(BaseModel):
    """One row of the command log, exactly as SqliteCommandLog returns it."""
    id: str
    system_id: int
    cmd: dict
    status: str
    error: str | None = None
    created_at: float
    updated_at: float


class ErrorBody(BaseModel):
    error: str    # stable machine-readable code, e.g. UNKNOWN_COMMAND, NO_VEHICLE
    detail: str   # human-readable
