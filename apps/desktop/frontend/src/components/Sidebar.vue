<script setup lang="ts">
/** Cluster switcher + navigation tree (sections with icons, CRDs by group). */
import { tr } from '@/lib/i18n'
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import type { cluster, resources } from '../../wailsjs/go/models'
import { useClusters } from '@/stores/clusters'
import { loginNeeded, stateLabel, useRancher } from '@/stores/rancher'
import { errMsg } from '@/lib/bridge'
import { isProd, isReadOnly } from '@/lib/guard'
import { AppVersion, ApproveExec } from '../../wailsjs/go/api/ClusterAPI'
import type { NavigateTarget, View } from '@/lib/types'
import { objects, togglePin, type ObjRef } from '@/lib/recents'
import { hue, initials, useDismiss } from '@/lib/dom'
import { themePref, type ThemePref } from '@/lib/theme'
import Icon from './Icon.vue'
import { forwards } from '@/lib/forward'

const view = defineModel<View | null>('view', { required: true })
defineProps<{ updateAvailable?: boolean }>()
const emit = defineEmits<{ addKubeconfig: []; openCluster: [name: string]; checkUpdate: []; settings: []; openTab: [view: View]; clusterMenu: [e: MouseEvent, name: string]; rancherKey: [url: string]; navigate: [target: NavigateTarget] }>()

const store = useClusters()
const { contexts, active, sections, connecting, error, failed, catalog } = storeToRefs(store)

// Rancher clusters: origin tag, session state, and one-click login when it expired.
const ranchers = useRancher()
onMounted(ranchers.load)
const rancherOf = (c: cluster.ContextInfo) => ranchers.serverOf(c.source)
const needLogin = computed(() => loginNeeded(error.value))
// Plain connection errors go away on their own after a few seconds (the
// interactive ones, Rancher login and command approval, stay until answered).
const networkHint = computed(() => /i\/o timeout|deadline exceeded|no such host|connection refused|network is unreachable/i.test(error.value))
let errTimer = 0
watch(error, (e) => {
  clearTimeout(errTimer)
  if (!e || loginNeeded(e) || /^exec-approval:/.test(e)) return
  errTimer = window.setTimeout(() => { if (error.value === e) error.value = '' }, 12000)
})
// A kubeconfig wants to run a command for credentials: ask once per command.
const execAsk = computed(() => {
  const m = /^exec-approval:([^|]+)\|(.*)$/s.exec(error.value)
  return m ? { context: m[1], command: m[2] } : null
})
async function allowExec() {
  const a = execAsk.value
  if (!a) return
  try {
    await ApproveExec(a.context)
    await store.activate(a.context)
  } catch (e) {
    error.value = errMsg(e)
  }
}

// Pinned and recent objects of the active cluster.
const pinnedObjs = computed(() => (active.value ? objects.pinned[active.value] ?? [] : []))
const recentObjs = computed(() => (active.value ? (objects.recent[active.value] ?? []).slice(0, 5) : []))
const quickOpen = ref((() => { try { return localStorage.getItem('leme:quick') !== '0' } catch { return true } })())
watch(quickOpen, (v) => { try { localStorage.setItem('leme:quick', v ? '1' : '0') } catch { /* storage unavailable */ } })
const openObj = (o: ObjRef) => emit('navigate', { kind: o.kind, search: o.name, namespace: o.namespace, open: true })
watch(needLogin, (n) => n && ranchers.fetchInfo(n.url), { immediate: true })
const loggingIn = ref(false)
// Session of the active cluster's Rancher: expired, or expiring within 3 days.
const activeRancher = computed(() => {
  const c = contexts.value.find((x) => x.name === active.value)
  return c ? rancherOf(c) : undefined
})
watch(() => activeRancher.value?.url, (u) => u && ranchers.fetchInfo(u), { immediate: true })
const expiresIn = computed(() => {
  const at = activeRancher.value?.expiresAt
  const ms = at ? Date.parse(at) - Date.now() : NaN
  return Number.isNaN(ms) ? null : ms
})
const rancherBanner = computed(() => {
  const r = activeRancher.value
  if (!r || needLogin.value) return null
  if (r.state === 'expired' || r.state === 'logged-out') return { tone: 'warn', text: tr('A sessão do Rancher {a} expirou.', { a: r.name }), url: r.url }
  const ms = expiresIn.value
  if (r.state === 'ok' && ms !== null && ms < 3 * 86400_000) {
    const h = Math.max(1, Math.round(ms / 3600_000))
    return { tone: 'soon', text: `A sessão do Rancher ${r.name} expira em ${h < 48 ? `${h} h` : `${Math.round(h / 24)} dias`}.`, url: r.url }
  }
  return null
})
async function renew(url: string) {
  if (ranchers.browserLogin(url) === false) return emit('rancherKey', url)
  loggingIn.value = true
  try {
    await ranchers.login(url)
    if (active.value) await store.activate(active.value)
  } catch (e) {
    if (!/cancel/i.test(errMsg(e))) error.value = errMsg(e)
  } finally {
    loggingIn.value = false
  }
}

