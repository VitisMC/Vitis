package network

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

const maxVarIntBytes = 5

var (
	ErrMalformedVarInt  = errors.New("malformed varint")
	ErrNegativeLength   = errors.New("negative frame length")
	ErrFrameTooLarge    = errors.New("frame too large")
	ErrMalformedPacket  = errors.New("malformed packet")
	ErrHandlerDuplicate = errors.New("handler already exists")
	ErrHandlerNotFound  = errors.New("handler not found")
)

// ProtocolError marks protocol-level violations that should close the connection.
type ProtocolError struct {
	err error
}

// Error returns the protocol error string.
func (e *ProtocolError) Error() string {
	return e.err.Error()
}

// Unwrap returns the wrapped error.
func (e *ProtocolError) Unwrap() error {
	return e.err
}

// NewProtocolError wraps an error as protocol violation.
func NewProtocolError(err error) error {
	if err == nil {
		return nil
	}
	return &ProtocolError{err: err}
}

// IsProtocolError reports whether err represents a protocol violation.
func IsProtocolError(err error) bool {
	var protocolErr *ProtocolError
	return errors.As(err, &protocolErr)
}

// Packet is the normalized representation used inside the pipeline.
type Packet struct {
	ID      int32
	Payload []byte
}

// EncodedFrame stores bytes ready to be written to the socket.
type EncodedFrame struct {
	Bytes  []byte
	Pooled bool
}

// Handler is a bidirectional packet stage for inbound and outbound flow.
type Handler interface {
	HandleInbound(session Session, packet *Packet) error
	HandleOutbound(session Session, packet *Packet) error
}

// Pipeline defines a composable and extensible packet processing chain.
type Pipeline interface {
	AddLast(name string, handler Handler) error
	AddBefore(baseName string, name string, handler Handler) error
	AddAfter(baseName string, name string, handler Handler) error
	Remove(name string) error
	FireInbound(session Session, frame []byte) error
	FireOutbound(session Session, packet *Packet) (EncodedFrame, error)
}

type namedHandler struct {
	name    string
	handler Handler
}

// DefaultPipeline is the default implementation with lock-free handler reads.
type DefaultPipeline struct {
	bufferPool   *BufferPool
	maxFrameSize int

	compressionThreshold atomic.Int32

	mu       sync.Mutex
	handlers atomic.Value
}

// NewPipeline builds a pipeline with configurable max frame size.
func NewPipeline(bufferPool *BufferPool, maxFrameSize int) *DefaultPipeline {
	if maxFrameSize <= 0 {
		maxFrameSize = 2 << 20
	}

	if bufferPool == nil {
		bufferPool = NewBufferPool(nil)
	}

	p := &DefaultPipeline{
		bufferPool:   bufferPool,
		maxFrameSize: maxFrameSize,
	}
	p.compressionThreshold.Store(-1)
	p.handlers.Store([]namedHandler{})
	return p
}

// EnableCompression sets the compression threshold on the pipeline.
func (p *DefaultPipeline) EnableCompression(threshold int) {
	p.compressionThreshold.Store(int32(threshold))
}

