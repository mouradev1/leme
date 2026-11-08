<script setup lang="ts">
import { tr } from '@/lib/i18n'
import { isReadOnly } from '@/lib/guard'
/** Helm releases: list, history, values, rollback, uninstall. */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { History, ListReleases, Rollback, Uninstall, Values } from '../../wailsjs/go/api/HelmAPI'
import type { helm } from '../../wailsjs/go/models'
import { useClusters } from '@/stores/clusters'
import { age } from '@/lib/format'
import { statusTone, toneClass } from '@/lib/status'
import { errMsg } from '@/lib/bridge'
import NamespaceSelect from './NamespaceSelect.vue'
import SearchBox from './SearchBox.vue'
import { usePanelWidth } from '@/lib/resize'
import Icon from './Icon.vue'
import HelmInstallDialog from './HelmInstallDialog.vue'

const props = defineProps<{ cluster: string }>()
const emit = defineEmits<{ confirm: [title: string, run: () => Promise<void>, danger?: boolean]; toast: [msg: string] }>()
const store = useClusters()
const { namespaces, selectedNamespaces } = storeToRefs(store)

const releases = ref<helm.Release[]>([])
const dialog = ref<{ release?: helm.Release } | null>(null)
function onDone(r: helm.Release) {
  emit('toast', tr('{a}: revisão {b} ({c})', { a: r.name, b: r.revision, c: r.status }))
  void load()
  if (sel.value?.name === r.name && sel.value.namespace === r.namespace) void open(r)
}
const loading = ref(false)
const error = ref('')
const search = ref('')
const now = ref(Date.now())

const ns = computed(() => (selectedNamespaces.value.length === 1 ? selectedNamespaces.value[0] : ''))
async function load() {
  loading.value = true
  try {
    releases.value = await ListReleases(props.cluster, ns.value)
    error.value = ''
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    loading.value = false
    now.value = Date.now()
  }
}
watch([() => props.cluster, ns], load, { immediate: true })
const timer = setInterval(load, 30_000) // releases change rarely; no watch API for them
onBeforeUnmount(() => clearInterval(timer))

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  const set = new Set(selectedNamespaces.value)
  return releases.value.filter((r) =>
    (set.size <= 1 || set.has(r.namespace)) &&
    (!q || `${r.name} ${r.namespace} ${r.chart} ${r.status}`.toLowerCase().includes(q)))
})

// ---- details panel ----
const sel = ref<helm.Release | null>(null)
const panel = usePanelWidth('helm', 460)
const history = ref<helm.Release[]>([])
const values = ref('')
const allValues = ref(false)
async function open(r: helm.Release) {
  sel.value = r
  history.value = []
  values.value = ''
  try {
    const [h, v] = await Promise.all([History(props.cluster, r.namespace, r.name), Values(props.cluster, r.namespace, r.name, allValues.value)])
    history.value = [...h].sort((a, b) => b.revision - a.revision)
    values.value = v
  } catch (e) {
    emit('toast', errMsg(e))
  }
}
watch(allValues, () => sel.value && open(sel.value))

function rollback(r: helm.Release, revision: number) {
  emit('confirm', tr('Rollback de {a} para a revisão {b}?', { a: r.name, b: revision }), async () => {
    await Rollback(props.cluster, r.namespace, r.name, revision)
    emit('toast', tr('{a}: rollback para revisão {b} feito', { a: r.name, b: revision }))
    await load()
    if (sel.value) await open(sel.value)
  }, true)
}
function uninstall(r: helm.Release) {
  emit('confirm', `Desinstalar o release ${r.name} (${r.namespace})?`, async () => {
    await Uninstall(props.cluster, r.namespace, r.name)
    sel.value = null
    emit('toast', `${r.name} desinstalado`)
    await load()
  }, true)
}
</script>

