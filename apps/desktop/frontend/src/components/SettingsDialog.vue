<script setup lang="ts">
/** App settings: language, update notice, node shell. */
import { onMounted, ref } from 'vue'
import { Prefs, SetNodeShell, SetUpdateCheck } from '../../wailsjs/go/api/AppAPI'
import { errMsg } from '@/lib/bridge'
import { lang, tr, type Lang } from '@/lib/i18n'

const emit = defineEmits<{ close: [] }>()

const updates = ref(true)
const shellOn = ref(true)
const image = ref('')
const defaultImage = ref('')
const msg = ref<{ ok: boolean; text: string } | null>(null)

onMounted(async () => {
  const p = await Prefs().catch(() => null)
  if (!p) return
  updates.value = p.updateCheck
  shellOn.value = p.nodeShellEnabled
  image.value = p.nodeShellImage
  defaultImage.value = p.defaultShellImage
})

const setLang = (l: Lang) => (lang.value = l)
const saveUpdates = () => SetUpdateCheck(updates.value).catch(() => {})
async function saveShell() {
  msg.value = null
  try {
    await SetNodeShell(shellOn.value, image.value)
    msg.value = { ok: true, text: tr('Salvo') }
  } catch (e) {
    msg.value = { ok: false, text: errMsg(e) }
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60" @click.self="emit('close')">
    <div class="w-[520px] max-w-[94vw] rounded-xl border border-zinc-800 bg-zinc-900 text-sm shadow-2xl">
      <div class="flex items-center border-b border-zinc-800 px-4 py-3">
        <h2 class="font-semibold text-zinc-100">{{ tr('Ajustes') }}</h2>
        <button class="ml-auto rounded px-2 py-1 text-xs text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" @click="emit('close')">{{ tr('Fechar') }}</button>
      </div>
      <div class="grid gap-5 px-4 py-4 text-xs">
        <section class="grid gap-2">
          <h3 class="text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">{{ tr('Idioma') }}</h3>
          <div class="flex gap-1.5">
            <button
              v-for="[v, l] in ([['pt', 'Português'], ['en', 'English']] as [Lang, string][])" :key="v"
              class="rounded-md border px-3 py-1.5" :class="lang === v ? 'border-sky-600 bg-sky-500/15 text-sky-200' : 'border-zinc-800 text-zinc-300 hover:border-zinc-600'"
              @click="setLang(v)"
            >{{ l }}</button>
          </div>
          <p class="text-zinc-500">{{ tr('O idioma do sistema é usado na primeira vez.') }}</p>
        </section>

        <section class="grid gap-2">
          <h3 class="text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">{{ tr('Atualizações') }}</h3>
          <label class="flex items-center gap-2 text-zinc-300">
            <input v-model="updates" type="checkbox" class="accent-sky-500" @change="saveUpdates" />
            {{ tr('Avisar quando houver uma versão nova (uma consulta anônima ao GitHub por dia)') }}
          </label>
        </section>

        <section class="grid gap-2">
          <h3 class="text-[11px] font-semibold tracking-wider text-zinc-500 uppercase">{{ tr('Shell no node (pod privilegiado temporário)') }}</h3>
          <label class="flex items-center gap-2 text-zinc-300">
            <input v-model="shellOn" type="checkbox" class="accent-sky-500" /> {{ tr('Permitir abrir shell root nos nodes') }}
          </label>
          <label class="grid gap-1 text-zinc-400">{{ tr('Imagem do pod (fixada por digest)') }}
            <input
              v-model="image" :disabled="!shellOn" :placeholder="defaultImage" spellcheck="false"
              class="h-8 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-[11px] text-zinc-200 outline-none focus:border-sky-600 disabled:opacity-50"
            />
          </label>
          <p class="text-zinc-500">{{ tr('Deixe vazio para usar a imagem padrão. Para um registry interno, use uma imagem fixada por digest.') }}</p>
          <div class="flex items-center gap-2">
            <button class="rounded-md bg-sky-600 px-3 py-1.5 font-medium text-white hover:bg-sky-500" @click="saveShell">{{ tr('Salvar') }}</button>
            <span v-if="msg" :class="msg.ok ? 'text-emerald-400' : 'text-rose-400'">{{ msg.text }}</span>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
