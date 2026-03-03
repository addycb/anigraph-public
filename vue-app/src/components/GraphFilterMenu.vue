<template>
  <v-menu
    :close-on-content-click="false"
    location="bottom end"
    :attach="graphContainer"
    scroll-strategy="close"
  >
    <template v-slot:activator="{ props }">
      <v-btn
        icon
        size="small"
        v-bind="props"
        variant="elevated"
        color="white"
      >
        <v-icon>mdi-filter-variant</v-icon>
        <v-badge
          v-if="activeFilterCount > 0"
          :content="activeFilterCount"
          color="primary"
          floating
        ></v-badge>
      </v-btn>
    </template>
    <v-list
      density="compact"
      width="320"
      style="max-height: min(560px, 70vh); overflow-y: auto"
      class="graph-menu-list"
      @click.stop
    >
      <v-list-subheader>Filters</v-list-subheader>

      <!-- Staff Categories -->
      <v-list-item
        class="filter-list-header"
        style="cursor: pointer"
        @click="
          filterSectionsCollapsed.staffCategories =
            !filterSectionsCollapsed.staffCategories
        "
      >
        <div class="d-flex align-center gap-1">
          <v-icon size="small">{{
            filterSectionsCollapsed.staffCategories
              ? "mdi-chevron-right"
              : "mdi-chevron-down"
          }}</v-icon>
          <span>Staff Categories</span>
          <v-badge
            v-if="activeStaffCategoryFilterCount > 0"
            :content="activeStaffCategoryFilterCount"
            color="primary"
            inline
            class="ml-1"
          ></v-badge>
        </div>
        <template #append>
          <v-btn
            size="x-small"
            variant="text"
            @click.stop="toggleAllCategories"
            :disabled="filterSectionsCollapsed.staffCategories"
          >
            {{
              selectedCategories.length === allVisibleCategories.length
                ? "Clear All"
                : "Select All"
            }}
          </v-btn>
        </template>
      </v-list-item>
      <v-expand-transition>
        <div v-if="!filterSectionsCollapsed.staffCategories">
          <v-list-item class="pa-0 px-2">
            <div class="category-groups-list w-100">
              <template
                v-for="group in visibleCategoryGroups"
                :key="group.key"
              >
                <div class="category-group-row">
                  <div class="d-flex align-center w-100">
                    <v-checkbox
                      :model-value="isGroupFullySelected(group.key)"
                      :indeterminate="isGroupPartiallySelected(group.key)"
                      @click.stop="toggleGroupCategories(group.key)"
                      hide-details
                      density="compact"
                      class="flex-grow-0 mr-1"
                    ></v-checkbox>
                    <div
                      class="d-flex align-center flex-grow-1 cursor-pointer group-header-clickable"
                      @click="toggleFilterGroupExpanded(group.key)"
                    >
                      <div
                        class="group-color-indicator mr-2"
                        :style="{
                          backgroundColor: groupColors[group.key],
                        }"
                      ></div>
                      <span class="group-title">{{ group.title_en }}</span>
                      <v-spacer></v-spacer>
                      <span class="text-caption text-grey mr-2"
                        >{{ getGroupStaffCountFromGraph(group.key) }}
                        staff</span
                      >
                      <v-icon size="small">{{
                        expandedFilterGroups.has(group.key)
                          ? "mdi-chevron-up"
                          : "mdi-chevron-down"
                      }}</v-icon>
                    </div>
                  </div>
                  <v-expand-transition>
                    <div
                      v-if="expandedFilterGroups.has(group.key)"
                      class="child-categories-container pl-8 mt-1"
                    >
                      <div
                        v-for="childKey in getVisibleChildCategories(group.key)"
                        :key="childKey"
                        class="child-category-row d-flex align-center"
                      >
                        <v-checkbox
                          :model-value="
                            selectedCategories.includes(childKey)
                          "
                          @click.stop="toggleCategory(childKey)"
                          hide-details
                          density="compact"
                          class="flex-grow-0 mr-1"
                        ></v-checkbox>
                        <span class="child-category-title">{{
                          getCategoryTitleShort(childKey)
                        }}</span>
                      </div>
                    </div>
                  </v-expand-transition>
                </div>
              </template>
              <div class="category-group-row">
                <div class="d-flex align-center w-100">
                  <v-checkbox
                    :model-value="selectedCategories.includes('other')"
                    @click.stop="toggleCategory('other')"
                    hide-details
                    density="compact"
                    class="flex-grow-0 mr-1"
                  ></v-checkbox>
                  <div
                    class="group-color-indicator mr-2"
                    :style="{
                      backgroundColor: groupColors.other,
                    }"
                  ></div>
                  <span class="group-title">Other</span>
                  <v-spacer></v-spacer>
                  <span class="text-caption text-grey mr-2"
                    >{{ getOtherStaffCountFromGraph() }} staff</span
                  >
                </div>
              </div>
            </div>
          </v-list-item>
        </div>
      </v-expand-transition>

      <!-- Format -->
      <template v-if="availableFormats.length > 0">
        <v-divider class="my-2"></v-divider>
        <v-list-item
          class="filter-list-header"
          style="cursor: pointer"
          @click="
            filterSectionsCollapsed.format =
              !filterSectionsCollapsed.format
          "
        >
          <div class="d-flex align-center gap-1">
            <v-icon size="small">{{
              filterSectionsCollapsed.format
                ? "mdi-chevron-right"
                : "mdi-chevron-down"
            }}</v-icon>
            <span>Format</span>
            <v-badge
              v-if="activeFormatFilterCount > 0"
              :content="activeFormatFilterCount"
              color="primary"
              inline
              class="ml-1"
            ></v-badge>
          </div>
          <template #append>
            <v-btn
              size="x-small"
              variant="text"
              @click.stop="toggleAllFormats"
              :disabled="filterSectionsCollapsed.format"
            >
              {{
                selectedFormats.length === availableFormats.length
                  ? "Clear"
                  : "Select All"
              }}
            </v-btn>
          </template>
        </v-list-item>
        <v-expand-transition>
          <div v-if="!filterSectionsCollapsed.format">
            <v-list-item>
              <v-chip-group v-model="selectedFormats" multiple column>
                <v-chip
                  v-for="format in availableFormats"
                  :key="format"
                  :value="format"
                  color="primary"
                  filter
                  variant="outlined"
                  size="small"
                >
                  {{ formatAnimeFormat(format) }}
                </v-chip>
              </v-chip-group>
            </v-list-item>
          </div>
        </v-expand-transition>
      </template>

      <!-- Anime Filters (genres + tags) -->
      <template
        v-if="availableGenres.length > 0 || availableTags.length > 0"
      >
        <v-divider class="my-2"></v-divider>
        <v-list-item
          class="filter-list-header"
          style="cursor: pointer"
          @click="
            filterSectionsCollapsed.animeFilters =
              !filterSectionsCollapsed.animeFilters
          "
        >
          <div class="d-flex align-center gap-1">
            <v-icon size="small">{{
              filterSectionsCollapsed.animeFilters
                ? "mdi-chevron-right"
                : "mdi-chevron-down"
            }}</v-icon>
            <span>Anime Filters</span>
            <v-badge
              v-if="activeAnimeFilterCount > 0"
              :content="activeAnimeFilterCount"
              color="primary"
              inline
              class="ml-1"
            ></v-badge>
          </div>
          <template #append>
            <v-chip
              v-if="loadingRecommendationCounts"
              color="primary"
              size="x-small"
              variant="tonal"
            >
              <v-progress-circular
                indeterminate
                size="12"
                width="2"
                class="mr-1"
              ></v-progress-circular>
              Loading
            </v-chip>
          </template>
        </v-list-item>
        <v-expand-transition>
          <div v-if="!filterSectionsCollapsed.animeFilters">
            <v-list-subheader v-if="availableGenres.length > 0"
              >Genres</v-list-subheader
            >
            <v-list-item v-if="availableGenres.length > 0">
              <div class="d-flex flex-wrap ga-1">
                <v-chip
                  v-for="genre in availableGenres"
                  :key="genre"
                  :variant="
                    recommendationFilters.genres.includes(genre)
                      ? 'flat'
                      : 'outlined'
                  "
                  color="primary"
                  size="small"
                  @click="
                    recommendationFilters.genres.includes(genre)
                      ? $emit('removeGenreFilter', genre)
                      : $emit('addGenreFilter', genre)
                  "
                  :disabled="
                    loadingRecommendationCounts ||
                    recommendationFilterCounts.genres[genre] === 0
                  "
                >
                  {{ genre }}
                  <span
                    v-if="
                      !loadingRecommendationCounts &&
                      recommendationFilterCounts.genres[genre] !== undefined
                    "
                    class="ml-1 text-caption"
                  >
                    ({{ recommendationFilterCounts.genres[genre] }})
                  </span>
                </v-chip>
              </div>
            </v-list-item>
            <v-list-subheader v-if="availableTags.length > 0"
              >Tags</v-list-subheader
            >
            <v-list-item v-if="availableTags.length > 0">
              <div class="d-flex flex-wrap ga-1">
                <v-chip
                  v-for="tag in availableTags.slice(0, 20)"
                  :key="tag.name"
                  :variant="
                    recommendationFilters.tags.includes(tag.name)
                      ? 'flat'
                      : 'outlined'
                  "
                  color="primary"
                  size="small"
                  @click="
                    recommendationFilters.tags.includes(tag.name)
                      ? $emit('removeTagFilter', tag.name)
                      : $emit('addTagFilter', tag.name)
                  "
                  :disabled="
                    loadingRecommendationCounts ||
                    recommendationFilterCounts.tags[tag.name] === 0
                  "
                >
                  {{ tag.name }}
                  <span
                    v-if="
                      !loadingRecommendationCounts &&
                      recommendationFilterCounts.tags[tag.name] !== undefined
                    "
                    class="ml-1 text-caption"
                  >
                    ({{ recommendationFilterCounts.tags[tag.name] }})
                  </span>
                </v-chip>
              </div>
            </v-list-item>
            <v-list-item v-if="hasRecommendationFilters">
              <v-btn
                variant="outlined"
                size="small"
                @click="$emit('clearRecommendationFilters')"
                block
              >
                Clear Anime Filters
              </v-btn>
            </v-list-item>
          </div>
        </v-expand-transition>
      </template>
    </v-list>
  </v-menu>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { formatAnimeFormat } from "@/utils/formatters";
