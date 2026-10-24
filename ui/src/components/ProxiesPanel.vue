<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { api, formatBytes } from "../api";
import { t } from "../i18n";

const types = ["tcp", "udp", "http", "https", "tcpmux", "stcp", "sudp", "xtcp"];
const type = ref("tcp");
const rows = ref<any[]>([]);
const loading = ref(false);
const error = ref("");

async function load() {
  loading.value = true;
  error.value = "";
  try { rows.value = await api.proxies(type.value); } catch (e) { error.value = (e as Error).message; rows.value = []; } finally { loading.value = false; }
}
onMounted(load);
watch(type, load);

// frps names a proxy "<user>.<name>" and gives the user beside it
const split = (p: any): [string, string] => {
  const user: string = p.user ?? "";
  const name: string = p.name ?? "";
  return [user, user && name.startsWith(user + ".") ? name.slice(user.length + 1) : name];
};
const remote = (p: any) => {
  const c = p.conf ?? {};
  if (c.remotePort) return `:${c.remotePort}`;
  const parts: string[] = [...(c.customDomains ?? [])];
  if (c.subdomain) parts.push(`${c.subdomain}.*`);
  return parts.join(", ") || "—";
};
const when = (s: string) => (s ? s.replace("T", " ").slice(0, 16) : "—");
</script>

<template>
  <div class="mb-4 flex flex-wrap items-center gap-2">
    <div class="flex flex-wrap gap-1 rounded-lg bg-surface-2 p-1">
      <button v-for="ty in types" :key="ty" class="rounded-md px-2.5 py-1 text-sm transition-colors"
        :class="type === ty ? 'bg-surface text-text shadow-sm' : 'text-text-2 hover:text-text'" @click="type = ty">{{ ty }}</button>
    </div>
    <button class="btn-outline ml-auto" :disabled="loading" @click="load">{{ t("Refresh") }}</button>
  </div>

  <div class="card overflow-hidden">
    <div v-if="error" class="p-8 text-center text-sm text-danger">{{ t("frps dashboard unreachable") }}: {{ error }}</div>
    <div v-else-if="loading && rows.length === 0" class="p-8 text-center text-sm text-text-3">{{ t("Loading…") }}</div>
    <div v-else-if="rows.length === 0" class="p-8 text-center text-sm text-text-2">{{ t("No proxies of this type.") }}</div>
    <div v-else class="overflow-x-auto">
      <table class="w-full">
        <thead class="border-b border-line bg-surface-2/60">
          <tr>
            <th class="th">{{ t("User") }}</th>
            <th class="th">{{ t("Name") }}</th>
            <th class="th">{{ t("Remote") }}</th>
            <th class="th">{{ t("Status") }}</th>
            <th class="th">{{ t("Client") }}</th>
            <th class="th">{{ t("Today in / out") }}</th>
            <th class="th">{{ t("Connections") }}</th>
            <th class="th">{{ t("Last start") }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in rows" :key="p.name" class="border-b border-line last:border-0 hover:bg-surface-2/40">
            <td class="td mono">{{ split(p)[0] || "—" }}</td>
            <td class="td mono font-medium">{{ split(p)[1] }}</td>
            <td class="td mono">{{ remote(p) }}</td>
            <td class="td"><span class="badge" :class="p.status === 'online' ? 'bg-ok-soft text-ok' : 'bg-surface-2 text-text-3'">{{ t(p.status) }}</span></td>
            <td class="td mono text-text-2">{{ p.clientVersion || "—" }}</td>
            <td class="td mono text-text-2">{{ formatBytes(p.todayTrafficIn) }} / {{ formatBytes(p.todayTrafficOut) }}</td>
            <td class="td mono text-text-2">{{ p.curConns ?? 0 }}</td>
            <td class="td mono text-text-2">{{ when(p.lastStartTime) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
