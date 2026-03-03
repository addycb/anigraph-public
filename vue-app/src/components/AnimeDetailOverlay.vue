<template>
  <div>
    <!-- Backdrop that covers the graph -->
    <transition name="fade">
      <div
        v-if="selectedAnime"
        class="anime-overlay-backdrop"
        @click="$emit('close')"
      ></div>
    </transition>

    <!-- 4-panel overlay - covers the graph area -->
    <transition name="fade-slow">
      <v-card
        v-if="selectedAnime"
        class="anime-info-overlay"
        elevation="16"
      >
        <v-btn
          icon
          variant="elevated"
          class="overlay-close-btn"
          @click="$emit('close')"
          size="small"
        >
          <v-icon>mdi-close</v-icon>
        </v-btn>

        <div class="overlay-body">
          <v-row class="ma-0" style="padding: 12px">
            <!-- Panel 1: Anime Image (hide in category view) -->
            <v-col
              v-if="!selectedAnime.isCategoryView"
              cols="12"
              md="3"
              class="pa-2"
            >
              <div class="overlay-panel-container cover-art-container">
                <div class="overlay-cover-art-wrapper">
                  <v-img
                    :src="
                      selectedAnime.coverImage ||
                      selectedAnime.image ||
                      '/placeholder-anime.jpg'
                    "
                    class="rounded"
                  ></v-img>
                  <!-- Favorite / Bookmark buttons -->
                  <div class="overlay-action-buttons">
                    <v-btn
                      icon
                      class="overlay-favorite-btn"
                      :class="{
                        favorited: isFavorited(
                          selectedAnime.id || selectedAnime.anilistId,
                        ),
                      }"
                      @click.stop="
                        $emit(
                          'toggleFavorite',
                          selectedAnime.id || selectedAnime.anilistId,
                        )
                      "
                      size="large"
                      variant="flat"
                    >
                      <v-icon size="28" color="white">
                        {{
                          isFavorited(
                            selectedAnime.id || selectedAnime.anilistId,
                          )
                            ? "mdi-heart"
                            : "mdi-heart-outline"
                        }}
                      </v-icon>
                    </v-btn>
                    <div class="overlay-btn-divider"></div>
                    <ListButton
                      :anime-id="
                        selectedAnime.id || selectedAnime.anilistId
                      "
                      bubble-mode
                    />
                  </div>
                </div>
                <!-- Title below image -->
                <div class="overlay-cover-title mt-2 text-center">
                  {{
                    selectedAnime.title ||
                    selectedAnime.title_english ||
                    selectedAnime.label ||
                    "Unknown"
                  }}
                </div>
              </div>
            </v-col>

            <!-- Panel 2: Summary & Link OR Staff List for Category View -->
            <v-col
              :cols="12"
              :md="selectedAnime.isCategoryView ? 12 : 3"
              class="pa-2"
            >
              <div class="overlay-panel-container">
                <div class="overlay-panel-header">
                  {{
                    selectedAnime.isCategoryView
                      ? selectedAnime.categoryName
                      : "Anime Details"
                  }}
                </div>
                <div class="overlay-scrollable-list">
                  <!-- Category View: Show staff list -->
                  <template v-if="selectedAnime.isCategoryView">
                    <div
                      v-for="(staffMember, idx) in selectedAnime.staff"
                      :key="idx"
                      class="category-staff-item"
                      @click="
                        staffMember.staff?.staff_id &&
                        navigateTo(
                          `/staff/${encodeURIComponent(staffMember.staff.staff_id)}`,
                        )
                      "
                    >
                      <v-avatar size="40" class="mr-3">
                        <v-img
                          v-if="staffMember.staff?.image"
                          :src="staffMember.staff.image"
                        ></v-img>
                        <v-icon v-else>mdi-account</v-icon>
                      </v-avatar>
                      <div class="flex-grow-1">
                        <div class="text-body-2 font-weight-bold">
                          {{
                            staffMember.staff?.name_en ||
                            staffMember.staff?.name_ja ||
                            "Unknown"
                          }}
                        </div>
                        <div class="text-caption text-grey">
                          {{
                            Array.isArray(staffMember.role)
                              ? staffMember.role.join(", ")
                              : staffMember.role || "Unknown Role"
                          }}
                        </div>
                      </div>
                    </div>
                  </template>
                  <!-- Normal View: Show anime details -->
                  <template v-else>
                    <!-- Year & Format chips at top -->
                    <div class="d-flex flex-wrap ga-1 mb-2">
                      <v-chip
                        v-if="selectedAnime.format"
                        size="x-small"
                        variant="tonal"
                        >{{ formatAnimeFormat(selectedAnime.format) }}</v-chip
                      >
                      <v-chip
                        v-if="
                          selectedAnime.year || selectedAnime.seasonYear
                        "
                        size="x-small"
                        variant="tonal"
                        >{{
                          selectedAnime.year || selectedAnime.seasonYear
                        }}</v-chip
                      >
                      <ScoreChip
                        :score="selectedAnime.averageScore"
                        style-variant="default"
                      />
                    </div>
                    <p
                      class="text-caption mb-3"
                      style="line-height: 1.4"
                      v-if="selectedAnime.description"
                      v-html="selectedAnime.description"
                    ></p>
                    <p class="text-caption text-grey mb-3" v-else>
                      No description available
                    </p>
                    <v-btn
                      :to="`/anime/${encodeURIComponent(selectedAnime.id || selectedAnime.anilistId)}`"
                      color="primary"
                      size="x-small"
                      variant="text"
                      prepend-icon="mdi-open-in-new"
                      @click.stop
                    >
                      View Full Page
                    </v-btn>
                  </template>
                </div>
              </div>
            </v-col>

            <!-- Panel 3: Shared Staff (hide in category view) -->
            <v-col
              v-if="!selectedAnime.isCategoryView"
              cols="12"
              md="3"
              class="pa-2"
            >
              <div class="overlay-panel-container">
                <div
                  class="overlay-panel-header d-flex align-center justify-space-between"
                >
                  <span
                    >Shared Staff
                    <span
                      v-if="sharedStaff.length > 0"
                      class="overlay-staff-count"
                      >({{ sharedStaff.length }})</span
                    ></span
                  >
                  <span class="comparison-indicator">
                    Main → Selected
                  </span>
                </div>
                <div class="overlay-scrollable-list">
                  <!-- Summary View (grouped by category) -->
                  <div class="summary-view">
                    <div
                      v-for="[category, staffList] in sortedCategoryEntries"
                      :key="category"
                      :data-category="category"
                      class="category-card"
                      :class="{
                        'category-card-expanded':
                          expandedRoleCards.has(category),
                      }"
                      @click="toggleRoleCard(category)"
                    >
                      <div class="category-card-header">
                        <!-- Category color indicator -->
                        <div
                          class="category-color-bar"
                          :style="{
                            backgroundColor: categoryColors[category],
                          }"
                        ></div>
                        <span class="category-name">{{
                          getCategoryDisplayName(category)
                        }}</span>
                        <div class="d-flex align-center ga-1">
                          <v-chip
                            size="x-small"
                            :style="{
                              backgroundColor: categoryColors[category],
                              color: 'white',
                            }"
                            variant="flat"
                          >
                            {{ staffList.length }}
                          </v-chip>
                          <v-icon size="small" class="expand-icon">
                            {{
                              expandedRoleCards.has(category)
                                ? "mdi-chevron-up"
                                : "mdi-chevron-down"
                            }}
                          </v-icon>
                        </div>
                      </div>

                      <!-- Collapsed: Show avatars -->
                      <div
                        v-if="!expandedRoleCards.has(category)"
                        class="category-card-staff"
                      >
                        <v-avatar
                          v-for="(staff, idx) in staffList.slice(0, 5)"
                          :key="idx"
                          size="24"
                          class="staff-avatar"
                          :class="{
                            'staff-avatar-overlap': idx > 0,
                          }"
                        >
                          <v-img
                            v-if="staff.image"
                            :src="staff.image"
                          ></v-img>
                          <v-icon v-else size="small">mdi-account</v-icon>
                          <v-tooltip activator="parent" location="top"
                            >{{ staff.name }}</v-tooltip
                          >
                        </v-avatar>
                        <span
                          v-if="staffList.length > 5"
                          class="text-caption ml-2"
                        >
                          +{{ staffList.length - 5 }} more
                        </span>
                      </div>

                      <!-- Expanded: Show full staff list with role indicators -->
                      <div
                        v-else
                        class="category-card-expanded-list"
                        @click.stop
                      >
                        <RouterLink
                          v-for="(staff, idx) in staffList"
                          :key="idx"
                          class="expanded-staff-item"
                          :to="`/staff/${encodeURIComponent(staff.staffId)}`"
                        >
                          <v-avatar size="20" class="mr-2">
                            <v-img
                              v-if="staff.image"
                              :src="staff.image"
                            ></v-img>
                            <v-icon v-else size="x-small">mdi-account</v-icon>
                          </v-avatar>
                          <div class="expanded-staff-content">
                            <div class="staff-name-row">
                              <span class="text-caption font-weight-bold"
                                >{{ staff.name }}</span
                              >
                              <!-- Split color indicator showing both roles' categories -->
                              <div
                                class="staff-role-color"
                                :class="{
                                  'staff-role-color-split':
                                    getRoleColor(staff.mainRole) !==
                                    getRoleColor(staff.selectedRole),
                                }"
                                :style="{
                                  background:
                                    getRoleColor(staff.mainRole) ===
                                    getRoleColor(staff.selectedRole)
                                      ? getRoleColor(staff.mainRole)
                                      : `linear-gradient(90deg, ${getRoleColor(staff.mainRole)} 50%, ${getRoleColor(staff.selectedRole)} 50%)`,
                                }"
                              ></div>
                            </div>
                            <div class="staff-roles-text">
                              <span class="role-text">{{
                                staff.mainRole
                              }}</span>
                              <v-icon
                                size="x-small"
                                class="role-separator"
                                >mdi-chevron-right</v-icon
                              >
                              <span class="role-text">{{
                                staff.selectedRole
                              }}</span>
                            </div>
                          </div>
                        </RouterLink>
                      </div>
                    </div>
                  </div>

                  <p
                    v-if="sharedStaff.length === 0"
                    class="text-caption text-grey pa-2"
                  >
                    No shared staff found
                  </p>
                </div>
              </div>
            </v-col>

            <!-- Panel 4: Metadata (hide in category view) -->
            <v-col
              v-if="!selectedAnime.isCategoryView"
              cols="12"
              md="3"
              class="pa-2"
            >
              <div class="overlay-panel-container">
                <div class="overlay-panel-header">Metadata</div>
                <div class="overlay-scrollable-list">
                  <!-- Shared Studios -->
                  <div v-if="sharedStudios.length > 0" class="mb-3">
                    <div class="text-caption font-weight-medium mb-1">
                      Shared Studios
                    </div>
                    <div class="d-flex flex-wrap ga-1">
                      <v-chip
                        v-for="studio in sharedStudios"
                        :key="studio"
                        size="x-small"
                        color="primary"
                        variant="flat"
                        :to="`/studio/${encodeURIComponent(studio)}`"
                        @click.stop
                      >
                        {{ studio }}
                      </v-chip>
                    </div>
                  </div>

                  <!-- Studios -->
                  <div v-if="otherStudios.length > 0" class="mb-3">
                    <div class="text-caption font-weight-medium mb-1">
                      Studios
                    </div>
                    <div class="d-flex flex-wrap ga-1">
                      <v-chip
                        v-for="studio in otherStudios"
                        :key="studio"
                        size="x-small"
                        color="primary"
                        variant="tonal"
                        :to="`/studio/${encodeURIComponent(studio)}`"
                        @click.stop
                      >
                        {{ studio }}
                      </v-chip>
                    </div>
                  </div>

                  <!-- Shared Genres -->
                  <div v-if="sharedGenres.length > 0" class="mb-3">
                    <div class="text-caption font-weight-medium mb-1">
                      Shared Genres
                    </div>
                    <div class="d-flex flex-wrap ga-1">
                      <v-chip
                        v-for="genre in sharedGenres"
                        :key="genre"
                        size="x-small"
                        color="primary"
                        variant="flat"
                      >
                        {{ genre }}
                      </v-chip>
                    </div>
                  </div>

                  <!-- Other Genres -->
                  <div v-if="otherGenres.length > 0" class="mb-3">
                    <div class="text-caption font-weight-medium mb-1">
                      Other Genres
                    </div>
                    <div class="d-flex flex-wrap ga-1">
                      <v-chip
                        v-for="genre in otherGenres"
                        :key="genre"
                        size="x-small"
                        color="primary"
                        variant="tonal"
                      >
                        {{ genre }}
                      </v-chip>
                    </div>
                  </div>

                  <!-- Shared Tags -->
                  <div v-if="sharedTags.length > 0" class="mb-3">
                    <div class="text-caption font-weight-medium mb-1">
                      Shared Tags
                    </div>
                    <div class="d-flex flex-wrap ga-1">
                      <v-chip
                        v-for="tag in sharedTags"
                        :key="tag.name || tag"
                        size="x-small"
                        color="secondary"
                        variant="flat"
                      >
                        {{ tag.name || tag }}
                      </v-chip>
                    </div>
                  </div>

                  <!-- Other Tags -->
                  <div v-if="otherTags.length > 0">
                    <div class="text-caption font-weight-medium mb-1">
                      Other Tags
                    </div>
                    <div class="d-flex flex-wrap ga-1">
                      <v-chip
                        v-for="tag in otherTags"
                        :key="tag.name || tag"
                        size="x-small"
                        color="secondary"
                        variant="tonal"
                      >
                        {{ tag.name || tag }}
                      </v-chip>
                    </div>
                  </div>

                  <p
                    v-if="!hasAnyMetadata"
                    class="text-caption text-grey pa-2"
                  >
                    No metadata available
                  </p>
                </div>
              </div>
            </v-col>
          </v-row>
        </div>
      </v-card>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import { formatAnimeFormat } from "@/utils/formatters";
