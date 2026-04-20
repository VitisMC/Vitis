package query

import (
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

func testInfo() *StaticServerInfo {
	return &StaticServerInfo{
		MotdValue:       "Test Server",
		GameTypeValue:   "SMP",
		MapValue:        "world",
		NumPlayersValue: 2,
		MaxPlayersValue: 20,
		HostPortValue:   25565,
		HostIPValue:     "127.0.0.1",
		PlayersValue:    []string{"Alice", "Bob"},
		PluginsValue:    "Vitis",
		VersionValue:    "1.21.4",
	}
}

func handshake(t *testing.T, conn *net.UDPConn, addr *net.UDPAddr, sessionID int32) int32 {
	t.Helper()
	pkt := make([]byte, 7)
	pkt[0] = 0xFE
	pkt[1] = 0xFD
	pkt[2] = 0x09
	binary.BigEndian.PutUint32(pkt[3:7], uint32(sessionID))

	if _, err := conn.WriteToUDP(pkt, addr); err != nil {
		t.Fatalf("handshake write failed: %v", err)
	}

	buf := make([]byte, 256)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("handshake read failed: %v", err)
	}

	if buf[0] != 0x09 {
		t.Fatalf("expected handshake type 0x09, got 0x%02x", buf[0])
	}
	respSession := int32(binary.BigEndian.Uint32(buf[1:5]))
	if respSession != sessionID {
		t.Fatalf("session mismatch: expected %d, got %d", sessionID, respSession)
	}

	tokenEnd := 5
	for tokenEnd < n && buf[tokenEnd] != 0 {
		tokenEnd++
	}
	tokenStr := string(buf[5:tokenEnd])
	token, err := strconv.Atoi(tokenStr)
	if err != nil {
		t.Fatalf("parse challenge token %q: %v", tokenStr, err)
	}
	return int32(token)
}

func TestQueryBasicStat(t *testing.T) {
	info := testInfo()
	srv := New(Config{
		Address: "127.0.0.1:0",
		Info:    info,
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer srv.Stop()

	srvAddr := srv.conn.LocalAddr().(*net.UDPAddr)
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	client, err := net.ListenUDP("udp", clientAddr)
	if err != nil {
		t.Fatalf("client listen failed: %v", err)
	}
	defer client.Close()

	sessionID := int32(42)
	token := handshake(t, client, srvAddr, sessionID)

	pkt := make([]byte, 11)
	pkt[0] = 0xFE
	pkt[1] = 0xFD
	pkt[2] = 0x00
	binary.BigEndian.PutUint32(pkt[3:7], uint32(sessionID))
	binary.BigEndian.PutUint32(pkt[7:11], uint32(token))

	if _, err := client.WriteToUDP(pkt, srvAddr); err != nil {
		t.Fatalf("stat write failed: %v", err)
	}

	buf := make([]byte, 4096)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("stat read failed: %v", err)
	}
	if n < 5 {
		t.Fatalf("response too short: %d bytes", n)
	}
	if buf[0] != 0x00 {
		t.Fatalf("expected stat type 0x00, got 0x%02x", buf[0])
	}
}

func TestQueryFullStat(t *testing.T) {
	info := testInfo()
	srv := New(Config{
		Address: "127.0.0.1:0",
		Info:    info,
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer srv.Stop()

	srvAddr := srv.conn.LocalAddr().(*net.UDPAddr)
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	client, err := net.ListenUDP("udp", clientAddr)
	if err != nil {
		t.Fatalf("client listen failed: %v", err)
	}
	defer client.Close()

	sessionID := int32(99)
	token := handshake(t, client, srvAddr, sessionID)

	pkt := make([]byte, 15)
	pkt[0] = 0xFE
	pkt[1] = 0xFD
	pkt[2] = 0x00
	binary.BigEndian.PutUint32(pkt[3:7], uint32(sessionID))
	binary.BigEndian.PutUint32(pkt[7:11], uint32(token))
	binary.BigEndian.PutUint32(pkt[11:15], 0)

	if _, err := client.WriteToUDP(pkt, srvAddr); err != nil {
		t.Fatalf("full stat write failed: %v", err)
	}

	buf := make([]byte, 4096)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("full stat read failed: %v", err)
	}
	if n < 20 {
		t.Fatalf("full stat response too short: %d bytes", n)
	}
	if buf[0] != 0x00 {
		t.Fatalf("expected stat type 0x00, got 0x%02x", buf[0])
	}

	resp := string(buf[:n])
	found := false
	for i := 0; i < len(resp)-5; i++ {
		if resp[i:i+5] == "Alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected player 'Alice' in full stat response")
	}
}

func TestQueryHandshakeOnly(t *testing.T) {
	info := testInfo()
	srv := New(Config{
		Address: "127.0.0.1:0",
		Info:    info,
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer srv.Stop()

	srvAddr := srv.conn.LocalAddr().(*net.UDPAddr)
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	client, err := net.ListenUDP("udp", clientAddr)
	if err != nil {
		t.Fatalf("client listen failed: %v", err)
	}
	defer client.Close()

	token := handshake(t, client, srvAddr, 1)
	if token == 0 {
		t.Fatal("expected non-zero challenge token")
	}
}
