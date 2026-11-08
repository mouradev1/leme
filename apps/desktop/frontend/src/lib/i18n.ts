/**
 * Two languages: Portuguese (the source text) and English.
 *
 * tr('Texto em português') returns the English text when the language is
 * English, using the Portuguese text as the key; missing entries fall back to
 * Portuguese, so nothing ever shows blank. Placeholders: tr('{n} linhas', { n }).
 * The choice is saved and also sent to the backend, so its messages follow.
 */
import { ref, watch } from 'vue'
import { SetLanguage } from '../../wailsjs/go/api/AppAPI'
import { en } from './i18n-en'

export type Lang = 'pt' | 'en'
const KEY = 'leme:lang'

function initial(): Lang {
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'pt' || saved === 'en') return saved
  } catch { /* storage unavailable */ }
  return /^pt\b/i.test(navigator.language) ? 'pt' : 'en'
}

export const lang = ref<Lang>(initial())
document.documentElement.lang = lang.value === 'pt' ? 'pt-BR' : 'en'
watch(lang, (l) => {
  try { localStorage.setItem(KEY, l) } catch { /* storage unavailable */ }
  document.documentElement.lang = l === 'pt' ? 'pt-BR' : 'en'
  void SetLanguage(l).catch(() => {})
}, { immediate: false })
void SetLanguage(lang.value).catch(() => {})

export function tr(pt: string, vars?: Record<string, unknown>): string {
  let s = lang.value === 'en' ? en[pt] ?? pt : pt
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.split(`{${k}}`).join(String(v))
  return s
}

/** Locale for dates and numbers. */
export const locale = () => (lang.value === 'pt' ? 'pt-BR' : 'en-US')
