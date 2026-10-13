package network

import (
	"context"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

const (
	connStateOpen uint32 = iota
	connStateClosing
	connStateClosed
)

const (
	defaultReadBufferSize        = 4096
	defaultReadAccumulatorSize   = 8192
	defaultWriteQueueCapacity    = 1024
	defaultMaxFrameSize          = 2 << 20
	defaultMaxWriteBatchFrames   = 32
	defaultMaxWriteBatchBytes    = 128 << 10
	defaultOwnedWorkerPoolSize   = 1
	defaultOwnedWorkerPoolQueued = 256
)

var (
	ErrConnClosed         = errors.New("connection closed")
	ErrConnClosing        = errors.New("connection closing")
	ErrWriteQueueSaturate = errors.New("write queue saturated")
	ErrInboundSaturate    = errors.New("inbound worker queue saturated")
	ErrIdleTimeout        = errors.New("connection idle timeout")
)

// WriteBackpressureMode controls behavior when the write queue is full.
type WriteBackpressureMode int

const (
	// WriteBackpressureDropConn closes the connection when queue capacity is exceeded.
	WriteBackpressureDropConn WriteBackpressureMode = iota
	// WriteBackpressureBlock blocks the sender until queue space becomes available.
	WriteBackpressureBlock
)

// InboundDispatchMode controls behavior when worker queue is saturated.
type InboundDispatchMode int

const (
	// InboundDispatchDropConn closes the connection when worker queue is full.
	InboundDispatchDropConn InboundDispatchMode = iota
	// InboundDispatchBlock blocks read loop until worker queue space is available.
	InboundDispatchBlock
)

type outboundFrame struct {
	frame     EncodedFrame
	encrypted bool
}

// ConnConfig configures connection hot-path behavior.
type ConnConfig struct {
	ReadBufferSize        int
	ReadAccumulatorSize   int
	MaxFrameSize          int
	WriteQueueCapacity    int
	WriteBackpressureMode WriteBackpressureMode
	InboundDispatchMode   InboundDispatchMode
	MaxWriteBatchFrames   int
	MaxWriteBatchBytes    int
	ReadDeadline          time.Duration
	WriteDeadline         time.Duration
	IdleTimeout           time.Duration
	SessionFactory        func(conn *Conn, ctx context.Context) Session
	OnError               func(conn *Conn, err error)
	OnClose               func(conn *Conn, err error)
}

// Conn represents a single client TCP connection.
type Conn struct {
	raw        net.Conn
	pipeline   Pipeline
	workerPool *WorkerPool
	bufferPool *BufferPool

	cfg        ConnConfig
	writeQueue *BoundedMPSCQueue[outboundFrame]

	ctx    context.Context
	cancel context.CancelFunc

	session Session

	state      atomic.Uint32
	started    atomic.Uint32
	ownedPool  bool
	closeCause error

	compressionThreshold atomic.Int32
	lastReadNanos        atomic.Int64

	cipherMu      sync.Mutex
	encryptStream cipher.Stream
	decryptStream cipher.Stream
	cipherEnabled atomic.Bool

	closeCauseMu sync.Mutex
	closeOnce    sync.Once
	startOnce    sync.Once
	wg           sync.WaitGroup
	done         chan struct{}
}

// NewConn builds a connection with independent read and write loops.
func NewConn(raw net.Conn, pipeline Pipeline, workerPool *WorkerPool, bufferPool *BufferPool, cfg ConnConfig) *Conn {
	if raw == nil {
		panic("new conn: nil net.Conn")
	}

	cfg = normalizeConnConfig(cfg)
	if bufferPool == nil {
		bufferPool = NewBufferPool(nil)
	}
	if pipeline == nil {
		pipeline = NewPipeline(bufferPool, cfg.MaxFrameSize)
	}

	ownedPool := false
	if workerPool == nil {
		workerPool = NewWorkerPool(WorkerPoolConfig{
			Size:          defaultOwnedWorkerPoolSize,
			QueueCapacity: defaultOwnedWorkerPoolQueued,
		})
		ownedPool = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Conn{
		raw:        raw,
		pipeline:   pipeline,
		workerPool: workerPool,
		bufferPool: bufferPool,
		cfg:        cfg,
		writeQueue: NewBoundedMPSCQueue[outboundFrame](cfg.WriteQueueCapacity),
		ctx:        ctx,
		cancel:     cancel,
		ownedPool:  ownedPool,
		done:       make(chan struct{}),
	}
	c.state.Store(connStateOpen)
	c.lastReadNanos.Store(time.Now().UnixNano())
	c.compressionThreshold.Store(-1)
	if cfg.SessionFactory != nil {
		c.session = cfg.SessionFactory(c, ctx)
	} else {
		c.session = newConnSession(c, ctx)
	}

	return c
}

// Start starts the read and write loops for the connection.
func (c *Conn) Start() {
	c.startOnce.Do(func() {
		if c.state.Load() != connStateOpen {
			c.finalize()
			return
		}

		c.started.Store(1)
		c.wg.Add(2)
		go c.readLoopRunner()
		go c.writeLoopRunner()
		go func() {
			c.wg.Wait()
			c.finalize()
		}()
	})
}

// Session returns the bound session abstraction.
func (c *Conn) Session() Session {
	return c.session
}

// Context returns a context cancelled on connection shutdown.
func (c *Conn) Context() context.Context {
	return c.ctx
}

// RemoteAddr returns the remote endpoint address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.raw.RemoteAddr()
}

// LocalAddr returns the local endpoint address.
func (c *Conn) LocalAddr() net.Addr {
	return c.raw.LocalAddr()
}

// EnableCompression activates protocol compression with the given threshold.
func (c *Conn) EnableCompression(threshold int) {
	c.compressionThreshold.Store(int32(threshold))
	if dp, ok := c.pipeline.(*DefaultPipeline); ok {
		dp.EnableCompression(threshold)
	}
}

// CompressionThreshold returns the current compression threshold, or -1 if disabled.
func (c *Conn) CompressionThreshold() int {
	return int(c.compressionThreshold.Load())
}

// EnableEncryption sets cipher streams on the connection for encryption.
func (c *Conn) EnableEncryption(encrypt, decrypt interface{}) {
	enc, ok1 := encrypt.(cipher.Stream)
	dec, ok2 := decrypt.(cipher.Stream)
	if !ok1 || !ok2 {
		return
	}
	c.cipherMu.Lock()
	c.encryptStream = enc
	c.decryptStream = dec
	c.cipherEnabled.Store(true)
	c.cipherMu.Unlock()
}

// Done is closed after the connection reaches the closed state.
func (c *Conn) Done() <-chan struct{} {
	return c.done
}

// Close performs graceful shutdown and waits until closed.
func (c *Conn) Close() error {
	c.initiateClose(nil, false)
	if c.started.Load() == 0 {
		c.finalize()
	}
	<-c.done
	return c.CloseCause()
}

// ForceClose closes the connection immediately and waits until closed.
func (c *Conn) ForceClose(err error) error {
	c.initiateClose(err, true)
	if c.started.Load() == 0 {
		c.finalize()
	}
	<-c.done
	return c.CloseCause()
}

// CloseCause returns the first non-nil close cause observed by the connection.
func (c *Conn) CloseCause() error {
	c.closeCauseMu.Lock()
	defer c.closeCauseMu.Unlock()
	return c.closeCause
}

// Send enqueues an outbound packet through the configured pipeline.
func (c *Conn) Send(packet *Packet) error {
	if packet == nil {
		return fmt.Errorf("send packet: nil packet")
	}

	state := c.state.Load()
	if state == connStateClosed {
		return ErrConnClosed
	}
	if state == connStateClosing {
		return ErrConnClosing
	}

	frame, err := c.pipeline.FireOutbound(c.session, packet)
	if err != nil {
		c.handleAsyncPipelineError(err)
		return err
	}

	item := outboundFrame{frame: frame, encrypted: c.cipherEnabled.Load()}
	switch c.cfg.WriteBackpressureMode {
	case WriteBackpressureBlock:
		err = c.writeQueue.EnqueueWait(c.ctx, item)
	default:
		err = c.writeQueue.Enqueue(item)
		if errors.Is(err, ErrQueueFull) {
			err = fmt.Errorf("%w: %d", ErrWriteQueueSaturate, c.writeQueue.Capacity())
			c.releaseOutbound(item)
			c.initiateClose(err, true)
			return err
		}
	}

	if err != nil {
		c.releaseOutbound(item)
		if errors.Is(err, context.Canceled) || errors.Is(err, ErrQueueClosed) {
			return ErrConnClosing
		}
		return err
	}

	return nil
}

func (c *Conn) readLoopRunner() {
	defer c.wg.Done()
	defer c.recoverLoopPanic("read loop")

	if err := c.readLoop(); err != nil {
		c.reportError(err)
		if IsProtocolError(err) {
			c.initiateClose(err, true)
			return
		}
		c.initiateClose(err, false)
		return
	}

	c.initiateClose(nil, false)
}

func (c *Conn) writeLoopRunner() {
	defer c.wg.Done()
	defer c.recoverLoopPanic("write loop")

	if err := c.writeLoop(); err != nil {
		c.reportError(err)
		c.initiateClose(err, true)
		return
	}

	c.initiateClose(nil, false)
}

func (c *Conn) readLoop() error {
	readBuf := c.bufferPool.Get(c.cfg.ReadBufferSize)
	defer c.bufferPool.Put(readBuf)

	acc := c.bufferPool.Get(c.cfg.ReadAccumulatorSize)
	defer func() {
		c.bufferPool.Put(acc)
	}()

	start := 0
	end := 0

	for {
		if c.ctx.Err() != nil {
			return nil
		}

		if end == len(acc) {
			if start > 0 {
				copy(acc, acc[start:end])
				end -= start
				start = 0
			} else {
				var grown []byte
				grown, start, end = c.growAccumulator(acc, start, end)
				c.bufferPool.Put(acc)
				acc = grown
			}
		}

		if readWait := c.nextReadWait(); readWait > 0 {
			if err := c.raw.SetReadDeadline(time.Now().Add(readWait)); err != nil {
				return err
			}
		}

		readEncrypted := c.cipherEnabled.Load()
		n, err := c.raw.Read(readBuf)
		if n > 0 {
			c.lastReadNanos.Store(time.Now().UnixNano())
			if readEncrypted {
				c.decrypt(readBuf[:n])
			}
			if end+n > len(acc) {
				if start > 0 {
					copy(acc, acc[start:end])
					end -= start
					start = 0
				}
				for end+n > len(acc) {
					var grown []byte
					grown, start, end = c.growAccumulator(acc, start, end)
					c.bufferPool.Put(acc)
					acc = grown
				}
			}

			copy(acc[end:end+n], readBuf[:n])
			end += n

			for start < end {
				frameLen, headerLen, complete, decodeErr := TryDecodeFrame(acc[start:end], c.cfg.MaxFrameSize)
				if decodeErr != nil {
					return decodeErr
				}
				if !complete {
					break
				}

				frameStart := start + headerLen
				frameEnd := frameStart + frameLen
				frameBuf := c.bufferPool.Get(frameLen)
				copy(frameBuf, acc[frameStart:frameEnd])

				dispatchBuf := frameBuf[:frameLen]
				if threshold := c.compressionThreshold.Load(); threshold >= 0 {
					decompressed, decompErr := DecompressFrameWithThreshold(dispatchBuf, int(threshold))
					if decompErr != nil {
						c.bufferPool.Put(frameBuf)
						return NewProtocolError(decompErr)
					}
					c.bufferPool.Put(frameBuf)
					dispatchBuf = decompressed
					frameBuf = nil
				}

				wasEncrypted := c.cipherEnabled.Load()
				if submitErr := c.dispatchInbound(dispatchBuf); submitErr != nil {
					return submitErr
				}

				start = frameEnd
				if !wasEncrypted && c.cipherEnabled.Load() && start < end {
					c.decrypt(acc[start:end])
				}
			}

			if start == end {
				start = 0
				end = 0
			} else if start > 0 && start >= len(acc)/2 {
				copy(acc, acc[start:end])
				end -= start
				start = 0
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}

			if c.ctx.Err() != nil {
				return nil
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if c.state.Load() != connStateOpen {
					return nil
				}
				if c.cfg.IdleTimeout <= 0 {
					return fmt.Errorf("read timeout: %w", err)
				}
				lastRead := time.Unix(0, c.lastReadNanos.Load())
				if time.Since(lastRead) >= c.cfg.IdleTimeout {
					return ErrIdleTimeout
				}
				continue
			}

			return err
		}
	}
}

func (c *Conn) writeLoop() error {
	batch := make([]outboundFrame, 0, c.cfg.MaxWriteBatchFrames)

	for {
		item, err := c.writeQueue.DequeueWait(c.ctx)
		if err != nil {
			if errors.Is(err, ErrQueueClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		batch = batch[:0]
		batch = append(batch, item)
		totalBytes := len(item.frame.Bytes)

		for len(batch) < c.cfg.MaxWriteBatchFrames && totalBytes < c.cfg.MaxWriteBatchBytes {
			next, ok, dequeueErr := c.writeQueue.TryDequeue()
			if dequeueErr != nil {
				if errors.Is(dequeueErr, ErrQueueClosed) {
					break
				}
				c.releaseBatch(batch)
				return dequeueErr
			}
			if !ok {
				break
			}

			batch = append(batch, next)
			totalBytes += len(next.frame.Bytes)
		}

		if c.cfg.WriteDeadline > 0 {
			if setErr := c.raw.SetWriteDeadline(time.Now().Add(c.cfg.WriteDeadline)); setErr != nil {
				c.releaseBatch(batch)
				return setErr
			}
		}

		var writeErr error
		for i := range batch {
			if batch[i].encrypted {
				writeErr = c.writeEncrypted(batch[i].frame.Bytes)
			} else {
				writeErr = writeAll(c.raw, batch[i].frame.Bytes)
			}
			if writeErr != nil {
				break
			}
		}
		c.releaseBatch(batch)
		if writeErr != nil {
			if errors.Is(writeErr, net.ErrClosed) || errors.Is(writeErr, io.EOF) {
				return nil
			}
			if c.ctx.Err() != nil {
				return nil
			}
			return writeErr
		}
	}
}

func (c *Conn) dispatchInbound(frame []byte) error {
	defer c.bufferPool.Put(frame)
	return c.pipeline.FireInbound(c.session, frame)
}

func (c *Conn) initiateClose(err error, force bool) {
	if err != nil {
		c.storeCloseCause(err)
	}

	for {
		state := c.state.Load()
		switch state {
		case connStateClosed:
			if force {
				_ = c.raw.Close()
			}
			return
		case connStateClosing:
			if force {
				_ = c.raw.Close()
			}
			return
		case connStateOpen:
			if !c.state.CompareAndSwap(connStateOpen, connStateClosing) {
				continue
			}

			c.writeQueue.Close()
			_ = c.raw.SetReadDeadline(time.Now())
			if force {
				c.cancel()
				_ = c.raw.SetWriteDeadline(time.Now())
				_ = c.raw.Close()
			}
			return
		default:
			return
		}
	}
}

func (c *Conn) nextReadWait() time.Duration {
	wait := c.cfg.ReadDeadline
	if c.cfg.IdleTimeout <= 0 {
		return wait
	}
	remaining := c.cfg.IdleTimeout - time.Since(time.Unix(0, c.lastReadNanos.Load()))
	if remaining <= 0 {
		return time.Nanosecond
	}
	if wait <= 0 || remaining < wait {
		return remaining
	}
	return wait
}

func (c *Conn) finalize() {
	c.closeOnce.Do(func() {
		c.state.Store(connStateClosed)
		c.cancel()
		_ = c.raw.Close()

		if c.ownedPool {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
			_ = c.workerPool.Stop(stopCtx)
			stopCancel()
		}

		close(c.done)
		if c.cfg.OnClose != nil {
			c.cfg.OnClose(c, c.CloseCause())
		}
	})
}

func (c *Conn) decrypt(data []byte) {
	c.cipherMu.Lock()
	if c.decryptStream != nil {
		c.decryptStream.XORKeyStream(data, data)
	}
	c.cipherMu.Unlock()
}

func (c *Conn) writeEncrypted(data []byte) error {
	c.cipherMu.Lock()
	defer c.cipherMu.Unlock()
	if c.encryptStream == nil {
		return fmt.Errorf("encrypted write without cipher")
	}
	buf := c.bufferPool.Get(len(data))
	defer c.bufferPool.Put(buf)
	c.encryptStream.XORKeyStream(buf[:len(data)], data)
	return writeAll(c.raw, buf[:len(data)])
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func (c *Conn) growAccumulator(acc []byte, start int, end int) ([]byte, int, int) {
	used := end - start
	newSize := len(acc) << 1
	if newSize == 0 {
		newSize = c.cfg.ReadAccumulatorSize
	}
	if newSize < used+c.cfg.ReadBufferSize {
		newSize = used + c.cfg.ReadBufferSize
	}

	next := c.bufferPool.Get(newSize)
	copy(next, acc[start:end])
	return next, 0, used
}

func (c *Conn) releaseOutbound(item outboundFrame) {
	if item.frame.Pooled {
		c.bufferPool.Put(item.frame.Bytes)
	}
}

func (c *Conn) releaseBatch(batch []outboundFrame) {
	for i := range batch {
		c.releaseOutbound(batch[i])
	}
}

func (c *Conn) handleAsyncPipelineError(err error) {
	if err == nil {
		return
	}

	c.reportError(err)
	if IsProtocolError(err) {
		c.initiateClose(err, true)
		return
	}
	c.initiateClose(err, false)
}

func (c *Conn) recoverLoopPanic(loopName string) {
	if r := recover(); r != nil {
		err := fmt.Errorf("%s panic: %v", loopName, r)
		c.reportError(fmt.Errorf("%w\n%s", err, debug.Stack()))
		c.initiateClose(err, true)
	}
}

func (c *Conn) storeCloseCause(err error) {
	if err == nil {
		return
	}

	c.closeCauseMu.Lock()
	defer c.closeCauseMu.Unlock()
	if c.closeCause != nil {
		return
	}
	c.closeCause = err
}

func (c *Conn) reportError(err error) {
	if err == nil {
		return
	}
	if c.cfg.OnError != nil {
		c.cfg.OnError(c, err)
	}
}

func normalizeConnConfig(cfg ConnConfig) ConnConfig {
	if cfg.ReadBufferSize <= 0 {
		cfg.ReadBufferSize = defaultReadBufferSize
	}
	if cfg.ReadAccumulatorSize < cfg.ReadBufferSize {
		cfg.ReadAccumulatorSize = cfg.ReadBufferSize << 1
		if cfg.ReadAccumulatorSize < defaultReadAccumulatorSize {
			cfg.ReadAccumulatorSize = defaultReadAccumulatorSize
		}
	}
	if cfg.MaxFrameSize <= 0 {
		cfg.MaxFrameSize = defaultMaxFrameSize
	}
	if cfg.WriteQueueCapacity <= 0 {
		cfg.WriteQueueCapacity = defaultWriteQueueCapacity
	}
	if cfg.MaxWriteBatchFrames <= 0 {
		cfg.MaxWriteBatchFrames = defaultMaxWriteBatchFrames
	}
	if cfg.MaxWriteBatchBytes <= 0 {
		cfg.MaxWriteBatchBytes = defaultMaxWriteBatchBytes
	}
	return cfg
}
