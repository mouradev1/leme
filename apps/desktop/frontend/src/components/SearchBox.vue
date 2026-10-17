<script setup lang="ts">
/** Search with case/regex toggles. ⌘F / Ctrl+F focuses, Esc clears. */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'

defineProps<{ placeholder?: string; error?: string }>()
const query = defineModel<string>({ required: true })
const caseSensitive = defineModel<boolean>('caseSensitive', { default: false })
const regex = defineModel<boolean>('regex', { default: false })

const input = ref<HTMLInputElement | null>(null)
function onKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
    // ⌘F inside the YAML editor/terminal belongs to them (find in document).
    if ((e.target as HTMLElement | null)?.closest?.('.monaco-editor, .xterm')) return
    e.preventDefault()
    input.value?.focus()
    input.value?.select()
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div
    class="flex h-8 min-w-0 flex-1 items-center gap-1 rounded-md border bg-zinc-900 pr-1 pl-2.5 focus-within:border-sky-600"
    :class="error ? 'border-rose-700' : 'border-zinc-800'"
    :title="error || 'Termos separados por espaço = E. Prefixo ! exclui. Busca em nome, namespace, colunas e labels (app=nginx).'"
  >
    <svg class="h-3.5 w-3.5 shrink-0 text-zinc-500" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6">
      <circle cx="7" cy="7" r="4.5" /><path d="m10.5 10.5 3 3" />
    </svg>
    <input
      ref="input" v-model="query" :placeholder="placeholder ?? 'Buscar…'"
      class="min-w-0 flex-1 bg-transparent px-1 text-xs outline-none placeholder:text-zinc-600"
      spellcheck="false" @keydown.esc="query = ''"
    />
    <button v-if="query" class="px-1 text-zinc-500 hover:text-zinc-200" title="Limpar (Esc)" @click="query = ''"><Icon name="close" :size="12" /></button>
    <button
      class="rounded px-1.5 py-0.5 font-mono text-[11px]"
      :class="caseSensitive ? 'bg-sky-600/30 text-sky-300' : 'text-zinc-500 hover:text-zinc-200'"
      title="Diferenciar maiúsculas/minúsculas" @click="caseSensitive = !caseSensitive"
    >Aa</button>
    <button
      class="rounded px-1.5 py-0.5 font-mono text-[11px]"
      :class="regex ? 'bg-sky-600/30 text-sky-300' : 'text-zinc-500 hover:text-zinc-200'"
      title="Expressão regular" @click="regex = !regex"
    >.*</button>
    <kbd class="hidden rounded border border-zinc-700 px-1 font-sans text-[10px] text-zinc-500 lg:block">⌘F</kbd>
  </div>
</template>
