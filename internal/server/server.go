// Package server runs the two listeners: frps's plugin endpoint, and the UI (the embedded page,
// its API) behind the admin's session.
package server

import (
	"context"
	"crypto/rand"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"frpdesk/internal/api"
	"frpdesk/internal/config"
	"frpdesk/internal/plugin"
	"frpdesk/internal/users"
	"frpdesk/ui"
)

const maxBody = 1 << 20 // a request body: a user, a login, a plugin call; never more

type Server struct {
	cfg     *config.Config
	api     *api.API
	plugin  *http.Server
	web     *http.Server
	cancel  context.CancelFunc
	sampled chan struct{} // closed when the sampler has stopped (and saved)
	version string
}

func bodyLimit(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)
}

func New(cfg *config.Config, store *users.Store, version string) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	p := gin.New()
	p.Use(gin.Recovery(), bodyLimit)
	p.POST("/handler", (&plugin.Plugin{Users: store}).Handle)

	// the session cookie: signed and encrypted with keys made now (a restart logs everyone out),
	// holding the admin's name only
	authKey, encKey := make([]byte, 32), make([]byte, 32)
	if _, err := rand.Read(authKey); err != nil {
		return nil, err
	}
	if _, err := rand.Read(encKey); err != nil {
		return nil, err
	}
	store2 := cookie.NewStore(authKey, encKey)
	store2.Options(sessions.Options{
		Path: "/", HttpOnly: true, Secure: cfg.Server.CookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: cfg.Server.SessionHours * 3600,
	})
	w := gin.New()
	// the client's address for the login throttle and the audit log: the proxy in front (on this
	// machine: a tunnel, nginx, caddy) says it in X-Forwarded-For; anyone else's header is ignored
	if err := w.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		return nil, err
	}
	w.Use(gin.Recovery(), bodyLimit, sessions.Sessions("frpdesk", store2))
	a := api.New(cfg, store, version)
	a.Register(w.Group("/api"))

	dist, err := fs.Sub(ui.Dist, "dist")
	if err != nil {
		return nil, err
	}
	static := http.FS(dist)
	fileServer := http.FileServer(static)
	w.NoRoute(func(c *gin.Context) {
		// only a page is served here: the API's paths and anything but GET/HEAD are a 404 (the
		// plugin endpoint lives on the other listener)
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no such endpoint"})
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("X-Frame-Options", "DENY")
		// the app is self-contained: its own scripts and styles (inline style attributes for the
		// charts' colours), data: for the icon, nothing from elsewhere
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		// a built file as it is; anything else is the app (its router takes the path)
		if f, err := static.Open(c.Request.URL.Path); err == nil {
			f.Close()
			if c.Request.URL.Path != "/" && !strings.HasSuffix(c.Request.URL.Path, "/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	timeouts := func(h http.Handler) *http.Server {
		return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
			WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	}
	return &Server{cfg: cfg, api: a, version: version, plugin: timeouts(p), web: timeouts(w)}, nil
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.sampled = make(chan struct{})
	go func() {
		defer close(s.sampled)
		s.api.RunSampler(ctx)
	}()
	errs := make(chan error, 2)
	serve := func(srv *http.Server, addr, what string) {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			errs <- err
			return
		}
		log.Printf("%s on %s", what, l.Addr())
		if err := srv.Serve(l); !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}
	go serve(s.plugin, net.JoinHostPort(s.cfg.Server.PluginAddr, strconv.Itoa(s.cfg.Server.PluginPort)), "frps plugin endpoint")
	go serve(s.web, net.JoinHostPort(s.cfg.Server.UIAddr, strconv.Itoa(s.cfg.Server.UIPort)), "web UI")
	return <-errs
}

// Shutdown stops the listeners and the sampler (which saves its history)
func (s *Server) Shutdown() {
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.plugin.Shutdown(ctx)
	s.web.Shutdown(ctx)
	if s.sampled != nil { // the sampler saves its history on the way out
		select {
		case <-s.sampled:
		case <-ctx.Done():
		}
	}
}
