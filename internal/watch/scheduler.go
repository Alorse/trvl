package watch

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Scheduler runs periodic price checks in the background.
// Call Start to launch the goroutine and Stop for graceful shutdown.
// Start and Stop are each idempotent and safe for concurrent use.
type Scheduler struct {
	dir      string        // root directory for the watch store (e.g. ~/.trvl)
	interval time.Duration // how often to run checks
	checker  PriceChecker  // injected for testability

	startOnce sync.Once
	cancel    context.CancelFunc
	done      chan struct{}
}

// NoopChecker is a PriceChecker that always returns zero price.
// Used when no real price source is available.
type NoopChecker struct{}

func (NoopChecker) CheckPrice(_ context.Context, w Watch) (float64, string, string, error) {
	return 0, w.Currency, "", nil
}

// NewScheduler creates a Scheduler. If checker is nil, NoopChecker is used.
// interval defaults to 30 minutes if zero.
func NewScheduler(dir string, interval time.Duration, checker PriceChecker) *Scheduler {
	if checker == nil {
		checker = NoopChecker{}
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Scheduler{
		dir:      dir,
		interval: interval,
		checker:  checker,
		done:     make(chan struct{}),
	}
}

// Start launches the background goroutine. Idempotent — subsequent calls are no-ops.
func (s *Scheduler) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go s.run(ctx)
	})
}

// Stop signals the background goroutine to exit and waits for it to finish.
// Any in-flight price check is cancelled immediately.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
}

// run is the background loop. ctx is cancelled when Stop is called.
func (s *Scheduler) run(ctx context.Context) {
	defer close(s.done)

	// Run one check immediately on startup, then repeat on interval.
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce performs a single round of price checks.
// ctx should be cancelled (via Stop) to abort in-flight checks promptly.
func (s *Scheduler) runOnce(ctx context.Context) {
	store := NewStore(s.dir)
	if err := store.Load(); err != nil {
		slog.Warn("scheduler: load watches", "err", err)
		return
	}

	watches := store.List()
	if len(watches) == 0 {
		return
	}

	// Filter to active watches only: not expired and travel date not passed.
	active := activeWatches(watches)
	if len(active) == 0 {
		return
	}

	// Bound check duration to half the interval but also respect the stop signal.
	checkCtx, cancel := context.WithTimeout(ctx, s.interval/2)
	defer cancel()

	results := CheckAll(checkCtx, store, s.checker)

	triggered := 0
	for _, r := range results {
		if r.Error != nil {
			slog.Warn("scheduler: check error",
				"watch_id", r.Watch.ID,
				"route", r.Watch.Origin+"→"+r.Watch.Destination,
				"err", r.Error,
			)
			continue
		}
		if r.NewPrice > 0 {
			slog.Info("scheduler: price checked",
				"watch_id", r.Watch.ID,
				"route", r.Watch.Origin+"→"+r.Watch.Destination,
				"price", r.NewPrice,
				"currency", r.Currency,
				"below_goal", r.BelowGoal,
			)
		}
		if r.BelowGoal {
			triggered++
			slog.Info("scheduler: price below target",
				"watch_id", r.Watch.ID,
				"route", r.Watch.Origin+"→"+r.Watch.Destination,
				"price", r.NewPrice,
				"target", r.Watch.BelowPrice,
				"currency", r.Currency,
			)
		}
	}

	slog.Info("scheduler: check complete",
		"checked", len(results),
		"triggered", triggered,
	)
}

// activeWatches filters watches to those that are still worth checking:
//   - no depart date set (route watch), OR
//   - depart date is today or in the future
func activeWatches(watches []Watch) []Watch {
	now := time.Now()
	today := now.Format("2006-01-02")

	var active []Watch
	for _, w := range watches {
		if isActive(w, today) {
			active = append(active, w)
		}
	}
	return active
}

// isActive returns true if the watch should still be checked.
func isActive(w Watch, today string) bool {
	// Route watches (no dates) are always active.
	if w.IsRouteWatch() {
		return true
	}

	// Date-range watches: active if the range end is today or later.
	if w.IsDateRange() {
		return w.DepartTo >= today
	}

	// Room and specific-date watches: active if depart date is today or later.
	if w.DepartDate != "" {
		return w.DepartDate >= today
	}

	return true
}
