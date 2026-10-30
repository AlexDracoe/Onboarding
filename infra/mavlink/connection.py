import logging

from pymavlink import mavutil

from config import CONFIG

logger = logging.getLogger(__name__)



#connection_string = mavutil.mavlink_connection('udpin:127.0.0.1:14540')
class PymavlinkConnection():
    def __init__(self):
        self.connection = None
    
    def connect(self, connection_string):
        self.connection = mavutil.mavlink_connection(connection_string)
        
        
    def wait_for_heartbeat(self):
        if self.connection:
            self.connection.wait_heartbeat()
            logger.info("Heartbeat from system %u component %u", self.connection.target_system, self.connection.target_component)
        else:
            raise Exception("Connection not established. Call connect() first.")

    def disconnect(self):
        if self.connection:
            self.connection.close()
            self.connection = None
    
    def receive(self):
        if self.connection:
            # Returns None after the timeout, so the Receiver loop can notice stop_event.
            message = self.connection.recv_match(blocking=True, timeout=CONFIG.receive_timeout_s)
            return message
        else:
            raise Exception("Connection not established. Call connect() first.")
   
    def send(self, command):
        if self.connection:
            self.connection.mav.send(command)
            logger.info("Sent command: %s", command)

    def mode_mapping(self):
        """Mode name to number"""

        if self.connection:
            return self.connection.mode_mapping()
        raise Exception("Connection not established. Call connect() first")

