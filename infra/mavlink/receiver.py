from domain.interfaces import Connection
from domain.updates import CommandAckUpdate

class Receiver():
    def __init__(self, connection: Connection, mapper, state_manager, ack_tracker):
        self.connection = connection
        self.mapper = mapper
        self.state_manager = state_manager
        self.ack_tracker =  ack_tracker

    def start(self):
        while True:
            message = self.connection.receive()#if update go to update? else acktracker?
            update = self.mapper.map(message)
            if isinstance(update, CommandAckUpdate):
                self.ack_tracker.handle_ack(update)
            elif update:
                self.state_manager.apply_update(update)
            else:
                print(f"Received unknown message type: {message.get_type()}")