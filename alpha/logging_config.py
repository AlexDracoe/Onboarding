import logging

LOG_FORMAT = "%(asctime)s %(levelname)s [%(threadName)s] %(name)s: %(message)s"


class _QuietPolling(logging.Filter):
    """Drop uvicorn's access line for successful GETs. The dashboard polls every 300 ms to 1 s,
    which buries the command/ACK logs. POSTs and any 4xx/5xx still show."""

    def filter(self, record: logging.LogRecord) -> bool:
        args = record.args
        is_ok_get = isinstance(args, tuple) and len(args) >= 5 and args[1] == "GET" and args[4] < 400
        return not is_ok_get


def setup_logging(level: int = logging.INFO) -> None:
    logging.basicConfig(level=level, format=LOG_FORMAT)
    logging.getLogger("uvicorn.access").addFilter(_QuietPolling())
