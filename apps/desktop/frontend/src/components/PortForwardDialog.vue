<script setup lang="ts">
/** Choose a remote port and (optionally) a local port, then forward. */
import { computed, onMounted, ref } from 'vue'
import { BrowserOpenURL, ClipboardSetText } from '../../wailsjs/runtime/runtime'
import { ServicePorts, Start } from '../../wailsjs/go/api/PortForwardAPI'
import { PodDetails } from '../../wailsjs/go/api/ResourceAPI'
import { portforward } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'
import { forwardURL, type ForwardTarget } from '@/lib/forward'
import Icon from './Icon.vue'

const props = defineProps<{ target: ForwardTarget }>()
const emit = defineEmits<{ close: []; started: [f: portforward.Forward] }>()

interface PortOption { port: number; label: string }
const options = ref<PortOption[]>([])
const remote = ref<number | null>(props.target.port ?? null)
const local = ref<number | null>(null) // null = automatic
const openBrowser = ref(true)
const loading = ref(true)
const busy = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    if (props.target.kind === 'service') {
      const ps = await ServicePorts(props.target.cluster, props.target.namespace, props.target.name)
      options.value = ps.map((p) => ({ port: p.port, label: `${p.name ? `${p.name} · ` : ''}${p.port}/${p.protocol} → ${p.targetPort}` }))
    } else {
      const d = await PodDetails(props.target.cluster, props.target.namespace, props.target.name)
      const seen = new Set<number>()
      for (const c of [...(d.containers ?? [])]) {
        for (const p of c.ports ?? []) {
          if (seen.has(p.containerPort)) continue
          seen.add(p.containerPort)
          options.value.push({ port: p.containerPort, label: `${p.name ? `${p.name} · ` : ''}${p.containerPort}/${p.protocol} (${c.name})` })
        }
      }
    }
    if (remote.value == null && options.value.length) remote.value = options.value[0].port
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    loading.value = false
  }
})

const valid = computed(() => !!remote.value && remote.value > 0 && remote.value < 65536 && (local.value == null || (local.value > 0 && local.value < 65536)))

async function start() {
  if (!valid.value) return
  busy.value = true
  error.value = ''
  try {
    const f = await Start(props.target.cluster, portforward.Request.createFrom({
      namespace: props.target.namespace, kind: props.target.kind, name: props.target.name,
      remotePort: remote.value, localPort: local.value ?? 0,
    }))
    void ClipboardSetText(forwardURL(f))
    if (openBrowser.value) BrowserOpenURL(forwardURL(f))
    emit('started', f)
    emit('close')
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-[2px]" @click.self="emit('close')">
    <div class="w-[440px] max-w-[92vw] rounded-xl border border-zinc-800 bg-zinc-900 text-sm shadow-2xl">
      <div class="flex items-center gap-2 border-b border-zinc-800 px-5 py-3">
        <Icon name="forward" :size="16" class="text-sky-300" />
        <h2 class="min-w-0 flex-1 truncate font-semibold text-zinc-100">Port-forward · {{ target.kind === 'service' ? 'Service' : 'Pod' }} {{ target.name }}</h2>
        <button class="grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" title="Fechar" @click="emit('close')"><Icon name="close" :size="14" /></button>
      </div>

      <div class="space-y-4 px-5 py-4 text-xs">
        <div>
          <div class="mb-1.5 text-zinc-400">Porta {{ target.kind === 'service' ? 'do service' : 'do container' }}</div>
          <p v-if="loading" class="text-zinc-500">carregando portas…</p>
          <div v-else class="space-y-1">
            <label v-for="o in options" :key="o.port" class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-zinc-800">
              <input v-model="remote" type="radio" :value="o.port" class="accent-sky-500" />
              <span class="font-mono text-zinc-200">{{ o.label }}</span>
            </label>
            <label class="flex items-center gap-2 px-2 py-1.5 text-zinc-400">
              outra porta
              <input
                type="number" min="1" max="65535" placeholder="ex: 8080"
                class="h-7 w-28 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-zinc-200 outline-none focus:border-sky-600"
                @input="remote = Number(($event.target as HTMLInputElement).value) || null"
              />
            </label>
          </div>
        </div>

        <div>
          <div class="mb-1.5 text-zinc-400">Porta local (no seu computador)</div>
          <input
            v-model.number="local" type="number" min="1" max="65535" placeholder="automática"
            class="h-8 w-40 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-zinc-200 outline-none focus:border-sky-600"
            @input="local = Number(($event.target as HTMLInputElement).value) || null"
          />
          <p class="mt-1 text-zinc-500">Só acessível nesta máquina (127.0.0.1). O endereço é copiado ao iniciar.</p>
        </div>

        <label class="flex cursor-pointer items-center gap-2 text-zinc-300">
          <input v-model="openBrowser" type="checkbox" class="accent-sky-500" /> Abrir no navegador
        </label>

        <p v-if="error" class="rounded-md border border-rose-900/60 bg-rose-950/40 px-3 py-2 text-rose-300">{{ error }}</p>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-800 px-5 py-3">
        <button class="rounded-md bg-zinc-800 px-3 py-1.5 text-xs hover:bg-zinc-700" @click="emit('close')">Cancelar</button>
        <button class="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50" :disabled="!valid || busy" @click="start">
          {{ busy ? 'Conectando…' : 'Iniciar' }}
        </button>
      </div>
    </div>
  </div>
</template>
