/**
 * Recent and pinned objects per cluster (this computer only). Recents are
 * the last objects whose details were opened; pins are kept until removed.
 */
import { reactive, watch } from 'vue'

export interface ObjRef { kind: string; name: string; namespace: string }
interface Store { recent: Record<string, ObjRef[]>; pinned: Record<string, ObjRef[]> }

const KEY = 'leme:objects'
const MAX_RECENT = 12

function load(): Store {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? '{}')
    return { recent: v.recent ?? {}, pinned: v.pinned ?? {} }
  } catch {
    return { recent: {}, pinned: {} }
  }
}

export const objects = reactive<Store>(load())
watch(objects, () => {
  try { localStorage.setItem(KEY, JSON.stringify(objects)) } catch { /* storage unavailable */ }
}, { deep: true })

const same = (a: ObjRef, b: ObjRef) => a.kind === b.kind && a.name === b.name && a.namespace === b.namespace

export function addRecent(cluster: string, o: ObjRef) {
  const list = (objects.recent[cluster] ?? []).filter((x) => !same(x, o))
  objects.recent[cluster] = [o, ...list].slice(0, MAX_RECENT)
}

export const isPinned = (cluster: string, o: ObjRef) => (objects.pinned[cluster] ?? []).some((x) => same(x, o))

export function togglePin(cluster: string, o: ObjRef) {
  const list = objects.pinned[cluster] ?? []
  objects.pinned[cluster] = isPinned(cluster, o) ? list.filter((x) => !same(x, o)) : [...list, o]
}
