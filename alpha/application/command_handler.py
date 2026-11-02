from pymavlink import mavutil

class UnknownMode(Exception):
    """The mode name is not valid for the connected vehicle (or no heartbeat has arrived yet)."""
    def __init__(self, mode):
        self.mode = mode
        super().__init__(f"Unknown mode: {mode}")

class CommandHandler():
    ARM_NAME = "COMPONENT_ARM_DISARM"
    SET_MODE_NAME = "DO_SET_MODE" 
    TAKEOFF_NAME = "NAV_TAKEOFF"
    GOTO_NAME = "DO_REPOSITION"

    def __init__ (self, connection, ack_tracker):
       self.connection = connection
       self.ack_tracker = ack_tracker
    
    def arm(self, system_id):
        """Returns a MAVLINK CMD LONG OBJ"""
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            target_component=1,
            command=mavutil.mavlink.MAV_CMD_COMPONENT_ARM_DISARM,
            confirmation=0,
            param1=1, param2=0, param3=0, param4=0, param5=0, param6=0, param7=0
        )
        

    def disarm(self, system_id):
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            target_component=mavutil.mavlink.MAV_COMP_ID_AUTOPILOT1,
            command=mavutil.mavlink.MAV_CMD_COMPONENT_ARM_DISARM,
            confirmation=0,
            param1=0, param2=0, param3=0, param4=0, param5=0, param6=0, param7=0
        )
    
    def set_mode(self, system_id, mode): 
        modes = self.connection.mode_mapping()
        if modes is None or mode not in modes:
            raise UnknownMode(mode)
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system = system_id,
            target_component=mavutil.mavlink.MAV_COMP_ID_AUTOPILOT1,
            command=mavutil.mavlink.MAV_CMD_DO_SET_MODE,
            confirmation=0,
            param1=mavutil.mavlink.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED,
            param2=modes[mode],
            param3=0, param4=0, param5=0, param6=0, param7=0
        )
    
    def takeoff(self, system_id, altitude):
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            target_component = mavutil.mavlink.MAV_COMP_ID_AUTOPILOT1,
            command = mavutil.mavlink.MAV_CMD_NAV_TAKEOFF,
            confirmation=0,
            param1=0, param2=0, param3=0, param4=float('nan'), param5=0, param6=0, param7=altitude
        )

    
    def goto(self, system_id, lat, long, altitude):
        """Generates a command to fly immediately to a specific GPS coordinate.
        
        Requires the vehicle to be in GUIDED or equivalent autonomous mode.
        """
        return mavutil.mavlink.MAVLink_command_int_message(
            target_system=system_id,
            target_component=mavutil.mavlink.MAV_COMP_ID_AUTOPILOT1,
            frame=mavutil.mavlink.MAV_FRAME_GLOBAL_RELATIVE_ALT_INT,
            command=mavutil.mavlink.MAV_CMD_DO_REPOSITION,
            autocontinue=0,
            current=0,
            param1=-1,    
            param2=0,     
            param3=0,     
            param4=float('nan'), 
            x=int(lat* 1e7),   
            y=int(long* 1e7),  
            z=altitude 
        )

