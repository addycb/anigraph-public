<template>
  <div class="graph-container">
    <!-- Connection Graph / Staff Collapsible -->
    <div class="graph-panel-wrapper" ref="graphContainer">
      <v-expansion-panels v-model="graphOpen" class="graph-panel">
        <v-expansion-panel eager>
          <v-expansion-panel-title>
            <div
              class="d-flex justify-space-between align-center"
              style="width: 100%"
            >
              <span class="text-h5">{{
                graphViewMode === "graph" ? "Connection Graph" : "Staff"
              }}</span>
              <div class="d-flex align-center" style="gap: 12px">
                <!-- View Mode Toggle (Graph/Staff) -->
                <v-btn-toggle
                  v-model="graphViewMode"
                  mandatory
                  dense
                  @click.stop
                  :disabled="graphOpen !== 0"
                >
                  <v-tooltip
                    text="Connection Graph"
                    location="bottom"
                    v-model="graphViewTooltip"
                    :attach="graphContainer"
                  >
                    <template v-slot:activator="{ props }">
                      <v-btn value="graph" size="small" v-bind="props">
                        <v-icon>mdi-graph</v-icon>
                      </v-btn>
                    </template>
                  </v-tooltip>
                  <v-tooltip
                    text="Staff List"
                    location="bottom"
                    v-model="staffViewTooltip"
                    :attach="graphContainer"
                  >
                    <template v-slot:activator="{ props }">
                      <v-btn value="staff" size="small" v-bind="props">
                        <v-icon>mdi-account-group</v-icon>
                      </v-btn>
                    </template>
                  </v-tooltip>
                </v-btn-toggle>
              </div>
            </div>
          </v-expansion-panel-title>
          <v-expansion-panel-text>
            <!-- Graph View -->
            <div v-if="graphViewMode === 'graph'" class="graph-view-content">
              <div class="graph-wrapper">
                <div ref="graphRef" class="graph-svg-container"></div>

                <!-- Edge Label (fixed position) -->
                <transition name="fade">
                  <div
                    v-if="hoveredEdgeLabel || hoveredEdgeSingleStaff"
                    class="graph-edge-label"
                    :style="{
                      borderColor: hoveredEdgeCategory
                        ? categoryColors[hoveredEdgeCategory]
                        : 'var(--color-primary)',
                    }"
                  >
                    <template v-if="hoveredEdgeSingleStaff">
                      <div class="edge-label-staff-name">
                        {{ hoveredEdgeSingleStaff.staffName }}
                      </div>
                      <div class="edge-label-two-tier">
                        <span class="edge-label-role">{{
                          hoveredEdgeSingleStaff.mainRole
                        }}</span>
                        <v-icon size="x-small" class="edge-label-arrow"
                          >mdi-arrow-down</v-icon
                        >
                        <span class="edge-label-role">{{
                          hoveredEdgeSingleStaff.otherRole
                        }}</span>
                      </div>
                    </template>
                    <template v-else>
                      {{ hoveredEdgeLabel }}
                    </template>
                  </div>
                </transition>

                <!-- Hover Preview for Anime Nodes (desktop only) -->
                <transition name="fade-hover">
                  <div
                    v-if="hoveredAnimeNode && !isMobile"
                    class="anime-hover-preview"
                    :style="{
                      left: hoveredAnimeNode.x + 'px',
                      top: hoveredAnimeNode.y + 'px',
                    }"
                  >
                    <AnimePreviewCard :anime="hoveredAnimeNode.node" />
                  </div>
                </transition>

                <!-- Hover Preview for Staff Nodes (desktop only) -->
                <transition name="fade-hover">
                  <div
                    v-if="hoveredStaffNode && !isMobile"
                    class="anime-hover-preview"
                    :style="{
                      left: hoveredStaffNode.x + 'px',
                      top: hoveredStaffNode.y + 'px',
                    }"
                  >
                    <StaffPreviewCard
                      :staff="hoveredStaffNode.node"
                      :roles="hoveredStaffNode.roles"
                      :category-color="
                        categoryColors[
                          hoveredStaffNode.node.category || 'other'
                        ]
                      "
                      :primary-occupations="
                        staffOccupationsMap.get(
                          String(hoveredStaffNode.node.id),
                        ) || []
                      "
                    />
                  </div>
                </transition>

                <!-- Graph Controls (collapsible corner overlay) -->
                <div class="graph-settings-menu">
                  <!-- Collapsible group: Filters | Fullscreen | Settings -->
                  <!-- Rendered BEFORE the toggle so the toggle stays pinned to the right edge -->
                  <transition name="graph-controls-fade">
                    <div
                      v-if="!graphControlsCollapsed"
                      class="graph-controls-collapsible"
                    >
                      <!-- Filters Button -->
                      <GraphFilterMenu
                        :graph-container="graphContainer"
                        :category-groups="categoryGroups"
                        :group-colors="groupColors"
                        :category-colors="categoryColors"
                        :available-genres="availableGenres"
                        :available-tags="availableTags"
                        :available-formats="availableFormats"
                        :graph-data="graphData"
                        :hide-producers="hideProducers"
                        :recommendation-filters="recommendationFilters"
                        :recommendation-filter-counts="recommendationFilterCounts"
                        :loading-recommendation-counts="loadingRecommendationCounts"
                        :has-recommendation-filters="hasRecommendationFilters"
                        :active-filter-count="activeFilterCount"
                        :active-staff-category-filter-count="activeStaffCategoryFilterCount"
                        :active-format-filter-count="activeFormatFilterCount"
                        :active-anime-filter-count="activeAnimeFilterCount"
                        v-model:selected-categories="selectedCategories"
                        v-model:selected-formats="selectedFormats"
                        @add-genre-filter="addGenreFilter"
                        @remove-genre-filter="removeGenreFilter"
                        @add-tag-filter="addTagFilter"
                        @remove-tag-filter="removeTagFilter"
                        @clear-recommendation-filters="clearRecommendationFilters"
                      />

                      <!-- Fullscreen button -->
                      <v-btn
                        v-if="!isMobile"
                        icon
                        size="small"
                        variant="elevated"
                        color="white"
                        @click="toggleFullscreen"
                        :disabled="loading"
                      >
                        <v-icon>{{
                          isFullscreen
                            ? "mdi-fullscreen-exit"
                            : "mdi-fullscreen"
                        }}</v-icon>
                      </v-btn>

                      <!-- Settings gear -->
                      <GraphSettingsMenu
                        :graph-container="graphContainer"
                        :min-connections-options="minConnectionsOptions"
                        v-model:min-connections="minConnections"
                        v-model:graph-sort-mode="graphSortMode"
                        v-model:graph-sort-order="graphSortOrder"
                        v-model:use-category-nodes="useCategoryNodes"
                        v-model:hide-staff-nodes="hideStaffNodes"
                        v-model:same-role-only="sameRoleOnly"
                        v-model:hide-producers="hideProducers"
                        v-model:hide-lonely-staff="hideLonelyStaff"
                        v-model:hide-related-anime="hideRelatedAnime"
                        v-model:show-favorites="showFavorites"
                        v-model:show-favorite-icon="showFavoriteIcon"
                        v-model:hover-highlight="hoverHighlight"
                        v-model:hover-dimmed-edges="hoverDimmedEdges"
                      />
                    </div>
                  </transition>

                  <!-- Collapse/expand toggle — always last (rightmost = pinned to corner) -->
                  <v-btn
                    icon
                    size="small"
                    variant="elevated"
                    color="white"
                    @click="graphControlsCollapsed = !graphControlsCollapsed"
                  >
                    <v-icon>{{
                      graphControlsCollapsed
                        ? "mdi-chevron-left"
                        : "mdi-chevron-right"
                    }}</v-icon>
                  </v-btn>
                </div>

                <AnimeDetailOverlay
                  :selected-anime="selectedAnime"
                  :main-anime-data="mainAnimeData"
                  :category-colors="categoryColors"
                  :group-colors="groupColors"
                  :pinned-category="pinnedCategory"
                  :is-favorited="isFavorited"
                  @close="closeOverlay"
                  @toggle-favorite="toggleFavorite"
                />
              </div>
              <div v-if="loading" class="graph-loading">
                <v-progress-circular
                  indeterminate
                  color="primary"
                  size="64"
                ></v-progress-circular>
                <p class="mt-4">Loading graph data...</p>
              </div>

              <!-- Empty state for no anime found -->
              <div
                v-if="!loading && !hasAnimeNodes"
                class="graph-empty-state"
                style="pointer-events: none"
              >
                <v-icon size="64" color="grey-lighten-1"
                  >mdi-chart-bubble</v-icon
                >
                <p class="text-h6 mt-4">
                  No anime found from staff connections
                </p>
              </div>

              <GraphColorLegend
                :category-groups="categoryGroups"
                :group-colors="groupColors"
                :active-legend-groups="activeLegendGroups"
                :loading="loading"
                :has-anime-nodes="hasAnimeNodes"
                v-model:selected-legend-groups="selectedLegendGroups"
                @hover="hoveredLegendGroup = $event"
              />
            </div>

            <StaffListView
              v-if="graphViewMode === 'staff'"
              :staff="staff"
              :category-groups="categoryGroups"
              :categorized-staff="categorizedStaff"
              :uncategorized-staff="uncategorizedStaff"
              :group-colors="groupColors"
              :hide-producers="hideProducers"
              :graph-view-mode="graphViewMode"
              :category-staff-ids-map="categoryStaffIdsMap"
              :group-staff-ids-map="groupStaffIdsMap"
              :focus-category="staffFocusCategory"
              v-model:selected-staff="selectedStaff"
              @staff-changed="onStaffChanged"
            />
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </div>

    <!-- Similar Works Recommendations -->
    <v-expansion-panels class="mt-4 recommendations-section" v-model="recOpen">
      <v-expansion-panel>
        <v-expansion-panel-title>
          <div class="d-flex align-center gap-2">
            <span class="text-h6">Similar Works</span>
            <v-select
              v-model="recommendationMode"
              :items="[
                { value: 'filtered', title: 'Shared staff (graph)' },
                { value: 'all', title: 'Shared staff (all)' },
                { value: 'recommended', title: 'Recommended' },
              ]"
              label="Mode"
              name="recommendation-mode"
              density="compact"
              variant="outlined"
              hide-details
              class="recommendation-mode-select"
              @click.stop
            ></v-select>
          </div>
        </v-expansion-panel-title>
        <v-expansion-panel-text>
          <!-- Loading state for recommended mode (initial load only, not page transitions) -->
          <v-container
            v-if="
              recommendationMode === 'recommended' &&
              recommendedLoading &&
              recommendedAnime.length === 0
            "
            class="text-center py-8"
          >
            <v-progress-circular
              indeterminate
              color="primary"
              size="48"
            ></v-progress-circular>
            <p class="text-body-2 mt-3">Loading recommendations...</p>
          </v-container>

          <!-- Empty state for recommended mode -->
          <v-container
            v-else-if="
              recommendationMode === 'recommended' &&
              !recommendedLoading &&
              recommendedAnime.length === 0
            "
            class="text-center py-8"
          >
            <v-icon size="48" color="grey-lighten-1"
              >mdi-information-outline</v-icon
            >
            <p class="text-body-1 mt-3 text-grey">
              No recommendations available for this anime.
            </p>
            <p class="text-body-2 mt-2 text-grey">
              Try exploring anime from the same studio to find similar content.
            </p>
          </v-container>

          <!-- Empty state for staff-based modes -->
          <v-container
            v-else-if="
              recommendationMode !== 'recommended' &&
              recommendations.length === 0
            "
            class="text-center py-8"
          >
            <v-icon size="48" color="grey-lighten-1"
              >mdi-account-group-outline</v-icon
            >
            <p class="text-body-1 mt-3 text-grey">
              No similar works found based on shared staff.
            </p>
            <p class="text-body-2 mt-2 text-grey">
              We find similar works by looking at staff members who worked on
              this anime and their other projects. Our information for this
              anime doesn't yet meet these requirements.
            </p>
            <p class="text-body-2 mt-2 text-grey">
              Try the "Recommended" tab or explore anime from the same studio to
              find similar content.
            </p>
          </v-container>

          <!-- Recommendations grid -->
          <v-row v-else>
            <v-col
              v-for="rec in paginatedRecommendations"
              :key="rec.anilistId"
              cols="6"
              sm="4"
              md="3"
              lg="2"
            >
              <AnimeCard
                :anime="{
                  anilistId: rec.anilistId,
                  title: rec.title,
                  coverImage: rec.coverImage,
                  averageScore: rec.averageScore,
                  format: rec.format,
                  seasonYear: rec.seasonYear,
                  staffCount:
                    recommendationMode !== 'recommended'
                      ? rec.staffCount
                      : undefined,
                  description: rec.description,
                }"
                :staff-by-role="
                  recommendationMode !== 'recommended'
                    ? getStaffByRole(rec.categoryBreakdown)
                    : undefined
                "
              />
            </v-col>

            <!-- Invisible placeholders to maintain grid size on partial pages -->
            <v-col
              v-for="i in itemsPerPage - paginatedRecommendations.length"
              :key="`placeholder-${i}`"
              cols="6"
              sm="4"
              md="3"
              lg="2"
              style="visibility: hidden"
            >
              <v-card class="recommendation-card">
                <v-img aspect-ratio="0.7" cover></v-img>
                <v-card-title class="text-body-2 pa-2">&nbsp;</v-card-title>
              </v-card>
            </v-col>
          </v-row>
          <div v-if="totalPages > 1" class="d-flex justify-center mt-4">
            <v-pagination
              v-model="currentPage"
              :length="totalPages"
              :total-visible="5"
            ></v-pagination>
          </div>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
  </div>
</template>

<script setup lang="ts">
import {
  ref,
  shallowRef,
  onMounted,
  watch,
  onBeforeUnmount,
  computed,
  nextTick,
} from "vue";
import {
  select,
  zoom as d3Zoom,
  zoomTransform,
  zoomIdentity,
  drag,
  transition,
} from "d3";
import type { Selection, Simulation } from "d3";
import { useRouter, RouterLink } from "vue-router";
import { api } from "@/utils/api";
import { formatAnimeFormat } from "@/utils/formatters";
import {
  STAFF_CATEGORIES,
  CATEGORY_GROUPS,
  CATEGORY_TO_GROUP,
  categorizeStaff,
  categorizeRole,
  getCategoryTitle as getCategoryTitleUtil,
  getGroupTitle as getGroupTitleUtil,
  getCategoryByKey,
  getGroupByKey,
  type CategoryGroup,
} from "@/utils/staffCategories";
import type { GraphNode, GraphLink, GraphData } from "@/types/graph";
import GraphFilterMenu from "@/components/GraphFilterMenu.vue";
import GraphSettingsMenu from "@/components/GraphSettingsMenu.vue";
import AnimeDetailOverlay from "@/components/AnimeDetailOverlay.vue";
import GraphColorLegend from "@/components/GraphColorLegend.vue";
import StaffListView from "@/components/StaffListView.vue";
import { useGraphRecommendations } from "@/composables/useGraphRecommendations";

const props = defineProps<{
  animeId: string;
  staff?: any[];
  relations?: any[];
  initialGraphData?: GraphData | null;
  animeData?: any | null;
}>();

const { getUserId, isAuthenticated } = useAuth();
const { requireLogin } = useLoginRequired();
const { fetchFavorites, favoritedAnimeIds, isFavorited, toggleFavorite } =
  useFavorites();
const route = useRoute();
const router = useRouter();

// Wrapper that toggles favorite state AND patches the d3 heart immediately
const handleToggleFavorite = async (animeId: number | string) => {
  if (!isAuthenticated.value) {
    requireLogin();
    return;
  }

  const id = typeof animeId === "string" ? parseInt(animeId) : animeId;
  const wasFavorited = isFavorited(id);
  const success = await toggleFavorite(id);
  if (!success) return;

  if (!graphRef.value || !showFavoriteIcon.value) return;
  select(graphRef.value)
    .selectAll("g")
    .filter(
      (d: any) => d && d.type === "anime" && parseInt(String(d.id)) === id,
    )
    .each(function () {
      const nodeG = select(this);
      if (wasFavorited) {
        nodeG.select(".favorite-heart").remove();
      } else {
        appendFavoriteHeart(nodeG);
      }
    });
};

const graphRef = ref<HTMLElement | null>(null);
const loading = ref(true);
const graphData = shallowRef<GraphData | null>(null);

// Shared node lookup map from graphData — avoids rebuilding in every function
const graphNodeMap = computed(() => {
  if (!graphData.value) return new Map<string | number, GraphNode>();
  const map = new Map<string | number, GraphNode>();
  graphData.value.nodes.forEach((node) => map.set(node.id, node));
  return map;
});
const selectedNode = ref<GraphNode | null>(null);
const hideProducers = ref(true);
const hideLonelyStaff = ref(false);
const hideRelatedAnime = ref(false);
const showFavorites = ref(true);
const showFavoriteIcon = ref(true);
const useCategoryNodes = ref(false); // Category aggregation mode
const hideStaffNodes = ref(false); // Anime-only mode: hide staff/category nodes
const hoverHighlight = ref(false); // Dim unrelated nodes/edges on hover
const hoverDimmedEdges = ref(false); // Allow hovering edges dimmed by legend selection
const graphSortMode = ref<"connections" | "title" | "rating">("connections");
const graphSortOrder = ref<"asc" | "desc">("desc");
const categoryNodesThreshold = 15; // Auto-enable category nodes when staff count exceeds this
const selectedCategories = ref<string[]>([
  ...STAFF_CATEGORIES.filter((cat) => cat.key !== "production").map(
    (cat) => cat.key,
  ),
  "other",
]);

