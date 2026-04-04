package network

import (
	"context"
	"net"
	"testing"
	"time"
)

type captureHandler struct {
	recv chan Packet
}

func TestListenerSetMaxConnections(t *testing.T) {
	listener, err := NewListener(ListenerConfig{Address: "127.0.0.1:0", MaxConnections: 10})
	if err != nil {
		t.Fatalf("new listener failed: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	if listener.MaxConnections() != 10 {
		t.Fatalf("expected initial max connections 10, got %d", listener.MaxConnections())
	}

	listener.SetMaxConnections(256)
	if listener.MaxConnections() != 256 {
		t.Fatalf("expected updated max connections 256, got %d", listener.MaxConnections())
	}
}

func (h *captureHandler) HandleInbound(_ Session, packet *Packet) error {
	payload := make([]byte, len(packet.Payload))
	copy(payload, packet.Payload)
	h.recv <- Packet{ID: packet.ID, Payload: payload}
	return nil
}

func (h *captureHandler) HandleOutbound(_ Session, _ *Packet) error {
	return nil
}

func TestListenerAcceptsAndProcessesDummyPacket(t *testing.T) {
	recv := make(chan Packet, 1)
	handler := &captureHandler{recv: recv}

	listener, err := NewListener(ListenerConfig{
		Address: "127.0.0.1:0",
		PipelineFactory: func(bufferPool *BufferPool, maxFrameSize int) Pipeline {
			p := NewPipeline(bufferPool, maxFrameSize)
			if addErr := p.AddLast("capture", handler); addErr != nil {
				t.Fatalf("add handler failed: %v", addErr)
			}
			return p
		},
	})
	if err != nil {
		t.Fatalf("new listener failed: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = listener.Shutdown(shutdownCtx)
	}()

	if err := listener.Start(); err != nil {
		t.Fatalf("start listener failed: %v", err)
	}

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer client.Close()

	packetBody := appendVarInt(nil, 0x01)
	packetBody = append(packetBody, 0xAB, 0xCD)
	frame := appendVarInt(nil, int32(len(packetBody)))
	frame = append(frame, packetBody...)

	if _, err := client.Write(frame); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	select {
	case packet := <-recv:
		if packet.ID != 0x01 {
			t.Fatalf("unexpected packet id: %d", packet.ID)
		}
		if len(packet.Payload) != 2 || packet.Payload[0] != 0xAB || packet.Payload[1] != 0xCD {
			t.Fatalf("unexpected payload: %v", packet.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for inbound packet")
	}
}
