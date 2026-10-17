<script setup lang="ts">
/** Cluster overview: nodes, pod phases, live usage and recent warnings. */
import { computed, ref, toRef } from 'vue'
import { storeToRefs } from 'pinia'
import type { resources } from '../../wailsjs/go/models'
import { useResourceWatch } from '@/composables/useResourceWatch'
import { useMetrics } from '@/composables/useMetrics'
import { useClusters } from '@/stores/clusters'
import type { NavigateTarget } from '@/lib/types'
import { bytes, cell, cpu } from '@/lib/format'
import { statusTone, toneClass } from '@/lib/status'

const props = defineProps<{ cluster: string }>()
const emit = defineEmits<{ navigate: [NavigateTarget] }>()
const { server, selectedNamespaces } = storeToRefs(useClusters())

const cl = toRef(props, 'cluster')
const ns = computed(() => (selectedNamespaces.value.length === 1 ? selectedNamespaces.value[0] : ''))
const noQuery = ref<resources.Query>({ namespace: '', labelSelector: '', fieldSelector: '' })

const nodes = useResourceWatch(cl, ref({ group: '', version: 'v1', resource: 'nodes', namespaced: false }), noQuery)
const pods = useResourceWatch(cl, ref({ group: '', version: 'v1', resource: 'pods', namespaced: true }),
  computed(() => ({ namespace: ns.value, labelSelector: '', fieldSelector: '' })))
const warnings = useResourceWatch(cl, ref({ group: '', version: 'v1', resource: 'events', namespaced: true }),
  computed(() => ({ namespace: ns.value, labelSelector: '', fieldSelector: 'type=Warning' })))
const { usage, available } = useMetrics(cl, ref('nodes'), ref(''))

const colIdx = (cols: { name: string }[], name: string) => cols.findIndex((c) => c.name.toLowerCase() === name)

const nodeReady = computed(() => {
  const i = colIdx(nodes.columns.value, 'status')
  return nodes.rows.value.filter((r) => /^Ready/.test(cell(r.cells[i]))).length
})
const podPhases = computed(() => {
  const i = colIdx(pods.columns.value, 'status')
  const m = new Map<string, number>()
  for (const r of pods.rows.value) {
    const s = cell(r.cells[i]) || 'Unknown'
    m.set(s, (m.get(s) ?? 0) + 1)
  }
  return [...m].sort((a, b) => b[1] - a[1])
})
const totals = computed(() => {
  let c = 0, mem = 0
  for (const u of usage.value.values()) { c += u.cpu; mem += u.memory }
  return { cpu: c, mem }
})
const nodeUsage = computed(() =>
  nodes.rows.value.map((r) => ({ name: r.name, u: usage.value.get(`/${r.name}`) })).sort((a, b) => (b.u?.cpu ?? 0) - (a.u?.cpu ?? 0)))
const maxCpu = computed(() => Math.max(1, ...nodeUsage.value.map((n) => n.u?.cpu ?? 0)))

const recentWarnings = computed(() => {
  const cols = warnings.columns.value
  const [iReason, iObj, iMsg, iSeen] = ['reason', 'object', 'message', 'last seen'].map((n) => colIdx(cols, n))
  return [...warnings.rows.value]
    .sort((a, b) => (b.created ? Date.parse(b.created) : 0) - (a.created ? Date.parse(a.created) : 0))
    .slice(0, 50)
    .map((r) => ({ uid: r.uid, ns: r.namespace, reason: cell(r.cells[iReason]), obj: cell(r.cells[iObj]), msg: cell(r.cells[iMsg]), seen: cell(r.cells[iSeen]) }))
})
</script>

