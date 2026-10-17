<script setup lang="ts">
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Cordon, Delete, Drain, PodContainers, Restart, Scale } from '../wailsjs/go/api/ResourceAPI'
import { Unsubscribe } from '../wailsjs/go/api/StreamAPI'
import type { resources } from '../wailsjs/go/models'
import Sidebar from '@/components/Sidebar.vue'
import ResourceTable from '@/components/ResourceTable.vue'
import Overview from '@/components/Overview.vue'
import HelmReleases from '@/components/HelmReleases.vue'
import YamlEditor from '@/components/YamlEditor.vue'
import Dock from '@/components/Dock.vue'
import KubeconfigManager from '@/components/KubeconfigManager.vue'
import UpdateDialog from '@/components/UpdateDialog.vue'
import { CheckUpdate } from '../wailsjs/go/api/AppAPI'
import type { update } from '../wailsjs/go/models'
import Icon from '@/components/Icon.vue'
import { useClusters } from '@/stores/clusters'
import { usePanelWidth } from '@/lib/resize'
import { errMsg, newId, on } from '@/lib/bridge'
import { hue, initials } from '@/lib/dom'
import type { DockTab, NavigateTarget, NewDockTab, Row, RowAction, View } from '@/lib/types'

const store = useClusters()
const { contexts, active, server, selectedNamespaces, connecting } = storeToRefs(store)
onMounted(() => {
  store.load()
  // New-version notice: one anonymous request to GitHub's releases endpoint,
  // a few seconds after startup; off when the user disabled it or on dev builds.
  setTimeout(() => {
    CheckUpdate(true)
      .then((i) => { if (i.available) { updateInfo.value = i; showUpdate.value = true } })
      .catch(() => { /* offline or rate-limited: stay quiet */ })
  }, 4000)
})

const updateInfo = shallowRef<update.Info | null>(null)
const showUpdate = ref(false)
async function checkUpdateNow() {
  try {
    const i = await CheckUpdate(false)
    updateInfo.value = i
    if (i.available) showUpdate.value = true
    else flash(i.latest ? `Você está na versão mais recente (v${i.latest}).` : 'Nenhuma versão publicada ainda.')
  } catch (e) {
    flash(`Não foi possível verificar atualizações: ${errMsg(e)}`)
  }
}

const view = shallowRef<View | null>(null)
const search = ref('')
const showKubeconfigs = ref(false)
const toast = ref('')
let toastTimer = 0
function flash(msg: string) {
  toast.value = msg
  clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = ''), 4500)
}

async function openCluster(name: string) {
  await store.activate(name)
  if (store.active === name) view.value = { type: 'overview' }
}

// ---- YAML editor panel -----------------------------------------------------
// cluster: the editor is bound to the cluster it was opened in (never the
// active one), and closes when the user switches clusters.
const editing = shallowRef<{ cluster: string; kind?: resources.Ref; namespace?: string; name?: string; initialYaml?: string; key: string } | null>(null)
watch(active, () => (editing.value = null))

// Changing view clears the search, unless the change came from a link.
let keepSearch = false
watch(view, () => {
  if (!keepSearch) search.value = ''
  keepSearch = false
  editing.value = null
})

function navigate(t: NavigateTarget) {
  const k = store.findKind(t.kind)
  if (!k) return flash(`Tipo ${t.kind} não encontrado neste cluster`)
  keepSearch = true
  view.value = { type: 'resource', kind: k }
  search.value = t.search ?? ''
  if (t.namespace && k.namespaced && selectedNamespaces.value.length && !selectedNamespaces.value.includes(t.namespace)) {
    selectedNamespaces.value = [t.namespace]
  }
}

const currentKind = computed(() => (view.value?.type === 'resource' ? view.value.kind : null))

function createTemplate(k: resources.Kind): string {
  const apiVersion = k.group ? `${k.group}/${k.version}` : k.version
  const ns = selectedNamespaces.value.length === 1 ? selectedNamespaces.value[0] : 'default'
  const body = k.kind === 'ConfigMap' || k.kind === 'Secret' ? 'data:\n  chave: valor' : 'spec: {}'
  return `# Novo ${k.kind}: ajuste o manifesto e clique em Apply (⌘S).\n# Use "Diff" para validar no servidor antes (dry-run).\napiVersion: ${apiVersion}\nkind: ${k.kind}\nmetadata:\n  name: novo-${k.kind.toLowerCase()}\n${k.namespaced ? `  namespace: ${ns}\n` : ''}${body}\n`
}

