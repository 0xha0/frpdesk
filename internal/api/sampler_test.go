package api

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSampler(t *testing.T) {
	var cur serverInfo
	path := filepath.Join(t.TempDir(), "history.json")
	s := newSampler(func() (serverInfo, error) { return cur, nil }, path)
	s.Keep = 3
	t0 := time.Unix(1_000_000, 0)

	cur = serverInfo{TotalTrafficIn: 1000, TotalTrafficOut: 10, CurConns: 2, ClientCounts: 1}
	if s.sample(t0); len(s.Samples()) != 0 {
		t.Fatalf("the first sample should only prime, got %d", len(s.Samples()))
	}
	cur = serverInfo{TotalTrafficIn: 7000, TotalTrafficOut: 70, CurConns: 3, ClientCounts: 1}
	s.sample(t0.Add(60 * time.Second))
	got := s.Samples()
	if len(got) != 1 || got[0].In != 100 || got[0].Out != 1 || got[0].Conns != 3 {
		t.Fatalf("after 60 s: %+v", got)
	}
	// midnight: the today counter starts again; the new value counts as the delta
	cur = serverInfo{TotalTrafficIn: 600, TotalTrafficOut: 0, CurConns: 1, ClientCounts: 1}
	s.sample(t0.Add(120 * time.Second))
	if got = s.Samples(); got[1].In != 10 || got[1].Out != 0 {
		t.Fatalf("after the rollover: %+v", got[1])
	}
	// the ring keeps Keep samples
	for i := 3; i <= 6; i++ {
		s.sample(t0.Add(time.Duration(i) * 60 * time.Second))
	}
	if got = s.Samples(); len(got) != 3 || got[0].T != t0.Add(240*time.Second).Unix() {
		t.Fatalf("ring: %d samples, first at %d", len(got), got[0].T)
	}
	// saved and loaded again (only samples within Keep*Interval of now survive: these are old)
	s.save()
	again := newSampler(nil, path)
	if n := len(again.Samples()); n != 0 {
		t.Fatalf("old samples should be dropped on load, got %d", n)
	}
}
