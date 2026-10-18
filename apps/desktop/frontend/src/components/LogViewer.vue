<script setup lang="ts">
/**
 * Virtualized live log tail. Works for one container, all containers of a pod,
 * or every pod of a workload (each line is tagged with its source).
 */
import { computed, nextTick, ref, toRef, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { stream } from '../../wailsjs/go/models'
import { useLogStream } from '@/composables/useLogStream'
import type { LogLine } from '@/lib/types'

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
      <span class="ml-auto">{{ visible.length }} linhas · {{ statusText[status] ?? status }}</span>
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
  </div>
</template>