import {
  STAFF_CATEGORIES,
  categorizeRole,
} from "@/utils/staffCategories";

const props = defineProps<{
  selectedAnime: any;
  mainAnimeData: any;
  categoryColors: Record<string, string>;
  groupColors: Record<string, string>;
  pinnedCategory: string | null;
  isFavorited: (id: number | string) => boolean;
}>();

defineEmits<{
  close: [];
  toggleFavorite: [animeId: number | string];
}>();

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

const expandedRoleCards = ref<Set<string>>(new Set());

// Reset expanded role cards when anime selection changes,
// then auto-expand the pinned category if one was set (edge click)
watch(() => props.selectedAnime, async () => {
  expandedRoleCards.value.clear();
  if (props.pinnedCategory) {
    await nextTick();
    expandedRoleCards.value = new Set([props.pinnedCategory]);
  }
});

const toggleRoleCard = (role: string) => {
  if (expandedRoleCards.value.has(role)) {
    expandedRoleCards.value.delete(role);
  } else {
    expandedRoleCards.value.add(role);
  }
  expandedRoleCards.value = new Set(expandedRoleCards.value);
};

const getRoleColor = (role: string): string => {
  const category = categorizeRole(role);
  return props.categoryColors[category] || props.categoryColors.other;
};

const sharedStaff = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];

  const mainStaff = props.mainAnimeData.staff || [];
  const selectedStaffList = props.selectedAnime.staff || [];

  const selectedStaffById = new Map<string, any>();
  selectedStaffList.forEach((member: any) => {
    if (member.staff?.staff_id)
      selectedStaffById.set(member.staff.staff_id, member);
  });

  const shared: any[] = [];

  mainStaff.forEach((mainMember: any) => {
    const matchingStaff = mainMember.staff?.staff_id
      ? selectedStaffById.get(mainMember.staff.staff_id)
      : undefined;

    if (matchingStaff) {
      const mainRole = Array.isArray(mainMember.role)
        ? mainMember.role.join(", ")
        : mainMember.role || "Unknown";
      const selectedRole = Array.isArray(matchingStaff.role)
        ? matchingStaff.role.join(", ")
        : matchingStaff.role || "Unknown";

      shared.push({
        staffId: mainMember.staff.staff_id,
        name: mainMember.staff.name_en || mainMember.staff.name_ja || "Unknown",
        image: mainMember.staff.image,
        mainRole: mainRole,
        selectedRole: selectedRole,
        mainAnimeTitle:
          props.mainAnimeData.title ||
          props.mainAnimeData.title_english ||
          "Main Anime",
        selectedAnimeTitle:
          props.selectedAnime.title ||
          props.selectedAnime.title_english ||
          "Selected Anime",
      });
    }
  });

  return shared;
});

