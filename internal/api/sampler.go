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

// An Hour: one hour of samples folded together: the averages, the peaks, and the bytes that
// went through (so days can be summed without frps, which forgets its counters on a restart)
type Hour struct {
	T          int64   `json:"t"` // the hour's start, unix seconds
	N          int     `json:"n"` // samples in it
	Conns      float64 `json:"conns"`
	ConnsMax   int64   `json:"connsMax"`
	Clients    float64 `json:"clients"`
	ClientsMax int64   `json:"clientsMax"`
	In         float64 `json:"in"` // bytes per second, averaged
	InMax      float64 `json:"inMax"`
	Out        float64 `json:"out"`
	OutMax     float64 `json:"outMax"`
	InBytes    int64   `json:"inBytes"`
	OutBytes   int64   `json:"outBytes"`
}

func (h *Hour) add(s Sample, dt float64) {
	n := float64(h.N)
	h.Conns = (h.Conns*n + float64(s.Conns)) / (n + 1)
	h.Clients = (h.Clients*n + float64(s.Clients)) / (n + 1)
	h.In = (h.In*n + s.In) / (n + 1)
	h.Out = (h.Out*n + s.Out) / (n + 1)
	h.ConnsMax = max(h.ConnsMax, s.Conns)
	h.ClientsMax = max(h.ClientsMax, s.Clients)
	h.InMax = max(h.InMax, s.In)
	h.OutMax = max(h.OutMax, s.Out)
	h.InBytes += int64(s.In * dt)
	h.OutBytes += int64(s.Out * dt)
	h.N++
}

// Sampler reads frps's serverinfo every Interval. It keeps a day of samples (one a minute) and 30
// days of hours (frps itself keeps traffic by day only, for 7 days, in memory). The rings are
// saved to a file now and then and at shutdown, so a restart keeps them.
type Sampler struct {
	Interval time.Duration
	KeepMin  int // minute samples kept: a day
	KeepHour int // hours kept: 30 days
	fetch    func() (info serverInfo, err error)
	path     string

	mu      sync.Mutex
	minutes []Sample
	hours   []Hour
	current *Hour // the hour being filled
	primed  bool
	last    serverInfo
	lastT   time.Time
	lastErr time.Time
}

func newSampler(fetch func() (serverInfo, error), path string) *Sampler {
	s := &Sampler{Interval: time.Minute, KeepMin: 24 * 60, KeepHour: 30 * 24, fetch: fetch, path: path}
	s.load()
	return s
}

func (s *Sampler) Samples() []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Sample, len(s.minutes))
	copy(out, s.minutes)
	return out
}

// Hours: the finished hours and the one being filled (so the last point is never missing)
func (s *Sampler) Hours() []Hour {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Hour, 0, len(s.hours)+1)
	out = append(out, s.hours...)
	if s.current != nil && s.current.N > 0 {
		out = append(out, *s.current)
	}
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
			sm := Sample{T: now.Unix(), Conns: info.CurConns, Clients: info.ClientCounts,
				In: delta(info.TotalTrafficIn, s.last.TotalTrafficIn) / dt, Out: delta(info.TotalTrafficOut, s.last.TotalTrafficOut) / dt}
			s.minutes = append(s.minutes, sm)
			if len(s.minutes) > s.KeepMin {
				s.minutes = s.minutes[len(s.minutes)-s.KeepMin:]
			}
			s.fold(sm, dt)
		}
	}
	s.last, s.lastT, s.primed = info, now, true
	return s.primed
}

// fold a sample into its hour (called with mu held)
func (s *Sampler) fold(sm Sample, dt float64) {
	start := sm.T - sm.T%3600
	if s.current != nil && s.current.T != start {
		s.hours = append(s.hours, *s.current)
		if len(s.hours) > s.KeepHour {
			s.hours = s.hours[len(s.hours)-s.KeepHour:]
		}
		s.current = nil
	}
	if s.current == nil {
		s.current = &Hour{T: start}
	}
	s.current.add(sm, dt)
}

// delta of a "today" counter: at midnight frps starts it again from zero (and after a restart), so
// a drop means the new value is what came since then (the part before it is lost)
func delta(now, before int64) float64 {
	if now < before {
		return float64(now)
	}
	return float64(now - before)
}

type historyFile struct {
	Samples []Sample `json:"samples"`
	Hours   []Hour   `json:"hours"`
	Current *Hour    `json:"current,omitempty"`
}

func (s *Sampler) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var f historyFile
	if json.Unmarshal(data, &f) != nil {
		return
	}
	now := time.Now()
	cutMin := now.Add(-time.Duration(s.KeepMin) * s.Interval).Unix()
	for _, x := range f.Samples {
		if x.T >= cutMin {
			s.minutes = append(s.minutes, x)
		}
	}
	cutHour := now.Add(-time.Duration(s.KeepHour) * time.Hour).Unix()
	for _, h := range f.Hours {
		if h.T >= cutHour {
			s.hours = append(s.hours, h)
		}
	}
	if f.Current != nil && f.Current.T >= cutHour {
		s.current = f.Current
	}
}

func (s *Sampler) save() {
	if s.path == "" {
		return
	}
	s.mu.Lock()
	f := historyFile{Samples: append([]Sample(nil), s.minutes...), Hours: append([]Hour(nil), s.hours...)}
	if s.current != nil {
		c := *s.current
		f.Current = &c
	}
	s.mu.Unlock()
	data, err := json.Marshal(f)
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
