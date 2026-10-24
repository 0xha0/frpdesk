<script setup lang="ts">
// part-to-whole: one horizontal bar, a segment per item (the biggest first, a tail of two or more
// folded into "Other"), 2px gaps, a legend with the value and share of each; hover shows a
// segment's figures, a click selects it (the parent drills into it). The share is printed in the
// legend only: text on a coloured segment can't keep its contrast on every hue.
import { computed, ref } from "vue";
import { t } from "../i18n";

export interface ShareItem { key: string; label: string; value: number }

const props = defineProps<{ items: ShareItem[]; format: (v: number) => string; max?: number }>();
const emit = defineEmits<{ select: [key: string] }>();
const hover = ref<string | null>(null);

const colors = ["var(--series-1)", "var(--series-2)", "var(--series-3)", "var(--series-4)", "var(--series-5)"];
const OTHER = "\u0000other";
const shown = computed(() => {
  const sorted = [...props.items].filter((i) => i.value > 0).sort((a, b) => b.value - a.value);
  const cap = props.max ?? 5;
  // a tail of one is just the next row, not an "Other" that hides it
  const head = sorted.length > cap + 1 ? sorted.slice(0, cap) : sorted;
  const tail = sorted.slice(head.length);
  const rows = head.map((i, n) => ({ ...i, color: colors[n % colors.length], other: false }));
  if (tail.length) rows.push({ key: OTHER, label: t("Other ({n})", { n: tail.length }), value: tail.reduce((s, i) => s + i.value, 0), color: "var(--text-3)", other: true });
  const total = rows.reduce((s, r) => s + r.value, 0) || 1;
  return rows.map((r) => ({ ...r, share: r.value / total }));
});
const total = computed(() => props.items.reduce((s, i) => s + i.value, 0));
const dim = (key: string) => (hover.value !== null && hover.value !== key ? 0.45 : 1);
</script>

<template>
  <div v-if="shown.length === 0" class="py-6 text-center text-sm text-text-3">{{ t("No traffic in this period.") }}</div>
  <template v-else>
    <div class="flex h-6 w-full gap-0.5">
      <template v-for="r in shown" :key="r.key">
        <button v-if="!r.other" type="button"
          class="h-full min-w-[3px] rounded-sm transition-opacity first:rounded-l-md last:rounded-r-md focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
          :style="{ width: `${r.share * 100}%`, background: r.color, opacity: dim(r.key) }"
          :aria-label="`${r.label}: ${format(r.value)} (${(r.share * 100).toFixed(1)}%)`" :title="`${r.label}: ${format(r.value)} (${(r.share * 100).toFixed(1)}%)`"
          @pointerenter="hover = r.key" @pointerleave="hover = null" @focus="hover = r.key" @blur="hover = null"
          @click="emit('select', r.key)"></button>
        <div v-else class="h-full min-w-[3px] rounded-sm last:rounded-r-md transition-opacity" :style="{ width: `${r.share * 100}%`, background: r.color, opacity: dim(r.key) }"
          :title="`${r.label}: ${format(r.value)} (${(r.share * 100).toFixed(1)}%)`" @pointerenter="hover = r.key" @pointerleave="hover = null"></div>
      </template>
    </div>
    <ul class="mt-3 divide-y divide-line text-sm">
      <li v-for="r in shown" :key="r.key" @pointerenter="hover = r.key" @pointerleave="hover = null">
        <button v-if="!r.other" type="button" class="flex w-full items-center gap-2.5 py-1.5 text-left text-text-2 hover:text-text" @click="emit('select', r.key)">
          <span class="inline-block h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: r.color }"></span>
          <span class="mono min-w-0 flex-1 truncate">{{ r.label }}</span>
          <span class="mono tabular-nums">{{ format(r.value) }}</span>
          <span class="mono w-12 text-right tabular-nums text-text-3">{{ (r.share * 100).toFixed(1) }}%</span>
        </button>
        <div v-else class="flex w-full items-center gap-2.5 py-1.5 text-text-2">
          <span class="inline-block h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: r.color }"></span>
          <span class="mono min-w-0 flex-1 truncate">{{ r.label }}</span>
          <span class="mono tabular-nums">{{ format(r.value) }}</span>
          <span class="mono w-12 text-right tabular-nums text-text-3">{{ (r.share * 100).toFixed(1) }}%</span>
        </div>
      </li>
      <li class="flex items-center gap-2.5 py-1.5 text-xs text-text-3"><span class="flex-1">{{ t("Total") }}</span><span class="mono tabular-nums">{{ format(total) }}</span><span class="w-12"></span></li>
    </ul>
  </template>
</template>
