// Package httpx holds HTTP server plumbing shared by the Go services.
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Serve runs an HTTP/1.1 + cleartext HTTP/2 (h2c) server until ctx is cancelled.
// h2c lets gRPC clients talk to Connect handlers without TLS inside the compose network.
func Serve(ctx context.Context, addr string, h http.Handler) error {
	p := new(http.Protocols)
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	srv := &http.Server{Addr: addr, Handler: h, Protocols: p, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// OriginHosts converts "http://localhost:5173" style origins into the host patterns
// that coder/websocket's OriginPatterns expects ("localhost:5173").
func OriginHosts(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
