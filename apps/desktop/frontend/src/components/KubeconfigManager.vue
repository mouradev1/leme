<script setup lang="ts">
/**
 * Add clusters: paste a kubeconfig (saved privately by the app) or point to a
 * kubeconfig file anywhere on disk.
 */
import { tr } from '@/lib/i18n'
import { computed, onMounted, ref } from 'vue'
import { AddKubeconfig, KubeconfigFiles, PickKubeconfig, RemoveKubeconfig, RevealKubeconfig, SaveKubeconfig } from '../../wailsjs/go/api/ClusterAPI'
import type { cluster } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'
import Icon from './Icon.vue'
import RancherPanel from './RancherPanel.vue'

const props = defineProps<{ rancher?: string }>() // open on the Rancher tab, API key field of this server
const emit = defineEmits<{ close: []; changed: []; connect: [context: string] }>()

const tab = ref<'paste' | 'file' | 'rancher'>(props.rancher ? 'rancher' : 'paste')
const files = ref<cluster.KubeconfigFile[]>([])
const busy = ref(false)
const msg = ref<{ ok: boolean; text: string } | null>(null)

// paste
const content = ref('')
const name = ref('')
const added = ref<cluster.SavedKubeconfig | null>(null)

// file
const path = ref('')

async function refresh() {
  files.value = await KubeconfigFiles()
}
onMounted(refresh)

async function run(fn: () => Promise<void>) {
  busy.value = true
  msg.value = null
  try {
    await fn()
    await refresh()
    emit('changed')
  } catch (e) {
    msg.value = { ok: false, text: errMsg(e) }
    await refresh() // a multi-select may have partially succeeded
  } finally {
    busy.value = false
  }
}

const savePasted = () =>
  run(async () => {
    added.value = await SaveKubeconfig(name.value, content.value)
    content.value = ''
    name.value = ''
    const shadowed = added.value.shadowed ?? []
    msg.value = {
      ok: true,
      text: tr('{a} cluster(s) adicionado(s).', { a: added.value.contexts.length }) +
        (shadowed.length ? tr('\nJá existia um context com o nome {a}: vale o que já estava na lista (renomeie no YAML para manter os dois).', { a: shadowed.join(', ') }) : ''),
    }
  })

const pick = () => run(async () => {
  const n = await PickKubeconfig()
  if (n > 0) msg.value = { ok: true, text: tr('{a} cluster(s) adicionado(s).', { a: n }) }
})
const addTyped = () => run(async () => {
  const n = await AddKubeconfig(path.value)
  path.value = ''
  msg.value = { ok: true, text: tr('{a} cluster(s) adicionado(s).', { a: n }) }
})
const remove = (p: string) => run(() => RemoveKubeconfig(p))

