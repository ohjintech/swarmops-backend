# swarmops-backend

The SwarmOps backend: one deployable stack built from one repo.

| Service | Lang | Port | Role |
|---|---|---|---|
| `control-api` | Go | 8080 | `FleetService` over Connect / gRPC / gRPC-Web. Validates commands, publishes them over MQTT, tracks their lifecycle (`QUEUED → SENT → ACKED → COMPLETED / REJECTED / FAILED / TIMED_OUT`), handles idempotency keys |
| `telemetry` | Go | 8081 | MQTT ingest → Redis (latest state). `/ws` gateway sends viewport-filtered `FleetSnapshot`s at 4 Hz, plus acks and events as they arrive |
| `planner` | Python | 9090 | gRPC `PlannerService`. Multi-drone lawnmower survey planning (shapely + Hungarian assignment) |
| `swarmctl` | Go | — | operator CLI |

Infrastructure: Mosquitto (MQTT), Redis, and Postgres 16 with TimescaleDB and PostGIS.

```
console ──Connect──▶ control-api ──MQTT cmd──▶ ┐
console ◀──WS────── telemetry ◀──MQTT telem── ├── mosquitto ◀──▶ fleet-sim / real drones
                     control-api ◀──MQTT ack── ┘
```

## First-time setup

You need Go 1.24+, [uv](https://docs.astral.sh/uv/), [Task](https://taskfile.dev) and Docker. Clone the sibling repos next to this one:

```
swarm-ops/
├── swarmops-contracts/
├── swarmops-backend/     ← you are here
├── swarmops-fleet-sim/
└── swarmops-console/
```

```bash
task tidy                 # writes go.sum (commit it)
task test
```

> `github.com/swarm-ops` is a placeholder module owner. Replace it with your GitHub user or org.

## Run everything locally

```bash
task up                   # broker, redis, db, control-api, telemetry, planner, fleet-sim (10 drones)
task ctl -- drones
task ctl -- takeoff d-001 25
task ctl -- goto d-001 37.8060 -122.4620
task ctl -- rtl d-001
```

Then start `swarmops-console` (`npm run dev`) and open http://localhost:5173.

To break things, use the chaos API on http://localhost:8090/docs (link loss, battery, faults, spawning drones).

## Deploy (single VPS)

1. Point `api.<domain>` at the VPS. Copy `.env.example` to `deploy/.env` and set `API_DOMAIN`, `ALLOWED_ORIGINS` and the passwords.
2. Run `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml up -d`
3. Optionally set `vars.DEPLOY_ENABLED=true` and the `VPS_*` secrets so CI redeploys on every push to `main`.

Caddy terminates TLS and routes `/ws` to telemetry and everything else to control-api. Nothing else is published.

## Roadmap

- **Milestone 3:** Postgres persistence (migrations are already in `migrations/`), geofence checks in `internal/control/validate.go`, control leases, audit log, JWT auth plus WebSocket tickets
- `PlanSurvey` exposed through control-api, with the routes dispatched as `FollowRoute` commands
- Alert rules in ingest: separation distance, geofence breach, stale heartbeat
