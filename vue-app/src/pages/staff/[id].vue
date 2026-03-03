<template>
  <v-app>
    <AppBar clickable-title />

    <v-main>
      <!-- Loading State -->
      <v-container v-if="loading" class="text-center py-12">
        <v-progress-circular
          indeterminate
          color="primary"
          size="64"
        ></v-progress-circular>
        <p class="text-h6 mt-4">Loading staff details...</p>
      </v-container>

      <!-- Error State -->
      <v-container v-else-if="error" class="text-center py-12">
        <v-icon size="64" color="error">mdi-alert-circle</v-icon>
        <p class="text-h6 mt-4">{{ error }}</p>
        <v-btn color="primary" class="mt-4" to="/">
          Return Home
        </v-btn>
      </v-container>

      <!-- Staff Details -->
      <v-container v-else-if="staff" fluid>
        <!-- Main Content -->
        <v-row class="mt-4">
          <!-- Left Column: Staff Info -->
          <v-col cols="12" md="4" lg="3">
            <v-card class="sticky-card">
              <v-card-text class="text-center">
                <!-- Staff Image -->
                <div class="staff-image-container mb-4">
                  <v-img
                    v-if="staff.image_large || staff.image_medium"
                    :src="staff.image_large || staff.image_medium"
                    :alt="staff.name_en || staff.name_ja"
                    rounded="lg"
                  />
                  <v-avatar v-else size="120" color="grey">
                    <v-icon size="80">mdi-account</v-icon>
                  </v-avatar>
                </div>

                <!-- Names -->
                <h2 class="text-h5 mb-2">
                  {{ staff.name_en || staff.name_ja || 'Unknown Staff' }}
                </h2>
                <p v-if="staff.name_en && staff.name_ja" class="text-subtitle-1 text-medium-emphasis mb-3">
                  {{ staff.name_ja }}
                </p>
                <p v-if="staff.pen_name_en || staff.pen_name_ja" class="text-subtitle-2 text-medium-emphasis mb-3">
                  Pen Name: {{ staff.pen_name_en || staff.pen_name_ja }}
                </p>

                <v-divider class="my-3"></v-divider>

                <!-- Additional Info Section -->
                <div class="info-section mb-3">
                  <div v-if="staff.gender" class="mb-2">
                    <h4 class="text-caption text-medium-emphasis">Gender</h4>
                    <p class="text-body-2">{{ staff.gender }}</p>
                  </div>

                  <div v-if="staff.dateOfBirth_year" class="mb-2">
                    <h4 class="text-caption text-medium-emphasis">Date of Birth</h4>
                    <p class="text-body-2">
                      {{ formatDate(staff.dateOfBirth_year, staff.dateOfBirth_month, staff.dateOfBirth_day) }}
                      <span v-if="staff.age"> ({{ staff.age }} years old)</span>
                    </p>
                  </div>

                  <div v-if="staff.dateOfDeath_year" class="mb-2">
                    <h4 class="text-caption text-medium-emphasis">Date of Death</h4>
                    <p class="text-body-2">
                      {{ formatDate(staff.dateOfDeath_year, staff.dateOfDeath_month, staff.dateOfDeath_day) }}
                    </p>
                  </div>

                  <div v-if="staff.homeTown" class="mb-2">
                    <h4 class="text-caption text-medium-emphasis">Hometown</h4>
                    <p class="text-body-2">{{ staff.homeTown }}</p>
                  </div>

                  <div v-if="staff.bloodType" class="mb-2">
                    <h4 class="text-caption text-medium-emphasis">Blood Type</h4>
                    <p class="text-body-2">{{ staff.bloodType }}</p>
                  </div>


                  <div v-if="staff.primaryOccupations && staff.primaryOccupations.filter((o: any) => o).length > 0" class="mb-2">
                    <h4 class="text-caption text-medium-emphasis">Primary Occupations</h4>
                    <div>
                      <v-chip
                        v-for="occupation in staff.primaryOccupations.filter((o: any) => o)"
                        :key="occupation"
                        size="small"
                        variant="outlined"
                        class="mr-1 mb-1"
                      >
                        {{ occupation }}
                      </v-chip>
                    </div>
                  </div>
                </div>

                <v-divider v-if="staff.gender || staff.dateOfBirth_year || staff.dateOfDeath_year || staff.homeTown || staff.bloodType || (staff.primaryOccupations && staff.primaryOccupations.filter((o: any) => o).length > 0)" class="my-3"></v-divider>

                <!-- Categories -->
                <div v-if="sortedCategories.length > 0">
                  <h4 class="text-subtitle-2 text-medium-emphasis mb-2">Roles</h4>
                  <v-chip
                    v-for="category in sortedCategories"
                    :key="category"
                    :color="categoryColors[category]"
                    size="small"
                    class="mr-1 mb-1 cursor-pointer"
                    :variant="selectedCategories.includes(category) ? 'flat' : 'tonal'"
                    @click="toggleCategoryFilter(category)"
                    :disabled="loadingFilterCounts || (hasActiveFilters && !selectedCategories.includes(category) && filterCounts.categories[category] === 0)"
                  >
                    <v-icon v-if="selectedCategories.includes(category)" start size="small">mdi-check</v-icon>
                    {{ getCategoryLabel(category) }}
                    <span v-if="hasActiveFilters && filterCounts.categories[category] !== undefined" class="ml-1">
                      ({{ filterCounts.categories[category] }})
                    </span>
                    <span v-else class="ml-1">
                      ({{ categoryCounts[category] || 0 }})
                    </span>
                  </v-chip>
                </div>

                <v-divider class="my-3"></v-divider>

                <!-- Stats -->
                <div class="text-center">
                  <div class="text-h4">
                    <span v-if="hasActiveFilters">{{ filteredFilmographyCount }} / </span>{{ totalFilmographyCount }}
                  </div>
                  <div class="text-caption text-medium-emphasis">
                    <span v-if="hasActiveFilters">Filtered / Total </span>Anime Credits
                  </div>
                </div>
              </v-card-text>

              <!-- Styles Section -->
              <v-expansion-panels v-if="staff.genreStats && staff.genreStats.length > 0" v-model="stylesOpen">
                <v-expansion-panel>
                  <v-expansion-panel-title>
                    <span class="text-h6">Filter by</span>
                  </v-expansion-panel-title>
                  <v-expansion-panel-text>
                    <!-- Active Filters -->
                    <ActiveFilters
                      :has-filters="hasActiveFilters"
                      :summary="`Showing ${filteredFilmographyCount} of ${totalFilmographyCount} credits`"
                      @clear-all="clearAllFilters"
                    >
                      <v-chip
                        v-for="category in selectedCategories"
                        :key="`selected-category-${category}`"
                        size="small"
                        closable
                        @click:close="toggleCategoryFilter(category)"
                        :color="categoryColors[category]"
                        class="mr-1 mb-1"
                      >
                        {{ getCategoryLabel(category) }}
                      </v-chip>
                      <v-chip
                        v-for="genre in selectedGenres"
                        :key="`selected-${genre}`"
                        size="small"
                        closable
                        @click:close="toggleGenreFilter(genre)"
                        color="primary"
                        class="mr-1 mb-1"
                      >
                        {{ genre }}
                      </v-chip>
                      <v-chip
                        v-for="tag in selectedTags"
                        :key="`selected-${tag}`"
                        size="small"
                        closable
                        @click:close="toggleTagFilter(tag)"
                        color="secondary"
                        class="mr-1 mb-1"
                      >
                        {{ tag }}
                      </v-chip>
                      <v-chip
                        v-if="selectedFormat !== 'all'"
                        size="small"
                        closable
                        @click:close="selectedFormat = 'all'; calculateFilterCounts()"
                        color="accent"
                        class="mr-1 mb-1"
                      >
                        {{ selectedFormat === 'anime' ? 'Anime' : 'Manga' }}
                      </v-chip>
                    </ActiveFilters>

                    <v-divider v-if="hasActiveFilters" class="my-3"></v-divider>

                    <!-- Genres -->
                    <FilterSection
                      v-if="staff.genreStats && staff.genreStats.length > 0"
                      title="Top Genres"
                      :items="staff.genreStats"
                      :selected-items="selectedGenres"
                      :filter-counts="filterCounts.genres"
                      :loading-counts="loadingFilterCounts"
                      :has-active-filters="hasActiveFilters"
                      :limit="10"
                      chip-size="small"
                      chip-variant="outlined"
                      selected-class="selected-genre-filter"
                      section-class="mb-4"
                      @toggle="toggleGenreFilter"
                    />

                    <!-- Tags -->
                    <FilterSection
                      v-if="staff.tagStats && staff.tagStats.length > 0"
                      title="Common Themes"
                      :items="staff.tagStats"
                      :selected-items="selectedTags"
                      :filter-counts="filterCounts.tags"
                      :loading-counts="loadingFilterCounts"
                      :has-active-filters="hasActiveFilters"
                      :limit="10"
                      chip-size="small"
                      chip-variant="tonal"
                      selected-class="selected-tag-filter"
                      @toggle="toggleTagFilter"
                    />
                  </v-expansion-panel-text>
                </v-expansion-panel>
              </v-expansion-panels>
            </v-card>
          </v-col>

          <!-- Right Column: Tabs (Filmography, Timeline) -->
          <v-col cols="12" md="8" lg="9">
            <v-tabs v-model="activeTab">
              <v-tab value="overview">Works</v-tab>
              <v-tab v-if="staff.description" value="bio">Bio</v-tab>
              <v-tab v-if="sakugabooruPosts.length > 0" value="samples">Samples</v-tab>
            </v-tabs>

            <v-window v-model="activeTab" :touch="false">
              <!-- Overview Tab: Filmography -->
              <v-window-item value="overview">
                <!-- View and Sort Controls -->
                <ViewToolbar
                  v-model:card-size="cardSize"
                  v-model:sort-by="sortBy"
                  v-model:year-markers-enabled="showYearMarkers"
                  :sort-order="sortOrder"
                  :sort-options="sortOptions"
                  :show-sort="true"
                  :show-year-markers="true"
                  @toggle-sort-order="toggleSortOrder"
                >
                  <template #left>
                    <!-- Format Filter (only show if has both anime and manga) -->
                    <v-select
                      v-if="hasBothAnimeAndManga"
                      v-model="selectedFormat"
                      :items="[
                        { value: 'all', title: 'All' },
                        { value: 'anime', title: 'Anime' },
                        { value: 'manga', title: 'Manga' }
                      ]"
                      label="Format"
                      variant="outlined"
                      density="compact"
                      hide-details
                      style="max-width: 130px;"
                      @update:model-value="calculateFilterCounts"
                    ></v-select>
                  </template>
                </ViewToolbar>

                <v-row>
                  <template v-for="(item, idx) in activeFilmography" :key="item.isYearMarker ? `year-${item.year}` : item.isSpacer ? `spacer-${idx}` : item.anime?.anilistId || idx">
                    <!-- Spacer -->
                    <v-col v-if="item.isSpacer && showYearMarkers" cols="12" sm="6" md="4" :lg="cardColSize" class="spacer-col">
                      <YearCard spacer />
                    </v-col>
                    <!-- Year Marker -->
                    <v-col v-else-if="item.isYearMarker && showYearMarkers" cols="12" sm="6" md="4" :lg="cardColSize" class="year-marker-col">
                      <YearCard :year="item.year" :count="item.count" count-label="credit" :continued="item.continued" />
                    </v-col>
                    <!-- Credit Card -->
                    <v-col
                      v-else-if="!item.isSpacer && !item.isYearMarker"
                      cols="12"
                      sm="6"
                      md="4"
                      :lg="cardColSize"
                    >
                      <AnimeCard :anime="item.anime" :role="item.role" :show-season="true" :show-year="!showYearMarkers || sortBy === 'score'" compact-layout />
                    </v-col>
                  </template>
                </v-row>

                <!-- Pagination -->
                <v-row v-if="totalPages > 1" class="mt-4">
                  <v-col cols="12" class="d-flex justify-center">
                    <v-pagination
                      v-model="currentPage"
                      :length="totalPages"
                      :total-visible="7"
                    ></v-pagination>
                  </v-col>
                </v-row>
              </v-window-item>

              <!-- Bio Tab -->
              <v-window-item value="bio">
                <v-card v-if="staff.description" class="mb-4">
                  <v-card-title class="d-flex align-center justify-space-between">
                    Biography
                    <a
                      :href="`https://anilist.co/staff/${staff.staff_id}`"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-caption text-medium-emphasis text-decoration-none d-flex align-center"
                    >
                      <v-icon size="14" class="mr-1">mdi-open-in-new</v-icon>
                      AniList
                    </a>
                  </v-card-title>
                  <v-card-text>
                    <div class="staff-bio" v-html="sanitizeBio(staff.description)"></div>
                  </v-card-text>
                </v-card>
              </v-window-item>

              <!-- Samples Tab: Sakugabooru Clips -->
              <v-window-item value="samples">
                <SakugaClipsGrid
                  :posts="sakugabooruPosts"
                  :sakugabooru-tag="staff.sakugabooruTag"
                />
              </v-window-item>

            </v-window>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/utils/api'
