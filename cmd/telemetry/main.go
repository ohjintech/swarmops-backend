// telemetry ingests drone MQTT traffic and serves the realtime WebSocket gateway.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eclipse/paho.golang/paho"

	"github.com/swarm-ops/swarmops-backend/internal/config"
	"github.com/swarm-ops/swarmops-backend/internal/httpx"
	"github.com/swarm-ops/swarmops-backend/internal/mqttx"
	"github.com/swarm-ops/swarmops-backend/internal/store"
	"github.com/swarm-ops/swarmops-backend/internal/telemetry"
	"github.com/swarm-ops/swarmops-backend/internal/topics"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadCommon()
	addr := config.String("HTTP_ADDR", ":8081")
	snapshotEvery := config.Duration("SNAPSHOT_INTERVAL", 250*time.Millisecond)

	hub := telemetry.NewHub(httpx.OriginHosts(cfg.AllowedOrigins))
	in := &telemetry.Ingest{Hub: hub, Store: store.New(cfg.RedisAddr)}

	h, _ := os.Hostname()
	cm, err := mqttx.Connect(ctx, mqttx.Options{
		URL:      cfg.MQTTURL,
		ClientID: "telemetry-" + h,
		Username: cfg.MQTTUsername,
		Password: cfg.MQTTPassword,
		Subscriptions: []paho.SubscribeOptions{
			{Topic: topics.TelemetryAll, QoS: 0},
			{Topic: topics.AckAll, QoS: 1},
			{Topic: topics.EventAll, QoS: 1},
		},
		OnMessage: in.HandleMessage,
	})
	if err != nil {
		slog.Error("mqtt", "err", err)
		os.Exit(1)
	}

	go hub.Run(ctx, snapshotEvery)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", hub.ServeWS)
	mux.HandleFunc("GET /healthz", httpx.Healthz)

	if err := httpx.Serve(ctx, addr, mux); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
	<-cm.Done()
}
