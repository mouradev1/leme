import type { resources } from '../../wailsjs/go/models'
import type { Row, RowAction } from './types'

export const actionLabel: Record<RowAction, string> = {
  details: 'Detalhes',
  edit: 'Editar YAML',
  copy: 'Copiar nome',
  logs: 'Logs',
  shell: 'Shell',
  scale: 'Escalar…',
  restart: 'Reiniciar (rollout)',
  cordon: 'Cordon',
  uncordon: 'Uncordon',
  drain: 'Drain…',
  delete: 'Excluir',
}

export const dangerous = new Set<RowAction>(['delete', 'drain'])

export const isKind = (k: resources.Ref, group: string, resource: string) =>
  k.group === group && k.resource === resource

/** Kind-specific quick actions (the generic ones are always appended). */
export function actionsFor(k: resources.Kind, row: Row): RowAction[] {
  const specific: RowAction[] = []
  if (isKind(k, '', 'pods')) specific.push('logs', 'shell')
  if (isKind(k, 'apps', 'deployments') || isKind(k, 'apps', 'statefulsets')) specific.push('scale', 'restart')
  if (isKind(k, 'apps', 'daemonsets')) specific.push('restart')
  if (isKind(k, '', 'nodes')) {
    const cordoned = row.cells.some((c) => String(c).includes('SchedulingDisabled'))
    specific.push(cordoned ? 'uncordon' : 'cordon', 'drain')
  }
  const generic: RowAction[] = ['edit', 'copy']
  if (k.verbs.includes('delete') && !isKind(k, '', 'nodes')) generic.push('delete')
  return [...specific, ...generic]
}
