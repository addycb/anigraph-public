<template>
  <div class="d-flex align-center" style="gap: 8px;">
    <span class="text-caption text-medium-emphasis">Size:</span>
    <v-btn-toggle
      :model-value="modelValue"
      @update:model-value="$emit('update:modelValue', $event)"
      mandatory
      color="primary"
      variant="outlined"
      density="compact"
    >
      <v-btn value="small" size="small">
        <v-icon size="small">mdi-view-grid-compact</v-icon>
      </v-btn>
      <v-btn value="medium" size="small">
        <v-icon size="small">mdi-view-grid</v-icon>
      </v-btn>
      <v-btn value="large" size="small">
        <v-icon size="small">mdi-view-comfy</v-icon>
      </v-btn>
      <v-btn value="xlarge" size="small">
        <v-icon size="small">mdi-view-agenda</v-icon>
      </v-btn>
    </v-btn-toggle>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: 'small' | 'medium' | 'large' | 'xlarge'
}>()

defineEmits<{
  'update:modelValue': [value: 'small' | 'medium' | 'large' | 'xlarge']
}>()
</script>
