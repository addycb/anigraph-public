<template>
  <div :class="sectionClass">
    <div class="d-flex align-center justify-space-between mb-2">
      <h4 class="text-subtitle-2 text-medium-emphasis">{{ title }}</h4>
      <v-btn
        v-if="items.length > limit"
        variant="text"
        size="small"
        color="primary"
        @click="expanded = !expanded"
      >
        {{ expanded ? 'Show Less' : `Show All (${items.length})` }}
      </v-btn>
    </div>
    <div>
      <FilterChip
        v-for="item in displayedItems"
        :key="item.name"
        :label="item.name"
        :count="getCount(item)"
        :selected="selectedItems.includes(item.name)"
        :disabled="loadingCounts || (hasActiveFilters && !selectedItems.includes(item.name) && filterCounts[item.name] === 0)"
        :size="chipSize"
        :variant="chipVariant"
        :color="chipColor"
        :chip-class="selectedItems.includes(item.name) ? selectedClass : ''"
        @click="$emit('toggle', item.name)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  title: string
  items: Array<{ name: string; count: number }>
  selectedItems: string[]
  filterCounts: Record<string, number>
  loadingCounts?: boolean
  hasActiveFilters?: boolean
  limit?: number
  chipSize?: string
  chipVariant?: string
  chipColor?: string
  selectedClass?: string
  sectionClass?: string
}>()

defineEmits<{
  'toggle': [name: string]
}>()

const expanded = ref(false)

const displayedItems = computed(() => {
  const lim = props.limit ?? 10
  return expanded.value ? props.items : props.items.slice(0, lim)
})

const getCount = (item: { name: string; count: number }) => {
  if (props.filterCounts[item.name] !== undefined) {
    return props.filterCounts[item.name]
  }
  return item.count
}
</script>
