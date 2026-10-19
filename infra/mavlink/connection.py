from pymavlink import mavutil



#connection_string = mavutil.mavlink_connection('udpin:127.0.0.1:14540')
class PymavlinkConnection():
    def __init__(self):
        self.connection = None
    
    def connect(self, connection_string):
        self.connection = mavutil.mavlink_connection(connection_string)
        
        
    def wait_for_heartbeat(self):
        if self.connection:
            self.connection.wait_heartbeat()
            print("Heartbeat from system (system %u component %u)" % (self.connection.target_system, self.connection.target_component))
        else:
            raise Exception("Connection not established. Call connect() first.")

    def disconnect(self):
        if self.connection:
            self.connection.close()
            self.connection = None
    
    def receive(self):
        if self.connection:
            message = self.connection.recv_match(blocking=True)
            return message
        else:
            raise Exception("Connection not established. Call connect() first.")
   
    def send(self, command):
        if self.connection:
            self.connection.mav.send(command)
            print("sent command")
            print(command)

