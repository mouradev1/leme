import type { Row } from './types'
import { cell } from './format'

export interface Matcher {
  test: (row: Row) => boolean
  error?: string
}

// Rows are frozen and replaced on every change, so a WeakMap keyed by the
// row object is a free, self-invalidating cache (GC'd with the row).
const cache = new WeakMap<Row, [raw: string, lower: string]>()

/** Searchable text of a row: name, namespace, every cell, labels as k=v. */
function haystack(r: Row): [string, string] {
  let h = cache.get(r)
  if (!h) {
    const parts = [r.name, r.namespace ?? '', ...r.cells.map(cell)]
    if (r.owner) parts.push(r.owner.name)
    for (const [k, v] of Object.entries(r.labels ?? {})) parts.push(`${k}=${v}`)
    const raw = parts.join('\u0000')
    h = [raw, raw.toLowerCase()]
    cache.set(r, h)
  }
  return h
}

/**
 * Space-separated terms are AND-ed: "nginx running" finds rows matching both.
 * A term starting with "!" excludes: "!kube-system". Regex mode uses the
 * whole query as one pattern.
 */
export function makeMatcher(query: string, caseSensitive: boolean, regex: boolean): Matcher | null {
  const q = query.trim()
  if (!q) return null
  if (regex) {
    try {
      const re = new RegExp(q, caseSensitive ? '' : 'i')
      return { test: (r) => re.test(haystack(r)[0]) }
    } catch (e) {
      return { test: () => true, error: (e as Error).message }
    }
  }
  const terms = q.split(/\s+/).map((t) => {
    const neg = t.startsWith('!') && t.length > 1
    const text = neg ? t.slice(1) : t
    return { neg, text: caseSensitive ? text : text.toLowerCase() }
  })
  return {
    test: (r) => {
      const [raw, lower] = haystack(r)
      const h = caseSensitive ? raw : lower
      return terms.every((t) => h.includes(t.text) !== t.neg)
    },
  }
}
