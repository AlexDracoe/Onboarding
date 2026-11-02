from infra.mapper import Mapper

def test_heartbeat_creates_vehicle():
    mappper = Mapper()
    manager = StateManager()

    update = mapper.map_heartbeat(
        system_id = 1,
        component_id = 1,
        vehicle_type = "Copter",
        autopilot = "MOUSEPILOT",
        is_armed = False,
        flight_mode = "STABILIZE",
        system_status = "NOMINAL"
    )

    




def test_heartbeat_updates_same_vehicle():



def test_position_update_changes_position_velocity_maintain_heartbeat_state():