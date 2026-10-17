<script setup lang="ts">
/**
 * Side panel for one object: properties (server table columns, generic for
 * any kind), labels, owner, live metrics, ConfigMap/Secret data editing and
 * the object's events (a scoped live watch on core/v1 events).
 */
import { computed, onBeforeUnmount, ref } from 'vue'
import type { resources } from '../../wailsjs/go/models'
import { useResourceWatch } from '@/composables/useResourceWatch'
import type { Column, NavigateTarget, Row, RowAction } from '@/lib/types'
import { actionLabel, actionsFor, isKind } from '@/lib/actions'
import { age, bytes, cell, cpu } from '@/lib/format'
import { STATUS_COLUMNS, statusTone, toneClass } from '@/lib/status'
import type { Usage } from '@/composables/useMetrics'
import DataSection from './DataSection.vue'
import PodDiagnoses from './PodDiagnoses.vue'
import PodSection from './PodSection.vue'
import { usePodDetails } from '@/composables/usePodDetails'
import Icon from './Icon.vue'
import { usePanelWidth } from '@/lib/resize'

const props = defineProps<{
  cluster: string
  kind: resources.Kind
  row: Row
  columns: Column[]
  usage?: Usage
  gone?: boolean // object was deleted while the panel was open
}>()
const emit = defineEmits<{
  close: []
  action: [RowAction, Row]
  navigate: [NavigateTarget]
  container: [action: 'logs' | 'previous' | 'shell', container: string, row: Row]
}>()

const now = ref(Date.now())
const t = setInterval(() => (now.value = Date.now()), 1000)
onBeforeUnmount(() => clearInterval(t))

const extraProps = computed(() =>
  props.columns
    .map((c, i) => ({ name: c.name, value: cell(props.row.cells[i]) }))
    .filter((p, i) => i > 0 && p.name !== 'Age' && p.value !== '' && p.value !== '<none>'),
)
const labels = computed(() => Object.entries(props.row.labels ?? {}))
const labelsOpen = ref(false)
const created = computed(() => (props.row.created ? Date.parse(props.row.created) : 0))

// Pods get a richer, structured view (containers, metrics, restart diagnosis).
// The drawer is keyed by uid, so the kind is fixed for its lifetime.
const isPod = isKind(props.kind, '', 'pods')
const pod = isPod
  ? usePodDetails(computed(() => props.cluster), computed(() => props.row.namespace ?? ''), computed(() => props.row.name), computed(() => props.row.rv))
  : null

const dataResource = computed(() =>
  isKind(props.kind, '', 'configmaps') ? 'configmaps' : isKind(props.kind, '', 'secrets') ? 'secrets' : null)

// Events of this object, live.
const eventsRef = ref<resources.Ref>({ group: '', version: 'v1', resource: 'events', namespaced: true })
const eventsQuery = computed<resources.Query>(() => ({
  namespace: props.kind.namespaced ? (props.row.namespace ?? '') : '',
  labelSelector: '',
  fieldSelector: `involvedObject.name=${props.row.name},involvedObject.kind=${props.kind.kind}`,
}))
const ev = useResourceWatch(computed(() => props.cluster), eventsRef, eventsQuery)
const events = computed(() => {
  const idx = (n: string) => ev.columns.value.findIndex((c) => c.name.toLowerCase() === n)
  const [iType, iReason, iMsg, iSeen] = ['type', 'reason', 'message', 'last seen'].map(idx)
  return ev.rows.value
    .map((r) => ({
      uid: r.uid,
      type: cell(r.cells[iType]),
      reason: cell(r.cells[iReason]),
      message: cell(r.cells[iMsg]),
      seen: cell(r.cells[iSeen]),
      ts: r.created ? Date.parse(r.created) : 0,
    }))
    .sort((a, b) => b.ts - a.ts)
})

// Edit/Delete live as header icons; the rest as buttons.
// Logs/Shell/Edit/Delete live as header icons; the rest as buttons below.
const HEADER_ACTIONS = ['copy', 'edit', 'delete', 'logs', 'shell']
const quick = computed(() => actionsFor(props.kind, props.row).filter((a) => !HEADER_ACTIONS.includes(a)))
const hasAction = (a: RowAction) => actionsFor(props.kind, props.row).includes(a)