async function loginAndRetry() {
  const n = needLogin.value
  if (!n) return
  loggingIn.value = true
  try {
    await ranchers.login(n.url)
    if (failed.value) await store.activate(failed.value)
  } catch (e) {
    if (!/cancel/i.test(errMsg(e))) error.value = errMsg(e)
  } finally {
    loggingIn.value = false
  }
}

// Conventional ordering inside sections; anything else follows alphabetically.
const ORDER = [
  'Pod', 'Deployment', 'DaemonSet', 'StatefulSet', 'ReplicaSet', 'Job', 'CronJob',
  'ConfigMap', 'Secret', 'ResourceQuota', 'LimitRange', 'HorizontalPodAutoscaler', 'PodDisruptionBudget',
  'Service', 'Endpoints', 'EndpointSlice', 'Ingress', 'IngressClass', 'NetworkPolicy',
  'PersistentVolumeClaim', 'PersistentVolume', 'StorageClass',
  'ServiceAccount', 'ClusterRole', 'Role', 'ClusterRoleBinding', 'RoleBinding',
]
const rank = (k: resources.Kind) => (ORDER.indexOf(k.kind) + 1 || 999)
const sorted = (ks: resources.Kind[]) => [...ks].sort((a, b) => rank(a) - rank(b) || a.kind.localeCompare(b.kind))

// Single-kind sections render as direct links (Nodes, Namespaces, Events).
const LEAF = new Set(['Nodes', 'Namespaces', 'Events'])
const leafKind = (section: string) =>
  catalog.value.find((k) => k.group === '' && k.resource === section.toLowerCase())

const tree = computed(() =>
  sections.value.map((s) => {
    if (s.name !== 'Custom Resources') return { name: s.name, kinds: sorted(s.kinds), groups: [] as { group: string; kinds: resources.Kind[] }[] }
    const byGroup = new Map<string, resources.Kind[]>()
    for (const k of s.kinds) {
      if (k.resource === 'customresourcedefinitions') continue
      if (!byGroup.has(k.group)) byGroup.set(k.group, [])
      byGroup.get(k.group)!.push(k)
    }
    const groups = [...byGroup].sort(([a], [b]) => a.localeCompare(b)).map(([group, kinds]) => ({ group, kinds: sorted(kinds) }))
    const crd = s.kinds.filter((k) => k.resource === 'customresourcedefinitions')
    return { name: s.name, kinds: crd, groups }
  }),
)

// Expanded sections, remembered across restarts.
const open = ref<Set<string>>(new Set((() => {
  try { return JSON.parse(localStorage.getItem('leme:nav') ?? '["Workloads"]') as string[] } catch { return ['Workloads'] }
})()))
watch(open, (v) => { try { localStorage.setItem('leme:nav', JSON.stringify([...v])) } catch { /* ignore */ } }, { deep: true })
const toggle = (key: string) => (open.value.has(key) ? open.value.delete(key) : open.value.add(key))

const filter = ref('')
const matches = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return null
  return catalog.value.filter((k) =>
    k.kind.toLowerCase().includes(q) || k.resource.includes(q) || k.shortNames?.some((s) => s === q) || k.group.includes(q))
})

const isActive = (k: resources.Kind) =>
  view.value?.type === 'resource' && view.value.kind.group === k.group && view.value.kind.resource === k.resource
