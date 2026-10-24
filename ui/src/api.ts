// the API client: JSON in and out, errors as exceptions with the server's message, a 401 sends the
// app to the login page. Every call carries X-Requested-With: the server refuses a change without it.
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

export interface Sample { t: number; conns: number; clients: number; in: number; out: number }
export interface Hour { t: number; n: number; conns: number; connsMax: number; clients: number; clientsMax: number; in: number; inMax: number; out: number; outMax: number; inBytes: number; outBytes: number }
export interface History { range: string; interval: number; samples?: Sample[]; hours?: Hour[] }
export interface DayTraffic { name: string; user: string; type: string; in: number[]; out: number[] }
export interface Traffic { days: string[]; total: { in: number[]; out: number[] }; proxies: DayTraffic[] }
export interface FrpsClient {
  key: string; user: string; clientID: string; runID: string; version?: string; wireProtocol?: string;
  hostname: string; clientIP?: string; firstConnectedAt: number; lastConnectedAt: number; disconnectedAt?: number; online: boolean;
}

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { "X-Requested-With": "fetch" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(path, { method, headers, body: body !== undefined ? JSON.stringify(body) : undefined, credentials: "same-origin" });
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
  exportUrl: (name: string, lang: string) => `/api/users/${encodeURIComponent(name)}/frpc.toml?lang=${lang}`,
  serverInfo: () => call<any>("GET", "/api/frps/serverinfo"),
  proxies: (type: string) => call<{ proxies: any[] }>("GET", `/api/frps/proxies/${type}`).then((r) => r.proxies ?? []),
  // frps answers a bare array (older builds wrapped it)
  clients: () => call<FrpsClient[] | { clients: FrpsClient[] }>("GET", "/api/frps/clients").then((r) => Array.isArray(r) ? r : r?.clients ?? []),
  traffic: () => call<Traffic>("GET", "/api/frps/traffic"),
  history: (range: "24h" | "7d" | "30d") => call<History>("GET", `/api/frps/history?range=${range}`),
};

export function formatBytes(n: number | undefined, digits?: number): string {
  if (!n || n < 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  const d = digits ?? (v < 10 && i > 0 ? 1 : 0);
  return `${v.toFixed(d)} ${units[i]}`;
}

export const formatRate = (n: number | undefined) => `${formatBytes(n)}/s`;

export function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

export function formatDateTime(unix: number): string {
  if (!unix) return "—";
  const d = new Date(unix * 1000);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
