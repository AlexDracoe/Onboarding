"""Rules for each command's JSON body. One model per command; the spec points at its model.

extra="forbid" rejects keys we don't know, so a typo like {"altitde": 10} is an error, not silently ignored.
"""
from pydantic import BaseModel, ConfigDict, Field, ValidationError

from domain.geo import distance_m

# Keep in step with TAKEOFF_MIN_ALT / TAKEOFF_MAX_ALT in web/src/modes.ts.
TAKEOFF_MIN_ALT = 2.0
TAKEOFF_MAX_ALT = 100.0
MAX_ALT = 400.0
# Farthest goto target we accept from the vehicle's current position.
MAX_GOTO_DISTANCE_M = 500.0

class NoParams(BaseModel):
    """arm/ disarm: no body allowed"""
    model_config = ConfigDict(extra="forbid")

class SetModeParams(BaseModel):
    model_config = ConfigDict(extra="forbid")
    mode: str = Field(min_length=1)

class TakeoffParams(BaseModel):
    model_config = ConfigDict(extra="forbid")
    altitude: float = Field(ge=TAKEOFF_MIN_ALT, le=TAKEOFF_MAX_ALT,allow_inf_nan=False)

def describe(error: ValidationError)-> str:
    """One Readable line"""
    parts = []
    for e in error.errors():
        field = ".".join(str(x) for x in e['loc'])
        parts.append(f"{field}: {e['msg']}" if field else e["msg"])
    return ";".join(parts)

class InvalidParams(Exception):

    def __init__(self, name:str, detail:str):
        self.name = name
        self.detail = detail
        super().__init__(f"Invalid params for {name}: {detail}")

class GotoParams(BaseModel):
    model_config = ConfigDict(extra="forbid")
    lat: float = Field(ge=-90, le=90, description="Latitude between -90 and 90")
    long: float = Field(
        ge=-180, le=180, description="Longitude between -180 and 180"
    )
    altitude: float = Field(ge=TAKEOFF_MIN_ALT, le=MAX_ALT,allow_inf_nan=False)


def check_goto_distance(params: dict, lat: float | None, long: float | None) -> None:
    """Needs the vehicle's position, which is not in the body, so Pydantic can't do it.
    lat/long are the vehicle's current position (None until the first GPS fix)."""
    if lat is None or long is None:
        raise InvalidParams("goto", "vehicle has no position fix yet")
    d = distance_m(lat, long, params["lat"], params["long"])
    if d > MAX_GOTO_DISTANCE_M:
        raise InvalidParams("goto", f"target is {d:.0f} m away, max is {MAX_GOTO_DISTANCE_M:.0f} m")
