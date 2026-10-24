<script setup lang="ts">
// grouped columns (one group per day, one column per series): <= 24px thick, rounded at the top,
// square on the baseline, a 2px gap between neighbours; each column is its own hover target
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { t } from "../i18n";
import type { Series } from "./LineChart.vue";
import { cleanTicks } from "../ticks";

const props = defineProps<{ series: Series[]; labels: string[]; format: (v: number) => string; height?: number; base?: number }>();

const host = ref<HTMLElement | null>(null);
const width = ref(640);
const height = computed(() => props.height ?? 200);
const pad = computed(() => ({ l: width.value < 480 ? 56 : 68, r: 12, t: 12, b: 24 }));
const hover = ref<{ g: number; s: number } | null>(null);
const table = ref(false);
let ro: ResizeObserver | null = null;
onMounted(() => {
  ro = new ResizeObserver((e) => { width.value = Math.max(240, Math.floor(e[0].contentRect.width)); });
  if (host.value) ro.observe(host.value);
});
onBeforeUnmount(() => ro?.disconnect());

const n = computed(() => props.labels.length);
const max = computed(() => {
  let m = 0;
  for (const s of props.series) for (const v of s.values) if (v > m) m = v;
  return m || 1;
});
const ticks = computed(() => cleanTicks(max.value, props.base ?? 10));
const top = computed(() => ticks.value[ticks.value.length - 1] || max.value);
const plotW = computed(() => width.value - pad.value.l - pad.value.r);
const slot = computed(() => (n.value ? plotW.value / n.value : plotW.value));
const bar = computed(() => Math.min(24, Math.max(4, (slot.value * 0.7 - 2 * (props.series.length - 1)) / Math.max(1, props.series.length))));
const groupW = computed(() => bar.value * props.series.length + 2 * (props.series.length - 1));
const x = (g: number, s: number) => pad.value.l + g * slot.value + (slot.value - groupW.value) / 2 + s * (bar.value + 2);
const y = (v: number) => pad.value.t + (1 - v / top.value) * (height.value - pad.value.t - pad.value.b);
const base = computed(() => y(0));
// a column with its top corners rounded (4px) and a square base
function rect(g: number, s: number, v: number): string {
  const x0 = x(g, s), w = bar.value, y0 = y(v), r = Math.min(4, w / 2, Math.max(0, base.value - y0));
  if (base.value - y0 < 0.5) return "";
  return `M${x0},${base.value} V${y0 + r} Q${x0},${y0} ${x0 + r},${y0} H${x0 + w - r} Q${x0 + w},${y0} ${x0 + w},${y0 + r} V${base.value} Z`;
}
const tip = computed(() => {
  if (!hover.value) return null;
  const { g, s } = hover.value;
  const left = x(g, s) + bar.value + 10;
  return { g, s, left: left + 180 > width.value ? x(g, s) - 190 : left };
});
</script>

<template>
  <div ref="host" class="relative">
    <div class="mb-1 flex items-center justify-end gap-3 text-xs text-text-2">
      <span v-if="series.length > 1" v-for="s in series" :key="s.name" class="inline-flex items-center gap-1.5">
        <span class="inline-block h-2.5 w-2.5 rounded-sm" :style="{ background: s.color }"></span>{{ s.name }}
      </span>
      <button class="btn-ghost !px-2 !py-0.5 text-xs" @click="table = !table">{{ table ? t("Chart") : t("Table") }}</button>
    </div>
    <div v-if="table" class="max-h-56 overflow-auto rounded-lg border border-line">
      <table class="w-full text-xs">
        <thead class="sticky top-0 bg-surface-2"><tr><th class="th">{{ t("Day") }}</th><th v-for="s in series" :key="s.name" class="th text-right">{{ s.name }}</th></tr></thead>
        <tbody><tr v-for="(l, i) in labels" :key="i" class="border-t border-line"><td class="td mono !py-1">{{ l }}</td><td v-for="s in series" :key="s.name" class="td mono !py-1 text-right tabular-nums">{{ format(s.values[i] ?? 0) }}</td></tr></tbody>
      </table>
    </div>
    <svg v-else :width="width" :height="height" class="block select-none" role="img" @pointerleave="hover = null">
      <g v-for="v in ticks" :key="v">
        <line :x1="pad.l" :x2="width - pad.r" :y1="y(v)" :y2="y(v)" stroke="var(--grid)" stroke-width="1" />
        <text :x="pad.l - 8" :y="y(v) + 3.5" text-anchor="end" class="fill-text-3" font-size="11" style="font-variant-numeric: tabular-nums">{{ format(v) }}</text>
      </g>
      <line :x1="pad.l" :x2="width - pad.r" :y1="base" :y2="base" stroke="var(--axis)" stroke-width="1" />
      <text v-for="(l, g) in labels" :key="g" :x="pad.l + g * slot + slot / 2" :y="height - 6" text-anchor="middle" class="fill-text-3" font-size="11">{{ l }}</text>
      <template v-for="(s, si) in series" :key="s.name">
        <g v-for="(v, g) in s.values" :key="g" tabindex="0" class="outline-none" @pointerenter="hover = { g, s: si }" @focus="hover = { g, s: si }" @blur="hover = null">
          <rect :x="x(g, si) - 1" :y="pad.t" :width="bar + 2" :height="base - pad.t" fill="transparent" />
          <path :d="rect(g, si, v)" :fill="s.color" :fill-opacity="hover && (hover.g !== g || hover.s !== si) ? 0.55 : 1" />
        </g>
      </template>
    </svg>
    <div v-if="tip && !table" class="pointer-events-none absolute top-6 z-10 min-w-[11rem] rounded-lg border border-line bg-surface px-3 py-2 text-xs shadow-lg" :style="{ left: tip.left + 'px' }">
      <div class="mb-1 text-text-3">{{ labels[tip.g] }}</div>
      <div v-for="(s, si) in series" :key="s.name" class="flex items-center justify-between gap-3" :class="si === tip.s ? '' : 'opacity-70'">
        <span class="inline-flex items-center gap-1.5 text-text-2"><span class="inline-block h-2.5 w-2.5 rounded-sm" :style="{ background: s.color }"></span>{{ s.name }}</span>
        <span class="mono font-semibold tabular-nums">{{ format(s.values[tip.g] ?? 0) }}</span>
      </div>
    </div>
  </div>
</template>
