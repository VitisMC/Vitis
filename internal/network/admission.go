package network

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

const (
	defaultAdmissionRate  = 10.0
	defaultAdmissionBurst = 20
	defaultMaxTrackedIPs  = 4096
)

type admissionBucket struct {
	tokens float64
	at     time.Time
}

// ipAdmissionLimiter applies a bounded token bucket before a connection gets a
// session or an outbound queue.
type ipAdmissionLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	maxIPs  int
	now     func() time.Time
	buckets map[string]admissionBucket
}

func newIPAdmissionLimiter(rate float64, burst, maxIPs int, now func() time.Time) *ipAdmissionLimiter {
	if rate <= 0 {
		rate = defaultAdmissionRate
	}
	if burst <= 0 {
		burst = defaultAdmissionBurst
	}
	if maxIPs <= 0 {
		maxIPs = defaultMaxTrackedIPs
	}
	if now == nil {
		now = time.Now
	}
	return &ipAdmissionLimiter{
		rate:    rate,
		burst:   float64(burst),
		maxIPs:  maxIPs,
		now:     now,
		buckets: make(map[string]admissionBucket),
	}
}

// Allow consumes one admission token for addr. New addresses are rejected when
// the bounded tracking table is full, preventing an address-spray memory leak.
func (l *ipAdmissionLimiter) Allow(addr net.Addr) bool {
	if l == nil {
		return false
	}
	key := admissionAddressKey(addr)
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, exists := l.buckets[key]
	if !exists {
		if len(l.buckets) >= l.maxIPs {
			l.evictInactive(now)
			if len(l.buckets) >= l.maxIPs {
				return false
			}
		}
		bucket = admissionBucket{tokens: l.burst, at: now}
	}
	if elapsed := now.Sub(bucket.at); elapsed > 0 {
		bucket.tokens += elapsed.Seconds() * l.rate
		if bucket.tokens > l.burst {
			bucket.tokens = l.burst
		}
	}
	bucket.at = now
	if bucket.tokens < 1 {
		l.buckets[key] = bucket
		return false
	}
	bucket.tokens--
	l.buckets[key] = bucket
	return true
}

func (l *ipAdmissionLimiter) evictInactive(now time.Time) {
	// Keep a bucket long enough to preserve the configured burst semantics, but
	// never let one-off address traffic permanently exhaust the bounded table.
	inactiveAfter := time.Minute
	refillSeconds := l.burst / l.rate
	if refillSeconds > 60 {
		if refillSeconds >= float64((24*time.Hour)/time.Second) {
			inactiveAfter = 24 * time.Hour
		} else {
			inactiveAfter = time.Duration(refillSeconds * float64(time.Second))
		}
	}
	for key, bucket := range l.buckets {
		if now.Sub(bucket.at) >= inactiveAfter {
			delete(l.buckets, key)
		}
	}
}

func admissionAddressKey(addr net.Addr) string {
	if addr == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	if parsed, err := netip.ParseAddr(host); err == nil {
		return parsed.Unmap().String()
	}
	return host
}