// Plain click: show in the current tab. Middle click or ⌘/Ctrl-click: new tab.
function nav(v: View, e?: MouseEvent) {
  if (e && (e.button === 1 || e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    emit('openTab', v)
  } else if (!e || e.button === 0) view.value = v
}
const go = (k: resources.Kind, e?: MouseEvent) => nav({ type: 'resource', kind: k }, e)
const clusterOpen = ref(true)

// Cluster list mode: 'compact' shows only the active cluster (switch via a
// popover) so the resource tree gets the full height; 'list' shows them all.
const clusterView = ref<'compact' | 'list'>((() => {
  try { return localStorage.getItem('leme:cluster-view') === 'list' ? 'list' : 'compact' } catch { return 'compact' }
})())
watch(clusterView, (v) => { try { localStorage.setItem('leme:cluster-view', v) } catch { /* ignore */ } })
const compact = computed(() => clusterView.value === 'compact' && !!active.value)
const activeCtx = computed(() => contexts.value.find((c) => c.name === active.value))

const switcherOpen = ref(false)
const switcherQ = ref('')
const switcherRoot = ref<HTMLElement | null>(null)
useDismiss(switcherRoot, () => (switcherOpen.value = false))
const switcherList = computed(() => {
  const q = switcherQ.value.trim().toLowerCase()
  return q ? contexts.value.filter((c) => c.name.toLowerCase().includes(q)) : contexts.value
})
function pick(name: string) {
  switcherOpen.value = false
  switcherQ.value = ''
  emit('openCluster', name)
}

const version = ref('')
onMounted(async () => (version.value = await AppVersion().catch(() => '')))

const THEMES: { value: ThemePref; icon: string; label: string }[] = [
  { value: 'system', icon: 'monitor', label: 'Seguir o sistema' },
  { value: 'dark', icon: 'moon', label: 'Escuro' },
  { value: 'light', icon: 'sun', label: 'Claro' },
]
</script>

<template>
  <aside class="flex w-64 shrink-0 flex-col border-r border-zinc-800/80 bg-zinc-925 text-[13px] select-none">
    <!-- Clusters -->
    <div class="flex items-center px-3 pt-3 pb-1.5">
      <button class="flex items-center gap-1 text-[11px] font-semibold tracking-wider text-zinc-500 uppercase hover:text-zinc-300" @click="clusterOpen = !clusterOpen">
        <Icon name="chevron" :size="12" class="transition-transform" :class="clusterOpen && 'rotate-90'" /> {{ tr('Clusters') }}
      </button>
      <button
        v-if="active" class="ml-auto grid h-6 w-6 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100"
        :title="clusterView === 'compact' ? tr('Mostrar todos os clusters') : tr('Mostrar só o cluster atual')"
        @click="clusterView = clusterView === 'compact' ? 'list' : 'compact'"
      >
        <Icon :name="clusterView === 'compact' ? 'list' : 'single'" :size="14" />
      </button>
      <button class="grid h-6 w-6 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :class="!active && 'ml-auto'" :title="tr('Adicionar cluster (colar configuração ou escolher arquivo)')" @click="emit('addKubeconfig')">
        <Icon name="plus" :size="14" />
      </button>
    </div>

    <!-- Compact: only the active cluster, switch through a searchable popover -->
    <div v-if="compact && clusterOpen" ref="switcherRoot" class="relative px-1.5 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg border border-zinc-800 bg-zinc-900/70 px-2 py-2 text-left hover:border-zinc-700"
        :title="tr('Trocar de cluster (botão direito: desconectar)')" @click="switcherOpen = !switcherOpen"
        @contextmenu.prevent="emit('clusterMenu', $event, active)"
      >
        <span class="relative grid h-7 w-7 shrink-0 place-items-center rounded-md text-[11px] font-bold text-white" :style="{ background: `hsl(${hue(active)} 55% 42%)` }">
          {{ initials(active) }}
          <span class="absolute -right-0.5 -bottom-0.5 h-2 w-2 rounded-full border-2 border-zinc-925 bg-emerald-400" />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-zinc-100">{{ active }}</span>
          <span class="block truncate text-[11px] text-zinc-500">{{ activeCtx?.auth }} · {{ contexts.length }} clusters</span>
        </span>
        <Icon name="swap" :size="14" class="text-zinc-500" />
      </button>
      <div v-if="switcherOpen" class="absolute top-full right-1.5 left-1.5 z-40 mt-1 overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/40">
        <div class="border-b border-zinc-800 p-2">
          <input
            v-model="switcherQ" :placeholder="tr('Buscar cluster…')" autofocus
            class="w-full rounded bg-zinc-950 px-2 py-1.5 text-xs outline-none placeholder:text-zinc-600"
            @keydown.enter="switcherList[0] && pick(switcherList[0].name)" @keydown.esc="switcherOpen = false"
          />
        </div>
        <div class="max-h-72 overflow-auto py-1">
          <button
            v-for="c in switcherList" :key="c.name"
            class="flex w-full items-center gap-2.5 px-2.5 py-1.5 text-left hover:bg-zinc-800"
            :class="c.name === active && 'text-sky-300'" @click="pick(c.name)"
            @contextmenu.prevent="switcherOpen = false; emit('clusterMenu', $event, c.name)"
          >
            <span class="relative grid h-5 w-5 shrink-0 place-items-center rounded text-[9px] font-bold text-white" :style="{ background: `hsl(${hue(c.name)} 55% 42%)` }">
              {{ initials(c.name) }}
              <span v-if="c.connected" class="absolute -right-0.5 -bottom-0.5 h-1.5 w-1.5 rounded-full bg-emerald-400" />
            </span>
            <span class="min-w-0 flex-1 truncate">{{ c.name }}</span>
            <span v-if="connecting === c.name" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
          </button>
          <p v-if="!switcherList.length" class="px-3 py-2 text-xs text-zinc-500">{{ tr('Nenhum cluster.') }}</p>
        </div>
      </div>
    </div>

    <!-- List: every cluster -->
    <div v-show="clusterOpen && !compact" class="max-h-52 overflow-auto px-1.5 pb-2">
      <button
        v-for="c in contexts" :key="c.name"
        class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left hover:bg-zinc-800/70"
        :class="c.name === active && 'bg-zinc-800 text-zinc-50'"
        :title="tr('{a}\nauth: {b}\n{c}\n\nBotão direito: conectar / desconectar', { a: c.server, b: c.auth, c: c.source })" @click="emit('openCluster', c.name)"
        @contextmenu.prevent="emit('clusterMenu', $event, c.name)"
      >
        <span
          class="relative grid h-6 w-6 shrink-0 place-items-center rounded-md text-[10px] font-bold text-white"
          :style="{ background: `hsl(${hue(c.name)} 55% 42%)` }"
        >
          {{ initials(c.name) }}
          <span v-if="c.connected" class="absolute -right-0.5 -bottom-0.5 h-2 w-2 rounded-full border-2 border-zinc-925 bg-emerald-400" />
        </span>
        <span class="min-w-0 flex-1 truncate">{{ c.name }}</span>
        <span v-if="isProd(c.name)" class="shrink-0 rounded bg-rose-600/90 px-1 text-[9px] font-semibold tracking-wide text-white uppercase" :title="tr('Cluster de produção')">{{ tr('prod') }}</span>
        <span v-if="isReadOnly(c.name)" class="shrink-0 rounded bg-zinc-800 px-1 text-[9px] tracking-wide text-zinc-400 uppercase" :title="tr('Somente leitura: nada pode ser alterado')">{{ tr('leitura') }}</span>
        <span
          v-if="rancherOf(c)" class="shrink-0 rounded px-1 text-[9px] tracking-wide uppercase"
          :class="rancherOf(c)!.state === 'ok' ? 'bg-zinc-800 text-zinc-500' : 'bg-amber-500/15 text-amber-300'"
          :title="`Rancher ${rancherOf(c)!.name}: ${tr(stateLabel[rancherOf(c)!.state] ?? rancherOf(c)!.state)}`"
        >{{ tr('rancher') }}</span>
        <span v-if="connecting === c.name" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
      </button>
      <p v-if="!contexts.length" class="px-2 py-2 text-xs text-zinc-500">
        {{ tr('Nenhum context.') }} <button class="text-sky-400 hover:underline" @click="emit('addKubeconfig')">{{ tr('Adicionar kubeconfig') }}</button>
      </p>
    </div>
    <div
      v-if="rancherBanner" class="mx-3 mb-2 rounded-md border p-2 text-xs"
      :class="rancherBanner.tone === 'warn' ? 'border-amber-900/60 bg-amber-950/40 text-amber-200' : 'border-zinc-800 bg-zinc-900 text-zinc-300'"
    >
      {{ rancherBanner.text }}
      <button
        class="ml-1 rounded bg-sky-600 px-2 py-0.5 font-medium text-white hover:bg-sky-500 disabled:opacity-60" :disabled="loggingIn"
        @click="renew(rancherBanner.url)"
      >{{ loggingIn ? 'Aguardando…' : ranchers.browserLogin(rancherBanner.url) === false ? 'Renovar API key' : 'Renovar' }}</button>
    </div>
    <div v-if="error && needLogin" class="mx-3 mb-2 rounded-md border border-amber-900/60 bg-amber-950/40 p-2 text-xs text-amber-200">
      {{ needLogin.text }}
      <div class="mt-1.5 flex items-center gap-2">
        <button
          v-if="ranchers.browserLogin(needLogin.url) === false"
          class="rounded bg-sky-600 px-2 py-0.5 font-medium text-white hover:bg-sky-500" @click="emit('rancherKey', needLogin.url)"
        >{{ tr('Renovar API key') }}</button>
        <button
          v-else class="rounded bg-sky-600 px-2 py-0.5 font-medium text-white hover:bg-sky-500 disabled:opacity-60" :disabled="loggingIn"
          @click="loginAndRetry"
        >{{ loggingIn ? tr('Aguardando login no navegador…') : tr('Entrar no Rancher') }}</button>
        <button v-if="loggingIn" class="text-zinc-400 hover:text-zinc-100" @click="ranchers.cancelLogin(needLogin.url)">{{ tr('Cancelar') }}</button>
      </div>
    </div>
    <div v-else-if="execAsk" class="mx-3 mb-2 rounded-md border border-amber-900/60 bg-amber-950/40 p-2 text-xs text-amber-200">
      {{ tr('O kubeconfig de') }} <b>{{ execAsk.context }}</b> {{ tr('quer executar este comando para obter credenciais:') }}
      <code class="mt-1 block rounded bg-black/40 px-1.5 py-1 font-mono text-[11px] break-all text-zinc-100 select-text">{{ execAsk.command }}</code>
      <p class="mt-1 text-amber-200/80">{{ tr('Permita só se você reconhece o comando (ex.: aws, kubelogin, gke-gcloud-auth-plugin). A permissão vale para este comando exato.') }}</p>
      <div class="mt-1.5 flex gap-2">
        <button class="rounded bg-sky-600 px-2 py-0.5 font-medium text-white hover:bg-sky-500" @click="allowExec">{{ tr('Permitir e conectar') }}</button>
        <button class="text-zinc-400 hover:text-zinc-100" @click="error = ''">{{ tr('Cancelar') }}</button>
      </div>
    </div>
    <div v-else-if="error" class="relative mx-3 mb-2 rounded-md border border-rose-900/60 bg-rose-950/50 p-2 pr-7 text-xs break-words text-rose-300">
      {{ error }}
      <p v-if="networkHint" class="mt-1 text-rose-200/80">{{ tr('O cluster não respondeu: confira a VPN ou a rede e tente de novo.') }}</p>
      <button class="absolute top-1.5 right-1.5 rounded p-0.5 text-rose-300/70 hover:bg-rose-900/50 hover:text-rose-100" :title="tr('Fechar')" @click="error = ''">
        <Icon name="close" :size="11" />
      </button>
    </div>

    <!-- Navigation -->
    <template v-if="active">
      <div v-if="pinnedObjs.length || recentObjs.length" class="border-t border-zinc-800/80 px-1.5 pt-2 pb-1">
        <button class="flex w-full items-center gap-1 px-2 pb-1 text-[10px] font-semibold tracking-wider text-zinc-500 uppercase hover:text-zinc-300" @click="quickOpen = !quickOpen">
          <Icon :name="quickOpen ? 'collapse' : 'chevron'" :size="10" /> {{ tr('Fixados e recentes') }}
        </button>
        <template v-if="quickOpen">
          <div v-for="o in pinnedObjs" :key="`p${o.kind}${o.namespace}${o.name}`" class="group flex items-center gap-2 rounded-md px-2 py-1 text-xs hover:bg-zinc-800/70">
            <Icon name="pin" :size="11" class="shrink-0 text-amber-300" />
            <button class="min-w-0 flex-1 truncate text-left text-zinc-200" :title="`${o.kind} ${o.namespace ? o.namespace + '/' : ''}${o.name}`" @click="openObj(o)">{{ o.name }}</button>
            <span class="shrink-0 text-[10px] text-zinc-600">{{ o.kind }}</span>
            <button class="hidden shrink-0 text-zinc-500 group-hover:block hover:text-zinc-200" :title="tr('Desafixar')" @click="togglePin(active, o)"><Icon name="close" :size="10" /></button>
          </div>
          <button
            v-for="o in recentObjs" :key="`r${o.kind}${o.namespace}${o.name}`"
            class="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-xs hover:bg-zinc-800/70"
            :title="`${o.kind} ${o.namespace ? o.namespace + '/' : ''}${o.name}`" @click="openObj(o)"
          >
            <Icon name="refresh" :size="11" class="shrink-0 text-zinc-600" />
            <span class="min-w-0 flex-1 truncate text-zinc-400">{{ o.name }}</span>
            <span class="shrink-0 text-[10px] text-zinc-600">{{ o.kind }}</span>
          </button>
        </template>
      </div>
      <div class="border-t border-zinc-800/80 px-3 pt-3 pb-2">
        <input
          v-model="filter" :placeholder="tr('Filtrar recursos…  (ex: deploy, ing)')"
          class="h-7 w-full rounded-md border border-zinc-800 bg-zinc-900 px-2 text-xs outline-none placeholder:text-zinc-600 focus:border-sky-600"
          @keydown.esc="filter = ''"
        />
      </div>
      <nav class="min-h-0 flex-1 overflow-auto px-1.5 pb-3" @mousedown.middle.prevent>
        <!-- Flat results while filtering -->
        <template v-if="matches">
          <button
            v-for="k in matches" :key="`${k.group}/${k.resource}`"
            class="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left hover:bg-zinc-800/70"
            :class="isActive(k) && 'bg-sky-500/15 text-sky-200'" @click="go(k, $event)" @auxclick="go(k, $event)"
          >
            <span class="min-w-0 flex-1 truncate">{{ k.kind }}</span>
            <span class="truncate text-[10px] text-zinc-600">{{ k.group || 'core' }}</span>
          </button>
          <p v-if="!matches.length" class="px-2 py-2 text-xs text-zinc-500">{{ tr('Nada encontrado.') }}</p>
        </template>

        <template v-else>
          <button
            class="nav-item" :class="view?.type === 'overview' && 'nav-active'"
            @click="nav({ type: 'overview' }, $event)" @auxclick="nav({ type: 'overview' }, $event)"
          ><Icon name="overview" /> {{ tr('Overview') }}</button>

          <template v-for="s in tree" :key="s.name">
            <!-- direct links -->
            <button
              v-if="LEAF.has(s.name) && leafKind(s.name)"
              class="nav-item" :class="isActive(leafKind(s.name)!) && 'nav-active'"
              @click="go(leafKind(s.name)!, $event)" @auxclick="go(leafKind(s.name)!, $event)"
            ><Icon :name="s.name" /> {{ s.name }}</button>

            <!-- collapsible sections -->
            <div v-else-if="!LEAF.has(s.name)">
              <button class="nav-item" @click="toggle(s.name)">
                <Icon name="chevron" :size="12" class="-ml-1 text-zinc-500 transition-transform" :class="open.has(s.name) && 'rotate-90'" />
                <Icon :name="s.name" /> {{ s.name }}
                <span v-if="s.name === 'Custom Resources'" class="ml-auto text-[10px] text-zinc-600">{{ s.groups.length }}</span>
              </button>
              <div v-if="open.has(s.name)" class="ml-[22px] border-l border-zinc-800 pl-1.5">
                <button
                  v-for="k in s.kinds" :key="`${k.group}/${k.resource}`"
                  class="nav-sub" :class="isActive(k) && 'nav-active'" @click="go(k, $event)" @auxclick="go(k, $event)"
                >{{ s.name === 'Custom Resources' ? 'Definitions' : k.kind }}</button>
                <div v-for="g in s.groups" :key="g.group">
                  <button class="nav-sub text-zinc-400" @click="toggle(`crd:${g.group}`)">
                    <Icon name="chevron" :size="11" class="text-zinc-600 transition-transform" :class="open.has(`crd:${g.group}`) && 'rotate-90'" />
                    <span class="truncate">{{ g.group }}</span>
                  </button>
                  <div v-if="open.has(`crd:${g.group}`)" class="ml-3 border-l border-zinc-800 pl-1.5">
                    <button
                      v-for="k in g.kinds" :key="k.resource"
                      class="nav-sub" :class="isActive(k) && 'nav-active'" @click="go(k, $event)" @auxclick="go(k, $event)"
                    >{{ k.kind }}</button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Helm sits right after Events -->
            <div v-if="s.name === 'Events'">
              <button class="nav-item" @click="toggle('Helm')">
                <Icon name="chevron" :size="12" class="-ml-1 text-zinc-500 transition-transform" :class="open.has('Helm') && 'rotate-90'" />
                <Icon name="Helm" /> {{ tr('Helm') }}
              </button>
              <div v-if="open.has('Helm')" class="ml-[22px] border-l border-zinc-800 pl-1.5">
                <button class="nav-sub" :class="view?.type === 'helm' && 'nav-active'" @click="nav({ type: 'helm' }, $event)" @auxclick="nav({ type: 'helm' }, $event)">{{ tr('Releases') }}</button>
              </div>
              <button class="nav-item" :class="view?.type === 'portforwards' && 'nav-active'" @click="nav({ type: 'portforwards' }, $event)" @auxclick="nav({ type: 'portforwards' }, $event)">
                <Icon name="forward" /> {{ tr('Port Forwards') }}
                <span v-if="forwards.length" class="ml-auto rounded-full bg-sky-500/15 px-1.5 text-[10px] text-sky-300">{{ forwards.length }}</span>
              </button>
            </div>
          </template>

          <button class="mt-2 flex items-center gap-1.5 px-2 text-[11px] text-zinc-600 hover:text-zinc-300" :title="tr('Recarregar lista de tipos (após instalar CRDs)')" @click="store.refreshCatalog()">
            <Icon name="refresh" :size="12" /> {{ tr('redescobrir tipos') }}
          </button>
        </template>
      </nav>
    </template>
    <div v-else class="flex-1" />

    <!-- Theme -->
    <div class="flex items-center gap-2 border-t border-zinc-800/80 px-3 py-2">
      <button class="text-left text-[11px] text-zinc-500 hover:text-zinc-200" :title="tr('Verificar atualizações')" @click="emit('checkUpdate')">
        {{ tr('Leme') }} <span class="font-mono">{{ version === 'dev' ? 'dev' : `v${version}` }}</span>
        <span v-if="updateAvailable" class="ml-1 rounded-full bg-sky-500/15 px-1.5 text-sky-300">{{ tr('nova versão') }}</span>
      </button>
      <button class="ml-auto grid h-6 w-6 place-items-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200" :title="tr('Ajustes')" @click="emit('settings')">
        <Icon name="sliders" :size="13" />
      </button>
      <div class="flex rounded-md border border-zinc-800 p-0.5">
        <button
          v-for="t in THEMES" :key="t.value" :title="t.label"
          class="grid h-6 w-7 place-items-center rounded"
          :class="themePref === t.value ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-200'"
          @click="themePref = t.value"
        ><Icon :name="t.icon" :size="14" /></button>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@reference "../style.css";
.nav-item { @apply flex w-full items-center gap-2.5 rounded-md px-2 py-[5px] text-left text-zinc-300 hover:bg-zinc-800/70 hover:text-zinc-100; }
.nav-sub { @apply flex w-full items-center gap-1.5 rounded-md px-2 py-[3px] text-left text-zinc-400 hover:bg-zinc-800/70 hover:text-zinc-100; }
.nav-active { @apply bg-sky-500/15 text-sky-200 hover:bg-sky-500/20; }
</style>
