<script setup lang="ts">
import { tr } from '@/lib/i18n'
import { isProd, isReadOnly, loadPrefs, setPrefs } from '@/lib/guard'
import { computed, onBeforeUnmount, onMounted, provide, ref, shallowRef, watch } from 'vue'
import { confirmKey, toastKey } from '@/lib/confirm'
import { storeToRefs } from 'pinia'
import { Cordon, Delete, Drain, PodContainers, Restart, Scale, WorkloadPods } from '../wailsjs/go/api/ResourceAPI'
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
import PortForwardDialog from '@/components/PortForwardDialog.vue'
import PortForwards from '@/components/PortForwards.vue'
import TabStrip, { type TabLabel } from '@/components/TabStrip.vue'
import HostSlot from '@/components/HostSlot.vue'
import LogViewer from '@/components/LogViewer.vue'
import TerminalPane from '@/components/TerminalPane.vue'
import ContextMenu, { type MenuItem } from '@/components/ContextMenu.vue'
import {
  activeOf, closeTab as closeMainTab, dragging, ensure, forgetDock, moveTab, openTab as openMainTab,
  placeDock, placedDockIds, setView, split, unsplit, workspaces, type MainTab,
} from '@/lib/workspace'
import { dropHosts, hostOf } from '@/lib/hosts'
import { icons } from '@/lib/icons'
import { forwardRequest, forwards, forwardURL, openForward, trackForwards } from '@/lib/forward'
import { CheckUpdate } from '../wailsjs/go/api/AppAPI'
import { ClipboardSetText } from '../wailsjs/runtime/runtime'
import type { update } from '../wailsjs/go/models'
import Icon from '@/components/Icon.vue'
import CommandPalette, { type PaletteItem } from '@/components/CommandPalette.vue'
import ShortcutsHelp from '@/components/ShortcutsHelp.vue'
import SettingsDialog from '@/components/SettingsDialog.vue'
import { objects } from '@/lib/recents'
import { useClusters } from '@/stores/clusters'
import { usePanelWidth } from '@/lib/resize'
import { errMsg, newId, on } from '@/lib/bridge'
import { hue, initials } from '@/lib/dom'
import type { DockTab, NavigateTarget, NewDockTab, Row, RowAction, View } from '@/lib/types'

const store = useClusters()
const { contexts, active, server, selectedNamespaces, connecting } = storeToRefs(store)
onMounted(() => {
  store.load()
  void loadPrefs()
  trackForwards()
  // New-version notice: a few seconds after startup, then again on the first
  // use of each new day (the app often stays open for days).
  setTimeout(autoCheckUpdate, 4000)
  window.addEventListener('focus', autoCheckUpdate)
  dayTimer = window.setInterval(autoCheckUpdate, 60 * 60 * 1000)
})
onBeforeUnmount(() => {
  window.removeEventListener('focus', autoCheckUpdate)
  clearInterval(dayTimer)
})

const updateInfo = shallowRef<update.Info | null>(null)
const showUpdate = ref(false)
// Automatic check: one anonymous request to GitHub's releases endpoint, at most
// once per day (off when the user disabled it or on dev builds). The dialog
// shows at most once a day; "Depois" means "remind me tomorrow".
const UPDATE_DAY_KEY = 'leme:update-checked-day'
let dayTimer = 0
let checking = false
const today = () => new Date().toLocaleDateString('sv') // YYYY-MM-DD, local time
function lastCheckDay() {
  try { return localStorage.getItem(UPDATE_DAY_KEY) ?? '' } catch { return '' }
}
async function autoCheckUpdate() {
  const day = today()
  if (checking || showUpdate.value || lastCheckDay() === day) return
  checking = true
  try {
    const i = await CheckUpdate(true)
    try { localStorage.setItem(UPDATE_DAY_KEY, day) } catch { /* try again next time */ }
    if (i.available) { updateInfo.value = i; showUpdate.value = true }
  } catch { /* offline or rate-limited: stay quiet, retry later */ } finally {
    checking = false
  }
}

async function checkUpdateNow() {
  try {
    const i = await CheckUpdate(false)
    updateInfo.value = i
    if (i.available) showUpdate.value = true
    else flash(i.latest ? tr('Você está na versão mais recente (v{a}).', { a: i.latest }) : tr('Nenhuma versão publicada ainda.'))
  } catch (e) {
    flash(tr('Não foi possível verificar atualizações: {a}', { a: errMsg(e) }))
  }
}

const showKubeconfigs = ref(false)
const rancherKeyFor = ref('') // open the clusters dialog on this Rancher's API key field
const toast = ref('')
let toastTimer = 0
function flash(msg: string) {
  toast.value = msg
  clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = ''), 4500)
}

async function openCluster(name: string) {
  await store.activate(name)
  if (store.active === name) ensure(name) // first time: an Overview tab; later: the saved tabs
}

