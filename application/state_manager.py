import logging
import time
from domain.models import VehicleState, Position, Velocity
from domain.updates import HeartbeatUpdate, GlobalPositionIntUpdate, BatteryStatusUpdate
from dataclasses import asdict
import json
import threading
import copy

logger = logging.getLogger(__name__)


class StateManager():
    def __init__(self, verbose: bool = False):
        self._lock = threading.Lock()
        self.verbose = verbose
        self.vehicles: dict[int, VehicleState] = {}

    def apply_update(self, update):
        with self._lock:
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
                logger.warning("Received unknown update type: %s", type(update))
        if self.verbose:
            self.print_vehicle(update.system_id)

    def get(self, system_id: int):
        with self._lock:
            return_vehicle = self.vehicles.get(system_id)
            if return_vehicle is not None:
                return copy.deepcopy(return_vehicle)
            else: return None

    def get_all_vehicles(self):
        with self._lock:
            return_all_vehicles = self.vehicles
            return copy.deepcopy(return_all_vehicles)



    def print_vehicle(self, system_id: int):
        print(json.dumps(asdict(self.get(system_id)), indent=2))