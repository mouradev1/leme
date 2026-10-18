<script setup lang="ts">
/** Cluster switcher + navigation tree (sections with icons, CRDs by group). */
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import type { resources } from '../../wailsjs/go/models'
import { useClusters } from '@/stores/clusters'
import { AppVersion } from '../../wailsjs/go/api/ClusterAPI'
import type { View } from '@/lib/types'
import { hue, initials, useDismiss } from '@/lib/dom'
import { themePref, type ThemePref } from '@/lib/theme'
import Icon from './Icon.vue'
import { forwards } from '@/lib/forward'

const view = defineModel<View | null>('view', { required: true })
defineProps<{ updateAvailable?: boolean }>()
const emit = defineEmits<{ addKubeconfig: []; openCluster: [name: string]; checkUpdate: [] }>()

const store = useClusters()
const { contexts, active, sections, connecting, error, catalog } = storeToRefs(store)

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
const go = (k: resources.Kind) => (view.value = { type: 'resource', kind: k })
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
        <Icon name="chevron" :size="12" class="transition-transform" :class="clusterOpen && 'rotate-90'" /> Clusters
      </button>
      <button
        v-if="active" class="ml-auto grid h-6 w-6 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100"
        :title="clusterView === 'compact' ? 'Mostrar todos os clusters' : 'Mostrar só o cluster atual'"
        @click="clusterView = clusterView === 'compact' ? 'list' : 'compact'"
      >
        <Icon :name="clusterView === 'compact' ? 'list' : 'single'" :size="14" />
      </button>
      <button class="grid h-6 w-6 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :class="!active && 'ml-auto'" title="Adicionar cluster (colar configuração ou escolher arquivo)" @click="emit('addKubeconfig')">
        <Icon name="plus" :size="14" />
      </button>
    </div>

    <!-- Compact: only the active cluster, switch through a searchable popover -->
    <div v-if="compact && clusterOpen" ref="switcherRoot" class="relative px-1.5 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg border border-zinc-800 bg-zinc-900/70 px-2 py-2 text-left hover:border-zinc-700"
        title="Trocar de cluster" @click="switcherOpen = !switcherOpen"
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
            v-model="switcherQ" placeholder="Buscar cluster…" autofocus
            class="w-full rounded bg-zinc-950 px-2 py-1.5 text-xs outline-none placeholder:text-zinc-600"
            @keydown.enter="switcherList[0] && pick(switcherList[0].name)" @keydown.esc="switcherOpen = false"
          />
        </div>
        <div class="max-h-72 overflow-auto py-1">
          <button
            v-for="c in switcherList" :key="c.name"
            class="flex w-full items-center gap-2.5 px-2.5 py-1.5 text-left hover:bg-zinc-800"
            :class="c.name === active && 'text-sky-300'" @click="pick(c.name)"
          >
            <span class="relative grid h-5 w-5 shrink-0 place-items-center rounded text-[9px] font-bold text-white" :style="{ background: `hsl(${hue(c.name)} 55% 42%)` }">
              {{ initials(c.name) }}
              <span v-if="c.connected" class="absolute -right-0.5 -bottom-0.5 h-1.5 w-1.5 rounded-full bg-emerald-400" />
            </span>
            <span class="min-w-0 flex-1 truncate">{{ c.name }}</span>
            <span v-if="connecting === c.name" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
          </button>
          <p v-if="!switcherList.length" class="px-3 py-2 text-xs text-zinc-500">Nenhum cluster.</p>
        </div>
      </div>
    </div>

    <!-- List: every cluster -->
    <div v-show="clusterOpen && !compact" class="max-h-52 overflow-auto px-1.5 pb-2">
      <button
        v-for="c in contexts" :key="c.name"
        class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left hover:bg-zinc-800/70"
        :class="c.name === active && 'bg-zinc-800 text-zinc-50'"
        :title="`${c.server}\nauth: ${c.auth}\n${c.source}`" @click="emit('openCluster', c.name)"
      >
        <span
          class="relative grid h-6 w-6 shrink-0 place-items-center rounded-md text-[10px] font-bold text-white"
          :style="{ background: `hsl(${hue(c.name)} 55% 42%)` }"
        >
          {{ initials(c.name) }}
          <span v-if="c.connected" class="absolute -right-0.5 -bottom-0.5 h-2 w-2 rounded-full border-2 border-zinc-925 bg-emerald-400" />
        </span>
        <span class="min-w-0 flex-1 truncate">{{ c.name }}</span>
        <span v-if="connecting === c.name" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" />
      </button>
      <p v-if="!contexts.length" class="px-2 py-2 text-xs text-zinc-500">
        Nenhum context. <button class="text-sky-400 hover:underline" @click="emit('addKubeconfig')">Adicionar kubeconfig</button>
      </p>
    </div>
    <div v-if="error" class="mx-3 mb-2 rounded-md border border-rose-900/60 bg-rose-950/50 p-2 text-xs break-words text-rose-300">{{ error }}</div>

    <!-- Navigation -->
    <template v-if="active">
      <div class="border-t border-zinc-800/80 px-3 pt-3 pb-2">
        <input
          v-model="filter" placeholder="Filtrar recursos…  (ex: deploy, ing)"
          class="h-7 w-full rounded-md border border-zinc-800 bg-zinc-900 px-2 text-xs outline-none placeholder:text-zinc-600 focus:border-sky-600"
          @keydown.esc="filter = ''"
        />
      </div>
      <nav class="min-h-0 flex-1 overflow-auto px-1.5 pb-3">
        <!-- Flat results while filtering -->
        <template v-if="matches">
          <button
            v-for="k in matches" :key="`${k.group}/${k.resource}`"
            class="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left hover:bg-zinc-800/70"
            :class="isActive(k) && 'bg-sky-500/15 text-sky-200'" @click="go(k)"
          >
            <span class="min-w-0 flex-1 truncate">{{ k.kind }}</span>
            <span class="truncate text-[10px] text-zinc-600">{{ k.group || 'core' }}</span>
          </button>
          <p v-if="!matches.length" class="px-2 py-2 text-xs text-zinc-500">Nada encontrado.</p>
        </template>

        <template v-else>
          <button
            class="nav-item" :class="view?.type === 'overview' && 'nav-active'"
            @click="view = { type: 'overview' }"
          ><Icon name="overview" /> Overview</button>

          <template v-for="s in tree" :key="s.name">
            <!-- direct links -->
            <button
              v-if="LEAF.has(s.name) && leafKind(s.name)"
              class="nav-item" :class="isActive(leafKind(s.name)!) && 'nav-active'"
              @click="go(leafKind(s.name)!)"
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
                  class="nav-sub" :class="isActive(k) && 'nav-active'" @click="go(k)"
                >{{ s.name === 'Custom Resources' ? 'Definitions' : k.kind }}</button>
                <div v-for="g in s.groups" :key="g.group">
                  <button class="nav-sub text-zinc-400" @click="toggle(`crd:${g.group}`)">
                    <Icon name="chevron" :size="11" class="text-zinc-600 transition-transform" :class="open.has(`crd:${g.group}`) && 'rotate-90'" />
                    <span class="truncate">{{ g.group }}</span>
                  </button>
                  <div v-if="open.has(`crd:${g.group}`)" class="ml-3 border-l border-zinc-800 pl-1.5">
                    <button
                      v-for="k in g.kinds" :key="k.resource"
                      class="nav-sub" :class="isActive(k) && 'nav-active'" @click="go(k)"
                    >{{ k.kind }}</button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Helm sits right after Events -->
            <div v-if="s.name === 'Events'">
              <button class="nav-item" @click="toggle('Helm')">
                <Icon name="chevron" :size="12" class="-ml-1 text-zinc-500 transition-transform" :class="open.has('Helm') && 'rotate-90'" />
                <Icon name="Helm" /> Helm
              </button>
              <div v-if="open.has('Helm')" class="ml-[22px] border-l border-zinc-800 pl-1.5">
                <button class="nav-sub" :class="view?.type === 'helm' && 'nav-active'" @click="view = { type: 'helm' }">Releases</button>
              </div>
              <button class="nav-item" :class="view?.type === 'portforwards' && 'nav-active'" @click="view = { type: 'portforwards' }">
                <Icon name="forward" /> Port Forwards
                <span v-if="forwards.length" class="ml-auto rounded-full bg-sky-500/15 px-1.5 text-[10px] text-sky-300">{{ forwards.length }}</span>
              </button>
            </div>
          </template>

          <button class="mt-2 flex items-center gap-1.5 px-2 text-[11px] text-zinc-600 hover:text-zinc-300" title="Recarregar lista de tipos (após instalar CRDs)" @click="store.refreshCatalog()">
            <Icon name="refresh" :size="12" /> redescobrir tipos
          </button>
        </template>
      </nav>
    </template>
    <div v-else class="flex-1" />

    <!-- Theme -->
    <div class="flex items-center gap-2 border-t border-zinc-800/80 px-3 py-2">
      <button class="text-left text-[11px] text-zinc-500 hover:text-zinc-200" title="Verificar atualizações" @click="emit('checkUpdate')">
        Leme <span class="font-mono">{{ version === 'dev' ? 'dev' : `v${version}` }}</span>
        <span v-if="updateAvailable" class="ml-1 rounded-full bg-sky-500/15 px-1.5 text-sky-300">nova versão</span>
      </button>
      <div class="ml-auto flex rounded-md border border-zinc-800 p-0.5">
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
