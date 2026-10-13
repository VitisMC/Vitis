package world

import (
	"context"
	"runtime"
	"runtime/debug"
	"time"
)

// releaseGenerationMemoryLocked schedules one cancellable reclamation after a job stops.
// The pregeneration mutex protects the handoff to a subsequent job.
func (p *pregeneration) releaseGenerationMemoryLocked() {
	p.startGenerationReleaseLocked(false)
}

// startGenerationReleaseLocked serializes cache reclamation for jobs and idle worlds.
func (p *pregeneration) startGenerationReleaseLocked(idle bool) {
	p.cancelGenerationReleaseLocked()
	previous := p.releaseDone
	ctx, cancel := context.WithCancel(context.Background())
	p.releaseCancel = cancel
	p.releaseDone = make(chan struct{})
	p.releaseForIdle = idle
	go p.releaseGenerationMemory(ctx, p.releaseDone, previous, idle)
}

// cancelGenerationReleaseLocked prevents pending reclamation from clearing a new job's caches.
func (p *pregeneration) cancelGenerationReleaseLocked() {
	if p.releaseCancel != nil {
		p.releaseCancel()
		p.releaseCancel = nil
	}
}

// releaseGenerationMemory waits off the tick thread for generation to become idle.
func (p *pregeneration) releaseGenerationMemory(ctx context.Context, done chan<- struct{}, previous <-chan struct{}, idle bool) {
	defer close(done)
	if previous != nil {
		// Serialize reclamation so shutdown joins every preceding worker too.
		<-previous
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		p.mu.Lock()
		if ctx.Err() != nil || p.state == "running" || idle && p.w.tracker.Count() != 0 {
			p.mu.Unlock()
			return
		}
		released := p.w.chunks.TryReleaseGenerationCaches()
		p.mu.Unlock()
		if released {
			if ctx.Err() == nil {
				// Two cycles release both generations of sync.Pool scratch.
				runtime.GC()
			}
			if ctx.Err() == nil {
				debug.FreeOSMemory()
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// stopGenerationRelease cancels reclamation and joins its worker during shutdown.
func (p *pregeneration) stopGenerationRelease(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.mu.Lock()
	p.cancelGenerationReleaseLocked()
	done := p.releaseDone
	p.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
