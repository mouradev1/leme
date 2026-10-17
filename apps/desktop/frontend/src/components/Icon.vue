<script setup lang="ts">
import { icons } from '@/lib/icons'

defineProps<{ name: string; size?: number }>()
</script>

<template>
  <svg
    :width="size ?? 16" :height="size ?? 16" viewBox="0 0 16 16" fill="none" stroke="currentColor"
    stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round" class="shrink-0" aria-hidden="true"
  >
    <path :d="icons[name] ?? icons.Other" />
  </svg>
</template>
