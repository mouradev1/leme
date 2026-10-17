<script setup lang="ts">
/**
 * YAML editor with server-side validation and diff.
 *
 *  Diff  -> Apply(dryRun) returns the object exactly as the API server would
 *           persist it (defaults, webhooks, strict field validation) and shows
 *           it against the live version in Monaco's DiffEditor.
 *  Apply -> edit semantics (like `kubectl edit`): what's in the editor is
 *           what gets saved, including removed fields. resourceVersion in the
 *           buffer is an optimistic lock: a stale buffer fails, never silently
 *           overwrites. Warns when a deploy tool also manages the object.
 */
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import type * as Monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { Apply, GetYAML } from '../../wailsjs/go/api/ResourceAPI'
import type { resources } from '../../wailsjs/go/models'
import { loadMonaco, type MonacoApi } from '@/lib/monaco'
import { monacoTheme } from '@/lib/theme'
import { errMsg } from '@/lib/bridge'
import Icon from './Icon.vue'

const props = defineProps<{
  cluster: string
  kind?: resources.Ref // omit (with initialYaml) to create a new resource
  namespace?: string
  name?: string
  initialYaml?: string
}>()
const emit = defineEmits<{ close: []; applied: [yaml: string] }>()

const host = ref<HTMLElement | null>(null)
const mode = ref<'edit' | 'diff'>('edit')
const busy = ref(false)
const showKeys = ref(false)
const dirty = ref(false) // buffer differs from what was last loaded/applied
const confirmClose = ref(false)

function requestClose() {
  if (dirty.value) confirmClose.value = true
  else emit('close')
}
const message = ref<{ kind: 'ok' | 'err' | 'warn'; text: string } | null>(null)

let monaco: MonacoApi
const editor = shallowRef<Monaco.editor.IStandaloneCodeEditor | null>(null)
const diff = shallowRef<Monaco.editor.IStandaloneDiffEditor | null>(null)
let model: Monaco.editor.ITextModel
let live = '' // last version fetched from the cluster

const KEYS: [string, string][] = [
  ['⌘F', 'Buscar'],
  ['⌥⌘F', 'Buscar e substituir'],
  ['⌘G / ⇧⌘G', 'Próxima / anterior'],
  ['⌘D', 'Adicionar próxima ocorrência ao cursor'],
  ['⇧⌘L', 'Selecionar todas as ocorrências'],
  ['⌥ + clique', 'Cursor extra'],
  ['⌥⇧ + arrastar', 'Seleção em coluna'],
  ['⌘/', 'Comentar / descomentar linha'],
  ['⌥↑ / ⌥↓', 'Mover linha'],
  ['⇧⌥↓', 'Duplicar linha'],
  ['⇧⌘K', 'Apagar linha'],
  ['⌥⌘[ / ⌥⌘]', 'Recolher / expandir bloco'],
  ['⌘S', 'Aplicar no cluster'],
]

const common: Monaco.editor.IEditorOptions = {
  automaticLayout: true, // ResizeObserver-based; the drawer is resizable
  minimap: { enabled: false },
  fontSize: 13,
  scrollBeyondLastLine: false,
  renderWhitespace: 'boundary',
  folding: true,
  showFoldingControls: 'mouseover',
  find: { addExtraSpaceOnTop: false, seedSearchStringFromSelection: 'selection', autoFindInSelection: 'multiline' },
}

// The editor the user is typing in (the modified side when diffing).
function active(): Monaco.editor.ICodeEditor | null {
  return editor.value ?? diff.value?.getModifiedEditor() ?? null
}
function run(actionId: string) {
  const ed = active()
  if (!ed) return
  ed.focus()
  void ed.getAction(actionId)?.run()
}

async function fetchLive() {
  if (!props.kind || !props.name) return props.initialYaml ?? ''
  return GetYAML(props.cluster, props.kind, props.namespace ?? '', props.name)
}

onMounted(async () => {
  monaco = await loadMonaco()
  live = await fetchLive().catch((e) => {
    message.value = { kind: 'err', text: errMsg(e) }
    return ''
  })
  model = monaco.editor.createModel(live, 'yaml')
  model.updateOptions({ tabSize: 2, insertSpaces: true })
  model.onDidChangeContent(() => {
    dirty.value = model.getValue() !== live
    if (!dirty.value) confirmClose.value = false
  })
  showEditor()
})

function disposeViews() {
  editor.value?.dispose()
  diff.value?.dispose()
  editor.value = diff.value = null
}

function showEditor() {
  disposeViews()
  mode.value = 'edit'
  editor.value = monaco.editor.create(host.value!, { ...common, model, theme: monacoTheme() })
  editor.value.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => void apply())
}

async function showDiff() {
  busy.value = true
  message.value = null
  try {
    const { yaml: result, warnings } = await Apply(props.cluster, model.getValue(), { dryRun: true })
    disposeViews()
    mode.value = 'diff'
    const d = monaco.editor.createDiffEditor(host.value!, {
      ...common, theme: monacoTheme(), readOnly: true, renderSideBySide: true,
    })
    d.setModel({
      original: monaco.editor.createModel(live, 'yaml'),
      modified: monaco.editor.createModel(result, 'yaml'),
    })
    diff.value = d
    message.value = warnings?.length
      ? { kind: 'warn', text: `Dry-run OK: à direita está exatamente o que o servidor gravaria.\n${warnings.join('\n')}` }
      : { kind: 'ok', text: 'Dry-run OK: à direita está exatamente o que o servidor gravaria.' }
  } catch (e) {
    message.value = { kind: 'err', text: errMsg(e) }
  } finally {
    busy.value = false
  }
}