// AddLast appends a handler at the end of the chain.
func (p *DefaultPipeline) AddLast(name string, handler Handler) error {
	if name == "" {
		return fmt.Errorf("add handler: empty name")
	}
	if handler == nil {
		return fmt.Errorf("add handler %q: nil handler", name)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.snapshotHandlers()
	if indexOfHandler(current, name) >= 0 {
		return fmt.Errorf("add handler %q: %w", name, ErrHandlerDuplicate)
	}

	next := make([]namedHandler, len(current)+1)
	copy(next, current)
	next[len(current)] = namedHandler{name: name, handler: handler}
	p.handlers.Store(next)
	return nil
}

// AddBefore inserts a handler before baseName.
func (p *DefaultPipeline) AddBefore(baseName string, name string, handler Handler) error {
	if name == "" {
		return fmt.Errorf("add before %q: empty name", baseName)
	}
	if handler == nil {
		return fmt.Errorf("add before %q: nil handler", baseName)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.snapshotHandlers()
	if indexOfHandler(current, name) >= 0 {
		return fmt.Errorf("add before %q: %w", name, ErrHandlerDuplicate)
	}

	idx := indexOfHandler(current, baseName)
	if idx < 0 {
		return fmt.Errorf("add before %q: %w", baseName, ErrHandlerNotFound)
	}

	next := make([]namedHandler, len(current)+1)
	copy(next, current[:idx])
	next[idx] = namedHandler{name: name, handler: handler}
	copy(next[idx+1:], current[idx:])
	p.handlers.Store(next)
	return nil
}

// AddAfter inserts a handler after baseName.
func (p *DefaultPipeline) AddAfter(baseName string, name string, handler Handler) error {
	if name == "" {
		return fmt.Errorf("add after %q: empty name", baseName)
	}
	if handler == nil {
		return fmt.Errorf("add after %q: nil handler", baseName)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.snapshotHandlers()
	if indexOfHandler(current, name) >= 0 {
		return fmt.Errorf("add after %q: %w", name, ErrHandlerDuplicate)
	}

	idx := indexOfHandler(current, baseName)
	if idx < 0 {
		return fmt.Errorf("add after %q: %w", baseName, ErrHandlerNotFound)
	}

	insertAt := idx + 1
	next := make([]namedHandler, len(current)+1)
	copy(next, current[:insertAt])
	next[insertAt] = namedHandler{name: name, handler: handler}
	copy(next[insertAt+1:], current[insertAt:])
	p.handlers.Store(next)
	return nil
}

// Remove removes a handler by name.
func (p *DefaultPipeline) Remove(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.snapshotHandlers()
	idx := indexOfHandler(current, name)
	if idx < 0 {
		return fmt.Errorf("remove %q: %w", name, ErrHandlerNotFound)
	}

	next := make([]namedHandler, 0, len(current)-1)
	next = append(next, current[:idx]...)
	next = append(next, current[idx+1:]...)
	p.handlers.Store(next)
	return nil
}

// FireInbound decodes a frame into packet then dispatches handlers in forward order.
func (p *DefaultPipeline) FireInbound(session Session, frame []byte) error {
	packet, err := decodePacket(frame)
	if err != nil {
		return NewProtocolError(err)
	}

	handlers := p.snapshotHandlers()
	for i := range handlers {
		if err := handlers[i].handler.HandleInbound(session, packet); err != nil {
			if IsProtocolError(err) {
				return err
			}
			return fmt.Errorf("inbound handler %q failed: %w", handlers[i].name, err)
		}
	}

	return nil
}

// FireOutbound dispatches handlers in reverse order then encodes and frames the packet.
func (p *DefaultPipeline) FireOutbound(session Session, packet *Packet) (EncodedFrame, error) {
	if packet == nil {
		return EncodedFrame{}, fmt.Errorf("encode outbound: nil packet")
	}

	handlers := p.snapshotHandlers()
	for i := len(handlers) - 1; i >= 0; i-- {
		if err := handlers[i].handler.HandleOutbound(session, packet); err != nil {
			if IsProtocolError(err) {
				return EncodedFrame{}, err
			}
			return EncodedFrame{}, fmt.Errorf("outbound handler %q failed: %w", handlers[i].name, err)
		}
	}

	packetBytes, pooledPacket, err := encodePacket(packet, p.bufferPool)
	if err != nil {
		return EncodedFrame{}, err
	}

	threshold := p.compressionThreshold.Load()
	if threshold >= 0 {
		compressed, compErr := CompressFrame(packetBytes, int(threshold))
		if pooledPacket {
			p.bufferPool.Put(packetBytes)
		}
		if compErr != nil {
			return EncodedFrame{}, compErr
		}
		return EncodedFrame{Bytes: compressed, Pooled: false}, nil
	}

	frame, err := encodeFrame(packetBytes, p.bufferPool, p.maxFrameSize)
	if pooledPacket {
		p.bufferPool.Put(packetBytes)
	}
	if err != nil {
		return EncodedFrame{}, err
	}

	return frame, nil
}

// TryDecodeFrame extracts frame metadata from buffered bytes using Minecraft VarInt length prefix.
func TryDecodeFrame(data []byte, maxFrameSize int) (frameLen int, headerLen int, complete bool, err error) {
	if maxFrameSize <= 0 {
		maxFrameSize = 2 << 20
	}

	value, consumed, ok, decodeErr := decodeVarInt(data)
	if decodeErr != nil {
		return 0, 0, false, NewProtocolError(decodeErr)
	}
	if !ok {
		return 0, 0, false, nil
	}
	if value < 0 {
		return 0, 0, false, NewProtocolError(ErrNegativeLength)
	}
	if int(value) > maxFrameSize {
		return 0, 0, false, NewProtocolError(fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, value, maxFrameSize))
	}

	total := consumed + int(value)
	if len(data) < total {
		return int(value), consumed, false, nil
	}

	return int(value), consumed, true, nil
}

func indexOfHandler(handlers []namedHandler, name string) int {
	for i := range handlers {
		if handlers[i].name == name {
			return i
		}
	}
	return -1
}

func (p *DefaultPipeline) snapshotHandlers() []namedHandler {
	return p.handlers.Load().([]namedHandler)
}

func decodePacket(frame []byte) (*Packet, error) {
	packetID, consumed, ok, err := decodeVarInt(frame)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrMalformedPacket
	}
	if packetID < 0 {
		return nil, ErrMalformedPacket
	}
	if consumed > len(frame) {
		return nil, ErrMalformedPacket
	}

	return &Packet{
		ID:      packetID,
		Payload: frame[consumed:],
	}, nil
}

func encodePacket(packet *Packet, pool *BufferPool) ([]byte, bool, error) {
	if packet.ID < 0 {
		return nil, false, NewProtocolError(ErrMalformedPacket)
	}

	idLen := varIntSize(packet.ID)
	total := idLen + len(packet.Payload)

	var buf []byte
	pooled := false
	if pool != nil {
		buf = pool.Get(total)
		pooled = true
	} else {
		buf = make([]byte, total)
	}

	offset := writeVarInt(buf, packet.ID)
	copy(buf[offset:], packet.Payload)
	return buf[:total], pooled, nil
}

func encodeFrame(packetBytes []byte, pool *BufferPool, maxFrameSize int) (EncodedFrame, error) {
	if maxFrameSize > 0 && len(packetBytes) > maxFrameSize {
		return EncodedFrame{}, NewProtocolError(fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(packetBytes), maxFrameSize))
	}

	frameLen := len(packetBytes)
	headLen := varIntSize(int32(frameLen))
	total := headLen + frameLen

	var buf []byte
	pooled := false
	if pool != nil {
		buf = pool.Get(total)
		pooled = true
	} else {
		buf = make([]byte, total)
	}

	offset := writeVarInt(buf, int32(frameLen))
	copy(buf[offset:], packetBytes)

	return EncodedFrame{Bytes: buf[:total], Pooled: pooled}, nil
}

func decodeVarInt(src []byte) (value int32, consumed int, ok bool, err error) {
	var result int32
	for i := 0; i < maxVarIntBytes; i++ {
		if i >= len(src) {
			return 0, 0, false, nil
		}

		b := src[i]
		result |= int32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return result, i + 1, true, nil
		}
	}

	if len(src) >= maxVarIntBytes {
		if src[maxVarIntBytes-1]&0x80 != 0 {
			return 0, 0, false, ErrMalformedVarInt
		}
	}

	return 0, 0, false, nil
}

func varIntSize(v int32) int {
	uv := uint32(v)
	if uv == 0 {
		return 1
	}

	size := 0
	for uv != 0 {
		size++
		uv >>= 7
	}
	return size
}

func writeVarInt(dst []byte, v int32) int {
	uv := uint32(v)
	idx := 0
	for {
		if uv&^uint32(0x7F) == 0 {
			dst[idx] = byte(uv)
			idx++
			return idx
		}

		dst[idx] = byte((uv & 0x7F) | 0x80)
		uv >>= 7
		idx++
	}
}