const selectedStaff = ref<Set<string>>(new Set()); // Track individual staff selection
const selectedFormats = ref<string[]>([]); // Empty array means all formats selected
const staffCategories = STAFF_CATEGORIES;
const categoryGroups = CATEGORY_GROUPS;
const isSyncingFilters = ref(false); // Prevent infinite sync loops
const isFirstLoad = ref(true); // Track if this is the first load to auto-adjust filters
const isInitialLoad = ref(true); // Prevent watchers from firing during initial data load

// State for Staff View - two-tier expansion panels
const groupsOpen = ref<string[]>([]);
const childCategoriesOpen = ref<Record<string, string[]>>({});

// Initialize childCategoriesOpen for each group
CATEGORY_GROUPS.forEach((group) => {
  childCategoriesOpen.value[group.key] = [];
});

// Overlay state for anime detail view
const selectedAnime = ref<any>(null);
const mainAnimeData = shallowRef<any>(null);
const isLoadingMainAnime = ref(true);

// Hover preview state for anime nodes
const hoveredAnimeNode = ref<{ node: GraphNode; x: number; y: number } | null>(
  null,
);

// Hover preview state for staff nodes
const hoveredStaffNode = ref<{
  node: GraphNode;
  x: number;
  y: number;
  roles: string[];
} | null>(null);

// Shared Staff view controls
const expandedRoleCards = ref<Set<string>>(new Set()); // Track which role cards are expanded
const pinnedCategory = ref<string | null>(null); // Category to pin to top of shared staff list (set by edge click)
const staffFocusCategory = ref<string | null>(null); // Category to focus in StaffListView (set by center-category edge click)

// Color mapping for parent groups (children inherit from parent)
const groupColors: Record<string, string> = {
  direction: "#1976d2", // Blue - Direction
  writing_story: "#9c27b0", // Purple - Writing & Story
  design: "#e91e63", // Pink - Design
  music_op_ed: "#ffc107", // Amber - Music & OP/ED
  animation: "#4caf50", // Green - Animation
  art_color: "#795548", // Brown - Art & Color
  post_production: "#009688", // Teal - Post-Production
  sound: "#00bcd4", // Cyan - Sound
  production_group: "#607d8b", // Blue Grey - Production
  other: "#9e9e9e", // Grey - Other
};

// Build categoryColors by mapping each detailed category to its parent group color
const categoryColors: Record<string, string> = (() => {
  const colors: Record<string, string> = { other: groupColors.other };
  STAFF_CATEGORIES.forEach((cat) => {
    const parentGroup = CATEGORY_TO_GROUP[cat.key] || "other";
    colors[cat.key] = groupColors[parentGroup] || groupColors.other;
  });
  return colors;
})();

// Filtered data as ref instead of computed to break synchronous dependency
const filteredData = ref<{
  nodes: GraphNode[];
  links: GraphLink[];
  center: string;
} | null>(null);

// Genre/Tag filtering for recommendations

// Use filter metadata composable
const {
  filterMetadataLoaded,
  loadingFilterMetadata,
  loadFilterMetadata,
  calculateFilterCounts: calculateFilterCountsFromComposable,
  getFilteredMetadata,
  useBitmaps,
  lookupTables,
} = useFilterMetadata();

// Shared metadata map for O(1) lookup when iterating graph nodes
const graphAnimeMetadataMap = computed(() => {
  if (!filterMetadataLoaded.value) return new Map<number, any>();
  const metadata = getFilteredMetadata(parseInt(props.animeId));
  return new Map(metadata.map((a: any) => [a.id, a]));
});

// Anime node IDs before genre/tag filtering — used to compute stable available genres/tags
const preGenreTagAnimeNodeIds = ref<Set<number>>(new Set());

// Adult content filtering for graph filter options
const { includeAdult } = useSettings();

// Recommendation logic (composable)
const {
  recommendationMode,
  currentPage,
  itemsPerPage,
  recommendedAnime,
  recommendedLoading,
  recommendedPagination,
  recommendationFilters,
  recommendationFilterCounts,
  loadingRecommendationCounts,
  filteredRecommendations,
  allRecommendations,
  recommendations,
  filteredRecommendationsByGenreTags,
  paginatedRecommendations,
  totalPages,
  hasRecommendations,
  hasRecommendationFilters,
  availableGenres,
  availableTags,
  computeRecommendationsFromData,
  getStaffByRole,
  getSortedCategories,
  fetchRecommendedAnime,
  fetchRecommendationFilterCounts,
  addGenreFilter,
  removeGenreFilter,
  addTagFilter,
  removeTagFilter,
  clearRecommendationFilters,
  resetState: recReset,
} = useGraphRecommendations({
  animeId: () => props.animeId,
  graphData: () => graphData.value,
  filteredData: () => filteredData.value,
  filterMetadataLoaded: () => filterMetadataLoaded.value,
  loadingFilterMetadata: () => loadingFilterMetadata.value,
  loadFilterMetadata,
  getFilteredMetadata,
  useBitmaps: () => useBitmaps.value,
  lookupTables: () => lookupTables.value,
  preGenreTagAnimeNodeIds: () => preGenreTagAnimeNodeIds.value,
  categoryColors,
  includeAdult: () => includeAdult.value,
  graphAnimeMetadataMap: () => graphAnimeMetadataMap.value,
  isInitialLoad: () => isInitialLoad.value,
});


// Expansion panel states - panels start open
const graphOpen = ref(0);
const recOpen = ref(0);

// View toggle for Connection Graph / Staff
const graphViewMode = ref<"graph" | "staff">("graph");

// Filter options
const sameRoleOnly = ref(true);

const minConnections = ref(2);
const filteredAnimeConnectionCounts = ref<Map<string | number, number>>(
  new Map(),
);
// Computed options for minConnections based on graph data
const minConnectionsOptions = computed(() => {
  const floor = graphData.value?.minConnectionsFloor ?? 2;

  if (!graphData.value) {
    const options = [];
    for (let i = floor; i <= 20; i++) {
      options.push(i);
    }
    return options;
  }

  if (filteredAnimeConnectionCounts.value.size === 0) {
    return [floor];
  }

  const maxConnections = Math.max(
    ...Array.from(filteredAnimeConnectionCounts.value.values()),
    floor,
  );

  const options = [];
  for (let i = floor; i <= maxConnections; i++) {
    options.push(i);
  }
  return options;
});

// Edge label hover state
const hoveredEdgeLabel = ref<string | null>(null);
const hoveredEdgeCategory = ref<string | null>(null);
const hoveredEdgeSingleStaff = ref<{
  staffName: string;
  mainRole: string;
  otherRole: string;
} | null>(null);
const selectedLegendGroups = ref<Set<string>>(new Set());
const hoveredLegendGroup = ref<string | null>(null);
const hoveredAnimeNodeId = ref<string | number | null>(null);

// Tooltip visibility controls for the toggle buttons
const graphViewTooltip = ref(false);
const staffViewTooltip = ref(false);

// Fullscreen state
const isFullscreen = ref(false);
const graphContainer = ref<HTMLElement | null>(null);
const isMobile = ref(false);

// Graph controls overlay state
const graphControlsCollapsed = ref(false);

// Per-section active filter counts
const activeStaffCategoryFilterCount = computed(() => {
  return allVisibleCategories.value.filter(
    (cat) => !selectedCategories.value.includes(cat),
  ).length;
});

const activeFormatFilterCount = computed(() => {
  if (
    selectedFormats.value.length > 0 &&
    selectedFormats.value.length < availableFormats.value.length
  ) {
    return selectedFormats.value.length;
  }
  return 0;
});

const activeAnimeFilterCount = computed(() => {
  return (
    recommendationFilters.value.genres.length +
    recommendationFilters.value.tags.length
  );
});

// Total count of active filter items for the Filters button badge
const activeFilterCount = computed(() => {
  return (
    activeStaffCategoryFilterCount.value +
    activeFormatFilterCount.value +
    activeAnimeFilterCount.value
  );
});

// Recommendation filter mode

// Append a favorite heart emoji to a d3 node group selection
const appendFavoriteHeart = (nodeG: Selection<any, any, any, any>) => {
  nodeG
    .append("text")
    .attr("class", "favorite-heart")
    .text("❤️")
    .attr("x", 18)
    .attr("y", -18)
    .attr("text-anchor", "middle")
    .attr("font-size", "16px")
    .style("filter", "drop-shadow(0 0 2px rgba(255, 255, 255, 0.8))")
    .style("pointer-events", "none");
};

// Calculate hover preview position, constraining to graph container bounds
const calcHoverPosition = (
  d: GraphNode,
  svgNode: SVGSVGElement,
  config: {
    cardWidth: number;
    cardHeight: number;
    nodeOffset: number;
    yOffset: number;
    minY?: number;
  },
) => {
  const transform = zoomTransform(svgNode);
  const screenX = d.x! * transform.k + transform.x;
  const screenY = d.y! * transform.k + transform.y;
  const containerWidth = graphRef.value?.clientWidth || 800;
  const containerHeight = graphRef.value?.clientHeight || 600;
  const minY = config.minY ?? 20;
  const maxY = containerHeight - config.cardHeight - 20;
  let x = screenX + config.nodeOffset;
  if (x + config.cardWidth + 20 > containerWidth) {
    x = screenX - config.nodeOffset - config.cardWidth;
  }
  const y = Math.max(minY, Math.min(maxY, screenY + config.yOffset));
  return { x, y };
};

const ANIME_HOVER_CONFIG = {
  cardWidth: 280,
  cardHeight: 450,
  nodeOffset: 40,
  yOffset: -100,
  minY: 245,
};
const STAFF_HOVER_CONFIG = {
  cardWidth: 220,
  cardHeight: 140,
  nodeOffset: 30,
  yOffset: -60,
};

// Generation counter — incremented on every fetchGraphData call so stale API
// responses (from a previous call) are silently discarded.
let fetchGeneration = 0;

let simulation: Simulation<GraphNode, GraphLink> | null = null;
let zoomBehaviorRef: any = null;
let svgSelectionRef: any = null;
let initialTransformRef: any = null;
const staffRolesMap = new Map<string | number, string[]>();

const CATEGORY_ORDER = [
  "direction",
  "writing",
  "character_design",
  "music",
  "key_animation",
  "art_direction",
  "sound_production",
  "photography_editing",
  "other_animation",
  "production",
  "other",
];

// Build a lookup map from staff prop to get primaryOccupations by staff_id
// Uses String keys to avoid type mismatches between graph node IDs and API staff_ids
const staffOccupationsMap = computed(() => {
  const map = new Map<string, string[]>();
  if (!props.staff) return map;
  props.staff.forEach((s: any) => {
    if (s.staff?.staff_id && s.staff.primaryOccupations?.length) {
      map.set(String(s.staff.staff_id), s.staff.primaryOccupations);
    }
  });
  return map;
});

// Single debounced render — all filter watchers funnel through here so that
// rapid changes (or a clamp from minConnectionsOptions) coalesce into one pass.
let renderTimeout: ReturnType<typeof setTimeout> | null = null;

// Fingerprint of the last rendered graph — skip re-render when filters don't change the graph.
// Uses a fast numeric hash (djb2) over node IDs and link endpoints to avoid
// allocating large intermediate strings on every filter change.
let lastRenderedFingerprint = 0;

const computeFilteredDataFingerprint = (
  data: typeof filteredData.value,
): number => {
  if (!data) return 0;
  // djb2-style hash seeded with node + link counts for fast early divergence
  let hash = 5381 * 33 + data.nodes.length;
  hash = (hash * 33 + data.links.length) | 0;

  for (let i = 0; i < data.nodes.length; i++) {
    const id = data.nodes[i].id;
    if (typeof id === "number") {
      hash = (hash * 33 + id) | 0;
    } else {
      for (let j = 0; j < id.length; j++) {
        hash = (hash * 33 + id.charCodeAt(j)) | 0;
      }
    }
  }

  for (let i = 0; i < data.links.length; i++) {
    const l = data.links[i];
    const s = typeof l.source === "object" ? l.source.id : l.source;
    const t = typeof l.target === "object" ? l.target.id : l.target;
    if (typeof s === "number") {
      hash = (hash * 33 + s) | 0;
    } else {
      for (let j = 0; j < s.length; j++) {
        hash = (hash * 33 + s.charCodeAt(j)) | 0;
      }
    }
    if (typeof t === "number") {
      hash = (hash * 33 + t) | 0;
    } else {
      for (let j = 0; j < t.length; j++) {
        hash = (hash * 33 + t.charCodeAt(j)) | 0;
      }
    }
  }

  return hash;
};

const renderGraphIfChanged = () => {
  const fp = computeFilteredDataFingerprint(filteredData.value);
  if (fp !== lastRenderedFingerprint) {
    console.log(
      "[GraphVis] renderGraphIfChanged: fingerprint changed, re-rendering. selectedFormats:",
      [...selectedFormats.value],
      "nodes:",
      filteredData.value?.nodes.length,
    );
    lastRenderedFingerprint = fp;
    renderGraph();
  } else {
    console.log(
      "[GraphVis] renderGraphIfChanged: fingerprint unchanged, skipping",
    );
  }
};

const scheduleRender = () => {
  if (renderTimeout) clearTimeout(renderTimeout);
  console.log("[GraphVis] scheduleRender: queued (450ms). selectedFormats:", [
    ...selectedFormats.value,
  ]);
  renderTimeout = setTimeout(() => {
    console.log(
      "[GraphVis] scheduleRender: executing. selectedFormats:",
      [...selectedFormats.value],
      "isInitialLoad:",
      isInitialLoad.value,
    );
    updateFilteredData();
    renderGraphIfChanged();
  }, 450);
};

// Shared function to compute recommendations from graph data


// Available formats from graph anime nodes
const availableFormats = computed(() => {
  if (!graphData.value) return [];

  // Get all unique formats from anime nodes (excluding center)
  const formatSet = new Set<string>();
  graphData.value.nodes.forEach((node) => {
    if (node.type === "anime" && node.format) {
      formatSet.add(node.format);
    }
  });

  return Array.from(formatSet).sort();
});


// Check if there are any anime nodes (excluding center node)
const hasAnimeNodes = computed(() => {
  if (!filteredData.value) return false;

  return filteredData.value.nodes.some(
    (node) => node.type === "anime" && node.id !== filteredData.value!.center,
  );
});

// Which legend groups have at least one visible edge in the current filtered graph
const activeLegendGroups = computed(() => {
  if (!filteredData.value) return new Set<string>();
  const groups = new Set<string>();
  filteredData.value.links.forEach((link) => {
    const cat = link.category || "other";
    const group = CATEGORY_TO_GROUP[cat] || cat;
    groups.add(group);
  });
  return groups;
});



// Get sorted categories for tooltip display (highest count first)

// Compute all visible categories (including production if not hidden)
const allVisibleCategories = computed(() => {
  const categories = [
    ...staffCategories
      .filter((c) => c.key !== "production")
      .map((cat) => cat.key),
  ];
  if (!hideProducers.value) {
    categories.push("production");
  }
  categories.push("other");
  return categories;
});

// Staff categorization
const staffSortByName = (a: any, b: any) => {
  const nameA = (a.staff?.name_en || a.staff?.name_ja || '').toLowerCase();
  const nameB = (b.staff?.name_en || b.staff?.name_ja || '').toLowerCase();
  return nameA.localeCompare(nameB);
};

const categorizedStaff = computed(() => {
  if (!props.staff) return {};
  const { categorized } = categorizeStaff(props.staff);
  // Sort each category alphabetically by name
  for (const key in categorized) {
    categorized[key].sort(staffSortByName);
  }
  return categorized;
});

const uncategorizedStaff = computed(() => {
  if (!props.staff) return [];
  const { uncategorized } = categorizeStaff(props.staff);
  return uncategorized.sort(staffSortByName);
});

// Cached staff ID lookups per category and per group — avoids repeated .map/.filter
const categoryStaffIdsMap = computed(() => {
  const map: Record<string, string[]> = {};
  STAFF_CATEGORIES.forEach((cat) => {
    const staff = categorizedStaff.value[cat.key] || [];
    map[cat.key] = staff.map((s: any) => s.staff?.staff_id).filter(Boolean);
  });
  map["other"] = uncategorizedStaff.value
    .map((s: any) => s.staff?.staff_id)
    .filter(Boolean);
  return map;
});

const groupStaffIdsMap = computed(() => {
  const map: Record<string, string[]> = {};
  CATEGORY_GROUPS.forEach((group) => {
    const ids: string[] = [];
    group.children.forEach((childKey) => {
      ids.push(...(categoryStaffIdsMap.value[childKey] || []));
    });
    map[group.key] = ids;
  });
  return map;
});

const getCategoryTitle = (category: any) => {
  return getCategoryTitleUtil(category);
};

// Get short category title (just English name)
const getCategoryTitleShort = (categoryKey: string): string => {
  if (categoryKey === "other") return "Other";
  const category = getCategoryByKey(categoryKey);
  return category ? category.title_en : categoryKey;
};

