<script setup lang="ts">
// ranked magnitudes: a horizontal bar per row in one hue (the biggest first), the value at the
// bar's end, an in/out split in the row's text
import { computed } from "vue";
import { t } from "../i18n";

export interface BarRow { label: string; value: number; in?: number; out?: number }
const props = defineProps<{ rows: BarRow[]; format: (v: number) => string }>();
const sorted = computed(() => [...props.rows].sort((a, b) => b.value - a.value));
const max = computed(() => Math.max(1, ...props.rows.map((r) => r.value)));
</script>

<template>
  <div v-if="rows.length === 0" class="py-6 text-center text-sm text-text-3">{{ t("No traffic in this period.") }}</div>
  <ul v-else class="space-y-2 text-sm">
    <li v-for="r in sorted" :key="r.label">
      <div class="mb-0.5 flex items-baseline justify-between gap-3">
        <span class="mono min-w-0 truncate">{{ r.label }}</span>
        <span class="mono shrink-0 tabular-nums">{{ format(r.value) }}<span v-if="r.in !== undefined" class="text-text-3"> · {{ t("in") }} {{ format(r.in) }} · {{ t("out") }} {{ format(r.out ?? 0) }}</span></span>
      </div>
      <div class="h-2.5 w-full rounded-sm bg-surface-2" :title="`${r.label}: ${format(r.value)}`">
        <div class="h-full rounded-sm" :style="{ width: `${Math.max(1, (r.value / max) * 100)}%`, background: 'var(--series-1)' }"></div>
      </div>
    </li>
  </ul>
</template>