import { STAFF_CATEGORIES, CATEGORY_TO_GROUP } from '@/utils/staffCategories'
import { filterAdultContent } from '@/utils/contentFilters'
import { sortByYearAndScore } from '@/utils/sorting'
import { flattenWithYearMarkers, paginateWithYearMarkers } from '@/utils/yearMarkers'
import { useSanitizeHtml } from '@/composables/useSanitizeHtml'
import { useSettings } from '@/composables/useSettings'
import { useSortable } from '@/composables/useSortable'
import { useCardSize } from '@/composables/useCardSize'

const { sanitizeBio } = useSanitizeHtml()
const route = useRoute()
const router = useRouter()
const { includeAdult } = useSettings()
const staffId = computed(() => route.params.id as string)

// Manual fetch replacing useAsyncData
const staffResponse = ref<any>(null)
const pending = ref(false)
const fetchError = ref<string | null>(null)

async function fetchStaff() {
  if (!staffId.value) return
  pending.value = true
  fetchError.value = null
  try {
    staffResponse.value = await api<any>(`/staff/${encodeURIComponent(staffId.value)}`)
  } catch (e: any) {
    fetchError.value = e.message
  } finally {
    pending.value = false
  }
}

onMounted(fetchStaff)
watch(staffId, fetchStaff)

