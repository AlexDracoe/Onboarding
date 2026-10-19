import time
from domain.models import VehicleState, Position, Velocity
from domain.updates import HeartbeatUpdate, GlobalPositionIntUpdate, BatteryStatusUpdate
from dataclasses import asdict
import json
class StateManager():
    def __init__(self, verbose: bool = False):
        self.verbose = verbose
        self.vehicles: dict[int, VehicleState] = {}

    def apply_update(self, update):
        if update.system_id not in self.vehicles:
            self.vehicles[update.system_id] = VehicleState()
        vehicle = self.vehicles[update.system_id]
        #print(update)
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
            vehicle.velocity =  update.velocity
            vehicle.heading = update.heading
            vehicle.last_gpiu_update_time = time.monotonic()
        elif isinstance(update, BatteryStatusUpdate):
            vehicle.system_id = update.system_id
            vehicle.batteries=update.batteries
        else:
            print(f"Received unknown update type: {type(update)}")
        if self.verbose:
            self.print_vehicle(update.system_id)

    #def get_or_create_vehicle(self):

    def print_vehicle(self, system_id: int):
        print(json.dumps(asdict(self.vehicles[system_id]), indent=2))