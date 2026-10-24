package api

import (
	"sync"
	"time"
)

// loginLimiter slows a password guesser down: after 5 failures from one address the next try waits
// a minute, then two, four … up to fifteen; a success clears the address. In memory, per address.
type loginLimiter struct {
	mu    sync.Mutex
	fails map[string]*failRecord
}

type failRecord struct {
	count int
	until time.Time
	seen  time.Time
}

const (
	limiterFree  = 5
	limiterBase  = time.Minute
	limiterMax   = 15 * time.Minute
	limiterReset = time.Hour
)

func newLoginLimiter() *loginLimiter { return &loginLimiter{fails: map[string]*failRecord{}} }

// blocked: how long the address still has to wait, if it does
func (l *loginLimiter) blocked(addr string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	if r, ok := l.fails[addr]; ok && r.until.After(now) {
		return r.until.Sub(now)
	}
	return 0
}

func (l *loginLimiter) fail(addr string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.fails[addr]
	if r == nil {
		r = &failRecord{}
		l.fails[addr] = r
	}
	r.count++
	r.seen = now
	if r.count >= limiterFree {
		wait := limiterBase << (r.count - limiterFree)
		if wait > limiterMax || wait <= 0 {
			wait = limiterMax
		}
		r.until = now.Add(wait)
	}
}

func (l *loginLimiter) success(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, addr)
}

// prune forgets addresses quiet for an hour (called with mu held)
func (l *loginLimiter) prune(now time.Time) {
	for addr, r := range l.fails {
		if now.Sub(r.seen) > limiterReset {
			delete(l.fails, addr)
		}
	}
}