const sharedStaffByCategory = computed(() => {
  if (!sharedStaff.value.length) return {};

  const grouped: Record<string, any[]> = {};
  sharedStaff.value.forEach((staff: any) => {
    const category = categorizeRole(staff.mainRole);
    if (!grouped[category]) {
      grouped[category] = [];
    }
    grouped[category].push(staff);
  });

  return grouped;
});

const sortedCategoryEntries = computed(() => {
  const entries = Object.entries(sharedStaffByCategory.value);
  const pinned = props.pinnedCategory;

  return entries.sort((a, b) => {
    if (pinned) {
      if (a[0] === pinned) return -1;
      if (b[0] === pinned) return 1;
    }
    return CATEGORY_ORDER.indexOf(a[0]) - CATEGORY_ORDER.indexOf(b[0]);
  });
});

const getCategoryDisplayName = (categoryKey: string): string => {
  const category = STAFF_CATEGORIES.find((c) => c.key === categoryKey);
  return category ? category.title_en : "Other";
};

const sharedGenres = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];
  const mainGenres = props.mainAnimeData.genres || [];
  const selectedGenreSet = new Set(props.selectedAnime.genres || []);
  return mainGenres.filter((genre: string) => selectedGenreSet.has(genre));
});

const sharedTags = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];
  const mainTags = props.mainAnimeData.tags || [];
  const selectedTags = props.selectedAnime.tags || [];
  const selectedTagNameSet = new Set(selectedTags.map((t: any) => t.name || t));
  return mainTags.filter((tag: any) => selectedTagNameSet.has(tag.name || tag));
});