// Compute visible category groups (excluding production group if hideProducers)
const visibleCategoryGroups = computed(() => {
  return categoryGroups.filter((group) => {
    if (hideProducers.value && group.key === "production_group") return false;
    return true;
  });
});

// Get children of a group that are visible (respects hideProducers)
const getVisibleChildCategories = (groupKey: string): string[] => {
  const group = getGroupByKey(groupKey);
  if (!group) return [];
  if (hideProducers.value && groupKey === "production_group") return [];
  return group.children;
};

// Get count of visible children for a group
const getGroupVisibleChildrenCount = (groupKey: string): number => {
  return getVisibleChildCategories(groupKey).length;
};

// Check if a group is fully selected (all visible children selected)
const isGroupFullySelected = (groupKey: string): boolean => {
  const children = getVisibleChildCategories(groupKey);
  if (children.length === 0) return false;
  return children.every((childKey) =>
    selectedCategories.value.includes(childKey),
  );
};

// Check if a group is partially selected (some but not all children selected)
const isGroupPartiallySelected = (groupKey: string): boolean => {
  const children = getVisibleChildCategories(groupKey);
  if (children.length === 0) return false;
  const selectedCount = children.filter((childKey) =>
    selectedCategories.value.includes(childKey),
  ).length;
  return selectedCount > 0 && selectedCount < children.length;
};

// Get selected count for a group
const getGroupSelectedCount = (groupKey: string): number => {
  const children = getVisibleChildCategories(groupKey);
  return children.filter((childKey) =>
    selectedCategories.value.includes(childKey),
  ).length;
};

// Get staff count for a group from graph nodes (only counts staff in current filtered graph)
const getGroupStaffCountFromGraph = (groupKey: string): number => {
  if (!graphData.value) return 0;
  const group = getGroupByKey(groupKey);
  if (!group) return 0;

  return graphData.value.nodes.filter((node) => {
    if (node.type !== "staff") return false;
    const nodeCategory = node.category || "other";
    return group.children.includes(nodeCategory);
  }).length;
};

// Get staff count for "other" category from graph nodes
const getOtherStaffCountFromGraph = (): number => {
  if (!graphData.value) return 0;
  return graphData.value.nodes.filter(
    (node) =>
      node.type === "staff" && (node.category === "other" || !node.category),
  ).length;
};

// Toggle all categories in a group
const toggleGroupCategories = (groupKey: string) => {
  const children = getVisibleChildCategories(groupKey);
  if (children.length === 0) return;

  const isFullySelected = isGroupFullySelected(groupKey);

  if (isFullySelected) {
    // Deselect all children
    selectedCategories.value = selectedCategories.value.filter(
      (key) => !children.includes(key),
    );
  } else {
    // Select all children
    const newSelected = new Set(selectedCategories.value);
    children.forEach((childKey) => newSelected.add(childKey));
    selectedCategories.value = Array.from(newSelected);
  }
};

// Toggle a single category
const toggleCategory = (categoryKey: string) => {
  if (selectedCategories.value.includes(categoryKey)) {
    selectedCategories.value = selectedCategories.value.filter(
      (key) => key !== categoryKey,
    );
  } else {
    selectedCategories.value = [...selectedCategories.value, categoryKey];
  }
};

// Staff View helpers - get staff count for a group
const getGroupStaffCount = (groupKey: string): number => {
  const group = getGroupByKey(groupKey);
  if (!group) return 0;
  return group.children.reduce((sum, childKey) => {
    return sum + (categorizedStaff.value[childKey]?.length || 0);
  }, 0);
};




// State for staff expansion panels (legacy, kept for backwards compatibility)
const categoriesOpen = ref<number[]>([]);

// Initialize expansion panels for the two-tier structure
const initializeGroupsOpen = () => {
  // Calculate total staff count
  let totalStaffCount = 0;
  staffCategories.forEach((category) => {
    const staffInCategory = categorizedStaff.value[category.key] || [];
    totalStaffCount += staffInCategory.length;
  });
  totalStaffCount += uncategorizedStaff.value.length;

  // If total staff <= 15, open all groups and child categories
  if (totalStaffCount <= 15) {
    const openGroups: string[] = [];
    categoryGroups.forEach((group) => {
      if (getGroupStaffCount(group.key) > 0) {
        openGroups.push(group.key);
        // Also open all child categories within this group
        childCategoriesOpen.value[group.key] = group.children.filter(
          (childKey) => categorizedStaff.value[childKey]?.length > 0,
        );
      }
    });
    if (uncategorizedStaff.value.length > 0) {
      openGroups.push("other");
    }
    groupsOpen.value = openGroups;
  } else {
    groupsOpen.value = [];
  }
};

// Initialize expansion panels - open all if total staff <= 15, otherwise close all
const initializeCategoriesOpen = () => {
  initializeGroupsOpen();
};

// Watch for staff changes to reinitialize panel states
watch(
  [categorizedStaff, uncategorizedStaff],
  () => {
    initializeCategoriesOpen();
  },
  { immediate: true },
);

// Function to compute filtered nodes and links based on selected categories
const updateFilteredData = () => {
  if (!graphData.value) {
    filteredData.value = null;
    return;
  }

  const nodeMap = graphNodeMap.value;

  // Use shared helper for base staff/link filtering
  let visibleStaffIds: Set<string>;
  let visibleLinks: GraphLink[];

  if (selectedCategories.value.length === 0) {
    visibleStaffIds = new Set();
    visibleLinks = [];
  } else {
    const base = computeBaseFiltered(
      selectedCategories.value,
      sameRoleOnly.value,
      nodeMap,
    );
    visibleStaffIds = base.visibleStaffIds;
    visibleLinks = base.visibleLinks;
  }

  // Further filter by individual staff selection (if any staff are selected)
  let finalStaffIds = visibleStaffIds;
  if (selectedStaff.value.size > 0) {
    finalStaffIds = new Set(
      [...visibleStaffIds].filter((id) => selectedStaff.value.has(id)),
    );
    // Update visible links to only include those with selected staff
    visibleLinks = visibleLinks.filter((link) => {
      const sourceId =
        typeof link.source === "object" ? link.source.id : link.source;
      const targetId =
        typeof link.target === "object" ? link.target.id : link.target;
      return finalStaffIds.has(sourceId) || finalStaffIds.has(targetId);
    });
  }

  // Calculate anime connection counts from visible links only
  // This ensures anime nodes respect both category filters and "match category across connections"
  const connectedAnimeIds = new Set<string | number>();
  const animeConnectionCounts = new Map<string | number, number>();

  visibleLinks.forEach((link) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;

    const sourceNode = nodeMap.get(sourceId);
    const targetNode = nodeMap.get(targetId);

    // Count staff->anime connections for related anime (only if staff is visible)
    if (
      sourceNode?.type === "staff" &&
      targetNode?.type === "anime" &&
      finalStaffIds.has(sourceId)
    ) {
      connectedAnimeIds.add(targetId);
      animeConnectionCounts.set(
        targetId,
        (animeConnectionCounts.get(targetId) || 0) + 1,
      );
    }
    // Also count anime->staff connections for center anime (only if staff is visible)
    if (
      sourceNode?.type === "anime" &&
      targetNode?.type === "staff" &&
      finalStaffIds.has(targetId)
    ) {
      connectedAnimeIds.add(sourceId);
      animeConnectionCounts.set(
        sourceId,
        (animeConnectionCounts.get(sourceId) || 0) + 1,
      );
    }
  });

  // Pre-compute genre/tag filter IDs and metadata map once (hoisted out of the loop below)
  const hasActiveGenreTagFilters =
    filterMetadataLoaded.value &&
    (recommendationFilters.value.genres.length > 0 ||
      recommendationFilters.value.tags.length > 0);

  let animeMetadataMap: Map<number, any> | null = null;
  let activeFilterGenreIds: any[] = [];
  let activeFilterTagIds: any[] = [];

  if (hasActiveGenreTagFilters) {
    const metadata = getFilteredMetadata(parseInt(props.animeId));
    animeMetadataMap = new Map(metadata.map((a: any) => [a.id, a]));

    activeFilterGenreIds =
      useBitmaps.value && lookupTables.value
        ? recommendationFilters.value.genres
            .map((name: string) => lookupTables.value.genres.indexOf(name))
            .filter((id: number) => id !== -1)
        : recommendationFilters.value.genres;

    activeFilterTagIds =
      useBitmaps.value && lookupTables.value
        ? recommendationFilters.value.tags
            .map((name: string) => lookupTables.value.tags.indexOf(name))
            .filter((id: number) => id !== -1)
        : recommendationFilters.value.tags;
  }

  // Pre-compute related anime IDs as a Set for O(1) lookup
  const relatedAnimeIdSet =
    hideRelatedAnime.value && props.relations
      ? new Set(props.relations.map((rel: any) => String(rel.anilistId)))
      : null;

  // Pre-compute favorited anime IDs as a Set for O(1) lookup
  const favoritedIdSet =
    !showFavorites.value && favoritedAnimeIds.value.size > 0
      ? new Set([...favoritedAnimeIds.value].map((id) => String(id)))
      : null;

  // First pass: filter by every criterion except genre/tag and minConnections.
  // This intermediate set is used to derive stable available genres/tags (so
  // selecting a genre doesn't cause other genre chips to disappear).
  const preMinConnectionCounts = new Map<string | number, number>();

  const animeNodeCandidatesPreGenreTag = graphData.value.nodes.filter((n) => {
    if (n.type !== "anime" || !connectedAnimeIds.has(n.id)) return false;
    if (n.id === graphData.value!.center) return true;
    if (relatedAnimeIdSet && relatedAnimeIdSet.has(String(n.id))) return false;
    if (favoritedIdSet && favoritedIdSet.has(String(n.id))) return false;

    // Apply format filter
    if (selectedFormats.value.length > 0 && n.format) {
      if (!selectedFormats.value.includes(n.format)) return false;
    }

    return true;
  });

  // Expose pre-genre/tag anime IDs for computing stable available genres/tags.
  // Apply minConnections here so filter counts match what the graph actually shows.
  // (animeConnectionCounts is staff-link based and independent of genre/tag filters.)
  preGenreTagAnimeNodeIds.value = new Set(
    animeNodeCandidatesPreGenreTag
      .filter(
        (n) =>
          n.id !== graphData.value!.center &&
          (animeConnectionCounts.get(n.id) || 0) >= minConnections.value,
      )
      .map((n) => parseInt(String(n.id))),
  );

  // Second pass: apply genre/tag filters on top
  const animeNodeCandidates = animeNodeCandidatesPreGenreTag.filter((n) => {
    if (n.id === graphData.value!.center) return true;

    // Apply genre/tag filters
    if (hasActiveGenreTagFilters && animeMetadataMap) {
      const animeMetadata = animeMetadataMap.get(parseInt(String(n.id)));

      if (animeMetadata) {
        const animeGenres = useBitmaps.value
          ? animeMetadata.g || []
          : animeMetadata.genres || [];
        const animeTags = useBitmaps.value
          ? animeMetadata.t || []
          : animeMetadata.tags || [];

        const genreMatch =
          activeFilterGenreIds.length === 0 ||
          activeFilterGenreIds.every((id: any) => animeGenres.includes(id));
        const tagMatch =
          activeFilterTagIds.length === 0 ||
          activeFilterTagIds.every((id: any) => animeTags.includes(id));

        if (!genreMatch || !tagMatch) return false;
      }
    }

    preMinConnectionCounts.set(n.id, animeConnectionCounts.get(n.id) || 0);
    return true;
  });

  // Expose counts for the minConnectionsOptions dropdown
  filteredAnimeConnectionCounts.value = preMinConnectionCounts;

  // Second pass: apply minConnections threshold
  const animeNodes = animeNodeCandidates.filter(
    (n) =>
      n.id === graphData.value!.center ||
      (preMinConnectionCounts.get(n.id) || 0) >= minConnections.value,
  );

  // NOW filter out lonely staff - must run AFTER anime filtering to check actual visible anime
  if (hideLonelyStaff.value) {
    const visibleAnimeIds = new Set(animeNodes.map((n) => n.id));
    const staffWithVisibleAnimeConnections = new Set<string>();

    // Check which staff have connections to anime that will actually be visible
    visibleLinks.forEach((link) => {
      const sourceId =
        typeof link.source === "object" ? link.source.id : link.source;
      const targetId =
        typeof link.target === "object" ? link.target.id : link.target;

      const sourceNode = nodeMap.get(sourceId);
      const targetNode = nodeMap.get(targetId);

      // Check if this is a staff->anime link where anime is NOT the center AND will be visible
      if (
        sourceNode?.type === "staff" &&
        targetNode?.type === "anime" &&
        targetId !== graphData.value!.center &&
        visibleAnimeIds.has(targetId)
      ) {
        staffWithVisibleAnimeConnections.add(sourceId);
      }
    });

    // Filter out staff that don't have connections to visible anime
    finalStaffIds = new Set(
      [...finalStaffIds].filter((id) =>
        staffWithVisibleAnimeConnections.has(id),
      ),
    );

    // Update visible links to only include those with non-lonely staff
    visibleLinks = visibleLinks.filter((link) => {
      const sourceId =
        typeof link.source === "object" ? link.source.id : link.source;
      const targetId =
        typeof link.target === "object" ? link.target.id : link.target;
      return finalStaffIds.has(sourceId) || finalStaffIds.has(targetId);
    });
  }

  const staffNodes = graphData.value.nodes.filter(
    (n) => n.type === "staff" && finalStaffIds.has(n.id),
  );

  // Filter links to only show edges between visible nodes (prevents orphaned edges)
  const visibleAnimeIds = new Set(animeNodes.map((n) => n.id));
  const allVisibleNodeIds = new Set([
    ...visibleAnimeIds,
    ...finalStaffIds,
    graphData.value!.center,
  ]);

  const finalLinks = visibleLinks.filter((link) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;

    // Only show link if BOTH endpoints are visible
    const bothVisible =
      allVisibleNodeIds.has(sourceId) && allVisibleNodeIds.has(targetId);

    return bothVisible;
  });

  // ANIME-ONLY MODE: Hide staff/category nodes, show per-category edges from center to anime
  // Mirrors category nodes mode but without visible category nodes — one edge per (category, anime) pair
  if (hideStaffNodes.value) {
    // Build map: staffId → role in center anime (for hover display)
    const centerRoleByStaffId = new Map<string | number, string | undefined>();
    finalLinks.forEach((l) => {
      const sourceId = typeof l.source === "object" ? l.source.id : l.source;
      const targetId = typeof l.target === "object" ? l.target.id : l.target;
      let staffId: string | number | null = null;
      if (sourceId === graphData.value!.center) staffId = targetId;
      else if (targetId === graphData.value!.center) staffId = sourceId;
      if (staffId !== null) {
        const role = l.role;
        centerRoleByStaffId.set(
          staffId,
          role
            ? Array.isArray(role)
              ? role.join(", ")
              : String(role)
            : undefined,
        );
      }
    });

    // Aggregate per (category, animeId) — mirrors category node mode's categoryAnimeConnections
    const categoryAnimeConnections = new Map<
      string,
      Map<
        string | number,
        {
          staffCount: number;
          staffNames: string[];
          roles: string[];
          staffDetails: Array<{
            staffId: string;
            staffName: string;
            image?: string;
            mainRole: string;
            otherRole: string;
            category: string;
          }>;
        }
      >
    >();

    finalLinks.forEach((link) => {
      const sourceId =
        typeof link.source === "object" ? link.source.id : link.source;
      const targetId =
        typeof link.target === "object" ? link.target.id : link.target;

      const sourceNode = nodeMap.get(sourceId);
      const targetNode = nodeMap.get(targetId);

      // Staff → Anime link (not center)
      if (
        sourceNode?.type === "staff" &&
        targetNode?.type === "anime" &&
        targetId !== graphData.value!.center
      ) {
        const category = sourceNode.category || "other";
        const staffName = sourceNode.label;
        const role = Array.isArray(link.role)
          ? link.role.join(", ")
          : link.role || "";

        if (!categoryAnimeConnections.has(category)) {
          categoryAnimeConnections.set(category, new Map());
        }
        const animeMap = categoryAnimeConnections.get(category)!;
        if (!animeMap.has(targetId)) {
          animeMap.set(targetId, {
            staffCount: 0,
            staffNames: [],
            roles: [],
            staffDetails: [],
          });
        }
        const conn = animeMap.get(targetId)!;
        conn.staffCount++;
        conn.staffNames.push(staffName);
        if (role) conn.roles.push(role);

        const mainRole = centerRoleByStaffId.get(sourceId) || "Unknown";
        conn.staffDetails.push({
          staffId: String(sourceId),
          staffName,
          image: sourceNode.image,
          mainRole,
          otherRole: role || "Unknown",
          category,
        });
      }
    });

    // Create per-category center → anime links (one edge per category per anime)
    const directLinks: GraphLink[] = [];
    categoryAnimeConnections.forEach((animeMap, categoryKey) => {
      animeMap.forEach((conn, animeId) => {
        directLinks.push({
          source: graphData.value!.center,
          target: animeId,
          type: "anime-anime",
          category: categoryKey,
          staffCount: conn.staffCount,
          staffNames: conn.staffNames,
          staffDetails: conn.staffDetails,
          role:
            conn.staffCount === 1
              ? `${conn.staffNames[0]}${conn.roles[0] ? ` (${conn.roles[0]})` : ""}`
              : `${conn.staffCount} shared staff`,
        });
      });
    });

    // Assign perpendicular bezier offsets so parallel edges fan out instead of stacking
    const edgesByTarget = new Map<string | number, GraphLink[]>();
    directLinks.forEach((link) => {
      const targetId = link.target as string | number;
      if (!edgesByTarget.has(targetId)) edgesByTarget.set(targetId, []);
      edgesByTarget.get(targetId)!.push(link);
    });
    edgesByTarget.forEach((edges) => {
      const n = edges.length;
      edges.forEach((edge, i) => {
        edge.parallelOffset = (i - (n - 1) / 2) * 30;
      });
    });

    filteredData.value = {
      nodes: animeNodes,
      links: directLinks,
      center: graphData.value.center,
    };
  } else if (useCategoryNodes.value && staffNodes.length > 0) {
    // CATEGORY NODE MODE: Aggregate staff into category nodes
    // Group staff by category
    const staffByCategory = new Map<string, GraphNode[]>();
    staffNodes.forEach((staff) => {
      const category = staff.category || "other";
      if (!staffByCategory.has(category)) {
        staffByCategory.set(category, []);
      }
      staffByCategory.get(category)!.push(staff);
    });

    // Pre-build map: staffId → formatted role string (from center links in finalLinks)
    const centerRoleByStaffId = new Map<string | number, string | undefined>();
    finalLinks.forEach((l) => {
      const sourceId = typeof l.source === "object" ? l.source.id : l.source;
      const targetId = typeof l.target === "object" ? l.target.id : l.target;
      let staffId: string | number | null = null;
      if (sourceId === graphData.value!.center) staffId = targetId;
      else if (targetId === graphData.value!.center) staffId = sourceId;
      if (staffId !== null) {
        const role = l.role;
        centerRoleByStaffId.set(
          staffId,
          role
            ? Array.isArray(role)
              ? role.join(", ")
              : String(role)
            : undefined,
        );
      }
    });

    // Create category nodes
    const categoryNodes: GraphNode[] = [];
    staffByCategory.forEach((staffList, categoryKey) => {
      const categoryDef = getCategoryByKey(categoryKey);
      const categoryLabel = categoryDef
        ? categoryDef.title_en
        : categoryKey === "other"
          ? "Other"
          : categoryKey;

      categoryNodes.push({
        id: `category-${categoryKey}`,
        label: `${categoryLabel} (${staffList.length})`,
        type: "category",
        group: "staff",
        category: categoryKey,
        staffCount: staffList.length,
        staffList: staffList.map((s) => ({
          id: String(s.id),
          name: s.label,
          image: s.image,
          role: centerRoleByStaffId.get(s.id),
        })),
      });
    });

    // Create aggregated links from center to category nodes
    const categoryLinks: GraphLink[] = [];

    // Links from center anime to category nodes
    staffByCategory.forEach((staffList, categoryKey) => {
      categoryLinks.push({
        source: graphData.value!.center,
        target: `category-${categoryKey}`,
        type: "center-category",
        category: categoryKey,
        staffCount: staffList.length,
        role: `${staffList.length} staff`,
      });
    });

    // Links from category nodes to other anime (aggregated by category-anime pair)
    // Map: categoryKey -> animeId -> { staffCount, staffNames, roles }
    const categoryAnimeConnections = new Map<
      string,
      Map<
        string | number,
        { staffCount: number; staffNames: string[]; roles: string[] }
      >
    >();

    finalLinks.forEach((link) => {
      const sourceId =
        typeof link.source === "object" ? link.source.id : link.source;
      const targetId =
        typeof link.target === "object" ? link.target.id : link.target;

      const sourceNode = nodeMap.get(sourceId);
      const targetNode = nodeMap.get(targetId);

      // Staff -> Anime link (not center)
      if (
        sourceNode?.type === "staff" &&
        targetNode?.type === "anime" &&
        targetId !== graphData.value!.center
      ) {
        const category = sourceNode.category || "other";
        const staffName = sourceNode.label;
        const role = Array.isArray(link.role)
          ? link.role.join(", ")
          : link.role || "";

        if (!categoryAnimeConnections.has(category)) {
          categoryAnimeConnections.set(category, new Map());
        }
        const animeMap = categoryAnimeConnections.get(category)!;
        if (!animeMap.has(targetId)) {
          animeMap.set(targetId, { staffCount: 0, staffNames: [], roles: [] });
        }
        const conn = animeMap.get(targetId)!;
        conn.staffCount++;
        conn.staffNames.push(staffName);
        if (role) conn.roles.push(role);
      }
    });

    // Create category -> anime links
    categoryAnimeConnections.forEach((animeMap, categoryKey) => {
      animeMap.forEach((conn, animeId) => {
        categoryLinks.push({
          source: `category-${categoryKey}`,
          target: animeId,
          type: "category-anime",
          category: categoryKey,
          staffCount: conn.staffCount,
          staffNames: conn.staffNames,
          role:
            conn.staffCount === 1
              ? `${conn.staffNames[0]}${conn.roles[0] ? ` (${conn.roles[0]})` : ""}`
              : `${conn.staffCount} shared staff`,
        });
      });
    });

    filteredData.value = {
      nodes: [...animeNodes, ...categoryNodes],
      links: categoryLinks,
      center: graphData.value.center,
    };
  } else {
    // Normal mode: individual staff nodes
    filteredData.value = {
      nodes: [...animeNodes, ...staffNodes],
      links: finalLinks,
      center: graphData.value.center,
    };
  }
};

