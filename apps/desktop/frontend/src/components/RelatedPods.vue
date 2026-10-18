<script setup lang="ts">
/**
 * Live list of the pods behind an object: a workload's or Service's pods (by
 * its selector) or the pods scheduled on a Node (spec.nodeName). One small
 * filtered watch, only while the panel is open.
 */
import { computed, ref, watch } from 'vue'
import { PodSelector } from '../../wailsjs/go/api/ResourceAPI'
import type { resources } from '../../wailsjs/go/models'
import { useResourceWatch } from '@/composables/useResourceWatch'
import { errMsg } from '@/lib/bridge'
import { cell } from '@/lib/format'
import { statusTone, toneClass } from '@/lib/status'
import type { NavigateTarget, Row } from '@/lib/types'

const props = defineProps<{ cluster: string; kind: resources.Kind; row: Row; mode: 'node' | 'selector' }>()
const emit = defineEmits<{ navigate: [NavigateTarget] }>()

const MAX = 200
const podsRef = ref<resources.Ref>({ group: '', version: 'v1', resource: 'pods', namespaced: true })
const selector = ref('')
const selectorError = ref('')

// Workload selectors are immutable; a Service's can change, so re-read it when
// the Service itself changes.
async function loadSelector() {
  if (props.mode !== 'selector') return
  try {
    const ref = { group: props.kind.group, version: props.kind.version, resource: props.kind.resource, namespaced: props.kind.namespaced }
    selector.value = await PodSelector(props.cluster, ref, props.row.namespace ?? '', props.row.name)
    selectorError.value = ''
  } catch (e) {
    selectorError.value = errMsg(e)
  }
}
watch(() => (props.kind.resource === 'services' ? props.row.rv : ''), loadSelector, { immediate: true })

// Built from string computeds: props.row is replaced on every update of the
// object, and a new query object would restart the watch each time.
const name = computed(() => props.row.name)
const namespace = computed(() => props.row.namespace ?? '')
const ready = computed(() => props.mode === 'node' || !!selector.value)
const query = computed<resources.Query>(() =>
  props.mode === 'node'
    ? { namespace: '', labelSelector: '', fieldSelector: `spec.nodeName=${name.value}` }
    : { namespace: namespace.value, labelSelector: selector.value, fieldSelector: '' })
const w = useResourceWatch(computed(() => props.cluster), computed(() => (ready.value ? podsRef.value : null)), query)

const pods = computed(() => {
  const idx = (n: string) => w.columns.value.findIndex((c) => c.name === n)
  const [iReady, iStatus, iRestarts, iNode] = ['Ready', 'Status', 'Restarts', 'Node'].map(idx)
  return w.rows.value
    .map((r) => ({
      uid: r.uid,
      name: r.name,
      namespace: r.namespace ?? '',
      ready: cell(r.cells[iReady]),
      status: cell(r.cells[iStatus]),
      restarts: cell(r.cells[iRestarts]).replace(/ \(.*\)$/, ''), // "3 (2m ago)" → "3"
      node: cell(r.cells[iNode]),
    }))
    .sort((a, b) => a.namespace.localeCompare(b.namespace) || a.name.localeCompare(b.name))
})
const shown = computed(() => pods.value.slice(0, MAX))
const readyCount = computed(() => pods.value.filter((p) => {
  const [a, b] = p.ready.split('/')
  return a && a === b && p.status === 'Running'
}).length)

const open = (p: { name: string; namespace: string }) => emit('navigate', { kind: 'Pod', search: p.name, namespace: p.namespace })
</script>

<template>
  <section class="border-b border-zinc-800/70 px-4 py-4 text-xs">
    <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-zinc-100">
      Pods <span class="rounded-full bg-zinc-800 px-1.5 text-[10px] font-normal text-zinc-400">{{ pods.length }}</span>
      <span v-if="pods.length" class="text-[11px] font-normal text-zinc-500">{{ readyCount }} prontos</span>
      <span v-if="w.loading.value" class="animate-pulse text-xs font-normal text-zinc-500">carregando…</span>
    </h3>
    <p v-if="selectorError || w.error.value" class="text-rose-300">{{ selectorError || w.error.value }}</p>
    <p v-else-if="!pods.length && !w.loading.value && ready" class="text-zinc-500">Nenhum pod.</p>
    <table v-if="pods.length" class="w-full">
      <thead class="text-left text-[11px] text-zinc-500">
        <tr>
          <th class="py-1 pr-2 font-normal">Nome</th>
          <th class="px-2 font-normal">Ready</th>
          <th class="px-2 font-normal">Status</th>
          <th class="px-2 text-right font-normal">Reinícios</th>
          <th class="pl-2 font-normal">{{ mode === 'node' ? 'Namespace' : 'Node' }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in shown" :key="p.uid" class="border-t border-zinc-800/60">
          <td class="max-w-0 truncate py-1.5 pr-2" :title="p.name">
            <button class="text-sky-400 hover:underline" @click="open(p)">{{ p.name }}</button>
          </td>
          <td class="px-2 font-mono text-zinc-300">{{ p.ready }}</td>
          <td class="px-2 whitespace-nowrap" :class="toneClass[statusTone(p.status)]">{{ p.status }}</td>
          <td class="px-2 text-right font-mono" :class="p.restarts !== '0' ? 'text-amber-300' : 'text-zinc-400'">{{ p.restarts }}</td>
          <td class="max-w-[120px] truncate pl-2 text-zinc-400" :title="mode === 'node' ? p.namespace : p.node">
            <button
              v-if="mode !== 'node' && p.node" class="hover:text-sky-400 hover:underline"
              @click="emit('navigate', { kind: 'Node', search: p.node })"
            >{{ p.node }}</button>
            <span v-else>{{ mode === 'node' ? p.namespace : '' }}</span>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-if="pods.length > MAX" class="mt-2 text-zinc-500">mostrando {{ MAX }} de {{ pods.length }}</p>
  </section>
</template>