// Logs/Shell container picker: hover the icon to choose a container when the
// pod has more than one. With a single container, a click goes straight in.
const containers = computed(() => {
  const d = pod?.details.value
  return d ? [...(d.initContainers ?? []), ...(d.containers ?? [])] : []
})
const runningContainers = computed(() => containers.value.filter((c) => c.state === 'running'))
const menuOpen = ref<'logs' | 'shell' | null>(null)
let menuTimer = 0
function hoverMenu(which: 'logs' | 'shell' | null) {
  clearTimeout(menuTimer)
  if (which) menuOpen.value = which
  else menuTimer = window.setTimeout(() => (menuOpen.value = null), 180) // forgiving when moving into the menu
}
function openLogs() {
  if (containers.value.length === 1) emit('container', 'logs', containers.value[0].name, props.row)
  else emit('action', 'logs', props.row) // all containers, or details not loaded yet
}
function openShell() {
  const list = runningContainers.value
  if (list.length === 1) emit('container', 'shell', list[0].name, props.row)
  else if (!containers.value.length) emit('action', 'shell', props.row)
  else menuOpen.value = 'shell'
}
function pick(action: 'logs' | 'previous' | 'shell', container: string) {
  menuOpen.value = null
  emit('container', action, container, props.row)
}
const dotClass = (c: { state: string; ready: boolean; stateReason: string }) =>
  c.state === 'running' ? (c.ready ? 'bg-emerald-500' : 'bg-amber-500')
    : c.state === 'waiting' && /BackOff|Err/.test(c.stateReason) ? 'bg-rose-500'
      : c.state === 'terminated' ? 'bg-zinc-500' : 'bg-amber-500'

// Drag the left edge to resize; double-click it to restore the default width.
const panel = usePanelWidth(isPod ? 'details-pod' : 'details', isPod ? 560 : 480)
const canDelete = computed(() => actionsFor(props.kind, props.row).includes('delete'))
</script>

