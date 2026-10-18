import { shallowRef } from 'vue'
import { List } from '../../wailsjs/go/api/PortForwardAPI'
import type { portforward } from '../../wailsjs/go/models'
import { on } from './bridge'

/** A request to open the port-forward dialog (from any view). */
export interface ForwardTarget {
  cluster: string
  kind: 'pod' | 'service'
  namespace: string
  name: string
  port?: number // preselected remote port
}

export const forwardRequest = shallowRef<ForwardTarget | null>(null)
export const openForward = (t: ForwardTarget) => (forwardRequest.value = t)

/** Active forwards across all clusters, kept in sync by the backend. */
export const forwards = shallowRef<portforward.Forward[]>([])
let started = false
export function trackForwards() {
  if (started) return
  started = true
  void List().then((l) => (forwards.value = l ?? []))
  on<portforward.Forward[]>('portforwards:changed', (l) => (forwards.value = l ?? []))
}

export const forwardURL = (f: { localPort: number }) => `http://localhost:${f.localPort}`
