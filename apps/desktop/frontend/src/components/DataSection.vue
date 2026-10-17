<script setup lang="ts">
/**
 * Inline editor for ConfigMap / Secret data: one editor per key, add/remove
 * keys, Save with optimistic locking (resourceVersion). Secret values are
 * hidden until revealed. Binary keys are shown read-only.
 */
import { computed, ref, watch } from 'vue'
import { GetData, SetData } from '../../wailsjs/go/api/ResourceAPI'
import { errMsg } from '@/lib/bridge'
import CodeBox from './CodeBox.vue'
import Icon from './Icon.vue'

const props = defineProps<{ cluster: string; resource: 'configmaps' | 'secrets'; namespace: string; name: string; rv: string }>()
const emit = defineEmits<{ saved: [] }>()

const isSecret = computed(() => props.resource === 'secrets')
const original = ref<Record<string, string>>({})
const draft = ref<Record<string, string>>({})
const binary = ref<string[]>([])
const version = ref('')
const loading = ref(true)
const saving = ref(false)
const msg = ref<{ ok: boolean; text: string } | null>(null)
const revealed = ref(new Set<string>())
const newKey = ref('')
const staleRemote = ref(false)

async function load() {
  loading.value = true
  try {
    const d = await GetData(props.cluster, props.resource, props.namespace, props.name)
    original.value = { ...d.data }
    draft.value = { ...d.data }
    binary.value = d.binary ?? []
    version.value = d.resourceVersion
    staleRemote.value = false
    msg.value = null
  } catch (e) {
    msg.value = { ok: false, text: errMsg(e) }
  } finally {
    loading.value = false
  }
}
load()

const keys = computed(() => Object.keys(draft.value).sort())
const dirty = computed(() => JSON.stringify(sortObj(draft.value)) !== JSON.stringify(sortObj(original.value)))
const changed = (k: string) => draft.value[k] !== original.value[k]
function sortObj(o: Record<string, string>) {
  return Object.fromEntries(Object.entries(o).sort(([a], [b]) => a.localeCompare(b)))
}

// The object changed in the cluster (watch delta bumped rv).
watch(() => props.rv, (rv) => {
  if (rv === version.value) return
  if (dirty.value) staleRemote.value = true
  else void load()
})

const lang = (k: string) => (/\.(ya?ml)$/i.test(k) ? 'yaml' : 'plaintext')
const keyError = computed(() => {
  const k = newKey.value.trim()
  if (!k) return ''
  if (!/^[-._a-zA-Z0-9]+$/.test(k)) return 'Use apenas letras, números, "-", "_" e "."'
  if (k in draft.value || binary.value.includes(k)) return 'Chave já existe'
  return ''
})
function addKey() {
  const k = newKey.value.trim()
  if (!k || keyError.value) return
  draft.value = { ...draft.value, [k]: '' }
  revealed.value.add(k)
  newKey.value = ''
}
function removeKey(k: string) {
  const { [k]: _, ...rest } = draft.value
  draft.value = rest
}
function toggleReveal(k: string) {
  if (revealed.value.has(k)) revealed.value.delete(k)
  else revealed.value.add(k)
}

async function save() {
  saving.value = true
  msg.value = null
  try {
    version.value = await SetData(props.cluster, props.resource, props.namespace, props.name, draft.value, version.value)
    original.value = { ...draft.value }
    staleRemote.value = false
    msg.value = { ok: true, text: 'Salvo.' }
    emit('saved')
  } catch (e) {
    const text = errMsg(e)
    msg.value = {
      ok: false,
      text: /conflict|modified/i.test(text) ? 'O objeto foi alterado no cluster desde que você abriu. Recarregue para ver a versão atual.' : text,
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section>
    <h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-zinc-100">
      Data
      <span class="rounded-full bg-zinc-800 px-1.5 text-[10px] font-normal text-zinc-400">{{ keys.length + binary.length }}</span>
      <button class="ml-auto text-zinc-500 hover:text-zinc-200" title="Recarregar do cluster" @click="load"><Icon name="refresh" :size="13" /></button>
    </h3>

    <div v-if="staleRemote" class="mb-3 flex items-center gap-2 rounded-md border border-amber-900/60 bg-amber-950/40 px-3 py-2 text-xs text-amber-200">
      Alterado no cluster enquanto você editava.
      <button class="ml-auto underline" @click="load">Descartar e recarregar</button>
    </div>

    <p v-if="loading" class="text-xs text-zinc-500">carregando…</p>
    <p v-else-if="!keys.length && !binary.length" class="text-xs text-zinc-500">Sem dados.</p>

    <div v-for="k in keys" :key="k" class="mb-4">
      <div class="mb-1.5 flex items-center gap-2 text-xs">
        <span class="font-mono text-zinc-200">{{ k }}</span>
        <span v-if="changed(k)" class="h-1.5 w-1.5 rounded-full bg-amber-400" title="modificado" />
        <span class="ml-auto flex items-center gap-1">
          <button v-if="isSecret" class="rounded px-1.5 py-0.5 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" @click="toggleReveal(k)">
            {{ revealed.has(k) ? 'ocultar' : 'mostrar' }}
          </button>
          <button class="rounded p-1 text-zinc-500 hover:bg-rose-950 hover:text-rose-300" title="Remover chave" @click="removeKey(k)"><Icon name="close" :size="11" /></button>
        </span>
      </div>
      <CodeBox v-if="!isSecret || revealed.has(k)" v-model="draft[k]" :language="lang(k)" />
      <button v-else class="w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-left font-mono text-xs tracking-widest text-zinc-500 hover:border-zinc-700" @click="toggleReveal(k)">
        ••••••••••••  <span class="tracking-normal text-zinc-600">({{ draft[k].length }} caracteres)</span>
      </button>
    </div>

    <div v-for="k in binary" :key="`bin:${k}`" class="mb-3 flex items-center gap-2 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-xs">
      <span class="font-mono text-zinc-300">{{ k }}</span>
      <span class="ml-auto text-zinc-600">binário (somente leitura)</span>
    </div>

    <div v-if="!loading" class="mt-2 flex items-center gap-2">
      <button
        class="rounded-md bg-sky-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:bg-zinc-800 disabled:text-zinc-500"
        :disabled="!dirty || saving" @click="save"
      >{{ saving ? 'Salvando…' : 'Save' }}</button>
      <button v-if="dirty" class="text-xs text-zinc-400 hover:text-zinc-200" @click="draft = { ...original }">Descartar</button>
      <div class="ml-auto flex items-center gap-1">
        <input
          v-model="newKey" placeholder="nova-chave" class="h-7 w-36 rounded-md border bg-zinc-950 px-2 font-mono text-xs outline-none"
          :class="keyError ? 'border-rose-700' : 'border-zinc-800 focus:border-sky-600'" :title="keyError" @keydown.enter="addKey"
        />
        <button class="h-7 rounded-md border border-zinc-800 px-2 text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-40" :disabled="!newKey || !!keyError" @click="addKey">+ chave</button>
      </div>
    </div>
    <p v-if="msg" class="mt-2 text-xs" :class="msg.ok ? 'text-emerald-400' : 'text-rose-400'">{{ msg.text }}</p>
  </section>
</template>
