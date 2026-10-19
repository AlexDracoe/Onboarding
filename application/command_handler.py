from pymavlink import mavutil


# TODO(learn): this layer builds pymavlink messages directly, so the application layer depends
# on pymavlink. Design question: how should commands cross the boundary (roadmap step 7)?
class CommandHandler():
    def __init__ (self, connection, ack_tracker):
        self.connection = connection
        self.ack_tracker = ack_tracker
  

    def arm(self, system_id):
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            target_component =1,
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
        mode_id=None
        custom_mode=None
        custom_sub_mode=None
        if mode not in connection.mode_mapping():
            print('Unknown mode : {}'.format(mode))
            print('Try:', list(master.mode_mapping().keys()))

        mode_id = self.connection.mode_mapping()[mode]
        #if custom sets param2
        #if customsub sets param3 but how does None map in its meaning to when we send cmdlong?
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            #target_component,component_id,
            command=mavutil.mavlink.MAV_CMD_DO_SET_MODE,
            confirmation=0,
            param1=mode_id, param2=0, param3=0, param4=0, param5=0, param6=0, param7=0
        )

    
    def takeoff(self, system_id, altitude):
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            target_component = mavutil.mavlink.MAV_COMP_ID_ALL,
            command = mavutil.mavlink.MAV_CMD_NAV_TAKEOFF,
            confirmation=0,
            param1=0, param2=0, param3=0, param4=0, param5=0, param6=0, param7=altitude
        )

    
    def goto(self, system_id, lat, long, altitude):
        """Generates a command to fly immediately to a specific GPS coordinate.
        
        Requires the vehicle to be in GUIDED or equivalent autonomous mode.
        """
        return mavutil.mavlink.MAVLink_command_long_message(
            target_system=system_id,
            target_component=mavutil.mavlink.MAV_COMP_ID_ALL,
            command=mavutil.mavlink.MAV_CMD_DO_REPOSITION,
            confirmation=0,
            param1=-1,    
            param2=1,     
            param3=0,     
            param4=float('nan'), 
            param5=lat,   
            param6=long,  
            param7=altitude 
        )