const sharedStudios = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];
  const mainStudioNames = (props.mainAnimeData.studios || []).map(
    (s: any) => s.name || s,
  );
  const selectedStudioNameSet = new Set(
    (props.selectedAnime.studios || []).map((s: any) => s.name || s),
  );
  return mainStudioNames.filter((studioName: string) =>
    selectedStudioNameSet.has(studioName),
  );
});

const otherStudios = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];
  const mainStudioNameSet = new Set(
    (props.mainAnimeData.studios || []).map((s: any) => s.name || s),
  );
  const selectedStudioNames = (props.selectedAnime.studios || []).map(
    (s: any) => s.name || s,
  );
  return selectedStudioNames.filter(
    (studioName: string) => !mainStudioNameSet.has(studioName),
  );
});

const otherGenres = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];
  const mainGenreSet = new Set(props.mainAnimeData.genres || []);
  const selectedGenres = props.selectedAnime.genres || [];
  return selectedGenres.filter((genre: string) => !mainGenreSet.has(genre));
});

const otherTags = computed(() => {
  if (!props.selectedAnime || !props.mainAnimeData) return [];
  const mainTags = props.mainAnimeData.tags || [];
  const selectedTags = props.selectedAnime.tags || [];
  const mainTagNameSet = new Set(mainTags.map((t: any) => t.name || t));
  return selectedTags.filter(
    (tag: any) => !mainTagNameSet.has(tag.name || tag),
  );
});

