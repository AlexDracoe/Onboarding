import json
import logging
import sqlite3
import threading
import time
import uuid

logger = logging.getLogger(__name__)


SCHEMA = """
CREATE TABLE IF NOT EXISTS commands (
    id         TEXT PRIMARY KEY,      -- uuid4 string
    created_at REAL NOT NULL,         -- unix seconds (wall clock), when registered
    updated_at REAL NOT NULL,         -- unix seconds, last status change
    system_id  INTEGER NOT NULL,
    cmd        TEXT NOT NULL,         -- JSON object: command name + params
    status     TEXT NOT NULL,         -- PENDING / ACKED / ACHIEVED / REJECTED / ...
    error      TEXT                   -- NULL unless the command failed
);
CREATE INDEX IF NOT EXISTS idx_commands_system_created
    ON commands (system_id, created_at DESC);
"""


class SqliteCommandLog:
    """Owns the one SQLite connection. Nothing else in the project talks to the DB.

    Callers come from several threads (API worker threads, the main thread), so the single
    connection is shared with check_same_thread=False and every call is serialised by a Lock.
    Returns plain dicts, so the API layer can build its schema from them.
    """

    def __init__(self, path: str):
        self._lock = threading.Lock()
        self._conn = sqlite3.connect(path, check_same_thread=False)
        self._conn.row_factory = sqlite3.Row  # rows behave like dicts: row["status"]
        with self._lock:
            self._conn.execute("PRAGMA journal_mode=WAL")  # readers don't block the writer
            self._conn.executescript(SCHEMA)

    # SAMPLE (finished): copy its shape for the others.
    def create(self, system_id: int, cmd: dict, status: str) -> str:
        """Insert a new command row and return its id."""
        command_id = str(uuid.uuid4())
        now = time.time()
        
        with self._lock, self._conn:
            self._conn.execute(
                "INSERT INTO commands (id, created_at, updated_at, system_id, cmd, status) "
                "VALUES (?, ?, ?, ?, ?, ?)",   # ? placeholders, never f-strings (SQL injection)
                (command_id, now, now, system_id, json.dumps(cmd), status),
            )
        return command_id

    def update_status(self, command_id: str, status: str, error: str | None = None) -> None:
        """Set a new status (and optionally an error). Raises KeyError for an unknown id."""
        now = time.time()

        with self._lock, self._conn:
            result = self._conn.execute(
                "UPDATE commands SET status = ?, error = ?, updated_at = ? WHERE id = ?",
                (status, error, now, command_id),
            )
            if result.rowcount == 0:
                raise KeyError(command_id)
            
    

    def get(self, command_id: str) -> dict | None:
        """One command as a dict, or None if the id is unknown."""

        with self._lock:
            row = self._conn.execute(
                "SELECT * FROM commands WHERE id = ?",
                (command_id,)
            ).fetchone()

            if row is not None:
                return self._row_to_dict(row)
            else: 
                return None


    def list_recent(self, limit: int = 50, system_id: int | None = None) -> list[dict]:
        """Newest first, optionally for one vehicle."""

        query = "SELECT * FROM commands"
        params = []
        if system_id is not None:
            query+=" WHERE system_id = ?"
            params.append(system_id)
        query += " ORDER BY created_at DESC LIMIT ?"
        params.append(limit)
        
        with self._lock:
            rows =  self._conn.execute(query, params).fetchall()
        return [self._row_to_dict(row) for row in rows]
            

        

    def close(self) -> None:
        with self._lock:
            self._conn.close()

    @staticmethod
    def _row_to_dict(row: sqlite3.Row) -> dict:
        new_dict = dict(row)
        new_dict['cmd'] = json.loads(new_dict['cmd'])

        return new_dict
