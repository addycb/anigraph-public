<template>
  <v-chip
    :size="size"
    :variant="variant"
    :color="color"
    :class="chipClass"
    class="mr-1 mb-1 cursor-pointer"
    :disabled="disabled"
    @click="$emit('click')"
  >
    <v-icon v-if="selected" start size="small">mdi-check</v-icon>
    {{ label }}
    <span v-if="count !== undefined" class="ml-1">
      ({{ count }})
    </span>
  </v-chip>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  count?: number
  selected?: boolean
  disabled?: boolean
  size?: string
  variant?: string
  color?: string
  chipClass?: string
}>()

defineEmits<{
  'click': []
}>()
</script>

<style scoped>
.cursor-pointer {
  cursor: pointer;
}
</style>