const looksLikeKubeconfig = computed(() => /kind:\s*Config/.test(content.value) || /clusters:/.test(content.value))
// Sources: cluster names first, the file path as a quiet second line.
const filter = ref('')
const totalClusters = computed(() => files.value.reduce((n, f) => n + f.names.length, 0))
const shownFiles = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return files.value
  return files.value.filter((f) => f.path.toLowerCase().includes(q) || f.names.some((n) => n.toLowerCase().includes(q)))
})
const sep = (p: string) => (p.includes('\\') && !p.includes('/') ? '\\' : '/')
function shortPath(f: cluster.KubeconfigFile) {
  const s = sep(f.path)
  const name = f.path.split(s).pop() ?? f.path
  if (f.managed) return tr('colado no Leme · {a}', { a: name })
  const p = f.path.replace(/^(\/Users\/[^/]+|\/home\/[^/]+|[A-Za-z]:\\Users\\[^\\]+)/, '~')
  const parts = p.split(s)
  return parts.length > 4 && p.length > 56 ? [parts[0], '…', ...parts.slice(-2)].join(s) : p
}
// Files outside the app that add no cluster (all their names come from an
// earlier file, or none at all). Removing only takes them off the list.
const redundant = computed(() => files.value.filter((f) => f.removable && !f.managed && f.exists && !f.names.length))
const cleanArmed = ref(false)
async function cleanRedundant() {
  if (!cleanArmed.value) {
    cleanArmed.value = true
    window.setTimeout(() => (cleanArmed.value = false), 4000)
    return
  }
  cleanArmed.value = false
  const list = redundant.value.map((f) => f.path)
  await run(async () => {
    for (const p of list) await RemoveKubeconfig(p)
    msg.value = { ok: true, text: tr('{a} arquivo(s) tirado(s) da lista. Nenhum arquivo foi apagado do disco.', { a: list.length }) }
  })
}
const fileManager = /Mac/i.test(navigator.platform) ? 'Finder' : /Win/i.test(navigator.platform) ? 'Explorer' : tr('gerenciador de arquivos')
async function reveal(path: string) {
  try { await RevealKubeconfig(path) } catch (e) { msg.value = { ok: false, text: errMsg(e) } }
}
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60" @click.self="emit('close')">
    <div class="flex max-h-[90vh] w-[680px] max-w-[94vw] flex-col rounded-xl border border-zinc-800 bg-zinc-900 text-sm shadow-2xl">
      <div class="flex items-center border-b border-zinc-800 px-4 py-3">
        <h2 class="font-semibold text-zinc-100">{{ tr('Adicionar clusters') }}</h2>
        <button class="ml-auto grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="tr('Fechar')" @click="emit('close')">
          <Icon name="close" :size="14" />
        </button>
      </div>

      <div class="flex gap-1 px-4 pt-3">
        <button class="tab" :class="tab === 'paste' && 'tab-on'" @click="tab = 'paste'; msg = null">{{ tr('Colar configuração') }}</button>
        <button class="tab" :class="tab === 'file' && 'tab-on'" @click="tab = 'file'; msg = null">{{ tr('Arquivo') }}</button>
        <button class="tab" :class="tab === 'rancher' && 'tab-on'" @click="tab = 'rancher'; msg = null">{{ tr('Rancher') }}</button>
      </div>

      <div class="min-h-0 overflow-auto px-4 py-3">
        <!-- Paste -->
        <template v-if="tab === 'paste'">
          <p class="mb-2 text-xs text-zinc-500">
            {{ tr('Cole o kubeconfig (o mesmo conteúdo do') }} <span class="font-mono">{{ tr('~/.kube/config') }}</span> {{ tr('ou o que o provedor de nuvem fornece). O app valida, salva num arquivo privado e o cluster já aparece na lista.') }}
          </p>
          <textarea
            v-model="content" spellcheck="false" autocomplete="off"
            placeholder="apiVersion: v1&#10;kind: Config&#10;clusters:&#10;- name: ...&#10;  cluster:&#10;    server: https://...&#10;contexts:&#10;- ...&#10;users:&#10;- ..."
            class="h-56 w-full resize-y rounded-md border border-zinc-800 bg-zinc-950 p-3 font-mono text-xs leading-relaxed text-zinc-200 outline-none placeholder:text-zinc-700 focus:border-sky-600"
          />
          <div class="mt-2 flex items-center gap-2">
            <input
              v-model="name" :placeholder="tr('Nome do arquivo (opcional)')"
              class="h-8 w-56 rounded-md border border-zinc-800 bg-zinc-950 px-2 text-xs outline-none focus:border-sky-600"
            />
            <span v-if="content && !looksLikeKubeconfig" class="text-[11px] text-amber-300">{{ tr('Isso não parece um kubeconfig.') }}</span>
            <button
              class="ml-auto h-8 rounded-md bg-sky-600 px-4 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50"
              :disabled="busy || !content.trim()" @click="savePasted"
            >{{ tr('Salvar e adicionar') }}</button>
          </div>
          <div v-if="added && msg?.ok" class="mt-3 flex flex-wrap items-center gap-1.5">
            <span class="text-xs text-zinc-500">{{ tr('Conectar agora:') }}</span>
            <button
              v-for="c in added.contexts" :key="c"
              class="rounded-md border border-zinc-700 px-2 py-0.5 font-mono text-xs text-zinc-200 hover:border-sky-600 hover:text-sky-300"
              @click="emit('connect', c); emit('close')"
            >{{ c }}</button>
          </div>
        </template>

        <!-- Rancher -->
        <RancherPanel v-else-if="tab === 'rancher'" :focus="props.rancher" @connect="(c) => { emit('connect', c); emit('close') }" />

        <!-- File -->
        <template v-else>
          <div class="flex gap-2">
            <button class="h-8 rounded-md bg-sky-600 px-3 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50" :disabled="busy" @click="pick">
              {{ tr('Escolher arquivo…') }}
            </button>
            <input
              v-model="path" :placeholder="tr('ou digite o caminho: ~/projetos/cliente/kubeconfig.yaml')"
              class="h-8 min-w-0 flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-xs outline-none focus:border-sky-600"
              @keydown.enter="path && addTyped()"
            />
            <button class="h-8 rounded-md bg-zinc-800 px-3 text-xs hover:bg-zinc-700 disabled:opacity-50" :disabled="busy || !path" @click="addTyped">{{ tr('Adicionar') }}</button>
          </div>
          <p class="mt-2 text-xs text-zinc-500">{{ tr('O arquivo continua onde está e é recarregado automaticamente quando muda.') }}</p>
        </template>

        <p v-if="msg" class="mt-3 whitespace-pre-wrap text-xs" :class="msg.ok ? 'text-emerald-400' : 'text-rose-400'">{{ msg.text }}</p>

        <!-- Sources -->
        <div class="mt-5 mb-2 flex items-center gap-2">
          <h3 class="text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">{{ tr('Fontes de clusters') }}</h3>
          <span class="text-[11px] text-zinc-600">{{ tr('{a} arquivo(s) · {b} cluster(s)', { a: files.length, b: totalClusters }) }}</span>
          <input
            v-if="files.length > 6" v-model="filter" :placeholder="tr('Buscar cluster ou arquivo…')"
            class="ml-auto h-7 w-52 rounded-md border border-zinc-800 bg-zinc-950 px-2 text-xs outline-none focus:border-sky-600"
          />
        </div>
        <div v-if="redundant.length" class="mb-2 flex items-center gap-2 rounded-md border border-zinc-800 bg-zinc-950/60 px-3 py-2 text-xs text-zinc-400">
          <span>{{ tr('{n} arquivo(s) não acrescentam nenhum cluster.', { n: redundant.length }) }}</span>
          <button
            class="ml-auto rounded px-2 py-0.5" :class="cleanArmed ? 'bg-rose-600 text-white' : 'bg-zinc-800 text-zinc-200 hover:bg-zinc-700'"
            :disabled="busy" :title="tr('Só tira da lista; os arquivos continuam no disco')" @click="cleanRedundant"
          >{{ cleanArmed ? tr('Clique de novo para confirmar') : tr('Tirar da lista') }}</button>
        </div>
        <ul class="divide-y divide-zinc-800 overflow-hidden rounded-md border border-zinc-800">
          <li v-for="f in shownFiles" :key="f.path" class="flex items-start gap-2.5 px-3 py-2.5">
            <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full" :class="f.exists ? (f.names.length ? 'bg-emerald-400' : 'bg-zinc-600') : 'bg-rose-500'" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-1">
                <button
                  v-for="n in f.names.slice(0, 8)" :key="n"
                  class="max-w-64 truncate rounded border border-zinc-700 px-1.5 py-px text-xs text-zinc-100 hover:border-sky-600 hover:text-sky-300"
                  :title="tr('Conectar em {a}', { a: n })" @click="emit('connect', n); emit('close')"
                >{{ n }}</button>
                <span v-if="f.names.length > 8" class="text-xs text-zinc-500">+{{ f.names.length - 8 }}</span>
                <span v-if="!f.exists" class="text-xs text-rose-400">{{ tr('arquivo não encontrado') }}</span>
                <span v-else-if="!f.names.length && f.shadowed.length" class="text-xs text-zinc-500" :title="tr('Mesmo nome já usado por um arquivo acima: {a}', { a: f.shadowed.join(', ') })">
                  nenhum cluster novo: {{ f.shadowed.join(', ') }} já vem de outro arquivo
                </span>
                <span v-else-if="!f.names.length" class="text-xs text-zinc-500">{{ tr('arquivo sem clusters') }}</span>
                <span v-if="f.names.length && f.shadowed.length" class="text-[11px] text-zinc-500" :title="f.shadowed.join(', ')">· {{ tr('{n} repetido(s)', { n: f.shadowed.length }) }}</span>
              </div>
              <div class="mt-1 truncate font-mono text-[11px] text-zinc-500" :title="f.path">{{ shortPath(f) }}</div>
            </div>
            <span v-if="f.managed" class="mt-0.5 shrink-0 rounded bg-zinc-800 px-1.5 text-[10px] text-zinc-400">{{ tr('colado') }}</span>
            <span v-else-if="!f.removable" class="mt-0.5 shrink-0 rounded bg-zinc-800 px-1.5 text-[10px] text-zinc-400">{{ tr('padrão') }}</span>
            <button
              v-if="f.exists" class="grid h-6 w-6 shrink-0 place-items-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-100"
              :title="tr('Mostrar no {a}', { a: fileManager })" @click="reveal(f.path)"
            ><Icon name="folder" :size="13" /></button>
            <button
              v-if="f.removable" class="mt-0.5 shrink-0 text-xs text-rose-400 hover:text-rose-300" :disabled="busy"
              :title="f.managed ? tr('Remove o cluster e apaga o arquivo salvo pelo app') : tr('Remove da lista (o arquivo não é apagado)')"
              @click="remove(f.path)"
            >{{ tr('remover') }}</button>
          </li>
          <li v-if="!shownFiles.length" class="px-3 py-3 text-xs text-zinc-500">{{ tr('Nada encontrado para "{q}".', { q: filter }) }}</li>
        </ul>
        <p class="mt-2 text-[11px] text-zinc-600">{{ tr('Clique num cluster para conectar. Se dois arquivos tiverem um cluster com o mesmo nome, vale o primeiro da lista (igual ao kubectl).') }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";
.tab { @apply rounded-md px-3 py-1.5 text-xs text-zinc-400 hover:text-zinc-100; }
.tab-on { @apply bg-zinc-800 text-zinc-100; }
</style>
