package api

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// A Sample: frps's gauges at one moment, and the throughput since the sample before
type Sample struct {
	T       int64   `json:"t"` // unix seconds
	Conns   int64   `json:"conns"`
	Clients int64   `json:"clients"`
	In      float64 `json:"in"`  // bytes per second, over the interval before T
	Out     float64 `json:"out"` // bytes per second
}

// Sampler reads frps's serverinfo every Interval and keeps the last Keep samples (frps itself keeps
// traffic by day only, so anything finer is sampled here). The ring is saved to a file now and then
// and at shutdown, so a restart keeps the day.
type Sampler struct {
	Interval time.Duration
	Keep     int
	fetch    func() (info serverInfo, err error)
	path     string

	mu      sync.Mutex
	samples []Sample
	primed  bool
	last    serverInfo
	lastT   time.Time
	lastErr time.Time
}

func newSampler(fetch func() (serverInfo, error), path string) *Sampler {
	s := &Sampler{Interval: time.Minute, Keep: 24 * 60, fetch: fetch, path: path}
	s.load()
	return s
}

func (s *Sampler) Samples() []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Sample, len(s.samples))
	copy(out, s.samples)
	return out
}

// Run samples until ctx ends, saving every 10 samples and at the end
func (s *Sampler) Run(ctx context.Context) {
	s.sample(time.Now())
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			s.save()
			return
		case now := <-t.C:
			if s.sample(now) {
				if n++; n%10 == 0 {
					s.save()
				}
			}
		}
	}
}

// sample: one reading; the first after a start only primes the throughput (it needs a delta)
func (s *Sampler) sample(now time.Time) bool {
	info, err := s.fetch()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if now.Sub(s.lastErr) > 10*time.Minute { // not a line a minute while frps is down
			log.Printf("sampler: frps dashboard: %v", err)
			s.lastErr = now
		}
		s.primed = false
		return false
	}
	if s.primed {
		dt := now.Sub(s.lastT).Seconds()
		if dt > 0 {
			s.samples = append(s.samples, Sample{T: now.Unix(), Conns: info.CurConns, Clients: info.ClientCounts,
				In: delta(info.TotalTrafficIn, s.last.TotalTrafficIn) / dt, Out: delta(info.TotalTrafficOut, s.last.TotalTrafficOut) / dt})
			if len(s.samples) > s.Keep {
				s.samples = s.samples[len(s.samples)-s.Keep:]
			}
		}
	}
	s.last, s.lastT, s.primed = info, now, true
	return s.primed
}

// delta of a "today" counter: at midnight frps starts it again from zero, so a drop means a new
// day and the new value is what came since midnight (the part of the interval before it is lost)
func delta(now, before int64) float64 {
	if now < before {
		return float64(now)
	}
	return float64(now - before)
}

func (s *Sampler) load() {
	if s.path == "" {
		return
	}
	var f struct {
		Samples []Sample `json:"samples"`
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	if json.Unmarshal(data, &f) != nil {
		return
	}
	cut := time.Now().Add(-time.Duration(s.Keep) * s.Interval).Unix()
	for _, x := range f.Samples {
		if x.T >= cut {
			s.samples = append(s.samples, x)
		}
	}
}

func (s *Sampler) save() {
	if s.path == "" {
		return
	}
	data, err := json.Marshal(struct {
		Samples []Sample `json:"samples"`
	}{s.Samples()})
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("sampler: can't save %s: %v", s.path, err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		log.Printf("sampler: can't save %s: %v", s.path, err)
	}
}
