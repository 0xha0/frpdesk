// Package api is the web UI's JSON API (/api/...): the admin's session, the users, each user's
// frpc.toml, and frps's own dashboard API passed through (and summed, and sampled) for the server
// and proxies pages.
package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"frpdesk/internal/config"
	"frpdesk/internal/users"
)

const sessionKey = "admin"

type API struct {
	Cfg     *config.Config
	Users   *users.Store
	Version string
	frps    *frpsClient
	sampler *Sampler
	limiter *loginLimiter
}

func New(cfg *config.Config, store *users.Store, version string) *API {
	a := &API{Cfg: cfg, Users: store, Version: version, frps: newFrpsClient(cfg.Frps), limiter: newLoginLimiter()}
	a.sampler = newSampler(func() (serverInfo, error) {
		var info serverInfo
		err := a.frps.get("/api/serverinfo", &info)
		return info, err
	}, filepath.Join(filepath.Dir(cfg.UsersFile), "history.json"))
	return a
}

// RunSampler samples frps until ctx ends (call it in a goroutine)
func (a *API) RunSampler(ctx context.Context) { a.sampler.Run(ctx) }

func (a *API) Register(r *gin.RouterGroup) {
	r.Use(noCrossSite)
	r.POST("/login", a.login)
	r.POST("/logout", a.logout)
	auth := r.Group("", a.requireLogin)
	auth.GET("/me", a.me)
	auth.GET("/users", a.listUsers)
	auth.POST("/users", a.addUser)
	u := auth.Group("/users/:user", validName)
	u.PUT("", a.updateUser)
	u.DELETE("", a.removeUser)
	u.POST("/enable", a.setEnabled(true))
	u.POST("/disable", a.setEnabled(false))
	u.GET("/frpc.toml", a.exportUser)
	auth.GET("/frps/serverinfo", a.passthrough("/api/serverinfo"))
	auth.GET("/frps/clients", a.passthrough("/api/clients"))
	auth.GET("/frps/proxies/:type", a.frpsProxies)
	auth.GET("/frps/traffic", a.frpsTraffic)
	auth.GET("/frps/history", a.frpsHistory)
}

func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// noCrossSite: a request that changes something must come from the app's own JavaScript (the
// header fetch sets, which a form or a link from another site can't), and never cross-site
func noCrossSite(c *gin.Context) {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return
	}
	if c.GetHeader("X-Requested-With") == "" || strings.EqualFold(c.GetHeader("Sec-Fetch-Site"), "cross-site") {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "the request must come from the app"})
	}
}

func validName(c *gin.Context) {
	if !users.ValidName(c.Param("user")) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": users.ErrBadName.Error()})
	}
}

func (a *API) loggedIn(c *gin.Context) bool {
	v, _ := sessions.Default(c).Get(sessionKey).(string)
	return v != "" && equal(v, a.Cfg.Server.AdminUser)
}

func (a *API) requireLogin(c *gin.Context) {
	if !a.loggedIn(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not logged in"})
	}
}

func (a *API) login(c *gin.Context) {
	addr := c.ClientIP()
	now := time.Now()
	if wait := a.limiter.blocked(addr, now); wait > 0 {
		c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": fmt.Sprintf("too many failed logins: wait %d s", int(wait.Seconds())+1)})
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || len(in.Username) > 256 || len(in.Password) > 1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password"})
		return
	}
	// both compared whatever the first says: no early exit to time
	userOK := equal(in.Username, a.Cfg.Server.AdminUser)
	pwdOK := equal(in.Password, a.Cfg.Server.AdminPassword)
	s := sessions.Default(c)
	if !(userOK && pwdOK) {
		a.limiter.fail(addr, now)
		log.Printf("audit: failed login for %q from %s", in.Username, addr)
		s.Delete(sessionKey)
		s.Save()
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong username or password"})
		return
	}
	a.limiter.success(addr)
	log.Printf("audit: login by %s from %s", a.Cfg.Server.AdminUser, addr)
	s.Set(sessionKey, a.Cfg.Server.AdminUser)
	if err := s.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "can't start the session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": a.Cfg.Server.AdminUser})
}

func (a *API) logout(c *gin.Context) {
	s := sessions.Default(c)
	s.Clear()
	s.Options(sessions.Options{Path: "/", MaxAge: -1})
	s.Save()
	c.Status(http.StatusNoContent)
}

func (a *API) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user":    a.Cfg.Server.AdminUser,
		"version": a.Version,
		"client":  gin.H{"server_addr": a.Cfg.Client.ServerAddr, "server_port": a.Cfg.Client.ServerPort, "tls_server_name": a.Cfg.Client.TLSServerName},
	})
}

