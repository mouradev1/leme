<script setup lang="ts">
/** Short name and icon color of a cluster (shown only in Leme; the kubeconfig is not changed). */
import { computed, ref } from 'vue'
import { errMsg } from '@/lib/bridge'
import { hue, initials } from '@/lib/dom'
import { clusterPrefs, setPrefs } from '@/lib/guard'
import { tr } from '@/lib/i18n'

const props = defineProps<{ cluster: string }>()
const emit = defineEmits<{ close: [] }>()

const alias = ref(clusterPrefs.value[props.cluster]?.alias ?? '')
const color = ref(clusterPrefs.value[props.cluster]?.color ?? '')
const error = ref('')
const HUES = ['210', '190', '160', '120', '45', '25', '0', '330', '280', '250']

const shown = computed(() => alias.value.trim() || props.cluster)
const bg = (h: string) => `hsl(${h === '' ? hue(props.cluster) : Number(h)} 55% 42%)`

async function save() {
  try {
    await setPrefs(props.cluster, { alias: alias.value.trim(), color: color.value })
    emit('close')
  } catch (e) {
    error.value = errMsg(e)
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60" @click.self="emit('close')">
    <div class="w-[420px] max-w-[94vw] rounded-xl border border-zinc-800 bg-zinc-900 p-5 text-sm shadow-2xl">
      <h2 class="font-semibold text-zinc-100">{{ tr('Nome e cor do cluster') }}</h2>
      <p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500" :title="cluster">{{ cluster }}</p>

      <div class="mt-4 flex items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-950/60 p-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg text-xs font-bold text-white" :style="{ background: bg(color) }">{{ initials(shown) }}</span>
        <span class="min-w-0 truncate text-zinc-100">{{ shown }}</span>
      </div>

      <label class="mt-4 grid gap-1 text-xs text-zinc-400">{{ tr('Nome curto (só no Leme)') }}
        <input
          v-model="alias" maxlength="40" autofocus spellcheck="false" :placeholder="cluster"
          class="h-8 rounded-md border border-zinc-800 bg-zinc-950 px-2 text-sm text-zinc-100 outline-none focus:border-sky-600"
          @keydown.enter="save" @keydown.esc="emit('close')"
        />
      </label>

      <div class="mt-4 text-xs text-zinc-400">{{ tr('Cor') }}</div>
      <div class="mt-1.5 flex flex-wrap gap-1.5">
        <button
          class="grid h-7 place-items-center rounded-md border px-2 text-[11px]"
          :class="color === '' ? 'border-sky-500 text-zinc-100' : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'"
          @click="color = ''"
        >{{ tr('automática') }}</button>
        <button
          v-for="h in HUES" :key="h" class="h-7 w-7 rounded-md border-2"
          :class="color === h ? 'border-zinc-100' : 'border-transparent hover:border-zinc-500'"
          :style="{ background: bg(h) }" :title="h" @click="color = h"
        />
      </div>

      <p v-if="error" class="mt-3 text-xs text-rose-400">{{ error }}</p>
      <p class="mt-4 text-[11px] text-zinc-500">{{ tr('O kubeconfig não é alterado. Os atalhos e a paleta continuam aceitando o nome original.') }}</p>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-md bg-zinc-800 px-3 py-1.5 text-xs hover:bg-zinc-700" @click="emit('close')">{{ tr('Cancelar') }}</button>
        <button class="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500" @click="save">{{ tr('Salvar') }}</button>
      </div>
    </div>
  </div>
</template>
