import { onScopeDispose } from 'vue'

const GRACE_MS = 30_000

/**
 * Stops a live stream while the window is minimized/hidden and resumes it
 * when it comes back, so a forgotten leme window costs no network, no CPU and
 * no Go/JS garbage. A short grace period avoids churn on quick alt-tabs.
 * `resume` should restart without clearing what's on screen.
 */
export function usePauseWhenHidden(pause: () => void, resume: () => void) {
  let timer = 0
  let paused = false

  function onChange() {
    if (document.hidden) {
      clearTimeout(timer)
      timer = window.setTimeout(() => {
        paused = true
        pause()
      }, GRACE_MS)
    } else {
      clearTimeout(timer)
      if (paused) {
        paused = false
        resume()
      }
    }
  }

  document.addEventListener('visibilitychange', onChange)
  onScopeDispose(() => {
    clearTimeout(timer)
    document.removeEventListener('visibilitychange', onChange)
  })
}
