<template>
  <div v-if="hasFilters" class="mb-3">
    <div class="d-flex align-center justify-space-between mb-2">
      <h4 class="text-subtitle-2 text-medium-emphasis mb-0">Active Filters</h4>
      <v-btn
        variant="text"
        size="x-small"
        @click="$emit('clear-all')"
        prepend-icon="mdi-close-circle"
      >
        Clear All
      </v-btn>
    </div>
    <div class="mb-2">
      <slot></slot>
    </div>
    <p class="text-caption text-medium-emphasis">
      {{ summary }}
    </p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  hasFilters: boolean
  summary: string
}>()

defineEmits<{
  'clear-all': []
}>()
</script>
