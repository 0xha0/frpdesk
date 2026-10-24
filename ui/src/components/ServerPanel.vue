<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from "vue";
import { api, formatBytes, type Me } from "../api";
import { t } from "../i18n";

const me = inject<Ref<Me | null>>("me");
const info = ref<any | null>(null);
const error = ref("");

async function load() {
  error.value = "";
  try { info.value = await api.serverInfo(); } catch (e) { error.value = (e as Error).message; }
}
onMounted(load);

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
const stats = computed(() => info.value ? [
  ["Current connections", info.value.curConns],
  ["Clients", info.value.clientCounts],
  ["Total traffic in", formatBytes(info.value.totalTrafficIn)],
  ["Total traffic out", formatBytes(info.value.totalTrafficOut)],
] : []);
</script>

<template>
  <div v-if="error" class="card p-8 text-center text-sm text-danger">{{ t("frps dashboard unreachable") }}: {{ error }}</div>
  <template v-else-if="info">
    <div class="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4">
      <div v-for="[k, v] in stats" :key="k" class="card p-4">
        <div class="text-xs text-text-3">{{ t(k) }}</div>
        <div class="mono mt-1 text-xl font-semibold">{{ v }}</div>
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
