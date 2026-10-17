import { onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { PodDetails } from '../../wailsjs/go/api/ResourceAPI'
import type { pods } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'

/**
 * Structured pod view + restart diagnosis. Refetched when the pod's
 * resourceVersion changes (the table watch already tells us), debounced so a
 * burst of status updates causes one request.
 */
export function usePodDetails(cluster: Ref<string>, namespace: Ref<string>, name: Ref<string>, rv: Ref<string>) {
  const details = shallowRef<pods.Details | null>(null)
  const error = ref('')
  let timer = 0

  async function load() {
    try {
      details.value = await PodDetails(cluster.value, namespace.value, name.value)
      error.value = ''
    } catch (e) {
      error.value = errMsg(e)
    }
  }

  watch([cluster, namespace, name], () => { details.value = null; void load() }, { immediate: true })
  watch(rv, () => {
    clearTimeout(timer)
    timer = window.setTimeout(load, 800)
  })
  onScopeDispose(() => clearTimeout(timer))
  return { details, error, reload: load }
}
