// Package mqttx wraps paho's autopaho with the defaults every SwarmOps service uses.
package mqttx

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

type Options struct {
	URL           string
	ClientID      string
	Username      string
	Password      string
	Subscriptions []paho.SubscribeOptions
	OnMessage     func(topic string, payload []byte)
}

// Connect starts a self-reconnecting MQTT v5 connection. Subscriptions are
// (re)established on every successful connect.
func Connect(ctx context.Context, o Options) (*autopaho.ConnectionManager, error) {
	u, err := url.Parse(o.URL)
	if err != nil {
		return nil, fmt.Errorf("parse MQTT_URL: %w", err)
	}
	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{u},
		KeepAlive:                     20,
		CleanStartOnInitialConnection: true,
		ConnectUsername:               o.Username,
		ConnectPassword:               []byte(o.Password),
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			slog.Info("mqtt connected", "client", o.ClientID, "url", o.URL)
			if len(o.Subscriptions) == 0 {
				return
			}
			if _, err := cm.Subscribe(ctx, &paho.Subscribe{Subscriptions: o.Subscriptions}); err != nil {
				slog.Error("mqtt subscribe failed", "err", err)
			}
		},
		OnConnectError: func(err error) { slog.Warn("mqtt connect failed", "err", err) },
		ClientConfig: paho.ClientConfig{
			ClientID: o.ClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					if o.OnMessage != nil {
						o.OnMessage(pr.Packet.Topic, pr.Packet.Payload)
					}
					return true, nil
				},
			},
			OnClientError: func(err error) { slog.Warn("mqtt client error", "err", err) },
		},
	}
	return autopaho.NewConnection(ctx, cfg)
}