import {
  STAFF_CATEGORIES,
  CATEGORY_GROUPS,
  getCategoryByKey,
  getGroupByKey,
  type CategoryGroup,
} from "@/utils/staffCategories";
import type { GraphNode } from "@/types/graph";

const props = defineProps<{
  graphContainer: HTMLElement | null;
  categoryGroups: CategoryGroup[];
  groupColors: Record<string, string>;
  categoryColors: Record<string, string>;
  availableGenres: string[];
  availableTags: Array<{ name: string }>;
  availableFormats: string[];
  graphData: { nodes: GraphNode[] } | null;
  hideProducers: boolean;
  recommendationFilters: { genres: string[]; tags: string[] };
  recommendationFilterCounts: any;
  loadingRecommendationCounts: boolean;
  hasRecommendationFilters: boolean;
  activeFilterCount: number;
  activeStaffCategoryFilterCount: number;
  activeFormatFilterCount: number;
  activeAnimeFilterCount: number;
}>();

defineEmits<{
  addGenreFilter: [genre: string];
  removeGenreFilter: [genre: string];
  addTagFilter: [tag: string];
  removeTagFilter: [tag: string];
  clearRecommendationFilters: [];
}>();

const selectedCategories = defineModel<string[]>("selectedCategories", {
  required: true,
});
const selectedFormats = defineModel<string[]>("selectedFormats", {
  required: true,
});

