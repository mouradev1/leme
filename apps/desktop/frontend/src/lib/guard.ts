/**
 * Per-cluster safety marks. Production: risky actions ask for the cluster
 * name. Read-only: actions that change the cluster are hidden here and also
 * refused by the backend.
 */
import { ref } from 'vue'
import { ClusterPrefs, SetClusterPrefs } from '../../wailsjs/go/api/ClusterAPI'
import type { settings } from '../../wailsjs/go/models'
import type { RowAction } from './types'

export const clusterPrefs = ref<Record<string, settings.ClusterPrefs>>({})

export async function loadPrefs() {
  try { clusterPrefs.value = await ClusterPrefs() } catch { /* keep the last known */ }
}

export async function setPrefs(cluster: string, p: Partial<settings.ClusterPrefs>) {
  const next = { production: false, readOnly: false, ...clusterPrefs.value[cluster], ...p }
  await SetClusterPrefs(cluster, next as settings.ClusterPrefs)
  await loadPrefs()
}

export const isProd = (c: string) => !!clusterPrefs.value[c]?.production
export const isReadOnly = (c: string) => !!clusterPrefs.value[c]?.readOnly

/** Actions that never change the cluster. */
const READ_ONLY_OK = new Set<RowAction>(['details', 'copy', 'logs', 'portforward'])
export const allowedIn = (cluster: string, actions: RowAction[]) =>
  isReadOnly(cluster) ? actions.filter((a) => READ_ONLY_OK.has(a)) : actions
