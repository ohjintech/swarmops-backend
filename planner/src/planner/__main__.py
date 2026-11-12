import logging
import os

from .server import serve


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    serve(int(os.environ.get("PLANNER_PORT", "9090")))


if __name__ == "__main__":
    main()