const toggleAllCategories = () => {
  if (selectedCategories.value.length === allVisibleCategories.value.length) {
    // All selected, clear all
    selectedCategories.value = [];
  } else {
    // Not all selected, select all
    selectedCategories.value = [...allVisibleCategories.value];
  }
};

const toggleAllFormats = () => {
  if (selectedFormats.value.length === availableFormats.value.length) {
    selectedFormats.value = [];
  } else {
    selectedFormats.value = [...availableFormats.value];
  }
};

// Shared helpers for graph filtering — used by both autoAdjust and updateFilteredData

// Filter staff and links by category + sameRoleOnly settings
const computeBaseFiltered = (
  categories: string[],
  sameRole: boolean,
  nodeMap: Map<string | number, GraphNode>,
): { visibleStaffIds: Set<string>; visibleLinks: GraphLink[] } => {
  if (!graphData.value) return { visibleStaffIds: new Set(), visibleLinks: [] };

  // Build staff→roles map for sameRoleOnly check
  const staffRolesInCenter = new Map<string, Set<string>>();
  graphData.value.links.forEach((link) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;
    if (sourceId === graphData.value!.center) {
      if (!staffRolesInCenter.has(targetId))
        staffRolesInCenter.set(targetId, new Set());
      staffRolesInCenter.get(targetId)!.add(link.category || "other");
    }
  });

  const visibleStaffIds = new Set(
    graphData.value.nodes
      .filter(
        (n) => n.type === "staff" && categories.includes(n.category || "other"),
      )
      .map((n) => n.id),
  );

  const visibleLinks = graphData.value.links.filter((link) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;
    if (!visibleStaffIds.has(sourceId) && !visibleStaffIds.has(targetId))
      return false;

    if (sameRole) {
      const sourceNode = nodeMap.get(sourceId);
      const targetNode = nodeMap.get(targetId);
      let staffId: string | null = null;
      if (sourceNode?.type === "staff") staffId = sourceId;
      if (targetNode?.type === "staff") staffId = targetId;
      if (staffId) {
        if (staffRolesInCenter.has(staffId)) {
          const staffParentGroups = new Set(
            [...staffRolesInCenter.get(staffId)!].map(
              (cat) => CATEGORY_TO_GROUP[cat] || cat,
            ),
          );
          const linkParentGroup =
            CATEGORY_TO_GROUP[link.category || "other"] ||
            link.category ||
            "other";
          return staffParentGroups.has(linkParentGroup);
        }
        return false;
      }
    }
    return true;
  });

  return { visibleStaffIds, visibleLinks };
};

// Count staff→anime connections from visible links, optionally filtering by format
const countAnimeConnections = (
  links: GraphLink[],
  staffIds: Set<string>,
  nodeMap: Map<string | number, GraphNode>,
  formats: string[] = [],
): Map<string | number, number> => {
  if (!graphData.value) return new Map();
  const counts = new Map<string | number, number>();
  links.forEach((link) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;
    const sourceNode = nodeMap.get(sourceId);
    const targetNode = nodeMap.get(targetId);
    if (
      sourceNode?.type === "staff" &&
      targetNode?.type === "anime" &&
      targetId !== graphData.value!.center &&
      staffIds.has(sourceId)
    ) {
      if (
        formats.length === 0 ||
        !targetNode.format ||
        formats.includes(targetNode.format)
      ) {
        counts.set(targetId, (counts.get(targetId) || 0) + 1);
      }
    }
  });
  return counts;
};

// Find lowest minConnections threshold that keeps anime count at or under targetMax
const findBestMinConnections = (
  counts: Map<string | number, number>,
  targetMax: number,
): number => {
  const floor = graphData.value?.minConnectionsFloor ?? 2;
  const values = Array.from(counts.values());
  if (values.filter((c) => c >= floor).length <= targetMax) return floor;
  for (let threshold = floor + 1; threshold <= 20; threshold++) {
    if (values.filter((c) => c >= threshold).length <= targetMax)
      return threshold;
  }
  return 20;
};

// Auto-adjust filters on first load to show a reasonable number of related anime
const autoAdjustFiltersForFirstLoad = () => {
  console.log(
    "[GraphVis] autoAdjustFiltersForFirstLoad called:",
    "isFirstLoad:",
    isFirstLoad.value,
    "graphData:",
    !!graphData.value,
    "selectedFormats:",
    [...selectedFormats.value],
  );
  if (!isFirstLoad.value || !graphData.value) {
    console.log(
      "[GraphVis] autoAdjust: SKIPPED (isFirstLoad:",
      isFirstLoad.value,
      "graphData:",
      !!graphData.value,
      ")",
    );
    return;
  }

  const TARGET_MAX = 80;
  const nodeMap = graphNodeMap.value;

  // Step 1: Count with current filters (categories, sameRoleOnly, format)
  const { visibleStaffIds, visibleLinks } = computeBaseFiltered(
    selectedCategories.value,
    sameRoleOnly.value,
    nodeMap,
  );
  const initialCounts = countAnimeConnections(
    visibleLinks,
    visibleStaffIds,
    nodeMap,
    selectedFormats.value,
  );
  minConnections.value = findBestMinConnections(initialCounts, TARGET_MAX);

  const finalAnimeCount = Array.from(initialCounts.values()).filter(
    (c) => c >= minConnections.value,
  ).length;
  console.log(
    "[GraphVis] autoAdjust: initialCounts size:",
    initialCounts.size,
    "finalAnimeCount:",
    finalAnimeCount,
    "minConnections:",
    minConnections.value,
  );

  // If <20 results, progressively relax filters
  if (finalAnimeCount < 20) {
    // Step 2: Open up format filter (allow all formats)
    console.log(
      "[GraphVis] autoAdjust: opening formats (was:",
      [...selectedFormats.value],
      "to:",
      [...availableFormats.value],
      ")",
    );
    selectedFormats.value = [...availableFormats.value];
    const allFormatCounts = countAnimeConnections(
      visibleLinks,
      visibleStaffIds,
      nodeMap,
    );
    minConnections.value = findBestMinConnections(allFormatCounts, TARGET_MAX);

    const countWithAllFormats = Array.from(allFormatCounts.values()).filter(
      (c) => c >= minConnections.value,
    ).length;

    if (countWithAllFormats === 0) {
      // Step 3: Disable sameRoleOnly
      sameRoleOnly.value = false;
      const { visibleStaffIds: vs2, visibleLinks: vl2 } = computeBaseFiltered(
        selectedCategories.value,
        false,
        nodeMap,
      );
      const noSameRoleCounts = countAnimeConnections(vl2, vs2, nodeMap);
      minConnections.value = findBestMinConnections(
        noSameRoleCounts,
        TARGET_MAX,
      );

      const countNoSameRole = Array.from(noSameRoleCounts.values()).filter(
        (c) => c >= minConnections.value,
      ).length;

      if (countNoSameRole === 0 && hideProducers.value) {
        // Step 4: Enable producers
        hideProducers.value = false;
        if (!selectedCategories.value.includes("production")) {
          selectedCategories.value = [
            ...selectedCategories.value,
            "production",
          ];
        }

        const allStaffIds = graphData
          .value!.nodes.filter((n) => n.type === "staff")
          .map((n) => n.id);
        selectedStaff.value = new Set(allStaffIds);

        const { visibleStaffIds: vs3, visibleLinks: vl3 } = computeBaseFiltered(
          selectedCategories.value,
          false,
          nodeMap,
        );
        const withProducersCounts = countAnimeConnections(vl3, vs3, nodeMap);
        minConnections.value = findBestMinConnections(
          withProducersCounts,
          TARGET_MAX,
        );
      }
    }
  }

  // Mark that we've done the initial auto-adjustment
  isFirstLoad.value = false;
  console.log(
    "[GraphVis] autoAdjust: DONE. Final selectedFormats:",
    [...selectedFormats.value],
    "minConnections:",
    minConnections.value,
    "sameRoleOnly:",
    sameRoleOnly.value,
  );
};

const fetchGraphData = async (preloadedData?: GraphData) => {
  const thisGeneration = ++fetchGeneration;
  console.log(
    "[GraphVis] fetchGraphData called:",
    "gen:",
    thisGeneration,
    "preloaded:",
    !!preloadedData,
    "preloaded center:",
    preloadedData?.center,
    "animeId prop:",
    props.animeId,
    "isFirstLoad:",
    isFirstLoad.value,
  );
  loading.value = true;
  isInitialLoad.value = true;
  isFirstLoad.value = true;
  selectedLegendGroups.value = new Set();
  if (renderTimeout) clearTimeout(renderTimeout);
  try {
    let data: GraphData;
    if (preloadedData) {
      data = preloadedData;
    } else {
      console.log(
        "[GraphVis] fetchGraphData: fetching from API for",
        props.animeId,
      );
      const response = await api(
        `/graph/${encodeURIComponent(props.animeId)}`,
      );
      // Discard if a newer fetchGraphData call has started while we were awaiting
      if (thisGeneration !== fetchGeneration) {
        console.log(
          "[GraphVis] fetchGraphData: STALE response (gen",
          thisGeneration,
          "vs current",
          fetchGeneration,
          "), discarding",
        );
        return;
      }
      if (!response.success) return;
      data = response.data;
    }
    console.log(
      "[GraphVis] fetchGraphData: got data, center:",
      data.center,
      "nodes:",
      data.nodes.length,
      "links:",
      data.links.length,
    );
    graphData.value = data;

    // Initialize selectedStaff with all staff IDs (all selected by default)
    // Exclude producers if hideProducers is true
    const allStaffIds = graphData.value.nodes
      .filter((n) => {
        if (n.type !== "staff") return false;
        // Skip producers if hideProducers is enabled
        if (hideProducers.value && n.category === "production") return false;
        return true;
      })
      .map((n) => n.id);
    selectedStaff.value = new Set(allStaffIds);

    // Initialize selectedFormats with only the center anime's format
    const centerNode = graphData.value.nodes.find(
      (n) => n.id === graphData.value!.center,
    );
    if (centerNode?.format) {
      selectedFormats.value = [centerNode.format];
      console.log(
        "[GraphVis] fetchGraphData: set selectedFormats to center format:",
        centerNode.format,
      );
    } else {
      console.log(
        "[GraphVis] fetchGraphData: center node has no format, selectedFormats stays:",
        selectedFormats.value,
      );
    }

    // Auto-enable category nodes if staff count exceeds threshold (only on first page load)
    if (isFirstLoad.value) {
      const staffCount = allStaffIds.length;
      console.log("[GraphVis] Auto-enable check:", {
        staffCount,
        threshold: categoryNodesThreshold,
        willEnable: staffCount > categoryNodesThreshold,
      });
      if (staffCount > categoryNodesThreshold) {
        useCategoryNodes.value = true;
        console.log("[GraphVis] Auto-enabled category nodes");
      }
    }

    // Auto-adjust filters on first load to show reasonable number of anime
    autoAdjustFiltersForFirstLoad();

    // Apply URL state AFTER auto-adjust but BEFORE the first render so URL params
    // override auto-adjust results without triggering a second render pass.
    // Watchers are all guarded by !isInitialLoad so no side-effects fire here.
    applyURLState();

    console.log(
      "[GraphVis] fetchGraphData: before render. selectedFormats:",
      [...selectedFormats.value],
      "isFirstLoad:",
      isFirstLoad.value,
    );
    updateFilteredData();
    renderGraph();
    lastRenderedFingerprint = computeFilteredDataFingerprint(
      filteredData.value,
    );
    console.log(
      "[GraphVis] fetchGraphData: render complete. filteredData nodes:",
      filteredData.value?.nodes.length,
      "links:",
      filteredData.value?.links.length,
    );

    setTimeout(() => {
      console.log(
        "[GraphVis] fetchGraphData: setting isInitialLoad=false (delayed). selectedFormats:",
        [...selectedFormats.value],
      );
      isInitialLoad.value = false;
    }, 100);
  } catch (error) {
    console.error("Error fetching graph data:", error);
  } finally {
    loading.value = false;
  }
};