const hasAnyMetadata = computed(() => {
  return (
    sharedStudios.value.length > 0 ||
    sharedGenres.value.length > 0 ||
    sharedTags.value.length > 0 ||
    otherStudios.value.length > 0 ||
    otherGenres.value.length > 0 ||
    otherTags.value.length > 0
  );
});
</script>

<style scoped>
/* Overlay styles */
.anime-overlay-backdrop {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(var(--color-bg-rgb), 0.75);
  z-index: 100;
  backdrop-filter: blur(3px);
}

.anime-info-overlay {
  position: absolute;
  inset: 0;
  margin: auto;
  width: 95%;
  max-width: 1400px;
  max-height: 90%;
  height: fit-content;
  background: var(--color-surface-alt);
  z-index: 101;
  border-radius: 12px;
  box-shadow: var(--shadow-glow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.overlay-body {
  overflow-y: auto;
  flex: 1;
}

.overlay-close-btn {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 102;
  box-shadow:
    0 2px 6px rgba(0, 0, 0, 0.35),
    0 0 0 1px rgba(255, 255, 255, 0.18) !important;
}

.overlay-panel-container {
  height: 100%;
  min-height: 300px;
  display: flex;
  flex-direction: column;
  padding: 12px;
  border-radius: 8px;
  background: var(--color-surface);
  border: 1px solid rgba(var(--color-text-rgb), 0.15);
}

.cover-art-container {
  height: auto;
  min-height: auto;
}

.overlay-cover-art-wrapper {
  position: relative;
}

.overlay-action-buttons {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  background: rgba(var(--color-bg-rgb), 0.7);
  border-radius: 24px;
  border: 1px solid rgba(var(--color-primary-rgb), 0.2);
  padding: 3px;
  box-shadow: var(--shadow-md);
  transition: opacity 0.3s ease;
  opacity: 0;
  pointer-events: none;
}

.overlay-cover-art-wrapper:hover .overlay-action-buttons {
  opacity: 1;
  pointer-events: auto;
}

.overlay-favorite-btn {
  background-color: transparent !important;
  box-shadow: none !important;
}

.overlay-favorite-btn:hover {
  background-color: var(--color-primary-faint) !important;
}

.overlay-favorite-btn :deep(.v-icon) {
  color: var(--color-text) !important;
}

.overlay-favorite-btn.favorited :deep(.v-icon) {
  color: var(--color-error) !important;
}

.overlay-btn-divider {
  width: 1px;
  height: 28px;
  background: rgba(var(--color-primary-rgb), 0.3);
  margin: 0 3px;
}

.overlay-cover-title {
  font-weight: 600;
  font-size: 0.875rem;
  line-height: 1.3;
  color: rgb(var(--v-theme-on-surface));
}

.overlay-staff-count {
  font-weight: 400;
  opacity: 0.7;
  text-transform: none;
}

.overlay-panel-header {
  font-weight: 700;
  font-size: 0.75rem;
  margin-bottom: 12px;
  color: rgb(var(--v-theme-on-surface));
  opacity: 0.7;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 2px solid rgb(var(--v-theme-primary));
  padding-bottom: 6px;
}

.comparison-indicator {
  opacity: 0.6;
  display: flex;
  align-items: center;
  font-size: 0.65rem;
  text-transform: none;
  font-weight: 600;
}

.overlay-scrollable-list {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

/* Shared Staff Styles - Summary View */
.summary-view {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.category-card {
  background: rgba(var(--v-theme-surface-variant), 0.3);
  border: 1px solid rgba(var(--color-text-rgb), 0.15);
  border-radius: 8px;
  padding: 8px;
  transition: all 0.2s ease;
  cursor: pointer;
}

.category-card:hover {
  background: rgba(var(--v-theme-surface-variant), 0.5);
  transform: translateY(-1px);
}

.category-card-expanded {
  background: rgba(var(--v-theme-primary), 0.1);
  border-color: rgb(var(--v-theme-primary));
}

.category-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.category-color-bar {
  width: 3px;
  height: 20px;
  border-radius: 2px;
  flex-shrink: 0;
}

.category-name {
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(var(--v-theme-on-surface));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.category-card-staff {
  display: flex;
  align-items: center;
}

.staff-avatar {
  border: 2px solid rgb(var(--v-theme-surface));
  transition: transform 0.2s ease;
}

.staff-avatar:hover {
  transform: scale(1.2);
  z-index: 10;
}

.staff-avatar-overlap {
  margin-left: -8px;
}

.expand-icon {
  transition: transform 0.2s ease;
}

.category-card-expanded-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 4px;
  max-height: 200px;
  overflow-y: auto;
}

.expanded-staff-item {
  display: flex;
  align-items: center;
  padding: 4px;
  border-radius: 4px;
  transition: background 0.2s ease;
  cursor: pointer;
  text-decoration: none;
  color: inherit;
}

.expanded-staff-item:hover {
  background: rgba(var(--v-theme-primary), 0.1);
}

.expanded-staff-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.staff-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.staff-role-color {
  width: 24px;
  height: 3px;
  border-radius: 2px;
  flex-shrink: 0;
}

.staff-role-color-split {
  width: 32px;
}

.staff-roles-text {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  font-size: 0.7rem;
  line-height: 1.2;
}

.role-text {
  flex: 1;
  min-width: 0;
  color: rgba(var(--v-theme-on-surface), 0.7);
  word-break: break-word;
  overflow-wrap: break-word;
}

.role-separator {
  flex-shrink: 0;
  opacity: 0.5;
  margin-top: 1px;
}

/* Category staff list in overlay */
.category-staff-item {
  display: flex;
  align-items: center;
  padding: 8px;
  border-radius: 6px;
  margin-bottom: 4px;
  transition: background 0.2s ease;
  cursor: pointer;
  border: 1px solid rgba(var(--color-text-rgb), 0.12);
}

.category-staff-item:hover {
  background: rgba(var(--v-theme-primary), 0.1);
  border-color: rgb(var(--v-theme-primary));
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

.fade-slow-enter-active {
  transition: opacity 0.25s ease;
}

.fade-slow-leave-active {
  transition: none;
}

.fade-slow-enter-from,
.fade-slow-leave-to {
  opacity: 0;
}

/* Mobile responsiveness */
@media (max-width: 960px) {
  .anime-info-overlay {
    height: min(80vh, 90%);
    max-height: none;
  }

  .overlay-panel-container {
    min-height: 200px;
    padding: 8px;
  }

  .staff-roles-text {
    flex-direction: column;
    gap: 2px;
  }

  .role-separator {
    transform: rotate(90deg);
    margin: -2px 0;
  }

  .role-text {
    font-size: 0.65rem;
  }
}
</style>
