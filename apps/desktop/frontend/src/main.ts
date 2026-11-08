import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import './lib/theme' // applies data-theme before mount
import { lang } from './lib/i18n'

createApp(App).use(createPinia()).mount('#app')

// Splash (index.html): stays at least a moment so it doesn't flash, leaves as
// soon as the cluster list is ready (App dispatches "leme:ready"), never more
// than 3 seconds.
const shownAt = performance.now()
if (lang.value === 'en') {
  const tag = document.querySelector('#splash .tag')
  if (tag) tag.textContent = 'Kubernetes, without the weight.'
}
let gone = false
function hideSplash() {
  if (gone) return
  gone = true
  const el = document.getElementById('splash')
  if (!el) return
  window.setTimeout(() => {
    el.classList.add('out')
    window.setTimeout(() => el.remove(), 400)
  }, Math.max(0, 900 - (performance.now() - shownAt)))
}
window.addEventListener('leme:ready', hideSplash, { once: true })
window.setTimeout(hideSplash, 3000)
