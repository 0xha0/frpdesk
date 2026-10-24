# frpdesk

Users for [frp](https://github.com/fatedier/frp)'s server: each one a token and the ports (domains,
subdomains) it may take, checked by frps at every login and proxy through its server plugin; a web UI
to manage them, download each user's ready-to-run `frpc.toml`, and watch frps's proxies and traffic.
One binary (the UI is embedded), one TOML of users.

Made for renting out ports of a single frps: a user gets a block of ports and a config file, and
can't touch anyone else's.

## How it works

```
frpc (user A) ──┐                        ┌─ POST /handler ──► frpdesk ◄── browser (admin)
frpc (user B) ──┼──► frps ──httpPlugins──┘   (is this user, token, port allowed?)   │
                │                                                                   │
  the users' ports ◄── frps's dashboard API ◄───────────────────────────────────────┘
```

- **frps** runs as usual; `[[httpPlugins]]` in its config points at frpdesk's plugin endpoint.
- **frpdesk** answers Login, NewProxy, Ping, NewWorkConn and NewUserConn: the client's `user` must
  exist and be on, its `metadatas.token` must match, a tcp/udp proxy's `remotePort` must be in the
  user's ports, an http/https proxy's domains and subdomain in the user's lists. Empty lists allow
  anything frps allows. No users at all: nobody gets in.
- The **web UI** (its own listener) needs the admin's login; its session cookie is signed and
  encrypted with keys made at each start. frps's own dashboard is read through it for the Server
  and Proxies pages, so it can stay on 127.0.0.1.
- **Export**: each user has a button that downloads a complete `frpc.toml` (server address, the
  server's token, their user and token, TLS, an example proxy on their first port) from the
  `[client]` section of the config.

## Install

Releases: a tarball per platform with the binary, `frpdesk.example.toml` and `frps.example.toml`.
Or build: `make` (needs Go 1.26 and Node 22+; the UI is built into the binary).

1. `frpdesk.toml` next to the binary (see `frpdesk.example.toml`): the two listeners, the admin's
   login, frps's dashboard, and `[client]` (what the exported configs say).
2. In `frps.toml` (see `frps.example.toml`): `[[httpPlugins]]` at frpdesk's plugin endpoint, the
   dashboard on 127.0.0.1.
3. Run `frpdesk -c /etc/frpdesk/frpdesk.toml` (a systemd unit: run it as a user of its own with a
   writable directory for `users.toml`), restart frps, open the UI behind your HTTPS proxy, add users.

`frpdesk -check -c …` validates the config and the users' file.

## The users' file

`users.toml` beside the config, written by the UI (atomically, mode 600):

```toml
[tokens.alice]
user = "alice"
token = "…"
comment = "ports 30000-30099"
ports = ["30000-30099"]
domains = []
subdomains = []
enable = true
```

The layout is frps-panel's, so its `frps-tokens.toml` loads as it is.

## A user's frpc.toml

What the Export button gives (with `[client]` filled in):

```toml
serverAddr = "frp.example.com"
serverPort = 7000
auth.token = "…"
user = "alice"
metadatas.token = "…"
loginFailExit = false
transport.tls.enable = true

[[proxies]]
name = "ssh"
type = "tcp"
localIP = "127.0.0.1"
localPort = 22
remotePort = 30000
```

## Development

`cd ui && npm run dev` serves the UI with hot reload and passes `/api` to a running frpdesk on
127.0.0.1:7201. `make test` vets and tests the Go code.

## Credits and license

Started from [yhl452493373/frps-panel](https://github.com/yhl452493373/frps-panel) (Apache-2.0,
archived 2024): its users-as-frps-plugin idea and file layout. The code was rewritten; the UI is
new (Vue 3, Tailwind). Apache-2.0.