// Sync categories with staff selection
const syncCategoriesToStaff = () => {
  if (!graphData.value || isSyncingFilters.value) return;

  isSyncingFilters.value = true;

  const newCategories = new Set<string>();

  // For each category, check if any staff in that category are selected
  for (const [catKey, ids] of Object.entries(categoryStaffIdsMap.value)) {
    if (hideProducers.value && catKey === "production") continue;
    if (ids.some((id: string) => selectedStaff.value.has(id))) {
      newCategories.add(catKey);
    }
  }

  selectedCategories.value = Array.from(newCategories);

  // Use nextTick to ensure watchers fire before resetting the flag
  nextTick(() => {
    isSyncingFilters.value = false;
  });
};

// Sync staff selection with categories
const syncStaffToCategories = () => {
  if (!graphData.value || isSyncingFilters.value) return;

  isSyncingFilters.value = true;

  const newSelectedStaff = new Set<string>();

  // For each selected category, add all its staff
  selectedCategories.value.forEach((categoryKey) => {
    if (hideProducers.value && categoryKey === "production") return;
    const ids = categoryStaffIdsMap.value[categoryKey] || [];
    ids.forEach((id: string) => newSelectedStaff.add(id));
  });

  selectedStaff.value = newSelectedStaff;

  // Use nextTick to ensure watchers fire before resetting the flag
  nextTick(() => {
    isSyncingFilters.value = false;
  });
};

// Handler for StaffListView staffChanged event
const onStaffChanged = () => {
  syncCategoriesToStaff();
  updateFilteredData();
  renderGraphIfChanged();
};





// Overlay functions
const closeOverlay = () => {
  selectedAnime.value = null;
  expandedRoleCards.value.clear();
  pinnedCategory.value = null;
};

const toggleRoleCard = (role: string) => {
  if (expandedRoleCards.value.has(role)) {
    expandedRoleCards.value.delete(role);
  } else {
    expandedRoleCards.value.add(role);
  }
  // Trigger reactivity
  expandedRoleCards.value = new Set(expandedRoleCards.value);
};

const handleAnimeNodeClick = async (node: GraphNode) => {
  // Handle category node clicks - show staff from that category for the main anime
  if (node.type === "category") {
    // Get the actual staff data from categorizedStaff using the category key
    let categoryStaff = [];
    if (node.category === "other") {
      categoryStaff = uncategorizedStaff.value;
    } else {
      categoryStaff = categorizedStaff.value[node.category || ""] || [];
    }

    // Show overlay with staff list from this category
    selectedAnime.value = {
      id: props.animeId,
      title:
        mainAnimeData.value?.title ||
        mainAnimeData.value?.title_english ||
        "Main Anime",
      coverImage: mainAnimeData.value?.coverImage || mainAnimeData.value?.image,
      description: `Staff in category: ${node.label}`,
      year: mainAnimeData.value?.year || mainAnimeData.value?.seasonYear,
      staff: categoryStaff,
      isCategoryView: true,
      categoryName: node.label,
    };
    return;
  }

  // Only show overlay for anime nodes
  if (node.type !== "anime") {
    // For staff, navigate directly
    router.push(`/staff/${encodeURIComponent(node.id)}`);
    return;
  }

  // Don't show overlay for the main anime
  if (node.id === props.animeId || String(node.id) === String(props.animeId)) {
    return;
  }

  // Wait for main anime data to load before showing overlay
  if (isLoadingMainAnime.value) {
    return;
  }

  // Clear pinned category for normal clicks (not edge clicks)
  pinnedCategory.value = null;

  // Fetch the selected anime's full data
  try {
    const response = await api(`/anime/${encodeURIComponent(node.id)}`);
    if (response.success && response.data) {
      selectedAnime.value = {
        ...node,
        ...response.data,
      };
    }
  } catch (error) {
    console.error("Error fetching anime details:", error);
    selectedAnime.value = node;
  }
};

// Handle click on aggregated category-anime edge: open overlay then expand + scroll to the category
const handleEdgeClick = async (animeNode: GraphNode, category: string) => {
  // Don't act on center anime
  if (
    animeNode.id === props.animeId ||
    String(animeNode.id) === String(props.animeId)
  )
    return;
  if (isLoadingMainAnime.value) return;

  // Pin the clicked category BEFORE setting selectedAnime so the overlay
  // watch sees it when it fires and auto-expands the category card.
  pinnedCategory.value = category;

  // Fetch anime data (same as handleAnimeNodeClick for anime nodes)
  try {
    const response = await api(
      `/anime/${encodeURIComponent(animeNode.id)}`,
    );
    if (response.success && response.data) {
      selectedAnime.value = {
        ...animeNode,
        ...response.data,
      };
    } else {
      selectedAnime.value = animeNode;
    }
  } catch (error) {
    console.error("Error fetching anime details:", error);
    selectedAnime.value = animeNode;
  }

  // On mobile, panels stack vertically so the "Shared Staff" panel is off-screen.
  // Scroll the overlay body to bring the pinned category card into view.
  if (isMobile.value) {
    await nextTick();
    await new Promise((resolve) => setTimeout(resolve, 100));
    const overlayBody = document.querySelector(".overlay-body");
    const categoryCard = document.querySelector(
      `.category-card[data-category="${category}"]`,
    );
    if (overlayBody && categoryCard) {
      const bodyRect = overlayBody.getBoundingClientRect();
      const cardRect = categoryCard.getBoundingClientRect();
      overlayBody.scrollTo({
        top: overlayBody.scrollTop + (cardRect.top - bodyRect.top) - 12,
        behavior: "smooth",
      });
    }
  }
};

// Handle click on center-category edge: switch to Staff tab, open the matching group/child, scroll to it
const handleCenterCategoryEdgeClick = async (categoryKey: string) => {
  // Clear edge hover state so it doesn't persist when returning to graph tab
  hoveredEdgeLabel.value = null;
  hoveredEdgeCategory.value = null;
  hoveredEdgeSingleStaff.value = null;

  // Switch to staff view first so StaffListView mounts
  graphViewMode.value = "staff";
  await nextTick();

  // Set the focus category so StaffListView opens the right group/child and scrolls to it.
  // Clear first to ensure the watch fires even if clicking the same category twice.
  staffFocusCategory.value = null;
  await nextTick();
  staffFocusCategory.value = categoryKey;
};

const truncateDescription = (desc: string, maxLength: number = 150): string => {
  if (!desc) return "";
  // Remove HTML tags
  const stripped = desc.replace(/<[^>]*>/g, "");
  if (stripped.length <= maxLength) return stripped;
  return stripped.substring(0, maxLength) + "...";
};


const renderGraph = () => {
  if (!graphRef.value || !filteredData.value) return;

  // Stop and clean up previous simulation
  if (simulation) {
    simulation.stop();
    simulation = null;
  }

  // Clear previous graph and hover states
  if (hoverActivationTimeout) {
    clearTimeout(hoverActivationTimeout);
    hoverActivationTimeout = null;
  }
  if (hoverRestoreTimeout) {
    clearTimeout(hoverRestoreTimeout);
    hoverRestoreTimeout = null;
  }
  select(graphRef.value).selectAll("*").remove();
  hoveredAnimeNode.value = null;
  hoveredStaffNode.value = null;
  hoveredAnimeNodeId.value = null;

  // Read theme colors for D3 rendering
  const rootStyle = getComputedStyle(document.documentElement);
  const themePrimary = rootStyle.getPropertyValue("--color-primary").trim();

  const container = graphRef.value;
  const width = container.clientWidth || 800;
  // Use actual container height in fullscreen, otherwise default to 600
  let height = 600;
  if (isFullscreen.value && container.clientHeight > 100) {
    height = container.clientHeight;
  }

  const svg = select(graphRef.value)
    .append("svg")
    .attr("width", width)
    .attr("height", height)
    .attr("viewBox", [0, 0, width, height])
    .style("text-rendering", "optimizeLegibility")
    .style("-webkit-font-smoothing", "antialiased");

  const g = svg.append("g");

  // Add zoom behavior
  const zoom = d3Zoom<SVGSVGElement, unknown>()
    .scaleExtent([0.1, 4])
    .on("zoom", (event) => {
      g.attr("transform", event.transform);
    });

  svg.call(zoom as any);
  zoomBehaviorRef = zoom;
  svgSelectionRef = svg;

  // PERFORMANCE: Create node lookup map for O(1) access (used in tooltips)
  const nodeMap = new Map<string | number, GraphNode>();
  filteredData.value.nodes.forEach((node) => {
    nodeMap.set(node.id, node);
  });

  // Find the center node
  const centerNode = nodeMap.get(filteredData.value.center);

  // Separate nodes by type
  const staffNodes = filteredData.value.nodes.filter(
    (n) => n.type === "staff" || n.type === "category",
  );
  const animeNodes = filteredData.value.nodes.filter(
    (n) => n.type === "anime" && n.id !== filteredData.value.center,
  );

  // Clear all node positions for fresh layout
  filteredData.value.nodes.forEach((node) => {
    node.x = undefined;
    node.y = undefined;
    node.fx = undefined;
    node.fy = undefined;
  });

  // Note: We skip physics simulation entirely and use static layout at the end
  // (Physics code removed for performance)

  // Create links with color-coding by category.
  // In hide-staff mode use <path> so parallel edges can be rendered as bezier curves;
  // all other modes keep the cheaper <line> element.
  const linkElement = hideStaffNodes.value ? "path" : "line";
  const link = g
    .append("g")
    .selectAll(linkElement)
    .data(filteredData.value.links)
    .join(linkElement)
    .attr("fill", "none")
    .attr("stroke", (d: GraphLink) => categoryColors[d.category || "other"])
    .style("stroke-opacity", 0.6)
    .attr("stroke-width", (d: GraphLink) => {
      // Make edges thicker based on staff count for category mode
      if (d.staffCount && d.staffCount > 1) {
        return Math.min(2 + Math.log2(d.staffCount) * 2, 10); // Scale from 2 to 10
      }
      return 2;
    })
    .attr("class", "graph-link");

  // Add hover handlers for edge labels (works in both modes now)
  link
    .on("mouseenter", function (event: any, d: GraphLink) {
      // Highlight the edge — grow from original width, never shrink
      const originalWidth =
        d.staffCount && d.staffCount > 1
          ? Math.min(2 + Math.log2(d.staffCount) * 2, 10)
          : 2;
      select(this)
        .attr("stroke-width", Math.max(originalWidth + 2, 4))
        .style("stroke-opacity", 1);

      hoveredEdgeCategory.value = d.category || "other";

      // For anime-only mode single staff edges, show two-tier role display
      if (d.staffDetails && d.staffDetails.length === 1) {
        const detail = d.staffDetails[0];
        hoveredEdgeSingleStaff.value = {
          staffName: detail.staffName,
          mainRole: detail.mainRole,
          otherRole: detail.otherRole,
        };
        hoveredEdgeLabel.value = null;
      } else {
        hoveredEdgeSingleStaff.value = null;
        hoveredEdgeLabel.value = Array.isArray(d.role)
          ? d.role.join(", ")
          : d.role || "Unknown Role";
      }
    })
    .on("mouseleave", function (event: any, d: GraphLink) {
      // Reset edge appearance - use original width calculation
      const originalWidth =
        d.staffCount && d.staffCount > 1
          ? Math.min(2 + Math.log2(d.staffCount) * 2, 10)
          : 2;

      // Respect legend selection: if groups are highlighted, restore to the
      // correct opacity for this edge instead of a flat 0.6.
      const legendGroups = effectiveLegendGroups.value;
      let restoreOpacity = 0.6;
      if (legendGroups.size > 0) {
        const linkGroup =
          CATEGORY_TO_GROUP[d.category || "other"] || d.category || "other";
        restoreOpacity = legendGroups.has(linkGroup) ? 0.85 : 0.05;
      }

      select(this)
        .attr("stroke-width", originalWidth)
        .style("stroke-opacity", restoreOpacity);

      // Hide label
      hoveredEdgeLabel.value = null;
      hoveredEdgeCategory.value = null;
      hoveredEdgeSingleStaff.value = null;
    });

  // Make aggregated category-anime edges clickable (only multi-staff, opens overlay with category expanded)
  if (useCategoryNodes.value) {
    link
      .filter(
        (d: GraphLink) =>
          d.type === "category-anime" && (d.staffCount || 0) > 1,
      )
      .style("cursor", "pointer")
      .on("click", function (event: any, d: GraphLink) {
        event.stopPropagation();
        const targetId =
          typeof d.target === "object" ? (d.target as GraphNode).id : d.target;
        const targetNode = nodeMap.get(targetId);
        if (targetNode && targetNode.type === "anime") {
          handleEdgeClick(targetNode, d.category || "other");
        }
      });

    // Make center-category edges clickable: switch to Staff tab and scroll to category
    link
      .filter((d: GraphLink) => d.type === "center-category")
      .style("cursor", "pointer")
      .on("click", function (event: any, d: GraphLink) {
        event.stopPropagation();
        handleCenterCategoryEdgeClick(d.category || "other");
      });
  }

  // Make anime-only mode edges clickable (opens overlay with dominant category expanded)
  if (hideStaffNodes.value) {
    link
      .filter(
        (d: GraphLink) => d.type === "anime-anime" && (d.staffCount || 0) > 1,
      )
      .style("cursor", "pointer")
      .on("click", function (event: any, d: GraphLink) {
        event.stopPropagation();
        const targetId =
          typeof d.target === "object" ? (d.target as GraphNode).id : d.target;
        const targetNode = nodeMap.get(targetId);
        if (targetNode && targetNode.type === "anime") {
          handleEdgeClick(targetNode, d.category || "other");
        }
      });
  }

  // Create node groups
  const node = g
    .append("g")
    .selectAll("g")
    .data(filteredData.value.nodes)
    .join("g");

  // Note: Drag handlers will be added by renderStaticLayout()

  // Add circles for nodes (background for images, or solid color if no image)
  node
    .append("circle")
    .attr("r", (d: GraphNode) => {
      if (d.group === "center") return 30;
      if (d.type === "anime") return 25;
      if (d.type === "category") return 20;
      return 20; // Regular staff
    })
    .attr("fill", (d: GraphNode) => {
      // Only show colored fill if no image available
      if (d.type === "staff" && d.image) return "#ffffff";
      if (d.group === "center") return d.image ? "#ffffff" : themePrimary;
      if (d.type === "anime") return d.image ? "#ffffff" : "#4caf50";
      if (d.type === "category") return categoryColors[d.category || "other"];
      return "#ff9800";
    })
    .attr("stroke-width", 2);

  // Add circular pattern definitions for images
  const defs = svg.append("defs");

  filteredData.value.nodes.forEach((d: GraphNode) => {
    if (d.image && d.type !== "category") {
      const radius = d.group === "center" ? 30 : d.type === "anime" ? 25 : 20;
      const size = radius * 2;
      const patternId = `img-${String(d.id).replace(/[^a-zA-Z0-9]/g, "_")}`;

      const pattern = defs
        .append("pattern")
        .attr("id", patternId)
        .attr("width", 1)
        .attr("height", 1)
        .attr("patternContentUnits", "objectBoundingBox");

      pattern
        .append("image")
        .attr("xlink:href", d.image)
        .attr("width", 1)
        .attr("height", 1)
        .attr("preserveAspectRatio", "xMidYMid slice");
    }
  });

  // Add image-filled circles on top for nodes with images
  node
    .filter((d: GraphNode) => d.type !== "category" && d.image)
    .append("circle")
    .attr("r", (d: GraphNode) => {
      if (d.group === "center") return 30;
      if (d.type === "anime") return 25;
      return 20; // staff
    })
    .attr("fill", (d: GraphNode) => {
      const patternId = `img-${String(d.id).replace(/[^a-zA-Z0-9]/g, "_")}`;
      return `url(#${patternId})`;
    });

  // Add icon for category nodes (folder/group icon)
  node
    .filter((d: GraphNode) => d.type === "category")
    .append("text")
    .text("👥") // Group icon
    .attr("x", 0)
    .attr("y", 5)
    .attr("text-anchor", "middle")
    .attr("font-size", "20px")
    .style("filter", "drop-shadow(0 0 1px white) drop-shadow(0 0 1px white)"); // Mild white glow for all category nodes

  // Add heart icon for favorited anime nodes
  if (showFavoriteIcon.value) {
    node
      .filter((d: GraphNode) => {
        if (d.type !== "anime") return false;
        const animeId = parseInt(String(d.id));
        return favoritedAnimeIds.value.has(animeId);
      })
      .each(function () {
        appendFavoriteHeart(select(this));
      });
  }

  // Add labels - always show
  node
    .append("text")
    .text((d: GraphNode) => {
      // Truncate long anime titles to prevent overlaps
      if (d.type === "anime" && d.label.length > 30) {
        return d.label.substring(0, 27) + "...";
      }
      return d.label;
    })
    .attr("x", 0)
    .attr("y", (d: GraphNode) =>
      d.group === "center" ? 45 : d.type === "anime" ? 40 : 35,
    )
    .attr("text-anchor", "middle")
    .attr("font-size", "13px")
    .attr(
      "font-family",
      '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    )
    .attr("fill", "#111")
    .attr("font-weight", "700")
    .style("text-shadow", "0 0 3px white, 0 0 3px white")
    .style("max-width", "200px");

  // Note: Click handling is done in drag 'end' event to avoid conflicts with drag behavior

  // Pre-build a map of staffId → roles for O(1) lookup during hover
  staffRolesMap.clear();
  filteredData.value.links.forEach((link: GraphLink) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;
    if (
      sourceId === filteredData.value!.center ||
      targetId === filteredData.value!.center
    ) {
      const staffId =
        sourceId === filteredData.value!.center ? targetId : sourceId;
      const role = link.role;
      if (role) {
        const roles = Array.isArray(role) ? role : [role];
        const existing = staffRolesMap.get(staffId) || [];
        staffRolesMap.set(staffId, [...existing, ...roles]);
      }
    }
  });

  // Add tooltips (but NOT for anime or staff nodes - they use custom hover previews)
  node
    .filter(
      (d: GraphNode) =>
        d.type === "category" ||
        (d.type === "anime" && d.id === filteredData.value!.center),
    )
    .append("title")
    .text((d: GraphNode) => {
      if (d.type === "category") {
        // Category node: show list of staff
        const staffList = d.staffList || [];
        const staffNames = staffList
          .map((s) => `${s.name}${s.role ? ` (${s.role})` : ""}`)
          .join("\n");
        return `${d.label}\n\n${staffNames}`;
      }
      if (d.type === "anime") {
        // Show format for anime nodes (TV, Movie, OVA, etc.)
        return `${d.label} (${d.format ? formatAnimeFormat(d.format) : "anime"})`;
      }
      return `${d.label} (${d.type})`;
    });

  // Use static layout (no physics simulation)
  renderStaticLayout(svg, g, width, height, link, node, zoom);

  // Apply legend highlights immediately so the graph never flashes in an unhighlighted state
  if (effectiveLegendGroups.value.size > 0) {
    applyLegendHighlight(effectiveLegendGroups.value);
  }
};

