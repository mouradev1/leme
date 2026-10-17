<script setup lang="ts">
/** Pod-specific body of the details panel: pod props, volumes, containers. */
import { computed, ref, toRef } from 'vue'
import type { pods } from '../../wailsjs/go/models'
import { usePodMetrics } from '@/composables/usePodMetrics'
import type { NavigateTarget } from '@/lib/types'
import { bytes, cpu } from '@/lib/format'
import MetricsChart, { type RefLine } from './MetricsChart.vue'
import Icon from './Icon.vue'

const props = defineProps<{ cluster: string; namespace: string; name: string; details: pods.Details }>()
const emit = defineEmits<{
  navigate: [NavigateTarget]
  container: [action: 'logs' | 'previous' | 'shell', container: string]
}>()

const { samples, error: metricsError, intervalMs } = usePodMetrics(toRef(props, 'cluster'), toRef(props, 'namespace'), toRef(props, 'name'))

const WINDOWS = [
  { label: '5 min', ms: 5 * 60_000 },
  { label: '15 min', ms: 15 * 60_000 },
  { label: '1 h', ms: 60 * 60_000 },
]
const windowMs = ref(15 * 60_000)
const metric = ref<'cpu' | 'memory'>('cpu')

const open = ref(new Set<string>())
const toggle = (k: string) => (open.value.has(k) ? open.value.delete(k) : open.value.add(k))

const containers = computed(() => [...(props.details.initContainers ?? []), ...(props.details.containers ?? [])])

function series(c: string) {
  return samples.value.filter((s) => s.c[c]).map((s) => ({ t: s.t, v: s.c[c][metric.value] }))
}
function refsFor(c: pods.Container): RefLine[] {
  const r: RefLine[] = []
  const req = metric.value === 'cpu' ? c.requests.cpu : c.requests.memory
  const lim = metric.value === 'cpu' ? c.limits.cpu : c.limits.memory
  if (req) r.push({ label: 'request', value: req, dash: '4 3' })
  if (lim) r.push({ label: 'limit', value: lim, dash: '1 3' })
  return r
}
const fmt = computed(() => (metric.value === 'cpu' ? cpu : bytes))
const latest = (c: string) => samples.value.at(-1)?.c[c]

function dot(c: pods.Container) {
  if (c.state === 'running') return c.ready ? 'bg-emerald-500' : 'bg-amber-500'
  if (c.state === 'waiting') return /BackOff|Err|Error/.test(c.stateReason) ? 'bg-rose-500' : 'bg-amber-500'
  if (c.state === 'terminated') return c.stateReason === 'Completed' ? 'bg-zinc-500' : 'bg-rose-500'
  return 'bg-zinc-600'
}
function stateText(c: pods.Container) {
  const parts = [c.state]
  if (c.stateReason) parts.push(c.stateReason)
  if (c.state === 'running') parts.push(c.ready ? 'ready' : 'not ready')
  return parts.join(', ')
}
const stateClass = (c: pods.Container) =>
  c.state === 'running' && c.ready ? 'text-emerald-400' : c.state === 'waiting' && /BackOff|Err/.test(c.stateReason) ? 'text-rose-400' : 'text-amber-300'

function probeChips(p: pods.Probe) {
  return [p.handler, p.target, `delay=${p.delay}s`, `timeout=${p.timeout}s`, `period=${p.period}s`, `#success=${p.success}`, `#failure=${p.failure}`].filter(Boolean)
}
const condOk = (s: string) => s === 'True'
const when = (iso: string) => (iso ? new Date(iso).toLocaleString() : '')
</script>

