from typing import Protocol, Any

class Connection(Protocol):
    def receive(self)->None:
        """Protocol requirement: Anything acting as a Connection must have a receive method."""
        ...
    def disconnect(self)->None:
        """Protocol requirement: Anything acting as a Connection must have a disconnect method."""
        ...
    def connect(self, connection_string: str)->None:
        """Protocol requirement: Anything acting as a Connection must have a connect method."""
        ...
    def send(self, message: Any)->None:
        """Protocol requirement: Anything acting as a Connection must have a send method."""
        ...
    def wait_for_heartbeat(self)->None:
        """Protocol requirement: Anything acting as a Connection must have a wait_for_heartbeat method."""
        ...
        