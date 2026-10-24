<script setup lang="ts">
// part-to-whole: one horizontal bar, a segment per item (the biggest first, the tail folded into
// "Other"), 2px gaps, a legend with the value and share of each; hover shows a segment's figures,
// a click selects it (the parent drills into it)
import { computed, ref } from "vue";
import { t } from "../i18n";

export interface ShareItem { key: string; label: string; value: number }

const props = defineProps<{ items: ShareItem[]; format: (v: number) => string; selected?: string; max?: number }>();
const emit = defineEmits<{ select: [key: string] }>();
const hover = ref<string | null>(null);

const colors = ["var(--series-1)", "var(--series-2)", "var(--series-3)", "var(--series-4)", "var(--series-5)"];
const shown = computed(() => {
  const sorted = [...props.items].filter((i) => i.value > 0).sort((a, b) => b.value - a.value);
  const cap = props.max ?? 5;
  const head = sorted.slice(0, cap);
  const tail = sorted.slice(cap);
  const rows = head.map((i, n) => ({ ...i, color: colors[n % colors.length], other: false }));
  if (tail.length) rows.push({ key: "", label: t("Other ({n})", { n: tail.length }), value: tail.reduce((s, i) => s + i.value, 0), color: "var(--text-3)", other: true });
  const total = rows.reduce((s, r) => s + r.value, 0) || 1;
  return rows.map((r) => ({ ...r, share: r.value / total }));
});
const total = computed(() => props.items.reduce((s, i) => s + i.value, 0));
</script>

<template>
  <div v-if="shown.length === 0" class="py-6 text-center text-sm text-text-3">{{ t("No traffic in this period.") }}</div>
  <template v-else>
    <div class="flex h-6 w-full gap-0.5 overflow-hidden rounded-md" role="img">
      <button v-for="r in shown" :key="r.key || '__other'" type="button"
        class="relative h-full min-w-[3px] transition-opacity focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
        :style="{ width: `${r.share * 100}%`, background: r.color, opacity: hover && hover !== (r.key || '__other') ? 0.45 : 1 }"
        :title="`${r.label}: ${format(r.value)} (${(r.share * 100).toFixed(1)}%)`"
        @pointerenter="hover = r.key || '__other'" @pointerleave="hover = null" @focus="hover = r.key || '__other'" @blur="hover = null"
        @click="!r.other && emit('select', r.key)">
        <span v-if="r.share > 0.12" class="absolute inset-0 grid place-items-center truncate px-1 text-[11px] font-medium text-white">{{ (r.share * 100).toFixed(0) }}%</span>
      </button>
    </div>
    <ul class="mt-3 divide-y divide-line text-sm">
      <li v-for="r in shown" :key="r.key || '__other'">
        <button type="button" class="flex w-full items-center gap-2.5 py-1.5 text-left hover:text-text" :class="[r.other ? 'cursor-default' : 'cursor-pointer', selected && selected === r.key ? 'text-accent' : 'text-text-2']"
          :disabled="r.other" @click="emit('select', r.key)" @pointerenter="hover = r.key || '__other'" @pointerleave="hover = null">
          <span class="inline-block h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: r.color }"></span>
          <span class="mono min-w-0 flex-1 truncate">{{ r.label }}</span>
          <span class="mono tabular-nums">{{ format(r.value) }}</span>
          <span class="mono w-12 text-right tabular-nums text-text-3">{{ (r.share * 100).toFixed(1) }}%</span>
        </button>
      </li>
      <li class="flex items-center gap-2.5 py-1.5 text-xs text-text-3"><span class="flex-1">{{ t("Total") }}</span><span class="mono tabular-nums">{{ format(total) }}</span><span class="w-12"></span></li>
    </ul>
  </template>
</template>