const expandedFilterGroups = ref<Set<string>>(new Set());
const filterSectionsCollapsed = ref({
  staffCategories: true,
  format: true,
  animeFilters: true,
});

const staffCategories = STAFF_CATEGORIES;

const allVisibleCategories = computed(() => {
  const categories = [
    ...staffCategories
      .filter((c) => c.key !== "production")
      .map((cat) => cat.key),
  ];
  if (!props.hideProducers) {
    categories.push("production");
  }
  categories.push("other");
  return categories;
});

const visibleCategoryGroups = computed(() => {
  return props.categoryGroups.filter((group) => {
    if (props.hideProducers && group.key === "production_group") return false;
    return true;
  });
});

const getVisibleChildCategories = (groupKey: string): string[] => {
  const group = getGroupByKey(groupKey);
  if (!group) return [];
  if (props.hideProducers && groupKey === "production_group") return [];
  return group.children;
};

const getCategoryTitleShort = (categoryKey: string): string => {
  if (categoryKey === "other") return "Other";
  const category = getCategoryByKey(categoryKey);
  return category ? category.title_en : categoryKey;
};

const isGroupFullySelected = (groupKey: string): boolean => {
  const children = getVisibleChildCategories(groupKey);
  if (children.length === 0) return false;
  return children.every((childKey) =>
    selectedCategories.value.includes(childKey),
  );
};

