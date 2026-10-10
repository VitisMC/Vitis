package network

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

type orderCaptureHandler struct {
	onPacket func(*Packet)
}

func TestGracefulCloseDrainsQueuedFrames(t *testing.T) {
	server, client := net.Pipe()
	conn := NewConn(server, nil, nil, nil, ConnConfig{WriteQueueCapacity: 64})
	conn.Start()

	const packetCount = 32
	for index := int32(0); index < packetCount; index++ {
		if err := conn.Send(&Packet{ID: index}); err != nil {
			t.Fatalf("Send(%d): %v", index, err)
		}
	}

	readDone := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(client)
		readDone <- data
	}()
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	data := <-readDone
	_ = client.Close()

	for want := int32(0); want < packetCount; want++ {
		frameLength, headerLength, complete, err := TryDecodeFrame(data, defaultMaxFrameSize)
		if err != nil || !complete {
			t.Fatalf("frame %d: complete=%v err=%v", want, complete, err)
		}
		packetID, _, ok, err := decodeVarInt(data[headerLength : headerLength+frameLength])
		if err != nil || !ok || packetID != want {
			t.Fatalf("frame %d packet id=%d ok=%v err=%v", want, packetID, ok, err)
		}
		data = data[headerLength+frameLength:]
	}
	if len(data) != 0 {
		t.Fatalf("unexpected trailing bytes: %d", len(data))
	}
}

func (h *orderCaptureHandler) HandleInbound(_ Session, packet *Packet) error {
	h.onPacket(packet)
	return nil
}

func (h *orderCaptureHandler) HandleOutbound(_ Session, _ *Packet) error { return nil }

type xorStream struct{}

func (xorStream) XORKeyStream(dst, src []byte) {
	for i := range src {
		dst[i] = src[i] ^ 0x80
	}
}

func TestConnectionPreservesInboundOrder(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	received := make(chan int32, 10_000)
	pipeline := NewPipeline(nil, 0)
	if err := pipeline.AddLast("capture", &orderCaptureHandler{onPacket: func(packet *Packet) {
		value, _, ok, err := decodeVarInt(packet.Payload)
		if err != nil || !ok {
			t.Errorf("decode sequence: %v", err)
			return
		}
		received <- value
	}}); err != nil {
		t.Fatal(err)
	}

	conn := NewConn(server, pipeline, nil, nil, ConnConfig{})
	conn.Start()
	defer conn.ForceClose(nil)

	frames := make([]byte, 0, 40_000)
	for i := int32(0); i < 10_000; i++ {
		body := appendVarInt(nil, 1)
		body = appendVarInt(body, i)
		frames = appendVarInt(frames, int32(len(body)))
		frames = append(frames, body...)
	}
	writeDone := make(chan error, 1)
	go func() { writeDone <- writeAll(client, frames) }()

	for want := int32(0); want < 10_000; want++ {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("packet order mismatch: got %d want %d", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out at packet %d", want)
		}
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
}

func TestConnectionDecryptsBufferedBytesAfterTransition(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	received := make(chan int32, 2)
	pipeline := NewPipeline(nil, 0)
	var conn *Conn
	if err := pipeline.AddLast("capture", &orderCaptureHandler{onPacket: func(packet *Packet) {
		received <- packet.ID
		if packet.ID == 0 {
			conn.EnableEncryption(xorStream{}, xorStream{})
		}
	}}); err != nil {
		t.Fatal(err)
	}
	conn = NewConn(server, pipeline, nil, nil, ConnConfig{})
	conn.Start()
	defer conn.ForceClose(nil)

	combined := []byte{1, 0, 1 ^ 0x80, 1 ^ 0x80}
	go func() { _ = writeAll(client, combined) }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, want := range []int32{0, 1} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("packet id: got %d want %d", got, want)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for encrypted packet")
		}
	}
}