// Returns an SVG path `d` string for a (possibly curved) edge.
// offset=0 → straight line; non-zero → quadratic bezier bent perpendicularly.
const computeLinkPath = (
  x1: number,
  y1: number,
  x2: number,
  y2: number,
  offset: number,
): string => {
  if (offset === 0) return `M${x1},${y1}L${x2},${y2}`;
  const dx = x2 - x1,
    dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy);
  if (len === 0) return `M${x1},${y1}L${x2},${y2}`;
  // Perpendicular unit vector (rotated 90°)
  const px = -dy / len,
    py = dx / len;
  // Control point at midpoint + perpendicular offset
  const cx = (x1 + x2) / 2 + px * offset;
  const cy = (y1 + y2) / 2 + py * offset;
  return `M${x1},${y1}Q${cx},${cy} ${x2},${y2}`;
};

const renderStaticLayout = (
  svg: any,
  g: any,
  width: number,
  height: number,
  link: any,
  node: any,
  zoom: any,
) => {
  // PERFORMANCE: Create node lookup map for O(1) access
  const nodeMap = new Map<string | number, GraphNode>();
  filteredData.value!.nodes.forEach((node) => {
    nodeMap.set(node.id, node);
  });

  // Calculate static positions with linear layout (left to right)
  const centerNode = nodeMap.get(filteredData.value!.center);
  const staffNodes = filteredData.value!.nodes.filter(
    (n) => n.type === "staff" || n.type === "category",
  );
  const animeNodes = filteredData.value!.nodes.filter(
    (n) => n.type === "anime" && n.id !== filteredData.value!.center,
  );

  // Count connections for each node
  // In category mode, use staffCount from links to count actual staff members
  // In normal mode, each link represents 1 staff member
  const connectionCounts = new Map<string, number>();
  filteredData.value!.links.forEach((link) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;

    // Use staffCount if available (category mode), otherwise count as 1 (normal mode)
    const count = link.staffCount || 1;

    connectionCounts.set(
      sourceId,
      (connectionCounts.get(sourceId) || 0) + count,
    );
    connectionCounts.set(
      targetId,
      (connectionCounts.get(targetId) || 0) + count,
    );
  });

  // Sort staff by category first, then by connection count within each category
  const sortedStaff = [...staffNodes].sort((a, b) => {
    const aCategory = a.category || "other";
    const bCategory = b.category || "other";

    // First sort by category order
    const aCategoryIndex = CATEGORY_ORDER.indexOf(aCategory);
    const bCategoryIndex = CATEGORY_ORDER.indexOf(bCategory);

    if (aCategoryIndex !== bCategoryIndex) {
      return aCategoryIndex - bCategoryIndex;
    }

    // Within same category, sort by connection count (descending)
    const aCount = connectionCounts.get(a.id) || 0;
    const bCount = connectionCounts.get(b.id) || 0;
    return bCount - aCount;
  });

  // Sort anime based on graphSortMode + graphSortOrder
  const sortedAnime = [...animeNodes].sort((a, b) => {
    let cmp: number;
    if (graphSortMode.value === "title") {
      cmp = a.label.localeCompare(b.label);
    } else if (graphSortMode.value === "rating") {
      const aScore = a.averageScore ?? 0;
      const bScore = b.averageScore ?? 0;
      cmp =
        aScore !== bScore ? aScore - bScore : a.label.localeCompare(b.label);
    } else {
      // connections
      const aCount = connectionCounts.get(a.id) || 0;
      const bCount = connectionCounts.get(b.id) || 0;
      cmp = aCount - bCount;
    }
    return graphSortOrder.value === "desc" ? -cmp : cmp;
  });

  // Layout parameters
  const padding = 100;
  const nodeSpacing = 60;

  // Position center anime on far left
  if (centerNode) {
    centerNode.x = padding;
    centerNode.y = height / 2;
  }

  // Position staff in middle column, vertically centered and sorted by connections
  // Ensure minimum distance from center anime (node sizes + labels need ~150px clearance)
  const minStaffX = padding + 150;
  const staffX = Math.max(width * 0.4, minStaffX);
  const staffTotalHeight = sortedStaff.length * nodeSpacing;
  const staffStartY = (height - staffTotalHeight) / 2 + nodeSpacing / 2;

  sortedStaff.forEach((node, i) => {
    node.x = staffX;
    node.y = staffStartY + i * nodeSpacing;
  });

  // Position anime on right side - add columns as needed to avoid overlaps
  // In anime-only mode (no staff column), position anime closer to center
  const minAnimeStartX = sortedStaff.length > 0 ? staffX + 150 : padding + 200;
  const animeStartX =
    sortedStaff.length > 0
      ? Math.max(width * 0.65, minAnimeStartX)
      : Math.max(width * 0.35, minAnimeStartX);
  const animeEndX = Math.max(width - padding, animeStartX + 50);
  const availableWidth = animeEndX - animeStartX;

  // Minimum spacing to prevent label/node overlaps
  const minAnimeVerticalSpacing = 70; // Node (50px diameter) + label (40px) + gap

  // Maximum rows we can fit without overlaps (constrained by staff height, min 10 to avoid short columns)
  // In anime-only mode (hideStaffNodes), use the number of distinct categories from edges
  // so the layout matches what it would look like with staff/category nodes visible
  let maxRows: number;
  if (sortedStaff.length > 0) {
    maxRows = Math.max(
      10,
      Math.floor(staffTotalHeight / minAnimeVerticalSpacing),
    );
  } else if (hideStaffNodes.value) {
    const edgeCategories = new Set<string>();
    filteredData.value!.links.forEach((l) => {
      if (l.category) edgeCategories.add(l.category);
    });
    const virtualStaffHeight = Math.max(edgeCategories.size, 10) * nodeSpacing;
    maxRows = Math.max(
      10,
      Math.floor(virtualStaffHeight / minAnimeVerticalSpacing),
    );
  } else {
    maxRows = Math.max(
      1,
      Math.floor((height - 2 * padding) / minAnimeVerticalSpacing),
    );
  }

  // Calculate how many columns we need
  const columnsNeeded = Math.ceil(sortedAnime.length / maxRows);

  // Horizontal spacing between columns - increased to prevent title overlaps
  const columnSpacing = 220;

  // Check if last column (minimum items) fits in viewport at minimum zoom
  const minZoom = 0.1;
  const lastColumnItems = sortedAnime.length % maxRows || maxRows;
  const lastColumnHeight = lastColumnItems * minAnimeVerticalSpacing;
  const canDecompress = lastColumnHeight * minZoom <= height;

  sortedAnime.forEach((node, i) => {
    const col = Math.floor(i / maxRows);
    const row = i % maxRows;

    // X position
    if (columnsNeeded === 1) {
      node.x = animeEndX;
    } else {
      node.x = animeStartX + col * columnSpacing;
    }

    // Y position
    const itemsInColumn = Math.min(maxRows, sortedAnime.length - col * maxRows);
    const isPartialColumn = itemsInColumn < maxRows;

    // Calculate the full column height (what a complete column would use)
    const fullColumnHeight = maxRows * minAnimeVerticalSpacing;

    // Center all columns vertically in the viewport
    const columnCenterY = height / 2;

    if (canDecompress && isPartialColumn) {
      // Decompressed: Distribute partial column within the same range as full columns
      // This ensures all columns align properly
      const columnStartY =
        columnCenterY - fullColumnHeight / 2 + minAnimeVerticalSpacing / 2;
      const columnEndY =
        columnCenterY + fullColumnHeight / 2 - minAnimeVerticalSpacing / 2;

      if (itemsInColumn === 1) {
        // Single item: center within the column range
        node.y = columnCenterY;
      } else {
        // Multiple items: distribute evenly within the full column range
        const spacing = (columnEndY - columnStartY) / (itemsInColumn - 1);
        node.y = columnStartY + row * spacing;
      }
    } else {
      // Compressed: Use constant 70px spacing, centered vertically
      const columnHeight = itemsInColumn * minAnimeVerticalSpacing;
      const columnStartY =
        columnCenterY - columnHeight / 2 + minAnimeVerticalSpacing / 2;

      node.y = columnStartY + row * minAnimeVerticalSpacing;
    }
  });

  // Render immediately with calculated positions (no tick needed)
  if (hideStaffNodes.value) {
    link.attr("d", (d: any) => {
      const srcNode =
        typeof d.source === "object" ? d.source : nodeMap.get(d.source);
      const tgtNode =
        typeof d.target === "object" ? d.target : nodeMap.get(d.target);
      return computeLinkPath(
        srcNode?.x ?? 0,
        srcNode?.y ?? 0,
        tgtNode?.x ?? 0,
        tgtNode?.y ?? 0,
        d.parallelOffset ?? 0,
      );
    });
  } else {
    link
      .attr("x1", (d: any) => {
        const sourceNode =
          typeof d.source === "object" ? d.source : nodeMap.get(d.source);
        return sourceNode?.x || 0;
      })
      .attr("y1", (d: any) => {
        const sourceNode =
          typeof d.source === "object" ? d.source : nodeMap.get(d.source);
        return sourceNode?.y || 0;
      })
      .attr("x2", (d: any) => {
        const targetNode =
          typeof d.target === "object" ? d.target : nodeMap.get(d.target);
        return targetNode?.x || 0;
      })
      .attr("y2", (d: any) => {
        const targetNode =
          typeof d.target === "object" ? d.target : nodeMap.get(d.target);
        return targetNode?.y || 0;
      });
  }

  node.attr("transform", (d: any) => `translate(${d.x},${d.y})`);

  // Add drag handlers for performance mode (direct position updates, no physics)
  // Track start position to distinguish clicks from drags
  let dragStartX = 0;
  let dragStartY = 0;
  let hasDragged = false;

  node.call(
    drag<any, GraphNode>()
      .on("start", function (event: any, d: GraphNode) {
        select(this).raise();
        dragStartX = event.x;
        dragStartY = event.y;
        hasDragged = false;
      })
      .on("drag", function (event: any, d: GraphNode) {
        // Check if actually dragged (movement > 5 pixels)
        const dx = event.x - dragStartX;
        const dy = event.y - dragStartY;
        if (Math.abs(dx) > 5 || Math.abs(dy) > 5) {
          hasDragged = true;
        }

        d.x = event.x;
        d.y = event.y;
        select(this).attr("transform", `translate(${d.x},${d.y})`);

        // Update connected links
        const connectedLinks = link.filter((l: any) => {
          const sourceId =
            typeof l.source === "object" ? l.source.id : l.source;
          const targetId =
            typeof l.target === "object" ? l.target.id : l.target;
          return sourceId === d.id || targetId === d.id;
        });
        if (hideStaffNodes.value) {
          connectedLinks.attr("d", (l: any) => {
            const srcNode =
              typeof l.source === "object" ? l.source : nodeMap.get(l.source);
            const tgtNode =
              typeof l.target === "object" ? l.target : nodeMap.get(l.target);
            return computeLinkPath(
              srcNode?.x ?? 0,
              srcNode?.y ?? 0,
              tgtNode?.x ?? 0,
              tgtNode?.y ?? 0,
              l.parallelOffset ?? 0,
            );
          });
        } else {
          connectedLinks
            .attr("x1", (l: any) => {
              const sourceNode =
                typeof l.source === "object" ? l.source : nodeMap.get(l.source);
              return sourceNode?.x || 0;
            })
            .attr("y1", (l: any) => {
              const sourceNode =
                typeof l.source === "object" ? l.source : nodeMap.get(l.source);
              return sourceNode?.y || 0;
            })
            .attr("x2", (l: any) => {
              const targetNode =
                typeof l.target === "object" ? l.target : nodeMap.get(l.target);
              return targetNode?.x || 0;
            })
            .attr("y2", (l: any) => {
              const targetNode =
                typeof l.target === "object" ? l.target : nodeMap.get(l.target);
              return targetNode?.y || 0;
            });
        }
      })
      .on("end", function (event: any, d: GraphNode) {
        // If didn't actually drag, treat as a click
        if (!hasDragged) {
          selectedNode.value = d;
          // Hide hover preview when clicking
          hoveredAnimeNode.value = null;
          // Trigger the click handler
          handleAnimeNodeClick(d);
        }
      }),
  );

  // Add hover handlers to circles only (not labels) for anime nodes to show preview
  node
    .filter(
      (d: GraphNode) =>
        d.type === "anime" && d.id !== filteredData.value!.center,
    )
    .selectAll("circle")
    .on("mouseenter", function (event: any, d: GraphNode) {
      if (!hoverHighlight.value) {
        // Highlight off — show preview card immediately, no debounce
        const pos = calcHoverPosition(d, svg.node(), ANIME_HOVER_CONFIG);
        hoveredAnimeNode.value = { node: d, ...pos };
        return;
      }

      // Highlight on — debounce both card and highlight together
      if (hoverRestoreTimeout) {
        clearTimeout(hoverRestoreTimeout);
        hoverRestoreTimeout = null;
      }
      if (hoverActivationTimeout) clearTimeout(hoverActivationTimeout);

      if (hoveredAnimeNodeId.value !== null) {
        // Already highlighting — crossfade immediately to the new node
        const pos = calcHoverPosition(d, svg.node(), ANIME_HOVER_CONFIG);
        hoveredAnimeNode.value = { node: d, ...pos };
        hoveredAnimeNodeId.value = d.id;
      } else {
        // Not currently highlighted — debounce to prevent accidental activation
        hoverActivationTimeout = setTimeout(() => {
          hoverActivationTimeout = null;
          const pos = calcHoverPosition(d, svg.node(), ANIME_HOVER_CONFIG);
          hoveredAnimeNode.value = { node: d, ...pos };
          hoveredAnimeNodeId.value = d.id;
        }, 160);
      }
    })
    .on("mouseleave", function (event: any, d: GraphNode) {
      hoveredAnimeNode.value = null;

      if (!hoverHighlight.value) return;

      if (hoverActivationTimeout) {
        clearTimeout(hoverActivationTimeout);
        hoverActivationTimeout = null;
      }
      // Only schedule a restore if this node is the currently highlighted one
      if (hoveredAnimeNodeId.value === d.id) {
        if (hoverRestoreTimeout) clearTimeout(hoverRestoreTimeout);
        hoverRestoreTimeout = setTimeout(() => {
          hoverRestoreTimeout = null;
          hoveredAnimeNodeId.value = null;
        }, 30);
      }
    })
    .on("mousemove", function (event: any, d: GraphNode) {
      if (hoveredAnimeNode.value && hoveredAnimeNode.value.node.id === d.id) {
        const pos = calcHoverPosition(d, svg.node(), ANIME_HOVER_CONFIG);
        hoveredAnimeNode.value = { node: d, ...pos };
      }
    });

  // Add hover handlers for staff nodes to show preview card
  node
    .filter((d: GraphNode) => d.type === "staff")
    .selectAll("circle")
    .on("mouseenter", function (event: any, d: GraphNode) {
      const pos = calcHoverPosition(d, svg.node(), STAFF_HOVER_CONFIG);
      hoveredStaffNode.value = {
        node: d,
        ...pos,
        roles: staffRolesMap.get(d.id) || [],
      };
    })
    .on("mouseleave", function () {
      hoveredStaffNode.value = null;
    })
    .on("mousemove", function (event: any, d: GraphNode) {
      if (hoveredStaffNode.value && hoveredStaffNode.value.node.id === d.id) {
        const pos = calcHoverPosition(d, svg.node(), STAFF_HOVER_CONFIG);
        hoveredStaffNode.value = {
          node: d,
          ...pos,
          roles: staffRolesMap.get(d.id) || [],
        };
      }
    });

  // Calculate bounds and fit zoom to show all content
  const allNodes = [...sortedStaff, ...sortedAnime];
  if (centerNode) allNodes.push(centerNode);

  if (allNodes.length > 0) {
    // Calculate bounding box with padding for labels
    const labelPadding = 60; // Space for labels below nodes
    const nodePadding = 30; // Node radius

    let minX = Infinity,
      maxX = -Infinity,
      minY = Infinity,
      maxY = -Infinity;
    allNodes.forEach((n) => {
      if (n.x !== undefined && n.y !== undefined) {
        minX = Math.min(minX, n.x - nodePadding);
        maxX = Math.max(maxX, n.x + nodePadding);
        minY = Math.min(minY, n.y - nodePadding);
        maxY = Math.max(maxY, n.y + labelPadding); // Extra space for labels
      }
    });

    // Add margin around the content
    const margin = 50;
    minX -= margin;
    maxX += margin;
    minY -= margin;
    maxY += margin;

    const contentWidth = maxX - minX;
    const contentHeight = maxY - minY;

    // Calculate scale to fit content
    const scaleX = width / contentWidth;
    const scaleY = height / contentHeight;
    const scale = Math.min(scaleX, scaleY, 1); // Don't zoom in past 1x

    // Calculate translation to center the content
    const contentCenterX = (minX + maxX) / 2;
    const contentCenterY = (minY + maxY) / 2;
    const translateX = width / 2 - contentCenterX * scale;
    const translateY = height / 2 - contentCenterY * scale;

    // Apply the initial transform
    const initialTransform = zoomIdentity
      .translate(translateX, translateY)
      .scale(scale);

    // Apply the transform using the passed zoom behavior
    svg.call(zoom.transform as any, initialTransform);
    initialTransformRef = initialTransform;
  }
};

