import logging
import time
from typing import Callable

from application.state_manager import StateManager
from application.ack_tracker import AckTracker, AckTimeout, CommandAlreadyPending, CommandRejected
from application.command_handler import CommandHandler
from application.command_params import NoParams, SetModeParams, TakeoffParams, GotoParams, InvalidParams, describe, check_goto_distance
from config import CONFIG
from domain.models import VehicleState
from domain.geo import distance_m
from dataclasses import dataclass, field
from pydantic import BaseModel, ValidationError
from copy import replace
import threading


Predicate = Callable[[VehicleState], bool]

@dataclass(frozen=True)
class CommandRequest:
    name: str
    system_id: int
    params: dict = field(default_factory=dict)

@dataclass
class CommandSpec:
    build: Callable[..., object]                       
    ack_name: str
    predicate: Callable[[VehicleState, dict], bool]   
    description: str
    params_model : type[BaseModel]
    timeout: float= CONFIG.ack_timeout_s
    # Optional rule that needs vehicle state: check(params, state_or_None) raises InvalidParams.
    check: Callable[[dict, VehicleState | None], None] | None = None



logger = logging.getLogger(__name__)
STD_ALT_TOLERANCE = 5.0
STD_POS_TOLERANCE = 7.0


class StateTimeout(Exception):
    """The vehicle never reached the expected state before the deadline."""
    def __init__(self, system_id, description, timeout):
        self.system_id = system_id
        self.description =  description
        self.timeout = timeout
        super().__init__(f"System {self.system_id} timed out waiting for {self.description} after {self.timeout}s")

class UnknownCommand(Exception):
    """The requested command name is not in the service's command table."""
    def __init__(self, name):
        self.name = name
        super().__init__(f"Unknown command: {name}")


class VehicleService:
    def __init__(self, state_manager: StateManager, command_handler: CommandHandler, ack_tracker: AckTracker, connection, command_log):
        self.state_manager = state_manager
        self.command_handler =  command_handler
        self.ack_tracker =  ack_tracker
        self.connection = connection
        self.command_log = command_log

        self._commands = {
                    "arm": CommandSpec(
                        build=self.command_handler.arm,
                        ack_name=CommandHandler.ARM_NAME,
                        predicate=lambda s, p: s.is_armed is True,
                        description="Armed",
                        params_model=NoParams
                    ),
                    "disarm": CommandSpec(
                        build=self.command_handler.disarm,
                        ack_name=CommandHandler.ARM_NAME,
                        predicate=lambda s, p: s.is_armed is False,
                        description="Disarmed",
                        params_model=NoParams
                    ),
                    "set_mode": CommandSpec(
                        build=self.command_handler.set_mode,
                        ack_name=CommandHandler.SET_MODE_NAME,
                        predicate=lambda s, p: s.flight_mode ==p["mode"],
                        description="Mode changed",
                        params_model=SetModeParams
                    ),
                    "takeoff": CommandSpec(
                        build=self.command_handler.takeoff,
                        ack_name=CommandHandler.TAKEOFF_NAME,
                        predicate=lambda s, p:(
                            s.position is not None
                            and s.position.relative_altitude is not None
                            and s.position.relative_altitude >= 0.9 * p["altitude"]
                        ),
                        description="Take off to altitude",
                        timeout=30,
                        params_model=TakeoffParams
                    ),
                    "goto": CommandSpec(
                        build=self.command_handler.goto,
                        ack_name=CommandHandler.GOTO_NAME,
                        predicate=lambda s, p:(
                            s.position is not None
                            and s.position.latitude is not None and s.position.longitude is not None and s.position.relative_altitude is not None
                            and distance_m(s.position.latitude, s.position.longitude, p["lat"], p["long"]) <= STD_POS_TOLERANCE
                            and abs(s.position.relative_altitude - p["altitude"]) <= STD_ALT_TOLERANCE
                        ),
                        description="Repositioning",
                        timeout=200,#TODO use distance from start to goal, plus velocity, give a time est and add 20% 
                        params_model=GotoParams,
                        check=lambda p, s: check_goto_distance(
                            p,
                            s.position.latitude if s and s.position else None,
                            s.position.longitude if s and s.position else None,
                        ),
                    ),
        }

    def wait_for_state(self, system_id: int, predicate: Predicate, description: str,
                       timeout: float) -> VehicleState:
        """Poll until predicate(state) is True. Returns the snapshot that satisfied it; raises StateTimeout."""
        deadline = time.monotonic() + timeout 
        while time.monotonic() < deadline:
            snapshot = self.state_manager.get(system_id)
            if snapshot is not None and predicate(snapshot):
                return snapshot
            time.sleep(CONFIG.state_poll_interval_s)

        # the state may have become true during the last sleep (or timeout was <= 0)
        snapshot = self.state_manager.get(system_id)
        if snapshot is not None and predicate(snapshot):
            return snapshot
        raise StateTimeout(system_id, description, timeout)

    def submit(self, request: CommandRequest)-> str:
        spec = self._commands.get(request.name)
        if spec is None:
            raise UnknownCommand(request.name)
    
        try: 
            clean = spec.params_model.model_validate(request.params).model_dump()
        except ValidationError as e:
            raise InvalidParams(request.name, describe(e))

        if spec.check is not None:
            spec.check(clean, self.state_manager.get(request.system_id))

        request = replace(request, params = clean)
        command_id = self.command_log.create(request.system_id, {"name": request.name}, "PENDING")
        
        worker = threading.Thread(
                target=self.execute,
                args=(request, spec, command_id),
                name=f"cmd-{command_id[:8]}",
                daemon=True
            )
        
        worker.start()
        return command_id

    def execute(self, request: CommandRequest, spec: CommandSpec, command_id):
        status, error = "FAILED", None
        try:
            message = spec.build(request.system_id, **request.params)
            pending = self.ack_tracker.register(request.system_id, spec.ack_name)
            try:
                self.connection.send(message)
            except Exception:
                self.ack_tracker.discard(pending)
                raise
            self.ack_tracker.wait_for_ack(pending)
            self.wait_for_state(request.system_id, lambda s: spec.predicate(s, request.params), spec.description, spec.timeout)
            status = "ACHIEVED"
        except CommandAlreadyPending as e:
            status, error = "ALREADY_PENDING", str(e)
            logger.warning(str(e))
        except AckTimeout as e:
            status, error = "ACK_TIMEOUT", str(e)
            logger.warning(str(e))
        except CommandRejected as e:
            status, error = "COMMAND_REJECTED", str(e)
            logger.warning(str(e))
        except StateTimeout as e:
            status, error = "STATE_TIMEOUT", str(e)
            logger.warning(str(e))
        except Exception as e:
            error = repr(e)
            logger.exception("command %s failed", command_id)
        finally:
            try:
                self.command_log.update_status(command_id, status, error)
            except Exception:
                logger.exception("could not record outcome for command %s", command_id)
            


# How a caller will use it (for reference, not code to keep here):
#   service = VehicleService(state_manager, command_handler, ack_tracker, connection, command_log)
#   service.execute(CommandRequest("arm", 1))
