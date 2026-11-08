<script setup lang="ts">
/**
 * Rancher servers: add the address, log in through the browser (Keycloak or
 * whatever the server uses) and its clusters join the cluster list. When the
 * session expires, logging in again fixes every cluster of that server.
 */
import { tr } from '@/lib/i18n'
import { onMounted, ref, watch } from 'vue'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { storeToRefs } from 'pinia'
import { errMsg } from '@/lib/bridge'
import { stateLabel, useRancher } from '@/stores/rancher'
import Icon from './Icon.vue'

const props = defineProps<{ focus?: string }>() // server to show the API key field for
const emit = defineEmits<{ connect: [context: string] }>()

const store = useRancher()
const { servers } = storeToRefs(store)
onMounted(store.load)

const url = ref('')
const name = ref('')
const busy = ref('') // url (or 'add') of the action in progress
const msg = ref<{ ok: boolean; text: string } | null>(null)
const keyFor = ref(props.focus ?? '') // server showing the API key field
const key = ref('')
const openList = ref('') // server whose cluster list is expanded

async function run(id: string, fn: () => Promise<string | void>) {
  busy.value = id
  msg.value = null
  try {
    const text = await fn()
    if (text) msg.value = { ok: true, text }
  } catch (e) {
    msg.value = { ok: false, text: errMsg(e) }
  } finally {
    busy.value = ''
  }
}

const add = () => run('add', async () => {
  const s = await store.add(url.value, name.value)
  url.value = ''
  name.value = ''
  store.fetchInfo(s.url)
  // Wait briefly for the login methods: OIDC servers go straight to the API key.
  for (let i = 0; i < 20 && store.browserLogin(s.url) === undefined; i++) await new Promise((r) => setTimeout(r, 100))
  if (store.browserLogin(s.url) === false) {
    keyFor.value = s.url
    openKeyPage(s.url)
    return tr('Rancher adicionado. Este servidor usa ') + (loginMethods(s.url) || tr('um login')) +
      tr(', que não entrega o token para apps: crie a API key na página que abriu e cole abaixo.')
  }
  await login(s.url)
})

// Not via run(): the wait can be long and must not lock the other buttons.
async function login(u: string) {
  msg.value = null
  try {
    const s = await store.login(u)
    msg.value = { ok: true, text: `Conectado ao Rancher ${s.name}${s.user ? ` como ${s.user}` : ''}: ${s.clusters.length} cluster(s).` }
    openList.value = u
  } catch (e) {
    const t = errMsg(e)
    if (!/cancel/i.test(t)) msg.value = { ok: false, text: t }
  }
}

const useKey = (u: string) => run(u, async () => {
  const s = await store.loginWithKey(u, key.value)
  key.value = ''
  keyFor.value = ''
  openList.value = u
  return tr('Conectado ao Rancher {a}: {b} cluster(s).', { a: s.name, b: s.clusters.length })
})
const refresh = (u: string) => run(u, async () => tr('{a} cluster(s) no Rancher.', { a: (await store.refresh(u)).clusters.length }))
const logout = (u: string) => run(u, async () => { await store.logout(u); return tr('Sessão encerrada e token revogado no Rancher.') })
const remove = (u: string, n: string) => {
  if (!confirm(tr('Remover o Rancher {a} do Leme? A sessão é encerrada e os clusters dele saem da lista.', { a: n }))) return
  run(u, () => store.remove(u))
}

watch(servers, (l) => l.forEach((s) => store.fetchInfo(s.url)), { immediate: true })
const { info } = storeToRefs(store)
const loginMethods = store.loginMethods

// Rancher's page to create an API key (the user picks the expiry there).
const openKeyPage = (u: string) => BrowserOpenURL(`${u}/dashboard/account/create-key`)

// Waiting a long time usually means this server doesn't hand the token to apps.
const slow = ref<Record<string, boolean>>({})
const timers: Record<string, number> = {}
watch(servers, (l) => {
  for (const s of l) {
    if (s.state === 'logging-in' && !timers[s.url]) timers[s.url] = window.setTimeout(() => (slow.value[s.url] = true), 25000)
    if (s.state !== 'logging-in' && timers[s.url]) {
      clearTimeout(timers[s.url])
      delete timers[s.url]
      slow.value[s.url] = false
    }
  }
}, { deep: true })

