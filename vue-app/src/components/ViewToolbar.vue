<template>
  <div class="d-flex align-center mb-4 flex-wrap view-toolbar" style="gap: 12px;">
    <!-- Left Section: Custom content via slot -->
    <slot name="left"></slot>

    <!-- Middle Section: Sort controls (optional) -->
    <SortControls
      v-if="showSort"
      :sort-by="sortBy"
      :sort-order="sortOrder"
      :sort-options="sortOptions"
      @update:sort-by="$emit('update:sortBy', $event)"
      @toggle-order="$emit('toggle-sort-order')"
    />

    <v-spacer></v-spacer>

    <!-- Right Section: Custom content via slot -->
    <slot name="right"></slot>

    <!-- Adult content filter indicator -->
    <v-chip
      v-if="!includeAdult && !adultBannerDismissed"
      :to="'/settings'"
      size="small"
      color="warning"
      variant="tonal"
      prepend-icon="mdi-eye-off-outline"
      closable
      class="flex-shrink-0"
      @click:close.prevent="dismissAdultBanner"
    >
      Adult content hidden
    </v-chip>

    <!-- Year Markers Toggle (only when sorting by year) -->
    <v-switch
      v-if="showYearMarkers && sortBy === 'year'"
      :model-value="yearMarkersEnabled"
      @update:model-value="$emit('update:yearMarkersEnabled', $event)"
      color="primary"
      density="compact"
      hide-details
      label="Year Markers"
      class="flex-shrink-0"
    ></v-switch>

    <!-- Right Section: Card Size -->
    <CardSizeSelector
      v-if="cardSize !== undefined"
      :model-value="cardSize"
      @update:model-value="$emit('update:cardSize', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useSettings } from '~/composables/useSettings'

const { includeAdult } = useSettings()

const adultBannerDismissed = ref(false)

onMounted(() => {
  adultBannerDismissed.value = localStorage.getItem('adult_banner_dismissed') === 'true'
})

const dismissAdultBanner = () => {
  adultBannerDismissed.value = true
  localStorage.setItem('adult_banner_dismissed', 'true')
}

defineProps<{
  cardSize?: 'small' | 'medium' | 'large' | 'xlarge'
  sortBy?: string
  sortOrder?: 'asc' | 'desc'
  sortOptions?: Array<{ value: string; title: string }>
  showSort?: boolean
  showYearMarkers?: boolean
  yearMarkersEnabled?: boolean
}>()

defineEmits<{
  'update:cardSize': [value: 'small' | 'medium' | 'large' | 'xlarge']
  'update:sortBy': [value: string]
  'update:yearMarkersEnabled': [value: boolean]
  'toggle-sort-order': []
}>()
</script>

<style scoped>
.view-toolbar {
  padding: 16px;
  background: linear-gradient(135deg, rgba(var(--color-surface-rgb), 0.3) 0%, rgba(var(--color-bg-rgb), 0.5) 100%);
  backdrop-filter: blur(10px);
  border-radius: 12px;
  border: 1px solid var(--color-primary-muted);
}
</style>
