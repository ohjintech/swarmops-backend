package telemetry

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/proto"

	"github.com/swarm-ops/swarmops-backend/internal/store"
	"github.com/swarm-ops/swarmops-backend/internal/topics"
	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
	realtimev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/realtime/v1"
)

// Ingest routes MQTT messages from drones into the hub and Redis.
//
// TODO(milestone 3): batch-write downsampled history to Timescale (pgx.CopyFrom)
// and evaluate alert rules (geofence breach, separation, stale heartbeat).
type Ingest struct {
	Hub   *Hub
	Store *store.Store
}

func (in *Ingest) HandleMessage(topic string, payload []byte) {
	_, droneID, kind, ok := topics.Parse(topic)
	if !ok {
		return
	}
	switch kind {
	case "telemetry":
		t := &dronev1.Telemetry{}
		if err := proto.Unmarshal(payload, t); err != nil || t.GetDroneId() != droneID {
			slog.Warn("dropping bad telemetry", "topic", topic, "err", err)
			return
		}
		if !in.Hub.Update(t) {
			return
		}
		if err := in.Store.PutTelemetry(context.Background(), t); err != nil {
			slog.Warn("redis write failed", "err", err)
		}
	case "ack":
		a := &dronev1.CommandAck{}
		if proto.Unmarshal(payload, a) == nil {
			in.Hub.Broadcast(&realtimev1.ServerFrame{Frame: &realtimev1.ServerFrame_Ack{Ack: a}})
		}
	case "event":
		e := &dronev1.DroneEvent{}
		if proto.Unmarshal(payload, e) == nil {
			slog.Info("drone event", "drone", e.GetDroneId(), "code", e.GetCode(), "msg", e.GetMessage())
			in.Hub.Broadcast(&realtimev1.ServerFrame{Frame: &realtimev1.ServerFrame_Event{Event: e}})
		}
	}
}