// ---- Dock --------------------------------------------------------------------
const tabs = ref<DockTab[]>([])
const activeTab = ref('')
const dockMax = ref(false)
// Tabs are bound to the cluster they were opened in. Switching clusters shows
// only that cluster's tabs; the others stay alive (hidden) and come back.
const lastTabByCluster = new Map<string, string>()
const clusterTabs = (c: string) => tabs.value.filter((t) => t.cluster === c)
function openTab(t: NewDockTab) {
  const tab = { ...t, cluster: active.value } as DockTab
  tabs.value.push(tab)
  activeTab.value = tab.id
}
function closeTab(id: string) {
  tabs.value = tabs.value.filter((t) => t.id !== id) // unmount => unsubscribe / close PTY
  if (activeTab.value === id) activeTab.value = clusterTabs(active.value).at(-1)?.id ?? ''
}
watch(activeTab, (id) => { if (active.value && id) lastTabByCluster.set(active.value, id) })
watch(active, (c) => {
  const remembered = lastTabByCluster.get(c)
  const mine = clusterTabs(c)
  activeTab.value = mine.find((t) => t.id === remembered)?.id ?? mine.at(-1)?.id ?? ''
})
const newTerminal = () => openTab({ id: newId(), type: 'shell', title: `local · ${active.value}` })

// ---- Confirm / prompt dialog -------------------------------------------------
const dialog = ref<{ title: string; input?: number; danger?: boolean; run: (n?: number) => Promise<void> } | null>(null)
async function confirmDialog() {
  const d = dialog.value!
  dialog.value = null
  try { await d.run(d.input) } catch (e) { flash(errMsg(e)) }
}
const confirm = (title: string, run: () => Promise<void>, danger = false) => (dialog.value = { title, run, danger })
// A pending confirmation (delete, drain, node shell...) belongs to the cluster
// it was asked in: never let it be confirmed while looking at another cluster.
watch(active, () => (dialog.value = null))

// kind: the object's own kind (a node opened from the pod list is a Node).
async function onAction(action: RowAction, row: Row, kind?: resources.Kind) {
  const k = kind ?? currentKind.value!
  const c = active.value
  const ns = row.namespace ?? ''
  switch (action) {
    case 'edit':
      editing.value = { cluster: active.value, kind: k, namespace: ns, name: row.name, key: newId() }
      break
    case 'logs':
    case 'shell': {
      const pc = await PodContainers(c, ns, row.name).catch((e) => (flash(errMsg(e)), null))
      if (!pc?.containers?.length) return
      if (action === 'logs') {
        openTab({ id: newId(), type: 'logs', title: row.name, namespace: ns, pod: row.name, containers: pc.containers })
      } else {
        openTab({ id: newId(), type: 'shell', title: row.name, target: { namespace: ns, pod: row.name, container: pc.containers[0], command: [] } })
      }
      break
    }
    case 'nodeshell':
      confirm(
        `Abrir um shell root no node ${row.name}? O Leme cria um pod privilegiado temporário em kube-system nesse node (acesso total à máquina) e o remove ao fechar o terminal.`,
        async () => openTab({ id: newId(), type: 'shell', title: `node ${row.name}`, node: row.name }),
        true,
      )
      break
    case 'delete':
      confirm(`Excluir ${k.kind} ${row.name}?`, async () => { await Delete(c, k, ns, row.name, false); flash(`${row.name} excluído`) }, true)
      break
    case 'scale':
      dialog.value = { title: `Escalar ${row.name} para quantas réplicas?`, input: 1,
        run: async (n) => { await Scale(c, k, ns, row.name, n ?? 0); flash(`${row.name} → ${n} réplica(s)`) } }
      break
    case 'restart':
      confirm(`Reiniciar (rollout restart) ${row.name}?`, async () => { await Restart(c, k, ns, row.name); flash(`${row.name} reiniciando`) })
      break
    case 'cordon':
    case 'uncordon':
      await Cordon(c, row.name, action === 'cordon').then(() => flash(`${row.name}: ${action}`)).catch((e) => flash(errMsg(e)))
      break
    case 'drain':
      confirm(`Drain do node ${row.name}? Todos os pods serão removidos (respeitando PDBs).`, async () => {
        const id = newId()
        const off = on<{ type: string; message: string }>(`drain:${id}`, (ev) => {
          flash(ev.type === 'done' ? `${row.name} drenado` : ev.message)
          if (ev.type !== 'progress') { off(); void Unsubscribe(id) }
        })
        await Drain(id, c, row.name, { force: false, deleteEmptyDirData: true, gracePeriodSeconds: -1, timeoutSeconds: 0, disableEviction: false })
      }, true)
      break
  }
}

