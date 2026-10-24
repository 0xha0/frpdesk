// the API client: JSON in and out, errors as exceptions with the server's message, a 401 sends the
// app to the login page
import router from "./router";

export interface User {
  user: string;
  token: string;
  comment: string;
  ports: string[];
  domains: string[];
  subdomains: string[];
  enable: boolean;
}

export interface Me {
  user: string;
  version: string;
  client: { server_addr: string; server_port: number; tls_server_name: string };
}

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json", "X-Requested-With": "fetch" } : { "X-Requested-With": "fetch" },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });
  if (res.status === 401 && !path.endsWith("/login")) {
    router.replace({ name: "login", query: { next: router.currentRoute.value.fullPath } });
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: any = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!res.ok) throw new ApiError(res.status, data?.error ?? `HTTP ${res.status}`);
  return data as T;
}

export const api = {
  login: (username: string, password: string) => call<{ user: string }>("POST", "/api/login", { username, password }),
  logout: () => call<void>("POST", "/api/logout"),
  me: () => call<Me>("GET", "/api/me"),
  users: () => call<{ users: User[] }>("GET", "/api/users").then((r) => r.users),
  addUser: (u: User) => call<User>("POST", "/api/users", u),
  updateUser: (name: string, u: User) => call<User>("PUT", `/api/users/${encodeURIComponent(name)}`, u),
  removeUser: (name: string) => call<void>("DELETE", `/api/users/${encodeURIComponent(name)}`),
  setEnabled: (name: string, on: boolean) => call<User>("POST", `/api/users/${encodeURIComponent(name)}/${on ? "enable" : "disable"}`),
  exportUrl: (name: string) => `/api/users/${encodeURIComponent(name)}/frpc.toml`,
  serverInfo: () => call<any>("GET", "/api/frps/serverinfo"),
  proxies: (type: string) => call<{ proxies: any[] }>("GET", `/api/frps/proxies/${type}`).then((r) => r.proxies ?? []),
};

export function formatBytes(n: number | undefined): string {
  if (!n) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${units[i]}`;
}
