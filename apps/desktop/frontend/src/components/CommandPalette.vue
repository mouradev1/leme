<script setup lang="ts">
/**
 * ⌘K palette: jump to a cluster, a resource list, any object by name (in the
 * selected namespaces of the active cluster) or run an app command.
 * Enter opens; ⌘Enter on a pod or workload opens its logs.
 */
import { tr } from '@/lib/i18n'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { Search } from '../../wailsjs/go/api/ResourceAPI'
import type { resources } from '../../wailsjs/go/models'
import Icon from './Icon.vue'

export interface PaletteItem {
  id: string
  group: string
  label: string
  sub?: string
  icon: string
  keys?: string // shortcut hint
  run: () => void
}

const props = defineProps<{ items: PaletteItem[]; cluster: string; namespaces: string[] }>()
const emit = defineEmits<{ close: []; object: [hit: resources.SearchHit, logs: boolean] }>()

const q = ref('')
const input = ref<HTMLInputElement | null>(null)
const hits = ref<resources.SearchHit[]>([])
const searching = ref(false)
const sel = ref(0)
onMounted(() => nextTick(() => input.value?.focus()))

// Objects: ask the cluster after a short pause (lists are cached a few seconds).
let timer = 0
let seq = 0
watch(q, (v) => {
  clearTimeout(timer)
  hits.value = []
  if (!props.cluster || v.trim().length < 2) return
  timer = window.setTimeout(async () => {
    const mine = ++seq
    searching.value = true
    try {
      const r = await Search(props.cluster, props.namespaces, v)
      if (mine === seq) hits.value = r ?? []
    } catch { if (mine === seq) hits.value = [] } finally {
      if (mine === seq) searching.value = false
    }
  }, 220)
})

const LOGS_KINDS = new Set(['Pod', 'Deployment', 'StatefulSet', 'DaemonSet', 'Job'])
const KIND_ICON: Record<string, string> = { Pod: 'list', Node: 'monitor', Deployment: 'overview', Service: 'forward', Secret: 'ban' }

// Simple fuzzy: every typed character in order; prefix and word starts rank higher.
function score(text: string, query: string): number {
  const t = text.toLowerCase()
  const s = query.toLowerCase().trim()
  if (!s) return 1
  if (t.startsWith(s)) return 100
  const i = t.indexOf(s)
  if (i >= 0) return 60 - Math.min(i, 30)
  let pos = 0
  for (const ch of s) {
    pos = t.indexOf(ch, pos)
    if (pos < 0) return 0
    pos++
  }
  return 10
}

interface Row { id: string; group: string; label: string; sub?: string; icon: string; keys?: string; run: (logs: boolean) => void; logs?: boolean }
const rows = computed<Row[]>(() => {
  const s = q.value
  const local = props.items
    .map((it) => ({ it, sc: Math.max(score(it.label, s), score(`${it.group} ${it.label} ${it.sub ?? ''}`, s) - 5) }))
    .filter((x) => x.sc > 0)
    .sort((a, b) => b.sc - a.sc)
    .slice(0, s ? 30 : 60)
    .map(({ it }) => ({ ...it, run: () => it.run() }))
  const objs: Row[] = hits.value.map((h) => ({
    id: `obj:${h.kind}/${h.namespace}/${h.name}`, group: tr('Objetos'), label: h.name,
    sub: `${h.kind}${h.namespace ? ` · ${h.namespace}` : ''}`, icon: KIND_ICON[h.kind] ?? 'list',
    logs: LOGS_KINDS.has(h.kind), run: (logs: boolean) => emit('object', h, logs && LOGS_KINDS.has(h.kind)),
  }))
  // While searching, objects come first: that's usually what a name means.
  return s.trim().length >= 2 ? [...objs, ...local] : local
})
watch(rows, () => (sel.value = 0))

