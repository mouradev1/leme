<script setup lang="ts" generic="T extends string">
/** Floating menu at screen coordinates (row ⋮ button and right-click). */
import { computed, ref } from 'vue'
import { useDismiss } from '@/lib/dom'

export interface MenuItem<V extends string> {
  value: V
  label: string
  danger?: boolean
  separatorBefore?: boolean
}

const props = defineProps<{ x: number; y: number; items: MenuItem<T>[]; title?: string }>()
const emit = defineEmits<{ select: [value: T]; close: [] }>()

const root = ref<HTMLElement | null>(null)
useDismiss(root, () => emit('close'))

// Keep the menu inside the window.
const style = computed(() => {
  const w = 200
  const h = props.items.length * 30 + (props.title ? 32 : 8)
  return {
    left: `${Math.min(props.x, window.innerWidth - w - 8)}px`,
    top: `${Math.min(props.y, window.innerHeight - h - 8)}px`,
    width: `${w}px`,
  }
})
</script>

<template>
  <Teleport to="body">
    <div ref="root" class="fixed z-[60] overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900/95 py-1 text-xs shadow-2xl shadow-black/60 backdrop-blur" :style="style">
      <div v-if="title" class="truncate border-b border-zinc-800 px-3 pt-1 pb-2 text-[11px] text-zinc-500">{{ title }}</div>
      <template v-for="it in items" :key="it.value">
        <div v-if="it.separatorBefore" class="my-1 border-t border-zinc-800" />
        <button
          class="block w-full px-3 py-1.5 text-left hover:bg-zinc-800"
          :class="it.danger ? 'text-rose-400 hover:bg-rose-950/50' : 'text-zinc-200'"
          @click="emit('select', it.value); emit('close')"
        >{{ it.label }}</button>
      </template>
    </div>
  </Teleport>
</template>