const isGroupPartiallySelected = (groupKey: string): boolean => {
  const children = getVisibleChildCategories(groupKey);
  if (children.length === 0) return false;
  const selectedCount = children.filter((childKey) =>
    selectedCategories.value.includes(childKey),
  ).length;
  return selectedCount > 0 && selectedCount < children.length;
};

const toggleGroupCategories = (groupKey: string) => {
  const children = getVisibleChildCategories(groupKey);
  if (children.length === 0) return;

  if (isGroupFullySelected(groupKey)) {
    selectedCategories.value = selectedCategories.value.filter(
      (key) => !children.includes(key),
    );
  } else {
    const newSelected = new Set(selectedCategories.value);
    children.forEach((childKey) => newSelected.add(childKey));
    selectedCategories.value = Array.from(newSelected);
  }
};

const toggleCategory = (categoryKey: string) => {
  if (selectedCategories.value.includes(categoryKey)) {
    selectedCategories.value = selectedCategories.value.filter(
      (key) => key !== categoryKey,
    );
  } else {
    selectedCategories.value = [...selectedCategories.value, categoryKey];
  }
};

const toggleFilterGroupExpanded = (groupKey: string) => {
  if (expandedFilterGroups.value.has(groupKey)) {
    expandedFilterGroups.value.delete(groupKey);
  } else {
    expandedFilterGroups.value.add(groupKey);
  }
  expandedFilterGroups.value = new Set(expandedFilterGroups.value);
};

const toggleAllCategories = () => {
  if (selectedCategories.value.length === allVisibleCategories.value.length) {
    selectedCategories.value = [];
  } else {
    selectedCategories.value = [...allVisibleCategories.value];
  }
};

const toggleAllFormats = () => {
  if (selectedFormats.value.length === props.availableFormats.length) {
    selectedFormats.value = [];
  } else {
    selectedFormats.value = [...props.availableFormats];
  }
};

const getGroupStaffCountFromGraph = (groupKey: string): number => {
  if (!props.graphData) return 0;
  const group = getGroupByKey(groupKey);
  if (!group) return 0;

  return props.graphData.nodes.filter((node) => {
    if (node.type !== "staff") return false;
    const nodeCategory = node.category || "other";
    return group.children.includes(nodeCategory);
  }).length;
};

const getOtherStaffCountFromGraph = (): number => {
  if (!props.graphData) return 0;
  return props.graphData.nodes.filter(
    (node) =>
      node.type === "staff" && (node.category === "other" || !node.category),
  ).length;
};
</script>

<style scoped>
.filter-list-header {
  font-weight: 600;
  min-height: 36px;
}

.cursor-pointer {
  cursor: pointer;
}

.category-groups-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.category-group-row {
  padding: 4px 0;
  border-bottom: 1px solid var(--color-primary-faint);
}

.category-group-row:last-child {
  border-bottom: none;
}

.group-header-clickable {
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.group-header-clickable:hover {
  background-color: var(--color-primary-faint);
}

.group-color-indicator {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  flex-shrink: 0;
}

.group-title {
  font-weight: 500;
  color: var(--color-text);
}

.child-categories-container {
  border-left: 2px solid var(--color-primary-medium);
  margin-left: 6px;
  padding-left: 8px;
}

.child-category-row {
  padding: 2px 0;
}

.child-category-title {
  font-size: 0.875rem;
  color: var(--color-text);
}

/* Show scrollbar by default on mobile for graph menu lists */
@media (max-width: 600px) {
  .graph-menu-list {
    overflow-y: scroll !important;
  }
}
</style>
