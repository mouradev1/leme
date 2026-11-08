import { tr } from '@/lib/i18n'
import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { Add, CancelLogin, Info, List, Login, LoginWithKey, Logout, Refresh, Remove } from '../../wailsjs/go/api/RancherAPI'
import type { rancher } from '../../wailsjs/go/models'
import { on } from '@/lib/bridge'

export const RANCHER_ORIGIN = 'rancher:'

/** "rancher-login:<url>|<message>" errors mean: log in to that Rancher, then retry. */
export function loginNeeded(msg: string): { url: string; text: string } | null {
  const m = /^rancher-login:([^|]+)\|(.*)$/s.exec(msg)
  return m ? { url: m[1], text: m[2] } : null
}

// Rancher hands the browser-login token to apps only with SAML providers;
// with OIDC (e.g. Keycloak OIDC) the user logs in but no token comes back.
const APP_LOGIN = new Set(['keyCloakProvider', 'oktaProvider', 'adfsProvider', 'pingProvider', 'shibbolethProvider'])

export const PROVIDERS: Record<string, string> = {
  localProvider: 'local', keyCloakProvider: 'Keycloak (SAML)', keyCloakOIDCProvider: 'Keycloak (OIDC)',
  genericOIDCProvider: 'OIDC', oktaProvider: 'Okta (SAML)', adfsProvider: 'ADFS (SAML)', pingProvider: 'Ping (SAML)',
  shibbolethProvider: 'Shibboleth (SAML)', azureADProvider: 'Azure AD', githubProvider: 'GitHub',
  activeDirectoryProvider: 'Active Directory', openLdapProvider: 'OpenLDAP', freeIpaProvider: 'FreeIPA',
  googleOAuthProvider: 'Google', cognitoProvider: 'Cognito',
}

export const stateLabel: Record<string, string> = {
  ok: 'conectado', expired: 'sessão expirada', 'logged-out': 'não conectado', 'logging-in': 'aguardando login no navegador…',
}

/** Rancher servers: browser login, their clusters show up in the normal list. */
export const useRancher = defineStore('rancher', () => {
  const servers = ref<rancher.Status[]>([])
  on<rancher.Status[]>('rancher:changed', (l) => (servers.value = l))

  async function load() {
    servers.value = await List()
  }

  // Public info per server (version, login methods), fetched once.
  const info = reactive<Record<string, rancher.Info>>({})
  function fetchInfo(url: string) {
    if (!info[url]) Info(url).then((i) => (info[url] = i)).catch(() => {})
  }
  /** true: browser login works; false: use an API key; undefined: unknown yet. */
  const browserLogin = (url: string): boolean | undefined => {
    const p = info[url]?.providers
    if (!p?.length) return undefined
    return p.some((x) => APP_LOGIN.has(x))
  }
  const loginMethods = (url: string) =>
    (info[url]?.providers ?? []).filter((p) => p !== 'localProvider').map((p) => tr(PROVIDERS[p] ?? p)).join(', ')

  const byUrl = computed(() => new Map(servers.value.map((s) => [s.url, s])))
  /** The Rancher a context comes from (ContextInfo.source = "rancher:<url>"). */
  const serverOf = (source: string) =>
    source?.startsWith(RANCHER_ORIGIN) ? byUrl.value.get(source.slice(RANCHER_ORIGIN.length)) : undefined

  return {
    servers, load, serverOf, info, fetchInfo, browserLogin, loginMethods,
    add: (url: string, name: string) => Add(url, name),
    login: (url: string) => Login(url),
    cancelLogin: (url: string) => CancelLogin(url),
    loginWithKey: (url: string, key: string) => LoginWithKey(url, key),
    refresh: (url: string) => Refresh(url),
    logout: (url: string) => Logout(url),
    remove: (url: string) => Remove(url),
  }
})