const staff = computed(() => staffResponse.value?.success ? staffResponse.value.data : null)
const loading = computed(() => pending.value)
const error = computed(() => {
  if (fetchError.value) return fetchError.value || 'Failed to load staff details'
  if (staffResponse.value && !staffResponse.value.success) return 'Failed to load staff details'
  return ''
})

// Sakugabooru posts
const sakugabooruPosts = computed(() => staff.value?.sakugabooruPosts || [])

// Sort state
const { sortBy, sortOrder, sortOptions, toggleSortOrder } = useSortable('year', 'desc')

// Card size state
const { cardSize, cardColSize, cardsPerRow, showYearMarkers } = useCardSize()

// Filter state
const selectedGenres = ref<string[]>([])
const selectedTags = ref<string[]>([])
const selectedCategories = ref<string[]>([])
const selectedFormat = ref<'all' | 'anime' | 'manga'>('all')
const filterCounts = ref<any>({ genres: {}, tags: {}, categories: {} })
const loadingFilterCounts = ref(false)

// Parse URL hash to extract tab, unselected indexes, max collaborators, exclude producers
// Format: #overview?u=0,2&n=7&e=1
const parseHash = (hash: string) => {
  if (!hash) return { tab: 'overview', unselectedIndexes: [], maxCollaborators: 5, excludeProducers: true }

  const hashWithoutPrefix = hash.substring(1) // Remove '#'
  const [tab, queryString] = hashWithoutPrefix.split('?')

  let unselectedIndexes: number[] = []
  let maxCollaborators = 5 // default
  let excludeProducers = true // default

  if (queryString) {
    const params = new URLSearchParams(queryString)

    const unselectedParam = params.get('u')
    if (unselectedParam) {
      unselectedIndexes = unselectedParam
        .split(',')
        .map(s => parseInt(s, 10))
        .filter(n => !isNaN(n))
    }

    const maxParam = params.get('n')
    if (maxParam) {
      const parsed = parseInt(maxParam, 10)
      if (!isNaN(parsed)) {
        maxCollaborators = parsed
      }
    }

    const excludeParam = params.get('e')
    if (excludeParam !== null) {
      excludeProducers = excludeParam === '1'
    }
  }

  return { tab: tab || 'overview', unselectedIndexes, maxCollaborators, excludeProducers }
}

