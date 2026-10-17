<script setup lang="ts">
/** Virtualized live log tail with search, follow mode and multi-container. */
import { computed, nextTick, ref, toRef, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { stream } from '../../wailsjs/go/models'
import { useLogStream } from '@/composables/useLogStream'

const props = defineProps<{ cluster: string; namespace: string; pod: string; containers: string[]; previous?: boolean }>()

const tail = ref(500)
const previous = ref(props.previous ?? false) // logs of the crashed (previous) run
const timestamps = ref(false)
const search = ref('')
const follow = ref(true)

const opts = computed(() => stream.LogOptions.createFrom({
  namespace: props.namespace,
  targets: props.containers.map((c) => ({ pod: props.pod, container: c })),
  tailLines: tail.value, sinceSeconds: 0,
  timestamps: timestamps.value, previous: previous.value, follow: !previous.value,
}))
const { lines, status, error } = useLogStream(toRef(props, 'cluster'), opts)

const visible = computed(() => {
  const q = search.value.trim().toLowerCase()
  const all = lines.value
  return q ? all.filter((l) => l.t.toLowerCase().includes(q)) : all
})
const multi = computed(() => props.containers.length > 1)

const scrollEl = ref<HTMLElement | null>(null)
const virtualizer = useVirtualizer(computed(() => ({
  count: visible.value.length,
  getScrollElement: () => scrollEl.value,
  estimateSize: () => 18,
  overscan: 30,
})))

watch(visible, async () => {
  if (!follow.value || !visible.value.length) return
  await nextTick()
  virtualizer.value.scrollToIndex(visible.value.length - 1, { align: 'end' })
})

function onScroll() {
  const el = scrollEl.value!
  follow.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40 // re-arm at bottom
}

const palette = ['text-sky-400', 'text-amber-400', 'text-emerald-400', 'text-fuchsia-400', 'text-rose-400']
const colorOf = (c: string) => palette[Math.max(0, props.containers.indexOf(c)) % palette.length]
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-center gap-3 border-b border-zinc-800 px-3 py-1.5 text-xs text-zinc-400">
      <input v-model="search" placeholder="Search…" class="w-56 rounded bg-zinc-900 px-2 py-1 text-zinc-200 outline-none" />
      <label>tail <input v-model.number="tail" type="number" class="w-16 rounded bg-zinc-900 px-1" /></label>
      <label><input v-model="timestamps" type="checkbox" /> timestamps</label>
      <label><input v-model="previous" type="checkbox" /> previous</label>
      <label><input v-model="follow" type="checkbox" /> follow</label>
      <span class="ml-auto">{{ visible.length }} lines · {{ status }}</span>
      <span v-if="error" class="truncate text-red-400">{{ error }}</span>
    </div>
    <div ref="scrollEl" class="min-h-0 flex-1 overflow-auto font-mono text-[12px] leading-[18px]" @scroll.passive="onScroll">
      <div class="relative" :style="{ height: `${virtualizer.getTotalSize()}px` }">
        <div
          v-for="v in virtualizer.getVirtualItems()" :key="v.index"
          class="absolute inset-x-0 whitespace-pre px-3"
          :style="{ transform: `translateY(${v.start}px)` }"
        >
          <span v-if="multi" :class="colorOf(visible[v.index].c)">[{{ visible[v.index].c }}] </span>{{ visible[v.index].t }}
        </div>
      </div>
    </div>
  </div>
</template>