const expires = (at: string) => {
  if (!at) return ''
  const d = new Date(at)
  return Number.isNaN(d.getTime()) ? '' : `expira ${d.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}`
}
const dot: Record<string, string> = { ok: 'bg-emerald-400', expired: 'bg-amber-400', 'logged-out': 'bg-zinc-500', 'logging-in': 'bg-sky-400 animate-pulse' }
</script>

<template>
  <div>
    <p class="mb-2 text-xs text-zinc-500">
      {{ tr('Informe o endereço do Rancher. O login abre no navegador, com o mesmo acesso que você usa hoje (Keycloak, AD…), e os clusters que você tem permissão de ver entram na lista. O token fica no Keychain do sistema.') }}
    </p>
    <div class="flex gap-2">
      <input
        v-model="url" :placeholder="tr('https://rancher.suaempresa.com')" spellcheck="false" autocomplete="off"
        class="h-8 min-w-0 flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-xs outline-none focus:border-sky-600"
        @keydown.enter="url && add()"
      />
      <input
        v-model="name" :placeholder="tr('Apelido (ex: prod)')" spellcheck="false"
        class="h-8 w-36 rounded-md border border-zinc-800 bg-zinc-950 px-2 text-xs outline-none focus:border-sky-600"
        @keydown.enter="url && add()"
      />
      <button class="h-8 rounded-md bg-sky-600 px-3 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50" :disabled="!!busy || !url.trim()" @click="add">
        {{ tr('Adicionar e entrar') }}
      </button>
    </div>
    <p class="mt-1.5 text-[11px] text-zinc-600">{{ tr('O apelido aparece antes do nome de cada cluster:') }} <span class="font-mono">{{ tr('prod/meu-cluster') }}</span>.</p>

    <p v-if="msg" class="mt-3 text-xs whitespace-pre-wrap" :class="msg.ok ? 'text-emerald-400' : 'text-rose-400'">{{ msg.text }}</p>

    <ul v-if="servers.length" class="mt-4 divide-y divide-zinc-800 overflow-hidden rounded-md border border-zinc-800">
      <li v-for="s in servers" :key="s.url" class="px-3 py-2.5">
        <div class="flex items-center gap-2">
          <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="dot[s.state] ?? 'bg-zinc-500'" />
          <span class="font-medium text-zinc-100">{{ s.name }}</span>
          <span class="min-w-0 truncate font-mono text-[11px] text-zinc-500" :title="s.url">{{ s.url.replace('https://', '') }}</span>
          <span class="ml-auto shrink-0 text-[11px]" :class="s.state === 'ok' ? 'text-zinc-500' : 'text-amber-300'">{{ tr(stateLabel[s.state] ?? s.state) ?? s.state }}</span>
        </div>
        <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 pl-3.5 text-[11px] text-zinc-500">
          <span v-if="s.user">{{ s.user }}</span>
          <span v-if="info[s.url]?.version" class="font-mono">{{ info[s.url].version }}</span>
          <span v-if="loginMethods(s.url)">login: {{ loginMethods(s.url) }}</span>
          <span v-if="s.state === 'ok' && expires(s.expiresAt)">{{ expires(s.expiresAt) }}</span>
          <button class="hover:text-zinc-200" @click="openList = openList === s.url ? '' : s.url">{{ tr('{n} cluster(s)', { n: s.clusters.length }) }}</button>
          <span class="ml-auto flex items-center gap-1">
            <button v-if="s.state === 'logging-in'" class="act" @click="store.cancelLogin(s.url)">{{ tr('Cancelar') }}</button>
            <button v-else-if="s.state !== 'ok' && store.browserLogin(s.url) !== false" class="act act-primary" :disabled="!!busy" @click="login(s.url)">{{ tr('Entrar') }}</button>
            <button
              v-if="s.state !== 'ok' && s.state !== 'logging-in' && store.browserLogin(s.url) === false" class="act act-primary"
              :title="tr('Abre a página do Rancher para criar a chave')" @click="keyFor = s.url; openKeyPage(s.url)"
            >{{ tr('Criar API key') }}</button>
            <button v-if="s.state === 'ok'" class="act" :disabled="!!busy" :title="tr('Buscar clusters novos')" @click="refresh(s.url)">
              <Icon name="refresh" :size="11" /> {{ tr('Atualizar') }}
            </button>
            <button v-if="s.state !== 'ok' && s.state !== 'logging-in' && store.browserLogin(s.url) !== false" class="act" @click="keyFor = keyFor === s.url ? '' : s.url">{{ tr('API key') }}</button>
            <button v-if="s.state === 'ok' || s.state === 'expired'" class="act" :disabled="!!busy" @click="logout(s.url)">{{ tr('Sair') }}</button>
            <button class="act text-rose-400 hover:text-rose-300" :disabled="!!busy" @click="remove(s.url, s.name)">{{ tr('Remover') }}</button>
          </span>
        </div>
        <p v-if="s.state === 'logging-in' && slow[s.url]" class="mt-2 ml-3.5 rounded-md border border-amber-900/60 bg-amber-950/40 p-2 text-[11px] leading-relaxed text-amber-200">
          Já entrou no navegador e nada aconteceu? Este Rancher provavelmente não entrega o token para apps com
          {{ loginMethods(s.url) || tr('este tipo de login') }}. Clique em <b>{{ tr('Cancelar') }}</b> {{ tr('e use') }} <b>{{ tr('API key') }}</b>{{ tr(': no Rancher, clique no seu avatar ›') }} <i>{{ tr('Account & API Keys') }}</i> › <i>{{ tr('Create API Key') }}</i>{{ tr(', e cole o') }}
          <i>{{ tr('Bearer Token') }}</i> {{ tr('aqui.') }}
        </p>
        <p v-if="keyFor === s.url" class="mt-2 pl-3.5 text-[11px] leading-relaxed text-zinc-500">
          {{ tr('Na página do Rancher (') }}<button class="text-sky-400 hover:underline" @click="openKeyPage(s.url)">{{ tr('abrir de novo') }}</button>{{ tr('): dê uma descrição como "Leme", escolha a validade, clique em') }} <i>{{ tr('Create') }}</i> {{ tr('e copie o') }} <i>{{ tr('Bearer Token') }}</i>{{ tr('. Quando a chave vencer, é só criar outra e colar aqui: todos os clusters deste Rancher voltam.') }}
        </p>
        <div v-if="keyFor === s.url" class="mt-2 flex gap-2 pl-3.5">
          <input
            v-model="key" type="password" :placeholder="tr('token-xxxxx:yyyyyyyy (Rancher › Account & API Keys › Create)')" autocomplete="off"
            class="h-7 min-w-0 flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-2 font-mono text-xs outline-none focus:border-sky-600"
            @keydown.enter="key && useKey(s.url)"
          />
          <button class="h-7 rounded-md bg-zinc-800 px-3 text-xs hover:bg-zinc-700 disabled:opacity-50" :disabled="!!busy || !key" @click="useKey(s.url)">{{ tr('Usar chave') }}</button>
        </div>
        <div v-if="openList === s.url && s.clusters.length" class="mt-2 flex flex-wrap gap-1.5 pl-3.5">
          <button
            v-for="c in s.clusters" :key="c"
            class="rounded-md border border-zinc-700 px-2 py-0.5 font-mono text-xs text-zinc-200 hover:border-sky-600 hover:text-sky-300"
            :title="s.state === 'ok' ? tr('Conectar') : tr('Entre no Rancher primeiro')" @click="emit('connect', c)"
          >{{ c }}</button>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
@reference "../style.css";
.act { @apply flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100 disabled:opacity-50; }
.act-primary { @apply bg-sky-600 text-white hover:bg-sky-500 hover:text-white; }
</style>
