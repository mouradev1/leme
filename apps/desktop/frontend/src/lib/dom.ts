import { onBeforeUnmount, onMounted, type Ref } from 'vue'

/** Calls `cb` on mousedown outside `el` or on Escape (popovers, menus). */
export function useDismiss(el: Ref<HTMLElement | null>, cb: () => void) {
  const onDown = (e: MouseEvent) => {
    if (el.value && !el.value.contains(e.target as Node)) cb()
  }
  const onKey = (e: KeyboardEvent) => e.key === 'Escape' && cb()
  onMounted(() => {
    document.addEventListener('mousedown', onDown, true)
    document.addEventListener('keydown', onKey)
  })
  onBeforeUnmount(() => {
    document.removeEventListener('mousedown', onDown, true)
    document.removeEventListener('keydown', onKey)
  })
}

/** Stable pleasant hue for a string (cluster avatars). */
export function hue(s: string): number {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return h % 360
}

export function initials(name: string): string {
  const parts = name.split(/[-_.@/\s]+/).filter(Boolean)
  return ((parts[0]?.[0] ?? '') + (parts.at(-1)?.[0] ?? '')).toUpperCase() || '?'
}
