<script setup lang="ts">
/**
 * Details of an object opened from a link in another view (a pod's node, its
 * ReplicaSet, a volume's ConfigMap...), without leaving the current list.
 * Watches just that one object (fieldSelector metadata.name), so it's live and
 * costs one tiny stream.
 */
import { computed, toRef } from 'vue'
import type { resources } from '../../wailsjs/go/models'
import { useResourceWatch } from '@/composables/useResourceWatch'
import { useMetrics } from '@/composables/useMetrics'
import { isKind } from '@/lib/actions'
import type { NavigateTarget, Row, RowAction } from '@/lib/types'
import DetailsDrawer from './DetailsDrawer.vue'
import Icon from './Icon.vue'

const props = defineProps<{
  cluster: string
  kind: resources.Kind
  name: string
  namespace?: string
  back?: string
}>()
const emit = defineEmits<{
  close: []
  back: []
  open: []
  action: [action: RowAction, row: Row, kind: resources.Kind]
  navigate: [NavigateTarget]
  container: [action: 'logs' | 'previous' | 'shell', container: string, row: Row]
}>()

const ns = computed(() => (props.kind.namespaced ? props.namespace ?? '' : ''))
const query = computed<resources.Query>(() => ({
  namespace: ns.value, labelSelector: '', fieldSelector: `metadata.name=${props.name}`,
}))
const { rows, columns, loading, error } = useResourceWatch(toRef(props, 'cluster'), toRef(props, 'kind'), query)
const row = computed(() => rows.value.find((r) => r.name === props.name) ?? null)

const scope = computed<'pods' | 'nodes' | null>(() =>
  isKind(props.kind, '', 'pods') ? 'pods' : isKind(props.kind, '', 'nodes') ? 'nodes' : null)
const { usage } = useMetrics(toRef(props, 'cluster'), scope, ns)
const usageOf = (r: Row) => usage.value.get(`${r.namespace ?? ''}/${r.name}`)
</script>

<template>
  <DetailsDrawer
    v-if="row" :key="row.uid"
    :cluster="cluster" :kind="kind" :row="row" :columns="columns" :usage="usageOf(row)"
    :back="back" peek
    @close="emit('close')" @back="emit('back')" @open="emit('open')"
    @action="(a, r) => emit('action', a, r, kind)" @navigate="emit('navigate', $event)"
    @container="(a, c, r) => emit('container', a, c, r)"
  />
  <aside v-else class="flex h-full w-[480px] shrink-0 flex-col border-l border-zinc-800 bg-zinc-925">
    <header class="flex h-12 shrink-0 items-center gap-2 border-b border-zinc-800 px-4 text-[13px]">
      <button v-if="back" class="grid h-7 w-7 -ml-2 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" :title="`Voltar para ${back}`" @click="emit('back')">
        <Icon name="back" :size="15" />
      </button>
      <span class="min-w-0 flex-1 truncate text-zinc-100"><b>{{ kind.kind }}:</b> {{ name }}</span>
      <button class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" title="Fechar" @click="emit('close')"><Icon name="close" :size="14" /></button>
    </header>
    <div class="p-4 text-xs">
      <p v-if="error" class="text-rose-300">{{ error }}</p>
      <p v-else-if="loading" class="flex items-center gap-2 text-zinc-500">
        <span class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" /> carregando…
      </p>
      <p v-else class="text-zinc-500">{{ kind.kind }} "{{ name }}" não encontrado (pode ter sido removido).</p>
    </div>
  </aside>
</template>
