<script setup lang="ts">
/** Keyboard shortcuts cheat sheet (?). */
import { tr } from '@/lib/i18n'
const emit = defineEmits<{ close: [] }>()
const mod = /Mac/i.test(navigator.platform) ? '⌘' : 'Ctrl+'
const groups = [
  { title: 'Geral', keys: [
    [`${mod}K`, tr('Paleta de comandos: objetos, recursos, clusters')],
    ['?', tr('Esta lista de atalhos')],
    [`${mod}F`, tr('Buscar na lista atual')],
  ] },
  { title: tr('Abas e telas'), keys: [
    [`${mod}T`, 'Nova aba'],
    [`${mod}W`, tr('Fechar aba')],
    [`${mod}1 … ${mod}9`, tr('Ir para a aba')],
    [`${mod}⇧] / ${mod}⇧[`, tr('Próxima / anterior')],
    [`${mod}\\`, 'Dividir / juntar a tela'],
  ] },
  { title: tr('Na lista'), keys: [
    [tr('↑ ↓  ou  j k'), tr('Selecionar linha (abre os detalhes)')],
    ['/', tr('Buscar')],
    ['l', tr('Logs')],
    ['s', tr('Shell')],
    ['e', 'Editar YAML'],
    ['f', 'Port-forward'],
    ['Delete', tr('Excluir (pede confirmação)')],
    ['Esc', tr('Fechar detalhes')],
  ] },
  { title: 'Terminal', keys: [
    [`${mod === '⌘' ? '⌘C' : 'Ctrl+Shift+C'}`, tr('Copiar seleção')],
    [`${mod === '⌘' ? '⌘V' : 'Ctrl+Shift+V'}`, 'Colar'],
    [`${mod === '⌘' ? '⌥ + arrastar' : 'Shift + arrastar'}`, tr('Selecionar mesmo em vim/htop')],
    [tr('Clique direito'), tr('Copiar, colar, limpar')],
  ] },
]
</script>

<template>
  <div class="fixed inset-0 z-[70] grid place-items-center bg-black/60" @click.self="emit('close')" @keydown.esc="emit('close')">
    <div class="max-h-[86vh] w-[720px] max-w-[94vw] overflow-auto rounded-xl border border-zinc-800 bg-zinc-900 p-5 text-sm shadow-2xl">
      <div class="mb-4 flex items-center">
        <h2 class="font-semibold text-zinc-100">{{ tr('Atalhos de teclado') }}</h2>
        <button class="ml-auto rounded px-2 py-1 text-xs text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" @click="emit('close')">{{ tr('Fechar') }}</button>
      </div>
      <div class="grid gap-x-8 gap-y-5 sm:grid-cols-2">
        <section v-for="g in groups" :key="g.title">
          <h3 class="mb-2 text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">{{ g.title }}</h3>
          <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
            <template v-for="[k, d] in g.keys" :key="k">
              <dt><kbd class="rounded border border-zinc-700 bg-zinc-800 px-1.5 py-0.5 font-mono text-[11px] whitespace-nowrap text-zinc-200">{{ k }}</kbd></dt>
              <dd class="text-zinc-400">{{ d }}</dd>
            </template>
          </dl>
        </section>
      </div>
    </div>
  </div>
</template>
