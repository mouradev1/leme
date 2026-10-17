<script setup lang="ts">
/**
 * Generic, virtualized resource list for ANY kind (built-in or CRD).
 * Columns come from the API server's Table printer (same as `kubectl get`);
 * we enrich them client-side: live metrics, owner links, container squares,
 * status colors, live age. Only visible rows hit the DOM.
 */
import { computed, h, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue'
import {
  FlexRender, getCoreRowModel, getSortedRowModel, useVueTable,
  type ColumnDef, type RowSelectionState, type SortingFn, type SortingState, type VisibilityState,
} from '@tanstack/vue-table'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { storeToRefs } from 'pinia'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import type { resources } from '../../wailsjs/go/models'
import { useResourceWatch } from '@/composables/useResourceWatch'
import { useMetrics } from '@/composables/useMetrics'
import { useClusters } from '@/stores/clusters'
import type { Column, NavigateTarget, Row, RowAction } from '@/lib/types'
import { age, bytes, cell as fmt, cpu } from '@/lib/format'
import { makeMatcher } from '@/lib/search'
import { STATUS_COLUMNS, statusTone, toneClass } from '@/lib/status'
import { actionLabel, actionsFor, dangerous, isKind } from '@/lib/actions'
import NamespaceSelect from './NamespaceSelect.vue'
import SearchBox from './SearchBox.vue'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'
import DetailsDrawer from './DetailsDrawer.vue'

const props = defineProps<{ cluster: string; kind: resources.Kind }>()
const emit = defineEmits<{
  action: [action: RowAction, row: Row]
  bulkDelete: [rows: Row[]]
  navigate: [target: NavigateTarget]
  create: []
  container: [action: 'logs' | 'previous' | 'shell', container: string, row: Row]
}>()
const search = defineModel<string>('search', { default: '' })

const store = useClusters()
const { namespaces, selectedNamespaces } = storeToRefs(store)
const caseSensitive = ref(false)
const regex = ref(false)

const isPods = computed(() => isKind(props.kind, '', 'pods'))
const isNodes = computed(() => isKind(props.kind, '', 'nodes'))

// One namespace -> server-side filter. Several -> watch all, filter here.
const nsFilter = computed(() => (props.kind.namespaced ? selectedNamespaces.value : []))
const queryNs = computed(() => (nsFilter.value.length === 1 ? nsFilter.value[0] : ''))
const query = computed<resources.Query>(() => ({ namespace: queryNs.value, labelSelector: '', fieldSelector: '' }))
const { columns, rows, loading, error } = useResourceWatch(toRef(props, 'cluster'), toRef(props, 'kind'), query)

const metricsScope = computed<'pods' | 'nodes' | null>(() => (isPods.value ? 'pods' : isNodes.value ? 'nodes' : null))
const { usage, available: metricsOk } = useMetrics(toRef(props, 'cluster'), metricsScope, queryNs)
const usageOf = (r: Row) => usage.value.get(`${r.namespace ?? ''}/${r.name}`)

const now = ref(Date.now())
const tick = setInterval(() => (now.value = Date.now()), 1000)
onBeforeUnmount(() => clearInterval(tick))

// ---- filtering -------------------------------------------------------------
const matcher = computed(() => makeMatcher(search.value, caseSensitive.value, regex.value))
const filtered = computed(() => {
  let list = rows.value
  if (nsFilter.value.length > 1) {
    const set = new Set(nsFilter.value)
    list = list.filter((r) => set.has(r.namespace ?? ''))
  }
  const m = matcher.value
  return m && !m.error ? list.filter(m.test) : list
})

// ---- columns ---------------------------------------------------------------
const statusIdx = computed(() => columns.value.findIndex((c) => STATUS_COLUMNS.has(c.name)))
const hasOwners = computed(() => rows.value.some((r) => r.owner))

const asNum = (v: unknown): number | null =>
  typeof v === 'number' ? v : typeof v === 'string' && /^-?\d/.test(v) ? parseFloat(v) : null
const smartSort: SortingFn<Row> = (a, b, id) => {
  const x = a.getValue(id), y = b.getValue(id)
  const nx = asNum(x), ny = asNum(y)
  if (nx !== null && ny !== null) return nx - ny
  return fmt(x).localeCompare(fmt(y), undefined, { numeric: true })
}

const link = (text: string, go: () => void, title = text) =>
  h('button', {
    class: 'max-w-full truncate text-left text-sky-400/90 hover:text-sky-300 hover:underline', title,
    onClick: (e: MouseEvent) => { e.stopPropagation(); go() },
  }, text)

function checkbox(checked: boolean, indeterminate: boolean, onChange: (e: unknown) => void) {
  return h('input', {
    type: 'checkbox', checked, indeterminate, class: 'h-3.5 w-3.5 cursor-pointer accent-sky-500',
    onClick: (e: MouseEvent) => e.stopPropagation(), onChange,
  })
}

/** "2/3" ready -> ■■□ like a container status strip. */
function readySquares(v: string, status: string) {
  const m = /^(\d+)\/(\d+)$/.exec(v)
  if (!m) return v
  const ready = +m[1], total = +m[2]
  const done = /Succeeded|Completed/.test(status)
  return h('div', { class: 'flex gap-[3px]', title: `${ready}/${total} containers prontos` },
    Array.from({ length: Math.min(total, 12) }, (_, i) =>
      h('span', { class: `h-2.5 w-2.5 rounded-[3px] ${done ? 'bg-zinc-600' : i < ready ? 'bg-emerald-500' : 'bg-amber-500/80'}` })))
}

function serverCol(c: Column, i: number): ColumnDef<Row, unknown> {
  const id = `c:${c.name}`
  const accessorFn = (r: Row) => r.cells[i] ?? ''
  if (isPods.value && c.name === 'Ready') {
    return { id, header: 'Containers', accessorFn, sortingFn: smartSort,
      cell: ({ row }) => readySquares(fmt(row.original.cells[i]), fmt(row.original.cells[statusIdx.value])) }
  }
  if (STATUS_COLUMNS.has(c.name)) {
    return { id, header: c.name, accessorFn, sortingFn: smartSort,
      cell: ({ getValue }) => { const v = fmt(getValue()); return h('span', { class: toneClass[statusTone(v)] }, v) } }
  }
  if (c.name === 'Restarts') {
    return { id, header: c.name, accessorFn, sortingFn: smartSort,
      cell: ({ getValue }) => { const v = fmt(getValue()); return h('span', { class: /^0\b/.test(v) || !v ? 'text-zinc-500' : 'text-amber-400' }, v) } }
  }
  if (isPods.value && c.name === 'Node') {
    return { id, header: c.name, accessorFn, sortingFn: smartSort,
      cell: ({ getValue }) => { const v = fmt(getValue()); return v && v !== '<none>' ? link(v, () => emit('navigate', { kind: 'Node', search: v })) : '' } }
  }
  return { id, header: c.name, accessorFn, sortingFn: smartSort, cell: ({ getValue }) => fmt(getValue()) }
}

const columnDefs = computed<ColumnDef<Row, unknown>[]>(() => {
  const d: ColumnDef<Row, unknown>[] = [
    {
      id: 'select', enableSorting: false, enableHiding: false,
      header: ({ table }) => checkbox(table.getIsAllRowsSelected(), table.getIsSomeRowsSelected(), table.getToggleAllRowsSelectedHandler()),
      cell: ({ row }) => checkbox(row.getIsSelected(), false, row.getToggleSelectedHandler()),
    },
    { id: 'name', header: 'Name', enableHiding: false, accessorFn: (r) => r.name, sortingFn: 'alphanumeric',
      cell: ({ row }) => h('span', { class: 'text-zinc-100', title: row.original.name }, row.original.name) },
  ]
  if (props.kind.namespaced) {
    d.push({ id: 'namespace', header: 'Namespace', accessorFn: (r) => r.namespace ?? '',
      cell: ({ row }) => link(row.original.namespace ?? '', () => (selectedNamespaces.value = [row.original.namespace!]), 'Filtrar por este namespace') })
  }
  if (metricsScope.value && metricsOk.value) {
    d.push(
      { id: 'cpu', header: 'CPU', sortDescFirst: true, accessorFn: (r) => usageOf(r)?.cpu ?? -1,
        cell: ({ getValue }) => { const v = getValue() as number; return v < 0 ? h('span', { class: 'text-zinc-600' }, '—') : cpu(v) } },
      { id: 'memory', header: 'Memory', sortDescFirst: true, accessorFn: (r) => usageOf(r)?.memory ?? -1,
        cell: ({ getValue }) => { const v = getValue() as number; return v < 0 ? h('span', { class: 'text-zinc-600' }, '—') : bytes(v) } },
    )
  }
  columns.value.forEach((c, i) => {
    if (i > 0 && c.name !== 'Age') d.push(serverCol(c, i))
  })
  if (hasOwners.value) {
    d.push({ id: 'owner', header: 'Controlled By', accessorFn: (r) => (r.owner ? `${r.owner.kind}/${r.owner.name}` : ''),
      cell: ({ row }) => {
        const o = row.original.owner
        return o ? link(o.kind, () => emit('navigate', { kind: o.kind, search: o.name, namespace: row.original.namespace }), `${o.kind}/${o.name}`) : ''
      } })
  }
  d.push(
    // -created: ascending sort = youngest first, like "smallest age".
    { id: 'age', header: 'Age', accessorFn: (r) => (r.created ? -Date.parse(r.created) : 0),
      cell: ({ getValue }) => age(-(getValue() as number), now.value) },
    { id: 'menu', enableSorting: false, enableHiding: false,
      header: () => h('button', {
        class: 'grid h-6 w-6 place-items-center rounded text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100', title: 'Colunas',
        onClick: (e: MouseEvent) => { e.stopPropagation(); colMenu.value = { x: e.clientX - 200, y: e.clientY + 8 } },
      }, '⋮'),
      cell: ({ row }) => h('button', {
        class: 'grid h-6 w-6 place-items-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-100',
        onClick: (e: MouseEvent) => { e.stopPropagation(); openMenu(e, row.original) },
      }, '⋮') },
  )
  return d
})

// ---- column visibility (persisted per kind) ---------------------------------
const colKey = `leme:cols:${props.kind.group}/${props.kind.resource}`
const overrides = ref<VisibilityState>((() => {
  try { return JSON.parse(localStorage.getItem(colKey) ?? '{}') } catch { return {} }
})())
const visibility = computed<VisibilityState>(() => {
  const v: VisibilityState = {}
  for (const c of columns.value) {
    // "wide" columns hidden by default, except Node on pods (too useful).
    if (c.priority > 0 && !(isPods.value && c.name === 'Node')) v[`c:${c.name}`] = false
  }
  return { ...v, ...overrides.value }
})
const colMenu = ref<{ x: number; y: number } | null>(null)

// ---- table -----------------------------------------------------------------
const sorting = ref<SortingState>([{ id: 'name', desc: false }])
const rowSelection = ref<RowSelectionState>({})
const table = useVueTable({
  get data() { return filtered.value },
  get columns() { return columnDefs.value },
  state: {
    get sorting() { return sorting.value },
    get columnVisibility() { return visibility.value },
    get rowSelection() { return rowSelection.value },
  },
  onSortingChange: (u) => (sorting.value = typeof u === 'function' ? u(sorting.value) : u),
  onRowSelectionChange: (u) => (rowSelection.value = typeof u === 'function' ? u(rowSelection.value) : u),
  onColumnVisibilityChange: (u) => {
    overrides.value = typeof u === 'function' ? u(visibility.value) : u
    try { localStorage.setItem(colKey, JSON.stringify(overrides.value)) } catch { /* ignore */ }
  },
  enableRowSelection: true,
  getRowId: (r) => r.uid,
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
})
const selectedRows = computed(() => table.getSelectedRowModel().rows.map((r) => r.original))
const tableRows = computed(() => table.getRowModel().rows)

// ---- layout ----------------------------------------------------------------
const ROW_H = 34
const WIDTHS: Record<string, [string, number]> = {
  select: ['40px', 40], name: ['minmax(240px,3fr)', 240], namespace: ['minmax(120px,1fr)', 120],
  cpu: ['76px', 76], memory: ['86px', 86], 'c:Ready': ['100px', 100], 'c:Restarts': ['90px', 90],
  owner: ['minmax(110px,1fr)', 110], age: ['70px', 70], menu: ['44px', 44],
}
const tracks = computed(() => table.getVisibleLeafColumns().map((c) => WIDTHS[c.id] ?? ['minmax(100px,1fr)', 100]))
const gridStyle = computed(() => ({ display: 'grid', gridTemplateColumns: tracks.value.map((t) => t[0]).join(' ') }))
const minWidth = computed(() => `${tracks.value.reduce((s, t) => s + t[1], 0)}px`)

const scrollEl = ref<HTMLElement | null>(null)
const virtualizer = useVirtualizer(computed(() => ({
  count: tableRows.value.length,
  getScrollElement: () => scrollEl.value,
  estimateSize: () => ROW_H,
  overscan: 12,
})))

// ---- details & menus ---------------------------------------------------------
const selectedUid = ref('')
const snapshot = ref<Row | null>(null)
const liveSelected = computed(() => (selectedUid.value ? rows.value.find((r) => r.uid === selectedUid.value) ?? null : null))
watch(liveSelected, (r) => r && (snapshot.value = r))
const detailsRow = computed(() => liveSelected.value ?? (selectedUid.value ? snapshot.value : null))

function select(r: Row) {
  selectedUid.value = selectedUid.value === r.uid ? '' : r.uid
  snapshot.value = r
}

const menu = ref<{ x: number; y: number; row: Row } | null>(null)
function openMenu(e: MouseEvent, row: Row) {
  menu.value = { x: e.clientX, y: e.clientY, row }
}
const menuItems = computed<MenuItem<RowAction>[]>(() => {
  if (!menu.value) return []
  return (['details', ...actionsFor(props.kind, menu.value.row)] as RowAction[]).map((a) => ({
    value: a, label: actionLabel[a], danger: dangerous.has(a), separatorBefore: a === 'delete',
  }))
})

function onAction(a: RowAction, row: Row) {
  if (a === 'details') return select(row)
  if (a === 'copy') return void ClipboardSetText(row.name)
  if (a === 'edit') selectedUid.value = '' // editor takes the side panel
  emit('action', a, row)
}

function bulkDelete() {
  emit('bulkDelete', selectedRows.value)
  rowSelection.value = {}
}

function onKey(e: KeyboardEvent) {
  const typing = (e.target as HTMLElement)?.closest('input,textarea,.monaco-editor,.xterm')
  if (e.key === 'Escape' && !typing && !menu.value && !colMenu.value) selectedUid.value = ''
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="relative flex h-full min-w-0">
    <div class="flex min-w-0 flex-1 flex-col">
      <!-- Toolbar -->
      <div class="flex flex-wrap items-center gap-2 border-b border-zinc-800/80 px-4 py-2.5">
        <h1 class="text-[15px] font-semibold text-zinc-100">{{ kind.kind }}</h1>
        <span class="rounded-full bg-zinc-800 px-2 py-0.5 text-[11px] text-zinc-400 tabular-nums">
          {{ filtered.length }}<template v-if="filtered.length !== rows.length"> / {{ rows.length }}</template>
        </span>
        <span v-if="loading" class="h-3 w-3 animate-spin rounded-full border-2 border-zinc-600 border-t-sky-400" title="Carregando…" />
        <span v-if="metricsScope && !metricsOk" class="text-[11px] text-zinc-600" title="metrics-server não encontrado no cluster">sem métricas</span>
        <div class="ml-auto flex min-w-[420px] flex-1 items-center justify-end gap-2">
          <NamespaceSelect v-model="selectedNamespaces" :options="namespaces" :disabled="!kind.namespaced" />
          <div class="flex max-w-md min-w-0 flex-1">
            <SearchBox
              v-model="search" v-model:case-sensitive="caseSensitive" v-model:regex="regex"
              :placeholder="`Buscar ${kind.kind}…`" :error="matcher?.error"
            />
          </div>
        </div>
      </div>

      <!-- Bulk actions -->
      <div v-if="selectedRows.length" class="flex items-center gap-3 border-b border-sky-900/50 bg-sky-950/30 px-4 py-1.5 text-xs">
        <span class="text-sky-200">{{ selectedRows.length }} selecionado(s)</span>
        <button v-if="kind.verbs.includes('delete')" class="rounded border border-rose-900 px-2 py-0.5 text-rose-300 hover:bg-rose-950" @click="bulkDelete">Excluir</button>
        <button class="text-zinc-400 hover:text-zinc-200" @click="rowSelection = {}">Limpar seleção</button>
      </div>
      <div v-if="error" class="border-b border-rose-900/50 bg-rose-950/40 px-4 py-1.5 text-xs text-rose-300">{{ error }}</div>

      <!-- Grid -->
      <div ref="scrollEl" class="min-h-0 flex-1 overflow-auto text-[13px]">
        <div :style="{ minWidth }">
          <div
            v-for="hg in table.getHeaderGroups()" :key="hg.id" :style="gridStyle"
            class="sticky top-0 z-10 h-9 items-center border-b border-zinc-800 bg-zinc-925/95 text-[11px] font-medium text-zinc-400 backdrop-blur"
          >
            <div
              v-for="header in hg.headers" :key="header.id"
              class="flex items-center gap-1 truncate px-3 select-none"
              :class="header.column.getCanSort() && 'cursor-pointer hover:text-zinc-100'"
              @click="header.column.getCanSort() && header.column.toggleSorting()"
            >
              <FlexRender :render="header.column.columnDef.header" :props="header.getContext()" />
              <span v-if="header.column.getIsSorted()" class="text-sky-400">{{ header.column.getIsSorted() === 'asc' ? '↑' : '↓' }}</span>
            </div>
          </div>

          <div class="relative" :style="{ height: `${virtualizer.getTotalSize()}px` }">
            <div
              v-for="v in virtualizer.getVirtualItems()" :key="tableRows[v.index].id"
              class="absolute inset-x-0 cursor-default items-center border-b border-zinc-800/50"
              :class="tableRows[v.index].id === selectedUid
                ? 'bg-sky-500/10 shadow-[inset_2px_0_0_var(--color-sky-400)]'
                : tableRows[v.index].getIsSelected() ? 'bg-sky-500/5' : 'hover:bg-zinc-800/40'"
              :style="{ ...gridStyle, height: `${ROW_H}px`, transform: `translateY(${v.start}px)` }"
              @click="select(tableRows[v.index].original)"
              @dblclick="onAction('edit', tableRows[v.index].original)"
              @contextmenu.prevent="openMenu($event, tableRows[v.index].original)"
            >
              <div v-for="c in tableRows[v.index].getVisibleCells()" :key="c.id" class="flex min-w-0 items-center truncate px-3">
                <FlexRender :render="c.column.columnDef.cell" :props="c.getContext()" />
              </div>
            </div>
          </div>

          <div v-if="!loading && !filtered.length" class="grid place-items-center py-24 text-center text-sm text-zinc-500">
            <div>
              <p class="text-zinc-300">Nenhum {{ kind.kind }} encontrado</p>
              <p v-if="search" class="mt-1 text-xs">Nada corresponde a “{{ search }}”.
                <button class="text-sky-400 hover:underline" @click="search = ''">Limpar busca</button></p>
              <p v-else-if="selectedNamespaces.length && kind.namespaced" class="mt-1 text-xs">Filtrando {{ selectedNamespaces.join(', ') }}.
                <button class="text-sky-400 hover:underline" @click="selectedNamespaces = []">Ver todos os namespaces</button></p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <DetailsDrawer
      v-if="detailsRow" :key="detailsRow.uid"
      :cluster="cluster" :kind="kind" :row="detailsRow" :columns="columns" :usage="usageOf(detailsRow)"
      :gone="!liveSelected"
      @close="selectedUid = ''" @action="onAction" @navigate="emit('navigate', $event)"
      @container="(a, c, r) => emit('container', a, c, r)"
    />

    <button
      v-if="kind.verbs.includes('create') && !detailsRow"
      class="absolute right-6 bottom-6 z-20 grid h-11 w-11 place-items-center rounded-full bg-sky-500 text-2xl leading-none text-white shadow-lg shadow-sky-900/50 transition hover:scale-105 hover:bg-sky-400"
      :title="`Criar ${kind.kind}`" @click="emit('create')"
    >+</button>

    <ContextMenu
      v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" :title="menu.row.name"
      @select="onAction($event, menu!.row)" @close="menu = null"
    />

    <!-- Column chooser -->
    <Teleport to="body">
      <div v-if="colMenu" class="fixed inset-0 z-[55]" @mousedown.self="colMenu = null">
        <div class="absolute w-56 rounded-lg border border-zinc-800 bg-zinc-900 py-1 text-xs shadow-2xl shadow-black/60" :style="{ left: `${Math.max(8, colMenu.x)}px`, top: `${colMenu.y}px` }">
          <div class="px-3 pt-1 pb-2 text-[11px] text-zinc-500">Colunas visíveis</div>
          <label
            v-for="c in table.getAllLeafColumns().filter((c) => c.getCanHide())" :key="c.id"
            class="flex cursor-pointer items-center gap-2 px-3 py-1.5 hover:bg-zinc-800"
          >
            <input type="checkbox" class="accent-sky-500" :checked="c.getIsVisible()" @change="c.toggleVisibility()" />
            {{ typeof c.columnDef.header === 'string' ? c.columnDef.header : c.id }}
          </label>
          <div class="mt-1 border-t border-zinc-800 px-3 pt-2 pb-1">
            <button class="text-zinc-400 hover:text-zinc-100" @click="table.setColumnVisibility({})">Restaurar padrão</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
