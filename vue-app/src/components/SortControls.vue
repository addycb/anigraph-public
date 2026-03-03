<template>
  <div class="d-flex align-center" style="gap: 8px;">
    <v-select
      :model-value="sortBy"
      @update:model-value="$emit('update:sortBy', $event)"
      :items="sortOptions"
      label="Sort by"
      variant="outlined"
      density="compact"
      hide-details
      style="max-width: 150px;"
    ></v-select>
    <v-btn
      icon
      color="primary"
      variant="tonal"
      density="compact"
      @click="$emit('toggle-order')"
      :title="sortOrder === 'desc' ? 'Descending' : 'Ascending'"
    >
      <v-icon>{{ sortOrder === 'desc' ? 'mdi-arrow-down' : 'mdi-arrow-up' }}</v-icon>
    </v-btn>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  sortBy: string
  sortOrder: 'asc' | 'desc'
  sortOptions: Array<{ value: string; title: string }>
}>()

defineEmits<{
  'update:sortBy': [value: string]
  'toggle-order': []
}>()
</script>