// ---- Main-area tabs and split panes (lib/workspace.ts) ----------------------
// One workspace per cluster; only each pane's active tab is mounted.
watch(active, (c) => { if (c) ensure(c) }, { immediate: true })
const ws = computed(() => (active.value ? workspaces[active.value] ?? null : null))
const focusedTab = computed(() => {
  const w = ws.value
  return w ? activeOf(w.panes[w.focus] ?? w.panes[0]) : null
})
// The sidebar shows and changes the focused pane's current tab.
const view = computed<View | null>({
  get: () => focusedTab.value?.view ?? null,
  set: (v) => {
    const w = ws.value
    if (!w || !v) return
    setView(w, w.focus, v)
    editing.value = null
  },
})
function openInNewTab(v: View) {
  const w = ws.value
  if (w) openMainTab(w, w.focus, v)
}

// Saved tabs keep the kind as it was; prefer the live catalog entry.
const kindFor = (k: resources.Kind) => store.catalog.find((c) => c.group === k.group && c.resource === k.resource) ?? k
const resourceKind = (t: MainTab) => (t.view.type === 'resource' ? kindFor(t.view.kind) : null)!

function tabLabel(t: MainTab): TabLabel {
  const v = t.view
  switch (v.type) {
    case 'overview': return { title: 'Overview', icon: 'overview' }
    case 'helm': return { title: 'Helm', icon: 'Helm' }
    case 'portforwards': return { title: 'Port Forwards', icon: 'forward' }
    case 'resource': return { title: v.kind.kind, sub: t.search || undefined, icon: icons[v.kind.category] ? v.kind.category : 'list' }
    case 'dock': {
      const d = tabs.value.find((x) => x.id === v.id)
      return d?.type === 'logs'
        ? { title: d.title, icon: 'logs', tone: 'text-amber-400' }
        : { title: d?.title ?? 'terminal', icon: 'terminal', tone: 'text-emerald-400' }
    }
  }
}

function closeMain(pane: number, id: string) {
  const w = ws.value
  const t = w?.panes[pane]?.tabs.find((x) => x.id === id)
  if (!w || !t) return
  if (t.view.type === 'dock') closeTab(t.view.id) // closing a stream's tab ends the stream
  else closeMainTab(w, pane, id)
}

// Split divider: drag to resize, double-click to center.
const panesEl = ref<HTMLElement | null>(null)
function startSplitDrag(e: MouseEvent) {
  const w = ws.value
  if (!w || !panesEl.value) return
  e.preventDefault()
  const r = panesEl.value.getBoundingClientRect()
  const move = (ev: MouseEvent) => (w.ratio = Math.min(0.8, Math.max(0.2, (ev.clientX - r.left) / r.width)))
  const up = () => {
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', up)
    document.body.style.cursor = ''
  }
  document.body.style.cursor = 'col-resize'
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
}
const paneStyle = (pi: number) => {
  const w = ws.value!
  return w.panes.length > 1 ? { flex: `${pi === 0 ? w.ratio : 1 - w.ratio} 1 0%` } : { flex: '1 1 0%' }
}

// Drop targets while a tab is dragged: beside (opens the split), or into a pane.
function dropZones(pi: number): ('here' | 'beside')[] {
  const w = ws.value, d = dragging.value
  if (!w || !d) return []
  if (w.panes.length === 1) {
    const zones: ('here' | 'beside')[] = []
    if (d.src === 'dock') zones.push('here')
    if (d.src === 'dock' || w.panes[0].tabs.length > 1) zones.push('beside')
    return zones
  }
  return d.src === 'dock' || d.pane !== pi ? ['here'] : []
}
function dropOnPane(pi: number, zone: 'here' | 'beside', at?: number) {
  const w = ws.value, d = dragging.value
  dragging.value = null
  if (!w || !d) return
  const to = zone === 'beside' ? w.panes.length : pi
  if (d.src === 'dock') placeDock(w, to, d.id)
  else moveTab(w, d.pane, d.id, to, at)
}
// A drag whose source element moved/unmounted may never fire dragend.
const endDrag = () => setTimeout(() => (dragging.value = null))
onMounted(() => { window.addEventListener('dragend', endDrag, true); window.addEventListener('drop', endDrag, true) })
onBeforeUnmount(() => { window.removeEventListener('dragend', endDrag, true); window.removeEventListener('drop', endDrag, true) })

