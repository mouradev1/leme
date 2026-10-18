import type { resources, terminal } from '../../wailsjs/go/models'


// Payloads pushed via Wails events. Request/response types for bound methods
// are generated in wailsjs/go/models.ts; these mirror structs that are only
// ever emitted (internal/stream, internal/metrics), so Wails doesn't see them.

export interface Column {
  name: string
  type: string
  format: string
  description: string
  priority: number // > 0 = "wide" column
}

export interface Row {
  uid: string
  name: string
  namespace?: string
  created: string | null
  rv: string
  cells: unknown[]
  labels?: Record<string, string>
  owner?: { kind: string; name: string }
}

export interface Delta {
  type: 'ADDED' | 'MODIFIED' | 'DELETED'
  row: Row
}

export interface ResourceEvent {
  type: 'reset' | 'append' | 'synced' | 'delta' | 'error'
  columns?: Column[]
  rows?: Row[]
  deltas?: Delta[]
  error?: string
}

export interface LogLine {
  p: string // pod
  c: string // container
  t: string // text
}

export interface LogEvent {
  type: 'lines' | 'error' | 'end'
  lines?: LogLine[]
  error?: string
}

export interface MetricsSnapshot {
  at: string
  items: { namespace?: string; name: string; cpu: number; memory: number }[]
  error?: string
}

export type RowAction =
  | 'details' | 'edit' | 'copy' | 'delete' | 'logs' | 'shell' | 'nodeshell' | 'portforward'
  | 'scale' | 'restart' | 'cordon' | 'uncordon' | 'drain'

/** Jump to another resource list, e.g. clicking "Controlled By" or "Node". */
export interface NavigateTarget {
  kind: string // Kind name, e.g. "ReplicaSet"
  search?: string
  namespace?: string
}

export type View =
  | { type: 'overview' }
  | { type: 'helm' }
  | { type: 'portforwards' }
  | { type: 'resource'; kind: resources.Kind }

// cluster: the kubeconfig context the tab was opened in. A tab ALWAYS talks to
// that cluster, never to whichever cluster is active now (a shell must not
// silently end up on another cluster when the user switches).
export type DockTab =
  | { id: string; cluster: string; type: 'logs'; title: string; namespace: string; targets: { pod: string; container: string }[]; previous?: boolean; note?: string }
  | { id: string; cluster: string; type: 'shell'; title: string; target?: terminal.ExecTarget; node?: string }

type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never
/** A tab as requested by the UI; the active cluster is stamped on open. */
export type NewDockTab = DistributiveOmit<DockTab, 'cluster'>
