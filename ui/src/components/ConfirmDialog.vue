<script setup lang="ts">
import { t } from "../i18n";
defineProps<{ title: string; text?: string; confirmLabel?: string; danger?: boolean }>();
const emit = defineEmits<{ confirm: []; cancel: [] }>();
</script>

<template>
  <div class="fixed inset-0 z-40 grid place-items-end bg-black/40 sm:place-items-center sm:p-4" @click.self="emit('cancel')">
    <div class="card w-full rounded-b-none p-5 shadow-xl sm:max-w-sm sm:rounded-b-xl">
      <h3 class="text-base font-semibold">{{ title }}</h3>
      <p v-if="text" class="mt-2 text-sm text-text-2">{{ text }}</p>
      <div class="mt-5 flex justify-end gap-2">
        <button class="btn-outline" @click="emit('cancel')">{{ t("Cancel") }}</button>
        <button :class="danger ? 'btn-danger' : 'btn-primary'" @click="emit('confirm')">{{ confirmLabel ?? "OK" }}</button>
      </div>
    </div>
  </div>
</template>