// Active tab, unselected indexes, max collaborators, exclude producers (initialize from URL hash)
const { tab: initialTab, unselectedIndexes: initialUnselected, maxCollaborators: initialMax, excludeProducers: initialExcludeProducers } = parseHash(route.hash)
const activeTab = ref(initialTab)
const unselectedIndexes = ref<number[]>(initialUnselected)
const maxCollaboratorsToShow = ref<number>(initialMax)
const excludeProducers = ref<boolean>(initialExcludeProducers)

// Styles panel open by default
const stylesOpen = ref([0])

// Parent group colors (same as in GraphVisualization)
const groupColors: Record<string, string> = {
  direction: '#1976d2',        // Blue - Direction
  writing_story: '#9c27b0',    // Purple - Writing
  design: '#e91e63',           // Pink - Design
  music_op_ed: '#ffc107',      // Amber - Music & OP/ED
  animation: '#4caf50',        // Green - Animation
  art_color: '#795548',        // Brown - Art & Color
  post_production: '#009688',  // Teal - Post-Production
  sound: '#00bcd4',            // Cyan - Sound
  production_group: '#607d8b', // Blue Grey - Production
  other: '#9e9e9e'             // Grey - Other
}

// Build categoryColors by mapping each detailed category to its parent group color
const categoryColors: Record<string, string> = (() => {
  const colors: Record<string, string> = { other: groupColors.other }
  STAFF_CATEGORIES.forEach(cat => {
    const parentGroup = CATEGORY_TO_GROUP[cat.key] || 'other'
    colors[cat.key] = groupColors[parentGroup] || groupColors.other
  })
  return colors
})()