<template>
  <div class="flex h-full min-w-0">
    <div class="flex min-w-0 flex-1 flex-col">
      <div class="flex items-center gap-2 border-b border-zinc-800/80 px-4 py-2.5">
        <h1 class="text-[15px] font-semibold text-zinc-100">{{ tr('Helm Releases') }}</h1>
        <span class="rounded-full bg-zinc-800 px-2 py-0.5 text-[11px] text-zinc-400">{{ filtered.length }}</span>
        <span v-if="loading" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
        <div class="ml-auto flex min-w-[420px] flex-1 items-center justify-end gap-2">
          <NamespaceSelect v-model="selectedNamespaces" :options="namespaces" :projects="store.projects" />
          <div class="flex max-w-md min-w-0 flex-1"><SearchBox v-model="search" :placeholder="tr('Buscar releases…')" /></div>
          <button v-if="!isReadOnly(cluster)" class="h-8 shrink-0 rounded-md bg-sky-600 px-3 text-xs font-medium text-white hover:bg-sky-500" @click="dialog = {}">{{ tr('Instalar chart') }}</button>
        </div>
      </div>
      <div v-if="error" class="bg-rose-950/40 px-4 py-1.5 text-xs text-rose-300">{{ error }}</div>
      <div class="min-h-0 flex-1 overflow-auto">
        <table class="w-full text-[13px]">
          <thead class="sticky top-0 bg-zinc-925 text-left text-[11px] font-medium text-zinc-400">
            <tr class="border-b border-zinc-800">
              <th class="px-4 py-2.5">{{ tr('Name') }}</th><th class="px-3">{{ tr('Namespace') }}</th><th class="px-3">{{ tr('Chart') }}</th>
              <th class="px-3">{{ tr('Version') }}</th><th class="px-3">{{ tr('App Version') }}</th><th class="px-3">{{ tr('Revision') }}</th>
              <th class="px-3">{{ tr('Status') }}</th><th class="px-3">{{ tr('Updated') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="r in filtered" :key="`${r.namespace}/${r.name}`"
              class="cursor-default border-b border-zinc-800/50 hover:bg-zinc-800/40"
              :class="sel?.name === r.name && sel?.namespace === r.namespace && 'bg-sky-500/10'"
              @click="open(r)"
            >
              <td class="px-4 py-2 text-zinc-100">{{ r.name }}</td>
              <td class="px-3 text-zinc-400">{{ r.namespace }}</td>
              <td class="px-3">{{ r.chart }}</td>
              <td class="px-3 text-zinc-400">{{ r.chartVersion }}</td>
              <td class="px-3 text-zinc-400">{{ r.appVersion }}</td>
              <td class="px-3 tabular-nums">{{ r.revision }}</td>
              <td class="px-3" :class="toneClass[statusTone(r.status)]">{{ r.status }}</td>
              <td class="px-3 text-zinc-400">{{ r.updated ? age(Date.parse(r.updated), now) : '' }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="!loading && !filtered.length" class="py-20 text-center text-sm text-zinc-500">{{ tr('Nenhum release Helm encontrado.') }}</p>
      </div>
    </div>

    <aside v-if="sel" class="relative flex shrink-0 flex-col border-l border-zinc-800 bg-zinc-925" :style="{ width: `${panel.width.value}px` }">
            <div
              class="absolute inset-y-0 -left-1 z-20 w-2 cursor-col-resize transition-colors hover:bg-sky-500/40"
              :class="panel.dragging.value && 'bg-sky-500/60'"
              :title="tr('Arraste para redimensionar (duplo clique: tamanho padrão)')"
              @mousedown="panel.start" @dblclick="panel.reset"
            />
      <header class="border-b border-zinc-800 px-4 py-3">
        <div class="flex items-center gap-2">
          <span class="rounded bg-violet-500/15 px-1.5 py-0.5 text-[10px] font-semibold text-violet-300 uppercase">{{ tr('Release') }}</span>
          <button class="ml-auto text-zinc-500 hover:text-zinc-200" :title="tr('Fechar')" @click="sel = null"><Icon name="close" :size="14" /></button>
        </div>
        <h2 class="mt-2 font-mono text-sm text-zinc-100">{{ sel.name }}</h2>
        <p class="text-xs text-zinc-500">{{ sel.namespace }} · {{ sel.chart }}-{{ sel.chartVersion }}</p>
        <div class="mt-3 flex gap-1.5">
          <button v-if="!isReadOnly(cluster)" class="rounded-md border border-sky-800 px-2 py-1 text-xs text-sky-300 hover:bg-sky-950" @click="dialog = { release: sel }">{{ tr('Atualizar / values') }}</button>
          <button v-if="!isReadOnly(cluster)" class="rounded-md border border-zinc-800 px-2 py-1 text-xs hover:bg-zinc-800" :disabled="sel.revision <= 1" @click="rollback(sel, 0)">{{ tr('Rollback p/ anterior') }}</button>
          <button v-if="!isReadOnly(cluster)" class="rounded-md border border-rose-900 px-2 py-1 text-xs text-rose-300 hover:bg-rose-950" @click="uninstall(sel)">{{ tr('Desinstalar') }}</button>
        </div>
      </header>
      <div class="min-h-0 flex-1 space-y-5 overflow-auto px-4 py-4 text-xs">
        <section>
          <h3 class="mb-2 text-[10px] font-semibold tracking-wider text-zinc-500 uppercase">{{ tr('Histórico') }}</h3>
          <ul class="space-y-1">
            <li v-for="h in history" :key="h.revision" class="flex items-center gap-2 rounded-md bg-zinc-900/60 px-3 py-2">
              <span class="w-8 font-mono text-zinc-300">#{{ h.revision }}</span>
              <span :class="toneClass[statusTone(h.status)]">{{ h.status }}</span>
              <span class="min-w-0 flex-1 truncate text-zinc-500" :title="h.description">{{ h.chartVersion }} · {{ h.description }}</span>
              <button v-if="h.revision !== sel.revision && !isReadOnly(cluster)" class="shrink-0 rounded px-1.5 py-0.5 text-sky-400 hover:bg-zinc-800" @click="rollback(sel, h.revision)">{{ tr('rollback') }}</button>
              <span v-else class="shrink-0 text-[10px] text-emerald-400">{{ tr('atual') }}</span>
            </li>
          </ul>
        </section>
        <section>
          <h3 class="mb-2 flex items-center text-[10px] font-semibold tracking-wider text-zinc-500 uppercase">
            {{ tr('Values') }}
            <label class="ml-auto flex items-center gap-1 font-normal normal-case"><input v-model="allValues" type="checkbox" class="accent-sky-500" /> {{ tr('incluir defaults do chart') }}</label>
          </h3>
          <pre class="max-h-[50vh] overflow-auto rounded-md bg-zinc-950 p-3 font-mono text-[11px] text-zinc-300 select-text">{{ values || '# (sem values customizados)' }}</pre>
        </section>
      </div>
    </aside>
    <HelmInstallDialog
      v-if="dialog" :cluster="cluster" :namespaces="namespaces" :release="dialog.release" :default-namespace="ns || undefined"
      @close="dialog = null" @done="onDone"
    />
  </div>
</template>
