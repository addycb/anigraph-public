<template>
  <v-card v-if="!loading && hasAnimeNodes" class="mt-4" variant="outlined">
    <v-card-title
      class="d-flex align-center justify-space-between text-subtitle-2"
    >
      <span>Connection Legend</span>
      <v-btn
        v-if="selectedLegendGroups.size > 0"
        size="x-small"
        variant="text"
        @click="clearLegendSelection"
      >
        Reset
      </v-btn>
    </v-card-title>
    <v-card-text>
      <div class="legend-grid">
        <div
          v-for="group in categoryGroups"
          :key="group.key"
          class="legend-item"
          :class="{
            'legend-item-active': activeLegendGroups.has(group.key),
            'legend-item-inactive': !activeLegendGroups.has(group.key),
            'legend-item-selected': selectedLegendGroups.has(group.key),
          }"
          @click="toggleLegendGroup(group.key)"
          @mouseenter="
            activeLegendGroups.has(group.key) &&
            $emit('hover', group.key)
          "
          @mouseleave="$emit('hover', null)"
        >
          <div
            class="legend-color"
            :style="{ backgroundColor: groupColors[group.key] }"
          ></div>
          <span class="legend-label">{{ group.title_en }}</span>
        </div>
        <div
          class="legend-item"
          :class="{
            'legend-item-active': activeLegendGroups.has('other'),
            'legend-item-inactive': !activeLegendGroups.has('other'),
            'legend-item-selected': selectedLegendGroups.has('other'),
          }"
          @click="toggleLegendGroup('other')"
          @mouseenter="
            activeLegendGroups.has('other') &&
            $emit('hover', 'other')
          "
          @mouseleave="$emit('hover', null)"
        >
          <div
            class="legend-color"
            :style="{ backgroundColor: groupColors.other }"
          ></div>
          <span class="legend-label">Other</span>
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<script setup lang="ts">
import type { CategoryGroup } from "@/utils/staffCategories";

const props = defineProps<{
  categoryGroups: CategoryGroup[];
  groupColors: Record<string, string>;
  activeLegendGroups: Set<string>;
  loading: boolean;
  hasAnimeNodes: boolean;
}>();

defineEmits<{
  hover: [groupKey: string | null];
}>();

const selectedLegendGroups = defineModel<Set<string>>("selectedLegendGroups", {
  required: true,
});

const toggleLegendGroup = (groupKey: string) => {
  if (!props.activeLegendGroups.has(groupKey)) return;
  const next = new Set(selectedLegendGroups.value);
  if (next.has(groupKey)) {
    next.delete(groupKey);
  } else {
    next.add(groupKey);
  }
  selectedLegendGroups.value = next;
};

const clearLegendSelection = () => {
  selectedLegendGroups.value = new Set();
};
</script>

<style scoped>
.legend-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  transition: opacity 0.25s ease;
}

.legend-item-active {
  cursor: pointer;
}

.legend-item-active:hover {
  opacity: 0.8;
}

.legend-item-inactive {
  opacity: 0.35;
  cursor: default;
}

.legend-color {
  width: 24px;
  height: 3px;
  border-radius: 2px;
}

.legend-item-active .legend-color {
  height: 4px;
}

.legend-label {
  font-size: 0.875rem;
  color: #666;
}

.legend-item-selected {
  background: rgba(var(--v-theme-primary), 0.1);
  border-radius: 4px;
  padding: 2px 6px;
  margin: -2px -6px;
}

.legend-item-selected .legend-color {
  height: 6px;
}

.legend-item-inactive .legend-label {
  color: #aaa;
}
</style>
