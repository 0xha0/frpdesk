<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, type Ref } from "vue";
import { api, formatBytes, formatRate, formatTime, formatDateTime, type Me, type Sample, type Hour, type Traffic, type FrpsClient } from "../api";
import { t } from "../i18n";
import LineChart from "./LineChart.vue";
import BarChart from "./BarChart.vue";
import Sparkline from "./Sparkline.vue";
import ShareBar from "./ShareBar.vue";
import BarList from "./BarList.vue";

type Range = "24h" | "7d" | "30d";
const me = inject<Ref<Me | null>>("me");
const info = ref<any | null>(null);
const samples = ref<Sample[]>([]);   // the last day, one a minute
const hours = ref<Hour[]>([]);       // the last 30 days, one an hour
const traffic = ref<Traffic | null>(null);
const clients = ref<FrpsClient[]>([]);
const error = ref("");
const proxy = ref("");   // "" = every proxy
const range = ref<Range>("24h");
const updated = ref<Date | null>(null);
let timer: number | undefined;

async function load() {
  try {
    const [i, h24, h30, tr, cl] = await Promise.all([api.serverInfo(), api.history("24h"), api.history("30d"), api.traffic(), api.clients()]);
    info.value = i; samples.value = h24.samples ?? []; hours.value = h30.hours ?? []; traffic.value = tr; clients.value = cl;
    error.value = "";
    updated.value = new Date();
  } catch (e) {
    error.value = (e as Error).message;
  }
}
onMounted(() => {
  load();
  // live figures: again every minute while the page is shown
  timer = window.setInterval(() => { if (document.visibilityState === "visible") load(); }, 60_000);
});
onBeforeUnmount(() => window.clearInterval(timer));

const in1 = "var(--series-1)";
const in2 = "var(--series-2)";
const pad = (n: number) => String(n).padStart(2, "0");
const dayOf = (unix: number) => { const d = new Date(unix * 1000); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`; };
const hourLabel = (unix: number) => { const d = new Date(unix * 1000); return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:00`; };

// the line charts' points: the day's minutes, or the range's hours (averages)
const points = computed(() => {
  if (range.value === "24h") return samples.value.map((s) => ({ t: s.t, conns: s.conns, clients: s.clients, in: s.in, out: s.out }));
  const cut = Date.now() / 1000 - (range.value === "7d" ? 7 : 30) * 86400;
  return hours.value.filter((h) => h.t >= cut).map((h) => ({ t: h.t, conns: h.conns, clients: h.clients, in: h.in, out: h.out }));
});
const labels = computed(() => points.value.map((p) => (range.value === "24h" ? formatTime(p.t) : hourLabel(p.t))));
const last = computed(() => samples.value[samples.value.length - 1]);
const recent = (pick: (s: Sample) => number) => samples.value.slice(-120).map(pick);
const peak = (pick: (s: Sample) => number) => samples.value.reduce((m, s) => Math.max(m, pick(s)), 0);
const rangeTitle = computed(() => t(range.value === "24h" ? "last 24 hours" : range.value === "7d" ? "last 7 days" : "last 30 days"));

// traffic by day: every proxy together from our hours (they survive frps's restarts; 7 or 30
// days by the range), one proxy from frps (it keeps 7 days per proxy, in memory)
const days = computed(() => (range.value === "30d" ? 30 : 7));
const daily = computed(() => {
  if (proxy.value) {
    const p = traffic.value?.proxies.find((x) => x.name === proxy.value);
    return p ? { labels: traffic.value!.days.map((d) => d.slice(5)), in: p.in, out: p.out, note: t("frps keeps 7 days per proxy") } : null;
  }
  const byDay = new Map<string, { in: number; out: number }>();
  for (const h of hours.value) {
    const k = dayOf(h.t);
    const e = byDay.get(k) ?? { in: 0, out: 0 };
    e.in += h.inBytes; e.out += h.outBytes;
    byDay.set(k, e);
  }
  const out = { labels: [] as string[], in: [] as number[], out: [] as number[], note: "" };
  for (let i = days.value - 1; i >= 0; i--) {
    const d = new Date(); d.setDate(d.getDate() - i);
    const k = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    const e = byDay.get(k);
    out.labels.push(k.slice(5)); out.in.push(e?.in ?? 0); out.out.push(e?.out ?? 0);
  }
  return out;
});
const daySeries = computed(() => daily.value ? [{ name: t("In"), color: in1, values: daily.value.in }, { name: t("Out"), color: in2, values: daily.value.out }] : []);
const periodTotal = computed(() => daySeries.value.reduce((s, x) => s + x.values.reduce((a, b) => a + b, 0), 0));

