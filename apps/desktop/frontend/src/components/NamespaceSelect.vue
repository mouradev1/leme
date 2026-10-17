<script setup lang="ts">
/** Multi-select namespace filter with type-ahead. [] means all namespaces. */
import { computed, nextTick, ref } from 'vue'
import { useDismiss } from '@/lib/dom'
import Icon from './Icon.vue'

const props = defineProps<{ options: string[]; disabled?: boolean }>()
const model = defineModel<string[]>({ required: true })

const open = ref(false)
const q = ref('')
const root = ref<HTMLElement | null>(null)
const input = ref<HTMLInputElement | null>(null)
useDismiss(root, () => (open.value = false))

const label = computed(() => {
  if (props.disabled) return 'Cluster-wide'
  const v = model.value
  if (!v.length) return 'Todos os namespaces'
  if (v.length <= 2) return v.join(', ')
  return `${v.length} namespaces`
})

const filtered = computed(() => {
  const s = q.value.trim().toLowerCase()
  const list = s ? props.options.filter((o) => o.toLowerCase().includes(s)) : props.options
  // Selected first, so they're easy to un-tick in long lists.
  const sel = new Set(model.value)
  return [...list.filter((o) => sel.has(o)), ...list.filter((o) => !sel.has(o))]
})

async function toggleOpen() {
  if (props.disabled) return
  open.value = !open.value
  q.value = ''
  await nextTick()
  input.value?.focus()
}

function toggle(ns: string) {
  const set = new Set(model.value)
  if (set.has(ns)) set.delete(ns)
  else set.add(ns)
  model.value = [...set].sort()
}

function only(ns: string) {
  model.value = [ns]
  open.value = false
}

function onEnter() {
  const first = filtered.value[0]
  if (first) only(first)
}
</script>

<template>
  <div ref="root" class="relative">
    <button
      class="flex h-8 w-56 items-center gap-2 rounded-md border border-zinc-800 bg-zinc-900 px-2.5 text-left text-xs hover:border-zinc-700 disabled:cursor-not-allowed disabled:opacity-50"
      :disabled="disabled" @click="toggleOpen"
    >
      <svg class="h-3.5 w-3.5 shrink-0 text-zinc-500" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M2 4.5h4l1.5 1.5H14v6.5H2z" />
      </svg>
      <span class="min-w-0 flex-1 truncate" :class="model.length ? 'text-sky-300' : 'text-zinc-300'">{{ label }}</span>
      <svg class="h-3 w-3 shrink-0 text-zinc-500" viewBox="0 0 12 12" fill="currentColor"><path d="M3 4.5 6 7.5 9 4.5z" /></svg>
    </button>

    <div v-if="open" class="absolute top-9 left-0 z-40 w-72 overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/50">
      <div class="border-b border-zinc-800 p-2">
        <input
          ref="input" v-model="q" placeholder="Filtrar namespaces…"
          class="w-full rounded bg-zinc-950 px-2 py-1.5 text-xs outline-none placeholder:text-zinc-600"
          @keydown.enter.prevent="onEnter"
        />
      </div>
      <div class="max-h-80 overflow-auto py-1 text-xs">
        <button
          class="flex w-full items-center gap-2 px-3 py-1.5 hover:bg-zinc-800"
          :class="!model.length && 'text-sky-300'" @click="model = []; open = false"
        >
          <span class="grid w-3.5 place-items-center"><Icon v-if="!model.length" name="check" :size="13" /></span> Todos os namespaces
        </button>
        <div class="my-1 border-t border-zinc-800" />
        <div
          v-for="ns in filtered" :key="ns"
          class="group flex cursor-pointer items-center gap-2 px-3 py-1.5 hover:bg-zinc-800"
          @click="toggle(ns)"
        >
          <span
            class="grid h-3.5 w-3.5 place-items-center rounded-sm border text-[10px]"
            :class="model.includes(ns) ? 'border-sky-500 bg-sky-500 text-white' : 'border-zinc-600'"
          ><Icon v-if="model.includes(ns)" name="check" :size="11" /></span>
          <span class="min-w-0 flex-1 truncate">{{ ns }}</span>
          <button class="hidden rounded px-1.5 text-[10px] text-zinc-400 group-hover:block hover:bg-zinc-700 hover:text-zinc-100" @click.stop="only(ns)">
            somente
          </button>
        </div>
        <p v-if="!filtered.length" class="px-3 py-2 text-zinc-500">Nenhum namespace</p>
      </div>
    </div>
  </div>
</template>
