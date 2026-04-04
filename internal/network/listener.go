package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	listenerStateInit uint32 = iota
	listenerStateRunning
	listenerStateClosing
	listenerStateClosed
)

const (
	defaultAcceptBackoffMin = 5 * time.Millisecond
	defaultAcceptBackoffMax = time.Second
	defaultListenerNetwork  = "tcp"
)

var (
	ErrListenerClosed = errors.New("listener closed")
)

// ListenerConfig configures TCP accept behavior and connection bootstrapping.
type ListenerConfig struct {
	Address             string
	Network             string
	AcceptBackoffMin    time.Duration
	AcceptBackoffMax    time.Duration
	MaxConnections      int
	WorkerPoolSize      int
	WorkerQueueCapacity int
	BufferClassSizes    []int
	ConnConfig          ConnConfig
	PipelineFactory     func(bufferPool *BufferPool, maxFrameSize int) Pipeline
	SessionFactory      func(conn *Conn, ctx context.Context) Session
	OnConnection        func(conn *Conn) error
	OnConnectionClosed  func(conn *Conn, err error)
	OnError             func(err error)
}

// Listener manages the network accept loop and active connections.
type Listener struct {
	listener   net.Listener
	cfg        ListenerConfig
	workerPool *WorkerPool
	bufferPool *BufferPool

	maxConnections atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc

	state        atomic.Uint32
	activeConns  atomic.Int64
	acceptWg     sync.WaitGroup
	closeOnce    sync.Once
	connections  sync.Map
	done         chan struct{}
	listenerLock sync.Mutex
}

