package course

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"course/internal/config"
)

func TestAppGracefulShutdownWaitsForInFlightRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	var logs bytes.Buffer
	app := &App{
		cfg:    &config.Config{ServiceName: "course-service", Environment: "test", ShutdownTimeout: 2 * time.Second},
		logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		server: &http.Server{Addr: address, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusOK)
		})},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() { runResult <- app.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", dialErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	requestResult := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + address)
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				err = context.DeadlineExceeded
			}
		}
		requestResult <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach the server")
	}
	cancel()
	select {
	case err := <-runResult:
		t.Fatalf("server stopped before the in-flight request finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-requestResult:
		if err != nil {
			t.Fatalf("in-flight request failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request did not finish")
	}
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("graceful shutdown timed out")
	}
	if !strings.Contains(logs.String(), `"msg":"server_stopped"`) {
		t.Fatal("missing server_stopped log event")
	}
}
