# Build context is the PARENT of swarmops-backend (swarm-ops/):
#   docker build -f swarmops-backend/planner/Dockerfile ..
FROM python:3.12-slim

COPY --from=ghcr.io/astral-sh/uv:0.7.13 /uv /bin/uv
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy PYTHONUNBUFFERED=1

COPY swarmops-contracts/gen/python /src/swarmops-contracts/gen/python
COPY swarmops-backend/planner /src/swarmops-backend/planner
WORKDIR /src/swarmops-backend/planner
RUN uv sync --no-dev

EXPOSE 9090
CMD ["/src/swarmops-backend/planner/.venv/bin/planner"]
