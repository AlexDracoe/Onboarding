from infra.mavlink.connection import PymavlinkConnection
from infra.mavlink.mapper import Mapper
from infra.mavlink.receiver import Receiver
# from tests.fakes.fake_connection import FakeConnection  # swap in for offline runs
from application.state_manager import StateManager
from domain.models import VehicleState
from application.command_handler import CommandHandler
from application.ack_tracker import AckTracker

import threading 
import time

connection =  PymavlinkConnection()
#connection = FakeConnection()
connection.connect('udpin:127.0.0.1:14540')
connection.wait_for_heartbeat()

mapper = Mapper()

state_manager = StateManager()
ack_tracker = AckTracker()

receiver = Receiver(connection, mapper, state_manager, ack_tracker)

receiver_thread = threading.Thread(
    target=receiver.start,
    daemon =True
)
receiver_thread.start()

command_handler = CommandHandler(connection, ack_tracker)
time.sleep(5)

command = command_handler.arm(system_id=1)
connection.send(command)
receiver_thread.join()
