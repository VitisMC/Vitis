package network

import (
	"net"
	"testing"
	"time"
)

func TestIPAdmissionLimiterRateAndRefill(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newIPAdmissionLimiter(2, 2, 8, func() time.Time { return now })
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 25565}
	if !limiter.Allow(addr) || !limiter.Allow(addr) {
		t.Fatal("burst should admit two connections")
	}
	if limiter.Allow(addr) {
		t.Fatal("third immediate connection should be rate limited")
	}
	now = now.Add(500 * time.Millisecond)
	if !limiter.Allow(addr) {
		t.Fatal("one token should refill after half a second")
	}
}

func TestIPAdmissionLimiterBoundsTrackedAddresses(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newIPAdmissionLimiter(1, 1, 1, func() time.Time { return now })
	if !limiter.Allow(&net.TCPAddr{IP: net.ParseIP("203.0.113.1"), Port: 1}) {
		t.Fatal("first address should be admitted")
	}
	if limiter.Allow(&net.TCPAddr{IP: net.ParseIP("203.0.113.2"), Port: 1}) {
		t.Fatal("untracked address should be rejected when table is full")
	}
	now = now.Add(time.Minute)
	if !limiter.Allow(&net.TCPAddr{IP: net.ParseIP("203.0.113.2"), Port: 1}) {
		t.Fatal("inactive address entry was not evicted")
	}
}

func TestAdmissionAddressKeyIgnoresSourcePort(t *testing.T) {
	a := admissionAddressKey(&net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 1})
	b := admissionAddressKey(&net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 2})
	if a != b {
		t.Fatalf("address keys differ by port: %q != %q", a, b)
	}
}
