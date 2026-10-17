import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { StreamLogs, Unsubscribe } from '../../wailsjs/go/api/StreamAPI'
import type { stream } from '../../wailsjs/go/models'
import type { LogEvent, LogLine } from '@/lib/types'
import { errMsg, newId, on, rafThrottle } from '@/lib/bridge'

/** Bounded ring buffer of log lines; memory is capped regardless of uptime. */
export function useLogStream(cluster: Ref<string>, opts: Ref<stream.LogOptions | null>, max = 20_000) {
  const lines = shallowRef<LogLine[]>([])
  const status = ref<'idle' | 'streaming' | 'ended' | 'error'>('idle')
  const error = ref('')

  let buf: LogLine[] = []
  let subId = ''
  let off: (() => void) | null = null
  // New array per frame (pointer copy, ~µs) so computeds downstream re-run;
  // mutating in place + triggerRef would be swallowed by computed identity checks.
  const publish = rafThrottle(() => (lines.value = buf.slice()))

  function onEvent(ev: LogEvent) {
    if (ev.type === 'lines' && ev.lines) {
      buf.push(...ev.lines)
      if (buf.length > max) buf.splice(0, buf.length - max)
      publish()
    } else if (ev.type === 'error') {
      error.value = ev.error ?? ''
      status.value = 'error'
    } else if (ev.type === 'end') {
      status.value = 'ended'
    }
  }

  function stop() {
    if (!subId) return
    void Unsubscribe(subId)
    off?.()
    subId = ''
  }

  async function start() {
    stop()
    buf = []
    lines.value = []
    error.value = ''
    if (!cluster.value || !opts.value) return
    status.value = 'streaming'
    const id = (subId = newId())
    off = on<LogEvent>(`logs:${id}`, onEvent)
    try {
      await StreamLogs(id, cluster.value, opts.value)
    } catch (e) {
      error.value = errMsg(e)
      status.value = 'error'
    }
  }

  watch([cluster, opts], start, { immediate: true, deep: true })
  onScopeDispose(stop)

  return { lines, status, error, restart: start, stop }
}