const getCategoryLabel = (categoryKey: string) => {
  const category = STAFF_CATEGORIES.find(cat => cat.key === categoryKey)
  return category?.title_en || 'Other'
}

// Check if staff has both anime and manga works
const hasBothAnimeAndManga = computed(() => {
  if (!baseFilteredFilmography.value.length) return false
  const formats = new Set(baseFilteredFilmography.value.map((credit: any) => credit.anime?.format).filter(Boolean))
  const animeFormats = ['TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC']
  const mangaFormats = ['MANGA', 'NOVEL', 'ONE_SHOT']

  const hasAnime = Array.from(formats).some(f => animeFormats.includes(f as string))
  const hasManga = Array.from(formats).some(f => mangaFormats.includes(f as string))

  return hasAnime && hasManga
})

// Check if any filters are active
const hasActiveFilters = computed(() => {
  return selectedGenres.value.length > 0 || selectedTags.value.length > 0 || selectedFormat.value !== 'all' || selectedCategories.value.length > 0
})

// Filtered filmography based on adult content setting
const baseFilteredFilmography = computed(() => {
  if (!staff.value?.filmography) return []
  return filterAdultContent(staff.value.filmography, includeAdult.value)
})

// Calculate category counts from filmography
const categoryCounts = computed(() => {
  const counts: Record<string, number> = {}
  baseFilteredFilmography.value.forEach((credit: any) => {
    const category = credit.category || 'other'
    counts[category] = (counts[category] || 0) + 1
  })
  return counts
})

// Sort categories by count
const sortedCategories = computed(() => {
  if (!staff.value?.categories) return []
  return [...staff.value.categories].sort((a: string, b: string) => {
    const countA = categoryCounts.value[a] || 0
    const countB = categoryCounts.value[b] || 0

    if (countA !== countB) {
      return countB - countA
    }

    const groupA = CATEGORY_TO_GROUP[a] || 'other'
    const groupB = CATEGORY_TO_GROUP[b] || 'other'
    if (groupA !== groupB) {
      return groupA.localeCompare(groupB)
    }

    return a.localeCompare(b)
  })
})

