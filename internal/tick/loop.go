package tick

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrLoopRunning = errors.New("tick loop already running")
	ErrLoopStopped = errors.New("tick loop not running")
)

const (
	defaultTargetTPS      = 20
	defaultMaxCatchUpTick = 5
)

// OverloadMode defines behavior when a tick exceeds the configured tick interval.
type OverloadMode uint8

const (
	OverloadSkipSleep OverloadMode = iota
	OverloadCatchUp
)

// WorldTicker represents a world that can be ticked by the loop.
type WorldTicker interface {
	Tick()
}

// WorldManager represents a world registry that also has global tick logic.
type WorldManager interface {
	Tick()
	Worlds() []WorldTicker
}

// LoopConfig configures tick cadence and overload behavior.
type LoopConfig struct {
	TargetTPS          int
	MaxCatchUpTicks    int
	OverloadMode       OverloadMode
	Scheduler          *DefaultScheduler
	SchedulerConfig    SchedulerConfig
	TimingAlpha        float64
	CancelPendingTasks bool
}

// Loop is the central game-thread tick loop.
type Loop struct {
	cfg LoopConfig

	worlds    WorldManager
	scheduler *DefaultScheduler
	timing    *Timing

	interval time.Duration

	running atomic.Uint32
	tick    atomic.Uint64

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewLoop constructs a tick loop instance.
func NewLoop(cfg LoopConfig, worlds WorldManager) (*Loop, error) {
	if cfg.TargetTPS <= 0 {
		cfg.TargetTPS = defaultTargetTPS
	}
	if cfg.MaxCatchUpTicks < 0 {
		cfg.MaxCatchUpTicks = defaultMaxCatchUpTick
	}

	scheduler := cfg.Scheduler
	if scheduler == nil {
		var err error
		scheduler, err = NewScheduler(cfg.SchedulerConfig)
		if err != nil {
			return nil, err
		}
	}

	interval := time.Second / time.Duration(cfg.TargetTPS)
	if interval <= 0 {
		interval = time.Second / defaultTargetTPS
	}

	l := &Loop{
		cfg:       cfg,
		worlds:    worlds,
		scheduler: scheduler,
		timing:    NewTiming(cfg.TimingAlpha),
		interval:  interval,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	return l, nil
}

// Start starts the tick loop goroutine.
func (l *Loop) Start() error {
	if l == nil {
		return ErrLoopStopped
	}
	if !l.running.CompareAndSwap(0, 1) {
		return ErrLoopRunning
	}

	go l.run()
	return nil
}

// Stop requests loop shutdown and waits for completion.
func (l *Loop) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if l.running.Load() == 0 {
		return ErrLoopStopped
	}

	l.scheduler.Close()
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})

	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-l.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := l.scheduler.Stop(ctx, l.cfg.CancelPendingTasks); err != nil {
		return err
	}

	l.running.Store(0)
	return nil
}

// Running reports whether the loop is currently running.
func (l *Loop) Running() bool {
	if l == nil {
		return false
	}
	return l.running.Load() == 1
}

// CurrentTick returns the currently completed tick count.
func (l *Loop) CurrentTick() uint64 {
	if l == nil {
		return 0
	}
	return l.tick.Load()
}

// Scheduler returns the loop scheduler API.
func (l *Loop) Scheduler() Scheduler {
	if l == nil {
		return nil
	}
	return l.scheduler
}

// Timing returns a current timing metrics snapshot.
func (l *Loop) Timing() TimingSnapshot {
	if l == nil {
		return TimingSnapshot{}
	}
	return l.timing.Snapshot()
}

func (l *Loop) run() {
	defer close(l.doneCh)

	nextTickAt := time.Now()
	lastTickStart := nextTickAt

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	for {
		select {
		case <-l.stopCh:
			return
		default:
		}

		tickStart := time.Now()
		tickNumber := l.tick.Add(1)

		l.scheduler.Tick(tickNumber)
		l.tickWorlds()

		tickEnd := time.Now()
		tickDuration := tickEnd.Sub(tickStart)
		tickPeriod := tickStart.Sub(lastTickStart)
		if tickNumber == 1 {
			tickPeriod = l.interval
		}
		l.timing.RecordTick(tickNumber, tickDuration, tickPeriod)
		lastTickStart = tickStart

		nextTickAt = nextTickAt.Add(l.interval)
		sleepDuration := time.Until(nextTickAt)
		if sleepDuration <= 0 {
			l.timing.IncrementOverload()
			l.applyOverloadPolicy(tickEnd, sleepDuration, &nextTickAt)
			continue
		}

		timer.Reset(sleepDuration)
		select {
		case <-timer.C:
		case <-l.stopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (l *Loop) tickWorlds() {
	if l.worlds == nil {
		return
	}

	l.worlds.Tick()
	worlds := l.worlds.Worlds()
	for i := range worlds {
		if worlds[i] != nil {
			worlds[i].Tick()
		}
	}
}

func (l *Loop) applyOverloadPolicy(now time.Time, sleepDuration time.Duration, nextTickAt *time.Time) {
	if nextTickAt == nil {
		return
	}

	if l.cfg.OverloadMode == OverloadSkipSleep {
		*nextTickAt = now
		return
	}

	if l.cfg.MaxCatchUpTicks <= 0 {
		return
	}

	overrun := -sleepDuration
	maxOverrun := l.interval * time.Duration(l.cfg.MaxCatchUpTicks)
	if overrun > maxOverrun {
		*nextTickAt = now.Add(-maxOverrun)
	}
}
