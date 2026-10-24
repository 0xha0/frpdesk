<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, type User } from "../api";
import { t, lang } from "../i18n";
import { toast } from "../toast";
import UserDialog from "./UserDialog.vue";
import ConfirmDialog from "./ConfirmDialog.vue";

const users = ref<User[]>([]);
const loading = ref(true);
const query = ref("");
const dialog = ref<{ user?: User } | null>(null);
const removing = ref<User | null>(null);

async function load() {
  loading.value = true;
  try { users.value = await api.users(); } catch (e) { toast((e as Error).message, "error"); } finally { loading.value = false; }
}
onMounted(load);

const shown = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q) return users.value;
  return users.value.filter((u) => [u.user, u.comment, ...u.ports, ...u.domains, ...u.subdomains].some((s) => s.toLowerCase().includes(q)));
});

function onSaved(u: User) {
  const i = users.value.findIndex((x) => x.user === u.user);
  if (i >= 0) users.value[i] = u; else users.value.push(u);
  users.value.sort((a, b) => a.user.localeCompare(b.user));
  dialog.value = null;
}

async function toggle(u: User) {
  try {
    const saved = await api.setEnabled(u.user, !u.enable);
    Object.assign(u, saved);
    toast(saved.enable ? t("Enabled") : t("Disabled"));
  } catch (e) { toast((e as Error).message, "error"); }
}

async function remove() {
  const u = removing.value!;
  try {
    await api.removeUser(u.user);
    users.value = users.value.filter((x) => x.user !== u.user);
    toast(t("User removed"));
  } catch (e) { toast((e as Error).message, "error"); } finally { removing.value = null; }
}

async function copy(text: string) {
  try { await navigator.clipboard.writeText(text); toast(t("Copied")); } catch { toast(t("Failed"), "error"); }
}

const mask = (s: string) => (s.length > 10 ? `${s.slice(0, 4)}…${s.slice(-4)}` : s);
const domains = (u: User) => [...u.domains, ...u.subdomains.map((s) => `${s}.*`)].join(", ");
const exportUrl = (u: User) => api.exportUrl(u.user, lang.value);
</script>

