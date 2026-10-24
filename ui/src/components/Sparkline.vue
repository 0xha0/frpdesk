<script setup lang="ts">
// a stat tile's trend: a thin line in a quiet hue, the last point in the accent
import { computed } from "vue";
const props = defineProps<{ values: number[]; width?: number; height?: number }>();
const w = computed(() => props.width ?? 96);
const h = computed(() => props.height ?? 28);
const max = computed(() => Math.max(1, ...props.values));
const pts = computed(() => props.values.map((v, i) => [
  props.values.length > 1 ? (i / (props.values.length - 1)) * (w.value - 4) + 2 : w.value / 2,
  h.value - 3 - (v / max.value) * (h.value - 6),
] as const));
const d = computed(() => pts.value.map(([x, y], i) => `${i ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`).join(" "));
const last = computed(() => pts.value[pts.value.length - 1]);
</script>

<template>
  <svg :width="w" :height="h" class="block" aria-hidden="true">
    <path v-if="values.length > 1" :d="d" fill="none" stroke="var(--text-3)" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round" />
    <circle v-if="last" :cx="last[0]" :cy="last[1]" r="2.5" fill="var(--accent)" />
  </svg>
</template>
