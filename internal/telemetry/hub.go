// Package telemetry ingests drone MQTT traffic and fans it out to WebSocket clients.
package telemetry

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
	realtimev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/realtime/v1"
)

// Hub holds the latest state of every drone and the set of connected clients.
// Clients receive a viewport-filtered FleetSnapshot every tick, plus acks and
// events as soon as they arrive.
type Hub struct {
	OriginPatterns []string

	mu      sync.RWMutex
	latest  map[string]*dronev1.Telemetry
	clients map[*client]struct{}
}

type client struct {
	send     chan []byte
	mu       sync.Mutex
	viewport *realtimev1.Viewport // nil = whole world
}

func NewHub(originPatterns []string) *Hub {
	return &Hub{
		OriginPatterns: originPatterns,
		latest:         map[string]*dronev1.Telemetry{},
		clients:        map[*client]struct{}{},
	}
}

// Update records telemetry, ignoring messages older than what we already have
// (link jitter reorders packets). Returns false if the message was stale.
func (h *Hub) Update(t *dronev1.Telemetry) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if prev, ok := h.latest[t.GetDroneId()]; ok && !t.GetTs().AsTime().After(prev.GetTs().AsTime()) {
		return false
	}
	h.latest[t.GetDroneId()] = t
	return true
}

// Broadcast sends a frame to every client immediately (acks, events).
func (h *Hub) Broadcast(f *realtimev1.ServerFrame) {
	b, err := proto.Marshal(f)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.offer(b)
	}
}

// Run emits snapshots at the given rate until ctx is done.
func (h *Hub) Run(ctx context.Context, every time.Duration) {
	tk := time.NewTicker(every)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			h.tick()
		}
	}
}

func (h *Hub) tick() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) == 0 {
		return
	}
	all := make([]*dronev1.Telemetry, 0, len(h.latest))
	for _, t := range h.latest {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].GetDroneId() < all[j].GetDroneId() })
	now := timestamppb.Now()
	for c := range h.clients {
		snap := &realtimev1.FleetSnapshot{Ts: now, TotalDrones: uint32(len(all))}
		vp := c.getViewport()
		for _, t := range all {
			if inViewport(vp, t.GetPosition()) {
				snap.Drones = append(snap.Drones, t)
			}
		}
		b, err := proto.Marshal(&realtimev1.ServerFrame{Frame: &realtimev1.ServerFrame_Snapshot{Snapshot: snap}})
		if err == nil {
			c.offer(b)
		}
	}
}

func inViewport(vp *realtimev1.Viewport, p *dronev1.Position) bool {
	if vp == nil {
		return true
	}
	return p.GetLat() >= vp.GetMinLat() && p.GetLat() <= vp.GetMaxLat() &&
		p.GetLon() >= vp.GetMinLon() && p.GetLon() <= vp.GetMaxLon()
}

// offer drops the frame if the client is too slow; the next snapshot supersedes it.
func (c *client) offer(b []byte) {
	select {
	case c.send <- b:
	default:
	}
}

func (c *client) getViewport() *realtimev1.Viewport {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.viewport
}

func (c *client) setViewport(vp *realtimev1.Viewport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.viewport = vp
}

// ServeWS upgrades the connection and pumps frames until either side closes.
// TODO(milestone 3): require a short-lived ticket issued by control-api.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.OriginPatterns})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	c := &client{send: make(chan []byte, 16)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	n := len(h.clients)
	h.mu.Unlock()
	slog.Info("ws client connected", "clients", n)
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var f realtimev1.ClientFrame
			if proto.Unmarshal(data, &f) != nil {
				continue
			}
			if vp := f.GetViewport(); vp != nil {
				c.setViewport(vp)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case b := <-c.send:
			wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageBinary, b)
			wcancel()
			if err != nil {
				return
			}
		}
	}
}