<template>
  <div class="mb-4 flex flex-wrap items-center gap-3">
    <input v-model="query" class="input w-full sm:max-w-xs sm:flex-1" :placeholder="t('Search users')" />
    <span class="text-sm text-text-3">{{ shown.length }} / {{ users.length }}</span>
    <button class="btn-primary ml-auto" @click="dialog = {}">+ {{ t("New user") }}</button>
  </div>

  <div v-if="loading" class="card p-8 text-center text-sm text-text-3">{{ t("Loading…") }}</div>
  <div v-else-if="users.length === 0" class="card p-8 text-center text-sm text-text-2">{{ t("No users yet. frps lets no client in until there is one.") }}</div>
  <div v-else-if="shown.length === 0" class="card p-8 text-center text-sm text-text-2">{{ t("No users match.") }}</div>
  <template v-else>
    <!-- a phone: a card per user -->
    <div class="grid gap-3 md:hidden">
      <div v-for="u in shown" :key="u.user" class="card p-4">
        <div class="flex items-center justify-between gap-2">
          <span class="mono text-base font-semibold">{{ u.user }}</span>
          <span class="badge" :class="u.enable ? 'bg-ok-soft text-ok' : 'bg-warn-soft text-warn'">{{ u.enable ? t("Enabled") : t("Disabled") }}</span>
        </div>
        <div class="mt-2 flex flex-wrap gap-1">
          <span v-for="p in u.ports" :key="p" class="badge mono bg-surface-2 text-text-2">{{ p }}</span>
          <span v-if="!u.ports.length" class="text-sm text-text-3">{{ t("Any port") }}</span>
        </div>
        <div v-if="domains(u)" class="mono mt-1 truncate text-sm text-text-2">{{ domains(u) }}</div>
        <div v-if="u.comment" class="mt-1 text-sm text-text-2">{{ u.comment }}</div>
        <button class="mono mt-2 inline-flex items-center gap-1.5 text-sm text-text-2" @click="copy(u.token)">
          {{ mask(u.token) }}
          <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="7" y="7" width="9" height="9" rx="1.5"/><path d="M4 13V5a1 1 0 011-1h8"/></svg>
          <span class="text-text-3">{{ t("Copy") }}</span>
        </button>
        <div class="mt-3 flex flex-wrap gap-2">
          <a class="btn-outline" :href="exportUrl(u)" :download="`frpc-${u.user}.toml`">
            <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M10 3v10m0 0 4-4m-4 4-4-4M4 16h12"/></svg>
            frpc.toml
          </a>
          <button class="btn-outline" @click="dialog = { user: u }">{{ t("Edit") }}</button>
          <button class="btn-outline" @click="toggle(u)">{{ u.enable ? t("Disable") : t("Enable") }}</button>
          <button class="btn-ghost ml-auto text-danger" @click="removing = u">{{ t("Remove") }}</button>
        </div>
      </div>
    </div>
    <!-- wider: the table -->
    <div class="card hidden overflow-hidden md:block">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead class="border-b border-line bg-surface-2/60">
            <tr>
              <th class="th">{{ t("User") }}</th>
              <th class="th">{{ t("Token") }}</th>
              <th class="th">{{ t("Ports") }}</th>
              <th class="th">{{ t("Domains") }}</th>
              <th class="th">{{ t("Notes") }}</th>
              <th class="th">{{ t("Status") }}</th>
              <th class="th"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in shown" :key="u.user" class="border-b border-line last:border-0 hover:bg-surface-2/40">
              <td class="td mono font-medium">{{ u.user }}</td>
              <td class="td">
                <button class="mono inline-flex items-center gap-1.5 text-text-2 hover:text-text" :title="t('Copy')" @click="copy(u.token)">
                  {{ mask(u.token) }}
                  <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="7" y="7" width="9" height="9" rx="1.5"/><path d="M4 13V5a1 1 0 011-1h8"/></svg>
                </button>
              </td>
              <td class="td mono">
                <span v-if="u.ports.length" class="flex flex-wrap gap-1"><span v-for="p in u.ports" :key="p" class="badge bg-surface-2 text-text-2">{{ p }}</span></span>
                <span v-else class="text-text-3">{{ t("Any port") }}</span>
              </td>
              <td class="td mono text-text-2">
                <span v-if="domains(u)">{{ domains(u) }}</span>
                <span v-else class="text-text-3">{{ t("Any") }}</span>
              </td>
              <td class="td max-w-[16rem] truncate text-text-2" :title="u.comment">{{ u.comment }}</td>
              <td class="td">
                <span class="badge" :class="u.enable ? 'bg-ok-soft text-ok' : 'bg-warn-soft text-warn'">{{ u.enable ? t("Enabled") : t("Disabled") }}</span>
              </td>
              <td class="td">
                <div class="flex justify-end gap-1 whitespace-nowrap">
                  <a class="btn-outline" :href="exportUrl(u)" :download="`frpc-${u.user}.toml`">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M10 3v10m0 0 4-4m-4 4-4-4M4 16h12"/></svg>
                    {{ t("Export frpc.toml") }}
                  </a>
                  <button class="btn-ghost" @click="dialog = { user: u }">{{ t("Edit") }}</button>
                  <button class="btn-ghost" @click="toggle(u)">{{ u.enable ? t("Disable") : t("Enable") }}</button>
                  <button class="btn-ghost text-danger" @click="removing = u">{{ t("Remove") }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </template>

  <UserDialog v-if="dialog" :user="dialog.user" @saved="onSaved" @close="dialog = null" />
  <ConfirmDialog v-if="removing" :title="t('Remove user {name}?', { name: removing.user })"
    :text="t('Its clients lose their login at once; the ports go free.')" :confirm-label="t('Remove')" danger
    @confirm="remove" @cancel="removing = null" />
</template>
