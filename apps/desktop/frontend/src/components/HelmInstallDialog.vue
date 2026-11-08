<script setup lang="ts">
/**
 * Install a chart, or upgrade a release (new values and/or chart version).
 * Remote charts only over https:// or oci:// (checked again in the backend).
 */
import { tr } from '@/lib/i18n'
import { computed, ref } from 'vue'
import { Install, Upgrade, Values } from '../../wailsjs/go/api/HelmAPI'
import { helm } from '../../wailsjs/go/models'
import { errMsg } from '@/lib/bridge'
import { isProd } from '@/lib/guard'

const props = defineProps<{ cluster: string; namespaces: string[]; release?: helm.Release; defaultNamespace?: string }>()
const emit = defineEmits<{ close: []; done: [rel: helm.Release] }>()

const upgrading = computed(() => !!props.release)
const repoURL = ref('')
const chart = ref(props.release ? '' : '')
const version = ref('')
const name = ref(props.release?.name ?? '')
const namespace = ref(props.release?.namespace ?? props.defaultNamespace ?? 'default')
const createNamespace = ref(false)
const values = ref('')
const wait = ref(true)
const busy = ref(false)
const error = ref('')
const confirmProd = ref('')

if (props.release) {
  Values(props.cluster, props.release.namespace, props.release.name, false).then((v) => (values.value = v.trim() === '{}' ? '' : v)).catch(() => {})
}

const newNs = computed(() => !props.namespaces.includes(namespace.value))
const needsProdName = computed(() => isProd(props.cluster))
const valid = computed(() =>
  name.value.trim() && namespace.value.trim() && (upgrading.value || chart.value.trim()) &&
  (!needsProdName.value || confirmProd.value.trim() === props.cluster))

async function submit() {
  if (!valid.value) return
  busy.value = true
  error.value = ''
  try {
    const rel = upgrading.value
      ? await Upgrade(props.cluster, helm.UpgradeRequest.createFrom({
        releaseName: name.value.trim(), namespace: namespace.value.trim(), chart: chart.value.trim(),
        repoURL: repoURL.value.trim(), version: version.value.trim(), valuesYAML: values.value, wait: wait.value,
      }))
      : await Install(props.cluster, helm.InstallRequest.createFrom({
        releaseName: name.value.trim(), namespace: namespace.value.trim(), chart: chart.value.trim(),
        repoURL: repoURL.value.trim(), version: version.value.trim(), valuesYAML: values.value,
        createNamespace: createNamespace.value || newNs.value, wait: wait.value,
      }))
    emit('done', rel)
    emit('close')
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/60" @click.self="!busy && emit('close')">
    <div class="flex max-h-[90vh] w-[640px] max-w-[94vw] flex-col rounded-xl border border-zinc-800 bg-zinc-900 text-sm shadow-2xl">
      <div class="border-b border-zinc-800 px-4 py-3">
        <h2 class="font-semibold text-zinc-100">{{ upgrading ? tr('Atualizar {a}', { a: release!.name }) : tr('Instalar chart') }}</h2>
        <p class="text-xs text-zinc-500">
          {{ upgrading ? tr('{a}-{b} em {c}. Deixe o chart vazio para só mudar os values.', { a: release!.chart, b: release!.chartVersion, c: release!.namespace }) : tr('Repositório https:// ou registry oci://.') }}
        </p>
      </div>
      <div class="grid min-h-0 gap-3 overflow-auto px-4 py-3 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <label class="grid gap-1">{{ tr('Repositório (opcional)') }}
            <input v-model="repoURL" :placeholder="tr('https://charts.bitnami.com/bitnami')" spellcheck="false" class="field font-mono" />
          </label>
          <label class="grid gap-1">Chart{{ upgrading ? tr(' (opcional)') : '' }}
            <input v-model="chart" :placeholder="upgrading ? tr('manter {a}', { a: release!.chart }) : tr('nginx  ou  oci://registry/charts/app')" spellcheck="false" class="field font-mono" />
          </label>
          <label class="grid gap-1">{{ tr('Versão do chart (opcional)') }}
            <input v-model="version" :placeholder="upgrading ? release!.chartVersion : 'mais recente'" spellcheck="false" class="field font-mono" />
          </label>
          <label class="grid gap-1">{{ tr('Nome do release') }}
            <input v-model="name" :disabled="upgrading" :placeholder="tr('meu-app')" spellcheck="false" class="field font-mono disabled:opacity-60" />
          </label>
          <label class="grid gap-1">{{ tr('Namespace') }}
            <input v-model="namespace" :disabled="upgrading" list="helm-ns" spellcheck="false" class="field font-mono disabled:opacity-60" />
            <datalist id="helm-ns"><option v-for="n in namespaces" :key="n" :value="n" /></datalist>
          </label>
          <div class="grid content-end gap-1.5 pb-1">
            <label v-if="!upgrading" class="flex items-center gap-1.5" :class="newNs && 'text-amber-300'">
              <input v-model="createNamespace" type="checkbox" :disabled="newNs" :checked="newNs || createNamespace" /> {{ tr('criar o namespace') }}{{ newNs ? tr(' (não existe)') : '' }}
            </label>
            <label class="flex items-center gap-1.5"><input v-model="wait" type="checkbox" /> {{ tr('esperar os recursos ficarem prontos') }}</label>
          </div>
        </div>
        <label class="grid gap-1">{{ tr('Values (YAML)') }}{{ upgrading ? tr(': substitui os values atuais do release') : '' }}
          <textarea v-model="values" spellcheck="false" rows="12" :placeholder="tr('replicaCount: 2')" class="field resize-y py-2 font-mono leading-relaxed" />
        </label>
        <label v-if="needsProdName" class="grid gap-1 text-rose-300">{{ tr('Cluster de produção: digite') }} <span class="font-mono text-zinc-100">{{ cluster }}</span> {{ tr('para confirmar') }}
          <input v-model="confirmProd" spellcheck="false" autocomplete="off" class="field font-mono" />
        </label>
        <p v-if="error" class="whitespace-pre-wrap text-rose-400 select-text">{{ error }}</p>
      </div>
      <div class="flex items-center gap-2 border-t border-zinc-800 px-4 py-3">
        <span v-if="busy" class="text-xs text-zinc-400">{{ wait ? tr('Aplicando e esperando os recursos…') : tr('Aplicando…') }}</span>
        <button class="ml-auto rounded-md bg-zinc-800 px-3 py-1.5 text-xs hover:bg-zinc-700" :disabled="busy" @click="emit('close')">{{ tr('Cancelar') }}</button>
        <button class="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50" :disabled="busy || !valid" @click="submit">
          {{ upgrading ? tr('Atualizar') : tr('Instalar') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";
.field { @apply h-8 rounded-md border border-zinc-800 bg-zinc-950 px-2 text-zinc-200 outline-none focus:border-sky-600; }
textarea.field { @apply h-auto; }
</style>
