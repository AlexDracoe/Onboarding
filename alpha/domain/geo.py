"""Pure geometry helpers. No I/O, no imports from other layers."""
import math

EARTH_RADIUS_M = 6371000.0


def distance_m(lat1: float, long1: float, lat2: float, long2: float) -> float:
    """Great-circle (haversine) distance in meters between two lat/long points in degrees."""
    rad_lat1 = math.radians(lat1)
    rad_lat2 = math.radians(lat2)
    delta_lat = rad_lat2 - rad_lat1
    delta_long = math.radians(long2 - long1)

    a = (
        math.sin(delta_lat / 2) ** 2
        + math.cos(rad_lat1) * math.cos(rad_lat2) * math.sin(delta_long / 2) ** 2
    )
    c = 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))
    return EARTH_RADIUS_M * c

