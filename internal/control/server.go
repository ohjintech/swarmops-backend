// Package control implements swarmops.control.v1.FleetService.
package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/swarm-ops/swarmops-backend/internal/store"
	"github.com/swarm-ops/swarmops-backend/internal/topics"
	controlv1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1"
	"github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1/controlv1connect"
	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
)

type Server struct {
	Store      *store.Store
	MQTT       *autopaho.ConnectionManager
	StaleAfter time.Duration
	AckTimeout time.Duration

	// Serialises read-modify-write of command records within this process.
	// Running several control-api replicas needs a Redis WATCH/Lua transition instead.
	mu sync.Mutex
}

var _ controlv1connect.FleetServiceHandler = (*Server)(nil)

func (s *Server) summary(t *dronev1.Telemetry) *controlv1.DroneSummary {
	stale := time.Since(t.GetTs().AsTime()) > s.StaleAfter
	return &controlv1.DroneSummary{Latest: t, Stale: stale}
}

func (s *Server) ListDrones(ctx context.Context, req *connect.Request[controlv1.ListDronesRequest]) (*connect.Response[controlv1.ListDronesResponse], error) {
	all, err := s.Store.ListTelemetry(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	res := &controlv1.ListDronesResponse{}
	for _, t := range all {
		if f := req.Msg.GetFleetId(); f != "" && t.GetFleetId() != f {
			continue
		}
		res.Drones = append(res.Drones, s.summary(t))
	}
	return connect.NewResponse(res), nil
}

func (s *Server) GetDrone(ctx context.Context, req *connect.Request[controlv1.GetDroneRequest]) (*connect.Response[controlv1.GetDroneResponse], error) {
	t, err := s.Store.GetTelemetry(ctx, req.Msg.GetDroneId())
	if err != nil {
		return nil, storeErr(err)
	}
	return connect.NewResponse(&controlv1.GetDroneResponse{Drone: s.summary(t)}), nil
}

func (s *Server) GetCommand(ctx context.Context, req *connect.Request[controlv1.GetCommandRequest]) (*connect.Response[controlv1.GetCommandResponse], error) {
	rec, err := s.Store.GetCommand(ctx, req.Msg.GetCommandId())
	if err != nil {
		return nil, storeErr(err)
	}
	return connect.NewResponse(&controlv1.GetCommandResponse{Command: rec}), nil
}

func (s *Server) SendCommand(ctx context.Context, req *connect.Request[controlv1.SendCommandRequest]) (*connect.Response[controlv1.SendCommandResponse], error) {
	m := req.Msg
	if m.GetDroneId() == "" || m.GetCommand() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("drone_id and command are required"))
	}
	if err := validateCommand(m.GetCommand()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	tel, err := s.Store.GetTelemetry(ctx, m.GetDroneId())
	if err != nil {
		return nil, storeErr(err)
	}
	if s.summary(tel).GetStale() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("drone link lost; no recent telemetry"))
	}
	// TODO(milestone 3): control leases, geofence checks, audit log.

	id := newCommandID()
	if key := m.GetIdempotencyKey(); key != "" {
		existing, claimed, err := s.Store.ClaimIdempotencyKey(ctx, key, id)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		if !claimed {
			rec, err := s.Store.GetCommand(ctx, existing)
			if err != nil {
				return nil, storeErr(err)
			}
			return connect.NewResponse(&controlv1.SendCommandResponse{Command: rec}), nil
		}
	}

	now := time.Now()
	cmd := proto.Clone(m.GetCommand()).(*dronev1.Command)
	cmd.CommandId = id
	cmd.DroneId = m.GetDroneId()
	cmd.IssuedAt = timestamppb.New(now)
	cmd.ExpiresAt = timestamppb.New(now.Add(s.AckTimeout))

	rec := &controlv1.CommandRecord{
		CommandId: id,
		DroneId:   cmd.GetDroneId(),
		Status:    stQueued,
		CreatedAt: timestamppb.New(now),
		UpdatedAt: timestamppb.New(now),
	}
	if err := s.Store.PutCommand(ctx, rec); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	payload, err := proto.Marshal(cmd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if _, err := s.MQTT.Publish(ctx, &paho.Publish{
		QoS:     1,
		Topic:   topics.Command(tel.GetFleetId(), cmd.GetDroneId()),
		Payload: payload,
	}); err != nil {
		s.transition(context.WithoutCancel(ctx), id, stFailed, "broker publish failed: "+err.Error())
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	if r := s.transition(ctx, id, stSent, ""); r != nil {
		rec = r
	}
	time.AfterFunc(s.AckTimeout, func() {
		s.transition(context.Background(), id, stTimedOut, "no acknowledgement from drone")
	})
	return connect.NewResponse(&controlv1.SendCommandResponse{Command: rec}), nil
}

// HandleAck is wired to the MQTT subscription on swarm/v1/+/+/ack.
func (s *Server) HandleAck(payload []byte) {
	ack := &dronev1.CommandAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		slog.Warn("bad ack payload", "err", err)
		return
	}
	next, ok := statusFromAck(ack.GetStatus())
	if !ok {
		return
	}
	s.transition(context.Background(), ack.GetCommandId(), next, ack.GetReason())
}

// transition applies a lifecycle change if it is allowed, returning the updated record.
func (s *Server) transition(ctx context.Context, id string, next status, reason string) *controlv1.CommandRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.Store.GetCommand(ctx, id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("load command", "id", id, "err", err)
		}
		return nil
	}
	if !canTransition(rec.GetStatus(), next) {
		return rec
	}
	rec.Status = next
	rec.Reason = reason
	rec.UpdatedAt = timestamppb.Now()
	if err := s.Store.PutCommand(ctx, rec); err != nil {
		slog.Error("save command", "id", id, "err", err)
	}
	slog.Info("command", "id", id, "drone", rec.GetDroneId(), "status", next.String(), "reason", reason)
	return rec
}

func storeErr(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeUnavailable, err)
}

func newCommandID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "cmd_" + hex.EncodeToString(b)
}
