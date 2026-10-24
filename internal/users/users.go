// Package users keeps frps's users: a name, a token, what they may take (ports, domains,
// subdomains), on or off. The file is TOML ([users.<name>] tables), saved atomically.
package users

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

type User struct {
	Name       string   `toml:"user" json:"user"`
	Token      string   `toml:"token" json:"token"`
	Comment    string   `toml:"comment" json:"comment"`
	Ports      []string `toml:"ports" json:"ports"`           // "8080", "30000-30099"; none: any port
	Domains    []string `toml:"domains" json:"domains"`       // http/https/tcpmux custom domains; none: any
	Subdomains []string `toml:"subdomains" json:"subdomains"` // none: any
	Enable     bool     `toml:"enable" json:"enable"`
}

// the file: the old frps-panel layout kept, so its users' files load as they are. Ports there may be
// numbers; they load as strings.
type file struct {
	Users map[string]rawUser `toml:"tokens"`
}

type rawUser struct {
	User       string   `toml:"user"`
	Token      string   `toml:"token"`
	Comment    string   `toml:"comment"`
	Ports      []any    `toml:"ports"`
	Domains    []string `toml:"domains"`
	Subdomains []string `toml:"subdomains"`
	Enable     bool     `toml:"enable"`
}

var (
	nameFormat   = regexp.MustCompile(`^\w+$`)
	tokenFormat  = regexp.MustCompile(`^[\w!@#$%^&*()-]+$`)
	portSingle   = regexp.MustCompile(`^\d{1,5}$`)
	portRange    = regexp.MustCompile(`^\d{1,5}-\d{1,5}$`)
	domainFormat = regexp.MustCompile(`^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.)+[a-zA-Z]{2,}$`)
	subFormat    = regexp.MustCompile(`^[a-zA-Z0-9-]{1,63}$`)
)

var (
	ErrExists    = errors.New("user exists")
	ErrNotFound  = errors.New("no such user")
	ErrBadName   = errors.New("name: letters, digits and _ only")
	ErrBadToken  = errors.New("token: letters, digits and !@#$%^&*()-_ only")
	ErrBadPorts  = errors.New("ports: a port or a range like 30000-30099")
	ErrBadDomain = errors.New("domains: a domain name each")
	ErrBadSub    = errors.New("subdomains: letters, digits and - only")
)

type Store struct {
	path  string
	mu    sync.RWMutex
	users map[string]User
}

func Load(path string) (*Store, error) {
	s := &Store{path: path, users: map[string]User{}}
	var f file
	if _, err := toml.DecodeFile(path, &f); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return s, nil
	}
	for name, r := range f.Users {
		if r.User == "" {
			r.User = name
		}
		u := User{Name: r.User, Token: r.Token, Comment: r.Comment, Domains: r.Domains, Subdomains: r.Subdomains, Enable: r.Enable}
		for _, p := range r.Ports {
			switch v := p.(type) {
			case string:
				u.Ports = append(u.Ports, strings.TrimSpace(v))
			case int64:
				u.Ports = append(u.Ports, strconv.FormatInt(v, 10))
			case float64:
				u.Ports = append(u.Ports, strconv.Itoa(int(v)))
			}
		}
		s.users[u.Name] = u
	}
	return s, nil
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

func (s *Store) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) Get(name string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[name]
	return u, ok
}

func clean(u User) (User, error) {
	u.Name = strings.TrimSpace(u.Name)
	u.Token = strings.TrimSpace(u.Token)
	u.Comment = strings.TrimSpace(u.Comment)
	if !nameFormat.MatchString(u.Name) {
		return u, ErrBadName
	}
	if !tokenFormat.MatchString(u.Token) {
		return u, ErrBadToken
	}
	u.Ports = compact(u.Ports)
	for _, p := range u.Ports {
		if !portSingle.MatchString(p) && !portRange.MatchString(p) {
			return u, ErrBadPorts
		}
		if lo, hi, ok := parsePorts(p); !ok || lo > hi || hi > 65535 || lo < 1 {
			return u, ErrBadPorts
		}
	}
	u.Domains = compact(u.Domains)
	for _, d := range u.Domains {
		if !domainFormat.MatchString(d) {
			return u, ErrBadDomain
		}
	}
	u.Subdomains = compact(u.Subdomains)
	for _, d := range u.Subdomains {
		if !subFormat.MatchString(d) {
			return u, ErrBadSub
		}
	}
	return u, nil
}

func compact(in []string) []string {
	out := []string{}
	for _, v := range in {
		if v = strings.TrimSpace(strings.ReplaceAll(v, " ", "")); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parsePorts(spec string) (lo, hi int, ok bool) {
	a, b, found := strings.Cut(spec, "-")
	lo, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, false
	}
	if !found {
		return lo, lo, true
	}
	hi, err = strconv.Atoi(b)
	if err != nil {
		return 0, 0, false
	}
	return lo, hi, true
}

// PortAllowed: may the user take this remote port? (no ports listed: any)
func (u User) PortAllowed(port int) bool {
	if len(u.Ports) == 0 {
		return true
	}
	for _, p := range u.Ports {
		if lo, hi, ok := parsePorts(p); ok && port >= lo && port <= hi {
			return true
		}
	}
	return false
}

// FirstPort: the lowest port the user may take (0: any)
func (u User) FirstPort() int {
	best := 0
	for _, p := range u.Ports {
		if lo, _, ok := parsePorts(p); ok && (best == 0 || lo < best) {
			best = lo
		}
	}
	return best
}

func (s *Store) Add(u User) (User, error) {
	u, err := clean(u)
	if err != nil {
		return u, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[u.Name]; exists {
		return u, ErrExists
	}
	s.users[u.Name] = u
	return u, s.save()
}

// Update replaces the user's fields (the name stays)
func (s *Store) Update(name string, u User) (User, error) {
	u.Name = name
	u, err := clean(u)
	if err != nil {
		return u, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[name]; !exists {
		return u, ErrNotFound
	}
	s.users[name] = u
	return u, s.save()
}

func (s *Store) SetEnabled(name string, on bool) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, exists := s.users[name]
	if !exists {
		return u, ErrNotFound
	}
	u.Enable = on
	s.users[name] = u
	return u, s.save()
}

func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[name]; !exists {
		return ErrNotFound
	}
	delete(s.users, name)
	return s.save()
}

// save writes the file atomically (a temp file renamed over it), mode 600. Called with mu held.
func (s *Store) save() error {
	f := file{Users: map[string]rawUser{}}
	for name, u := range s.users {
		ports := make([]any, len(u.Ports))
		for i, p := range u.Ports {
			ports[i] = p
		}
		f.Users[name] = rawUser{User: u.Name, Token: u.Token, Comment: u.Comment, Ports: ports,
			Domains: u.Domains, Subdomains: u.Subdomains, Enable: u.Enable}
	}
	tmp, err := os.OpenFile(s.path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	enc := toml.NewEncoder(tmp)
	enc.Indent = "    "
	if err := enc.Encode(f); err != nil {
		tmp.Close()
		os.Remove(s.path + ".tmp")
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(s.path + ".tmp")
		return err
	}
	return os.Rename(s.path+".tmp", s.path)
}
