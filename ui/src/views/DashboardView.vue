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
  { name: "server", label: "Server" },
  { name: "users", label: "Users" },
  { name: "proxies", label: "Proxies" },
];
</script>

<template>
  <div class="min-h-screen">
    <header class="sticky top-0 z-10 border-b border-line bg-surface/90 backdrop-blur">
      <div class="mx-auto max-w-6xl px-4">
        <div class="flex h-12 items-center gap-3">
          <router-link to="/" class="flex shrink-0 items-center gap-2.5">
            <span class="grid h-7 w-7 place-items-center rounded-lg bg-accent text-sm font-bold text-white">f</span>
            <span class="font-semibold">frpdesk</span>
          </router-link>
          <!-- the tabs beside the name on a wide screen; in a row of their own on a phone -->
          <nav class="ml-2 hidden gap-1 md:flex">
            <router-link v-for="tab in tabs" :key="tab.name" :to="{ name: tab.name }"
              class="rounded-lg px-3 py-1.5 text-sm text-text-2 hover:bg-surface-2 hover:text-text"
              active-class="!bg-accent-soft !text-accent font-medium">{{ t(tab.label) }}</router-link>
          </nav>
          <div class="ml-auto flex min-w-0 items-center gap-1 text-sm">
            <span v-if="me?.client.server_addr" class="mono hidden truncate text-text-3 lg:inline" :title="t('Clients connect to')">
              {{ me.client.server_addr }}:{{ me.client.server_port }}
            </span>
            <button class="btn-ghost shrink-0 text-xs" @click="setLang(lang === 'zh' ? 'en' : 'zh')">{{ lang === "zh" ? "EN" : "中文" }}</button>
            <button class="btn-ghost shrink-0" @click="signOut">{{ t("Sign out") }}</button>
          </div>
        </div>
        <nav class="-mx-1 flex gap-1 pb-2 md:hidden">
          <router-link v-for="tab in tabs" :key="tab.name" :to="{ name: tab.name }"
            class="flex-1 rounded-lg px-3 py-1.5 text-center text-sm text-text-2 hover:bg-surface-2 hover:text-text"
            active-class="!bg-accent-soft !text-accent font-medium">{{ t(tab.label) }}</router-link>
        </nav>
      </div>
    </header>
    <main class="mx-auto max-w-6xl px-4 py-4 md:py-6">
      <router-view />
    </main>
    <footer class="mx-auto max-w-6xl px-4 pb-6 text-xs text-text-3">frpdesk {{ me?.version }}</footer>
  </div>
</template>
