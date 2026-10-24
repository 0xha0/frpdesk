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

type Server struct {
	cfg     *config.Config
	plugin  *http.Server
	web     *http.Server
	version string
}

func New(cfg *config.Config, store *users.Store, version string) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	p := gin.New()
	p.Use(gin.Recovery())
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
	w.Use(gin.Recovery(), sessions.Sessions("frpdesk", store2))
	api.New(cfg, store, version).Register(w.Group("/api"))

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

	return &Server{cfg: cfg, version: version,
		plugin: &http.Server{Handler: p, ReadHeaderTimeout: 10 * time.Second},
		web:    &http.Server{Handler: w, ReadHeaderTimeout: 10 * time.Second}}, nil
}

func (s *Server) Run() error {
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

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.plugin.Shutdown(ctx)
	s.web.Shutdown(ctx)
}
