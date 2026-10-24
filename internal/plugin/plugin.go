// Package plugin answers frps's server plugin calls (httpPlugins in frps.toml, ops Login,
// NewWorkConn, NewUserConn, NewProxy, Ping): a client must name a user and that user's token
// (user, metadatas.token in frpc.toml), and may only take the ports, domains and subdomains the
// user is allowed.
package plugin

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"

	fp "github.com/fatedier/frp/pkg/plugin/server"
	"github.com/gin-gonic/gin"

	"frpdesk/internal/users"
)

type Plugin struct {
	Users *users.Store
}

func (p *Plugin) Handle(c *gin.Context) {
	var req fp.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw, _ := json.Marshal(req.Content)
	var res fp.Response
	switch req.Op {
	case "Login":
		var ct fp.LoginContent
		json.Unmarshal(raw, &ct)
		res = p.check(ct.User, ct.Metas["token"])
	case "NewProxy":
		var ct fp.NewProxyContent
		json.Unmarshal(raw, &ct)
		res = p.check(ct.User.User, ct.User.Metas["token"])
		if !res.Reject {
			res = p.checkProxy(ct)
		}
	case "Ping", "NewWorkConn", "NewUserConn":
		var ct struct {
			User fp.UserInfo `json:"user"`
		}
		json.Unmarshal(raw, &ct)
		res = p.check(ct.User.User, ct.User.Metas["token"])
	default:
		res.Unchange = true
	}
	c.JSON(http.StatusOK, res)
}

func reject(format string, a ...any) fp.Response {
	return fp.Response{Reject: true, RejectReason: fmt.Sprintf(format, a...)}
}

// check: the user exists, is on, and the token matches. No users at all lets no one in.
func (p *Plugin) check(name, token string) fp.Response {
	if p.Users.Count() == 0 {
		return reject("no users configured")
	}
	if name == "" || token == "" {
		return reject("user and metadatas.token are required")
	}
	u, ok := p.Users.Get(name)
	if !ok {
		return reject("user [%s] does not exist", name)
	}
	if !u.Enable {
		return reject("user [%s] is disabled", name)
	}
	if subtle.ConstantTimeCompare([]byte(u.Token), []byte(token)) != 1 {
		return reject("invalid token for user [%s]", name)
	}
	return fp.Response{Unchange: true}
}

func (p *Plugin) checkProxy(ct fp.NewProxyContent) fp.Response {
	u, _ := p.Users.Get(ct.User.User)
	switch ct.ProxyType {
	case "tcp", "udp":
		if !u.PortAllowed(ct.RemotePort) {
			return reject("user [%s] may not take port %d (allowed: %v)", u.Name, ct.RemotePort, u.Ports)
		}
	case "http", "https", "tcpmux":
		if len(u.Domains) > 0 {
			for _, d := range ct.CustomDomains {
				if !contains(u.Domains, d) {
					return reject("user [%s] may not take domain %s", u.Name, d)
				}
			}
		}
		if ct.ProxyType != "tcpmux" && len(u.Subdomains) > 0 && ct.SubDomain != "" && !contains(u.Subdomains, ct.SubDomain) {
			return reject("user [%s] may not take subdomain %s", u.Name, ct.SubDomain)
		}
	}
	return fp.Response{Unchange: true}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