// Per-container actions from the Pod details panel.
function onContainer(action: 'logs' | 'previous' | 'shell', container: string, row: Row) {
  const ns = row.namespace ?? ''
  if (action === 'shell') {
    openTab({ id: newId(), type: 'shell', title: `${row.name}/${container}`, target: { namespace: ns, pod: row.name, container, command: [] } })
  } else {
    openTab({
      id: newId(), type: 'logs', title: `${row.name}/${container}${action === 'previous' ? ' (anterior)' : ''}`,
      namespace: ns, pod: row.name, containers: [container], previous: action === 'previous',
    })
  }
}

function bulkDelete(rows: Row[]) {
  const k = currentKind.value!
  confirm(`Excluir ${rows.length} ${k.kind}(s)?`, async () => {
    const res = await Promise.allSettled(rows.map((r) => Delete(active.value, k, r.namespace ?? '', r.name, false)))
    const failed = res.filter((r) => r.status === 'rejected').length
    flash(failed ? `${rows.length - failed} excluído(s), ${failed} falha(s)` : `${rows.length} excluído(s)`)
  }, true)
}

const editorPanel = usePanelWidth('editor', 640)

const nsLabel = computed(() => (selectedNamespaces.value.length ? selectedNamespaces.value.join(', ') : 'todos os namespaces'))
</script>

