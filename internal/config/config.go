// Package config reads frpdesk.toml.
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server Server `toml:"server"`
	Frps   Frps   `toml:"frps"`
	Client Client `toml:"client"`

	// UsersFile is the users' file, next to the config (server.users_file, default users.toml)
	UsersFile string `toml:"-"`
}

// Server: the two listeners and the admin's login
type Server struct {
	// frps's plugin endpoint (POST /handler): frps only, no login
	PluginAddr string `toml:"plugin_addr"`
	PluginPort int    `toml:"plugin_port"`
	// the web UI and its API: put a TLS-terminating proxy in front
	UIAddr string `toml:"ui_addr"`
	UIPort int    `toml:"ui_port"`
	// the session cookie only over HTTPS (true behind an HTTPS proxy)
	CookieSecure  bool   `toml:"cookie_secure"`
	AdminUser     string `toml:"admin_user"`
	AdminPassword string `toml:"admin_password"`
	// how long a login lasts, in hours (0: until the browser closes)
	SessionHours int    `toml:"session_hours"`
	UsersFile    string `toml:"users_file"`
}

// Frps: frps's own dashboard (webServer in frps.toml), for the server and proxies pages
type Frps struct {
	Addr     string `toml:"addr"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	TLS      bool   `toml:"tls"`
}

// Client: what the exported frpc.toml says about reaching this server
type Client struct {
	ServerAddr string `toml:"server_addr"`
	ServerPort int    `toml:"server_port"`
	// frps's auth.token (the users' own tokens are theirs, in the users' file)
	Token string `toml:"token"`
	// transport.tls.serverName for the clients: an IP where the network filters TLS by name
	TLSServerName string `toml:"tls_server_name"`
	// a few lines put at the top of every exported config (how to reach the admin, rules)
	Notes string `toml:"notes"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: Server{PluginAddr: "127.0.0.1", PluginPort: 7200, UIAddr: "127.0.0.1", UIPort: 7201, UsersFile: "users.toml"},
		Frps:   Frps{Addr: "127.0.0.1", Port: 7500},
		Client: Client{ServerPort: 7000},
	}
	meta, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if u := meta.Undecoded(); len(u) > 0 {
		return nil, fmt.Errorf("%s: unknown keys: %v", path, u)
	}
	if strings.TrimSpace(cfg.Server.AdminUser) == "" || strings.TrimSpace(cfg.Server.AdminPassword) == "" {
		return nil, errors.New("server.admin_user and server.admin_password are required: the UI has no open mode")
	}
	if cfg.Server.UIPort == cfg.Server.PluginPort && cfg.Server.UIAddr == cfg.Server.PluginAddr {
		return nil, errors.New("server.ui_port must differ from server.plugin_port: the plugin endpoint has no login")
	}
	uf := cfg.Server.UsersFile
	if !filepath.IsAbs(uf) {
		uf = filepath.Join(filepath.Dir(path), uf)
	}
	cfg.UsersFile = uf
	return cfg, nil
}
