<script setup lang="ts">
/**
 * Bottom dock with logs/terminal tabs. Always shows its tab bar; can be
 * resized (drag the top edge), collapsed and maximized. Tabs stay mounted
 * (v-show) so shells and log streams survive switching tabs.
 */
import { onBeforeUnmount, ref, watch } from 'vue'
import type { DockTab } from '@/lib/types'
import LogViewer from './LogViewer.vue'
import TerminalPane from './TerminalPane.vue'
import Icon from './Icon.vue'

const props = defineProps<{ cluster: string; tabs: DockTab[] }>()
const active = defineModel<string>('active', { required: true })
const maximized = defineModel<boolean>('maximized', { default: false })
const emit = defineEmits<{ close: [id: string]; newTerminal: [] }>()

const height = ref(Number(safeGet('leme:dock-h')) || 320)
const collapsed = ref(props.tabs.length === 0)

// Opening a tab always reveals the dock.
watch(() => props.tabs.length, (n, old) => {
  if (n > old) collapsed.value = false
  if (n === 0) { collapsed.value = true; maximized.value = false }
})

let startY = 0, startH = 0
function onDrag(e: MouseEvent) {
  height.value = Math.min(Math.max(startH + (startY - e.clientY), 140), window.innerHeight - 160)
}
function stopDrag() {
  window.removeEventListener('mousemove', onDrag)
  window.removeEventListener('mouseup', stopDrag)
  document.body.style.cursor = ''
  try { localStorage.setItem('leme:dock-h', String(height.value)) } catch { /* ignore */ }
}
function startDrag(e: MouseEvent) {
  if (collapsed.value || maximized.value) return
  startY = e.clientY
  startH = height.value
  document.body.style.cursor = 'row-resize'
  window.addEventListener('mousemove', onDrag)
  window.addEventListener('mouseup', stopDrag)
}
onBeforeUnmount(stopDrag)

function safeGet(k: string) {
  try { return localStorage.getItem(k) } catch { return null }
}

function toggleCollapse() {
  collapsed.value = !collapsed.value
  if (collapsed.value) maximized.value = false
}
function toggleMax() {
  maximized.value = !maximized.value
  if (maximized.value) collapsed.value = false
}
</script>

<template>
  <section
    class="flex shrink-0 flex-col border-t border-zinc-800 bg-zinc-925"
    :class="maximized && 'min-h-0 flex-1'"
    :style="maximized ? {} : { height: collapsed ? '36px' : `${height}px` }"
  >
    <!-- resize handle -->
    <div v-if="!collapsed && !maximized" class="-mt-1 h-1.5 shrink-0 cursor-row-resize hover:bg-sky-500/40" @mousedown.prevent="startDrag" />

    <div class="flex h-[34px] shrink-0 items-center gap-0.5 px-2 text-xs" @dblclick.self="toggleMax">
      <button
        v-for="t in tabs" :key="t.id"
        class="group flex h-7 max-w-56 items-center gap-1.5 rounded-md px-2.5"
        :class="t.id === active && !collapsed ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200'"
        :title="t.title"
        @click="active = t.id; collapsed = false"
      >
        <Icon :name="t.type === 'logs' ? 'logs' : 'terminal'" :size="13" :class="t.type === 'logs' ? 'text-amber-400' : 'text-emerald-400'" />
        <span class="truncate">{{ t.title }}</span>
        <span class="ml-0.5 rounded p-0.5 text-zinc-500 opacity-60 group-hover:opacity-100 hover:bg-zinc-700 hover:text-zinc-100" @click.stop="emit('close', t.id)">
          <Icon name="close" :size="11" />
        </span>
      </button>
      <button class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" title="Novo terminal local" @click="emit('newTerminal')">
        <Icon name="plus" :size="14" />
      </button>
      <span v-if="!tabs.length" class="ml-1 text-zinc-600">Terminal</span>

      <div class="ml-auto flex items-center gap-0.5">
        <button v-if="tabs.length" class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="maximized ? 'Restaurar' : 'Maximizar'" @click="toggleMax">
          <Icon :name="maximized ? 'restore' : 'maximize'" :size="14" />
        </button>
        <button v-if="tabs.length" class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="collapsed ? 'Expandir' : 'Recolher'" @click="toggleCollapse">
          <Icon :name="collapsed ? 'expand' : 'collapse'" :size="14" />
        </button>
      </div>
    </div>

    <div v-show="!collapsed" class="relative min-h-0 flex-1 border-t border-zinc-800/80">
      <div v-for="t in tabs" v-show="t.id === active" :key="t.id" class="absolute inset-0">
        <LogViewer v-if="t.type === 'logs'" :cluster="cluster" :namespace="t.namespace" :pod="t.pod" :containers="t.containers" :previous="t.previous" />
        <TerminalPane v-else :cluster="cluster" :target="t.target" />
      </div>
    </div>
  </section>
</template>