<template>
  <div class="flex h-full flex-col bg-zinc-950 text-zinc-200">
    <div class="flex min-h-0 flex-1">
      <Sidebar v-model:view="view" @add-kubeconfig="showKubeconfigs = true" @open-cluster="openCluster" :update-available="!!updateInfo?.available" @check-update="checkUpdateNow" />

      <main class="flex min-w-0 flex-1 flex-col">
        <div v-show="!dockMax" class="flex min-h-0 flex-1">
          <section class="min-w-0 flex-1">
            <template v-if="active && view">
              <Overview v-if="view.type === 'overview'" :key="active" :cluster="active" @navigate="navigate" />
              <HelmReleases v-else-if="view.type === 'helm'" :key="active" :cluster="active" @confirm="confirm" @toast="flash" />
              <ResourceTable
                v-else :key="`${active}|${view.kind.group}/${view.kind.resource}`"
                v-model:search="search" :cluster="active" :kind="view.kind"
                @action="onAction" @bulk-delete="bulkDelete" @navigate="navigate" @container="onContainer"
                @create="editing = { cluster: active, initialYaml: createTemplate(view.kind), key: newId() }"
              />
            </template>

            <!-- Welcome -->
            <div v-else class="grid h-full place-items-center overflow-auto p-8">
              <div class="w-full max-w-3xl">
                <h1 class="text-xl font-semibold text-zinc-100">Bem-vindo ao Leme</h1>
                <p class="mt-1 text-sm text-zinc-500">Escolha um cluster para conectar. Nada é acessado até você clicar.</p>
                <div class="mt-6 grid gap-3 sm:grid-cols-2">
                  <button
                    v-for="c in contexts" :key="c.name"
                    class="group flex items-center gap-3 rounded-xl border border-zinc-800 bg-zinc-900/50 p-3 text-left transition hover:border-sky-700 hover:bg-zinc-900"
                    @click="openCluster(c.name)"
                  >
                    <span class="grid h-10 w-10 shrink-0 place-items-center rounded-lg text-sm font-bold text-white" :style="{ background: `hsl(${hue(c.name)} 55% 42%)` }">{{ initials(c.name) }}</span>
                    <span class="min-w-0 flex-1">
                      <span class="block truncate text-sm text-zinc-100">{{ c.name }}</span>
                      <span class="block truncate text-xs text-zinc-500">{{ c.server }}</span>
                      <span class="mt-0.5 inline-block rounded bg-zinc-800 px-1.5 text-[10px] text-zinc-400">{{ c.auth }}</span>
                    </span>
                    <span v-if="connecting === c.name" class="h-4 w-4 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
                  </button>
                  <button class="flex items-center justify-center gap-2 rounded-xl border border-dashed border-zinc-700 p-3 text-sm text-zinc-400 hover:border-sky-700 hover:text-sky-300" @click="showKubeconfigs = true">
                    <Icon name="plus" /> Adicionar cluster
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section v-if="editing && active" class="relative shrink-0 border-l border-zinc-800" :style="{ width: `${editorPanel.width.value}px` }">
            <div
              class="absolute inset-y-0 -left-1 z-20 w-2 cursor-col-resize transition-colors hover:bg-sky-500/40"
              :class="editorPanel.dragging.value && 'bg-sky-500/60'"
              title="Arraste para redimensionar (duplo clique: tamanho padrão)"
              @mousedown="editorPanel.start" @dblclick="editorPanel.reset"
            />
            <YamlEditor
              :key="editing.key" :cluster="editing.cluster" :kind="editing.kind" :namespace="editing.namespace"
              :name="editing.name" :initial-yaml="editing.initialYaml"
              @close="editing = null" @applied="flash('Aplicado com sucesso')"
            />
          </section>
        </div>

        <Dock
          v-if="active" v-model:active="activeTab" v-model:maximized="dockMax"
          :cluster="active" :tabs="tabs" @close="closeTab" @new-terminal="newTerminal"
        />
      </main>
    </div>

    <!-- Status bar -->
    <footer class="flex h-6 shrink-0 items-center gap-3 border-t border-zinc-800 bg-zinc-925 px-3 text-[11px] text-zinc-500">
      <template v-if="active">
        <span class="flex items-center gap-1.5 text-zinc-300"><span class="h-1.5 w-1.5 rounded-full bg-emerald-400" />{{ active }}</span>
        <span>{{ server?.version }}</span>
        <span class="truncate">{{ server?.server }}</span>
        <span class="ml-auto truncate">{{ nsLabel }}</span>
      </template>
      <span v-else>desconectado</span>
    </footer>

    <UpdateDialog v-if="showUpdate && updateInfo" :info="updateInfo" @close="showUpdate = false" />
    <KubeconfigManager v-if="showKubeconfigs" @close="showKubeconfigs = false" @changed="store.load()" @connect="openCluster" />

    <!-- Dialog -->
    <div v-if="dialog" class="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-[2px]" @click.self="dialog = null">
      <div class="w-[420px] rounded-xl border border-zinc-800 bg-zinc-900 p-5 text-sm shadow-2xl">
        <p class="mb-4 text-zinc-100">{{ dialog.title }}</p>
        <input
          v-if="dialog.input !== undefined" v-model.number="dialog.input" type="number" min="0" autofocus
          class="mb-4 w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 outline-none focus:border-sky-600"
          @keydown.enter="confirmDialog"
        />
        <div class="flex justify-end gap-2">
          <button class="rounded-md bg-zinc-800 px-3 py-1.5 hover:bg-zinc-700" @click="dialog = null">Cancelar</button>
          <button class="rounded-md px-3 py-1.5 text-white" :class="dialog.danger ? 'bg-rose-600 hover:bg-rose-500' : 'bg-sky-600 hover:bg-sky-500'" @click="confirmDialog">Confirmar</button>
        </div>
      </div>
    </div>
    <Transition enter-from-class="translate-y-2 opacity-0" leave-to-class="opacity-0" enter-active-class="transition" leave-active-class="transition">
      <div v-if="toast" class="fixed right-4 bottom-9 z-50 max-w-md rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2 text-xs shadow-xl">{{ toast }}</div>
    </Transition>
  </div>
</template>
