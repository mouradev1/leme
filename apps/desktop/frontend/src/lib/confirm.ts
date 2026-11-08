/** App-wide confirmation dialog (production clusters ask for the name), provided by App.vue. */
import type { InjectionKey } from 'vue'

export type ConfirmFn = (title: string, run: () => Promise<void>, danger?: boolean) => void
export const confirmKey: InjectionKey<ConfirmFn> = Symbol('confirm')
export const toastKey: InjectionKey<(msg: string) => void> = Symbol('toast')
