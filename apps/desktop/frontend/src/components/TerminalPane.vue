<script setup lang="ts">
/**
 * xterm.js front for both container exec and local PTY sessions.
 * Data path: keystrokes -> TerminalAPI.Write -> stdin pipe / PTY master;
 * output <- "term:<id>:data" (base64, batched per frame in Go).
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Terminal } from '@xterm/xterm'
import { Close, OpenExec, OpenLocal, Resize, Write } from '../../wailsjs/go/api/TerminalAPI'
import type { terminal } from '../../wailsjs/go/models'
import { b64ToBytes, errMsg, newId, on } from '@/lib/bridge'
import { terminalTheme, theme } from '@/lib/theme'

const props = defineProps<{
  cluster: string
  target?: terminal.ExecTarget // omit for a local shell scoped to `cluster`
}>()

const host = ref<HTMLElement | null>(null)
const id = newId()
const offs: (() => void)[] = []
let term: Terminal | null = null
let ro: ResizeObserver | null = null

watch(theme, () => term && (term.options.theme = terminalTheme()))

onMounted(async () => {
  const [{ Terminal }, { FitAddon }] = await Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')])
  term = new Terminal({
    fontSize: 13,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    cursorBlink: true,
    scrollback: 5000, // bounded: each line costs memory in the WebView
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
    else await OpenLocal(id, props.cluster, term.cols, term.rows)
    term.focus()
  } catch (e) {
    term.write(`\x1b[31m${errMsg(e)}\x1b[0m\r\n`)
  }
})

onBeforeUnmount(() => {
  void Close(id)
  offs.forEach((off) => off())
  ro?.disconnect()
  term?.dispose()
})
</script>

<template>
  <div ref="host" class="h-full w-full p-1" :style="{ background: terminalTheme().background }" />
</template>
