<script setup lang="ts">
// a line chart: thin lines, a wash under them, a crosshair that snaps to the nearest sample with one
// tooltip listing every series, arrow keys when focused, a table view of the same numbers
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { t } from "../i18n";
import { cleanTicks } from "../ticks";

export interface Series { name: string; color: string; values: number[] }

const props = defineProps<{
  series: Series[];
  labels: string[];        // one per x position (short, for the axis and tooltip)
  format: (v: number) => string;
  height?: number;
  base?: number;           // the axis steps in powers of this: 1024 for bytes (ticks at 1 MB, 2 MB …), else 10
}>();

const host = ref<HTMLElement | null>(null);
const width = ref(640);
const height = computed(() => props.height ?? 200);
const pad = { l: 68, r: 12, t: 12, b: 24 };
const hover = ref<number | null>(null);
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
// clean ticks: about 4 steps of a round number (round in the displayed unit: a power of `base`
// times 1, 2, 2.5, 5 …) reaching above the maximum
const ticks = computed(() => cleanTicks(max.value, props.base ?? 10));
const top = computed(() => ticks.value[ticks.value.length - 1] || max.value);
const x = (i: number) => pad.l + (n.value > 1 ? (i / (n.value - 1)) * (width.value - pad.l - pad.r) : 0);
const y = (v: number) => pad.t + (1 - v / top.value) * (height.value - pad.t - pad.b);
const path = (values: number[]) => values.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
const area = (values: number[]) => values.length ? `${path(values)} L${x(values.length - 1).toFixed(1)},${y(0)} L${x(0).toFixed(1)},${y(0)} Z` : "";
// a handful of x labels, evenly spaced
const xLabels = computed(() => {
  const want = Math.max(2, Math.min(8, Math.floor((width.value - pad.l - pad.r) / 80)));
  const step = Math.max(1, Math.ceil((n.value - 1) / (want - 1)));
  const out: number[] = [];
  for (let i = 0; i < n.value; i += step) out.push(i);
  if (out[out.length - 1] !== n.value - 1) out.push(n.value - 1);
  return out;
});

function move(e: PointerEvent) {
  if (n.value < 2) return;
  const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
  const px = e.clientX - rect.left;
  const i = Math.round(((px - pad.l) / (width.value - pad.l - pad.r)) * (n.value - 1));
  hover.value = Math.min(n.value - 1, Math.max(0, i));
}
function key(e: KeyboardEvent) {
  if (n.value === 0) return;
  if (e.key === "ArrowLeft") { hover.value = Math.max(0, (hover.value ?? n.value - 1) - 1); e.preventDefault(); }
  if (e.key === "ArrowRight") { hover.value = Math.min(n.value - 1, (hover.value ?? -1) + 1); e.preventDefault(); }
  if (e.key === "Escape") hover.value = null;
}
const tipX = computed(() => (hover.value === null ? 0 : x(hover.value) + 12 + 180 > width.value ? x(hover.value) - 12 - 180 : x(hover.value) + 12));
</script>

<template>
  <div ref="host" class="relative">
    <div class="mb-1 flex items-center justify-end gap-3 text-xs text-text-2">
      <span v-if="series.length > 1" v-for="s in series" :key="s.name" class="inline-flex items-center gap-1.5">
        <span class="inline-block h-0.5 w-4 rounded" :style="{ background: s.color }"></span>{{ s.name }}
      </span>
      <button class="btn-ghost !px-2 !py-0.5 text-xs" @click="table = !table">{{ table ? t("Chart") : t("Table") }}</button>
    </div>
    <div v-if="table" class="max-h-56 overflow-auto rounded-lg border border-line">
      <table class="w-full text-xs">
        <thead class="sticky top-0 bg-surface-2"><tr><th class="th">{{ t("Time") }}</th><th v-for="s in series" :key="s.name" class="th text-right">{{ s.name }}</th></tr></thead>
        <tbody><tr v-for="(l, i) in labels" :key="i" class="border-t border-line"><td class="td mono !py-1">{{ l }}</td><td v-for="s in series" :key="s.name" class="td mono !py-1 text-right tabular-nums">{{ format(s.values[i] ?? 0) }}</td></tr></tbody>
      </table>
    </div>
    <svg v-else :width="width" :height="height" class="block select-none touch-none outline-none focus-visible:ring-2 focus-visible:ring-accent/40 rounded"
      tabindex="0" role="img" @pointermove="move" @pointerleave="hover = null" @keydown="key">
      <g v-for="v in ticks" :key="v">
        <line :x1="pad.l" :x2="width - pad.r" :y1="y(v)" :y2="y(v)" stroke="var(--grid)" stroke-width="1" />
        <text :x="pad.l - 8" :y="y(v) + 3.5" text-anchor="end" class="fill-text-3" font-size="11" style="font-variant-numeric: tabular-nums">{{ format(v) }}</text>
      </g>
      <line :x1="pad.l" :x2="width - pad.r" :y1="y(0)" :y2="y(0)" stroke="var(--axis)" stroke-width="1" />
      <text v-for="i in xLabels" :key="i" :x="x(i)" :y="height - 6" :text-anchor="i === 0 ? 'start' : i === n - 1 ? 'end' : 'middle'" class="fill-text-3" font-size="11">{{ labels[i] }}</text>
      <template v-for="s in series" :key="s.name">
        <path :d="area(s.values)" :fill="s.color" fill-opacity="0.1" />
        <path :d="path(s.values)" fill="none" :stroke="s.color" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
      </template>
      <template v-if="hover !== null && n > 0">
        <line :x1="x(hover)" :x2="x(hover)" :y1="pad.t" :y2="y(0)" stroke="var(--axis)" stroke-width="1" />
        <circle v-for="s in series" :key="s.name" :cx="x(hover)" :cy="y(s.values[hover] ?? 0)" r="4" :fill="s.color" stroke="var(--surface)" stroke-width="2" />
      </template>
      <text v-if="n < 2" :x="(pad.l + width - pad.r) / 2" :y="height / 2" text-anchor="middle" class="fill-text-3" font-size="12">{{ t("Not enough samples yet") }}</text>
    </svg>
    <div v-if="hover !== null && !table && n > 0" class="pointer-events-none absolute top-6 z-10 min-w-[11rem] rounded-lg border border-line bg-surface px-3 py-2 text-xs shadow-lg" :style="{ left: tipX + 'px' }">
      <div class="mb-1 text-text-3">{{ labels[hover] }}</div>
      <div v-for="s in series" :key="s.name" class="flex items-center justify-between gap-3">
        <span class="inline-flex items-center gap-1.5 text-text-2"><span class="inline-block h-0.5 w-3 rounded" :style="{ background: s.color }"></span>{{ s.name }}</span>
        <span class="mono font-semibold tabular-nums">{{ format(s.values[hover] ?? 0) }}</span>
      </div>
    </div>
  </div>
</template>
