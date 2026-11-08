<script setup lang="ts">
/**
 * Bottom dock with logs/terminal tabs. Always shows its tab bar; can be
 * resized (drag the top edge), collapsed and maximized. The tabs' components
 * live in stable hosts (lib/hosts.ts), so shells and log streams survive
 * switching tabs and moving to a main-area pane and back.
 */
import { tr } from '@/lib/i18n'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { DockTab } from '@/lib/types'
import { dragging } from '@/lib/workspace'
import HostSlot from './HostSlot.vue'
import Icon from './Icon.vue'

const props = defineProps<{ cluster: string; tabs: DockTab[] }>()
const active = defineModel<string>('active', { required: true })
const maximized = defineModel<boolean>('maximized', { default: false })
const emit = defineEmits<{ close: [id: string]; newTerminal: []; menu: [e: MouseEvent, id: string]; dropBack: [dockId: string] }>()

// `tabs` holds every cluster's tabs; only the active cluster's are shown. The
// others stay mounted (hidden) so their shells/log streams keep running.
const visible = computed(() => props.tabs.filter((t) => t.cluster === props.cluster))
const elsewhere = computed(() => props.tabs.filter((t) => t.cluster !== props.cluster))
const elsewhereClusters = computed(() => [...new Set(elsewhere.value.map((t) => t.cluster))].join(', '))
const current = computed(() => visible.value.find((t) => t.id === active.value) ?? null)

// Drag a dock tab to a main-area pane; drop a stream tab from a pane back here.
function onDragStart(e: DragEvent, id: string) {
  dragging.value = { src: 'dock', id }
  e.dataTransfer?.setData('text/plain', id)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
const canTakeBack = computed(() => dragging.value?.src === 'main' && !!dragging.value.dock)
function onDrop(e: DragEvent) {
  const d = dragging.value
  if (d?.src !== 'main' || !d.dock) return
  e.preventDefault()
  emit('dropBack', d.dock)
  dragging.value = null
}

const height = ref(Number(safeGet('leme:dock-h')) || 320)
const collapsed = ref(visible.value.length === 0)

// Opening a tab always reveals the dock; a cluster without tabs collapses it.
watch(() => visible.value.length, (n, old) => {
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

    <div
      class="flex h-[34px] shrink-0 items-center gap-0.5 px-2 text-xs" :class="canTakeBack && 'bg-sky-500/10 outline outline-1 -outline-offset-1 outline-sky-600/60'"
      @dblclick.self="toggleMax" @dragover="canTakeBack && $event.preventDefault()" @drop="onDrop"
    >
      <button
        v-for="t in visible" :key="t.id"
        class="group flex h-7 max-w-56 items-center gap-1.5 rounded-md px-2.5"
        :class="t.id === active && !collapsed ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200'"
        :title="tr('{a} ({b}): arraste para a área principal ou clique com o botão direito', { a: t.title, b: t.cluster })"
        draggable="true"
        @click="active = t.id; collapsed = false" @contextmenu.prevent="emit('menu', $event, t.id)"
        @dragstart="onDragStart($event, t.id)" @dragend="dragging = null"
      >
        <Icon :name="t.type === 'logs' ? 'logs' : 'terminal'" :size="13" :class="t.type === 'logs' ? 'text-amber-400' : 'text-emerald-400'" />
        <span class="truncate">{{ t.title }}</span>
        <span class="ml-0.5 rounded p-0.5 text-zinc-500 opacity-60 group-hover:opacity-100 hover:bg-zinc-700 hover:text-zinc-100" @click.stop="emit('close', t.id)">
          <Icon name="close" :size="11" />
        </span>
      </button>
      <button class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="tr('Novo terminal local')" @click="emit('newTerminal')">
        <Icon name="plus" :size="14" />
      </button>
      <span v-if="canTakeBack" class="ml-1 text-sky-300">{{ tr('Solte aqui para voltar ao dock') }}</span>
      <span v-else-if="!visible.length" class="ml-1 text-zinc-600">{{ tr('Terminal') }}</span>
      <span
        v-if="elsewhere.length" class="ml-2 rounded-full border border-zinc-800 px-2 py-0.5 text-[11px] text-zinc-500"
        :title="tr('Abas abertas em: {a}. Elas continuam conectadas aos seus clusters e voltam quando você trocar para eles.', { a: elsewhereClusters })"
      >{{ tr('{n} em outros clusters', { n: elsewhere.length }) }}</span>

      <div class="ml-auto flex items-center gap-0.5">
        <button v-if="visible.length" class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="maximized ? 'Restaurar' : 'Maximizar'" @click="toggleMax">
          <Icon :name="maximized ? 'restore' : 'maximize'" :size="14" />
        </button>
        <button v-if="visible.length" class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="collapsed ? 'Expandir' : 'Recolher'" @click="toggleCollapse">
          <Icon :name="collapsed ? 'expand' : 'collapse'" :size="14" />
        </button>
      </div>
    </div>

    <div v-show="!collapsed" class="relative min-h-0 flex-1 border-t border-zinc-800/80">
      <!-- The stream itself is rendered (bound to t.cluster) in App.vue's hosts. -->
      <HostSlot v-if="current" :id="current.id" />
    </div>
  </section>
</template>
