package rcon

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func sendPacket(conn net.Conn, requestID, pktType int32, payload string) error {
	payloadBytes := []byte(payload)
	length := int32(4 + 4 + len(payloadBytes) + 2)

	buf := make([]byte, 4+length)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(length))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(requestID))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(pktType))
	copy(buf[12:], payloadBytes)
	buf[len(buf)-2] = 0
	buf[len(buf)-1] = 0

	_, err := conn.Write(buf)
	return err
}

func recvPacket(conn net.Conn) (requestID, pktType int32, payload string, err error) {
	var length int32
	if err = binary.Read(conn, binary.LittleEndian, &length); err != nil {
		return
	}
	buf := make([]byte, length)
	n := 0
	for n < int(length) {
		var nn int
		nn, err = conn.Read(buf[n:])
		if err != nil {
			return
		}
		n += nn
	}
	requestID = int32(binary.LittleEndian.Uint32(buf[0:4]))
	pktType = int32(binary.LittleEndian.Uint32(buf[4:8]))
	end := len(buf) - 2
	if end < 8 {
		end = 8
	}
	payload = string(buf[8:end])
	return
}

func TestRCONAuthAndCommand(t *testing.T) {
	srv := New(Config{
		Address:  "127.0.0.1:0",
		Password: "secret",
		Executor: func(cmd string) string {
			return "executed: " + cmd
		},
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer srv.Stop()

	addr := srv.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := sendPacket(conn, 1, packetTypeAuth, "secret"); err != nil {
		t.Fatalf("send auth failed: %v", err)
	}
	reqID, _, _, err := recvPacket(conn)
	if err != nil {
		t.Fatalf("recv auth response failed: %v", err)
	}
	if reqID != 1 {
		t.Fatalf("expected auth response reqID 1, got %d", reqID)
	}

	if err := sendPacket(conn, 2, packetTypeCommand, "help"); err != nil {
		t.Fatalf("send command failed: %v", err)
	}
	reqID, _, payload, err := recvPacket(conn)
	if err != nil {
		t.Fatalf("recv command response failed: %v", err)
	}
	if reqID != 2 {
		t.Fatalf("expected command response reqID 2, got %d", reqID)
	}
	if payload != "executed: help" {
		t.Fatalf("expected 'executed: help', got %q", payload)
	}
}

func TestRCONAuthFail(t *testing.T) {
	srv := New(Config{
		Address:  "127.0.0.1:0",
		Password: "correct",
		Executor: func(cmd string) string { return "" },
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer srv.Stop()

	addr := srv.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := sendPacket(conn, 1, packetTypeAuth, "wrong"); err != nil {
		t.Fatalf("send auth failed: %v", err)
	}
	reqID, _, _, err := recvPacket(conn)
	if err != nil {
		t.Fatalf("recv auth response failed: %v", err)
	}
	if reqID != authFailID {
		t.Fatalf("expected auth fail reqID %d, got %d", authFailID, reqID)
	}
}
