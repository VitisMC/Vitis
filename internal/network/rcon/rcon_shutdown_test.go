package rcon

import (
	"net"
	"testing"
	"time"
)

func TestStopClosesActiveConnections(t *testing.T) {
	server := New(Config{Address: "127.0.0.1:0", Password: "Q7v#2mT!9xL@4pR$"})
	if err := server.Start(); err != nil {
		t.Fatalf("start rcon: %v", err)
	}

	client, err := net.Dial("tcp", server.listener.Addr().String())
	if err != nil {
		server.Stop()
		t.Fatalf("dial rcon: %v", err)
	}
	defer client.Close()

	deadline := time.After(time.Second)
	for {
		active := false
		server.connections.Range(func(_, _ any) bool {
			active = true
			return false
		})
		if active {
			break
		}
		select {
		case <-deadline:
			server.Stop()
			t.Fatal("connection did not reach RCON handler")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	done := make(chan struct{})
	go func() {
		server.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rcon stop waited for the read deadline instead of closing the connection")
	}
}
