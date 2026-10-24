<script setup lang="ts">
import { toasts } from "./toast";
</script>

<template>
  <router-view />
  <div class="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex flex-col items-center gap-2 px-4">
    <transition-group name="toast">
      <div v-for="t in toasts" :key="t.id"
        class="pointer-events-auto rounded-lg px-4 py-2 text-sm shadow-lg"
        :class="t.kind === 'error' ? 'bg-danger-soft text-danger' : 'bg-text text-bg'">
        {{ t.text }}
      </div>
    </transition-group>
  </div>
</template>

<style>
.toast-enter-active, .toast-leave-active { transition: opacity .2s, transform .2s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateY(8px); }
</style>
