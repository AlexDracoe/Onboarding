"""App factory + lifespan: the one place that creates, wires and tears down the long-lived objects.

Run with:  .venv/bin/uvicorn api.app:app

Startup order: connection -> command log -> state/ack/mapper -> Receiver thread -> service.
Shutdown runs the same steps in reverse (ExitStack), so a failure half-way through startup
still cleans up whatever was already created.

The API starts even if no vehicle is talking: connect() on a udpin socket only binds, and the
Receiver fills StateManager as heartbeats arrive. Until then GET /vehicles/{id} answers 404 NO_VEHICLE.
"""
import logging
import threading
from contextlib import ExitStack, asynccontextmanager

from fastapi import FastAPI

from api.routes import router
from application.ack_tracker import AckTracker
from application.command_handler import CommandHandler
from application.state_manager import StateManager
from application.vehicle_service import VehicleService
from config import CONFIG
from infra.db.sqlite_command_log import SqliteCommandLog
from infra.mavlink.connection import PymavlinkConnection
from infra.mavlink.mapper import Mapper
from infra.mavlink.receiver import Receiver
from logging_config import setup_logging

logger = logging.getLogger(__name__)

RECEIVER_JOIN_TIMEOUT_S = CONFIG.receive_timeout_s + 2.0


def create_app(connection_factory=PymavlinkConnection) -> FastAPI:
    """connection_factory exists so a scratch run can swap in a fake connection."""

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        setup_logging()
        with ExitStack() as stack:
            connection = connection_factory()
            connection.connect(CONFIG.connection_string)
            stack.callback(connection.disconnect)           

            command_log = SqliteCommandLog(CONFIG.db_path)
            stack.callback(command_log.close)                

            state_manager = StateManager()
            ack_tracker = AckTracker()
            receiver = Receiver(connection, Mapper(), state_manager, ack_tracker)
            receiver_thread = threading.Thread(target=receiver.start, name="receiver", daemon=True)
            receiver_thread.start()

            def stop_receiver():
                
                receiver.stop()
                receiver_thread.join(timeout=RECEIVER_JOIN_TIMEOUT_S)
                if receiver_thread.is_alive():
                    logger.warning("receiver thread did not stop within %.1fs", RECEIVER_JOIN_TIMEOUT_S)
            stack.callback(stop_receiver)                    # runs first

            service = VehicleService(
                state_manager, CommandHandler(connection, ack_tracker), ack_tracker, connection, command_log
            )
            app.state.state_manager = state_manager
            app.state.command_log = command_log
            app.state.service = service
            logger.info("Service ready, listening to %s", CONFIG.connection_string)
            yield                                         
        logger.info("Service stopped")

    app = FastAPI(title="Drone service", lifespan=lifespan)
    app.include_router(router)
    return app


app = create_app()
