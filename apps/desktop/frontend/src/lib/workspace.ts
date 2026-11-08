/**
 * Main-area tabs and split panes, one workspace per cluster.
 *
 * Only each pane's active tab is mounted, so only the visible screens hold a
 * watch; hidden tabs list again when shown. Workspaces are saved per cluster
 * (browser storage, best effort) and come back when the app reopens. Tabs that
 * show a dock stream (logs/terminal) live only for this session.
 */
import { reactive, shallowRef, watch } from 'vue'
import { newId } from './bridge'
import type { View } from './types'

export interface MainTab { id: string; view: View; search: string; focus?: string } // focus: "ns/name" to open once
export interface Pane { tabs: MainTab[]; active: string }
export interface Workspace { panes: Pane[]; focus: number; ratio: number }

const KEY = 'leme:workspaces'
const MAX_PANES = 2

const tab = (view: View, search = ''): MainTab => ({ id: newId(), view, search })
const fresh = (): Workspace => {
  const t = tab({ type: 'overview' })
  return { panes: [{ tabs: [t], active: t.id }], focus: 0, ratio: 0.5 }
}

// Drop anything malformed or session-only (dock streams) from saved state.
function clean(w: Workspace | undefined): Workspace | null {
  if (!w || !Array.isArray(w.panes)) return null
  const panes = w.panes.slice(0, MAX_PANES)
    .map((p) => {
      const tabs = (p?.tabs ?? []).filter((t) => t?.id && t.view?.type && t.view.type !== 'dock'
        && (t.view.type !== 'resource' || !!t.view.kind?.resource))
        .map((t) => ({ id: t.id, view: t.view, search: typeof t.search === 'string' ? t.search : '' }))
      return { tabs, active: tabs.some((t) => t.id === p.active) ? p.active : tabs[0]?.id ?? '' }
    })
    .filter((p) => p.tabs.length)
  if (!panes.length) return null
  const ratio = Number(w.ratio)
  return { panes, focus: Math.min(Math.max(0, w.focus | 0), panes.length - 1), ratio: ratio > 0.15 && ratio < 0.85 ? ratio : 0.5 }
}

function load(): Record<string, Workspace> {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) ?? '{}') as Record<string, Workspace>
    const out: Record<string, Workspace> = {}
    for (const [c, w] of Object.entries(raw)) {
      const ok = clean(w)
      if (ok) out[c] = ok
    }
    return out
  } catch {
    return {}
  }
}

export const workspaces = reactive<Record<string, Workspace>>(load())

let saveTimer = 0
watch(workspaces, () => {
  clearTimeout(saveTimer)
  saveTimer = window.setTimeout(() => {
    try {
      const out: Record<string, Workspace> = {}
      for (const [c, w] of Object.entries(workspaces)) {
        const ok = clean(JSON.parse(JSON.stringify(w)))
        if (ok) out[c] = ok
      }
      localStorage.setItem(KEY, JSON.stringify(out))
    } catch { /* storage unavailable: tabs just won't be restored */ }
  }, 400)
}, { deep: true })

/** The cluster's workspace, created with an Overview tab on first use. */
export function ensure(cluster: string): Workspace {
  if (!workspaces[cluster]) workspaces[cluster] = fresh()
  return workspaces[cluster]
}

export const activeOf = (p: Pane) => p.tabs.find((t) => t.id === p.active) ?? null

/** Show `view` in the pane's current tab (a sidebar click). */
export function setView(w: Workspace, pane: number, view: View, search = '', focus?: string) {
  const p = w.panes[pane]
  const cur = activeOf(p)
  if (!cur || cur.view.type === 'dock') return openTab(w, pane, view, search, focus) // never replace a stream
  cur.view = view
  cur.search = search
  cur.focus = focus
  w.focus = pane
}

/** Open `view` in a new tab, right after the pane's current one. */
export function openTab(w: Workspace, pane: number, view: View, search = '', focus?: string) {
  const p = w.panes[pane]
  const t = tab(view, search)
  t.focus = focus
  const at = p.tabs.findIndex((x) => x.id === p.active)
  p.tabs.splice(at < 0 ? p.tabs.length : at + 1, 0, t)
  p.active = t.id
  w.focus = pane
}