// traffic by user: frps's per-proxy days summed per user (today for the 24h range, else 7 days:
// frps keeps no more per proxy); a user selected shows its proxies
const userSel = ref("");
const period = (p: { in: number[]; out: number[] }) => {
  if (range.value === "24h") return { in: p.in[p.in.length - 1] ?? 0, out: p.out[p.out.length - 1] ?? 0 };
  return { in: p.in.reduce((a, b) => a + b, 0), out: p.out.reduce((a, b) => a + b, 0) };
};
const byUser = computed(() => {
  const m = new Map<string, number>();
  for (const p of traffic.value?.proxies ?? []) {
    const v = period(p);
    m.set(p.user || "—", (m.get(p.user || "—") ?? 0) + v.in + v.out);
  }
  return [...m].map(([key, value]) => ({ key, label: key, value }));
});
const userProxies = computed(() => (traffic.value?.proxies ?? []).filter((p) => (p.user || "—") === userSel.value).map((p) => {
  const v = period(p);
  const name = p.name.startsWith(userSel.value + ".") ? p.name.slice(userSel.value.length + 1) : p.name;
  return { label: `${name} · ${p.type}`, value: v.in + v.out, in: v.in, out: v.out };
}));
const userPeriod = computed(() => (range.value === "24h" ? t("today") : t("last 7 days")));

const port = (v: number) => (v ? String(v) : "—");
const yesno = (v: boolean) => (v ? t("yes") : t("no"));
const fields = computed(() => info.value ? [
  ["Version", info.value.version],
  ["Bind port", port(info.value.bindPort)],
  ["KCP port", port(info.value.kcpBindPort)],
  ["QUIC port", port(info.value.quicBindPort)],
  ["HTTP port", port(info.value.vhostHTTPPort)],
  ["HTTPS port", port(info.value.vhostHTTPSPort)],
  ["TCPMUX port", port(info.value.tcpmuxHTTPConnectPort)],
  ["Subdomain host", info.value.subdomainHost || "—"],
  ["Max pool count", info.value.maxPoolCount],
  ["Max ports per client", info.value.maxPortsPerClient || "—"],
  ["Heartbeat timeout", info.value.heartbeatTimeout > 0 ? `${info.value.heartbeatTimeout}s` : "—"],
  ["Allowed ports", info.value.allowPortsStr || "—"],
  ["TLS only", yesno(info.value.tlsForce)],
] : []);
</script>