// Right-click menus for main tabs and dock tabs (the no-drag way to do it all).
const tabMenu = shallowRef<{ x: number; y: number; title: string; items: MenuItem<string>[]; run: (v: string) => void } | null>(null)
function openMainMenu(e: MouseEvent, pane: number, id: string) {
  const w = ws.value
  const t = w?.panes[pane]?.tabs.find((x) => x.id === id)
  if (!w || !t) return
  const isDock = t.view.type === 'dock'
  const items: MenuItem<string>[] = []
  if (w.panes.length > 1) items.push({ value: 'move', label: tr('Mover para o outro painel') })
  else if (w.panes[0].tabs.length > 1) items.push({ value: 'beside', label: tr('Abrir ao lado') })
  if (!isDock) items.push({ value: 'dup', label: 'Duplicar aba' })
  if (isDock) items.push({ value: 'dock', label: tr('Voltar para o dock') })
  items.push({ value: 'close', label: tr('Fechar aba'), separatorBefore: true })
  if (w.panes[pane].tabs.length > 1) items.push({ value: 'others', label: tr('Fechar as outras abas') })
  tabMenu.value = {
    x: e.clientX, y: e.clientY, title: tabLabel(t).title, items,
    run: (v) => {
      if (v === 'move') moveTab(w, pane, id, pane === 0 ? 1 : 0)
      else if (v === 'beside') moveTab(w, pane, id, w.panes.length)
      else if (v === 'dup') openMainTab(w, pane, t.view, t.search)
      else if (v === 'dock' && t.view.type === 'dock') backToDock(t.view.id)
      else if (v === 'close') closeMain(pane, id)
      else if (v === 'others') for (const o of [...w.panes[pane].tabs]) if (o.id !== id) closeMain(pane, o.id)
    },
  }
}
// Right-click a cluster: connect / open / disconnect.
function openClusterMenu(e: MouseEvent, name: string) {
  const c = contexts.value.find((x) => x.name === name)
  if (!c) return
  const items: MenuItem<string>[] = []
  if (!c.connected) items.push({ value: 'connect', label: tr('Conectar') })
  else if (name !== active.value) items.push({ value: 'connect', label: tr('Abrir') })
  if (c.connected) items.push({ value: 'disconnect', label: tr('Desconectar'), danger: true, separatorBefore: items.length > 0 })
  items.push(
    { value: 'prod', label: isProd(name) ? tr('Desmarcar produção') : tr('Marcar como produção'), separatorBefore: true },
    { value: 'ro', label: isReadOnly(name) ? tr('Permitir alterações') : tr('Somente leitura') },
    { value: 'copy', label: tr('Copiar nome'), separatorBefore: true },
  )
  tabMenu.value = {
    x: e.clientX, y: e.clientY, title: name, items,
    run: (v) => {
      if (v === 'connect') void openCluster(name)
      else if (v === 'disconnect') disconnectCluster(name)
      else if (v === 'prod') void setPrefs(name, { production: !isProd(name) }).then(() => flash(isProd(name) ? tr('{a} marcado como produção', { a: name }) : tr('{a} não é mais produção', { a: name })))
      else if (v === 'ro') void setPrefs(name, { readOnly: !isReadOnly(name) }).then(() => flash(isReadOnly(name) ? tr('{a} em somente leitura', { a: name }) : tr('{a} aceita alterações', { a: name })))
      else if (v === 'copy') void ClipboardSetText(name)
    },
  }
}

// Disconnect: close that cluster's streams first (a node shell removes its pod
// on close), then the session; its saved main tabs come back on reconnect.
function disconnectCluster(name: string) {
  const streams = tabs.value.filter((t) => t.cluster === name)
  const fwds = forwards.value.filter((f) => f.cluster === name)
  const run = async () => {
    for (const t of streams) closeTab(t.id)
    if (editing.value?.cluster === name) editing.value = null
    await store.disconnect(name)
    flash(tr('Desconectado de {a}', { a: name }))
  }
  const parts = [
    streams.length && tr('{a} aba(s) de logs/terminal', { a: streams.length }),
    fwds.length && `${fwds.length} port-forward(s)`,
  ].filter(Boolean)
  if (parts.length) confirm(tr('Desconectar de {a}? Isso fecha {b}.', { a: name, b: parts.join(' e ') }), run)
  else void run().catch((e) => flash(errMsg(e)))
}

function openDockMenu(e: MouseEvent, id: string) {
  const w = ws.value
  const d = tabs.value.find((x) => x.id === id)
  if (!w || !d) return
  tabMenu.value = {
    x: e.clientX, y: e.clientY, title: d.title,
    items: [
      { value: 'main', label: tr('Abrir na área principal') },
      ...(w.panes.length === 1 ? [{ value: 'beside', label: tr('Abrir ao lado') }] : []),
      { value: 'close', label: tr('Fechar'), separatorBefore: true },
    ],
    run: (v) => {
      if (v === 'main') placeDock(w, w.focus, id)
      else if (v === 'beside') placeDock(w, w.panes.length, id)
      else if (v === 'close') closeTab(id)
    },
  }
}

// ---- YAML editor panel -----------------------------------------------------
// cluster: the editor is bound to the cluster it was opened in (never the
// active one), and closes when the user switches clusters.
const editing = shallowRef<{ cluster: string; kind?: resources.Ref; namespace?: string; name?: string; initialYaml?: string; key: string } | null>(null)
watch(active, () => (editing.value = null))

// A link (Controlled By, Namespace...) opens in the pane it was clicked in.
function navigate(t: NavigateTarget, pane?: number) {
  const k = store.findKind(t.kind)
  if (!k) return flash(tr('Tipo {a} não encontrado neste cluster', { a: t.kind }))
  const w = ws.value
  if (!w) return
  setView(w, pane ?? w.focus, { type: 'resource', kind: k }, t.search ?? '', t.open && t.search ? `${t.namespace ?? ''}/${t.search}` : undefined)
  editing.value = null
  if (t.namespace && k.namespaced && selectedNamespaces.value.length && !selectedNamespaces.value.includes(t.namespace)) {
    selectedNamespaces.value = [t.namespace]
  }
}

const currentKind = computed(() => (view.value?.type === 'resource' ? kindFor(view.value.kind) : null))

