<script setup lang="ts">
/** Compact Monaco editor that grows with its content (3–18 lines). */
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import type * as Monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { loadMonaco } from '@/lib/monaco'
import { monacoTheme } from '@/lib/theme'

const props = defineProps<{ language?: string; readonly?: boolean }>()
const model = defineModel<string>({ required: true })

const host = ref<HTMLElement | null>(null)
const height = ref(64)
const editor = shallowRef<Monaco.editor.IStandaloneCodeEditor | null>(null)
const LINE = 19

function fit() {
  const lines = editor.value?.getModel()?.getLineCount() ?? 1
  height.value = Math.min(Math.max(lines, 3), 18) * LINE + 10
}

onMounted(async () => {
  const monaco = await loadMonaco()
  if (!host.value) return
  editor.value = monaco.editor.create(host.value, {
    value: model.value,
    language: props.language ?? 'plaintext',
    theme: monacoTheme(),
    readOnly: props.readonly,
    automaticLayout: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    lineNumbersMinChars: 3,
    fontSize: 12,
    lineHeight: LINE,
    renderLineHighlight: 'none',
    overviewRulerLanes: 0,
    scrollbar: { alwaysConsumeMouseWheel: false, verticalScrollbarSize: 8 },
    padding: { top: 5, bottom: 5 },
    wordWrap: 'on',
  })
  editor.value.onDidChangeModelContent(() => {
    model.value = editor.value!.getValue()
    fit()
  })
  fit()
})

// External updates (reload from cluster) without clobbering the cursor on typing.
watch(model, (v) => {
  if (editor.value && editor.value.getValue() !== v) {
    editor.value.setValue(v)
    fit()
  }
})

onBeforeUnmount(() => {
  editor.value?.getModel()?.dispose()
  editor.value?.dispose()
})
</script>

<template>
  <div ref="host" class="overflow-hidden rounded-md border border-zinc-800 bg-zinc-950" :style="{ height: `${height}px` }" />
</template>