<template>
  <div class="h-full overflow-auto px-6 py-5">
    <div class="mb-5 flex items-baseline gap-3">
      <h1 class="text-lg font-semibold text-zinc-100">{{ cluster }}</h1>
      <span class="text-xs text-zinc-500">{{ server?.version }} · {{ server?.server }}</span>
      <span v-if="ns" class="text-xs text-sky-300">namespace: {{ ns }}</span>
    </div>

    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <button class="card text-left" @click="emit('navigate', { kind: 'Node' })">
        <div class="card-label">Nodes</div>
        <div class="card-value">{{ nodeReady }}<span class="text-base text-zinc-500"> / {{ nodes.rows.value.length }}</span></div>
        <div class="text-[11px] text-zinc-500">prontos</div>
      </button>
      <button class="card text-left" @click="emit('navigate', { kind: 'Pod' })">
        <div class="card-label">Pods</div>
        <div class="card-value">{{ pods.rows.value.length }}</div>
        <div class="truncate text-[11px] text-zinc-500">{{ podPhases.slice(0, 3).map(([s, n]) => `${n} ${s}`).join(' · ') }}</div>
      </button>
      <div class="card">
        <div class="card-label">CPU em uso</div>
        <div class="card-value">{{ available ? cpu(totals.cpu) : '—' }}</div>
        <div class="text-[11px] text-zinc-500">{{ available ? 'soma dos nodes' : 'metrics-server ausente' }}</div>
      </div>
      <div class="card">
        <div class="card-label">Memória em uso</div>
        <div class="card-value">{{ available ? bytes(totals.mem) : '—' }}</div>
        <div class="text-[11px] text-zinc-500">working set</div>
      </div>
    </div>

    <div class="mt-5 grid gap-4 xl:grid-cols-[1fr_1.4fr]">
      <section class="card">
        <h2 class="card-label mb-3">Uso por node</h2>
        <div v-if="!available" class="text-xs text-zinc-500">Instale o metrics-server para ver uso em tempo real.</div>
        <ul v-else class="space-y-2.5">
          <li v-for="n in nodeUsage" :key="n.name" class="text-xs">
            <div class="mb-1 flex justify-between gap-2">
              <button class="truncate text-zinc-300 hover:text-sky-300" @click="emit('navigate', { kind: 'Node', search: n.name })">{{ n.name }}</button>
              <span class="shrink-0 font-mono text-zinc-500">{{ n.u ? `${cpu(n.u.cpu)} · ${bytes(n.u.memory)}` : '—' }}</span>
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-zinc-800">
              <div class="h-full rounded-full bg-sky-500/80 transition-all" :style="{ width: `${((n.u?.cpu ?? 0) / maxCpu) * 100}%` }" />
            </div>
          </li>
        </ul>
        <h2 class="card-label mt-5 mb-2">Pods por status</h2>
        <div class="flex flex-wrap gap-1.5">
          <span v-for="[s, n] in podPhases" :key="s" class="rounded-md bg-zinc-800/80 px-2 py-1 text-xs">
            <span :class="toneClass[statusTone(s)]">{{ s }}</span> <span class="text-zinc-400 tabular-nums">{{ n }}</span>
          </span>
        </div>
      </section>

      <section class="card min-w-0">
        <h2 class="card-label mb-3 flex items-center gap-2">
          Warnings recentes <span class="rounded-full bg-amber-500/15 px-1.5 text-amber-300">{{ warnings.rows.value.length }}</span>
        </h2>
        <p v-if="!recentWarnings.length" class="text-xs text-zinc-500">Nenhum warning no momento.</p>
        <ul class="max-h-[420px] space-y-1.5 overflow-auto">
          <li v-for="w in recentWarnings" :key="w.uid" class="rounded-md border-l-2 border-amber-500/70 bg-zinc-900/70 px-3 py-2 text-xs">
            <div class="flex gap-2">
              <span class="font-medium text-amber-300">{{ w.reason }}</span>
              <span class="truncate text-zinc-400">{{ w.ns }}/{{ w.obj }}</span>
              <span class="ml-auto shrink-0 text-zinc-600">{{ w.seen }}</span>
            </div>
            <p class="mt-0.5 break-words text-zinc-400">{{ w.msg }}</p>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";
.card { @apply rounded-xl border border-zinc-800/80 bg-zinc-900/50 p-4 transition hover:border-zinc-700; }
.card-label { @apply text-[11px] font-semibold tracking-wider text-zinc-500 uppercase; }
.card-value { @apply mt-1 text-2xl font-semibold text-zinc-100 tabular-nums; }
</style>