// Calculate filter counts for genres and tags
const calculateFilterCounts = () => {
  if (!staff.value) {
    return
  }

  loadingFilterCounts.value = true

  try {
    const checkGenres = staff.value.genreStats?.map((g: any) => g.name) || []
    const checkTags = staff.value.tagStats?.map((t: any) => t.name) || []
    const checkCategories = staff.value.categories || []

    if (checkGenres.length === 0 && checkTags.length === 0 && checkCategories.length === 0) {
      loadingFilterCounts.value = false
      return
    }

    const counts: any = {
      genres: {},
      tags: {},
      categories: {}
    }

    checkGenres.forEach((g: string) => counts.genres[g] = 0)
    checkTags.forEach((t: string) => counts.tags[t] = 0)
    checkCategories.forEach((c: string) => counts.categories[c] = 0)

    const filteredCount = getFilteredFilmography(baseFilteredFilmography.value).length
    const curSelectedGenres = selectedGenres.value
    const curSelectedTags = selectedTags.value
    const curSelectedCategories = selectedCategories.value
    const curFormat = selectedFormat.value

    const animeFormats = ['TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC']
    const mangaFormats = ['MANGA', 'NOVEL', 'ONE_SHOT']

    const prepared = baseFilteredFilmography.value.map((credit: any) => {
      const genreSet = new Set<string>(credit.anime?.genres || [])
      const tagSet = new Set<string>(credit.anime?.tags?.map((t: any) => t.name) || [])
      const category = credit.category || 'other'
      const format = credit.anime?.format
      let formatMatch = true
      if (curFormat !== 'all') {
        if (curFormat === 'anime') {
          formatMatch = animeFormats.includes(format)
        } else if (curFormat === 'manga') {
          formatMatch = mangaFormats.includes(format)
        }
      }
      return { genreSet, tagSet, category, formatMatch }
    })

    const passesGenreFilter = curSelectedGenres.length === 0
      ? prepared.map(() => true)
      : prepared.map(p => curSelectedGenres.every(g => p.genreSet.has(g)))

    const passesTagFilter = curSelectedTags.length === 0
      ? prepared.map(() => true)
      : prepared.map(p => curSelectedTags.every(t => p.tagSet.has(t)))

    const passesCategoryFilter = curSelectedCategories.length === 0
      ? prepared.map(() => true)
      : prepared.map(p => curSelectedCategories.includes(p.category))

    checkGenres.forEach((genre: string) => {
      if (curSelectedGenres.includes(genre)) {
        counts.genres[genre] = filteredCount
        return
      }

      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.formatMatch && passesTagFilter[i] && passesCategoryFilter[i] && passesGenreFilter[i] && p.genreSet.has(genre)) {
          count++
        }
      }
      counts.genres[genre] = count
    })

    checkTags.forEach((tag: string) => {
      if (curSelectedTags.includes(tag)) {
        counts.tags[tag] = filteredCount
        return
      }

      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.formatMatch && passesGenreFilter[i] && passesCategoryFilter[i] && passesTagFilter[i] && p.tagSet.has(tag)) {
          count++
        }
      }
      counts.tags[tag] = count
    })

    checkCategories.forEach((category: string) => {
      if (curSelectedCategories.includes(category)) {
        counts.categories[category] = filteredCount
        return
      }

      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.formatMatch && passesGenreFilter[i] && passesTagFilter[i] && (passesCategoryFilter[i] || p.category === category)) {
          count++
        }
      }
      counts.categories[category] = count
    })

    filterCounts.value = counts
  } finally {
    loadingFilterCounts.value = false
  }
}

// Toggle genre filter
const toggleGenreFilter = (genre: string) => {
  const index = selectedGenres.value.indexOf(genre)
  if (index === -1) {
    selectedGenres.value = [...selectedGenres.value, genre]
  } else {
    selectedGenres.value = selectedGenres.value.filter(g => g !== genre)
  }
  calculateFilterCounts()
}

// Toggle tag filter
const toggleTagFilter = (tag: string) => {
  const index = selectedTags.value.indexOf(tag)
  if (index === -1) {
    selectedTags.value = [...selectedTags.value, tag]
  } else {
    selectedTags.value = selectedTags.value.filter(t => t !== tag)
  }
  calculateFilterCounts()
}

// Toggle category filter
const toggleCategoryFilter = (category: string) => {
  const index = selectedCategories.value.indexOf(category)
  if (index === -1) {
    selectedCategories.value = [...selectedCategories.value, category]
  } else {
    selectedCategories.value = selectedCategories.value.filter(c => c !== category)
  }
  calculateFilterCounts()
}

// Clear all filters
const clearAllFilters = () => {
  selectedGenres.value = []
  selectedTags.value = []
  selectedCategories.value = []
  selectedFormat.value = 'all'
  calculateFilterCounts()
}

// Get filtered filmography based on selected genres/tags/format/category
const getFilteredFilmography = (filmography: any[]) => {
  if (!hasActiveFilters.value) {
    return filmography
  }

  return filmography.filter((credit: any) => {
    const animeGenres = credit.anime?.genres || []
    const animeTags = credit.anime?.tags?.map((t: any) => t.name) || []
    const animeFormat = credit.anime?.format
    const creditCategory = credit.category || 'other'

    const genreMatch = selectedGenres.value.length === 0 ||
      selectedGenres.value.every(g => animeGenres.includes(g))

    const tagMatch = selectedTags.value.length === 0 ||
      selectedTags.value.every(t => animeTags.includes(t))

    const categoryMatch = selectedCategories.value.length === 0 ||
      selectedCategories.value.includes(creditCategory)

    let formatMatch = true
    if (selectedFormat.value !== 'all') {
      const animeFormats = ['TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC']
      const mangaFormats = ['MANGA', 'NOVEL', 'ONE_SHOT']

      if (selectedFormat.value === 'anime') {
        formatMatch = animeFormats.includes(animeFormat)
      } else if (selectedFormat.value === 'manga') {
        formatMatch = mangaFormats.includes(animeFormat)
      }
    }

    return genreMatch && tagMatch && categoryMatch && formatMatch
  })
}

