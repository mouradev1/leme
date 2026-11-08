<script setup lang="ts">
/**
 * xterm.js front for both container exec and local PTY sessions.
 * Data path: keystrokes -> TerminalAPI.Write -> stdin pipe / PTY master;
 * output <- "term:<id>:data" (base64, batched per frame in Go).
 */
import { tr } from '@/lib/i18n'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Terminal } from '@xterm/xterm'
import { Close, OpenExec, OpenLocal, OpenNodeShell, Resize, Write } from '../../wailsjs/go/api/TerminalAPI'
import type { terminal } from '../../wailsjs/go/models'
import { b64ToBytes, errMsg, newId, on } from '@/lib/bridge'
import { terminalTheme, theme } from '@/lib/theme'
import { ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'

const props = defineProps<{
  cluster: string
  target?: terminal.ExecTarget // container shell
  node?: string // root shell on a node (temporary privileged pod)
  // neither: local shell scoped to `cluster`
}>()

const host = ref<HTMLElement | null>(null)
const id = newId()
const offs: (() => void)[] = []
let term: Terminal | null = null
let ro: ResizeObserver | null = null

watch(theme, () => term && (term.options.theme = terminalTheme()))

// Copy/paste. xterm draws its own selection, which the system's Copy command
// can't see, so copying goes through the native clipboard:
// ⌘C (macOS) / Ctrl+Shift+C (Windows, Linux), the right-click menu, or the
// Edit menu's Copy (handled via the "copy" event).
const isMac = /Mac/i.test(navigator.platform)
const toast = ref('')
let toastTimer = 0
function flash(t: string) {
  toast.value = t
  clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = ''), 1200)
}
async function copySelection() {
  const text = term?.getSelection()
  if (!text) return
  await ClipboardSetText(text)
  flash(tr('Copiado'))
  term?.focus()
}
async function pasteClipboard() {
  const text = await ClipboardGetText()
  if (text) term?.paste(text)
  term?.focus()
}
function onCopyEvent(e: ClipboardEvent) {
  if (!term?.hasSelection()) return
  e.clipboardData?.setData('text/plain', term.getSelection())
  e.preventDefault()
  flash(tr('Copiado'))
}

type MenuAction = 'copy' | 'paste' | 'all' | 'clear'
const menu = ref<{ x: number; y: number; items: MenuItem<MenuAction>[] } | null>(null)
function openMenu(e: MouseEvent) {
  const items: MenuItem<MenuAction>[] = []
  if (term?.hasSelection()) items.push({ value: 'copy', label: tr('Copiar  ({a})', { a: isMac ? '⌘C' : 'Ctrl+Shift+C' }) })
  items.push(
    { value: 'paste', label: `Colar  (${isMac ? '⌘V' : 'Ctrl+Shift+V'})` },
    { value: 'all', label: 'Selecionar tudo', separatorBefore: true },
    { value: 'clear', label: 'Limpar tela' },
  )
  menu.value = { x: e.clientX, y: e.clientY, items }
}
function onMenu(a: MenuAction) {
  if (a === 'copy') void copySelection()
  else if (a === 'paste') void pasteClipboard()
  else if (a === 'all') term?.selectAll()
  else term?.clear()
}

onMounted(async () => {
  const [{ Terminal }, { FitAddon }] = await Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')])
  term = new Terminal({
    fontSize: 13,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    cursorBlink: true,
    scrollback: 5000, // bounded: each line costs memory in the WebView
    macOptionClickForcesSelection: true, // Option+drag selects even when a program captures the mouse (vim, htop)
    theme: terminalTheme(),
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.open(host.value!)
  fit.fit()

  // Subscribe before opening so no early output (the prompt) is lost.
  offs.push(on<string>(`term:${id}:data`, (b64) => term?.write(b64ToBytes(b64))))
  offs.push(on<string>(`term:${id}:exit`, (reason) => {
    term?.write(`\r\n\x1b[90m[session closed${reason ? `: ${reason}` : ''}]\x1b[0m\r\n`)
  }))

  term.attachCustomKeyEventHandler((e) => {
    if (e.type !== 'keydown') return true
    const k = e.key.toLowerCase()
    const copyKey = isMac ? e.metaKey && !e.shiftKey && k === 'c' : e.ctrlKey && e.shiftKey && k === 'c'
    if (copyKey && term?.hasSelection()) {
      e.preventDefault()
      void copySelection()
      return false
    }
    if (!isMac && e.ctrlKey && e.shiftKey && k === 'v') { // macOS pastes natively via ⌘V
      e.preventDefault()
      void pasteClipboard()
      return false
    }
    if (isMac && e.metaKey && k === 'a') { term?.selectAll(); return false }
    return true
  })
  host.value!.addEventListener('copy', onCopyEvent, true)

  term.onData((d) => void Write(id, d))
  term.onResize(({ cols, rows }) => void Resize(id, cols, rows))

  let raf = 0
  ro = new ResizeObserver(() => {
    cancelAnimationFrame(raf)
    raf = requestAnimationFrame(() => fit.fit()) // fires onResize -> Resize RPC
  })
  ro.observe(host.value!)

  try {
    if (props.target) await OpenExec(id, props.cluster, props.target, term.cols, term.rows)
    else if (props.node) await OpenNodeShell(id, props.cluster, props.node, term.cols, term.rows)
    else await OpenLocal(id, props.cluster, term.cols, term.rows)
    term.focus()
  } catch (e) {
    term.write(`\x1b[31m${errMsg(e)}\x1b[0m\r\n`)
  }
})

onBeforeUnmount(() => {
  host.value?.removeEventListener('copy', onCopyEvent, true)
  void Close(id)
  offs.forEach((off) => off())
  ro?.disconnect()
  term?.dispose()
})
</script>

<template>
  <div class="relative h-full w-full">
    <div ref="host" class="h-full w-full p-1" :style="{ background: terminalTheme().background }" @contextmenu.prevent="openMenu" />
    <span v-if="toast" class="pointer-events-none absolute top-2 right-3 rounded bg-zinc-800/90 px-2 py-0.5 text-xs text-zinc-100">{{ toast }}</span>
    <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menu.items" @select="onMenu" @close="menu = null" />
  </div>
</template>