const grouped = computed(() => {
  const out: { group: string; items: { row: Row; index: number }[] }[] = []
  rows.value.forEach((row, index) => {
    let g = out.find((x) => x.group === row.group)
    if (!g) out.push((g = { group: row.group, items: [] }))
    g.items.push({ row, index })
  })
  return out
})

const list = ref<HTMLElement | null>(null)
function scrollToSel() {
  nextTick(() => list.value?.querySelector<HTMLElement>(`[data-i="${sel.value}"]`)?.scrollIntoView({ block: 'nearest' }))
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') { e.preventDefault(); sel.value = Math.min(rows.value.length - 1, sel.value + 1); scrollToSel() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); sel.value = Math.max(0, sel.value - 1); scrollToSel() }
  else if (e.key === 'Enter') { e.preventDefault(); pick(rows.value[sel.value], e.metaKey || e.ctrlKey) }
  else if (e.key === 'Escape') { e.preventDefault(); emit('close') }
}
function pick(r: Row | undefined, logs = false) {
  if (!r) return
  emit('close')
  r.run(logs)
}
</script>

<template>
  <div class="fixed inset-0 z-[70] flex justify-center bg-black/50 pt-[12vh]" @mousedown.self="emit('close')">
    <div class="flex max-h-[64vh] w-[640px] max-w-[94vw] flex-col self-start overflow-hidden rounded-xl border border-zinc-700/80 bg-zinc-900 shadow-2xl shadow-black/60">
      <div class="flex items-center gap-2 border-b border-zinc-800 px-3">
        <Icon name="search" :size="15" class="text-zinc-500" />
        <input
          ref="input" v-model="q" spellcheck="false" autocomplete="off"
          :placeholder="cluster ? tr('Buscar objeto, recurso, cluster ou comando em {a}…', { a: cluster }) : tr('Buscar cluster ou comando…')"
          class="h-12 min-w-0 flex-1 bg-transparent text-sm text-zinc-100 outline-none placeholder:text-zinc-600"
          @keydown="onKey"
        />
        <span v-if="searching" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
      </div>
      <div ref="list" class="min-h-0 flex-1 overflow-auto py-1">
        <template v-for="g in grouped" :key="g.group">
          <div class="px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider text-zinc-500 uppercase">{{ g.group }}</div>
          <button
            v-for="{ row, index } in g.items" :key="row.id" :data-i="index"
            class="flex w-full items-center gap-2.5 px-3 py-1.5 text-left text-[13px]"
            :class="index === sel ? 'bg-sky-500/15 text-zinc-50' : 'text-zinc-300 hover:bg-zinc-800/70'"
            @mousemove="sel = index" @click="pick(row, $event.metaKey || $event.ctrlKey)"
          >
            <Icon :name="row.icon" :size="14" class="shrink-0 text-zinc-500" />
            <span class="min-w-0 truncate">{{ row.label }}</span>
            <span v-if="row.sub" class="min-w-0 truncate text-xs text-zinc-500">{{ row.sub }}</span>
            <span v-if="row.keys" class="ml-auto shrink-0 font-mono text-[11px] text-zinc-500">{{ row.keys }}</span>
            <span v-else-if="row.logs && index === sel" class="ml-auto shrink-0 text-[11px] text-zinc-500">{{ tr('⌘↵ logs') }}</span>
          </button>
        </template>
        <p v-if="!rows.length && !searching" class="px-3 py-6 text-center text-xs text-zinc-500">
          {{ q.trim().length < 2 ? tr('Digite para buscar.') : tr('Nada encontrado.') }}
        </p>
      </div>
      <div class="flex gap-4 border-t border-zinc-800 px-3 py-1.5 text-[11px] text-zinc-500">
        <span>{{ tr('↑↓ navegar') }}</span><span>{{ tr('↵ abrir') }}</span><span>{{ tr('⌘↵ logs') }}</span><span>{{ tr('esc fechar') }}</span>
      </div>
    </div>
  </div>
</template>
