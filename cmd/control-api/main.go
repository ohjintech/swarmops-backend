// control-api serves swarmops.control.v1.FleetService over Connect / gRPC / gRPC-Web.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	connectcors "connectrpc.com/cors"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/cors"

	"github.com/swarm-ops/swarmops-backend/internal/config"
	"github.com/swarm-ops/swarmops-backend/internal/control"
	"github.com/swarm-ops/swarmops-backend/internal/httpx"
	"github.com/swarm-ops/swarmops-backend/internal/mqttx"
	"github.com/swarm-ops/swarmops-backend/internal/store"
	"github.com/swarm-ops/swarmops-backend/internal/topics"
	"github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1/controlv1connect"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadCommon()
	addr := config.String("HTTP_ADDR", ":8080")

	st := store.New(cfg.RedisAddr)
	srv := &control.Server{
		Store:      st,
		StaleAfter: cfg.StaleAfter,
		AckTimeout: config.Duration("ACK_TIMEOUT", 5*time.Second),
	}

	cm, err := mqttx.Connect(ctx, mqttx.Options{
		URL:           cfg.MQTTURL,
		ClientID:      "control-api-" + hostname(),
		Username:      cfg.MQTTUsername,
		Password:      cfg.MQTTPassword,
		Subscriptions: []paho.SubscribeOptions{{Topic: topics.AckAll, QoS: 1}},
		OnMessage:     func(_ string, payload []byte) { srv.HandleAck(payload) },
	})
	if err != nil {
		slog.Error("mqtt", "err", err)
		os.Exit(1)
	}
	srv.MQTT = cm

	mux := http.NewServeMux()
	mux.Handle(controlv1connect.NewFleetServiceHandler(srv))
	mux.HandleFunc("GET /healthz", httpx.Healthz)

	handler := cors.New(cors.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
		MaxAge:         7200,
	}).Handler(mux)

	if err := httpx.Serve(ctx, addr, handler); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
	<-cm.Done()
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
