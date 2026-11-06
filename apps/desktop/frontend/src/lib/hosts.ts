/**
 * Stable DOM homes for dock tabs (logs, terminals).
 *
 * Each dock tab's component is teleported once into its own host element and
 * never remounted. Showing the tab in the dock or in a main-area pane just
 * moves that host element (see HostSlot.vue), so a shell or a log stream keeps
 * running, scrollback and all, when it changes place. Hosts that are not shown
 * anywhere wait in a hidden parking element, like v-show.
 */
let parking: HTMLElement | null = null
const hosts = new Map<string, HTMLElement>()

function park(): HTMLElement {
  if (!parking) {
    parking = document.createElement('div')
    parking.style.display = 'none'
    document.body.appendChild(parking)
  }
  return parking
}

export function hostOf(id: string): HTMLElement {
  let el = hosts.get(id)
  if (!el) {
    el = document.createElement('div')
    el.className = 'absolute inset-0'
    park().appendChild(el)
    hosts.set(id, el)
  }
  return el
}

/** Put a host back in the parking element (only if `from` still holds it). */
export function parkHost(id: string, from: HTMLElement) {
  const el = hosts.get(id)
  if (el && el.parentElement === from) park().appendChild(el)
}

/** Forget hosts of closed tabs (after their components unmounted). */
export function dropHosts(keep: Set<string>) {
  for (const [id, el] of hosts) {
    if (keep.has(id)) continue
    el.remove()
    hosts.delete(id)
  }
}
