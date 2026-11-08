<script setup lang="ts">
/**
 * Tab bar of one main-area pane. Tabs can be reordered or dragged to the other
 * pane / beside this one; dock tabs (logs, terminals) can be dropped here too.
 */
import { tr } from '@/lib/i18n'
import { ref } from 'vue'
import { dragging, type MainTab } from '@/lib/workspace'
import Icon from './Icon.vue'

export interface TabLabel { title: string; sub?: string; icon: string; tone?: string }

const props = defineProps<{
  pane: number
  tabs: MainTab[]
  active: string
  focused: boolean // highlight when split, so it's clear where sidebar clicks go
  splitState: 'none' | 'can-split' | 'can-unsplit'
  label: (t: MainTab) => TabLabel
}>()
const emit = defineEmits<{
  select: [id: string]
  close: [id: string]
  new: []
  split: []
  unsplit: []
  drop: [at: number]
  menu: [e: MouseEvent, id: string]
}>()

const dropAt = ref<number | null>(null)

function onDragStart(e: DragEvent, t: MainTab) {
  dragging.value = { src: 'main', pane: props.pane, id: t.id, dock: t.view.type === 'dock' ? t.view.id : undefined }
  e.dataTransfer?.setData('text/plain', t.id) // some engines need data to start a drag
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
function onDragOverTab(e: DragEvent, i: number) {
  if (!dragging.value) return
  e.preventDefault()
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  dropAt.value = e.clientX < r.left + r.width / 2 ? i : i + 1
}
function onDragOverStrip(e: DragEvent) {
  if (!dragging.value) return
  e.preventDefault()
  if (e.target === e.currentTarget) dropAt.value = props.tabs.length
}
function onDrop(e: DragEvent) {
  if (!dragging.value || dropAt.value == null) return
  e.preventDefault()
  emit('drop', dropAt.value)
  dropAt.value = null
}
function onAux(e: MouseEvent, id: string) {
  if (e.button === 1) emit('close', id) // middle click closes, like a browser
}
</script>

<template>
  <div
    class="flex h-9 shrink-0 items-center gap-0.5 overflow-x-auto border-b px-1.5 text-xs [scrollbar-width:none]"
    :class="focused ? 'border-sky-700/60 bg-zinc-900/70' : 'border-zinc-800 bg-zinc-925'"
    @dragover="onDragOverStrip" @dragleave.self="dropAt = null" @drop="onDrop"
    @dblclick.self="emit('new')"
  >
    <template v-for="(t, i) in tabs" :key="t.id">
      <span v-if="dropAt === i" class="h-5 w-0.5 shrink-0 rounded bg-sky-400" />
      <button
        draggable="true"
        class="group flex h-7 max-w-52 shrink-0 items-center gap-1.5 rounded-md px-2.5"
        :class="t.id === active ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200'"
        :title="label(t).sub ? `${label(t).title} · ${label(t).sub}` : label(t).title"
        @click="emit('select', t.id)" @auxclick="onAux($event, t.id)" @mousedown.middle.prevent
        @contextmenu.prevent="emit('menu', $event, t.id)"
        @dragstart="onDragStart($event, t)" @dragend="dragging = null; dropAt = null"
        @dragover="onDragOverTab($event, i)"
      >
        <Icon :name="label(t).icon" :size="13" class="shrink-0" :class="label(t).tone ?? 'text-zinc-500'" />
        <span class="truncate">{{ label(t).title }}</span>
        <span v-if="label(t).sub" class="truncate text-zinc-500">{{ label(t).sub }}</span>
        <span
          class="ml-0.5 shrink-0 rounded p-0.5 text-zinc-500 opacity-0 group-hover:opacity-100 hover:bg-zinc-700 hover:text-zinc-100"
          :class="t.id === active && 'opacity-60'" :title="tr('Fechar aba')" @click.stop="emit('close', t.id)"
        ><Icon name="close" :size="11" /></span>
      </button>
    </template>
    <span v-if="dropAt === tabs.length" class="h-5 w-0.5 shrink-0 rounded bg-sky-400" />
    <button class="grid h-7 w-7 shrink-0 place-items-center rounded-md text-zinc-500 hover:bg-zinc-800 hover:text-zinc-100" :title="tr('Nova aba (clique do meio ou ⌘-clique no menu lateral também abre em nova aba)')" @click="emit('new')">
      <Icon name="plus" :size="13" />
    </button>
    <div class="ml-auto flex shrink-0 items-center pl-2">
      <button v-if="splitState === 'can-split'" class="strip-btn" :title="tr('Dividir tela (ou arraste uma aba para a direita da tela)')" @click="emit('split')">
        <Icon name="split" :size="14" />
      </button>
      <button v-else-if="splitState === 'can-unsplit'" class="strip-btn" :title="tr('Fechar divisão (as abas deste painel vão para o outro)')" @click="emit('unsplit')">
        <Icon name="close" :size="13" />
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";
.strip-btn { @apply grid h-7 w-7 place-items-center rounded-md text-zinc-500 hover:bg-zinc-800 hover:text-zinc-100; }
</style>