// NewListener creates a configured TCP listener ready to start.
func NewListener(cfg ListenerConfig) (*Listener, error) {
	cfg = normalizeListenerConfig(cfg)

	raw, err := net.Listen(cfg.Network, cfg.Address)
	if err != nil {
		return nil, err
	}

	workerPool := NewWorkerPool(WorkerPoolConfig{
		Size:          cfg.WorkerPoolSize,
		QueueCapacity: cfg.WorkerQueueCapacity,
	})
	bufferPool := NewBufferPool(cfg.BufferClassSizes)

	ctx, cancel := context.WithCancel(context.Background())
	l := &Listener{
		listener:   raw,
		cfg:        cfg,
		workerPool: workerPool,
		bufferPool: bufferPool,
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	l.maxConnections.Store(int64(cfg.MaxConnections))
	l.state.Store(listenerStateInit)
	return l, nil
}

// Start starts the accept loop.
func (l *Listener) Start() error {
	if !l.state.CompareAndSwap(listenerStateInit, listenerStateRunning) {
		if l.state.Load() == listenerStateRunning {
			return nil
		}
		return ErrListenerClosed
	}

	l.acceptWg.Add(1)
	go l.acceptLoop()
	return nil
}

// Addr returns the bound listener address.
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

// Done is closed when listener shutdown is complete.
func (l *Listener) Done() <-chan struct{} {
	return l.done
}

// ActiveConnections returns the current number of active connections.
func (l *Listener) ActiveConnections() int {
	return int(l.activeConns.Load())
}

// MaxConnections returns the current connection admission limit.
func (l *Listener) MaxConnections() int {
	if l == nil {
		return 0
	}
	return int(l.maxConnections.Load())
}

// SetMaxConnections updates the connection admission limit for future accepts.
func (l *Listener) SetMaxConnections(maxConnections int) {
	if l == nil {
		return
	}
	l.maxConnections.Store(int64(maxConnections))
}

// Shutdown gracefully closes listener and active connections.
func (l *Listener) Shutdown(ctx context.Context) error {
	if l.state.Load() == listenerStateClosed {
		return nil
	}

	l.beginClose()

	done := make(chan struct{})
	go func() {
		l.acceptWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	closeErr := l.closeAllConnections(false)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	poolErr := l.workerPool.Stop(stopCtx)
	stopCancel()

	l.finishClose()

	if closeErr != nil {
		return closeErr
	}
	if poolErr != nil {
		return poolErr
	}
	return nil
}

// Close force-closes listener and all active connections.
func (l *Listener) Close() error {
	if l.state.Load() == listenerStateClosed {
		return nil
	}

	l.beginClose()
	l.acceptWg.Wait()
	closeErr := l.closeAllConnections(true)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	poolErr := l.workerPool.Stop(stopCtx)
	stopCancel()

	l.finishClose()

	if closeErr != nil {
		return closeErr
	}
	if poolErr != nil {
		return poolErr
	}
	return nil
}

func (l *Listener) acceptLoop() {
	defer l.acceptWg.Done()

	backoff := l.cfg.AcceptBackoffMin
	for {
		rawConn, err := l.listener.Accept()
		if err != nil {
			if l.ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				l.reportError(err)
				time.Sleep(backoff)
				backoff <<= 1
				if backoff > l.cfg.AcceptBackoffMax {
					backoff = l.cfg.AcceptBackoffMax
				}
				continue
			}

			l.reportError(err)
			if l.ctx.Err() != nil {
				return
			}
			continue
		}

		backoff = l.cfg.AcceptBackoffMin
		if !l.admitConnection(rawConn) {
			_ = rawConn.Close()
		}
	}
}

func (l *Listener) admitConnection(raw net.Conn) bool {
	if l.ctx.Err() != nil {
		return false
	}

	maxConnections := l.MaxConnections()
	if maxConnections > 0 && l.ActiveConnections() >= maxConnections {
		l.reportError(fmt.Errorf("max connections reached: %d", maxConnections))
		return false
	}

	connCfg := l.cfg.ConnConfig
	originalOnClose := connCfg.OnClose
	connCfg.SessionFactory = l.cfg.SessionFactory
	connCfg.OnClose = func(c *Conn, err error) {
		l.connections.Delete(c)
		l.activeConns.Add(-1)
		if l.cfg.OnConnectionClosed != nil {
			l.cfg.OnConnectionClosed(c, err)
		}
		if originalOnClose != nil {
			originalOnClose(c, err)
		}
	}

	if connCfg.OnError == nil {
		connCfg.OnError = func(_ *Conn, err error) {
			l.reportError(err)
		}
	} else {
		originalOnError := connCfg.OnError
		connCfg.OnError = func(c *Conn, err error) {
			l.reportError(err)
			originalOnError(c, err)
		}
	}

	pipeline := l.newPipeline(connCfg.MaxFrameSize)
	conn := NewConn(raw, pipeline, l.workerPool, l.bufferPool, connCfg)
	l.connections.Store(conn, struct{}{})
	l.activeConns.Add(1)

	if l.cfg.OnConnection != nil {
		if err := l.cfg.OnConnection(conn); err != nil {
			l.reportError(err)
			_ = conn.ForceClose(err)
			return false
		}
	}

	conn.Start()
	return true
}

func (l *Listener) closeAllConnections(force bool) error {
	var firstErr error
	l.connections.Range(func(key, _ any) bool {
		conn, ok := key.(*Conn)
		if !ok {
			return true
		}

		var err error
		if force {
			err = conn.ForceClose(ErrListenerClosed)
		} else {
			err = conn.Close()
		}

		if err != nil && firstErr == nil {
			firstErr = err
		}
		return true
	})
	return firstErr
}

func (l *Listener) beginClose() {
	l.listenerLock.Lock()
	defer l.listenerLock.Unlock()

	if l.state.Load() == listenerStateClosed || l.state.Load() == listenerStateClosing {
		return
	}

	l.state.Store(listenerStateClosing)
	l.cancel()
	_ = l.listener.Close()
}

func (l *Listener) finishClose() {
	l.closeOnce.Do(func() {
		l.state.Store(listenerStateClosed)
		close(l.done)
	})
}

func (l *Listener) newPipeline(maxFrameSize int) Pipeline {
	if l.cfg.PipelineFactory != nil {
		return l.cfg.PipelineFactory(l.bufferPool, maxFrameSize)
	}
	return NewPipeline(l.bufferPool, maxFrameSize)
}

func (l *Listener) reportError(err error) {
	if err == nil {
		return
	}
	if l.cfg.OnError != nil {
		l.cfg.OnError(err)
	}
}

func normalizeListenerConfig(cfg ListenerConfig) ListenerConfig {
	if cfg.Network == "" {
		cfg.Network = defaultListenerNetwork
	}
	if cfg.AcceptBackoffMin <= 0 {
		cfg.AcceptBackoffMin = defaultAcceptBackoffMin
	}
	if cfg.AcceptBackoffMax < cfg.AcceptBackoffMin {
		cfg.AcceptBackoffMax = defaultAcceptBackoffMax
	}
	if cfg.WorkerPoolSize <= 0 {
		cfg.WorkerPoolSize = 1
	}
	if cfg.WorkerQueueCapacity <= 0 {
		cfg.WorkerQueueCapacity = cfg.WorkerPoolSize * 1024
	}
	return cfg
}
