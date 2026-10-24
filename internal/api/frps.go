package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"frpdesk/internal/config"
)

// frpsClient reads frps's dashboard API (webServer in frps.toml)
type frpsClient struct {
	cfg  config.Frps
	http *http.Client
}

func newFrpsClient(cfg config.Frps) *frpsClient {
	c := &frpsClient{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}}
	if cfg.TLS {
		c.http.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // frps's own certificate
	}
	return c
}

// raw: the body of a GET, as frps sent it (at most 8 MiB)
func (f *frpsClient) raw(path string) ([]byte, error) {
	scheme := "http"
	if f.cfg.TLS {
		scheme = "https"
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s:%d%s", scheme, f.cfg.Addr, f.cfg.Port, path), nil)
	if err != nil {
		return nil, err
	}
	if f.cfg.User != "" {
		req.SetBasicAuth(f.cfg.User, f.cfg.Password)
	}
	res, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return body, nil
}

func (f *frpsClient) get(path string, out any) error {
	body, err := f.raw(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// the dashboard's shapes (server/http/model/types.go in frp)
type serverInfo struct {
	TotalTrafficIn  int64 `json:"totalTrafficIn"` // today's
	TotalTrafficOut int64 `json:"totalTrafficOut"`
	CurConns        int64 `json:"curConns"`
	ClientCounts    int64 `json:"clientCounts"`
}

type proxyStats struct {
	Name string `json:"name"`
	User string `json:"user"`
}

type proxyTraffic struct {
	TrafficIn  []int64 `json:"trafficIn"` // [0] = today, [1] = yesterday …
	TrafficOut []int64 `json:"trafficOut"`
}

var proxyTypes = []string{"tcp", "udp", "http", "https", "tcpmux", "stcp", "sudp", "xtcp"}
var typeFormat = regexp.MustCompile(`^[a-z]+$`)

func frpsError(c *gin.Context, err error) {
	c.JSON(http.StatusBadGateway, gin.H{"error": "frps dashboard: " + err.Error()})
}

// passthrough hands frps's answer on as it is
func (a *API) passthrough(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := a.frps.raw(path)
		if err != nil {
			frpsError(c, err)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	}
}

func (a *API) frpsProxies(c *gin.Context) {
	t := c.Param("type")
	if !typeFormat.MatchString(t) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no such proxy type"})
		return
	}
	a.passthrough("/api/proxy/" + t)(c)
}

// dayTraffic: one proxy's last 7 days, oldest first
type dayTraffic struct {
	Name string  `json:"name"`
	User string  `json:"user"`
	Type string  `json:"type"`
	In   []int64 `json:"in"`
	Out  []int64 `json:"out"`
}

// frpsTraffic: every proxy's traffic by day (frps keeps 7), summed too. Days are the server's
// calendar days, oldest first; the last one is today so far.
func (a *API) frpsTraffic(c *gin.Context) {
	const days = 7
	var proxies []dayTraffic
	for _, t := range proxyTypes {
		var list struct {
			Proxies []proxyStats `json:"proxies"`
		}
		if err := a.frps.get("/api/proxy/"+t, &list); err != nil {
			frpsError(c, err)
			return
		}
		for _, p := range list.Proxies {
			if len(proxies) >= 1000 {
				break
			}
			var tr proxyTraffic
			if err := a.frps.get("/api/traffic/"+p.Name, &tr); err != nil {
				frpsError(c, err)
				return
			}
			proxies = append(proxies, dayTraffic{Name: p.Name, User: p.User, Type: t, In: reverse(pad(tr.TrafficIn, days)), Out: reverse(pad(tr.TrafficOut, days))})
		}
	}
	total := dayTraffic{In: make([]int64, days), Out: make([]int64, days)}
	for _, p := range proxies {
		for i := range days {
			total.In[i] += p.In[i]
			total.Out[i] += p.Out[i]
		}
	}
	sort.Slice(proxies, func(i, j int) bool { return sum(proxies[i]) > sum(proxies[j]) })
	labels := make([]string, days)
	now := time.Now()
	for i := range days {
		labels[i] = now.AddDate(0, 0, i-days+1).Format("2006-01-02")
	}
	c.JSON(http.StatusOK, gin.H{"days": labels, "total": gin.H{"in": total.In, "out": total.Out}, "proxies": proxies})
}

func pad(v []int64, n int) []int64 {
	out := make([]int64, n)
	copy(out, v)
	return out
}

func reverse(v []int64) []int64 {
	out := make([]int64, len(v))
	for i, x := range v {
		out[len(v)-1-i] = x
	}
	return out
}

func sum(d dayTraffic) int64 {
	var s int64
	for i := range d.In {
		s += d.In[i] + d.Out[i]
	}
	return s
}

// frpsHistory: the sampler's last 24 hours (connections, clients, throughput each minute)
func (a *API) frpsHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"interval": int(a.sampler.Interval.Seconds()), "samples": a.sampler.Samples()})
}
