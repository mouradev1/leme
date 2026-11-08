import { defineStore } from 'pinia'
import { computed, ref, shallowRef, watch } from 'vue'
import { ListContexts, Connect, Disconnect, Namespaces } from '../../wailsjs/go/api/ClusterAPI'
import { Catalog, RefreshCatalog } from '../../wailsjs/go/api/ResourceAPI'
import { NamespaceProjects } from '../../wailsjs/go/api/RancherAPI'
import type { api, cluster, resources } from '../../wailsjs/go/models'
import { errMsg, on } from '@/lib/bridge'

const CATEGORY_ORDER = [
  'Nodes', 'Workloads', 'Config', 'Network', 'Storage', 'Namespaces', 'Events',
  'Access Control', 'Custom Resources', 'Other',
]

/**
 * The "active cluster" lives only here. Backend calls always pass the context
 * name, so switching clusters is just changing `active`; the previous
 * cluster's session stays warm (instant switch back) until disconnected.
 */
export const useClusters = defineStore('clusters', () => {
  const contexts = ref<cluster.ContextInfo[]>([])
  const active = ref('')
  const server = ref<api.ServerInfo | null>(null)
  const catalog = shallowRef<resources.Kind[]>([])
  const namespaces = ref<string[]>([])
  const selectedNamespaces = ref<string[]>([]) // [] = all namespaces
  const connecting = ref('')
  const error = ref('')
  const projects = ref<Record<string, string>>({}) // namespace -> Rancher Project (Rancher clusters only)
  const failed = ref('') // context whose connection failed (to retry after a Rancher login)

  on<cluster.ContextInfo[]>('clusters:changed', (list) => (contexts.value = list))

  async function load() {
    contexts.value = await ListContexts()
  }

  async function activate(name: string) {
    connecting.value = name
    error.value = ''
    failed.value = ''
    try {
      server.value = await Connect(name) // runs exec plugins / SSO on first use
      const [kinds, nss] = await Promise.all([Catalog(name), Namespaces(name)])
      active.value = name
      catalog.value = kinds
      namespaces.value = nss
      selectedNamespaces.value = readNs(name).filter((n) => nss.includes(n))
      projects.value = {}
      NamespaceProjects(name).then((p) => { if (active.value === name) projects.value = p ?? {} }).catch(() => {})
      await load() // refresh "connected" flags
    } catch (e) {
      error.value = errMsg(e)
      failed.value = name
    } finally {
      connecting.value = ''
    }
  }

  /**
   * Close a cluster's connection (watches, log/exec streams, port-forwards).
   * Its saved tabs stay and come back on the next connect.
   */
  async function disconnect(name: string) {
    await Disconnect(name)
    if (active.value === name) {
      active.value = ''
      server.value = null
      catalog.value = []
      namespaces.value = []
      selectedNamespaces.value = []
    }
    await load()
  }

  /** Resolve a Kind name (e.g. "Node", "ReplicaSet"); built-ins win over CRDs with the same name. */
  function findKind(kindName: string): resources.Kind | undefined {
    const matches = catalog.value.filter((k) => k.kind === kindName)
    return matches.find((k) => k.category !== 'Custom Resources') ?? matches[0]
  }

  async function refreshCatalog() {
    if (active.value) catalog.value = await RefreshCatalog(active.value)
  }

  const sections = computed(() => {
    const groups = new Map<string, resources.Kind[]>()
    for (const k of catalog.value) {
      if (!groups.has(k.category)) groups.set(k.category, [])
      groups.get(k.category)!.push(k)
    }
    return CATEGORY_ORDER.filter((c) => groups.has(c)).map((c) => ({ name: c, kinds: groups.get(c)! }))
  })

  // Namespace selection is remembered per cluster (per-viewer convenience).
  watch(selectedNamespaces, (v) => {
    if (!active.value) return
    try { localStorage.setItem(`leme:ns:${active.value}`, JSON.stringify(v)) } catch { /* storage unavailable */ }
  })

  return {
    contexts, active, server, catalog, namespaces, selectedNamespaces, projects, connecting, error, failed, sections,
    load, activate, disconnect, refreshCatalog, findKind,
  }
})

function readNs(cluster: string): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(`leme:ns:${cluster}`) ?? '[]')
    return Array.isArray(v) ? v : []
  } catch {
    return []
  }
}
