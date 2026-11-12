// Package config reads service configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func String(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func Duration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func Int(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// List splits a comma-separated env var, trimming blanks.
func List(key, def string) []string {
	var out []string
	for _, s := range strings.Split(String(key, def), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Common holds settings shared by every backend service.
type Common struct {
	MQTTURL        string
	MQTTUsername   string
	MQTTPassword   string
	RedisAddr      string
	AllowedOrigins []string
	StaleAfter     time.Duration
}

func LoadCommon() Common {
	return Common{
		MQTTURL:        String("MQTT_URL", "mqtt://localhost:1883"),
		MQTTUsername:   String("MQTT_USERNAME", ""),
		MQTTPassword:   String("MQTT_PASSWORD", ""),
		RedisAddr:      String("REDIS_ADDR", "localhost:6379"),
		AllowedOrigins: List("ALLOWED_ORIGINS", "http://localhost:5173"),
		StaleAfter:     Duration("STALE_AFTER", 3*time.Second),
	}
}
