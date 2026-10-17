<script setup lang="ts">
/**
 * Single-series time chart (one y-axis, by design: CPU and memory are
 * separate charts). Requests/limits are reference lines with direct labels.
 * Hover = crosshair + tooltip. Plain SVG: no chart library in the bundle.
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { theme } from '@/lib/theme'

export interface Point { t: number; v: number }
export interface RefLine { label: string; value: number; dash: string }

const props = defineProps<{
  points: Point[]
  refs?: RefLine[]
  format: (n: number) => string
  windowMs: number
  gapMs: number // break the line when samples are further apart than this
  label: string // accessible name, e.g. "Uso de CPU"
}>()

// Validated per theme (dataviz validator): lightness band + ≥3:1 vs the panel surface.
const SERIES = computed(() => (theme.value === 'light' ? '#0284c7' : '#2f9bd8'))
const H = 150
const PAD = { l: 48, r: 72, t: 10, b: 22 }

const root = ref<HTMLElement | null>(null)
const width = ref(400)
let ro: ResizeObserver | null = null
onMounted(() => {
  ro = new ResizeObserver(([e]) => (width.value = Math.max(240, e.contentRect.width)))
  ro.observe(root.value!)
})
onBeforeUnmount(() => ro?.disconnect())

const end = computed(() => Math.max(Date.now(), props.points.at(-1)?.t ?? 0))
const start = computed(() => end.value - props.windowMs)
const visible = computed(() => props.points.filter((p) => p.t >= start.value))

function niceMax(v: number) {
  if (v <= 0) return 1
  const step = 10 ** Math.floor(Math.log10(v / 4))
  for (const m of [1, 2, 2.5, 5, 10]) if (step * m * 4 >= v) return step * m * 4
  return v
}
const yMax = computed(() => niceMax(Math.max(...visible.value.map((p) => p.v), ...(props.refs ?? []).map((r) => r.value), 0) * 1.1))
const yTicks = computed(() => [0, 1, 2, 3, 4].map((i) => (yMax.value / 4) * i))

const plotW = computed(() => width.value - PAD.l - PAD.r)
const plotH = H - PAD.t - PAD.b
const x = (t: number) => PAD.l + ((t - start.value) / props.windowMs) * plotW.value
const y = (v: number) => PAD.t + plotH - (v / yMax.value) * plotH

const path = computed(() => {
  let d = ''
  let prev = 0
  for (const p of visible.value) {
    d += `${!d || p.t - prev > props.gapMs ? 'M' : 'L'}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`
    prev = p.t
  }
  return d
})
const area = computed(() => {
  const v = visible.value
  if (v.length < 2) return ''
  return `${path.value}L${x(v.at(-1)!.t).toFixed(1)},${y(0)}L${x(v[0].t).toFixed(1)},${y(0)}Z`
})

const hhmm = (t: number) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
const xTicks = computed(() => [0, 1, 2, 3, 4].map((i) => start.value + (props.windowMs / 4) * i))

// ---- hover ----
const hover = ref<Point | null>(null)
function onMove(e: MouseEvent) {
  const rect = (e.currentTarget as SVGElement).getBoundingClientRect()
  const t = start.value + ((e.clientX - rect.left - PAD.l) / plotW.value) * props.windowMs
  let best: Point | null = null
  for (const p of visible.value) if (!best || Math.abs(p.t - t) < Math.abs(best.t - t)) best = p
  hover.value = best
}
const tipLeft = computed(() => (hover.value ? Math.min(x(hover.value.t) + 10, width.value - 150) : 0))
</script>

<template>
  <div ref="root" class="relative select-none">
    <svg :width="width" :height="H" role="img" :aria-label="label" class="block" @mousemove="onMove" @mouseleave="hover = null">
      <!-- recessive grid + y axis labels -->
      <g v-for="v in yTicks" :key="v">
        <line :x1="PAD.l" :x2="width - PAD.r" :y1="y(v)" :y2="y(v)" class="stroke-zinc-800" stroke-width="1" />
        <text :x="PAD.l - 6" :y="y(v) + 3" text-anchor="end" class="fill-zinc-500 text-[10px]">{{ format(v) }}</text>
      </g>
      <text v-for="t in xTicks" :key="t" :x="x(t)" :y="H - 6" text-anchor="middle" class="fill-zinc-500 text-[10px]">{{ hhmm(t) }}</text>

      <!-- requests / limits: neutral reference lines, labeled directly -->
      <g v-for="r in refs ?? []" :key="r.label">
        <line :x1="PAD.l" :x2="width - PAD.r" :y1="y(r.value)" :y2="y(r.value)" class="stroke-zinc-500" stroke-width="1.25" :stroke-dasharray="r.dash" />
        <text :x="width - PAD.r + 6" :y="y(r.value) + 3" class="fill-zinc-400 text-[10px]">{{ r.label }} {{ format(r.value) }}</text>
      </g>

      <path v-if="area" :d="area" :fill="SERIES" fill-opacity="0.12" />
      <path :d="path" fill="none" :stroke="SERIES" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

      <!-- latest value, labeled directly -->
      <template v-if="visible.length && !hover">
        <circle :cx="x(visible.at(-1)!.t)" :cy="y(visible.at(-1)!.v)" r="4" :fill="SERIES" class="stroke-zinc-925" stroke-width="2" />
      </template>

      <!-- crosshair -->
      <template v-if="hover">
        <line :x1="x(hover.t)" :x2="x(hover.t)" :y1="PAD.t" :y2="PAD.t + plotH" class="stroke-zinc-600" stroke-width="1" />
        <circle :cx="x(hover.t)" :cy="y(hover.v)" r="4.5" :fill="SERIES" class="stroke-zinc-925" stroke-width="2" />
      </template>
      <!-- generous hit area -->
      <rect :x="PAD.l" :y="PAD.t" :width="plotW" :height="plotH" fill="transparent" />
    </svg>

    <div
      v-if="hover" class="pointer-events-none absolute top-1 rounded-md border border-zinc-700 bg-zinc-900/95 px-2 py-1 text-[11px] shadow-lg"
      :style="{ left: `${tipLeft}px` }"
    >
      <div class="text-zinc-400">{{ new Date(hover.t).toLocaleTimeString() }}</div>
      <div class="font-mono text-zinc-100">{{ format(hover.v) }}</div>
    </div>
    <p v-if="!visible.length" class="absolute inset-0 grid place-items-center text-[11px] text-zinc-500">Coletando amostras… (a cada 15s)</p>
  </div>
</template>
