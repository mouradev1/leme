<script setup lang="ts">
/** Shows a dock tab's host element here (see lib/hosts.ts). */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { hostOf, parkHost } from '@/lib/hosts'

const props = defineProps<{ id: string }>()
const box = ref<HTMLElement | null>(null)

function attach(id: string, old?: string) {
  if (!box.value) return
  if (old) parkHost(old, box.value)
  box.value.appendChild(hostOf(id))
}
onMounted(() => attach(props.id))
watch(() => props.id, (id, old) => attach(id, old))
onBeforeUnmount(() => box.value && parkHost(props.id, box.value))
</script>

<template>
  <div ref="box" class="absolute inset-0" />
</template>