const sortedFilmography = computed(() => {
  if (!baseFilteredFilmography.value.length) return []

  const filteredFilmography = getFilteredFilmography(baseFilteredFilmography.value)

  return sortByYearAndScore(filteredFilmography, sortBy.value, sortOrder.value)
})

// Pagination
const SLOTS_PER_PAGE = 24
const currentPage = ref(1)

const paginatedPages = computed(() => {
  if (sortBy.value !== 'year' || !showYearMarkers.value) {
    return null
  }
  return paginateWithYearMarkers(sortedFilmography.value, cardsPerRow.value, SLOTS_PER_PAGE)
})

const totalPages = computed(() => {
  if (paginatedPages.value) {
    return Math.max(1, paginatedPages.value.length)
  }
  return Math.max(1, Math.ceil(sortedFilmography.value.length / SLOTS_PER_PAGE))
})

const activeFilmography = computed(() => {
  if (paginatedPages.value) {
    return paginatedPages.value[currentPage.value - 1] || []
  }
  const start = (currentPage.value - 1) * SLOTS_PER_PAGE
  return sortedFilmography.value.slice(start, start + SLOTS_PER_PAGE)
})

// Reset page when filters or sort changes
watch([selectedGenres, selectedTags, selectedCategories, selectedFormat, sortBy, sortOrder], () => {
  currentPage.value = 1
}, { deep: true })

// Computed for total and filtered counts
const totalFilmographyCount = computed(() => baseFilteredFilmography.value.length)
const filteredFilmographyCount = computed(() => {
  if (!hasActiveFilters.value) return totalFilmographyCount.value
  return getFilteredFilmography(baseFilteredFilmography.value).length
})

const goBack = () => {
  router.back()
}

const formatDate = (year?: number, month?: number, day?: number) => {
  if (!year) return ''

  const parts: (string | number)[] = [year]
  if (month) {
    const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
    parts.unshift(monthNames[month - 1])
  }
  if (day) {
    parts.unshift(day)
  }

  return parts.join(' ')
}

// Build hash string from tab, unselected indexes, max collaborators, exclude producers
const buildHash = (tab: string, unselectedIdxs: number[], maxCollabs: number, excludeProds: boolean) => {
  if (tab === 'overview') {
    return ''
  }

  let hash = `#${tab}`
  const params = new URLSearchParams()

  if (unselectedIdxs.length > 0) {
    params.set('u', unselectedIdxs.join(','))
  }

  if (maxCollabs !== 5) {
    params.set('n', maxCollabs.toString())
  }

  if (!excludeProds) {
    params.set('e', '0')
  }

  const queryString = params.toString()
  if (queryString) {
    hash += `?${queryString}`
  }

  return hash
}

// Update unselected indexes and URL
const updateUnselectedIndexes = (indexes: number[]) => {
  unselectedIndexes.value = indexes

  const newHash = buildHash(activeTab.value, indexes, maxCollaboratorsToShow.value, excludeProducers.value)
  const url = new URL(window.location.href)
  url.hash = newHash
  window.history.replaceState(window.history.state, '', url.toString())
}

// Update max collaborators and URL
const updateMaxCollaborators = (max: number) => {
  maxCollaboratorsToShow.value = max

  const newHash = buildHash(activeTab.value, unselectedIndexes.value, max, excludeProducers.value)
  const url = new URL(window.location.href)
  url.hash = newHash
  window.history.replaceState(window.history.state, '', url.toString())
}

// Update exclude producers and URL
const updateExcludeProducers = (exclude: boolean) => {
  excludeProducers.value = exclude

  const newHash = buildHash(activeTab.value, unselectedIndexes.value, maxCollaboratorsToShow.value, exclude)
  const url = new URL(window.location.href)
  url.hash = newHash
  window.history.replaceState(window.history.state, '', url.toString())
}

