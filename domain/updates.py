from dataclasses import dataclass, asdict, fields
import json
from domain.models import Position, Velocity, BatteryState

@dataclass
class HeartbeatUpdate():
    system_id: int = None
    component_id: int = None
    vehicle_type: str = None
    autopilot: str = None
    is_armed: bool = None
    flight_mode: str = None
    system_status: str = None


@dataclass
class GlobalPositionIntUpdate():
    system_id:int = None
    time_boot_ms:int = None
    position: Position = None
    velocity: Velocity = None
    heading:float|None = None

@dataclass
class BatteryStatusUpdate():
    system_id: int = None
    batteries: BatteryState = None

@dataclass
class CommandAckUpdate():
    system_id:int = None
    command:str = None
    result:str = None