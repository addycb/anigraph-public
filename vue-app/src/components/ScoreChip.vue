<template>
  <!-- Compact variant: warning color, label shape, icon -->
  <v-chip
    v-if="props.score && props.styleVariant === 'compact'"
    color="warning"
    :size="props.size"
    label
  >
    <v-icon start :size="props.size === 'x-small' ? 'x-small' : 'small'">mdi-star</v-icon>
    {{ props.score }}
  </v-chip>

  <!-- Default variant: primary color, tonal, icon -->
  <v-chip
    v-else-if="props.score && props.styleVariant === 'default'"
    color="primary"
    :size="props.size"
    variant="tonal"
  >
    <v-icon start :size="props.size === 'x-small' ? 'x-small' : 'small'">mdi-star</v-icon>
    {{ props.score }}
  </v-chip>
</template>

<script setup lang="ts">
interface ScoreChipProps {
  score?: number
  // Style variant: 'compact' uses label variant with warning color and icon
  // 'default' uses tonal variant with primary color and emoji
  styleVariant?: 'compact' | 'default'
  size?: string
}

const props = withDefaults(defineProps<ScoreChipProps>(), {
  styleVariant: 'compact',
  size: 'x-small'
})
</script>

<style scoped>
:deep(.v-icon) {
  color: var(--color-score) !important;
  font-size: 14px !important;
  margin-right: 0 !important;
  -webkit-text-stroke: 0.5px rgba(0, 0, 0, 0.3);
  paint-order: stroke fill;
}
</style>
