from application.state_manager import StateManager
from domain.models import Velocity, Position, VehicleState
from domain.updates import HeartbeatUpdate, GlobalPositionIntUpdate


def test_heartbeat_creates_vehicle():
    manager = StateManager()

    update = HeartbeatUpdate(
        system_id = 1,
        component_id = 1,
        vehicle_type = "Copter",
        autopilot = "MOUSEPILOT",
        is_armed = False,
        flight_mode = "STABILIZE",
        system_status = "NOMINAL"
    )

    manager.apply_update(update)

    vehicle = manager.vehicles[1]

    assert vehicle.system_id == 1
    assert vehicle.component_id == 1
    assert vehicle.vehicle_type == "Copter"
    assert vehicle.autopilot == "MOUSEPILOT"
    assert vehicle.is_armed is False
    assert vehicle.flight_mode == "STABILIZE"
    assert vehicle.system_status == "NOMINAL"
    assert vehicle.heartbeat_count == 1
    #assert manager.vehicles[1] is vehicle


def test_second_heartbeat_updates_existing_vehicles():
    manager = StateManager()

    update = HeartbeatUpdate(
        system_id = 1,
        component_id = 1,
        vehicle_type = "Copter",
        autopilot = "MOUSEPILOT",
        is_armed = False,
        flight_mode = "STABILIZE",
        system_status = "NOMINAL"
    )

    manager.apply_update(update)

    vehicle = manager.vehicles[1]
    
    update2 = HeartbeatUpdate(ls
    
        system_id = 1,
        component_id = 1,
        vehicle_type = "Copter",
        autopilot = "MOUSEPILOT",
        is_armed = True,
        flight_mode = "GUIDED",
        system_status = "NOMINAL"
    )

    manager.apply_update(update2)

    assert len(manager.vehicles) == 1
    assert vehicle.heartbeat_count ==2
    assert vehicle.is_armed == True
    assert vehicle.flight_mode == "GUIDED"

def test_position_update_does_not_overwrite_heartbeat_state():

    manager = StateManager()

    update = HeartbeatUpdate(
    system_id = 1,
    component_id = 1,
    vehicle_type = "Copter",
    autopilot = "MOUSEPILOT",
    is_armed = False,
    flight_mode = "STABILIZE",
    system_status = "NOMINAL"
    )

    manager.apply_update(update)

    vehicle = manager.vehicles[1]

    update2 = GlobalPositionIntUpdate(
        system_id=1,
        time_boot_ms=123,
        position=Position(
            latitude=35.0,
            longitude=-78.0,
            altitude=100.0,
            relative_altitude=20.0,
        ),
        velocity=Velocity(
            velocity_x=2.0,
            velocity_y=3.0,
            velocity_z=-1.0,
        ),
        heading=180.0,
    )

    manager.apply_update(update2)

    assert vehicle.position.latitude == 35.0
    assert vehicle.position.longitude == -78.0
    assert vehicle.velocity.velocity_x == 2.0
    assert vehicle.heading == 180.0

    assert vehicle.is_armed is False

