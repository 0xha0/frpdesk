package api

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	l := newLoginLimiter()
	now := time.Unix(1_000_000, 0)
	for i := 0; i < limiterFree-1; i++ {
		l.fail("a", now)
	}
	if d := l.blocked("a", now); d != 0 {
		t.Fatalf("blocked after %d failures: %v", limiterFree-1, d)
	}
	l.fail("a", now)
	if d := l.blocked("a", now); d != limiterBase {
		t.Fatalf("after %d failures want %v, got %v", limiterFree, limiterBase, d)
	}
	l.fail("a", now)
	if d := l.blocked("a", now); d != 2*limiterBase {
		t.Fatalf("doubling: got %v", d)
	}
	for i := 0; i < 10; i++ {
		l.fail("a", now)
	}
	if d := l.blocked("a", now); d != limiterMax {
		t.Fatalf("cap: got %v", d)
	}
	if d := l.blocked("b", now); d != 0 {
		t.Fatal("another address blocked")
	}
	l.success("a")
	if d := l.blocked("a", now); d != 0 {
		t.Fatal("not cleared by a success")
	}
	l.fail("c", now)
	if _, ok := l.fails["c"]; !ok {
		t.Fatal("c missing")
	}
	l.blocked("c", now.Add(2*limiterReset))
	if _, ok := l.fails["c"]; ok {
		t.Fatal("c not pruned after an hour")
	}
}