func (a *API) listUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": a.Users.List()})
}

func userError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, users.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, users.ErrExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, users.ErrBadName), errors.Is(err, users.ErrBadToken), errors.Is(err, users.ErrBadPorts),
		errors.Is(err, users.ErrBadDomain), errors.Is(err, users.ErrBadSub):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("users file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "saving the users' file failed"})
	}
}

func (a *API) addUser(c *gin.Context) {
	var u users.User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request body"})
		return
	}
	u, err := a.Users.Add(u)
	if err != nil {
		userError(c, err)
		return
	}
	log.Printf("audit: user %s added (ports %v) from %s", u.Name, u.Ports, c.ClientIP())
	c.JSON(http.StatusCreated, u)
}

func (a *API) updateUser(c *gin.Context) {
	var u users.User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request body"})
		return
	}
	u, err := a.Users.Update(c.Param("user"), u)
	if err != nil {
		userError(c, err)
		return
	}
	log.Printf("audit: user %s updated (ports %v, enabled %v) from %s", u.Name, u.Ports, u.Enable, c.ClientIP())
	c.JSON(http.StatusOK, u)
}

func (a *API) removeUser(c *gin.Context) {
	if err := a.Users.Remove(c.Param("user")); err != nil {
		userError(c, err)
		return
	}
	log.Printf("audit: user %s removed from %s", c.Param("user"), c.ClientIP())
	c.Status(http.StatusNoContent)
}

func (a *API) setEnabled(on bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := a.Users.SetEnabled(c.Param("user"), on)
		if err != nil {
			userError(c, err)
			return
		}
		log.Printf("audit: user %s enabled=%v from %s", u.Name, on, c.ClientIP())
		c.JSON(http.StatusOK, u)
	}
}

// exportUser: a complete frpc.toml for the user: this server, their login, one example proxy on
// their first port. A download (text/plain).
func (a *API) exportUser(c *gin.Context) {
	u, ok := a.Users.Get(c.Param("user"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such user"})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="frpc-%s.toml"`, u.Name))
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, a.ClientConfig(u))
}

func (a *API) ClientConfig(u users.User) string {
	cl := a.Cfg.Client
	var b strings.Builder
	fmt.Fprintf(&b, "# frpc.toml for user %s on %s (frpdesk %s, %s)\n", u.Name, cl.ServerAddr, a.Version, time.Now().Format("2006-01-02"))
	for _, line := range strings.Split(strings.TrimSpace(cl.Notes), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			fmt.Fprintf(&b, "# %s\n", line)
		}
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "serverAddr = %q\n", cl.ServerAddr)
	fmt.Fprintf(&b, "serverPort = %d\n", cl.ServerPort)
	if cl.Token != "" {
		fmt.Fprintf(&b, "auth.token = %q\n", cl.Token)
	} else {
		b.WriteString("auth.token = \"<the server's token: ask the admin>\"\n")
	}
	fmt.Fprintf(&b, "user = %q\n", u.Name)
	fmt.Fprintf(&b, "metadatas.token = %q\n", u.Token)
	b.WriteString("loginFailExit = false   # keep retrying (the network may be down at boot)\n")
	b.WriteString("transport.tls.enable = true\n")
	if cl.TLSServerName != "" {
		fmt.Fprintf(&b, "transport.tls.serverName = %q   # keep: the server's network needs it\n", cl.TLSServerName)
	}
	b.WriteString("\n")
	if len(u.Ports) > 0 {
		fmt.Fprintf(&b, "# your ports: %s (one remotePort per proxy)\n", strings.Join(u.Ports, ", "))
	} else {
		b.WriteString("# your ports: any the server allows\n")
	}
	if len(u.Domains) > 0 {
		fmt.Fprintf(&b, "# your domains (http/https proxies): %s\n", strings.Join(u.Domains, ", "))
	}
	if len(u.Subdomains) > 0 {
		fmt.Fprintf(&b, "# your subdomains: %s\n", strings.Join(u.Subdomains, ", "))
	}
	port := u.FirstPort()
	if port == 0 {
		port = 30000
	}
	fmt.Fprintf(&b, "\n# an example: ssh on this machine, reachable at %s:%d\n", cl.ServerAddr, port)
	b.WriteString("[[proxies]]\nname = \"ssh\"\ntype = \"tcp\"\nlocalIP = \"127.0.0.1\"\nlocalPort = 22\n")
	fmt.Fprintf(&b, "remotePort = %d\n", port)
	return b.String()
}
