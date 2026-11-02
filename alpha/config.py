from dataclasses import dataclass
import os


@dataclass(frozen=True)
class Config:
    ack_timeout_s: float = 10.0
    receive_backoff_s: float = 0.5
    receive_timeout_s: float = 1.0
    state_poll_interval_s: float = 0.1
    startup_timeout_s: float = 10.0
    connection_string: str = os.environ.get("MAVLINK_CONNECTION", "udpin:127.0.0.1:14540")
    db_path: str = os.environ.get("DB_PATH", "drone.db")


CONFIG = Config()
