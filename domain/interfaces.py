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
    def mode_mapping(self) -> dict[str, int] | None:
        """Protocol requirement: Anything acting as a Connection must have a mode_mapping method."""
        ...


class CommandLog(Protocol):
    """Where command records are persisted. The service depends on this, not on SQLite."""
    def create(self, system_id: int, cmd: dict, status: str) -> str:
        """Insert a new command record and return its id."""
        ...
    def update_status(self, command_id: str, status: str, error: str | None = None) -> None:
        """Set a new status (and optional error message) on an existing record."""
        ...
    def get(self, command_id: str) -> dict | None:
        ...
    def list_recent(self, limit: int = 50, system_id: int | None = None) -> list[dict]:
        ...
    
        