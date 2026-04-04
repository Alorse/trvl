package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubWatchDaemonTicker struct {
	ch      chan time.Time
	stopped bool
}

func (t *stubWatchDaemonTicker) Chan() <-chan time.Time {
	return t.ch
}

func (t *stubWatchDaemonTicker) Stop() {
	t.stopped = true
}

func TestRunWatchDaemonRunsImmediatelyAndOnTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := &stubWatchDaemonTicker{ch: make(chan time.Time, 1)}
	var buf bytes.Buffer
	runs := 0
	done := make(chan error, 1)

	go func() {
		done <- runWatchDaemon(ctx, &buf, time.Hour, true, func(context.Context) (int, error) {
			runs++
			if runs == 2 {
				cancel()
			}
			return 1, nil
		}, func(time.Duration) watchDaemonTicker {
			return ticker
		})
	}()

	ticker.ch <- time.Now()

	if err := <-done; err != nil {
		t.Fatalf("runWatchDaemon: %v", err)
	}
	if runs != 2 {
		t.Fatalf("run count = %d, want 2", runs)
	}
	if !ticker.stopped {
		t.Fatal("expected ticker to be stopped")
	}

	out := buf.String()
	if !strings.Contains(out, "Starting watch daemon (every 1h0m0s). Press Ctrl-C to stop.") {
		t.Fatalf("missing startup message in %q", out)
	}
	if !strings.Contains(out, "Watch daemon stopped.") {
		t.Fatalf("missing shutdown message in %q", out)
	}
}

func TestRunWatchDaemonLogsErrorsAndContinues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := &stubWatchDaemonTicker{ch: make(chan time.Time, 1)}
	var buf bytes.Buffer
	runs := 0
	done := make(chan error, 1)

	go func() {
		done <- runWatchDaemon(ctx, &buf, time.Hour, true, func(context.Context) (int, error) {
			runs++
			switch runs {
			case 1:
				return 0, errors.New("boom")
			case 2:
				cancel()
				return 1, nil
			default:
				return 1, nil
			}
		}, func(time.Duration) watchDaemonTicker {
			return ticker
		})
	}()

	ticker.ch <- time.Now()

	if err := <-done; err != nil {
		t.Fatalf("runWatchDaemon: %v", err)
	}
	if runs != 2 {
		t.Fatalf("run count = %d, want 2", runs)
	}

	out := buf.String()
	if !strings.Contains(out, "Initial: watch check failed: boom") {
		t.Fatalf("missing initial error log in %q", out)
	}
	if !strings.Contains(out, "Watch daemon stopped.") {
		t.Fatalf("missing shutdown message in %q", out)
	}
}

func TestRunWatchDaemonRejectsInvalidInterval(t *testing.T) {
	err := runWatchDaemon(context.Background(), &bytes.Buffer{}, 0, true, func(context.Context) (int, error) {
		return 0, nil
	}, nil)
	if err == nil {
		t.Fatal("expected invalid interval error")
	}
	if got := err.Error(); got != "watch interval must be greater than zero" {
		t.Fatalf("unexpected error: %q", got)
	}
}
