"""Thin routes over the application layer. No logic here beyond translating to/from HTTP.

Handlers are plain `def`, so FastAPI runs them in its thread pool (that is why StateManager and
SqliteCommandLog are locked). The objects they use are created once at startup and stored on
`app.state` (state_manager, command_log, service); see the lifespan.

Contract:
  POST /vehicles/{system_id}/commands/{name}  -> 202 {command_id, status}; 404 UNKNOWN_COMMAND
  GET  /vehicles/{system_id}                  -> state snapshot; 404 NO_VEHICLE until a heartbeat arrives
  GET  /commands?system_id=&limit=            -> newest first
  GET  /commands/{command_id}                 -> one record; 404 UNKNOWN_COMMAND_ID
Failures after the 202 (ack timeout, rejected, ...) are reported in the record's status/error.
"""
from dataclasses import asdict
from fastapi import APIRouter, Body, Depends, Query, Request
from fastapi.responses import JSONResponse

from api.schemas import CommandAccepted, CommandRecord, ErrorBody, VehicleStateOut
from application.vehicle_service import CommandRequest, UnknownCommand
from application.command_params import InvalidParams
router = APIRouter()

_NOT_FOUND = {404: {"model": ErrorBody}}


def get_service(request: Request):
    return request.app.state.service


def get_state_manager(request: Request):
    return request.app.state.state_manager


def get_command_log(request: Request):
    return request.app.state.command_log


def _error(status_code: int, code: str, detail: str) -> JSONResponse:
    """Error body is exactly ErrorBody (not FastAPI's {"detail": ...} wrapper)."""
    return JSONResponse(status_code=status_code, content=ErrorBody(error=code, detail=detail).model_dump())


@router.post(
    "/vehicles/{system_id}/commands/{name}",
    status_code=202,
    response_model=CommandAccepted,
    responses=_NOT_FOUND,
)

def submit_command(system_id: int, name: str, params: dict | None = Body(None), service=Depends(get_service)):
    try:
        command_id = service.submit( CommandRequest(name, system_id, params or {}))
    except UnknownCommand as e:
        return _error(404, "UNKNOWN_COMMAND", str(e))
    except InvalidParams as e:
        return _error(422, "INVALID_PARAMS", e.detail)
    return CommandAccepted(command_id=command_id, status="PENDING")


@router.get("/vehicles/{system_id}", response_model=VehicleStateOut, responses=_NOT_FOUND)
def get_vehicle(system_id: int, state_manager=Depends(get_state_manager)):
    state = state_manager.get(system_id)
    if state is None:
        return _error(404, "NO_VEHICLE", f"No telemetry from system {system_id} yet")
    return VehicleStateOut(**asdict(state))

@router.get("/vehicles", response_model=list[VehicleStateOut])
def list_vehicles(state_manager=Depends(get_state_manager)):
    snapshot_vehicles = state_manager.get_all_vehicles()
    return [VehicleStateOut(**asdict(v)) for v in snapshot_vehicles.values()]

@router.get("/commands", response_model=list[CommandRecord])
def list_commands(
    system_id: int | None = None,
    limit: int = Query(50, ge=1, le=500),
    command_log=Depends(get_command_log),
):
    return command_log.list_recent(limit=limit, system_id=system_id)


@router.get("/commands/{command_id}", response_model=CommandRecord, responses=_NOT_FOUND)
def get_command(command_id: str, command_log=Depends(get_command_log)):
    record = command_log.get(command_id)
    if record is None:
        return _error(404, "UNKNOWN_COMMAND_ID", f"No command {command_id}")
    return record
