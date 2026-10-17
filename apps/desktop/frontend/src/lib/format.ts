/** kubectl-style age: 45s, 12m, 3h4m, 2d5h, 120d. */
export function age(createdMs: number, nowMs: number): string {
  if (!createdMs) return ''
  let s = Math.max(0, Math.floor((nowMs - createdMs) / 1000))
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return m % 60 ? `${h}h${m % 60}m` : `${h}h`
  const d = Math.floor(h / 24)
  return d < 10 && h % 24 ? `${d}d${h % 24}h` : `${d}d`
}

export function cell(v: unknown): string {
  if (v == null) return ''
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

export function bytes(n: number): string {
  const u = ['B', 'Ki', 'Mi', 'Gi', 'Ti']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return `${n.toFixed(i ? 1 : 0)}${u[i]}`
}

/** CPU in millicores -> "250m" / "1.25". */
export function cpu(milli: number): string {
  return milli < 1000 ? `${milli}m` : (milli / 1000).toFixed(2)
}