async function apply() {
  busy.value = true
  message.value = null
  try {
    const { yaml: result, warnings } = await Apply(props.cluster, model.getValue(), { dryRun: false })
    live = result
    model.setValue(result) // fresh resourceVersion for the next save
    if (mode.value === 'diff') showEditor()
    message.value = warnings?.length
      ? { kind: 'warn', text: `Aplicado.\n${warnings.join('\n')}` }
      : { kind: 'ok', text: 'Aplicado.' }
    emit('applied', result)
  } catch (e) {
    message.value = { kind: 'err', text: errMsg(e) }
  } finally {
    busy.value = false
  }
}

async function reload() {
  live = await fetchLive()
  model.setValue(live)
  if (mode.value === 'diff') showEditor()
}

onBeforeUnmount(() => {
  if (diff.value) {
    const m = diff.value.getModel()
    m?.original.dispose()
    m?.modified.dispose()
  }
  disposeViews()
  model?.dispose()
})
</script>

<template>
  <div class="flex h-full flex-col bg-zinc-950">
    <!-- Row 1: identity + the two actions that must always be reachable -->
    <div class="flex h-11 shrink-0 items-center gap-2 border-b border-zinc-800/70 pr-2 pl-4">
      <div class="min-w-0 flex-1 truncate text-[13px]" :title="name ?? 'Novo recurso'">
        <span class="text-zinc-100">{{ name ?? 'Novo recurso' }}</span>
        <span v-if="namespace" class="ml-2 text-xs text-zinc-500">{{ namespace }}</span>
      </div>
      <span v-if="dirty" class="flex shrink-0 items-center gap-1.5 text-[11px] text-amber-300">
        <span class="h-1.5 w-1.5 rounded-full bg-amber-400" /> não salvo
      </span>
      <button class="shrink-0 rounded-md bg-sky-600 px-3 py-1 text-xs font-medium whitespace-nowrap text-white hover:bg-sky-500 disabled:opacity-50" :disabled="busy" title="Aplicar no cluster (⌘S)" @click="apply">
        Aplicar
      </button>
      <button class="tool shrink-0" title="Fechar editor" @click="requestClose"><Icon name="close" :size="15" /></button>
    </div>

    <!-- Row 2: tools (icons with tooltips; never wraps) -->
    <div class="flex h-9 shrink-0 items-center gap-0.5 overflow-x-auto border-b border-zinc-800 px-2 text-xs whitespace-nowrap">
      <button class="tool" title="Buscar (⌘F)" @click="run('actions.find')"><Icon name="search" :size="15" /></button>
      <button class="tool" title="Buscar e substituir (⌥⌘F)" @click="run('editor.action.startFindReplaceAction')"><Icon name="replace" :size="15" /></button>
      <button class="tool" title="Editar todas as ocorrências da seleção de uma vez (⇧⌘L)" @click="run('editor.action.selectHighlights')"><Icon name="cursors" :size="15" /></button>
      <div class="relative">
        <button class="tool" :class="showKeys && 'bg-zinc-800 text-zinc-100'" title="Atalhos do editor" @click="showKeys = !showKeys"><Icon name="help" :size="15" /></button>
        <div v-if="showKeys" class="fixed z-50 mt-1 w-72 rounded-lg border border-zinc-800 bg-zinc-900 p-3 text-[11px] text-zinc-300 shadow-2xl shadow-black/60" @mouseleave="showKeys = false">
          <div v-for="[k, d] in KEYS" :key="k" class="flex justify-between gap-3 py-0.5">
            <span class="text-zinc-400">{{ d }}</span><kbd class="font-mono text-zinc-200">{{ k }}</kbd>
          </div>
        </div>
      </div>
      <button class="tool" title="Recarregar do cluster (descarta alterações)" :disabled="busy" @click="reload"><Icon name="refresh" :size="15" /></button>

      <span class="mx-1.5 h-4 w-px shrink-0 bg-zinc-800" />
      <div class="flex shrink-0 rounded-md border border-zinc-800 p-0.5">
        <button class="seg" :class="mode === 'edit' && 'seg-on'" :disabled="busy" @click="mode !== 'edit' && showEditor()">Editar</button>
        <button class="seg" :class="mode === 'diff' && 'seg-on'" :disabled="busy" title="Comparar com o que o servidor gravaria (dry-run)" @click="mode !== 'diff' && showDiff()">Diff</button>
      </div>

    </div>

    <!-- Unsaved-changes guard -->
    <div v-if="confirmClose" class="flex items-center gap-3 border-b border-amber-900/60 bg-amber-950/40 px-4 py-2 text-xs text-amber-200">
      Há alterações não aplicadas.
      <button class="ml-auto rounded-md border border-amber-800 px-2 py-0.5 hover:bg-amber-900/40" @click="emit('close')">Descartar e fechar</button>
      <button class="rounded-md px-2 py-0.5 text-zinc-300 hover:bg-zinc-800" @click="confirmClose = false">Continuar editando</button>
    </div>
    <div
      v-if="message"
      class="whitespace-pre-wrap px-3 py-1.5 text-xs"
      :class="message.kind === 'err' ? 'bg-rose-950 text-rose-300' : message.kind === 'warn' ? 'bg-amber-950/60 text-amber-200' : 'bg-emerald-950 text-emerald-300'"
    >{{ message.text }}</div>
    <div ref="host" class="min-h-0 flex-1" />
  </div>
</template>

<style scoped>
@reference "../style.css";
.tool { @apply grid h-7 w-7 shrink-0 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100 disabled:opacity-40; }
.seg { @apply rounded px-2.5 py-0.5 text-zinc-400 hover:text-zinc-100 disabled:opacity-40; }
.seg-on { @apply bg-zinc-800 text-zinc-100; }
</style>