<template>
  <div class="text-xs">
    <!-- Pod properties -->
    <section class="border-b border-zinc-800/70 px-4 pt-1 pb-2">
      <div class="prop">
        <span class="prop-k">Status</span>
        <span class="prop-v" :class="details.phase === 'Running' ? 'text-emerald-400' : details.phase === 'Pending' ? 'text-amber-300' : 'text-zinc-200'">
          {{ details.phase }}<template v-if="details.reason"> ({{ details.reason }})</template>
        </span>
      </div>
      <div v-if="details.node" class="prop">
        <span class="prop-k">Node</span>
        <span class="prop-v"><button class="text-sky-400 hover:underline" @click="emit('navigate', { kind: 'Node', search: details.node })">{{ details.node }}</button></span>
      </div>
      <div v-if="details.podIP" class="prop"><span class="prop-k">Pod IP</span><span class="prop-v font-mono select-text">{{ details.podIP }}</span></div>
      <div v-if="(details.podIPs ?? []).length > 1" class="prop">
        <span class="prop-k">Pod IPs</span>
        <span class="prop-v flex flex-wrap gap-1"><span v-for="ip in details.podIPs" :key="ip" class="chip font-mono">{{ ip }}</span></span>
      </div>
      <div v-if="details.serviceAccount" class="prop">
        <span class="prop-k">Service Account</span>
        <span class="prop-v"><button class="text-sky-400 hover:underline" @click="emit('navigate', { kind: 'ServiceAccount', search: details.serviceAccount, namespace })">{{ details.serviceAccount }}</button></span>
      </div>
      <div class="prop"><span class="prop-k">QoS Class</span><span class="prop-v">{{ details.qosClass }}</span></div>
      <div v-if="details.priorityClass" class="prop"><span class="prop-k">Priority Class</span><span class="prop-v">{{ details.priorityClass }}</span></div>
      <div class="prop">
        <span class="prop-k">Conditions</span>
        <span class="prop-v flex flex-wrap gap-1">
          <span
            v-for="c in details.conditions ?? []" :key="c.type" class="chip"
            :class="condOk(c.status) ? '' : 'border border-amber-800 text-amber-300'"
            :title="[c.reason, c.message, when(c.lastTransition)].filter(Boolean).join('\n')"
          >{{ c.type }}</span>
        </span>
      </div>
      <div v-if="(details.tolerations ?? []).length" class="prop">
        <button class="prop-k flex items-center gap-1 text-left hover:text-zinc-300" @click="toggle('tol')">
          Tolerations <Icon name="chevron" :size="10" class="transition-transform" :class="open.has('tol') && 'rotate-90'" />
        </button>
        <span class="prop-v">
          <template v-if="!open.has('tol')">{{ details.tolerations.length }}</template>
          <span v-else class="flex flex-col gap-1"><span v-for="t in details.tolerations" :key="t" class="font-mono text-[11px]">{{ t }}</span></span>
        </span>
      </div>
      <div v-if="details.nodeSelector && Object.keys(details.nodeSelector).length" class="prop border-none">
        <span class="prop-k">Node Selector</span>
        <span class="prop-v flex flex-wrap gap-1"><span v-for="(v, k) in details.nodeSelector" :key="k" class="chip font-mono">{{ k }}={{ v }}</span></span>
      </div>
    </section>

    <!-- Volumes -->
    <section v-if="(details.volumes ?? []).length" class="border-b border-zinc-800/70 px-4 py-3">
      <button class="flex w-full items-center gap-2 text-sm font-semibold text-zinc-100" @click="toggle('vol')">
        Volumes <span class="rounded-full bg-zinc-800 px-1.5 text-[10px] font-normal text-zinc-400">{{ details.volumes.length }}</span>
        <Icon name="chevron" :size="12" class="ml-auto text-zinc-500 transition-transform" :class="open.has('vol') && 'rotate-90'" />
      </button>
      <ul v-if="open.has('vol')" class="mt-2 space-y-1">
        <li v-for="v in details.volumes" :key="v.name" class="flex items-center gap-2 rounded-md bg-zinc-900/60 px-2.5 py-1.5">
          <span class="font-mono text-zinc-200">{{ v.name }}</span>
          <span class="chip">{{ v.kind }}</span>
          <button
            v-if="['ConfigMap', 'Secret', 'PersistentVolumeClaim'].includes(v.kind) && v.source" class="ml-auto truncate text-sky-400 hover:underline"
            @click="emit('navigate', { kind: v.kind, search: v.source, namespace })"
          >{{ v.source }}</button>
          <span v-else class="ml-auto truncate text-zinc-500">{{ v.source }}</span>
        </li>
      </ul>
    </section>

    <!-- Containers -->
    <section class="px-4 py-3">
      <div class="mb-3 flex items-center gap-2">
        <h3 class="text-sm font-semibold text-zinc-100">Containers</h3>
        <div class="ml-auto flex items-center gap-1">
          <div class="flex overflow-hidden rounded-md border border-zinc-800">
            <button v-for="m in (['cpu', 'memory'] as const)" :key="m" class="px-2 py-1" :class="metric === m ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'" @click="metric = m">
              {{ m === 'cpu' ? 'CPU' : 'Memória' }}
            </button>
          </div>
          <select v-model="windowMs" class="h-[26px] rounded-md border border-zinc-800 bg-zinc-900 px-1.5 text-zinc-300">
            <option v-for="w in WINDOWS" :key="w.ms" :value="w.ms">{{ w.label }}</option>
          </select>
        </div>
      </div>
      <p class="mb-3 text-[11px] text-zinc-500">
        Métricas do Kubernetes Metrics Server, coletadas a cada 15s desde que o painel foi aberto nesta sessão.
        <span v-if="metricsError" class="block text-amber-300/90">{{ metricsError }}</span>
      </p>

      <article v-for="c in containers" :key="c.name" class="mb-4 overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/40">
        <header class="flex items-center gap-2 border-b border-zinc-800/80 px-3 py-2">
          <span class="h-2.5 w-2.5 rounded-[3px]" :class="dot(c)" />
          <span class="font-mono text-[13px] text-zinc-100">{{ c.name }}</span>
          <span v-if="c.init" class="chip">init</span>
          <span v-if="c.restartCount" class="rounded-full bg-amber-500/15 px-1.5 text-[10px] text-amber-300" :title="`${c.restartCount} reinício(s)`">{{ c.restartCount }} {{ c.restartCount === 1 ? 'reinício' : 'reinícios' }}</span>
          <span class="ml-auto flex gap-1">
            <button class="mini" @click="emit('container', 'logs', c.name)">Logs</button>
            <button v-if="c.restartCount" class="mini" @click="emit('container', 'previous', c.name)">Logs anteriores</button>
            <button v-if="c.state === 'running'" class="mini" @click="emit('container', 'shell', c.name)">Shell</button>
          </span>
        </header>

        <div v-if="!c.init" class="border-b border-zinc-800/60 px-2 pt-2">
          <div class="mb-1 flex items-baseline gap-2 px-1 text-[11px] text-zinc-400">
            {{ metric === 'cpu' ? 'Uso de CPU' : 'Uso de memória' }}
            <span v-if="latest(c.name)" class="font-mono text-zinc-100">{{ fmt(latest(c.name)![metric]) }}</span>
          </div>
          <MetricsChart
            :points="series(c.name)" :refs="refsFor(c)" :format="fmt" :window-ms="windowMs" :gap-ms="intervalMs * 4"
            :label="`${metric === 'cpu' ? 'CPU' : 'Memória'} do container ${c.name}`"
          />
        </div>

        <div class="px-3 pb-1">
          <div class="prop"><span class="prop-k">Status</span><span class="prop-v" :class="stateClass(c)">{{ stateText(c) }}</span></div>
          <div v-if="c.stateMessage" class="prop"><span class="prop-k">Mensagem</span><span class="prop-v break-words text-zinc-300 select-text">{{ c.stateMessage }}</span></div>
          <div v-if="c.lastTermination" class="prop">
            <span class="prop-k">Último término</span>
            <span class="prop-v">
              <span class="text-amber-300">{{ c.lastTermination.reason || 'Terminated' }}</span>
              · exit {{ c.lastTermination.exitCode }} <span class="text-zinc-400">({{ c.lastTermination.meaning }})</span>
              <span class="block text-zinc-500">{{ when(c.lastTermination.finishedAt) }}</span>
              <span v-if="c.lastTermination.message" class="mt-1 block font-mono text-[11px] whitespace-pre-wrap text-zinc-400 select-text">{{ c.lastTermination.message }}</span>
            </span>
          </div>
          <div class="prop"><span class="prop-k">Image</span><span class="prop-v"><span class="chip font-mono break-all select-text">{{ c.image }}</span></span></div>
          <div class="prop"><span class="prop-k">ImagePullPolicy</span><span class="prop-v">{{ c.imagePullPolicy }}</span></div>
          <div v-if="(c.ports ?? []).length" class="prop">
            <span class="prop-k">Ports</span>
            <span class="prop-v flex flex-wrap gap-1"><span v-for="p in c.ports" :key="p.containerPort" class="chip font-mono">{{ p.name ? `${p.name}: ` : '' }}{{ p.containerPort }}/{{ p.protocol }}</span></span>
          </div>
          <div class="prop">
            <span class="prop-k">Resources</span>
            <span class="prop-v font-mono text-[11px]">
              <span class="block">requests: cpu {{ c.requests.cpu ? cpu(c.requests.cpu) : '—' }} · mem {{ c.requests.memory ? bytes(c.requests.memory) : '—' }}</span>
              <span class="block">limits:   cpu {{ c.limits.cpu ? cpu(c.limits.cpu) : '—' }} · mem {{ c.limits.memory ? bytes(c.limits.memory) : '—' }}</span>
            </span>
          </div>
          <div v-if="(c.env ?? []).length || (c.envFrom ?? []).length" class="prop">
            <button class="prop-k flex items-center gap-1 text-left hover:text-zinc-300" @click="toggle(`env:${c.name}`)">
              Environment <Icon name="chevron" :size="10" class="transition-transform" :class="open.has(`env:${c.name}`) && 'rotate-90'" />
            </button>
            <span class="prop-v">
              <template v-if="!open.has(`env:${c.name}`)">{{ (c.env ?? []).length }} variáveis<template v-if="(c.envFrom ?? []).length"> + {{ c.envFrom.join(', ') }}</template></template>
              <span v-else class="flex flex-col gap-1 font-mono text-[11px]">
                <span v-for="e in c.env ?? []" :key="e.name" class="break-all select-text">
                  <span class="text-zinc-300">{{ e.name }}</span>=<span v-if="e.from" class="text-sky-300/80">{{ e.from }}</span><span v-else class="text-zinc-400">{{ e.value }}</span>
                </span>
                <span v-for="f in c.envFrom ?? []" :key="f" class="text-sky-300/80">envFrom: {{ f }}</span>
              </span>
            </span>
          </div>
          <div v-if="(c.mounts ?? []).length" class="prop">
            <span class="prop-k">Mounts</span>
            <span class="prop-v flex flex-col gap-1.5">
              <span v-for="m in c.mounts" :key="m.path + m.volume">
                <span class="chip font-mono break-all">{{ m.path }}</span>
                <span class="block text-zinc-500">de {{ m.volume }}{{ m.subPath ? ` (${m.subPath})` : '' }} {{ m.readOnly ? '(ro)' : '(rw)' }}</span>
              </span>
            </span>
          </div>
          <div v-for="[label, p] in ([['Liveness', c.liveness], ['Readiness', c.readiness], ['Startup', c.startup]] as const)" :key="label">
            <div v-if="p" class="prop">
              <span class="prop-k">{{ label }}</span>
              <span class="prop-v flex flex-wrap gap-1"><span v-for="chip in probeChips(p)" :key="chip" class="chip font-mono">{{ chip }}</span></span>
            </div>
          </div>
        </div>
      </article>
    </section>
  </div>
</template>

<style scoped>
@reference "../style.css";
.prop { @apply grid grid-cols-[130px_1fr] gap-3 border-b border-zinc-800/60 py-2.5; }
.prop:last-child { @apply border-none; }
.prop-k { @apply text-zinc-500; }
.prop-v { @apply min-w-0 text-zinc-200; }
.chip { @apply inline-block rounded bg-zinc-800 px-1.5 py-0.5 text-[11px] text-zinc-300; }
.mini { @apply rounded border border-zinc-700 px-2 py-0.5 text-[11px] text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100; }
</style>