watch(
  () => props.animeId,
  () => {
    console.log(
      "[GraphVis] animeId watcher fired:",
      props.animeId,
      "initialGraphData center:",
      props.initialGraphData?.center,
      "isFirstLoad:",
      isFirstLoad.value,
    );
    // Suppress watchers while resetting state for the new anime
    isInitialLoad.value = true;
    isFirstLoad.value = true;

    // Reset filter state that may have been modified by auto-adjust or user interaction
    selectedCategories.value = [
      ...STAFF_CATEGORIES.filter((cat) => cat.key !== "production").map(
        (cat) => cat.key,
      ),
      "other",
    ];
    hideProducers.value = true;
    sameRoleOnly.value = true;
    useCategoryNodes.value = false;
    hideStaffNodes.value = false;
    selectedFormats.value = [];
    minConnections.value = 2;
    recReset();
    lastRenderedFingerprint = 0;

    // Clear stale graph data and SVG so old graph doesn't flash
    graphData.value = null;
    filteredData.value = null;
    if (simulation) {
      simulation.stop();
      simulation = null;
    }
    if (graphRef.value) {
      select(graphRef.value).selectAll("*").remove();
    }

    // Clear stale UI state
    selectedAnime.value = null;
    hoveredAnimeNode.value = null;
    hoveredStaffNode.value = null;
    hoveredEdgeLabel.value = null;
    hoveredEdgeCategory.value = null;
    hoveredEdgeSingleStaff.value = null;
    expandedRoleCards.value = new Set();

    // If parent provides preloaded data, it will arrive via the initialGraphData watcher.
    // Otherwise, fetch directly.
    console.log(
      "[GraphVis] animeId watcher: initialGraphData available?",
      !!props.initialGraphData,
      "center:",
      props.initialGraphData?.center,
    );
    if (!props.initialGraphData) {
      console.log(
        "[GraphVis] animeId watcher: no initialGraphData, calling fetchGraphData()",
      );
      fetchGraphData();
    } else {
      console.log(
        "[GraphVis] animeId watcher: has initialGraphData, skipping fetch (waiting for initialGraphData watcher)",
      );
    }

    // Update mainAnimeData from parent prop if available
    if (props.animeData) {
      mainAnimeData.value = props.animeData;
      isLoadingMainAnime.value = false;
    }

    // If in recommended mode, refetch recommendations
    if (recommendationMode.value === "recommended") {
      fetchRecommendedAnime(1);
    }
  },
);

// When parent provides new preloaded graph data (e.g. after client-side navigation),
// use it directly instead of fetching
watch(
  () => props.initialGraphData,
  (newData, oldData) => {
    console.log(
      "[GraphVis] initialGraphData watcher fired:",
      "new center:",
      newData?.center,
      "old center:",
      oldData?.center,
      "isFirstLoad:",
      isFirstLoad.value,
      "isInitialLoad:",
      isInitialLoad.value,
    );
    if (!newData) {
      console.log(
        "[GraphVis] initialGraphData watcher: newData is null/undefined, skipping",
      );
      return;
    }
    // If the graph already rendered this center's data (e.g. onMounted fetched via API),
    // skip to avoid a redundant second render when the parent's lazy async data arrives.
    if (
      graphData.value &&
      String(graphData.value.center) === String(newData.center)
    ) {
      console.log(
        "[GraphVis] initialGraphData watcher: same center already rendered, skipping",
      );
      return;
    }
    console.log(
      "[GraphVis] initialGraphData watcher: calling fetchGraphData with preloaded data",
    );
    fetchGraphData(newData);
  },
);

// Keep mainAnimeData in sync with parent prop
watch(
  () => props.animeData,
  (newData) => {
    if (newData) {
      mainAnimeData.value = newData;
      isLoadingMainAnime.value = false;
    }
  },
);

watch(graphViewMode, (newMode) => {
  // Re-render graph when switching back to graph view
  if (newMode === "graph" && graphData.value) {
    // Small delay to ensure DOM is ready
    setTimeout(() => {
      updateFilteredData();
      renderGraph();
      lastRenderedFingerprint = computeFilteredDataFingerprint(
        filteredData.value,
      );
    }, 100);
  }
});

watch(selectedCategories, () => {
  console.log(
    "[GraphVis] selectedCategories watcher fired. isInitialLoad:",
    isInitialLoad.value,
    "isSyncingFilters:",
    isSyncingFilters.value,
  );
  if (graphData.value && !isSyncingFilters.value && !isInitialLoad.value) {
    console.log(
      "[GraphVis] selectedCategories watcher: syncing staff + scheduling render",
    );
    // Sync staff selection with category filters
    syncStaffToCategories();

    scheduleRender();
  }
});

watch(useCategoryNodes, () => {
  if (graphData.value && !isInitialLoad.value) {
    updateFilteredData();
    renderGraphIfChanged();
  }
});

watch(hideStaffNodes, () => {
  if (graphData.value && !isInitialLoad.value) {
    updateFilteredData();
    renderGraphIfChanged();
  }
});

watch([graphSortMode, graphSortOrder], () => {
  // Sort only affects layout, not topology — call renderGraph directly
  if (filteredData.value) {
    renderGraph();
  }
});

// Shared helper: apply legend group highlighting to the SVG.
// `groups` is the set of groups to highlight; empty means reset to default.
const HOVER_TRANSITION_MS = 120;

const applyLegendHighlight = (groups: Set<string>, animate = false) => {
  if (!graphRef.value) return;
  const svg = select(graphRef.value);
  const center = filteredData.value?.center;

  // Interrupt any running transitions so they don't overwrite values we set here.
  svg.selectAll(".graph-link").interrupt();
  svg.selectAll("g").interrupt();

  // Single shared transition so links + nodes animate on the exact same timer
  const t = animate
    ? (transition().duration(HOVER_TRANSITION_MS) as any)
    : null;
  const applyT = (sel: any) => (t ? sel.transition(t) : sel);

  if (groups.size === 0) {
    applyT(svg.selectAll(".graph-link"))
      .style("stroke-opacity", 0.6)
      .style("pointer-events", null);
    svg.selectAll("g").each(function () {
      const d = select(this).datum() as any;
      if (d && d.type) applyT(select(this)).style("opacity", 1);
    });
    return;
  }

  const blockDimmed = !hoverDimmedEdges.value;
  applyT(svg.selectAll(".graph-link"))
    .style("stroke-opacity", (d: any) => {
      const linkGroup =
        CATEGORY_TO_GROUP[d.category || "other"] || d.category || "other";
      return groups.has(linkGroup) ? 0.85 : 0.05;
    })
    .style("pointer-events", (d: any) => {
      if (!blockDimmed) return null;
      const linkGroup =
        CATEGORY_TO_GROUP[d.category || "other"] || d.category || "other";
      return groups.has(linkGroup) ? null : "none";
    });

  svg.selectAll("g").each(function () {
    const d = select(this).datum() as any;
    if (!d || !d.type) return;
    let opacity = 1;
    if (d.id !== center && (d.type === "staff" || d.type === "category")) {
      const nodeGroup =
        CATEGORY_TO_GROUP[d.category || "other"] || d.category || "other";
      opacity = groups.has(nodeGroup) ? 1 : 0.1;
    }
    applyT(select(this)).style("opacity", opacity);
  });
};

// Effective highlighted groups — driven by hover (transient) or click selection (persistent).
const effectiveLegendGroups = computed<Set<string>>(() => {
  if (hoveredLegendGroup.value) return new Set([hoveredLegendGroup.value]);
  return selectedLegendGroups.value;
});

// Legend highlighting — reacts to both hover and click selection changes
watch(effectiveLegendGroups, (groups) => {
  applyLegendHighlight(groups, true);
});

// Anime node hover highlighting — dim edges/nodes not connecting to the hovered anime
const applyNodeHoverHighlight = (animeId: string | number) => {
  if (!graphRef.value || !filteredData.value) return;
  const svg = select(graphRef.value);

  // Find all links and nodes connecting center → staff → hovered anime (or center → anime directly)
  const connectedStaffIds = new Set<string | number>();
  const connectedLinkIndices = new Set<number>();

  filteredData.value.links.forEach((link, idx) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;

    // Direct center → anime link (anime-only mode)
    if (sourceId === filteredData.value!.center && targetId === animeId) {
      connectedLinkIndices.add(idx);
      return;
    }

    // Staff → anime link to the hovered anime
    if (targetId === animeId) {
      connectedLinkIndices.add(idx);
      connectedStaffIds.add(sourceId);
    }
  });

  // Also highlight center → connected staff links
  filteredData.value.links.forEach((link, idx) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;

    if (
      sourceId === filteredData.value!.center &&
      connectedStaffIds.has(targetId)
    ) {
      connectedLinkIndices.add(idx);
    }
  });

  // Interrupt any stale transitions and crossfade everything to the new highlight state
  svg.selectAll(".graph-link").interrupt();
  svg.selectAll("g").interrupt();

  const t = transition().duration(HOVER_TRANSITION_MS) as any;

  svg
    .selectAll(".graph-link")
    .transition(t)
    .style("stroke-opacity", (_d: any, i: number) =>
      connectedLinkIndices.has(i) ? 0.85 : 0.05,
    );

  svg
    .selectAll("g")
    .filter(function () {
      const d = select(this).datum() as any;
      return d && d.type;
    })
    .transition(t)
    .style("opacity", (d: any) => {
      if (d.id === filteredData.value?.center) return 1;
      if (d.id === animeId) return 1;
      if (connectedStaffIds.has(d.id)) return 1;
      return 0.1;
    });
};

let hoverActivationTimeout: ReturnType<typeof setTimeout> | null = null;
let hoverRestoreTimeout: ReturnType<typeof setTimeout> | null = null;

watch(hoveredAnimeNodeId, (animeId) => {
  if (!graphRef.value || !hoverHighlight.value) return;

  if (!animeId) {
    applyLegendHighlight(effectiveLegendGroups.value, true);
    return;
  }

  applyNodeHoverHighlight(animeId);
});

// When hover highlight is toggled off, restore full opacity immediately
watch(hoverHighlight, (enabled) => {
  if (!enabled) {
    applyLegendHighlight(effectiveLegendGroups.value);
  }
});

// Reapply legend highlight when hoverDimmedEdges changes so pointer-events update live
watch(hoverDimmedEdges, () => {
  applyLegendHighlight(effectiveLegendGroups.value);
});

watch(
  [
    selectedFormats,
    sameRoleOnly,
    minConnections,
    hideLonelyStaff,
    hideRelatedAnime,
    showFavorites,
  ],
  () => {
    console.log(
      "[GraphVis] filter watcher fired. isInitialLoad:",
      isInitialLoad.value,
      "selectedFormats:",
      [...selectedFormats.value],
      "graphData:",
      !!graphData.value,
    );
    if (graphData.value && !isInitialLoad.value) {
      console.log("[GraphVis] filter watcher: scheduling render");
      scheduleRender();
    } else {
      console.log(
        "[GraphVis] filter watcher: SKIPPED (isInitialLoad:",
        isInitialLoad.value,
        ")",
      );
    }
  },
);

// Auto-adjust minConnections when the available range changes
watch(minConnectionsOptions, (newOptions) => {
  if (!isInitialLoad.value && newOptions.length > 0) {
    const maxOption = Math.max(...newOptions);
    const minOption = Math.min(...newOptions);

    // Clamp to nearest valid boundary if outside available range
    if (minConnections.value > maxOption) {
      minConnections.value = maxOption;
    } else if (minConnections.value < minOption) {
      minConnections.value = minOption;
    }
  }
});

// Watch hideProducers to add/remove production category from selectedCategories
watch(hideProducers, (newValue) => {
  console.log(
    "[GraphVis] hideProducers watcher fired:",
    newValue,
    "isInitialLoad:",
    isInitialLoad.value,
  );
  if (newValue) {
    // Remove production category if it's selected
    selectedCategories.value = selectedCategories.value.filter(
      (cat) => cat !== "production",
    );

    // Also remove all producer staff from selectedStaff to prevent the glitch
    // where producers are added to the graph when selected from the staff list
    if (categorizedStaff.value["production"]) {
      const producerStaffIds = categorizedStaff.value["production"]
        .map((s: any) => s.staff?.staff_id)
        .filter(Boolean);
      producerStaffIds.forEach((id: string) => selectedStaff.value.delete(id));
      // Trigger reactivity
      selectedStaff.value = new Set(selectedStaff.value);
    }
  } else {
    // Add production category if it's not already selected
    if (!selectedCategories.value.includes("production")) {
      selectedCategories.value = [...selectedCategories.value, "production"];
    }

    // Add all producer staff to selectedStaff when showing producers
    if (categorizedStaff.value["production"]) {
      const producerStaffIds = categorizedStaff.value["production"]
        .map((s: any) => s.staff?.staff_id)
        .filter(Boolean);
      producerStaffIds.forEach((id: string) => selectedStaff.value.add(id));
      // Trigger reactivity
      selectedStaff.value = new Set(selectedStaff.value);
    }
  }
});

// Reset to page 1 when recommendations change (staff-based modes only)

// Watch recommendation filters and update both graph and counts when they change
watch(
  recommendationFilters,
  () => {
    if (!isInitialLoad.value) {
      scheduleRender();
      fetchRecommendationFilterCounts();
      updateGraphURL();
    }
  },
  { deep: true },
);

// Persist graph filter state to URL (only after initial load so auto-adjust doesn't pollute URL)
watch(
  [graphViewMode, recommendationMode, selectedFormats, minConnections],
  () => {
    if (!isInitialLoad.value) updateGraphURL();
  },
);

// Watch for changes in graph data or filter metadata to update counts
watch([() => filteredData.value?.nodes.length, filterMetadataLoaded], () => {
  if (filteredData.value && filterMetadataLoaded.value) {
    // Small delay to ensure graph is fully updated
    nextTick(() => {
      fetchRecommendationFilterCounts();
    });
  }
});

// Sync d3 favorite hearts whenever favoritedAnimeIds changes (covers both
// the overlay button inside this component AND the button on the parent page)
watch(favoritedAnimeIds, (newSet) => {
  if (!graphRef.value || !showFavoriteIcon.value) return;
  select(graphRef.value)
    .selectAll("g")
    .filter((d: any) => d && d.type === "anime")
    .each(function () {
      const id = parseInt(String((select(this).datum() as GraphNode).id));
      const nodeG = select(this);
      const hasSvgHeart = !nodeG.select(".favorite-heart").empty();
      const isFav = newSet.has(id);

      if (isFav && !hasSvgHeart) {
        appendFavoriteHeart(nodeG);
      } else if (!isFav && hasSvgHeart) {
        nodeG.select(".favorite-heart").remove();
      }
    });
});

// Toggle favorite icons on/off without full re-render
watch(showFavoriteIcon, (show) => {
  if (!graphRef.value) return;
  if (!show) {
    select(graphRef.value).selectAll(".favorite-heart").remove();
  } else {
    select(graphRef.value)
      .selectAll("g")
      .filter(
        (d: any) =>
          d &&
          d.type === "anime" &&
          favoritedAnimeIds.value.has(parseInt(String(d.id))),
      )
      .each(function () {
        const nodeG = select(this);
        if (nodeG.select(".favorite-heart").empty()) {
          appendFavoriteHeart(nodeG);
        }
      });
  }
});

