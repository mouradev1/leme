import { EventsOn } from '../../wailsjs/runtime/runtime'

export const newId = (): string => crypto.randomUUID()

/** Typed EventsOn. Returns the unsubscribe function. */
export function on<T>(topic: string, cb: (payload: T) => void): () => void {
  return EventsOn(topic, (payload: T) => cb(payload))
}

/** Terminal output arrives base64-encoded (binary-safe across the JSON bridge). */
export function b64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

/** Coalesces many reactive writes into one per animation frame. */
export function rafThrottle(fn: () => void): () => void {
  let queued = false
  return () => {
    if (queued) return
    queued = true
    requestAnimationFrame(() => {
      queued = false
      fn()
    })
  }
}

export function errMsg(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
