import { tr } from '@/lib/i18n'
/**
 * "Problems now": pods that need attention, read from the live pods table the
 * overview already watches (no extra requests). The pod's details panel then
 * shows the full diagnosis (cause and what to do).
 */
import type { Row } from './types'
import { cell } from './format'

export type Severity = 'critical' | 'warning'
export interface Problem {
  uid: string
  name: string
  namespace: string
  reason: string
  hint: string
  severity: Severity
  restarts: number
}

const CRITICAL: [RegExp, string][] = [
  [/CrashLoopBackOff/, tr('o container inicia e cai em seguida')],
  [/ImagePullBackOff|ErrImagePull|InvalidImageName/, tr('não consegue baixar a imagem')],
  [/CreateContainerConfigError|CreateContainerError/, tr('falta ConfigMap, Secret ou permissão para criar o container')],
  [/OOMKilled/, tr('estourou o limite de memória')],
  [/Evicted/, tr('despejado do node (falta de recurso)')],
  [/^Error$|Init:Error|Init:CrashLoopBackOff/, tr('terminou com erro')],
  [/ContainerStatusUnknown|Unknown/, tr('o node parou de informar o estado')],
]

const minutes = (age: string) => {
  // kubectl ages: 45s, 3m, 2m30s, 5h, 3d, 2y...
  let total = 0
  for (const [, n, u] of age.matchAll(/(\d+)([smhdy])/g)) total += Number(n) * ({ s: 1 / 60, m: 1, h: 60, d: 1440, y: 525600 } as Record<string, number>)[u]
  return total
}

export function findProblems(columns: { name: string }[], rows: Row[]): Problem[] {
  const idx = (n: string) => columns.findIndex((c) => c.name.toLowerCase() === n)
  const [iReady, iStatus, iRestarts, iAge] = ['ready', 'status', 'restarts', 'age'].map(idx)
  const out: Problem[] = []
  for (const r of rows) {
    const status = cell(r.cells[iStatus])
    const restarts = Number.parseInt(cell(r.cells[iRestarts]), 10) || 0
    const base = { uid: r.uid, name: r.name, namespace: r.namespace ?? '', restarts }
    const crit = CRITICAL.find(([re]) => re.test(status))
    if (crit) {
      out.push({ ...base, reason: status, hint: crit[1], severity: 'critical' })
      continue
    }
    if (status === 'Completed' || status === 'Succeeded') continue
    if (/^Pending$|ContainerCreating|PodInitializing|^Init:/.test(status) && minutes(cell(r.cells[iAge])) >= 3) {
      out.push({ ...base, reason: status, hint: tr('parado há mais de 3 minutos sem iniciar'), severity: 'warning' })
      continue
    }
    const ready = /^(\d+)\/(\d+)$/.exec(cell(r.cells[iReady]))
    if (status === 'Running' && ready && Number(ready[1]) < Number(ready[2])) {
      out.push({ ...base, reason: tr('Não pronto'), hint: tr('{a} de {b} containers prontos (readiness falhando)', { a: ready[1], b: ready[2] }), severity: 'warning' })
      continue
    }
    if (restarts >= 5) out.push({ ...base, reason: tr('Reinícios'), hint: `reiniciou ${restarts} vezes`, severity: 'warning' })
  }
  return out.sort((a, b) => (a.severity === b.severity ? b.restarts - a.restarts : a.severity === 'critical' ? -1 : 1))
}
