<script setup lang="ts">
/** "Why is this pod restarting / not running", in plain language. */
import type { pods } from '../../wailsjs/go/models'

defineProps<{ items: pods.Diagnosis[] }>()
const emit = defineEmits<{ previousLogs: [container: string] }>()

const style = {
  error: { box: 'border-rose-900/70 bg-rose-950/40', title: 'text-rose-200', icon: '!', iconCls: 'bg-rose-500/20 text-rose-300' },
  warn: { box: 'border-amber-900/70 bg-amber-950/30', title: 'text-amber-200', icon: '!', iconCls: 'bg-amber-500/20 text-amber-300' },
  info: { box: 'border-zinc-800 bg-zinc-900/60', title: 'text-zinc-200', icon: 'i', iconCls: 'bg-zinc-700 text-zinc-300' },
} as const
const s = (sev: string) => style[(sev in style ? sev : 'info') as keyof typeof style]
</script>

<template>
  <section v-if="items.length" class="space-y-2">
    <div v-for="(d, i) in items" :key="i" class="rounded-lg border px-3 py-2.5 text-xs" :class="s(d.severity).box">
      <div class="flex items-start gap-2">
        <span class="mt-px grid h-4 w-4 shrink-0 place-items-center rounded-full text-[10px] font-bold" :class="s(d.severity).iconCls">{{ s(d.severity).icon }}</span>
        <div class="min-w-0 flex-1">
          <p class="font-medium" :class="s(d.severity).title">
            <span v-if="d.container" class="font-mono text-zinc-400">{{ d.container }}: </span>{{ d.title }}
          </p>
          <p v-if="d.detail" class="mt-1 break-words whitespace-pre-line text-zinc-300 select-text">{{ d.detail }}</p>
          <p v-if="d.hint" class="mt-1.5 text-zinc-400"><span class="text-zinc-500">Dica:</span> {{ d.hint }}</p>
          <button v-if="d.container && /Reiniciou|CrashLoop/.test(d.title)" class="mt-2 rounded border border-zinc-700 px-2 py-0.5 text-zinc-200 hover:bg-zinc-800" @click="emit('previousLogs', d.container)">
            Ver logs da execução anterior
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