// Function to close all tooltips
const closeAllTooltips = () => {
  graphViewTooltip.value = false;
  staffViewTooltip.value = false;
};

// URL state persistence for graph filters (g_ prefix to avoid conflicts with page search params)
let graphUrlTimeout: ReturnType<typeof setTimeout> | null = null;

const updateGraphURL = () => {
  if (graphUrlTimeout) clearTimeout(graphUrlTimeout);
  graphUrlTimeout = setTimeout(() => {
    const query: Record<string, any> = { ...route.query };

    // graphViewMode — omit when default ('graph')
    if (graphViewMode.value !== "graph") {
      query.g_view = graphViewMode.value;
    } else {
      delete query.g_view;
    }

    // recommendationMode — omit when default ('filtered')
    if (recommendationMode.value !== "filtered") {
      query.g_mode = recommendationMode.value;
    } else {
      delete query.g_mode;
    }

    // selectedFormats — omit when empty (means show all)
    if (selectedFormats.value.length > 0) {
      query.g_formats = selectedFormats.value.join(",");
    } else {
      delete query.g_formats;
    }

    // minConnections — omit when at minimum floor (auto-adjusted default)
    const floor = graphData.value?.minConnectionsFloor ?? 2;
    if (minConnections.value > floor) {
      query.g_min = String(minConnections.value);
    } else {
      delete query.g_min;
    }

    // recommendationFilters.genres
    if (recommendationFilters.value.genres.length > 0) {
      query.g_genres = recommendationFilters.value.genres.join(",");
    } else {
      delete query.g_genres;
    }

    // recommendationFilters.tags
    if (recommendationFilters.value.tags.length > 0) {
      query.g_tags = recommendationFilters.value.tags.join(",");
    } else {
      delete query.g_tags;
    }

    router.replace({ path: route.path, query });
  }, 300);
};

// Apply URL params to graph state — called after auto-adjust, before first render,
// so URL params override auto-adjust without causing a second render pass.
const applyURLState = () => {
  const query = route.query;
  const hasAnyGraphParam =
    query.g_view ||
    query.g_mode ||
    query.g_formats ||
    query.g_min ||
    query.g_genres ||
    query.g_tags;
  if (!hasAnyGraphParam) return;

  if (query.g_view === "graph" || query.g_view === "staff") {
    graphViewMode.value = query.g_view as "graph" | "staff";
  }

  if (
    query.g_mode &&
    ["filtered", "all", "recommended"].includes(String(query.g_mode))
  ) {
    recommendationMode.value = query.g_mode as
      | "filtered"
      | "all"
      | "recommended";
  }

  if (query.g_formats) {
    const formats = String(query.g_formats).split(",").filter(Boolean);
    if (formats.length > 0) selectedFormats.value = formats;
  }

  if (query.g_min) {
    const min = parseInt(String(query.g_min));
    if (!isNaN(min) && min > 0) minConnections.value = min;
  }

  if (query.g_genres) {
    const genres = String(query.g_genres).split(",").filter(Boolean);
    if (genres.length > 0)
      recommendationFilters.value = { ...recommendationFilters.value, genres };
  }

  if (query.g_tags) {
    const tags = String(query.g_tags).split(",").filter(Boolean);
    if (tags.length > 0)
      recommendationFilters.value = { ...recommendationFilters.value, tags };
  }
};

// Programmatic zoom controls (keyboard shortcuts)
const zoomIn = () => {
  if (!svgSelectionRef || !zoomBehaviorRef) return;
  svgSelectionRef
    .transition()
    .duration(250)
    .call((zoomBehaviorRef as any).scaleBy, 1.4);
};

const zoomOut = () => {
  if (!svgSelectionRef || !zoomBehaviorRef) return;
  svgSelectionRef
    .transition()
    .duration(250)
    .call((zoomBehaviorRef as any).scaleBy, 1 / 1.4);
};

const resetZoom = () => {
  if (!svgSelectionRef || !zoomBehaviorRef || !initialTransformRef) return;
  svgSelectionRef
    .transition()
    .duration(400)
    .call((zoomBehaviorRef as any).transform, initialTransformRef);
};

useKeyboardShortcuts({
  Escape: () => {
    if (selectedAnime.value) closeOverlay();
  },
  f: () => {
    if (!isMobile.value) toggleFullscreen();
  },
  F: () => {
    if (!isMobile.value) toggleFullscreen();
  },
  "+": (e) => {
    if (graphViewMode.value === "graph" && graphOpen.value === 0) {
      zoomIn();
      e.preventDefault();
    }
  },
  "=": (e) => {
    if (graphViewMode.value === "graph" && graphOpen.value === 0) {
      zoomIn();
      e.preventDefault();
    }
  },
  "-": (e) => {
    if (graphViewMode.value === "graph" && graphOpen.value === 0) {
      zoomOut();
      e.preventDefault();
    }
  },
  "0": (e) => {
    if (graphViewMode.value === "graph" && graphOpen.value === 0) {
      resetZoom();
      e.preventDefault();
    }
  },
});

// Check if device is mobile
const checkMobile = () => {
  isMobile.value = window.innerWidth < 768;
};

// Handle resize events
const handleResize = () => {
  checkMobile();
  renderGraph();
  lastRenderedFingerprint = computeFilteredDataFingerprint(filteredData.value);
};

// Resize the SVG to fit its container without re-laying-out the graph
const resizeSvg = () => {
  const container = graphRef.value;
  if (!container) return;
  const svgEl = container.querySelector('svg');
  if (!svgEl) return;
  const width = container.clientWidth || 800;
  const height = isFullscreen.value && container.clientHeight > 100 ? container.clientHeight : 600;
  const svgSel = select(svgEl);
  svgSel.attr('width', width).attr('height', height).attr('viewBox', [0, 0, width, height]);
};

// ResizeObserver for container-level resizes (e.g. sidebar collapse)
let resizeObserver: ResizeObserver | null = null;
let resizeObserverTimeout: ReturnType<typeof setTimeout> | null = null;

// Fullscreen functions
const toggleFullscreen = async () => {
  if (!graphContainer.value) return;

  if (!isFullscreen.value) {
    try {
      await graphContainer.value.requestFullscreen();
    } catch (error) {
      console.error("Error entering fullscreen:", error);
    }
  } else {
    try {
      await document.exitFullscreen();
    } catch (error) {
      console.error("Error exiting fullscreen:", error);
    }
  }
};

const handleFullscreenChange = () => {
  isFullscreen.value = !!document.fullscreenElement;

  // Re-render graph when entering/exiting fullscreen to adjust layout
  if (graphData.value && graphViewMode.value === "graph") {
    setTimeout(() => {
      renderGraph();
      lastRenderedFingerprint = computeFilteredDataFingerprint(
        filteredData.value,
      );
    }, 100);
  }
};

onMounted(async () => {
  console.log(
    "[GraphVis] onMounted: animeId:",
    props.animeId,
    "initialGraphData center:",
    props.initialGraphData?.center,
  );
  // Use preloaded graph data from parent page's useAsyncData if available
  if (props.initialGraphData) {
    fetchGraphData(props.initialGraphData);
  } else {
    fetchGraphData();
  }

  // Use anime data from parent prop if available, otherwise fetch
  if (props.animeData) {
    mainAnimeData.value = props.animeData;
    isLoadingMainAnime.value = false;
  } else {
    try {
      const response = await api(
        `/anime/${encodeURIComponent(props.animeId)}`,
      );
      if (response.success && response.data) {
        mainAnimeData.value = response.data;
      }
    } catch (error) {
      console.error("Error fetching main anime data:", error);
    } finally {
      isLoadingMainAnime.value = false;
    }
  }

  // Fetch user's favorites (if authenticated)
  await fetchFavorites();

  // Proactively load filter metadata for genre/tag filtering
  if (!filterMetadataLoaded.value && !loadingFilterMetadata.value) {
    await loadFilterMetadata();
  }

  // Check if mobile on mount
  checkMobile();

  window.addEventListener("resize", handleResize);

  // Observe container resizes (e.g. sidebar collapse/expand)
  // Only updates SVG dimensions — does NOT re-layout nodes
  if (graphRef.value) {
    resizeObserver = new ResizeObserver(() => {
      if (resizeObserverTimeout) clearTimeout(resizeObserverTimeout);
      resizeObserverTimeout = setTimeout(resizeSvg, 100);
    });
    resizeObserver.observe(graphRef.value);
  }

  // Add scroll event listener to close tooltips on scroll
  window.addEventListener("scroll", closeAllTooltips, true); // Use capture phase

  // Add fullscreen change event listener
  document.addEventListener("fullscreenchange", handleFullscreenChange);
});

// Reset expanded role cards when anime selection changes
watch(selectedAnime, () => {
  expandedRoleCards.value.clear();
});

onBeforeUnmount(() => {
  simulation?.stop();
  if (renderTimeout) clearTimeout(renderTimeout);
  if (graphUrlTimeout) clearTimeout(graphUrlTimeout);
  if (hoverActivationTimeout) clearTimeout(hoverActivationTimeout);
  if (hoverRestoreTimeout) clearTimeout(hoverRestoreTimeout);
  window.removeEventListener("resize", handleResize);
  resizeObserver?.disconnect();
  resizeObserver = null;
  if (resizeObserverTimeout) clearTimeout(resizeObserverTimeout);
  window.removeEventListener("scroll", closeAllTooltips, true);
  document.removeEventListener("fullscreenchange", handleFullscreenChange);
});
</script>

<style scoped>
.graph-container {
  width: 100%;
  margin: 0;
}

.graph-panel-wrapper {
  width: 100%;
  position: relative;
}

.graph-panel-wrapper:fullscreen {
  background: #fafafa;
  padding: 0;
  margin: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  border-radius: 0;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panels) {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  border-radius: 0;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel) {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  margin: 0;
  border: none;
  box-shadow: none;
  height: 100%;
  max-height: 100%;
  border-radius: 0;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel)::before,
.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel)::after {
  display: none;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title) {
  padding: 12px 16px;
  min-height: auto;
  pointer-events: none !important;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title__overlay) {
  display: none !important;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title__icon) {
  display: none !important;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title) .v-btn,
.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title) .v-btn-toggle,
.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title) .v-tooltip,
.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-title) .v-select {
  pointer-events: auto !important;
}

/* Staff button is now enabled in fullscreen */

/* Re-enable pointer events and interactivity for ALL staff view elements */
.graph-panel-wrapper:fullscreen .staff-view-content,
.graph-panel-wrapper:fullscreen .staff-view-content * {
  pointer-events: auto !important;
}

/* Ensure expansion panels can be clicked */
.graph-panel-wrapper:fullscreen .staff-view-content :deep(.v-expansion-panel) {
  pointer-events: auto !important;
}

.graph-panel-wrapper:fullscreen
  .staff-view-content
  :deep(.v-expansion-panel-title) {
  pointer-events: auto !important;
  cursor: pointer !important;
}

.graph-panel-wrapper:fullscreen
  .staff-view-content
  :deep(.v-expansion-panel-title__overlay) {
  display: block !important;
  pointer-events: auto !important;
}

.graph-panel-wrapper:fullscreen
  .staff-view-content
  :deep(.v-expansion-panel-title__icon) {
  display: block !important;
  pointer-events: auto !important;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-text) {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  height: 100%;
  max-height: 100%;
}

.graph-panel-wrapper:fullscreen :deep(.v-expansion-panel-text__wrapper) {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0 16px 16px 16px;
  overflow: hidden;
  height: 100%;
  max-height: 100%;
}

.graph-panel-wrapper:fullscreen .graph-view-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
  height: 100%;
  max-height: 100%;
}

.staff-view-content {
  max-height: 600px;
  overflow-y: auto;
}

.graph-wrapper {
  position: relative;
  width: 100%;
  min-height: 600px;
}

.graph-panel-wrapper:fullscreen .graph-wrapper {
  min-height: unset;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: visible;
  margin-bottom: 0;
  position: relative;
}

.graph-panel-wrapper:fullscreen .graph-svg-container {
  flex: 1;
  height: auto;
  min-height: 0;
  border: none;
  border-radius: 0;
  background: #fafafa;
  display: flex;
  overflow: hidden;
  position: relative;
  z-index: auto;
}

.graph-panel-wrapper:fullscreen .graph-svg-container svg {
  width: 100% !important;
  height: 100% !important;
  display: block;
}

.graph-panel-wrapper:fullscreen :deep(.v-card) {
  margin: 8px 0 8px 0;
  flex-shrink: 0;
}

/* Fix overlays in fullscreen - position relative to graph canvas area */
/* Fix staff list view in fullscreen - proper layout and scrolling */
/* Staff view container - fill available space */
.graph-panel-wrapper:fullscreen .staff-view-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
  height: 100%;
  padding-top: 8px;
}

/* Make staff expansion panels fill space and scroll */
.graph-panel-wrapper:fullscreen .staff-view-content > .v-expansion-panels {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
  margin-top: 0 !important;
  display: block !important; /* Override the flex display from generic CSS */
  height: auto !important; /* Allow natural height */
}

/* Staff group panels in fullscreen - reset generic fullscreen constraints */
.graph-panel-wrapper:fullscreen .staff-view-content :deep(.v-expansion-panel) {
  margin: 0 !important;
  display: block !important;
  flex: none !important;
  height: auto !important;
  max-height: none !important;
  overflow: visible !important;
}

/* Override the 100% height constraint for nested panels - they need to expand to fit content */
.graph-panel-wrapper:fullscreen
  .staff-view-content
  :deep(.v-expansion-panel-text) {
  height: auto !important;
  max-height: none !important;
  overflow: visible !important;
}

.graph-panel-wrapper:fullscreen
  .staff-view-content
  :deep(.v-expansion-panel-text__wrapper) {
  height: auto !important;
  max-height: none !important;
  overflow: visible !important;
  padding: 12px 16px !important;
}

/* Child category panels - reset all generic fullscreen constraints */
.graph-panel-wrapper:fullscreen
  .staff-view-content
.graph-panel-wrapper:fullscreen
  .staff-view-content
.graph-panel-wrapper:fullscreen .staff-view-content :deep(.v-list) {
  overflow: visible !important;
  max-height: none !important;
  height: auto !important;
}

.gap-2 {
  gap: 8px;
}

.graph-svg-container {
  width: 100%;
  height: 600px;
  border: 1px solid var(--color-primary-border);
  border-radius: 4px;
  position: relative;
  background: #fafafa;
}

.graph-svg-container :deep(circle) {
  stroke: var(--color-node-outline);
}

.graph-settings-menu {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 100;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
}

.graph-controls-collapsible {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
}

.graph-controls-fade-enter-active,
.graph-controls-fade-leave-active {
  transition: opacity 0.2s ease;
}

.graph-controls-fade-enter-from,
.graph-controls-fade-leave-to {
  opacity: 0;
}

.graph-edge-label {
  position: absolute;
  top: 12px;
  left: 45%;
  transform: translateX(-50%);
  z-index: 20;
  background: #fafafa;
  padding: 12px 24px;
  border-radius: 6px;
  border: 2px solid var(--color-primary);
  box-shadow: var(--shadow-sm);
  font-size: 14px;
  font-weight: bold;
  color: #1a1a1a;
  max-width: 90%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  pointer-events: none;
}

.edge-label-staff-name {
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 4px;
  text-align: center;
}

.edge-label-two-tier {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  white-space: normal;
}

.edge-label-role {
  font-size: 12px;
  font-weight: 500;
  opacity: 0.85;
}

.edge-label-arrow {
  opacity: 0.5;
}

.anime-hover-preview {
  position: absolute;
  z-index: 30;
  pointer-events: none;
  transform: translate(0, -50%);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.fade-hover-enter-active,
.fade-hover-leave-active {
  transition: opacity 0.4s ease;
}

.fade-hover-enter-from,
.fade-hover-leave-to {
  opacity: 0;
}

.graph-loading {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
}

.graph-empty-state {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  color: #9e9e9e;
}

.recommendation-card {
  height: 100%;
  transition: transform 0.2s;
}

.recommendation-card:hover {
  transform: translateY(-4px);
}

.recommendation-mode-select {
  max-width: 200px;
  margin-left: 8px;
}

.recommendation-mode-select :deep(.v-field) {
  font-size: 0.875rem;
}

.recommendation-mode-select :deep(.v-field__input) {
  min-height: 32px;
  padding-top: 4px;
  padding-bottom: 4px;
}

/* Graph edge interactions — stroke-opacity animated by d3 only (no CSS transition to avoid conflict) */
:deep(.graph-link) {
  transition: stroke-width 0.2s ease;
}

.cursor-pointer {
  cursor: pointer;
}

/* Category info overlay (opaque overlay on hover) */
/* Add gradient fade when content overflows */
/* Wrapper for animated content */
/* Auto-scroll animation - scrolls content after delay */
/* COMMENTED OUT - Not necessary with increased height
@keyframes auto-scroll {
  0%, 10% {
    transform: translateY(0);
  }
  90%, 100% {
    transform: translateY(calc(-100% + 240px));
  }
}

*/

/* Overlay styles */
.comparison-indicator {
  opacity: 0.6;
  display: flex;
  align-items: center;
  font-size: 0.65rem;
  text-transform: none;
  font-weight: 600;
}

/* Transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

</style>
