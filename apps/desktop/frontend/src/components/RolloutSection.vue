<script setup lang="ts">
/** Deployment rollout: progress, revision history and "go back to a version". */
import { tr } from '@/lib/i18n'
import { computed, inject, ref, watch } from 'vue'
import { Rollout, RolloutUndo } from '../../wailsjs/go/api/ResourceAPI'
import type { ops } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'
import { confirmKey, toastKey } from '@/lib/confirm'
import { age } from '@/lib/format'
import { isReadOnly } from '@/lib/guard'

const props = defineProps<{ cluster: string; namespace: string; name: string; rv: string }>()
const confirm = inject(confirmKey)
const toast = inject(toastKey, () => {})

const data = ref<ops.Rollout | null>(null)
const error = ref('')
async function load() {
  try {
    data.value = await Rollout(props.cluster, props.namespace, props.name)
    error.value = ''
  } catch (e) {
    error.value = errMsg(e)
  }
}
// The deployment's resourceVersion changes with every status update: reload then.
watch(() => [props.cluster, props.namespace, props.name, props.rv], load, { immediate: true })

const pct = (n: number) => (data.value && data.value.desired ? Math.min(100, (n / data.value.desired) * 100) : 0)
const shortImage = (img: string) => img.split('/').pop() ?? img
const when = (iso: string) => (iso ? age(Date.parse(iso), Date.now()) : '')
const canUndo = computed(() => !isReadOnly(props.cluster) && (data.value?.history?.length ?? 0) > 1)

function undo(rev: ops.Revision | null) {
  const label = rev ? tr('a revisão {a} ({b})', { a: rev.revision, b: rev.images.map(shortImage).join(', ') }) : tr('a versão anterior')
  confirm?.(tr('Voltar {a} para {b}? O deployment faz um novo rollout com essa versão.', { a: props.name, b: label }), async () => {
    const r = await RolloutUndo(props.cluster, props.namespace, props.name, rev?.revision ?? 0)
    toast(tr('{a}: voltando para a revisão {b}', { a: props.name, b: r }))
    await load()
  }, true)
}
</script>

<template>
  <section class="border-b border-zinc-800/70 px-4 py-3 text-xs">
    <div class="mb-2 flex items-center gap-2">
      <h3 class="text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">{{ tr('Rollout') }}</h3>
      <span v-if="data" :class="data.failed ? 'text-rose-400' : data.done ? 'text-emerald-400' : 'text-sky-300'">{{ data.message }}</span>
      <button
        v-if="canUndo" class="ml-auto rounded border border-zinc-700 px-2 py-0.5 text-zinc-300 hover:border-zinc-500 hover:text-zinc-100"
        :title="tr('Volta para a revisão anterior (como kubectl rollout undo)')" @click="undo(null)"
      >{{ tr('Voltar versão') }}</button>
    </div>
    <p v-if="error" class="text-rose-400">{{ error }}</p>
    <template v-if="data">
      <div class="relative mb-1 h-1.5 overflow-hidden rounded-full bg-zinc-800" :title="tr('{a} atualizados · {b} disponíveis · {c} desejados', { a: data.updated, b: data.available, c: data.desired })">
        <div class="absolute inset-y-0 left-0 bg-sky-500/40 transition-all" :style="{ width: `${pct(data.updated)}%` }" />
        <div class="absolute inset-y-0 left-0 transition-all" :class="data.failed ? 'bg-rose-500' : 'bg-emerald-500'" :style="{ width: `${pct(data.available)}%` }" />
      </div>
      <p class="mb-3 text-zinc-500 tabular-nums">{{ tr('{a}/{b} disponíveis · {c} na versão nova · revisão {d}', { a: data.available, b: data.desired, c: data.updated, d: data.revision }) }}</p>

      <ul v-if="data.history?.length" class="space-y-1">
        <li v-for="h in data.history" :key="h.revision" class="flex items-start gap-2 rounded px-1.5 py-1" :class="h.current && 'bg-zinc-800/60'">
          <span class="w-8 shrink-0 font-mono text-zinc-400">r{{ h.revision }}</span>
          <div class="min-w-0 flex-1">
            <div class="truncate font-mono text-zinc-200" :title="h.images.join('\n')">{{ h.images.map(shortImage).join(', ') }}</div>
            <div v-if="h.changeCause" class="truncate text-zinc-500" :title="h.changeCause">{{ h.changeCause }}</div>
          </div>
          <span class="shrink-0 text-zinc-500">{{ when(h.created) }}</span>
          <span v-if="h.current" class="shrink-0 rounded bg-emerald-500/15 px-1.5 text-emerald-300">{{ tr('atual') }}</span>
          <button v-else-if="canUndo" class="shrink-0 text-sky-400 hover:text-sky-300" @click="undo(h)">{{ tr('voltar') }}</button>
        </li>
      </ul>
    </template>
  </section>
</template>
