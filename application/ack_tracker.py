
from dataclasses import dataclass, field
from threading import Event, Lock
import time

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
        self.pending = {}

        # Protects pending commands shared between threads.
        self.lock = Lock()

    def register(self, system_id: int, command: str) -> PendingCommand:
        """Register a command before it is sent."""
        key = (system_id, command)
        new_command = PendingCommand(system_id = system_id, command = command)

        with self.lock:
            # TODO(learn): is a list per key the right shape? (v1 design: one outstanding command per vehicle)
            # Well from what i was understanding... we are taking the sysid and command, using that as a tuple for the key, the keys value is the commmand
            #and we put the keyvalue pair in the list of pending commands
            # COACH Q1: In your code the value is a *list* of PendingCommands (line below), not a single command.
            #   Then look at handle_ack: `self.pending[key].result = ...`. What type is self.pending[key] there,
            #   and what happens when you ask a list for `.result`?
            # YOU Q1: i see what you mean now, so we have a list and i appended the new command, so i think that it is better to have a list and we can search for maybe? but
            #also if we just have (id, cmd) = cmd its easier, and then at the handle ack yeah .result returns the list of commands
            # COACH Q2: v1 rule = one outstanding command per vehicle. If a second ARM for the same
            #   (system_id, command) is registered while the first is pending, what should happen?
            #   Given your answer, do you still need a list?
            # YOU Q2:we probably just want to overwrite the existing one, this would also reset the timeout? or cause a new one to be created? but how would that do the with mutex, also ig yeah we dont need the list per pending[key]
            #
            # COACH Q1 reply: You're landing in the right place: for v1 the list buys you nothing.
            #   Small correction: asking a list for `.result` doesn't "return the list", it raises AttributeError
            #   (a list has no such attribute). That would be raised on the Receiver thread and kill it.
            # COACH Q2 reply: Good instinct that "one per key" means no list. But careful with overwrite:
            #   the first caller is already blocked on the FIRST PendingCommand's Event. If you replace the dict
            #   entry, that Event is orphaned: nobody can ever set() it, so caller #1 sits until its own timeout
            #   even though an ACK arrived. (Timeouts aren't reset: the timeout belongs to each caller's
            #   wait_for_ack call, not to the entry.) So overwrite silently strands someone.
            #   On the mutex: your `with self.lock` here already makes "check if exists, then insert" one
            #   atomic step, which is exactly what you need.
            # COACH Q2.1: Two options: (a) raise an error if the key is already pending, (b) overwrite.
            #   Which fits "at most one outstanding command per vehicle" better, and what would the API
            #   caller see in each case? Pick one and implement it inside the lock.
            # YOU Q2.1:i think overwrite and make sure we set() the event so it doesnt become orphaned or just update the commandfield and thats the end of that
            #
            if key not in self.pending:
                self.pending[key] = new_command
            else:# did have that cmd, and sysid so check if we timedout and if so then lets set(), to finish that branch and then ig
                if self.pending[key].event == False: 
                    self.pending[key].event = Event()#timeedout???
                    self.pending[key].command = new_command
            

        return new_command

    def handle_ack(self, update):
        """Called by Receiver when a CommandAckUpdate arrives."""
        # TODO:
        # 1. Build the matching key from the update.
        key = (update.system_id, update.command)
        with self.lock:
            if key in self.pending:
                self.pending[key].result = update.result
                self.pending[key].event.set()
            else:
                print("Received Ack for command not registered")

        # TODO(learn): a Lock has no .set(); which object should be signalled, and should it happen inside or outside the lock?
        # I think we want to signal the event itself right, and it should be inside the lock
        # COACH Q3: Yes, the Event is right. Event.set() is already thread-safe, so what is the lock
        #   protecting here? Imagine you looked up the entry under the lock but called set() after
        #   releasing it. Can you describe a bad interleaving, or is it fine? Either answer is OK if you can say why.
        # YOU Q3:i am honestly at a loss right now, i think the lock is protecting... someoneelse updating result of the command? 
        #
        # COACH Q3 reply (more detailed, since you said you're at a loss):
        #   The lock does NOT protect `result`. It protects the *dict* (self.pending), which two threads touch:
        #     - Receiver thread: handle_ack reads it
        #     - caller thread: register adds to it, wait_for_ack removes from it
        #   The real danger is "check then use". Line 54 `if key in self.pending` and line 55 `self.pending[key]`
        #   are two separate steps. Without the lock, this can happen:
        #       Receiver: `key in self.pending` -> True
        #       Caller:   timeout fires, deletes the key
        #       Receiver: `self.pending[key]` -> KeyError, Receiver thread dies, telemetry stops
        #   Holding the lock makes "is it there + grab it" atomic w.r.t. removal. That's the whole job.
        #   Setting the Event after releasing the lock would be safe as long as you already hold a reference
        #   to the PendingCommand object (the Event itself is thread-safe). Setting it inside is simpler to reason about.
        #   One ordering rule that DOES matter: write `result` BEFORE `event.set()`. The waiter wakes the instant
        #   set() runs and will read result; if it was written after, the waiter could see None.
        #   In Go: a mutex around the map, and `ch <- result` (or close(ch)) after storing.
        # COACH Q3.1: Your code at lines 55-56 still treats the value as one PendingCommand. After you change
        #   register's shape, update these lines. Can you avoid looking the key up twice?
        # YOU Q3.1:
        # we could set a variable to self.pending[key] and use that? ig the same for register right
        # COACH Q4: An ACK for something nobody registered just prints and returns. Good. But also think:
        #   if the ACK arrives *after* wait_for_ack timed out and cleaned up, what does this branch do,
        #   and is that what you want?
        # YOU Q4:maybe we can still proccess the awk that was timeedout? we can probably display late awk? but i dont think mavlink would care right? doesnt a message typically have a TTL?


        # COACH Q4 reply: MAVLink itself has no TTL; the vehicle doesn't know you gave up and the ACK arrives
        #   over UDP whenever it arrives. So a late ACK finds no entry and hits your else branch: log and
        #   drop. That's the right v1 behavior; just keep it from crashing.
        #   Known caveat to keep in mind (don't solve it now): if the user retries ARM after a timeout, a late ACK
        #   from the first attempt could be taken as the answer to the second. COMMAND_ACK has no sequence number,
        #   so (system_id, command) is as precise as v1 gets.

    def wait_for_ack(self, pending: PendingCommand, timeout: float):
        """Wait for an ACK or timeout."""
        # TODO:
        # 1. Wait on pending.event with the timeout.
        # 2. Distinguish timeout from a received result.
        # 3. Return or raise an appropriate outcome.
        # 4. Remove the pending entry safely.
        # TODO(learn): implement wait_for_ack.
        #so i think we want to first set the timmer, then get the command, set its event to locked , so .wait()
        currentResult = pending.result
        pending.event.wait(timeout)
        if currentResult != pending.result:
            print("a change")
        else:
            print("no change(hard to know if timeout or update had no change after an ack?)")
        print("timeout")
        #ig another way to know if we timed out was using a timer and seeing if the time we waited was less than the timeout time?
        # COACH Q5: Events aren't "locked"; wait() just blocks until set() or the timeout.
        #   You noticed it's hard to tell timeout from ACK by comparing results. Python already answers this:
        #   read the docs for what Event.wait(timeout) *returns*. What does it return in each case?
        # YOU Q5: when the timeout expires it returns false, if the event is set or was already set then it returns true, so i guess i could do "if pending.event.wait(timeout)" and then i know
        #and i change the branching 
        # COACH Q5 reply: Exactly right. `if pending.event.wait(timeout):` replaces the whole snapshot-compare
        #   idea (and the timer idea), because wait() tells you directly whether set() happened.
        # COACH Q6: Line `print("timeout")` runs on every path, and `raise NotImplementedError` is still there.
        #   What should the caller get back on success, and on timeout? Is a timeout a return value
        #   or an exception? (Think about what the caller in main.py wants to do next.)
        # YOU Q6:a time out returned by Event is a value, but maybe for our programing we can then turn it into an exception?, the caller probably wants to know whats going on, so that we can update the api and the website
        #
        # COACH Q6 reply: Turning the timeout into an exception is a good call: "no answer at all" is different
        #   from "an answer". The caller can tell them apart and the API can map them to different responses.
        # COACH Q6.1: What about an ACK whose result is DENIED / FAILED? That's an *answer*, not a missing one.
        #   Should wait_for_ack return that result and let the caller decide, or also raise? Which is easier for
        #   the caller to handle (and for later IN_PROGRESS handling)? What would you return on success?
        # YOU Q6.1:
        #
        # COACH Q7: Nothing is removed from self.pending yet (your TODO step 4). Which path(s) need cleanup:
        #   ACK, timeout, both? What exactly gets removed, and does that need the lock?
        # YOU Q7:the key(id, cmd) along with its value get removed
        #
        # COACH Q7 reply: Right object, and cleanup is needed on BOTH paths (ACK and timeout). And yes, it needs
        #   the lock, because you're mutating the dict the Receiver reads.
        #   Two refinements to think about:
        #   1. Who owns cleanup: handle_ack, wait_for_ack, or both? Pick ONE owner. Hint: only wait_for_ack
        #      runs on every path, including timeout. Is there a Python construct that guarantees code runs
        #      whether you return normally or raise?
        #   2. On timeout, only remove the entry if it is still *your* PendingCommand (compare with `is`), not just
        #      "whatever is under that key now". Why could the entry under the key be someone else's by then?
        # YOU Q7.1:
        #
        # COACH NEXT (after the above): wire register -> send -> wait_for_ack in main.py,
        #   with cleanup even if connection.send() raises.
