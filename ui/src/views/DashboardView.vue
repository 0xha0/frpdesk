<script setup lang="ts">
import { onMounted, provide, ref } from "vue";
import { useRouter } from "vue-router";
import { api, type Me } from "../api";
import { t, lang, setLang } from "../i18n";
import { toast } from "../toast";

const router = useRouter();
const me = ref<Me | null>(null);
provide("me", me);

onMounted(async () => {
  try { me.value = await api.me(); } catch { /* a 401 already sent us to the login */ }
});

async function signOut() {
  await api.logout();
  toast(t("Signed out"));
  router.replace({ name: "login" });
}

const tabs = [
  { name: "users", label: "Users" },
  { name: "proxies", label: "Proxies" },
  { name: "server", label: "Server" },
];
</script>

<template>
  <div class="min-h-screen">
    <header class="sticky top-0 z-10 border-b border-line bg-surface/90 backdrop-blur">
      <div class="mx-auto flex max-w-6xl items-center gap-4 px-4 py-2.5">
        <router-link to="/" class="flex items-center gap-2.5">
          <span class="grid h-7 w-7 place-items-center rounded-lg bg-accent text-white text-sm font-bold">f</span>
          <span class="font-semibold">frpdesk</span>
        </router-link>
        <nav class="ml-2 flex gap-1">
          <router-link v-for="tab in tabs" :key="tab.name" :to="{ name: tab.name }"
            class="rounded-lg px-3 py-1.5 text-sm text-text-2 hover:bg-surface-2 hover:text-text"
            active-class="!bg-accent-soft !text-accent font-medium">{{ t(tab.label) }}</router-link>
        </nav>
        <div class="ml-auto flex items-center gap-1 text-sm">
          <span v-if="me?.client.server_addr" class="mono hidden text-text-3 sm:inline" :title="t('Clients connect to')">
            {{ me.client.server_addr }}:{{ me.client.server_port }}
          </span>
          <button class="btn-ghost text-xs" @click="setLang(lang === 'zh' ? 'en' : 'zh')">{{ lang === "zh" ? "EN" : "中文" }}</button>
          <button class="btn-ghost" @click="signOut">{{ t("Sign out") }}</button>
        </div>
      </div>
    </header>
    <main class="mx-auto max-w-6xl px-4 py-6">
      <router-view />
    </main>
    <footer class="mx-auto max-w-6xl px-4 pb-6 text-xs text-text-3">frpdesk {{ me?.version }}</footer>
  </div>
</template>
