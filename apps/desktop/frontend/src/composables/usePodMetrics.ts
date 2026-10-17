import { computed, onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { Unsubscribe, WatchPodMetrics } from '../../wailsjs/go/api/StreamAPI'
import type { MetricsSnapshot } from '@/lib/types'
import { newId, on } from '@/lib/bridge'
import { usePauseWhenHidden } from '@/lib/visibility'

export interface Sample {
  t: number
  c: Record<string, { cpu: number; memory: number }> // per container
}

const INTERVAL_S = 15 // metrics-server's default resolution
const MAX_SAMPLES = 240 // 1h at 15s
const MAX_PODS = 30

// metrics-server has no history, so we keep what we sampled while the app
// runs. Module-level: reopening a pod's panel shows the history again.
// Bounded: 30 pods × 240 samples, oldest pod evicted first.
const cache = new Map<string, Sample[]>()

function remember(key: string, samples: Sample[]) {
  cache.delete(key) // re-insert = most recently used
  cache.set(key, samples)
  while (cache.size > MAX_PODS) cache.delete(cache.keys().next().value!)
}

export function usePodMetrics(cluster: Ref<string>, namespace: Ref<string>, pod: Ref<string>) {
  const key = computed(() => `${cluster.value}/${namespace.value}/${pod.value}`)
  const samples = shallowRef<Sample[]>([])
  const error = ref('')

  let subId = ''
  let off: (() => void) | null = null
  function stop() {
    if (!subId) return
    void Unsubscribe(subId)
    off?.()
    subId = ''
  }

  async function start() {
    stop()
    const k = key.value
    samples.value = cache.get(k) ?? []
    error.value = ''
    const id = (subId = newId())
    off = on<MetricsSnapshot>(`metrics:${id}`, (snap) => {
      if (snap.error) {
        error.value = /not ?found/i.test(snap.error)
          ? 'O metrics-server ainda não tem dados deste pod (pods recém-criados levam ~1 min).'
          : /could not find the requested resource|metrics\.k8s\.io/i.test(snap.error)
            ? 'metrics-server não está instalado neste cluster.'
            : snap.error
        return
      }
      error.value = ''
      const c: Sample['c'] = {}
      for (const it of snap.items ?? []) c[it.name] = { cpu: it.cpu, memory: it.memory }
      const next = [...(cache.get(k) ?? []), { t: Date.parse(snap.at), c }].slice(-MAX_SAMPLES)
      remember(k, next)
      samples.value = next
    })
    await WatchPodMetrics(id, cluster.value, namespace.value, pod.value, INTERVAL_S).catch((e) => (error.value = String(e)))
  }

  watch(key, start, { immediate: true })
  usePauseWhenHidden(stop, () => void start()) // history is cached; the gap shows as a break in the line
  onScopeDispose(stop)
  return { samples, error, intervalMs: INTERVAL_S * 1000 }
}
