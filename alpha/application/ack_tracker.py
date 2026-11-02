import logging
from dataclasses import dataclass, field
from threading import Event, Lock
from config import CONFIG
logger = logging.getLogger(__name__)

# DECISIONS SO FAR (yours, from the review rounds)
#  - One outstanding command per (system_id, command): dict value is a single PendingCommand, no list.
#  - Receiver thread calls handle_ack; the caller thread calls register / wait_for_ack.
#  - The Lock protects the dict (check-then-use), not `result`. Never hold it while blocking in Event.wait().
#  - handle_ack writes `result` BEFORE event.set().
#  - Look the entry up once into a local variable and use that (your Q3.1 answer: yes, and the same applies in register).
#  - `if pending.event.wait(timeout):` tells you ACK (True) vs timeout (False). No snapshot/timer tricks.
#  - Timeout = exception (no answer at all). A DENIED/FAILED ACK is an answer.
#  - wait_for_ack owns cleanup on every path.
#  - Known v1 limitation: a late ACK from a timed-out attempt could match a retry (no sequence number in COMMAND_ACK).


class AckTimeout(Exception):
    """No COMMAND_ACK arrived before the timeout."""


class CommandAlreadyPending(Exception):
    """A command with the same (system_id, command) key is already awaiting its ACK."""

SUCCESS_RESULTS = {"ACCEPTED", "IN_PROGRESS"}

RETRYABLE_RESULTS = {"TEMPORARILY_REJECTED"}


class CommandRejected(Exception):
    """The vehicle answered with a COMMAND_ACK that is not a success."""
    def __init__(self, system_id, command, result, retryable):
        self.system_id = system_id
        self.command =  command
        self.result = result
        self.is_retryable = retryable
        super().__init__(f"{self.command} rejected by system {self.system_id}: {self.result}")

@dataclass
class PendingCommand:
    system_id: int
    command: str
    event: Event = field(default_factory=Event)
    result: str | None = None

class AckTracker():
    def __init__(self):
        # Key: (system_id, command)
        # Value: PendingCommand
        self.pendingCommands = {}

        # Protects pending commands shared between threads.
        self.lock = Lock()

    def register(self, system_id: int, command: str) -> PendingCommand:
        """Register a command before it is sent."""
        key = (system_id, command)
        new_command = PendingCommand(system_id = system_id, command = command)

        with self.lock:
            if key in self.pendingCommands:
                raise CommandAlreadyPending(f"{command} already pending for system {system_id}")
            self.pendingCommands[key] = new_command

        return new_command

    def handle_ack(self, update):
        """Called by Receiver when a CommandAckUpdate arrives."""
        key = (update.system_id, update.command)
        
        with self.lock:
            if self.pendingCommands.get(key):
                logger.info("ACK %s", update)
                current_ack = self.pendingCommands[key]
                current_ack.result = update.result
                current_ack.event.set()
                
            else:
                logger.warning("Received unexpected ACK: %s", key)

    def wait_for_ack(self, pending: PendingCommand, timeout=CONFIG.ack_timeout_s):
        """Wait for an ACK or timeout. Raises command rejected and denotes if command is retryable. Returns a string on success and raises ACKTimeout"""
        try:
            event_result = pending.event.wait(timeout)
            if event_result:
                if pending.result in SUCCESS_RESULTS:
                    return pending.result
                elif pending.result in RETRYABLE_RESULTS:
                    raise CommandRejected(pending.system_id, pending.command, pending.result, True)
                else:
                    raise CommandRejected(pending.system_id, pending.command, pending.result, False)# we might want to create a different type of exception for unknown non retryable results
            else:
                raise AckTimeout(f"{pending.command} Ack timeout for system {pending.system_id}")
        finally:
            self.discard(pending)

    def discard(self, pending: PendingCommand):
        """Remove a pending command"""
        key =  (pending.system_id,pending.command)
        with self.lock:
            if self.pendingCommands.get(key) is pending:
                del self.pendingCommands[key]
