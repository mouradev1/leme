// Colors for status-like cells (pods, nodes, PVCs, jobs, deployments...).

const RED = /(error|fail|crash|backoff|errimage|oomkill|evicted|notready|lost|unknown|invalid|denied)/i
const AMBER = /(pending|creating|terminating|init:|waiting|schedulingdisabled|progressing|released|suspended)/i
const GREEN = /^(running|ready|active|bound|available|true|healthy|deployed|succeeded|complete|completed)$/i

export type Tone = 'ok' | 'warn' | 'bad' | 'muted' | 'none'

export function statusTone(value: string): Tone {
  if (!value) return 'none'
  if (RED.test(value)) return 'bad'
  if (AMBER.test(value)) return 'warn'
  if (/^(succeeded|completed|complete)$/i.test(value)) return 'muted'
  if (GREEN.test(value)) return 'ok'
  return 'none'
}

export const toneClass: Record<Tone, string> = {
  ok: 'text-emerald-400',
  warn: 'text-amber-400',
  bad: 'text-rose-400',
  muted: 'text-teal-300/70',
  none: 'text-zinc-300',
}

export const STATUS_COLUMNS = new Set(['Status', 'STATUS', 'Phase', 'State'])
