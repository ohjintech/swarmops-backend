package telemetry

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
	realtimev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/realtime/v1"
)

func TestUpdateIgnoresOutOfOrderTelemetry(t *testing.T) {
	h := NewHub(nil)
	now := time.Now()
	newer := &dronev1.Telemetry{DroneId: "d", Ts: timestamppb.New(now), Seq: 2}
	older := &dronev1.Telemetry{DroneId: "d", Ts: timestamppb.New(now.Add(-time.Second)), Seq: 1}
	if !h.Update(newer) {
		t.Fatal("first update should apply")
	}
	if h.Update(older) {
		t.Fatal("older telemetry should be ignored")
	}
	if h.latest["d"].GetSeq() != 2 {
		t.Fatal("latest should still be seq 2")
	}
}

func TestInViewport(t *testing.T) {
	vp := &realtimev1.Viewport{MinLat: 37, MaxLat: 38, MinLon: -123, MaxLon: -122}
	if !inViewport(vp, &dronev1.Position{Lat: 37.5, Lon: -122.5}) {
		t.Error("inside point rejected")
	}
	if inViewport(vp, &dronev1.Position{Lat: 39, Lon: -122.5}) {
		t.Error("outside point accepted")
	}
	if !inViewport(nil, &dronev1.Position{Lat: 0, Lon: 0}) {
		t.Error("nil viewport should accept everything")
	}
}