<template>
  <aside class="relative flex h-full shrink-0 flex-col border-l border-zinc-800 bg-zinc-925" :style="{ width: `${panel.width.value}px` }">
    <div
      class="absolute inset-y-0 -left-1 z-20 w-2 cursor-col-resize transition-colors hover:bg-sky-500/40"
      :class="panel.dragging.value && 'bg-sky-500/60'"
      title="Arraste para redimensionar (duplo clique: tamanho padrão)"
      @mousedown="panel.start" @dblclick="panel.reset"
    />
    <!-- Header -->
    <header class="flex h-12 shrink-0 items-center gap-2 border-b border-zinc-800 px-4">
      <h2 class="min-w-0 flex-1 truncate text-[13px] text-zinc-100" :title="row.name">
        <span class="font-semibold">{{ kind.kind }}:</span> {{ row.name }}
      </h2>
      <span v-if="gone" class="rounded bg-rose-500/15 px-1.5 py-0.5 text-[10px] text-rose-300">excluído</span>
      <div v-if="hasAction('logs')" class="relative" @mouseenter="containers.length > 1 && hoverMenu('logs')" @mouseleave="hoverMenu(null)">
        <button class="icon-btn" :class="menuOpen === 'logs' && 'bg-zinc-800 text-zinc-100'" title="Logs" @click="openLogs">
          <Icon name="logs" :size="15" />
        </button>
        <div v-if="menuOpen === 'logs'" class="menu">
          <div class="menu-title">Logs</div>
          <button class="menu-item" @click="menuOpen = null; emit('action', 'logs', row)">
            <span class="w-2" /> Todos os containers <span class="ml-auto text-zinc-500">{{ containers.length }}</span>
          </button>
          <div class="my-1 border-t border-zinc-800" />
          <div v-for="c in containers" :key="c.name" class="group flex items-center">
            <button class="menu-item min-w-0 flex-1" @click="pick('logs', c.name)">
              <span class="h-2 w-2 shrink-0 rounded-[2px]" :class="dotClass(c)" />
              <span class="truncate font-mono">{{ c.name }}</span>
              <span v-if="c.init" class="rounded bg-zinc-800 px-1 text-[10px] text-zinc-400">init</span>
            </button>
            <button
              v-if="c.restartCount" class="mr-1 shrink-0 rounded px-1.5 py-0.5 text-[10px] text-amber-300 hover:bg-zinc-800"
              :title="`Logs da execução que caiu (${c.restartCount} reinícios)`" @click="pick('previous', c.name)"
            >anterior</button>
          </div>
        </div>
      </div>
      <div v-if="hasAction('shell')" class="relative" @mouseenter="runningContainers.length > 1 && hoverMenu('shell')" @mouseleave="hoverMenu(null)">
        <button class="icon-btn" :class="menuOpen === 'shell' && 'bg-zinc-800 text-zinc-100'" title="Shell" @click="openShell">
          <Icon name="terminal" :size="15" />
        </button>
        <div v-if="menuOpen === 'shell'" class="menu">
          <div class="menu-title">Shell em</div>
          <button v-for="c in runningContainers" :key="c.name" class="menu-item" @click="pick('shell', c.name)">
            <span class="h-2 w-2 shrink-0 rounded-[2px]" :class="dotClass(c)" />
            <span class="truncate font-mono">{{ c.name }}</span>
          </button>
          <p v-if="!runningContainers.length" class="px-3 py-2 text-zinc-500">Nenhum container em execução.</p>
        </div>
      </div>
      <button class="icon-btn" title="Editar YAML" @click="emit('action', 'edit', row)">
        <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor"><path d="M11.2 1.8a1.5 1.5 0 0 1 2.1 0l.9.9a1.5 1.5 0 0 1 0 2.1L5.5 13.5 2 14l.5-3.5z" /></svg>
      </button>
      <button v-if="canDelete" class="icon-btn hover:!text-rose-300" title="Excluir" @click="emit('action', 'delete', row)">
        <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor"><path d="M6 1.5h4l.5 1H14v1.5H2V2.5h3.5zM3.2 5h9.6l-.7 9.1a1 1 0 0 1-1 .9H4.9a1 1 0 0 1-1-.9z" /></svg>
      </button>
      <button class="icon-btn" title="Fechar (Esc)" @click="emit('close')"><Icon name="close" :size="14" /></button>
    </header>

    <div class="min-h-0 flex-1 overflow-auto">
      <div v-if="quick.length" class="flex flex-wrap gap-1.5 border-b border-zinc-800/70 px-4 py-3">
        <button
          v-for="a in quick" :key="a"
          class="rounded-md border px-2 py-1 text-xs"
          :class="a === 'drain' ? 'border-rose-900 text-rose-300 hover:bg-rose-950' : 'border-zinc-800 text-zinc-200 hover:bg-zinc-800'"
          @click="emit('action', a, row)"
        >{{ actionLabel[a] }}</button>
      </div>

      <div v-if="pod?.details.value?.diagnoses?.length" class="border-b border-zinc-800/70 px-4 py-3">
        <PodDiagnoses :items="pod.details.value.diagnoses" @previous-logs="emit('container', 'previous', $event, row)" />
      </div>
      <p v-if="pod?.error.value" class="border-b border-zinc-800/70 px-4 py-2 text-xs text-rose-300">{{ pod.error.value }}</p>

      <section v-if="usage && !isPod" class="grid grid-cols-2 gap-2 border-b border-zinc-800/70 px-4 py-3">
        <div class="rounded-lg border border-zinc-800 bg-zinc-900/60 p-3">
          <div class="text-[10px] tracking-wide text-zinc-500 uppercase">CPU</div>
          <div class="mt-1 font-mono text-lg text-zinc-100">{{ cpu(usage.cpu) }}</div>
        </div>
        <div class="rounded-lg border border-zinc-800 bg-zinc-900/60 p-3">
          <div class="text-[10px] tracking-wide text-zinc-500 uppercase">Memória</div>
          <div class="mt-1 font-mono text-lg text-zinc-100">{{ bytes(usage.memory) }}</div>
        </div>
      </section>

      <!-- Properties -->
      <section class="border-b border-zinc-800/70 px-4 pt-4 pb-2">
        <h3 class="mb-2 text-sm font-semibold text-zinc-100">Properties</h3>
        <div class="text-xs">
          <div class="prop">
            <span class="prop-k">Created</span>
            <span class="prop-v">{{ age(created, now) }} atrás <span class="text-zinc-500">({{ created ? new Date(created).toLocaleString() : '' }})</span></span>
          </div>
          <div class="prop"><span class="prop-k">Name</span><span class="prop-v font-mono select-text">{{ row.name }}</span></div>
          <div v-if="row.namespace" class="prop">
            <span class="prop-k">Namespace</span>
            <span class="prop-v"><button class="text-sky-400 hover:underline" @click="emit('navigate', { kind: 'Namespace', search: row.namespace })">{{ row.namespace }}</button></span>
          </div>
          <div class="prop">
            <span class="prop-k">Labels</span>
            <span class="prop-v">
              <button v-if="labels.length" class="flex items-center gap-1 text-zinc-300 hover:text-zinc-100" @click="labelsOpen = !labelsOpen">
                {{ labels.length }} Label{{ labels.length > 1 ? 's' : '' }}
                <Icon name="chevron" :size="11" class="transition-transform" :class="labelsOpen && 'rotate-90'" />
              </button>
              <span v-else class="text-zinc-600">—</span>
              <span v-if="labelsOpen" class="mt-1.5 flex flex-wrap gap-1">
                <span v-for="[k, v] in labels" :key="k" class="rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[11px] text-zinc-300 select-text">{{ k }}={{ v }}</span>
              </span>
            </span>
          </div>
          <div v-if="row.owner" class="prop">
            <span class="prop-k">Controlled By</span>
            <span class="prop-v">
              {{ row.owner.kind }}
              <button class="text-sky-400 hover:underline" @click="emit('navigate', { kind: row.owner.kind, search: row.owner.name, namespace: row.namespace })">{{ row.owner.name }}</button>
            </span>
          </div>
          <div v-for="p in isPod ? [] : extraProps" :key="p.name" class="prop">
            <span class="prop-k truncate" :title="p.name">{{ p.name }}</span>
            <span class="prop-v break-all select-text" :class="STATUS_COLUMNS.has(p.name) ? toneClass[statusTone(p.value)] : ''">{{ p.value }}</span>
          </div>
          <div class="prop border-none"><span class="prop-k">UID</span><span class="prop-v font-mono text-[11px] text-zinc-500 select-text">{{ row.uid }}</span></div>
        </div>
      </section>

      <PodSection
        v-if="pod?.details.value && row.namespace" :cluster="cluster" :namespace="row.namespace" :name="row.name" :details="pod.details.value"
        @navigate="emit('navigate', $event)" @container="(a, c) => emit('container', a, c, row)"
      />

      <!-- Data (ConfigMap / Secret) -->
      <div v-if="dataResource && row.namespace" class="border-b border-zinc-800/70 px-4 py-4">
        <DataSection :cluster="cluster" :resource="dataResource" :namespace="row.namespace" :name="row.name" :rv="row.rv" />
      </div>

      <!-- Events -->
      <section class="px-4 py-4 text-xs">
        <h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-zinc-100">
          Events <span class="rounded-full bg-zinc-800 px-1.5 text-[10px] font-normal text-zinc-400">{{ events.length }}</span>
          <span v-if="ev.loading.value" class="animate-pulse text-xs font-normal text-zinc-500">carregando…</span>
        </h3>
        <p v-if="!events.length && !ev.loading.value" class="text-zinc-500">Nenhum evento encontrado.</p>
        <ul class="space-y-2">
          <li v-for="e in events" :key="e.uid" class="rounded-lg border-l-2 bg-zinc-900/60 px-3 py-2" :class="e.type === 'Warning' ? 'border-amber-500' : 'border-zinc-700'">
            <div class="flex items-center gap-2">
              <span class="font-medium" :class="e.type === 'Warning' ? 'text-amber-300' : 'text-zinc-200'">{{ e.reason }}</span>
              <span class="ml-auto text-[11px] text-zinc-500">{{ e.seen }}</span>
            </div>
            <p class="mt-1 break-words text-zinc-400 select-text">{{ e.message }}</p>
          </li>
        </ul>
      </section>
    </div>
  </aside>
</template>

<style scoped>
@reference "../style.css";
.menu { @apply absolute top-full right-0 z-40 mt-1 w-64 rounded-lg border border-zinc-800 bg-zinc-900 py-1 text-xs shadow-2xl shadow-black/50; }
.menu-title { @apply px-3 pt-1 pb-1.5 text-[11px] text-zinc-500; }
.menu-item { @apply flex w-full items-center gap-2 px-3 py-1.5 text-left text-zinc-200 hover:bg-zinc-800; }
.icon-btn { @apply grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100; }
.prop { @apply grid grid-cols-[130px_1fr] gap-3 border-b border-zinc-800/60 py-2.5; }
.prop-k { @apply text-zinc-500; }
.prop-v { @apply min-w-0 text-zinc-200; }
</style>
