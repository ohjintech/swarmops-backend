// Package topics builds and parses the MQTT topic scheme defined in swarmops-contracts.
package topics

import (
	"fmt"
	"strings"
)

const (
	TelemetryAll = "swarm/v1/+/+/telemetry"
	AckAll       = "swarm/v1/+/+/ack"
	EventAll     = "swarm/v1/+/+/event"
)

func Command(fleetID, droneID string) string {
	return fmt.Sprintf("swarm/v1/%s/%s/cmd", fleetID, droneID)
}

// Parse splits swarm/v1/{fleet}/{drone}/{kind}.
func Parse(topic string) (fleetID, droneID, kind string, ok bool) {
	p := strings.Split(topic, "/")
	if len(p) != 5 || p[0] != "swarm" || p[1] != "v1" {
		return "", "", "", false
	}
	return p[2], p[3], p[4], true
}