<template>
  <div v-if="error && !info" class="card p-8 text-center text-sm text-danger">{{ t("frps dashboard unreachable") }}: {{ error }}</div>
  <template v-else-if="info">
    <!-- the filters: one row, above everything they scope -->
    <div class="mb-4 flex flex-wrap items-center gap-2 text-sm">
      <div class="flex gap-1 rounded-lg bg-surface-2 p-1">
        <button v-for="r in (['24h', '7d', '30d'] as Range[])" :key="r" class="rounded-md px-2.5 py-1 text-sm transition-colors"
          :class="range === r ? 'bg-surface text-text shadow-sm' : 'text-text-2 hover:text-text'" @click="range = r">{{ t(r) }}</button>
      </div>
      <select v-model="proxy" class="input w-auto max-w-[14rem] !py-1.5" :title="t('Traffic by day for')">
        <option value="">{{ t("All proxies") }}</option>
        <option v-for="p in traffic?.proxies ?? []" :key="p.name" :value="p.name">{{ p.name }}</option>
      </select>
      <span v-if="updated" class="ml-auto text-xs text-text-3">{{ t("Updated") }} {{ formatTime(Math.floor(updated.getTime() / 1000)) }}<span v-if="error" class="text-danger"> · {{ error }}</span></span>
      <button class="btn-outline" @click="load">{{ t("Refresh") }}</button>
    </div>

    <div class="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div class="card p-3 md:p-4">
        <div class="text-xs text-text-3">{{ t("Current connections") }}</div>
        <div class="mt-0.5 text-xl font-semibold md:text-2xl">{{ info.curConns }}</div>
        <Sparkline :values="recent((s) => s.conns)" />
        <div class="mt-1 text-xs text-text-3">{{ t("peak 24 h") }} {{ peak((s) => s.conns) }}</div>
      </div>
      <div class="card p-3 md:p-4">
        <div class="text-xs text-text-3">{{ t("Clients") }}</div>
        <div class="mt-0.5 text-xl font-semibold md:text-2xl">{{ info.clientCounts }}</div>
        <Sparkline :values="recent((s) => s.clients)" />
        <div class="mt-1 text-xs text-text-3">{{ t("peak 24 h") }} {{ peak((s) => s.clients) }}</div>
      </div>
      <div class="card p-3 md:p-4">
        <div class="text-xs text-text-3">{{ t("Inbound now") }}</div>
        <div class="mt-0.5 truncate text-xl font-semibold md:text-2xl">{{ formatRate(last?.in) }}</div>
        <Sparkline :values="recent((s) => s.in)" />
        <div class="mt-1 text-xs text-text-3">{{ t("today") }} {{ formatBytes(info.totalTrafficIn) }}</div>
      </div>
      <div class="card p-3 md:p-4">
        <div class="text-xs text-text-3">{{ t("Outbound now") }}</div>
        <div class="mt-0.5 truncate text-xl font-semibold md:text-2xl">{{ formatRate(last?.out) }}</div>
        <Sparkline :values="recent((s) => s.out)" />
        <div class="mt-1 text-xs text-text-3">{{ t("today") }} {{ formatBytes(info.totalTrafficOut) }}</div>
      </div>
    </div>

    <div class="card mb-4 p-3 md:p-4">
      <h3 class="mb-2 text-sm font-medium">{{ t("Throughput") }}, {{ rangeTitle }}<span v-if="range !== '24h'" class="ml-1 font-normal text-text-3">({{ t("hourly averages") }})</span></h3>
      <LineChart :series="[{ name: t('In'), color: in1, values: points.map((p) => p.in) }, { name: t('Out'), color: in2, values: points.map((p) => p.out) }]" :labels="labels" :format="formatRate" :height="220" :base="1024" />
    </div>
    <div class="mb-4 grid gap-4 md:grid-cols-2">
      <div class="card p-3 md:p-4">
        <h3 class="mb-2 text-sm font-medium">{{ t("Connections") }}, {{ rangeTitle }}</h3>
        <LineChart :series="[{ name: t('Connections'), color: in1, values: points.map((p) => p.conns) }]" :labels="labels" :format="(v) => String(Math.round(v))" :height="160" />
      </div>
      <div class="card p-3 md:p-4">
        <h3 class="mb-2 text-sm font-medium">{{ t("Clients") }}, {{ rangeTitle }}</h3>
        <LineChart :series="[{ name: t('Clients'), color: in1, values: points.map((p) => p.clients) }]" :labels="labels" :format="(v) => String(Math.round(v))" :height="160" />
      </div>
    </div>
    <div class="card mb-4 p-3 md:p-4">
      <div class="mb-2 flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 class="text-sm font-medium">{{ t("Traffic by day") }}, {{ proxy ? t("last 7 days") : t(days === 30 ? "last 30 days" : "last 7 days") }}<span class="ml-2 font-normal text-text-3">{{ proxy || t("All proxies") }}</span></h3>
        <span class="text-xs text-text-3">{{ daily?.note || t("from frpdesk's own samples") }} · {{ formatBytes(periodTotal) }}</span>
      </div>
      <BarChart v-if="daily" :series="daySeries" :labels="daily.labels" :format="(v) => formatBytes(v)" :height="200" :base="1024" />
    </div>

    <div class="card mb-4 p-3 md:p-4">
      <div class="mb-3 flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 class="text-sm font-medium">
          <template v-if="!userSel">{{ t("Traffic by user") }}, {{ userPeriod }}</template>
          <template v-else>{{ t("Proxies of {user}", { user: userSel }) }}, {{ userPeriod }}</template>
        </h3>
        <button v-if="userSel" class="btn-ghost !px-2 !py-0.5 text-xs" @click="userSel = ''">← {{ t("All users") }}</button>
        <span v-else-if="range === '30d'" class="text-xs text-text-3">{{ t("frps keeps 7 days") }}</span>
      </div>
      <ShareBar v-if="!userSel" :items="byUser" :format="(v) => formatBytes(v)" @select="userSel = $event" />
      <BarList v-else :rows="userProxies" :format="(v) => formatBytes(v)" />
    </div>

    <div class="card mb-4 overflow-hidden">
      <h3 class="border-b border-line px-4 py-2.5 text-sm font-medium">{{ t("Connected clients") }} <span class="font-normal text-text-3">{{ clients.length }}</span></h3>
      <div v-if="clients.length === 0" class="p-6 text-center text-sm text-text-2">{{ t("No clients.") }}</div>
      <template v-else>
        <div class="divide-y divide-line md:hidden">
          <div v-for="c in clients" :key="c.key" class="p-4">
            <div class="flex items-center justify-between gap-2">
              <span class="mono font-semibold">{{ c.user || "—" }}<span class="font-normal text-text-3"> · {{ c.hostname || "—" }}</span></span>
              <span class="badge" :class="c.online ? 'bg-ok-soft text-ok' : 'bg-surface-2 text-text-3'">{{ c.online ? t("online") : t("offline") }}</span>
            </div>
            <div class="mono mt-1 text-sm text-text-2">{{ c.version || "—" }} · {{ c.wireProtocol || "—" }} · {{ c.clientIP || "—" }}</div>
            <div class="mono mt-0.5 text-xs text-text-3">{{ t("Connected since") }} {{ formatDateTime(c.firstConnectedAt) }}</div>
          </div>
        </div>
        <div class="hidden overflow-x-auto md:block">
          <table class="w-full">
            <thead class="bg-surface-2/60"><tr>
              <th class="th">{{ t("User") }}</th><th class="th">{{ t("Host") }}</th><th class="th">{{ t("Version") }}</th><th class="th">{{ t("Protocol") }}</th><th class="th">{{ t("Address") }}</th><th class="th">{{ t("Connected since") }}</th><th class="th">{{ t("Status") }}</th>
            </tr></thead>
            <tbody>
              <tr v-for="c in clients" :key="c.key" class="border-t border-line hover:bg-surface-2/40">
                <td class="td mono font-medium">{{ c.user || "—" }}</td>
                <td class="td mono text-text-2">{{ c.hostname || "—" }}</td>
                <td class="td mono text-text-2">{{ c.version || "—" }}</td>
                <td class="td mono text-text-2">{{ c.wireProtocol || "—" }}</td>
                <td class="td mono text-text-2">{{ c.clientIP || "—" }}</td>
                <td class="td mono text-text-2">{{ formatDateTime(c.firstConnectedAt) }}</td>
                <td class="td"><span class="badge" :class="c.online ? 'bg-ok-soft text-ok' : 'bg-surface-2 text-text-3'">{{ c.online ? t("online") : t("offline") }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </div>

    <div class="grid gap-4 md:grid-cols-[1fr_18rem]">
      <div class="card divide-y divide-line">
        <div v-for="[k, v] in fields" :key="k" class="flex items-center justify-between gap-4 px-4 py-2.5 text-sm">
          <span class="text-text-2">{{ t(k) }}</span>
          <span class="mono text-right">{{ v }}</span>
        </div>
      </div>
      <div>
        <div class="card p-4">
          <div class="mb-2 text-xs text-text-3">{{ t("Proxies by type") }}</div>
          <div v-for="(n, ty) in info.proxyTypeCount" :key="ty" class="flex justify-between py-0.5 text-sm">
            <span class="mono text-text-2">{{ ty }}</span><span class="mono">{{ n }}</span>
          </div>
          <div v-if="!info.proxyTypeCount || Object.keys(info.proxyTypeCount).length === 0" class="text-sm text-text-3">0</div>
        </div>
        <div v-if="me?.client.server_addr" class="card mt-4 p-4 text-sm">
          <div class="mb-1 text-xs text-text-3">{{ t("Clients connect to") }}</div>
          <div class="mono">{{ me.client.server_addr }}:{{ me.client.server_port }}</div>
          <div v-if="me.client.tls_server_name" class="mono mt-1 text-text-2">tls.serverName = {{ me.client.tls_server_name }}</div>
        </div>
      </div>
    </div>
  </template>
  <div v-else class="card p-8 text-center text-sm text-text-3">{{ t("Loading…") }}</div>
</template>