// Watch for tab changes and update URL hash
watch(activeTab, (newTab) => {
  const newHash = buildHash(newTab, unselectedIndexes.value, maxCollaboratorsToShow.value, excludeProducers.value)

  const url = new URL(window.location.href)
  url.hash = newHash
  window.history.replaceState(window.history.state, '', url.toString())
})

// Watch for hash changes (e.g., browser back/forward)
watch(() => route.hash, (newHash) => {
  const { tab: newTab, unselectedIndexes: newUnselected, maxCollaborators: newMax, excludeProducers: newExcludeProducers } = parseHash(newHash)

  if (activeTab.value !== newTab) {
    activeTab.value = newTab
  }

  const currentUnselected = unselectedIndexes.value.slice().sort().join(',')
  const newUnselectedStr = newUnselected.slice().sort().join(',')
  if (currentUnselected !== newUnselectedStr) {
    unselectedIndexes.value = newUnselected
  }

  if (maxCollaboratorsToShow.value !== newMax) {
    maxCollaboratorsToShow.value = newMax
  }

  if (excludeProducers.value !== newExcludeProducers) {
    excludeProducers.value = newExcludeProducers
  }
})

// Reset filters when staff changes
watch(staffId, () => {
  selectedGenres.value = []
  selectedTags.value = []
  selectedCategories.value = []
})

// Watch for staff data loading - calculate initial counts
watch(() => staff.value, (newStaff) => {
  if (newStaff) {
    calculateFilterCounts()
  }
})

// Watch for filter changes and includeAdult changes
watch([selectedGenres, selectedTags, includeAdult], () => {
  if (staff.value) {
    calculateFilterCounts()
  }
}, { deep: true })

// Set page title
watchEffect(() => {
  const title = staff.value ? (staff.value.name_en || staff.value.name_ja || 'Unknown Staff') : 'Loading...'
  document.title = title + ' - Anigraph'
})
</script>

<style scoped>
.sticky-card {
  position: sticky;
  top: 80px;
  max-height: calc(100vh - 100px);
  overflow-y: auto;
  /* Hide scrollbar for Chrome, Safari and Opera */
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE and Edge */
}

.sticky-card::-webkit-scrollbar {
  display: none; /* Chrome, Safari, Opera */
}

/* Only apply sticky on medium+ screens */
@media (max-width: 960px) {
  .sticky-card {
    position: static;
    max-height: none;
    overflow-y: visible;
  }
}

.staff-image-container {
  display: flex;
  justify-content: center;
}

.staff-image-container .v-img {
  width: 100%;
  max-width: 200px;
}

.cursor-pointer {
  cursor: pointer;
}

.filter-info-box {
  padding: 8px 12px;
  border-radius: 4px;
  background-color: rgba(var(--v-theme-primary), 0.08);
  border-left: 3px solid rgb(var(--v-theme-primary));
}

.selected-genre-filter {
  background-color: rgb(var(--v-theme-secondary)) !important;
  color: rgb(var(--v-theme-on-secondary)) !important;
  border-color: rgb(var(--v-theme-secondary)) !important;
  font-weight: 600;
}

.selected-genre-filter :deep(.v-icon) {
  color: rgb(var(--v-theme-on-secondary)) !important;
}

.selected-tag-filter {
  background-color: rgb(var(--v-theme-accent)) !important;
  color: rgb(var(--v-theme-on-accent)) !important;
  font-weight: 600;
}

.selected-tag-filter :deep(.v-icon) {
  color: rgb(var(--v-theme-on-accent)) !important;
}

/* Ensure v-window takes full width of parent column */
:deep(.v-window) {
  width: 100%;
}

.spacer-col {
  padding-top: 12px !important;
  padding-bottom: 12px !important;
}

.year-marker-col {
  padding-top: 12px !important;
  padding-bottom: 12px !important;
}

.staff-bio :deep(strong) {
  display: block;
  margin-top: 1em;
  margin-bottom: 0.25em;
  font-size: 1.1em;
}

.staff-bio :deep(strong:first-child) {
  margin-top: 0;
}

.staff-bio :deep(a) {
  color: rgb(var(--v-theme-primary));
  text-decoration: none;
}

.staff-bio :deep(a:hover) {
  text-decoration: underline;
}

.staff-bio :deep(p) {
  margin-bottom: 0.75em;
}

.staff-bio :deep(ul) {
  padding-left: 1.5em;
  margin-bottom: 0.75em;
}

</style>
