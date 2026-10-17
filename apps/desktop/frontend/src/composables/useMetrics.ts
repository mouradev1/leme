import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { WatchMetrics, Unsubscribe } from '../../wailsjs/go/api/StreamAPI'
import { api } from '../../wailsjs/go/models'
import type { MetricsSnapshot } from '@/lib/types'
import { newId, on } from '@/lib/bridge'
import { usePauseWhenHidden } from '@/lib/visibility'

export interface Usage {
  cpu: number // millicores
  memory: number // bytes
}

/**
 * Polls metrics-server only while a Pods/Nodes view is mounted. Keyed by
 * "namespace/name" ("/name" for nodes). `available` turns false when the
 * cluster has no metrics API, so the UI can hide the columns gracefully.
 */
export function useMetrics(cluster: Ref<string>, scope: Ref<'pods' | 'nodes' | null>, namespace: Ref<string>) {
  const usage = shallowRef(new Map<string, Usage>())
  const available = ref(true)

  let subId = ''
  let off: (() => void) | null = null

  function stop() {
    if (!subId) return
    void Unsubscribe(subId)
    off?.()
    subId = ''
  }

  async function start(keep = false) {
    stop()
    if (!keep) usage.value = new Map()
    if (!cluster.value || !scope.value) return
    const id = (subId = newId())
    off = on<MetricsSnapshot>(`metrics:${id}`, (snap) => {
      if (snap.error) {
        available.value = false
        return
      }
      available.value = true
      const m = new Map<string, Usage>()
      for (const it of snap.items ?? []) m.set(`${it.namespace ?? ''}/${it.name}`, { cpu: it.cpu, memory: it.memory })
      usage.value = m
    })
    await WatchMetrics(id, cluster.value, api.MetricsQuery.createFrom({
      scope: scope.value, namespace: namespace.value, intervalSeconds: 15,
      promNamespace: '', promService: '', promPort: '',
    })).catch(() => (available.value = false))
  }

  watch([cluster, scope, namespace], () => start(), { immediate: true })
  usePauseWhenHidden(stop, () => void start(true))
  onScopeDispose(stop)
  return { usage, available }
}
