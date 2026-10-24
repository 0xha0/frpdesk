<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, ApiError } from "../api";
import { t, lang, setLang } from "../i18n";

const router = useRouter();
const route = useRoute();
const username = ref("");
const password = ref("");
const busy = ref(false);
const error = ref("");

// where to go once signed in: ?next= when it's a path of ours and not the login page itself
function next() {
  const n = route.query.next;
  return typeof n === "string" && n.startsWith("/") && !n.startsWith("/login") ? n : "/";
}

// already signed in (a stale link, a second tab): straight on
onMounted(async () => {
  try {
    await api.me();
    router.replace(next());
  } catch { /* not signed in: the form */ }
});

async function submit() {
  error.value = "";
  busy.value = true;
  try {
    await api.login(username.value, password.value);
    router.replace(next());
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 401 ? t("Wrong username or password") : String((e as Error).message);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center p-4">
    <form class="card w-full max-w-sm p-6 shadow-sm" @submit.prevent="submit">
      <div class="mb-6 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <span class="grid h-8 w-8 place-items-center rounded-lg bg-accent text-white font-bold">f</span>
          <span class="text-lg font-semibold">frpdesk</span>
        </div>
        <button type="button" class="btn-ghost text-xs" @click="setLang(lang === 'zh' ? 'en' : 'zh')">{{ lang === "zh" ? "EN" : "中文" }}</button>
      </div>
      <label class="mb-1 block text-sm text-text-2">{{ t("Username") }}</label>
      <input v-model="username" class="input mb-4" autocomplete="username" autofocus required />
      <label class="mb-1 block text-sm text-text-2">{{ t("Password") }}</label>
      <input v-model="password" type="password" class="input mb-4" autocomplete="current-password" required />
      <p v-if="error" class="mb-3 text-sm text-danger">{{ error }}</p>
      <button class="btn-primary w-full justify-center py-2" :disabled="busy">{{ busy ? t("Signing in…") : t("Sign in") }}</button>
    </form>
  </div>
</template>
