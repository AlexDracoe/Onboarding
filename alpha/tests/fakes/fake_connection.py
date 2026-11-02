

class FakeConnection():
    def __init__(self):
        self.connected = True  # Simulate a successful connection
    def connect(self, connection_string):
        self.connection = True
    def wait_for_heartbeat(self):
        # Simulate waiting for a heartbeat
        print("Simulated heartbeat received.")
    def receive(self):
        # Simulate receiving a message
        return "Fake message"
    def send(self, message):
        print(f"Simulated send: {message}")
    def disconnect(self):
        self.connected = False
        print("Simulated disconnection.")
    def mode_mapping(self):
            return {"STABILIZE": 0, "GUIDED": 4, "RTL": 6}