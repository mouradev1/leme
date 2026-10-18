<script setup lang="ts">
/** Active port-forwards across all clusters. */
import { onBeforeUnmount, ref } from 'vue'
import { BrowserOpenURL, ClipboardSetText } from '../../wailsjs/runtime/runtime'
import { Start, Stop } from '../../wailsjs/go/api/PortForwardAPI'
import { portforward } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'
import { forwards, forwardURL } from '@/lib/forward'
import { age } from '@/lib/format'
import Icon from './Icon.vue'

const emit = defineEmits<{ toast: [msg: string] }>()
const now = ref(Date.now())
const t = setInterval(() => (now.value = Date.now()), 1000)
onBeforeUnmount(() => clearInterval(t))

async function restart(f: portforward.Forward) {
  try {
    await Start(f.cluster, portforward.Request.createFrom({
      namespace: f.namespace, kind: f.kind, name: f.name, remotePort: f.remotePort, localPort: f.localPort,
    }))
    await Stop(f.id)
    emit('toast', `Reconectado em ${forwardURL(f)}`)
  } catch (e) {
    emit('toast', errMsg(e))
  }
}
function copy(f: portforward.Forward) {
  void ClipboardSetText(forwardURL(f))
  emit('toast', `${forwardURL(f)} copiado`)
}
</script>

<template>
  <div class="flex h-full min-w-0 flex-col">
    <div class="flex items-center gap-2 border-b border-zinc-800/80 px-4 py-2.5">
      <h1 class="text-[15px] font-semibold text-zinc-100">Port Forwards</h1>
      <span class="rounded-full bg-zinc-800 px-2 py-0.5 text-[11px] text-zinc-400">{{ forwards.length }}</span>
      <span class="ml-auto text-xs text-zinc-500">Inicie pelo painel de um pod (porta do container) ou de um service. Todos param ao fechar o Leme.</span>
    </div>
    <div class="min-h-0 flex-1 overflow-auto">
      <table class="w-full text-[13px]">
        <thead class="sticky top-0 bg-zinc-925 text-left text-[11px] font-medium text-zinc-400">
          <tr class="border-b border-zinc-800">
            <th class="px-4 py-2.5">Endereço local</th><th class="px-3">Destino</th><th class="px-3">Pod</th>
            <th class="px-3">Cluster</th><th class="px-3">Estado</th><th class="px-3">Há</th><th class="px-3" />
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in forwards" :key="f.id" class="border-b border-zinc-800/50 hover:bg-zinc-800/40">
            <td class="px-4 py-2">
              <button class="font-mono text-sky-400 hover:underline" title="Abrir no navegador" @click="BrowserOpenURL(forwardURL(f))">localhost:{{ f.localPort }}</button>
            </td>
            <td class="px-3"><span class="text-zinc-400">{{ f.kind === 'service' ? 'svc' : 'pod' }}/</span>{{ f.name }}<span class="text-zinc-500">:{{ f.remotePort }}</span> <span class="text-zinc-600">({{ f.namespace }})</span></td>
            <td class="px-3 text-zinc-400">{{ f.pod }}<span v-if="f.podPort !== f.remotePort" class="text-zinc-600">:{{ f.podPort }}</span></td>
            <td class="px-3 text-zinc-400">{{ f.cluster }}</td>
            <td class="px-3">
              <span v-if="f.state === 'active'" class="text-emerald-400">ativo</span>
              <span v-else class="text-rose-400" :title="f.error">parou</span>
            </td>
            <td class="px-3 text-zinc-500">{{ age(Date.parse(f.started), now) }}</td>
            <td class="px-3">
              <div class="flex justify-end gap-1">
                <button v-if="f.state !== 'active'" class="act" title="Reconectar (o pod pode ter sido recriado)" @click="restart(f)"><Icon name="refresh" :size="14" /></button>
                <button class="act" title="Copiar endereço" @click="copy(f)"><Icon name="logs" :size="14" /></button>
                <button class="act hover:!text-rose-300" title="Parar" @click="Stop(f.id)"><Icon name="close" :size="14" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!forwards.length" class="py-20 text-center text-sm text-zinc-500">Nenhum port-forward ativo.</p>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";
.act { @apply grid h-7 w-7 place-items-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100; }
</style>