export function closeTab(w: Workspace, pane: number, id: string) {
  const p = w.panes[pane]
  const at = p.tabs.findIndex((t) => t.id === id)
  if (at < 0) return
  p.tabs.splice(at, 1)
  if (p.active === id) p.active = (p.tabs[at] ?? p.tabs[at - 1])?.id ?? ''
  if (!p.tabs.length && w.panes.length > 1) dropPane(w, pane)
}

function dropPane(w: Workspace, pane: number) {
  w.panes.splice(pane, 1)
  w.focus = 0
}

/** Split: the focused tab is duplicated beside it (a stream tab can't be, so Overview). */
export function split(w: Workspace) {
  if (w.panes.length >= MAX_PANES) return
  const cur = activeOf(w.panes[w.focus])
  const t = tab(cur && cur.view.type !== 'dock' ? cur.view : { type: 'overview' }, cur?.view.type === 'dock' ? '' : cur?.search)
  w.panes.push({ tabs: [t], active: t.id })
  w.focus = w.panes.length - 1
}

/** Back to one pane: the second pane's tabs join the first. */
export function unsplit(w: Workspace) {
  if (w.panes.length < 2) return
  const [a, b] = w.panes
  a.tabs.push(...b.tabs)
  w.panes.splice(1)
  w.focus = 0
}

/**
 * Move a tab to pane `to` (MAX_PANES-1 or panes.length opens a new pane when
 * there's room), before index `at` (end when omitted).
 */
export function moveTab(w: Workspace, from: number, id: string, to: number, at?: number) {
  const src = w.panes[from]
  const i = src?.tabs.findIndex((t) => t.id === id) ?? -1
  if (i < 0) return
  if (to >= w.panes.length) {
    if (w.panes.length >= MAX_PANES) to = w.panes.length - 1
    else if (src.tabs.length === 1) return // moving a pane's only tab beside itself changes nothing
    else w.panes.push({ tabs: [], active: '' })
  }
  const [t] = src.tabs.splice(i, 1)
  const dst = w.panes[to]
  let pos = at ?? dst.tabs.length
  if (from === to && i < pos) pos-- // removal shifted the target left
  dst.tabs.splice(Math.min(pos, dst.tabs.length), 0, t)
  dst.active = t.id
  if (src.active === id && src !== dst) src.active = (src.tabs[i] ?? src.tabs[i - 1])?.id ?? ''
  w.focus = to
  if (!src.tabs.length && src !== dst) {
    dropPane(w, w.panes.indexOf(src))
    w.focus = w.panes.indexOf(dst)
  }
}

/** Show a dock stream (logs/terminal) as a tab in pane `pane` (or a new one). */
export function placeDock(w: Workspace, pane: number, dockId: string) {
  if (pane >= w.panes.length) {
    if (w.panes.length >= MAX_PANES) pane = w.panes.length - 1
    else w.panes.push({ tabs: [], active: '' })
  }
  const p = w.panes[pane]
  const t = tab({ type: 'dock', id: dockId })
  p.tabs.push(t)
  p.active = t.id
  w.focus = pane
}

/** Ids of dock streams currently shown in any pane of any cluster. */
export function placedDockIds(): Set<string> {
  const out = new Set<string>()
  for (const w of Object.values(workspaces))
    for (const p of w.panes)
      for (const t of p.tabs) if (t.view.type === 'dock') out.add(t.view.id)
  return out
}

/** A dock stream went back to the dock or was closed: drop its main tabs. */
export function forgetDock(dockId: string) {
  for (const w of Object.values(workspaces)) {
    for (let pi = w.panes.length - 1; pi >= 0; pi--) { // backwards: closing may drop a pane
      for (const t of [...w.panes[pi].tabs]) if (t.view.type === 'dock' && t.view.id === dockId) closeTab(w, pi, t.id)
    }
  }
}

// ---- Drag and drop (main tabs and dock tabs) ---------------------------------
export type DragTab = { src: 'main'; pane: number; id: string; dock?: string } | { src: 'dock'; id: string }
export const dragging = shallowRef<DragTab | null>(null)
