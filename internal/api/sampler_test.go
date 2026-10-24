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
	s.KeepMin = 3
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
	// the minute ring keeps KeepMin samples
	for i := 3; i <= 6; i++ {
		s.sample(t0.Add(time.Duration(i) * 60 * time.Second))
	}
	if got = s.Samples(); len(got) != 3 || got[0].T != t0.Add(240*time.Second).Unix() {
		t.Fatalf("ring: %d samples, first at %d", len(got), got[0].T)
	}
	// the hours: every sample so far is in one hour (t0 = 1000000 is 13:46:40 UTC, so the hour
	// boundary 1000800 comes at +800 s); its bytes = sum(rate*dt), its average the mean
	hs := s.Hours()
	if len(hs) != 1 || hs[0].N != 6 || hs[0].T != 1_000_000-1_000_000%3600 {
		t.Fatalf("hours: %+v", hs)
	}
	if hs[0].InBytes != 6000+600 || hs[0].InMax != 100 {
		t.Fatalf("hour bytes/max: %+v", hs[0])
	}
	// the next hour starts a new one; the finished one is kept
	cur = serverInfo{TotalTrafficIn: 1200, CurConns: 5, ClientCounts: 2}
	s.sample(t0.Add(900 * time.Second))
	if hs = s.Hours(); len(hs) != 2 || hs[1].N != 1 || hs[1].ConnsMax != 5 || hs[0].N != 6 {
		t.Fatalf("second hour: %+v", hs)
	}
	// saved and loaded again: the minutes are old (dropped), the hours within 30 days are kept
	s.save()
	again := newSampler(nil, path)
	if n := len(again.Samples()); n != 0 {
		t.Fatalf("old minutes should be dropped on load, got %d", n)
	}
	if n := len(again.Hours()); n != 0 { // 1970-01-12 is older than 30 days too
		t.Fatalf("old hours should be dropped on load, got %d", n)
	}
}

func TestSamplerKeepsRecentHours(t *testing.T) {
	var cur serverInfo
	path := filepath.Join(t.TempDir(), "history.json")
	s := newSampler(func() (serverInfo, error) { return cur, nil }, path)
	now := time.Now().Truncate(time.Hour)
	cur = serverInfo{TotalTrafficIn: 0}
	s.sample(now.Add(-2 * time.Hour))
	cur = serverInfo{TotalTrafficIn: 3600 * 50}
	s.sample(now.Add(-1 * time.Hour)) // 50 B/s over the hour before: 180000 bytes
	cur = serverInfo{TotalTrafficIn: 3600 * 50 * 2}
	s.sample(now)
	s.save()
	again := newSampler(nil, path)
	hs := again.Hours()
	if len(hs) != 2 || hs[0].InBytes != 180000 || hs[1].InBytes != 180000 {
		t.Fatalf("recent hours after a reload: %+v", hs)
	}
}
