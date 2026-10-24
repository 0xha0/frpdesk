package users

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidation(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "users.toml"), users: map[string]User{}}
	cases := []struct {
		u    User
		want error
	}{
		{User{Name: "bad name", Token: "token-12"}, ErrBadName},
		{User{Name: "ok", Token: "a b-token"}, ErrBadToken},
		{User{Name: "ok", Token: "short7!"}, ErrBadToken},
		{User{Name: "ok", Token: "token-12", Ports: []string{"99999"}}, ErrBadPorts},
		{User{Name: "ok", Token: "token-12", Ports: []string{"3000-2000"}}, ErrBadPorts},
		{User{Name: "ok", Token: "token-12", Ports: []string{"x"}}, ErrBadPorts},
		{User{Name: "ok", Token: "token-12", Domains: []string{"not a domain"}}, ErrBadDomain},
		{User{Name: "ok", Token: "token-12", Subdomains: []string{"a.b"}}, ErrBadSub},
		{User{Name: "ok", Token: "token-1_2", Ports: []string{" 30000 - 30099 ", "", "8080"}}, nil},
	}
	for _, c := range cases {
		_, err := s.Add(c.u)
		if !errors.Is(err, c.want) {
			t.Errorf("Add(%+v): got %v, want %v", c.u, err, c.want)
		}
	}
	if _, err := s.Add(User{Name: "ok", Token: "token-12"}); !errors.Is(err, ErrExists) {
		t.Errorf("a second ok: got %v, want ErrExists", err)
	}
	u, _ := s.Get("ok")
	if len(u.Ports) != 2 || u.Ports[0] != "30000-30099" || u.Ports[1] != "8080" {
		t.Errorf("ports not cleaned: %v", u.Ports)
	}
}

func TestPorts(t *testing.T) {
	u := User{Ports: []string{"30000-30099", "8080"}}
	for port, want := range map[int]bool{30000: true, 30099: true, 8080: true, 30100: false, 22: false} {
		if u.PortAllowed(port) != want {
			t.Errorf("PortAllowed(%d) = %v, want %v", port, !want, want)
		}
	}
	if u.FirstPort() != 8080 {
		t.Errorf("FirstPort = %d, want 8080", u.FirstPort())
	}
	if (User{}).PortAllowed(1) != true {
		t.Error("no ports listed should allow any")
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.toml")
	s, err := Load(path) // missing: empty
	if err != nil || s.Count() != 0 {
		t.Fatalf("Load(missing) = %d users, %v", s.Count(), err)
	}
	if _, err := s.Add(User{Name: "alice", Token: "token-t1", Ports: []string{"30000-30099"}, Enable: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetEnabled("alice", false); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %o, want 600", fi.Mode().Perm())
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := again.Get("alice")
	if !ok || u.Token != "token-t1" || u.Enable || len(u.Ports) != 1 {
		t.Errorf("reloaded: %+v", u)
	}
	if err := again.Remove("alice"); err != nil || again.Count() != 0 {
		t.Errorf("Remove: %v, %d left", err, again.Count())
	}
	if err := again.Remove("alice"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Remove again: %v", err)
	}
}

// frps-panel's file, with numeric ports, loads as it is
func TestOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frps-tokens.toml")
	os.WriteFile(path, []byte("[tokens]\n  [tokens.bob]\n    user = \"bob\"\n    token = \"xxxxxxxx\"\n    comment = \"\"\n    ports = [8080, \"10000-10200\"]\n    domains = []\n    subdomains = []\n    enable = true\n"), 0o600)
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := s.Get("bob")
	if !ok || len(u.Ports) != 2 || u.Ports[0] != "8080" || u.Ports[1] != "10000-10200" {
		t.Errorf("old file: %+v", u)
	}
}