function createTemplate(k: resources.Kind): string {
  const apiVersion = k.group ? `${k.group}/${k.version}` : k.version
  const ns = selectedNamespaces.value.length === 1 ? selectedNamespaces.value[0] : 'default'
  const body = k.kind === 'ConfigMap' || k.kind === 'Secret' ? 'data:\n  chave: valor' : 'spec: {}'
  return `# Novo ${k.kind}: ajuste o manifesto e clique em Apply (⌘S).\n# Use "Diff" para validar no servidor antes (dry-run).\napiVersion: ${apiVersion}\nkind: ${k.kind}\nmetadata:\n  name: novo-${k.kind.toLowerCase()}\n${k.namespaced ? `  namespace: ${ns}\n` : ''}${body}\n`
}

// ---- Command palette (⌘K) and keyboard shortcuts --------------------------------
const showPalette = ref(false)
const showHelp = ref(false)
const showSettings = ref(false)
const isMac = /Mac/i.test(navigator.platform)
const mod = isMac ? '⌘' : 'Ctrl+'

const paletteItems = computed<PaletteItem[]>(() => {
  const out: PaletteItem[] = []
  const c = active.value
  const w = ws.value
  const go = (v: View) => () => { view.value = v }
  if (c && w) {
    out.push(
      { id: 'a:overview', group: tr('Ações'), label: tr('Visão geral'), icon: 'overview', run: go({ type: 'overview' }) },
      { id: 'a:newtab', group: tr('Ações'), label: tr('Nova aba'), icon: 'plus', keys: `${mod}T`, run: () => openMainTab(w, w.focus, { type: 'overview' }) },
      { id: 'a:split', group: tr('Ações'), label: w.panes.length > 1 ? tr('Juntar a tela') : tr('Dividir a tela'), icon: 'split', keys: `${mod}\\`, run: () => (w.panes.length > 1 ? unsplit(w) : split(w)) },
      { id: 'a:term', group: tr('Ações'), label: tr('Terminal local neste cluster'), icon: 'terminal', run: newTerminal },
      { id: 'a:helm', group: tr('Ações'), label: 'Helm releases', icon: 'list', run: go({ type: 'helm' }) },
      { id: 'a:pf', group: tr('Ações'), label: tr('Port-forwards ativos'), icon: 'forward', run: go({ type: 'portforwards' }) },
      { id: 'a:prod', group: tr('Ações'), label: isProd(c) ? tr('Desmarcar produção') : tr('Marcar este cluster como produção'), icon: 'ban', run: () => void setPrefs(c, { production: !isProd(c) }) },
      { id: 'a:ro', group: tr('Ações'), label: isReadOnly(c) ? tr('Permitir alterações neste cluster') : tr('Somente leitura neste cluster'), icon: 'ban', run: () => void setPrefs(c, { readOnly: !isReadOnly(c) }) },
    )
    const obj = (o: { kind: string; name: string; namespace: string }) => () => navigate({ kind: o.kind, search: o.name, namespace: o.namespace, open: true })
    for (const o of objects.pinned[c] ?? []) out.push({ id: `p:${o.kind}/${o.namespace}/${o.name}`, group: tr('Fixados'), label: o.name, sub: `${o.kind}${o.namespace ? ` · ${o.namespace}` : ''}`, icon: 'pin', run: obj(o) })
    for (const o of objects.recent[c] ?? []) out.push({ id: `r:${o.kind}/${o.namespace}/${o.name}`, group: tr('Recentes'), label: o.name, sub: `${o.kind}${o.namespace ? ` · ${o.namespace}` : ''}`, icon: 'refresh', run: obj(o) })
    for (const k of store.catalog) out.push({ id: `k:${k.group}/${k.resource}`, group: tr('Recursos'), label: k.kind, sub: k.group || 'core', icon: icons[k.category] ? k.category : 'list', run: go({ type: 'resource', kind: k }) })
  }
  out.push({ id: 'a:add', group: tr('Ações'), label: tr('Adicionar cluster (kubeconfig ou Rancher)'), icon: 'plus', run: () => { rancherKeyFor.value = ''; showKubeconfigs.value = true } })
  out.push({ id: 'a:settings', group: tr('Ações'), label: tr('Ajustes'), sub: 'English / Português', icon: 'sliders', run: () => (showSettings.value = true) })
  out.push({ id: 'a:help', group: tr('Ações'), label: tr('Atalhos de teclado'), icon: 'keyboard', keys: '?', run: () => (showHelp.value = true) })
  for (const x of contexts.value) if (x.name !== c) out.push({ id: `c:${x.name}`, group: tr('Clusters'), label: x.name, sub: x.connected ? tr('conectado') : undefined, icon: 'monitor', run: () => void openCluster(x.name) })
  return out
})

function onPaletteObject(h: resources.SearchHit, logs: boolean) {
  if (logs) {
    const k = store.findKind(h.kind)
    if (k) return void onAction('logs', { uid: '', name: h.name, namespace: h.namespace, cells: [] } as unknown as Row, k)
  }
  navigate({ kind: h.kind, search: h.name, namespace: h.namespace, open: true })
}

