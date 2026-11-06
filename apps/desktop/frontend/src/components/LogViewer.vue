<script setup lang="ts">
/**
 * Virtualized live log tail. Works for one container, all containers of a pod,
 * or every pod of a workload (each line is tagged with its source).
 */
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
const previous = ref(props.previous ?? false) // logs of the crashed (previous) run
const timestamps = ref(false)
const search = ref('')
const follow = ref(true)
const source = ref('') // '' = all sources

const opts = computed(() => stream.LogOptions.createFrom({
  namespace: props.namespace,
  targets: props.targets,
  tailLines: tail.value, sinceSeconds: 0,
  timestamps: timestamps.value, previous: previous.value, follow: !previous.value,
}))
const { lines, status, error } = useLogStream(toRef(props, 'cluster'), opts)

const pods = computed(() => new Set(props.targets.map((t) => t.pod)))
const multiPod = computed(() => pods.value.size > 1)
const multi = computed(() => props.targets.length > 1)
const keyOf = (l: { p: string; c: string }) => `${l.p}/${l.c}`
const label = (l: { p: string; c: string }) => (multiPod.value ? keyOf(l) : l.c)

const visible = computed(() => {
  const q = search.value.trim().toLowerCase()
  let all: LogLine[] = lines.value
  if (source.value) all = all.filter((l) => keyOf(l) === source.value)
  return q ? all.filter((l) => l.t.toLowerCase().includes(q)) : all
})

const scrollEl = ref<HTMLElement | null>(null)
const virtualizer = useVirtualizer(computed(() => ({
  count: visible.value.length,
  getScrollElement: () => scrollEl.value,
  estimateSize: () => 18,
  overscan: 30,
})))

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
// ---- Download ----------------------------------------------------------------
type SaveKind = 'full' | 'screen'
const saveMenu = ref<{ x: number; y: number } | null>(null)
const saving = ref(false)
const saved = ref('') // feedback after a download
const saveItems = computed<MenuItem<SaveKind>[]>(() => [
  { value: 'full', label: source.value ? 'Logs completos desta fonte' : 'Logs completos (direto do cluster)' },
  { value: 'screen', label: `O que está na tela (${visible.value.length} linhas)` },
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
  try {
    if (kind === 'full') {
      const targets = source.value ? props.targets.filter((t) => `${t.pod}/${t.container}` === source.value) : props.targets
      const r = await SaveLogs(props.cluster, stream.LogOptions.createFrom({
        namespace: props.namespace, targets, tailLines: 0, sinceSeconds: 0,
        timestamps: timestamps.value, previous: previous.value, follow: false,
      }), fileName())
      if (r.path) saved.value = `Salvo: ${r.path} (${r.lines} linhas)`
    } else {
      const text = visible.value.map((l) => (multi.value ? `[${label(l)}] ${l.t}` : l.t)).join('\n') + '\n'
      const path = await SaveText(fileName(), text)
      if (path) saved.value = `Salvo: ${path} (${visible.value.length} linhas)`
    }
  } catch (e) {
    saved.value = `Erro ao salvar: ${e}`
  } finally {
    saving.value = false
  }
}

const statusText: Record<string, string> = { idle: 'parado', streaming: 'ao vivo', ended: 'encerrado', error: 'erro' }
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex flex-wrap items-center gap-3 border-b border-zinc-800 px-3 py-1.5 text-xs text-zinc-400">
      <input v-model="search" placeholder="Buscar nos logs…" class="w-56 rounded bg-zinc-900 px-2 py-1 text-zinc-200 outline-none" />
      <select v-if="multi" v-model="source" class="max-w-64 rounded bg-zinc-900 px-1.5 py-1 text-zinc-200 outline-none" title="Fonte">
        <option value="">Todas as fontes ({{ targets.length }})</option>
        <option v-for="t in targets" :key="`${t.pod}/${t.container}`" :value="`${t.pod}/${t.container}`">
          {{ multiPod ? `${t.pod}/${t.container}` : t.container }}
        </option>
      </select>
      <label title="Linhas iniciais por fonte">últimas <input v-model.number="tail" type="number" min="0" class="w-16 rounded bg-zinc-900 px-1" /></label>
      <label><input v-model="timestamps" type="checkbox" /> horário</label>
      <label title="Logs da execução anterior (antes do último reinício)"><input v-model="previous" type="checkbox" /> anterior</label>
      <label><input v-model="follow" type="checkbox" /> seguir</label>
      <span v-if="note" class="text-amber-300/90">{{ note }}</span>
      <span v-if="saved" class="max-w-96 truncate select-text" :class="saved.startsWith('Erro') ? 'text-rose-400' : 'text-emerald-400/90'" :title="saved">{{ saved }}</span>
      <span class="ml-auto">{{ visible.length }} linhas · {{ statusText[status] ?? status }}</span>
      <button
        class="flex items-center gap-1 rounded px-1.5 py-0.5 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100 disabled:opacity-50"
        :disabled="saving" title="Baixar logs"
        @click="saveMenu = { x: $event.clientX, y: $event.clientY }"
      ><Icon name="download" :size="13" />{{ saving ? 'Baixando…' : 'Baixar' }}</button>
      <span v-if="error" class="truncate text-rose-400" :title="error">{{ error }}</span>
    </div>
    <div ref="scrollEl" class="min-h-0 flex-1 overflow-auto font-mono text-[12px] leading-[18px]" @scroll.passive="onScroll">
      <div class="relative" :style="{ height: `${virtualizer.getTotalSize()}px` }">
        <div
          v-for="v in virtualizer.getVirtualItems()" :key="v.index"
          class="absolute inset-x-0 whitespace-pre px-3"
          :style="{ transform: `translateY(${v.start}px)` }"
        >
          <span v-if="multi" :class="colorOf(visible[v.index])">[{{ label(visible[v.index]) }}] </span>{{ visible[v.index].t }}
        </div>
      </div>
    </div>
    <ContextMenu v-if="saveMenu" :x="saveMenu.x" :y="saveMenu.y" :items="saveItems" title="Baixar logs" @select="save" @close="saveMenu = null" />
  </div>
</template>
