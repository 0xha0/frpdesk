// Package api is the web UI's JSON API (/api/...): the admin's session, the users, each user's
// frpc.toml, and frps's own dashboard API passed through for the server and proxies pages.
package api

import (
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	frps    *http.Client
}

func New(cfg *config.Config, store *users.Store, version string) *API {
	a := &API{Cfg: cfg, Users: store, Version: version, frps: &http.Client{Timeout: 10 * time.Second}}
	if cfg.Frps.TLS {
		a.frps.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // frps's own certificate
	}
	return a
}

func (a *API) Register(r *gin.RouterGroup) {
	r.POST("/login", a.login)
	r.POST("/logout", a.logout)
	auth := r.Group("", a.requireLogin)
	auth.GET("/me", a.me)
	auth.GET("/users", a.listUsers)
	auth.POST("/users", a.addUser)
	auth.PUT("/users/:user", a.updateUser)
	auth.DELETE("/users/:user", a.removeUser)
	auth.POST("/users/:user/enable", a.setEnabled(true))
	auth.POST("/users/:user/disable", a.setEnabled(false))
	auth.GET("/users/:user/frpc.toml", a.exportUser)
	auth.GET("/frps/serverinfo", a.frpsGet("/api/serverinfo"))
	auth.GET("/frps/proxies/:type", func(c *gin.Context) { a.frpsGet("/api/proxy/" + c.Param("type"))(c) })
	auth.GET("/frps/traffic/:name", func(c *gin.Context) { a.frpsGet("/api/traffic/" + c.Param("name"))(c) })
}

func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

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
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password"})
		return
	}
	// both compared whatever the first says: no early exit to time
	userOK := equal(in.Username, a.Cfg.Server.AdminUser)
	pwdOK := equal(in.Password, a.Cfg.Server.AdminPassword)
	s := sessions.Default(c)
	if !(userOK && pwdOK) {
		s.Delete(sessionKey)
		s.Save()
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong username or password"})
		return
	}
	s.Set(sessionKey, a.Cfg.Server.AdminUser)
	s.Save()
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "saving the users' file: " + err.Error()})
	}
}

func (a *API) addUser(c *gin.Context) {
	var u users.User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := a.Users.Add(u)
	if err != nil {
		userError(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (a *API) updateUser(c *gin.Context) {
	var u users.User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := a.Users.Update(c.Param("user"), u)
	if err != nil {
		userError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (a *API) removeUser(c *gin.Context) {
	if err := a.Users.Remove(c.Param("user")); err != nil {
		userError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) setEnabled(on bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := a.Users.SetEnabled(c.Param("user"), on)
		if err != nil {
			userError(c, err)
			return
		}
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

// frpsGet passes a GET to frps's dashboard API and returns its body as it is
func (a *API) frpsGet(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := "http"
		if a.Cfg.Frps.TLS {
			scheme = "https"
		}
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s://%s:%d%s", scheme, a.Cfg.Frps.Addr, a.Cfg.Frps.Port, path), nil)
		if a.Cfg.Frps.User != "" {
			req.SetBasicAuth(a.Cfg.Frps.User, a.Cfg.Frps.Password)
		}
		res, err := a.frps.Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "frps dashboard: " + err.Error()})
			return
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		if res.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("frps dashboard: HTTP %d", res.StatusCode)})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	}
}
