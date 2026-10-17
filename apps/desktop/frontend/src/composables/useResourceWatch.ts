import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { WatchResources, Unsubscribe } from '../../wailsjs/go/api/StreamAPI'
import type { resources } from '../../wailsjs/go/models'
import type { Column, ResourceEvent, Row } from '@/lib/types'
import { errMsg, newId, on, rafThrottle } from '@/lib/bridge'
import { usePauseWhenHidden } from '@/lib/visibility'

/**
 * Live, server-rendered table rows for any kind (incl. CRDs).
 *
 * Performance notes:
 *  - Rows live in a plain Map (not reactive). Only `rows` (a shallowRef
 *    array) is reactive, rebuilt at most once per animation frame. Vue never
 *    deep-proxies 10k row objects, which is what makes large Kubernetes UIs crawl.
 *  - The subscription exists only while the owning component is mounted.
 *  - Relists (after 410 Gone) build into a side map and swap atomically on
 *    "synced", so the table never flashes empty.
 */
export function useResourceWatch(
  cluster: Ref<string>,
  kind: Ref<resources.Ref | null>,
  query: Ref<resources.Query>,
) {
  const columns = shallowRef<Column[]>([])
  const rows = shallowRef<Row[]>([])
  const loading = ref(false)
  const error = ref('')

  let live = new Map<string, Row>()
  let pending: Map<string, Row> | null = null
  let subId = ''
  let off: (() => void) | null = null

  const publish = rafThrottle(() => {
    rows.value = Array.from(live.values())
  })

  function put(target: Map<string, Row>, list: Row[] | undefined) {
    for (const r of list ?? []) target.set(r.uid, Object.freeze(r))
  }

  function onEvent(ev: ResourceEvent) {
    switch (ev.type) {
      case 'reset':
        columns.value = ev.columns ?? []
        error.value = ''
        if (live.size === 0) put(live, ev.rows)
        else put((pending = new Map()), ev.rows)
        break
      case 'append':
        put(pending ?? live, ev.rows)
        break
      case 'synced':
        if (pending) {
          live = pending
          pending = null
        }
        loading.value = false
        break
      case 'delta':
        for (const d of ev.deltas ?? []) {
          if (d.type === 'DELETED') live.delete(d.row.uid)
          else live.set(d.row.uid, Object.freeze(d.row))
        }
        break
      case 'error':
        error.value = ev.error ?? 'watch error'
        break
    }
    publish()
  }

  function stop() {
    if (!subId) return
    void Unsubscribe(subId)
    off?.()
    subId = ''
    off = null
  }

  // keep=true (resume after the window was hidden): leave the current rows on
  // screen; the fresh snapshot swaps in atomically on "synced".
  async function start(keep = false) {
    stop()
    pending = null
    if (!keep) {
      live = new Map()
      rows.value = []
      columns.value = []
    }
    if (!cluster.value || !kind.value) return
    loading.value = true
    const id = (subId = newId())
    off = on<ResourceEvent>(`watch:${id}`, onEvent) // listen before starting
    try {
      await WatchResources(id, cluster.value, kind.value, query.value)
    } catch (e) {
      if (id === subId) {
        error.value = errMsg(e)
        loading.value = false
      }
    }
  }

  watch([cluster, kind, query], () => start(), { immediate: true, deep: true })
  usePauseWhenHidden(stop, () => void start(true))
  onScopeDispose(stop)

  return { columns, rows, loading, error, restart: () => start() }
}
