import logging
import sys
import threading
import time

from application.ack_tracker import AckTracker, AckTimeout, CommandAlreadyPending, CommandRejected
from application.command_handler import CommandHandler
from application.state_manager import StateManager
from config import CONFIG
from infra.mavlink.connection import PymavlinkConnection
from infra.mavlink.mapper import Mapper
from infra.mavlink.receiver import Receiver
from infra.db.sqlite_command_log import SqliteCommandLog
from application.vehicle_service import VehicleService, CommandRequest
from logging_config import setup_logging
# from tests.fakes.fake_connection import FakeConnection  # swap in for offline runs

logger = logging.getLogger(__name__)


def wait_for_vehicle(state_manager: StateManager, system_id: int, timeout: float) -> bool:
    """Startup only: block until the Receiver has delivered a first update for system_id."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if state_manager.get(system_id) is not None:
            return True
        time.sleep(CONFIG.state_poll_interval_s)
    return False


def main():
    setup_logging()

    connection = PymavlinkConnection()
    #connection = FakeConnection()
    connection.connect(CONFIG.connection_string)
    connection.wait_for_heartbeat()
    command_log = SqliteCommandLog(CONFIG.db_path)
    mapper = Mapper()
    state_manager = StateManager()
    ack_tracker = AckTracker()
    command_handler = CommandHandler(connection, ack_tracker)

    receiver = Receiver(connection, mapper, state_manager, ack_tracker)
    receiver_thread = threading.Thread(
        target=receiver.start,
        name="receiver",
        daemon=True
    )
    receiver_thread.start()

    try:
        vehicle_service = VehicleService(state_manager, command_handler, ack_tracker, connection, command_log)
        cmd_req_from_api = CommandRequest("arm",1) 
        if cmd_req_from_api is not None:
            cmd_result = vehicle_service.execute(cmd_req_from_api)
        
        print(cmd_result)
        receiver_thread.join()
    except KeyboardInterrupt:
        logger.info("Interrupted, shutting down")
    finally:
        receiver.stop()
        connection.disconnect()
        command_log.close()
    return 0

    
if __name__ == "__main__":
    sys.exit(main())
