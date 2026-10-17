<script setup lang="ts">
/** "New version available" notice. Download opens the release page in the browser. */
import { computed, onMounted, ref } from 'vue'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { Prefs, SetUpdateCheck, SkipVersion } from '../../wailsjs/go/api/AppAPI'
import type { update } from '../../wailsjs/go/models'
import Icon from './Icon.vue'

const props = defineProps<{ info: update.Info }>()
const emit = defineEmits<{ close: [] }>()

const notify = ref(true)
onMounted(async () => (notify.value = (await Prefs().catch(() => null))?.updateCheck ?? true))
const toggleNotify = () => SetUpdateCheck(notify.value).catch(() => {})

// Release notes are markdown; show the changelog part as plain text (never as HTML).
const notes = computed(() => {
  const body = props.info.notes ?? ''
  const i = body.indexOf("## What's Changed")
  const text = (i >= 0 ? body.slice(i + "## What's Changed".length) : body)
    .replace(/\*\*Full Changelog\*\*.*$/s, '')
    .replace(/^#+\s*/gm, '')
    .replace(/\bby @[\w-]+ in https:\/\/\S+/g, '')
    .replace(/[`*]/g, '')
    .trim()
  return text.length > 1200 ? `${text.slice(0, 1200)}…` : text
})

function download() {
  BrowserOpenURL(props.info.url)
  emit('close')
}
async function skip() {
  await SkipVersion(props.info.latest).catch(() => {})
  emit('close')
}
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-[2px]" @click.self="emit('close')">
    <div class="w-[480px] max-w-[92vw] rounded-xl border border-zinc-800 bg-zinc-900 text-sm shadow-2xl">
      <div class="flex items-start gap-3 px-5 pt-5">
        <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-sky-500/15 text-sky-300"><Icon name="drain" :size="20" /></div>
        <div class="min-w-0 flex-1">
          <h2 class="font-semibold text-zinc-100">Nova versão do Leme disponível</h2>
          <p class="mt-0.5 text-xs text-zinc-400">
            <span class="font-mono text-sky-300">v{{ info.latest }}</span> · você usa
            <span class="font-mono">{{ info.current === 'dev' ? 'dev' : `v${info.current}` }}</span>
          </p>
        </div>
        <button class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" title="Fechar" @click="emit('close')">
          <Icon name="close" :size="14" />
        </button>
      </div>

      <div v-if="notes" class="mx-5 mt-4">
        <div class="mb-1.5 text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">Novidades</div>
        <pre class="max-h-52 overflow-auto rounded-lg border border-zinc-800 bg-zinc-950 p-3 font-sans text-xs leading-relaxed whitespace-pre-wrap text-zinc-300 select-text">{{ notes }}</pre>
      </div>

      <div class="mt-5 flex items-center gap-2 border-t border-zinc-800 px-5 py-3">
        <label class="flex cursor-pointer items-center gap-1.5 text-xs text-zinc-400">
          <input v-model="notify" type="checkbox" class="accent-sky-500" @change="toggleNotify" /> Avisar sobre novas versões
        </label>
        <button class="ml-auto rounded-md px-2.5 py-1.5 text-xs text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" @click="skip">Pular esta versão</button>
        <button class="rounded-md bg-zinc-800 px-3 py-1.5 text-xs text-zinc-200 hover:bg-zinc-700" @click="emit('close')">Depois</button>
        <button class="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500" @click="download">Baixar</button>
      </div>
    </div>
  </div>
</template>
