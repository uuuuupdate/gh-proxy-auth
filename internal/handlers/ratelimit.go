package handlers

import (
	"context"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/dowork-shanqiu/gh-proxy-auth/internal/database"
)

// tokenBucket is a simple thread-safe token bucket rate limiter.
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64 // bytes per second
	tokens   float64
	maxBurst float64
	lastTime time.Time
}

func newTokenBucket(bytesPerSec int64) *tokenBucket {
	r := float64(bytesPerSec)
	burst := r
	if burst < float64(proxyBufSize) {
		burst = float64(proxyBufSize)
	}
	return &tokenBucket{
		rate:     r,
		tokens:   burst,
		maxBurst: burst,
		lastTime: time.Now(),
	}
}

func (tb *tokenBucket) wait(ctx context.Context, n int) error {
	for {
		tb.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(tb.lastTime).Seconds()
		if elapsed > 0 {
			tb.lastTime = now
			tb.tokens += elapsed * tb.rate
			if tb.tokens > tb.maxBurst {
				tb.tokens = tb.maxBurst
			}
		}
		need := float64(n)
		if tb.tokens >= need {
			tb.tokens -= need
			tb.mu.Unlock()
			return nil
		}
		deficit := need - tb.tokens
		tb.tokens = 0
		sleepDur := time.Duration(deficit / tb.rate * float64(time.Second))
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDur):
		}
	}
}

// Global bandwidth limiter – shared across all proxy connections.
var (
	globalLimiterMu sync.RWMutex
	globalLimiter   *tokenBucket
)

// SetGlobalLimit updates the global bandwidth limit.
// bytesPerSec <= 0 means unlimited.
func SetGlobalLimit(bytesPerSec int64) {
	globalLimiterMu.Lock()
	defer globalLimiterMu.Unlock()
	if bytesPerSec <= 0 {
		globalLimiter = nil
	} else {
		globalLimiter = newTokenBucket(bytesPerSec)
	}
}

func getGlobalLimiter() *tokenBucket {
	globalLimiterMu.RLock()
	defer globalLimiterMu.RUnlock()
	return globalLimiter
}

// InitGlobalLimiter loads the global speed limit from database settings.
func InitGlobalLimiter() {
	val := database.GetSetting("global_speed_limit")
	if val == "" {
		return
	}
	limit, err := strconv.ParseInt(val, 10, 64)
	if err != nil || limit <= 0 {
		return
	}
	SetGlobalLimit(limit)
}

// throttledReader wraps an io.Reader applying per-user and global rate limits.
type throttledReader struct {
	r    io.Reader
	user *tokenBucket // per-connection user limiter (nil = unlimited)
	ctx  context.Context
}

func (t *throttledReader) Read(p []byte) (int, error) {
	// Cap chunk size for smooth throttling
	if len(p) > proxyBufSize {
		p = p[:proxyBufSize]
	}
	n, err := t.r.Read(p)
	if n == 0 {
		return n, err
	}
	if t.user != nil {
		if e := t.user.wait(t.ctx, n); e != nil {
			return 0, e
		}
	}
	if gl := getGlobalLimiter(); gl != nil {
		if e := gl.wait(t.ctx, n); e != nil {
			return 0, e
		}
	}
	return n, err
}
