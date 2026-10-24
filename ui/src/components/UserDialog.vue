<script setup lang="ts">
import { reactive, ref } from "vue";
import { api, type User } from "../api";
import { t } from "../i18n";
import { toast } from "../toast";

const props = defineProps<{ user?: User }>();
const emit = defineEmits<{ saved: [User]; close: [] }>();

const editing = !!props.user;
const form = reactive({
  user: props.user?.user ?? "",
  token: props.user?.token ?? "",
  comment: props.user?.comment ?? "",
  ports: props.user?.ports.join(", ") ?? "",
  domains: props.user?.domains.join(", ") ?? "",
  subdomains: props.user?.subdomains.join(", ") ?? "",
  enable: props.user?.enable ?? true,
});
const busy = ref(false);
const error = ref("");

function generateToken() {
  const bytes = new Uint8Array(24);
  crypto.getRandomValues(bytes);
  form.token = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}
if (!editing) generateToken();

const split = (s: string) => s.split(/[,\s]+/).map((x) => x.trim()).filter(Boolean);

async function save() {
  error.value = "";
  busy.value = true;
  const u: User = { user: form.user.trim(), token: form.token.trim(), comment: form.comment.trim(),
    ports: split(form.ports), domains: split(form.domains), subdomains: split(form.subdomains), enable: form.enable };
  try {
    const saved = editing ? await api.updateUser(props.user!.user, u) : await api.addUser(u);
    toast(t("User saved"));
    emit("saved", saved);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="fixed inset-0 z-40 grid place-items-center bg-black/40 p-4" @click.self="emit('close')">
    <form class="card w-full max-w-lg p-5 shadow-xl" @submit.prevent="save">
      <h3 class="text-base font-semibold">{{ editing ? t("Edit user") : t("New user") }}</h3>
      <div class="mt-4 grid gap-4">
        <div>
          <label class="mb-1 block text-sm text-text-2">{{ t("User") }}</label>
          <input v-model="form.user" class="input mono" :disabled="editing" :placeholder="t('Letters, digits and _')" required pattern="\w+" autofocus />
        </div>
        <div>
          <label class="mb-1 block text-sm text-text-2">{{ t("Token") }}</label>
          <div class="flex gap-2">
            <input v-model="form.token" class="input mono" required />
            <button type="button" class="btn-outline shrink-0" @click="generateToken">{{ t("Generate") }}</button>
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm text-text-2">{{ t("Ports") }}</label>
          <input v-model="form.ports" class="input mono" :placeholder="t('Comma-separated: 30000, 30100-30199')" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm text-text-2">{{ t("Domains") }}</label>
            <input v-model="form.domains" class="input mono" :placeholder="t('Comma-separated domains')" />
          </div>
          <div>
            <label class="mb-1 block text-sm text-text-2">{{ t("Subdomains") }}</label>
            <input v-model="form.subdomains" class="input mono" :placeholder="t('Comma-separated subdomains')" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm text-text-2">{{ t("Notes") }}</label>
          <input v-model="form.comment" class="input" />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.enable" type="checkbox" class="h-4 w-4 accent-accent" />
          {{ t("Enabled") }}
        </label>
      </div>
      <p v-if="error" class="mt-3 text-sm text-danger">{{ error }}</p>
      <div class="mt-5 flex justify-end gap-2">
        <button type="button" class="btn-outline" @click="emit('close')">{{ t("Cancel") }}</button>
        <button class="btn-primary" :disabled="busy">{{ t("Save") }}</button>
      </div>
    </form>
  </div>
</template>
