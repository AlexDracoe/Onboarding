import logging
import threading
import time

from config import CONFIG
from domain.interfaces import Connection
from domain.updates import CommandAckUpdate

logger = logging.getLogger(__name__)


class Receiver():
    """Takes the single connection and receives messages, if it is an ACK, sends to ack_tracker, telem goes to the state manager"""
    def __init__(self, connection: Connection, mapper, state_manager, ack_tracker):
        self.connection = connection
        self.mapper = mapper
        self.state_manager = state_manager
        self.ack_tracker =  ack_tracker
        self.stop_event = threading.Event()

    def stop(self):
        """Ask the loop to exit. It is only checked between messages, so a blocked receive() delays it."""
        self.stop_event.set()

    def start(self):
        while not self.stop_event.is_set():
            # A failing receive() (closed socket etc.) would spin forever, so back off.
            try:
                message = self.connection.receive()#if update go to update? else acktracker?
            except Exception:
                logger.exception("receive() failed")
                time.sleep(CONFIG.receive_backoff_s)
                continue

            if message is None:
                continue

            # A bad message must not kill the only consumer thread: log and move on.
            try:
                update = self.mapper.map(message)
                if isinstance(update, CommandAckUpdate):
                    self.ack_tracker.handle_ack(update)
                elif update:
                    self.state_manager.apply_update(update)
                else:
                    logger.debug("Unmapped message type: %s", message.get_type())
            except Exception:
                logger.exception("Failed to process message %r", message)
        logger.info("Receiver stopped")
