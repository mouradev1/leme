<script setup lang="ts">
/**
 * Add clusters: paste a kubeconfig (saved privately by the app) or point to a
 * kubeconfig file anywhere on disk.
 */
import { computed, onMounted, ref } from 'vue'
import { AddKubeconfig, KubeconfigFiles, PickKubeconfig, RemoveKubeconfig, SaveKubeconfig } from '../../wailsjs/go/api/ClusterAPI'
import type { cluster } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'
import Icon from './Icon.vue'

const emit = defineEmits<{ close: []; changed: []; connect: [context: string] }>()

const tab = ref<'paste' | 'file'>('paste')
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
      text: `${added.value.contexts.length} cluster(s) adicionado(s).` +
        (shadowed.length ? `\nJá existia um context com o nome ${shadowed.join(', ')}: vale o que já estava na lista (renomeie no YAML para manter os dois).` : ''),
    }
  })

const pick = () => run(async () => {
  const n = await PickKubeconfig()
  if (n > 0) msg.value = { ok: true, text: `${n} cluster(s) adicionado(s).` }
})
const addTyped = () => run(async () => {
  const n = await AddKubeconfig(path.value)
  path.value = ''
  msg.value = { ok: true, text: `${n} cluster(s) adicionado(s).` }
})
const remove = (p: string) => run(() => RemoveKubeconfig(p))

const looksLikeKubeconfig = computed(() => /kind:\s*Config/.test(content.value) || /clusters:/.test(content.value))
const label = (f: cluster.KubeconfigFile) => (f.managed ? f.path.split('/').pop() : f.path)
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60" @click.self="emit('close')">
    <div class="flex max-h-[90vh] w-[680px] max-w-[94vw] flex-col rounded-xl border border-zinc-800 bg-zinc-900 text-sm shadow-2xl">
      <div class="flex items-center border-b border-zinc-800 px-4 py-3">
        <h2 class="font-semibold text-zinc-100">Adicionar clusters</h2>
        <button class="ml-auto grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" title="Fechar" @click="emit('close')">
          <Icon name="close" :size="14" />
        </button>
      </div>

      <div class="flex gap-1 px-4 pt-3">
        <button class="tab" :class="tab === 'paste' && 'tab-on'" @click="tab = 'paste'; msg = null">Colar configuração</button>
        <button class="tab" :class="tab === 'file' && 'tab-on'" @click="tab = 'file'; msg = null">Arquivo</button>
      </div>

      <div class="min-h-0 overflow-auto px-4 py-3">
        <!-- Paste -->
        <template v-if="tab === 'paste'">
          <p class="mb-2 text-xs text-zinc-500">
            Cole o kubeconfig (o mesmo conteúdo do <span class="font-mono">~/.kube/config</span> ou o que o provedor de nuvem fornece).
            O app valida, salva num arquivo privado e o cluster já aparece na lista.
          </p>
          <textarea
            v-model="content" spellcheck="false" autocomplete="off"
            placeholder="apiVersion: v1&#10;kind: Config&#10;clusters:&#10;- name: ...&#10;  cluster:&#10;    server: https://...&#10;contexts:&#10;- ...&#10;users:&#10;- ..."
            class="h-56 w-full resize-y rounded-md border border-zinc-800 bg-zinc-950 p-3 font-mono text-xs leading-relaxed text-zinc-200 outline-none placeholder:text-zinc-700 focus:border-sky-600"
          />
          <div class="mt-2 flex items-center gap-2">
            <input
              v-model="name" placeholder="Nome do arquivo (opcional)"
              class="h-8 w-56 rounded-md border border-zinc-800 bg-zinc-950 px-2 text-xs outline-none focus:border-sky-600"
            />
            <span v-if="content && !looksLikeKubeconfig" class="text-[11px] text-amber-300">Isso não parece um kubeconfig.</span>
            <button
              class="ml-auto h-8 rounded-md bg-sky-600 px-4 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50"
              :disabled="busy || !content.trim()" @click="savePasted"
            >Salvar e adicionar</button>
          </div>
          <div v-if="added && msg?.ok" class="mt-3 flex flex-wrap items-center gap-1.5">
            <span class="text-xs text-zinc-500">Conectar agora:</span>
            <button
              v-for="c in added.contexts" :key="c"
              class="rounded-md border border-zinc-700 px-2 py-0.5 font-mono text-xs text-zinc-200 hover:border-sky-600 hover:text-sky-300"
              @click="emit('connect', c); emit('close')"
            >{{ c }}</button>
          </div>
        </template>

        <!-- File -->
        <template v-else>
          <div class="flex gap-2">
            <button class="h-8 rounded-md bg-sky-600 px-3 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50" :disabled="busy" @click="pick">
              Escolher arquivo…
            </button>
            <input
              v-model="path" placeholder="ou digite o caminho: ~/projetos/cliente/kubeconfig.yaml"
              class="h-8 min-w-0 flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-xs outline-none focus:border-sky-600"
              @keydown.enter="path && addTyped()"
            />
            <button class="h-8 rounded-md bg-zinc-800 px-3 text-xs hover:bg-zinc-700 disabled:opacity-50" :disabled="busy || !path" @click="addTyped">Adicionar</button>
          </div>
          <p class="mt-2 text-xs text-zinc-500">O arquivo continua onde está e é recarregado automaticamente quando muda.</p>
        </template>

        <p v-if="msg" class="mt-3 whitespace-pre-wrap text-xs" :class="msg.ok ? 'text-emerald-400' : 'text-rose-400'">{{ msg.text }}</p>

        <!-- Sources -->
        <h3 class="mt-5 mb-2 text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">Fontes de clusters</h3>
        <ul class="divide-y divide-zinc-800 overflow-hidden rounded-md border border-zinc-800">
          <li v-for="f in files" :key="f.path" class="flex items-center gap-2 px-3 py-2">
            <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="f.exists ? 'bg-emerald-400' : 'bg-rose-500'" :title="f.exists ? '' : 'arquivo não existe'" />
            <span class="min-w-0 flex-1 truncate font-mono text-xs" :title="f.path">{{ label(f) }}</span>
            <span v-if="f.managed" class="shrink-0 rounded bg-zinc-800 px-1.5 text-[10px] text-zinc-400">colado</span>
            <span class="shrink-0 text-xs text-zinc-500">{{ f.contexts }} ctx</span>
            <button
              v-if="f.removable" class="shrink-0 text-xs text-rose-400 hover:text-rose-300" :disabled="busy"
              :title="f.managed ? 'Remove o cluster e apaga o arquivo salvo pelo app' : 'Remove da lista (o arquivo não é apagado)'"
              @click="remove(f.path)"
            >remover</button>
            <span v-else class="shrink-0 text-xs text-zinc-600">padrão</span>
          </li>
        </ul>
        <p class="mt-2 text-[11px] text-zinc-600">Se dois arquivos tiverem um context com o mesmo nome, vale o primeiro da lista (igual ao kubectl).</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";
.tab { @apply rounded-md px-3 py-1.5 text-xs text-zinc-400 hover:text-zinc-100; }
.tab-on { @apply bg-zinc-800 text-zinc-100; }
</style>