function selectTabAt(i: number) {
  const w = ws.value
  const p = w?.panes[w.focus]
  if (p && p.tabs[i]) p.active = p.tabs[i].id
}
function cycleTab(step: number) {
  const w = ws.value
  const p = w?.panes[w.focus]
  if (!p || !p.tabs.length) return
  const i = p.tabs.findIndex((t) => t.id === p.active)
  p.active = p.tabs[(i + step + p.tabs.length) % p.tabs.length].id
}
function onGlobalKey(e: KeyboardEvent) {
  const modKey = isMac ? e.metaKey : e.ctrlKey
  const k = e.key.toLowerCase()
  const el = e.target as HTMLElement | null
  const typing = !!el?.closest?.('input,textarea,select,[contenteditable],.monaco-editor,.xterm')
  if (modKey && !e.shiftKey && k === 'k') { e.preventDefault(); showPalette.value = !showPalette.value; return }
  if (showPalette.value || dialog.value) return
  const w = ws.value
  if (modKey && w) {
    if (k === 't' && !e.shiftKey) { e.preventDefault(); openMainTab(w, w.focus, { type: 'overview' }); return }
    if (k === 'w' && !e.shiftKey) {
      e.preventDefault()
      const p = w.panes[w.focus]
      if (p?.active) closeMain(w.focus, p.active)
      return
    }
    if (e.key === '\\') { e.preventDefault(); w.panes.length > 1 ? unsplit(w) : split(w); return }
    if (/^[1-9]$/.test(e.key) && !e.shiftKey) { e.preventDefault(); selectTabAt(Number(e.key) - 1); return }
    if (e.shiftKey && (e.key === ']' || e.key === '}')) { e.preventDefault(); cycleTab(1); return }
    if (e.shiftKey && (e.key === '[' || e.key === '{')) { e.preventDefault(); cycleTab(-1); return }
  }
  if (!typing && !modKey && e.key === '?') { e.preventDefault(); showHelp.value = !showHelp.value }
  else if (e.key === 'Escape' && showHelp.value) showHelp.value = false
}
onMounted(() => window.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKey))

// ---- Dock --------------------------------------------------------------------
const tabs = ref<DockTab[]>([])
const activeTab = ref('')
const dockMax = ref(false)
// Tabs are bound to the cluster they were opened in. Switching clusters shows
// only that cluster's tabs; the others stay alive (hidden) and come back.
// A tab moved to a main-area pane leaves the dock's bar while it's there.
const placed = computed(() => placedDockIds())
const dockTabs = computed(() => tabs.value.filter((t) => !placed.value.has(t.id)))
const lastTabByCluster = new Map<string, string>()
const clusterTabs = (c: string) => dockTabs.value.filter((t) => t.cluster === c)
watch(placed, (p) => { if (p.has(activeTab.value)) activeTab.value = clusterTabs(active.value).at(-1)?.id ?? '' })
function backToDock(id: string) {
  forgetDock(id)
  activeTab.value = id
}
// Hosts of closed tabs go once their components have unmounted.
watch(() => tabs.value.map((t) => t.id), (ids) => dropHosts(new Set(ids)), { flush: 'post' })
function openTab(t: NewDockTab) {
  const tab = { ...t, cluster: active.value } as DockTab
  tabs.value.push(tab)
  activeTab.value = tab.id
}
function closeTab(id: string) {
  forgetDock(id)
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
// typeName: production clusters ask for the cluster name before risky actions.
const dialog = ref<{ title: string; input?: number; danger?: boolean; typeName?: string; run: (n?: number) => Promise<void> } | null>(null)
const typed = ref('')
watch(dialog, () => (typed.value = ''))
const dialogReady = computed(() => !dialog.value?.typeName || typed.value.trim() === dialog.value.typeName)
async function confirmDialog() {
  const d = dialog.value!
  if (!dialogReady.value) return
  dialog.value = null
  try { await d.run(d.input) } catch (e) { flash(errMsg(e)) }
}
const confirm = (title: string, run: () => Promise<void>, danger = false) =>
  (dialog.value = { title, run, danger, typeName: danger && isProd(active.value) ? active.value : undefined })
provide(confirmKey, confirm)
provide(toastKey, (m: string) => flash(m))
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
      if (k.group === '' && k.resource === 'pods') {
        const pc = await PodContainers(c, ns, row.name).catch((e) => (flash(errMsg(e)), null))
        if (!pc?.containers?.length) return
        openTab({ id: newId(), type: 'logs', title: row.name, namespace: ns, targets: pc.containers.map((ct) => ({ pod: row.name, container: ct })) })
      } else {
        // Workload (Deployment, StatefulSet, DaemonSet, ReplicaSet, Job): all its pods.
        const w = await WorkloadPods(c, k, ns, row.name).catch((e) => (flash(errMsg(e)), null))
        if (!w) return
        if (!w.targets?.length) return flash(tr('{a} {b} não tem pods no momento', { a: k.kind, b: row.name }))
        openTab({
          id: newId(), type: 'logs', title: `${k.kind.toLowerCase()} ${row.name}`, namespace: ns, targets: w.targets,
          note: w.total > w.targets.length ? tr('mostrando {a} de {b} streams', { a: w.targets.length, b: w.total }) : undefined,
        })
      }
      break
    case 'shell': {
      const pc = await PodContainers(c, ns, row.name).catch((e) => (flash(errMsg(e)), null))
      if (!pc?.containers?.length) return
      openTab({ id: newId(), type: 'shell', title: row.name, target: { namespace: ns, pod: row.name, container: pc.containers[0], command: [] } })
      break
    }
    case 'portforward':
      openForward({ cluster: c, kind: k.resource === 'services' ? 'service' : 'pod', namespace: ns, name: row.name })
      break
    case 'nodeshell':
      confirm(
        tr('Abrir um shell root no node {a}? O Leme cria um pod privilegiado temporário em kube-system nesse node (acesso total à máquina) e o remove ao fechar o terminal.', { a: row.name }),
        async () => openTab({ id: newId(), type: 'shell', title: `node ${row.name}`, node: row.name }),
        true,
      )
      break
    case 'delete':
      confirm(tr('Excluir {a} {b}?', { a: k.kind, b: row.name }), async () => { await Delete(c, k, ns, row.name, false); flash(tr('{a} excluído', { a: row.name })) }, true)
      break
    case 'scale':
      dialog.value = { title: tr('Escalar {a} para quantas réplicas?', { a: row.name }), input: 1, typeName: isProd(c) ? c : undefined,
        run: async (n) => { await Scale(c, k, ns, row.name, n ?? 0); flash(tr('{a} → {b} réplica(s)', { a: row.name, b: n })) } }
      break
    case 'restart':
      confirm(tr('Reiniciar (rollout restart) {a}?', { a: row.name }), async () => { await Restart(c, k, ns, row.name); flash(tr('{a} reiniciando', { a: row.name })) }, isProd(c))
      break
    case 'cordon':
    case 'uncordon':
      if (isProd(c)) confirm(tr('{a} no node {b}?', { a: action === 'cordon' ? 'Cordon' : 'Uncordon', b: row.name }), async () => { await Cordon(c, row.name, action === 'cordon'); flash(`${row.name}: ${action}`) }, true)
      else await Cordon(c, row.name, action === 'cordon').then(() => flash(`${row.name}: ${action}`)).catch((e) => flash(errMsg(e)))
      break
    case 'drain':
      confirm(tr('Drain do node {a}? Todos os pods serão removidos (respeitando PDBs).', { a: row.name }), async () => {
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
      namespace: ns, targets: [{ pod: row.name, container }], previous: action === 'previous',
    })
  }
}

