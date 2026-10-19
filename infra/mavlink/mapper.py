from domain.updates import HeartbeatUpdate, GlobalPositionIntUpdate , CommandAckUpdate, BatteryStatusUpdate
from domain.models import Position, Velocity, BatteryState
from pymavlink import mavutil

class Mapper:
    def __init__(self):
        self.handlers = {
            "HEARTBEAT": self.map_heartbeat,
            "GLOBAL_POSITION_INT": self.map_global_position_int,
            "COMMAND_ACK": self.map_command_ack,
            #"BATTERY_STATUS":self.map_battery_status,
        }
        pass

    def map(self, message):
        message_type = message.get_type()

        handler = self.handlers.get(message_type)

        if handler:
            return handler(message)
        # Unmapped types return None; Receiver logs them.
        return None

    def enum_name(self, enum_group, value):
        enum = mavutil.mavlink.enums.get(enum_group,{})
        entry = enum.get(value)

        if entry and hasattr(entry, "name"):
            return entry.name.replace(f"{enum_group}_", "")
        return "UNKNOWN"



    def map_command_ack(self, message):
        system_id = message.get_srcSystem()
        
        command = self.enum_name("MAV_CMD", message.command)
        result = self.enum_name("MAV_RESULT", message.result)

        command_ack_update = CommandAckUpdate(
            system_id = system_id,
            command = command,
            result = result
        )
        return command_ack_update


    def map_battery_status(self, message):
        system_id = message.get_srcSystem()  
        component_id = message.get_srcComponent()
        battery_id = message.id
        
        battery_function = self.enum_name(
            "MAV_BATTERY_FUNCTION", message.battery_function
        )

        battery_type = self.enum_name(
            "MAV_BATTERY_TYPE", message.type
        )

        charge_state = self.enum_name(
            "MAV_BATTERY_CHARGE_STATE", message.charge_state
        )

        temperature = None if message.temperature == 32767 else message.temperature / 100.0  # cdegC -> Celsius
        current_battery = None if message.current_battery == -1 else message.current_battery / 100.0  # cA -> Amps
        
        current_consumed = None if message.current_consumed == -1 else message.current_consumed  # mAh
        energy_consumed = None if message.energy_consumed == -1 else message.energy_consumed  # hJ
        time_remaining = None if message.time_remaining <= 0 else message.time_remaining  # Seconds
        battery_remaining = None if message.battery_remaining == -1 else message.battery_remaining  # %

        raw_cells = list(message.voltages)
        if hasattr(message, 'voltages_ext'):
            raw_cells.extend(message.voltages_ext)
            
        cell_voltages = [
            round(cell / 1000.0, 2) if (cell!=65535 and cell!=0) else None
            for cell in raw_cells
        ]
        
        # TODO(learn): `cell_voltages_mv` is undefined (disabled code, so it never runs yet).
        #   Decide what total_voltage should be computed from, and in what units, before enabling BATTERY_STATUS.
        total_voltage = sum(cell_voltages) / 1000.0 if cell_voltages_mv else (message.voltages[0] / 1000.0 if message.voltages[0] != 65535 else None)

        battery_update = BatteryStatusUpdate(
            system_id=system_id,
            batteries = BatteryState(
                component_id=component_id,
                battery_id=battery_id,
                battery_function=battery_function,
                battery_type=battery_type,
                charge_state=charge_state,
                temperature=temperature,
                total_voltage=total_voltage,
                current_battery=current_battery,
                battery_remaining=battery_remaining,
                current_consumed=current_consumed,
                energy_consumed=energy_consumed,
                time_remaining=time_remaining,
                cell_voltages=cell_voltages,
                fault_bitmask=getattr(message, 'fault_bitmask', 0))
        )
        return battery_update


    def map_heartbeat(self, message):
        system_id = message.get_srcSystem()
        component_id = message.get_srcComponent()

        type_enum_name = mavutil.mavlink.enums['MAV_TYPE'][message.type].name
        vehicle_type = type_enum_name.replace('MAV_TYPE_', '')

        auto_enum_name = mavutil.mavlink.enums['MAV_AUTOPILOT'][message.autopilot].name
        autopilot = auto_enum_name.replace('MAV_AUTOPILOT_', '')

        is_armed = bool(message.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED)


        flight_mode = mavutil.mode_string_v10(message)
        system_status = mavutil.mavlink.enums['MAV_STATE'][message.system_status].name.replace('MAV_STATE_', '')

        heartbeat_update = HeartbeatUpdate(
            system_id=system_id,
            component_id=component_id,
            vehicle_type=vehicle_type,
            autopilot=autopilot,
            is_armed=is_armed,
            flight_mode=flight_mode,
            system_status=system_status
        )
        return heartbeat_update

    def map_global_position_int(self, message):
        system_id = message.get_srcSystem()
        time_boot_ms = message.time_boot_ms
        latitude = message.lat / 1e7  
        longitude = message.lon / 1e7 
        altitude = message.alt / 1e3  
        relative_altitude = message.relative_alt / 1e3  
        velocity_x = message.vx / 1e2
        velocity_y = message.vy / 1e2  
        velocity_z = message.vz / 1e2 
        heading = message.hdg
        if heading == 65535:
            heading = None
        else:
            heading = message.hdg / 1e2
        

        global_position_update = GlobalPositionIntUpdate(
            system_id=system_id,
            time_boot_ms=time_boot_ms,
            position=Position(latitude=latitude, longitude=longitude, altitude=altitude, relative_altitude=relative_altitude),
            velocity=Velocity(velocity_x=velocity_x, velocity_y=velocity_y, velocity_z=velocity_z),
            heading=heading
        )
        return global_position_update