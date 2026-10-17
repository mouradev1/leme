import { onBeforeUnmount, ref } from 'vue'

/**
 * Width of a right-side panel that the user resizes by dragging its left
 * edge. Remembered per panel (key) across restarts.
 */
export function usePanelWidth(key: string, initial: number, min = 360) {
  const storageKey = `leme:w:${key}`
  const width = ref(read() ?? initial)
  const dragging = ref(false)
  let startX = 0
  let startW = 0

  function read(): number | null {
    try {
      const v = Number(localStorage.getItem(storageKey))
      return v > 0 ? v : null
    } catch {
      return null
    }
  }

  const max = () => Math.max(min, window.innerWidth - 420) // always leave room for the list

  function onMove(e: MouseEvent) {
    width.value = Math.min(Math.max(startW + (startX - e.clientX), min), max())
  }
  function stop() {
    if (!dragging.value) return
    dragging.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', stop)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    try { localStorage.setItem(storageKey, String(Math.round(width.value))) } catch { /* ignore */ }
  }
  function start(e: MouseEvent) {
    e.preventDefault()
    dragging.value = true
    startX = e.clientX
    startW = width.value
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', stop)
  }
  /** Double-click on the handle: back to the default width. */
  function reset() {
    width.value = initial
    try { localStorage.removeItem(storageKey) } catch { /* ignore */ }
  }

  onBeforeUnmount(stop)
  return { width, dragging, start, reset }
}
