import { computed, ref, watch } from 'vue'

export type ThemePref = 'system' | 'dark' | 'light'

function read(): ThemePref {
  try {
    const v = localStorage.getItem('leme:theme')
    return v === 'dark' || v === 'light' || v === 'system' ? v : 'system'
  } catch {
    return 'system'
  }
}

const media = window.matchMedia('(prefers-color-scheme: light)')
const systemLight = ref(media.matches)
media.addEventListener('change', (e) => (systemLight.value = e.matches))

/** User preference (persisted). */
export const themePref = ref<ThemePref>(read())
/** What is actually applied right now. */
export const theme = computed<'dark' | 'light'>(() =>
  themePref.value === 'system' ? (systemLight.value ? 'light' : 'dark') : themePref.value)

watch(themePref, (v) => {
  try { localStorage.setItem('leme:theme', v) } catch { /* storage unavailable */ }
})
watch(theme, (t) => (document.documentElement.dataset.theme = t), { immediate: true })

export const monacoTheme = () => (theme.value === 'light' ? 'leme-light' : 'leme-dark')

/** xterm palette per theme (ANSI colors tuned for contrast on each background). */
export function terminalTheme() {
  return theme.value === 'light'
    ? {
        background: '#fafafa', foreground: '#27272a', cursor: '#18181b', selectionBackground: '#bae6fd',
        black: '#27272a', red: '#be123c', green: '#047857', yellow: '#a16207', blue: '#1d4ed8',
        magenta: '#a21caf', cyan: '#0e7490', white: '#52525b',
        brightBlack: '#52525b', brightRed: '#e11d48', brightGreen: '#059669', brightYellow: '#ca8a04',
        brightBlue: '#2563eb', brightMagenta: '#c026d3', brightCyan: '#0891b2', brightWhite: '#18181b',
      }
    : { background: '#09090b', foreground: '#e4e4e7', cursor: '#e4e4e7', selectionBackground: '#3f3f46' }
}