function bulkDelete(rows: Row[], k: resources.Kind) {
  confirm(tr('Excluir {a} {b}(s)?', { a: rows.length, b: k.kind }), async () => {
    const res = await Promise.allSettled(rows.map((r) => Delete(active.value, k, r.namespace ?? '', r.name, false)))
    const failed = res.filter((r) => r.status === 'rejected').length
    flash(failed ? tr('{a} excluído(s), {b} falha(s)', { a: rows.length - failed, b: failed }) : tr('{a} excluído(s)', { a: rows.length }))
  }, true)
}

const editorPanel = usePanelWidth('editor', 640)

const nsLabel = computed(() => (selectedNamespaces.value.length ? selectedNamespaces.value.join(', ') : tr('todos os namespaces')))
</script>

<template>
  <div class="flex h-full flex-col bg-zinc-950 text-zinc-200">
    <div
      v-if="active && (isProd(active) || isReadOnly(active))"
      class="flex h-5 shrink-0 items-center justify-center gap-2 text-[10px] font-semibold tracking-[0.18em] uppercase"
      :class="isProd(active) ? 'bg-rose-700 text-rose-50' : 'bg-zinc-800 text-zinc-300'"
    >
      <span v-if="isProd(active)">{{ tr('Produção') }} · {{ active }}</span>
      <span v-if="isReadOnly(active)">{{ isProd(active) ? '· ' : '' }}{{ tr('somente leitura') }}{{ isProd(active) ? '' : ` · ${active}` }}</span>
    </div>
    <div class="flex min-h-0 flex-1">
      <Sidebar v-model:view="view" @settings="showSettings = true" @open-tab="openInNewTab" @navigate="(n) => navigate(n)" @cluster-menu="openClusterMenu" @add-kubeconfig="rancherKeyFor = ''; showKubeconfigs = true" @rancher-key="(u) => { rancherKeyFor = u; showKubeconfigs = true }" @open-cluster="openCluster" :update-available="!!updateInfo?.available" @check-update="checkUpdateNow" />

      <main class="flex min-w-0 flex-1 flex-col">
        <div v-show="!dockMax" class="flex min-h-0 flex-1">
          <section ref="panesEl" class="flex min-w-0 flex-1">
            <template v-if="active && ws">
              <template v-for="(p, pi) in ws.panes" :key="pi">
                <div
                  v-if="pi > 0" class="relative z-10 -mx-0.5 w-1 shrink-0 cursor-col-resize bg-zinc-800 transition-colors hover:bg-sky-500/50"
                  :title="tr('Arraste para redimensionar (duplo clique: metade)')" @mousedown="startSplitDrag" @dblclick="ws.ratio = 0.5"
                />
                <div class="flex min-w-0 flex-col" :style="paneStyle(pi)" @mousedown.capture="ws.focus = pi">
                  <TabStrip
                    :pane="pi" :tabs="p.tabs" :active="p.active" :focused="ws.panes.length > 1 && ws.focus === pi" :label="tabLabel"
                    :split-state="ws.panes.length === 1 ? 'can-split' : pi === ws.panes.length - 1 ? 'can-unsplit' : 'none'"
                    @select="(id) => { p.active = id; ws!.focus = pi }" @close="(id) => closeMain(pi, id)"
                    @new="openMainTab(ws!, pi, { type: 'overview' })" @split="split(ws!)" @unsplit="unsplit(ws!)"
                    @drop="(at) => dropOnPane(pi, 'here', at)" @menu="(e, id) => openMainMenu(e, pi, id)"
                  />
                  <div class="relative min-h-0 flex-1">
                    <template v-for="t in p.tabs" :key="t.id">
                      <template v-if="t.id === p.active">
                        <HostSlot v-if="t.view.type === 'dock'" :id="t.view.id" />
                        <div v-else class="absolute inset-0">
                          <Overview v-if="t.view.type === 'overview'" :key="active" :cluster="active" @navigate="(n) => navigate(n, pi)" />
                          <HelmReleases v-else-if="t.view.type === 'helm'" :key="active" :cluster="active" @confirm="confirm" @toast="flash" />
                          <PortForwards v-else-if="t.view.type === 'portforwards'" @toast="flash" />
                          <ResourceTable
                            v-else-if="t.view.type === 'resource'" :key="`${active}|${t.view.kind.group}/${t.view.kind.resource}`"
                            v-model:search="t.search" :cluster="active" :kind="resourceKind(t)"
                            :focus="t.focus" :focused="pi === ws!.focus" @focus-done="t.focus = undefined"
                            @action="(a, r, k) => onAction(a, r, k ?? resourceKind(t))" @bulk-delete="(rows) => bulkDelete(rows, resourceKind(t))"
                            @navigate="(n) => navigate(n, pi)" @container="onContainer"
                            @create="editing = { cluster: active, initialYaml: createTemplate(resourceKind(t)), key: newId() }"
                          />
                        </div>
                      </template>
                    </template>
                    <div v-if="!p.tabs.length" class="grid h-full place-items-center text-sm text-zinc-500">
                      <div class="text-center">
                        <p>{{ tr('Nenhuma aba aberta.') }}</p>
                        <button class="mt-2 text-sky-400 hover:underline" @click="openMainTab(ws!, pi, { type: 'overview' })">{{ tr('Abrir Overview') }}</button>
                      </div>
                    </div>

                    <!-- Drop targets while dragging a tab -->
                    <div v-if="dropZones(pi).length" class="absolute inset-0 z-30 flex gap-2 p-2">
                      <div
                        v-for="z in dropZones(pi)" :key="z"
                        class="grid flex-1 place-items-center rounded-lg border-2 border-dashed border-sky-600/50 bg-zinc-950/70 text-xs text-sky-300 backdrop-blur-[1px] [&.drop-hot]:border-sky-400 [&.drop-hot]:bg-sky-950/60"
                        @dragover.prevent="($event.currentTarget as HTMLElement).classList.add('drop-hot')"
                        @dragleave="($event.currentTarget as HTMLElement).classList.remove('drop-hot')"
                        @drop.prevent="dropOnPane(pi, z)"
                      >{{ z === 'beside' ? tr('Abrir ao lado') : ws.panes.length > 1 ? tr('Mover para este painel') : tr('Abrir aqui') }}</div>
                    </div>
                  </div>
                </div>
              </template>
            </template>

            <!-- Welcome -->
            <div v-else class="grid h-full place-items-center overflow-auto p-8">
              <div class="w-full max-w-3xl">
                <h1 class="text-xl font-semibold text-zinc-100">{{ tr('Bem-vindo ao Leme') }}</h1>
                <p class="mt-1 text-sm text-zinc-500">{{ tr('Escolha um cluster para conectar. Nada é acessado até você clicar.') }}</p>
                <div class="mt-6 grid gap-3 sm:grid-cols-2">
                  <button
                    v-for="c in contexts" :key="c.name"
                    class="group flex items-center gap-3 rounded-xl border border-zinc-800 bg-zinc-900/50 p-3 text-left transition hover:border-sky-700 hover:bg-zinc-900"
                    @click="openCluster(c.name)" @contextmenu.prevent="openClusterMenu($event, c.name)"
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
                    <Icon name="plus" /> {{ tr('Adicionar cluster') }}
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section v-if="editing && active" class="relative shrink-0 border-l border-zinc-800" :style="{ width: `${editorPanel.width.value}px` }">
            <div
              class="absolute inset-y-0 -left-1 z-20 w-2 cursor-col-resize transition-colors hover:bg-sky-500/40"
              :class="editorPanel.dragging.value && 'bg-sky-500/60'"
              :title="tr('Arraste para redimensionar (duplo clique: tamanho padrão)')"
              @mousedown="editorPanel.start" @dblclick="editorPanel.reset"
            />
            <YamlEditor
              :key="editing.key" :cluster="editing.cluster" :kind="editing.kind" :namespace="editing.namespace"
              :name="editing.name" :initial-yaml="editing.initialYaml"
              @close="editing = null" @applied="flash(tr('Aplicado com sucesso'))"
            />
          </section>
        </div>

        <Dock
          v-if="active" v-model:active="activeTab" v-model:maximized="dockMax"
          :cluster="active" :tabs="dockTabs" @close="closeTab" @new-terminal="newTerminal"
          @menu="openDockMenu" @drop-back="backToDock"
        />
      </main>
    </div>

    <!-- Status bar -->
    <footer class="flex h-6 shrink-0 items-center gap-3 border-t border-zinc-800 bg-zinc-925 px-3 text-[11px] text-zinc-500">
      <template v-if="active">
        <span class="flex items-center gap-1.5 text-zinc-300"><span class="h-1.5 w-1.5 rounded-full bg-emerald-400" />{{ active }}</span>
        <span>{{ server?.version }}</span>
        <span class="truncate">{{ server?.server }}</span>
        <button v-if="forwards.length" class="hover:text-sky-300" :title="tr('Port-forwards ativos')" @click="view = { type: 'portforwards' }">{{ forwards.length }} port-forward{{ forwards.length > 1 ? 's' : '' }}</button>
        <span class="ml-auto truncate">{{ nsLabel }}</span>
      </template>
      <span v-else>{{ tr('desconectado') }}</span>
    </footer>

    <!-- Dock streams: rendered once into stable hosts, shown in the dock or a pane.
         Each talks to ITS tab's cluster (t.cluster), never the active one. -->
    <Teleport v-for="t in tabs" :key="t.id" :to="hostOf(t.id)">
      <LogViewer v-if="t.type === 'logs'" :cluster="t.cluster" :namespace="t.namespace" :targets="t.targets" :previous="t.previous" :note="t.note" />
      <TerminalPane v-else :cluster="t.cluster" :target="t.target" :node="t.node" />
    </Teleport>
    <ContextMenu v-if="tabMenu" :x="tabMenu.x" :y="tabMenu.y" :title="tabMenu.title" :items="tabMenu.items" @select="tabMenu.run" @close="tabMenu = null" />

    <PortForwardDialog
      v-if="forwardRequest" :target="forwardRequest" @close="forwardRequest = null"
      @started="(f) => flash(tr('Encaminhando {a} (endereço copiado)', { a: forwardURL(f) }))"
    />
    <CommandPalette
      v-if="showPalette" :items="paletteItems" :cluster="active" :namespaces="selectedNamespaces"
      @close="showPalette = false" @object="onPaletteObject"
    />
    <ShortcutsHelp v-if="showHelp" @close="showHelp = false" />
    <SettingsDialog v-if="showSettings" @close="showSettings = false" />
    <UpdateDialog v-if="showUpdate && updateInfo" :info="updateInfo" @close="showUpdate = false" />
    <KubeconfigManager v-if="showKubeconfigs" :rancher="rancherKeyFor || undefined" @close="showKubeconfigs = false; rancherKeyFor = ''" @changed="store.load()" @connect="openCluster" />

    <!-- Dialog -->
    <div v-if="dialog" class="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-[2px]" @click.self="dialog = null">
      <div class="w-[420px] rounded-xl border border-zinc-800 bg-zinc-900 p-5 text-sm shadow-2xl">
        <p v-if="dialog.typeName" class="mb-2 text-[11px] font-semibold tracking-wider text-rose-400 uppercase">{{ tr('Cluster de produção') }}</p>
        <p class="mb-4 text-zinc-100">{{ dialog.title }}</p>
        <input
          v-if="dialog.input !== undefined" v-model.number="dialog.input" type="number" min="0" autofocus
          class="mb-4 w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 outline-none focus:border-sky-600"
          @keydown.enter="confirmDialog"
        />
        <label v-if="dialog.typeName" class="mb-4 block text-xs text-zinc-400">
          {{ tr('Digite') }} <span class="font-mono text-zinc-100 select-text">{{ dialog.typeName }}</span> {{ tr('para confirmar') }}
          <input
            v-model="typed" autofocus spellcheck="false" autocomplete="off"
            class="mt-1.5 w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 font-mono text-sm outline-none focus:border-rose-600"
            @keydown.enter="confirmDialog"
          />
        </label>
        <div class="flex justify-end gap-2">
          <button class="rounded-md bg-zinc-800 px-3 py-1.5 hover:bg-zinc-700" @click="dialog = null">{{ tr('Cancelar') }}</button>
          <button
            class="rounded-md px-3 py-1.5 text-white disabled:opacity-40" :disabled="!dialogReady"
            :class="dialog.danger || dialog.typeName ? 'bg-rose-600 hover:bg-rose-500' : 'bg-sky-600 hover:bg-sky-500'" @click="confirmDialog"
          >{{ tr('Confirmar') }}</button>
        </div>
      </div>
    </div>
    <Transition enter-from-class="translate-y-2 opacity-0" leave-to-class="opacity-0" enter-active-class="transition" leave-active-class="transition">
      <div v-if="toast" class="fixed right-4 bottom-9 z-50 max-w-md rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2 text-xs shadow-xl">{{ toast }}</div>
    </Transition>
  </div>
</template>
