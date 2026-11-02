from dataclasses import dataclass, field, asdict
import json

@dataclass
class Position():
    latitude:float |None= None
    longitude:float |None= None
    altitude:float |None= None
    relative_altitude:float |None= None

@dataclass
class Velocity():
    velocity_x:float |None= None
    velocity_y:float |None= None
    velocity_z:float |None= None
@dataclass
class BatteryState():
    component_id: int = None
    battery_id: int = None
    battery_function: str = None
    battery_type: str = None
    charge_state: str = None
    temperature: float = None
    total_voltage: float =None
    cell_voltages: list[float]= None
    current_battery: int = None
    current_consumed: float = None
    energy_consumed: float = None
    battery_remaining: float = None
    time_remaining: float = None
    fault_bitmask: int  = None#
@dataclass
class VehicleState():
    system_id:int |None = None
    component_id:int |None = None
    vehicle_type:str |None = None
    autopilot:str |None= None
    is_armed:bool |None= None
    flight_mode:str |None= None
    system_status:str |None= None
    heartbeat_count:int = 0
    position:Position | None= None
    velocity:Velocity | None= None
    heading:float |None= None

    last_heartbeat_update_time:float |None= None
    last_gpiu_update_time:float |None= None

    batteries:dict[int, BatteryState] | None = None
