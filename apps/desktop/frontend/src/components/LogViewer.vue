<script setup lang="ts">
/**
 * Virtualized live log tail. Works for one container, all containers of a pod,
 * or every pod of a workload (each line is tagged with its source).
 *
 * Reading aids: period filter, error/warning highlight, "only problems",
 * JSON lines shown as "level message key=value", regex search, line wrap.
 */
import { tr } from '@/lib/i18n'
import { computed, nextTick, ref, toRef, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { stream } from '../../wailsjs/go/models'
import { SaveLogs, SaveText } from '../../wailsjs/go/api/StreamAPI'
import { useLogStream } from '@/composables/useLogStream'
import type { LogLine } from '@/lib/types'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'
import Icon from './Icon.vue'

const props = defineProps<{
  cluster: string
  namespace: string
  targets: { pod: string; container: string }[]
  previous?: boolean
  note?: string // e.g. "mostrando 30 de 48 streams"
}>()

const tail = ref(500)
const since = ref(0) // seconds; 0 = no time limit
const previous = ref(props.previous ?? false) // logs of the crashed (previous) run
const timestamps = ref(false)
const search = ref('')
const regex = ref(false)
const follow = ref(true)
const wrap = ref(false)
const pretty = ref(true) // JSON lines as "level message key=value"
const onlyProblems = ref(false)
const source = ref('') // '' = all sources

const PERIODS: [number, string][] = [[0, 'tudo'], [300, '5 min'], [900, '15 min'], [3600, '1 h'], [21600, '6 h'], [86400, '24 h']]

const opts = computed(() => stream.LogOptions.createFrom({
  namespace: props.namespace,
  targets: props.targets,
  tailLines: tail.value, sinceSeconds: since.value,
  timestamps: timestamps.value, previous: previous.value, follow: !previous.value,
}))
const { lines, status, error } = useLogStream(toRef(props, 'cluster'), opts)

const pods = computed(() => new Set(props.targets.map((t) => t.pod)))
const multiPod = computed(() => pods.value.size > 1)
const multi = computed(() => props.targets.length > 1)
const keyOf = (l: { p: string; c: string }) => `${l.p}/${l.c}`
const label = (l: { p: string; c: string }) => (multiPod.value ? keyOf(l) : l.c)

// ---- Levels and JSON ----------------------------------------------------------
type Level = 'error' | 'warn' | ''
const ERR = /\b(ERROR|ERR|FATAL|PANIC|CRITICAL|CRIT|EXCEPTION|Traceback)\b|"(level|severity|lvl)"\s*:\s*"(error|err|fatal|panic|critical|crit)"/i
const WARN = /\b(WARN|WARNING)\b|"(level|severity|lvl)"\s*:\s*"(warn|warning)"/i
const levelCache = new WeakMap<LogLine, Level>()
function levelOf(l: LogLine): Level {
  let v = levelCache.get(l)
  if (v === undefined) {
    v = ERR.test(l.t) ? 'error' : WARN.test(l.t) ? 'warn' : ''
    levelCache.set(l, v)
  }
  return v
}

// A JSON line becomes "LEVEL message key=value ..." (only when it parses).
const prettyCache = new WeakMap<LogLine, string>()
const MSG_KEYS = ['msg', 'message', 'log', 'event']
const LVL_KEYS = ['level', 'severity', 'lvl']
const TIME_KEYS = ['time', 'ts', 'timestamp', '@timestamp']
function textOf(l: LogLine): string {
  if (!pretty.value) return l.t
  let v = prettyCache.get(l)
  if (v !== undefined) return v
  v = l.t
  const start = l.t.indexOf('{')
  if (start >= 0 && start < 40 && l.t.trimEnd().endsWith('}')) {
    try {
      const obj = JSON.parse(l.t.slice(start)) as Record<string, unknown>
      if (obj && typeof obj === 'object' && !Array.isArray(obj)) {
        const pick = (keys: string[]) => keys.find((k) => k in obj)
        const lk = pick(LVL_KEYS), mk = pick(MSG_KEYS), tk = pick(TIME_KEYS)
        const rest = Object.entries(obj)
          .filter(([k]) => k !== lk && k !== mk && k !== tk)
          .map(([k, x]) => `${k}=${typeof x === 'string' ? x : JSON.stringify(x)}`)
        const prefix = l.t.slice(0, start)
        v = [prefix.trim(), tk ? String(obj[tk]) : '', lk ? String(obj[lk]).toUpperCase() : '', mk ? String(obj[mk]) : '', ...rest]
          .filter(Boolean).join('  ')
      }
    } catch { /* not JSON: show as is */ }
  }
  prettyCache.set(l, v)
  return v
}

// ---- Filtering ----------------------------------------------------------------
const matcher = computed<{ test: (s: string) => boolean; error?: string } | null>(() => {
  const q = search.value.trim()
  if (!q) return null
  if (regex.value) {
    try {
      const re = new RegExp(q, 'i')
      return { test: (s) => re.test(s) }
    } catch (e) {
      return { test: () => true, error: (e as Error).message }
    }
  }
  const lower = q.toLowerCase()
  return { test: (s) => s.toLowerCase().includes(lower) }
})

const visible = computed(() => {
  let all: LogLine[] = lines.value
  if (source.value) all = all.filter((l) => keyOf(l) === source.value)
  if (onlyProblems.value) all = all.filter((l) => levelOf(l) !== '')
  const m = matcher.value
  return m ? all.filter((l) => m.test(l.t)) : all
})
const problemCount = computed(() => lines.value.reduce((n, l) => n + (levelOf(l) ? 1 : 0), 0))

// ---- Virtual list (rows are measured, so wrapped lines take their height) --------
const scrollEl = ref<HTMLElement | null>(null)
const virtualizer = useVirtualizer(computed(() => ({
  count: visible.value.length,
  getScrollElement: () => scrollEl.value,
  estimateSize: () => 18,
  overscan: 30,
})))
const measure = (el: unknown) => { if (el instanceof Element) virtualizer.value.measureElement(el) }
watch([wrap, pretty], () => nextTick(() => virtualizer.value.measure()))

watch(visible, async () => {
  if (!follow.value || !visible.value.length) return
  await nextTick()
  virtualizer.value.scrollToIndex(visible.value.length - 1, { align: 'end' })
})

function onScroll() {
  const el = scrollEl.value!
  follow.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40 // re-arm at bottom
}

const palette = ['text-sky-400', 'text-amber-400', 'text-emerald-400', 'text-fuchsia-400', 'text-rose-400', 'text-teal-300', 'text-violet-300']
const colorIndex = computed(() => new Map(props.targets.map((t, i) => [`${t.pod}/${t.container}`, i])))
const colorOf = (l: { p: string; c: string }) => palette[(colorIndex.value.get(keyOf(l)) ?? 0) % palette.length]
const lineClass = (l: LogLine) => {
  const lv = levelOf(l)
  return lv === 'error' ? 'bg-rose-500/10 text-rose-200' : lv === 'warn' ? 'bg-amber-500/10 text-amber-100' : ''
}

// ---- Download ----------------------------------------------------------------
type SaveKind = 'full' | 'screen'
const saveMenu = ref<{ x: number; y: number } | null>(null)
const saving = ref(false)
const saved = ref('') // feedback after a download
const saveItems = computed<MenuItem<SaveKind>[]>(() => [
  { value: 'full', label: source.value ? tr('Logs completos desta fonte') : tr('Logs completos (direto do cluster)') },
  { value: 'screen', label: tr('O que está na tela ({a} linhas)', { a: visible.value.length }) },
])

function fileName() {
  const d = new Date()
  const p2 = (n: number) => String(n).padStart(2, '0')
  const stamp = `${d.getFullYear()}-${p2(d.getMonth() + 1)}-${p2(d.getDate())}_${p2(d.getHours())}${p2(d.getMinutes())}`
  const t = source.value ? props.targets.find((x) => keyOf({ p: x.pod, c: x.container }) === source.value) : props.targets[0]
  const base = !t ? 'logs' : multi.value && !source.value ? t.pod.replace(/-[a-z0-9]{5}$/, '') : `${t.pod}_${t.container}`
  return `${base}${previous.value ? '_anterior' : ''}_${stamp}.log`
}

async function save(kind: SaveKind) {
  saving.value = true
  saved.value = ''
  savedErr.value = false
  try {
    if (kind === 'full') {
      const targets = source.value ? props.targets.filter((t) => `${t.pod}/${t.container}` === source.value) : props.targets
      const r = await SaveLogs(props.cluster, stream.LogOptions.createFrom({
        namespace: props.namespace, targets, tailLines: 0, sinceSeconds: since.value,
        timestamps: timestamps.value, previous: previous.value, follow: false,
      }), fileName())
      if (r.path) saved.value = `Salvo: ${r.path} (${r.lines} linhas)`
    } else {
      const text = visible.value.map((l) => (multi.value ? `[${label(l)}] ${textOf(l)}` : textOf(l))).join('\n') + '\n'
      const path = await SaveText(fileName(), text)
      if (path) saved.value = `Salvo: ${path} (${visible.value.length} linhas)`
    }
  } catch (e) {
    saved.value = tr('Erro ao salvar: {a}', { a: e })
    savedErr.value = true
  } finally {
    saving.value = false
  }
}

const statusText = (s: string) => ({ idle: tr('parado'), streaming: tr('ao vivo'), ended: tr('encerrado'), error: tr('erro') } as Record<string, string>)[s] ?? s
const savedErr = ref(false)
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-zinc-800 px-3 py-1.5 text-xs text-zinc-400">
      <div class="flex items-center rounded bg-zinc-900 pr-1" :class="matcher?.error && 'ring-1 ring-rose-600'">
        <input v-model="search" :placeholder="regex ? tr('Expressão regular…') : tr('Buscar nos logs…')" class="w-52 bg-transparent px-2 py-1 text-zinc-200 outline-none" :title="matcher?.error" />
        <button class="rounded px-1 font-mono text-[11px]" :class="regex ? 'bg-sky-600/30 text-sky-200' : 'text-zinc-500 hover:text-zinc-200'" :title="tr('Expressão regular')" @click="regex = !regex">.*</button>
      </div>
      <select v-if="multi" v-model="source" class="max-w-64 rounded bg-zinc-900 px-1.5 py-1 text-zinc-200 outline-none" :title="tr('Fonte')">
        <option value="">{{ tr('Todas as fontes ({n})', { n: targets.length }) }}</option>
        <option v-for="t in targets" :key="`${t.pod}/${t.container}`" :value="`${t.pod}/${t.container}`">
          {{ multiPod ? `${t.pod}/${t.container}` : t.container }}
        </option>
      </select>
      <select v-model.number="since" class="rounded bg-zinc-900 px-1.5 py-1 text-zinc-200 outline-none" :title="tr('Período')">
        <option v-for="[s, l] in PERIODS" :key="s" :value="s">{{ s ? tr('últimos {a}', { a: l }) : tr('todo o período') }}</option>
      </select>
      <label :title="tr('Linhas iniciais por fonte')">{{ tr('últimas') }} <input v-model.number="tail" type="number" min="0" class="w-16 rounded bg-zinc-900 px-1" /></label>
      <button
        class="rounded px-1.5 py-0.5" :class="onlyProblems ? 'bg-rose-500/20 text-rose-200' : 'hover:bg-zinc-800 hover:text-zinc-100'"
        :title="tr('Mostrar só linhas de erro e aviso')" @click="onlyProblems = !onlyProblems"
      >{{ tr('erros e avisos') }} <span class="tabular-nums" :class="problemCount ? 'text-rose-300' : 'text-zinc-600'">{{ problemCount }}</span></button>
      <label :title="tr('Linhas JSON viram: nível, mensagem e campos')"><input v-model="pretty" type="checkbox" /> {{ tr('JSON legível') }}</label>
      <label><input v-model="wrap" type="checkbox" /> {{ tr('quebrar linhas') }}</label>
      <label><input v-model="timestamps" type="checkbox" /> {{ tr('horário') }}</label>
      <label :title="tr('Logs da execução anterior (antes do último reinício)')"><input v-model="previous" type="checkbox" /> {{ tr('anterior') }}</label>
      <label><input v-model="follow" type="checkbox" /> {{ tr('seguir') }}</label>
      <span v-if="note" class="text-amber-300/90">{{ note }}</span>
      <span v-if="saved" class="max-w-96 truncate select-text" :class="savedErr ? 'text-rose-400' : 'text-emerald-400/90'" :title="saved">{{ saved }}</span>
      <span class="ml-auto">{{ tr('{n} linhas', { n: visible.length }) }} · {{ statusText(status) }}</span>
      <button
        class="flex items-center gap-1 rounded px-1.5 py-0.5 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100 disabled:opacity-50"
        :disabled="saving" :title="tr('Baixar logs')"
        @click="saveMenu = { x: $event.clientX, y: $event.clientY }"
      ><Icon name="download" :size="13" />{{ saving ? 'Baixando…' : 'Baixar' }}</button>
      <span v-if="error" class="truncate text-rose-400" :title="error">{{ error }}</span>
    </div>
    <div ref="scrollEl" class="min-h-0 flex-1 overflow-auto font-mono text-[12px] leading-[18px]" @scroll.passive="onScroll">
      <div class="relative" :style="{ height: `${virtualizer.getTotalSize()}px` }">
        <div
          v-for="v in virtualizer.getVirtualItems()" :key="v.index" :ref="measure" :data-index="v.index"
          class="absolute inset-x-0 px-3 select-text" :class="[wrap ? 'break-all whitespace-pre-wrap' : 'whitespace-pre', lineClass(visible[v.index])]"
          :style="{ transform: `translateY(${v.start}px)` }"
        >
          <span v-if="multi" :class="colorOf(visible[v.index])">[{{ label(visible[v.index]) }}] </span>{{ textOf(visible[v.index]) }}
        </div>
      </div>
      <p v-if="!visible.length && lines.length" class="px-3 py-6 text-center text-zinc-500">{{ tr('Nenhuma linha com esses filtros.') }}</p>
    </div>
    <ContextMenu v-if="saveMenu" :x="saveMenu.x" :y="saveMenu.y" :items="saveItems" :title="tr('Baixar logs')" @select="save" @close="saveMenu = null" />
  </div>
</template>
