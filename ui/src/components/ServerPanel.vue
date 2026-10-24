<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, type Ref } from "vue";
import { api, formatBytes, formatRate, formatTime, formatDateTime, type Me, type History, type Traffic, type FrpsClient } from "../api";
import { t } from "../i18n";
import LineChart from "./LineChart.vue";
import BarChart from "./BarChart.vue";
import Sparkline from "./Sparkline.vue";

const me = inject<Ref<Me | null>>("me");
const info = ref<any | null>(null);
const history = ref<History | null>(null);
const traffic = ref<Traffic | null>(null);
const clients = ref<FrpsClient[]>([]);
const error = ref("");
const proxy = ref("");   // "" = every proxy
const updated = ref<Date | null>(null);
let timer: number | undefined;

async function load() {
  try {
    const [i, h, tr, cl] = await Promise.all([api.serverInfo(), api.history(), api.traffic(), api.clients()]);
    info.value = i; history.value = h; traffic.value = tr; clients.value = cl;
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

const samples = computed(() => history.value?.samples ?? []);
const labels = computed(() => samples.value.map((s) => formatTime(s.t)));
const last = computed(() => samples.value[samples.value.length - 1]);
const recent = (pick: (s: History["samples"][number]) => number) => samples.value.slice(-120).map(pick);
const peak = (pick: (s: History["samples"][number]) => number) => samples.value.reduce((m, s) => Math.max(m, pick(s)), 0);
// the series' colours as CSS variables (SVG takes them), so a theme change recolours the charts live
const in1 = "var(--series-1)";
const in2 = "var(--series-2)";

const dayLabels = computed(() => (traffic.value?.days ?? []).map((d) => d.slice(5)));
const daySeries = computed(() => {
  if (!traffic.value) return [];
  const src = proxy.value ? traffic.value.proxies.find((p) => p.name === proxy.value) : traffic.value.total;
  if (!src) return [];
  return [{ name: t("In"), color: in1, values: src.in }, { name: t("Out"), color: in2, values: src.out }];
});
const weekTotal = computed(() => daySeries.value.reduce((s, x) => s + x.values.reduce((a, b) => a + b, 0), 0));

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
    <div class="mb-4 flex flex-wrap items-center gap-3 text-sm">
      <label class="flex items-center gap-2 text-text-2">
        {{ t("Traffic by day for") }}
        <select v-model="proxy" class="input w-auto !py-1.5">
          <option value="">{{ t("All proxies") }}</option>
          <option v-for="p in traffic?.proxies ?? []" :key="p.name" :value="p.name">{{ p.name }}</option>
        </select>
      </label>
      <span class="ml-auto text-xs text-text-3" v-if="updated">{{ t("Updated") }} {{ formatTime(Math.floor(updated.getTime() / 1000)) }}<span v-if="error" class="text-danger"> · {{ error }}</span></span>
      <button class="btn-outline" @click="load">{{ t("Refresh") }}</button>
    </div>

    <div class="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div class="card p-4">
        <div class="text-xs text-text-3">{{ t("Current connections") }}</div>
        <div class="mt-1 flex items-end justify-between gap-2"><span class="text-2xl font-semibold">{{ info.curConns }}</span><Sparkline :values="recent((s) => s.conns)" /></div>
        <div class="mt-1 text-xs text-text-3">{{ t("peak 24 h") }} {{ peak((s) => s.conns) }}</div>
      </div>
      <div class="card p-4">
        <div class="text-xs text-text-3">{{ t("Clients") }}</div>
        <div class="mt-1 flex items-end justify-between gap-2"><span class="text-2xl font-semibold">{{ info.clientCounts }}</span><Sparkline :values="recent((s) => s.clients)" /></div>
        <div class="mt-1 text-xs text-text-3">{{ t("peak 24 h") }} {{ peak((s) => s.clients) }}</div>
      </div>
      <div class="card p-4">
        <div class="text-xs text-text-3">{{ t("Inbound now") }}</div>
        <div class="mt-1 flex items-end justify-between gap-2"><span class="text-2xl font-semibold">{{ formatRate(last?.in) }}</span><Sparkline :values="recent((s) => s.in)" /></div>
        <div class="mt-1 text-xs text-text-3">{{ t("today") }} {{ formatBytes(info.totalTrafficIn) }}</div>
      </div>
      <div class="card p-4">
        <div class="text-xs text-text-3">{{ t("Outbound now") }}</div>
        <div class="mt-1 flex items-end justify-between gap-2"><span class="text-2xl font-semibold">{{ formatRate(last?.out) }}</span><Sparkline :values="recent((s) => s.out)" /></div>
        <div class="mt-1 text-xs text-text-3">{{ t("today") }} {{ formatBytes(info.totalTrafficOut) }}</div>
      </div>
    </div>

    <div class="card mb-4 p-4">
      <h3 class="mb-2 text-sm font-medium">{{ t("Throughput, last 24 hours") }}</h3>
      <LineChart :series="[{ name: t('In'), color: in1, values: samples.map((s) => s.in) }, { name: t('Out'), color: in2, values: samples.map((s) => s.out) }]" :labels="labels" :format="formatRate" :height="220" :base="1024" />
    </div>
    <div class="mb-4 grid gap-4 md:grid-cols-2">
      <div class="card p-4">
        <h3 class="mb-2 text-sm font-medium">{{ t("Connections, last 24 hours") }}</h3>
        <LineChart :series="[{ name: t('Connections'), color: in1, values: samples.map((s) => s.conns) }]" :labels="labels" :format="(v) => String(Math.round(v))" :height="160" />
      </div>
      <div class="card p-4">
        <h3 class="mb-2 text-sm font-medium">{{ t("Clients, last 24 hours") }}</h3>
        <LineChart :series="[{ name: t('Clients'), color: in1, values: samples.map((s) => s.clients) }]" :labels="labels" :format="(v) => String(Math.round(v))" :height="160" />
      </div>
    </div>
    <div class="card mb-4 p-4">
      <div class="mb-2 flex items-baseline justify-between">
        <h3 class="text-sm font-medium">{{ t("Traffic by day, last 7 days") }}<span class="ml-2 text-text-3 font-normal">{{ proxy || t("All proxies") }}</span></h3>
        <span class="text-xs text-text-3">{{ t("7 days") }} {{ formatBytes(weekTotal) }}</span>
      </div>
      <BarChart :series="daySeries" :labels="dayLabels" :format="(v) => formatBytes(v)" :height="200" :base="1024" />
    </div>

    <div class="card mb-4 overflow-hidden">
      <h3 class="border-b border-line px-4 py-2.5 text-sm font-medium">{{ t("Connected clients") }} <span class="text-text-3 font-normal">{{ clients.length }}</span></h3>
      <div v-if="clients.length === 0" class="p-6 text-center text-sm text-text-2">{{ t("No clients.") }}</div>
      <div v-else class="overflow-x-auto">
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
