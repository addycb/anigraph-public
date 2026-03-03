<template>
  <v-app>
    <AppBar clickable-title />

    <v-main class="franchise-page">
      <!-- Loading State -->
      <v-container v-if="loading" class="loading-container">
        <v-progress-circular
          indeterminate
          color="primary"
          size="64"
        ></v-progress-circular>
        <p class="text-h6 mt-4">Loading franchise...</p>
      </v-container>

      <!-- Error State -->
      <v-container v-else-if="error" class="error-container">
        <v-icon size="64" color="error">mdi-alert-circle</v-icon>
        <p class="text-h6 mt-4">{{ error }}</p>
        <v-btn color="primary" class="mt-4" to="/">
          Return Home
        </v-btn>
      </v-container>

      <!-- Franchise Details -->
      <v-container v-else-if="franchise" fluid class="franchise-content">
        <!-- Hero Section -->
        <v-card class="hero-card mb-4">
          <v-card-text class="text-center pa-6">
            <v-icon size="48" color="primary" class="mb-3">mdi-family-tree</v-icon>
            <h1 class="text-h4 mb-2">{{ franchise.title }}</h1>
            <p class="text-subtitle-1 text-medium-emphasis">
              {{ totalEntriesCount }} {{ totalEntriesCount === 1 ? 'Entry' : 'Entries' }}
            </p>
          </v-card-text>
        </v-card>

        <!-- Filters Section -->
        <v-card v-if="(franchise.genreStats && franchise.genreStats.length > 0) || (franchise.tagStats && franchise.tagStats.length > 0)" class="mb-4 filters-card">
          <v-card-text>
            <!-- Genres -->
            <FilterSection
              v-if="franchise.genreStats && franchise.genreStats.length > 0"
              title="Genres"
              :items="franchise.genreStats"
              :selected-items="selectedGenres"
              :filter-counts="filterCounts.genres"
              :loading-counts="loadingFilterCounts"
              :has-active-filters="hasActiveFilters"
              :limit="8"
              chip-size="small"
              chip-variant="outlined"
              selected-class="selected-genre-filter"
              section-class="mb-4"
              @toggle="toggleGenreFilter"
              inline
            />

            <!-- Tags -->
            <FilterSection
              v-if="franchise.tagStats && franchise.tagStats.length > 0"
              title="Common Themes"
              :items="franchise.tagStats"
              :selected-items="selectedTags"
              :filter-counts="filterCounts.tags"
              :loading-counts="loadingFilterCounts"
              :has-active-filters="hasActiveFilters"
              :limit="8"
              chip-size="small"
              chip-variant="tonal"
              selected-class="selected-tag-filter"
              @toggle="toggleTagFilter"
              inline
            />
          </v-card-text>
        </v-card>

        <!-- Entries Section -->
        <div class="entries-section">
          <!-- View Toolbar -->
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
              <!-- Format Filter - Only show when has both anime and manga -->
              <v-select
                v-if="hasBothAnimeAndManga"
                v-model="selectedFormat"
                :items="formatOptions"
                label="Format"
                variant="outlined"
                density="compact"
                hide-details
                style="max-width: 130px;"
                @update:model-value="calculateFilterCounts"
              ></v-select>
            </template>
          </ViewToolbar>

          <!-- Entries Grid -->
          <DynamicScroller
            :items="entryRows"
            :min-item-size="200"
            key-field="id"
            page-mode
          >
            <template #default="{ item: row, index, active }">
              <DynamicScrollerItem
                :item="row"
                :active="active"
                :data-index="index"
              >
                <v-row>
                  <v-col
                    v-for="(item, colIdx) in row.items"
                    :key="item.isYearMarker ? `year-${item.year}` : item.isSpacer ? `spacer-${row.id}-${colIdx}` : item.anime?.anilistId || `${row.id}-${colIdx}`"
                    cols="12" sm="6" md="4" :lg="cardColSize"
                    :class="{ 'spacer-col': item.isSpacer && showYearMarkers, 'year-marker-col': item.isYearMarker && showYearMarkers }"
                  >
                    <template v-if="item.isSpacer && showYearMarkers">
                      <YearCard spacer />
                    </template>
                    <template v-else-if="item.isYearMarker && showYearMarkers">
                      <YearCard :year="item.year" :count="item.count" count-label="entry" />
                    </template>
                    <template v-else-if="!item.isSpacer && !item.isYearMarker">
                      <AnimeCard
                        :anime="item.anime"
                        :show-season="true"
                        :show-year="!showYearMarkers || sortBy === 'score'"
                      />
                    </template>
                  </v-col>
                </v-row>
              </DynamicScrollerItem>
            </template>
          </DynamicScroller>
        </div>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '@/utils/api'
import { filterAdultContent } from '@/utils/contentFilters'
import { sortByYearAndScore } from '@/utils/sorting'
import { flattenWithYearMarkers } from '@/utils/yearMarkers'
import { useSettings } from '@/composables/useSettings'
import { useSortable } from '@/composables/useSortable'
import { useCardSize } from '@/composables/useCardSize'
import { useVirtualGrid } from '@/composables/useVirtualGrid'

const route = useRoute()
const { includeAdult } = useSettings()

const franchiseId = computed(() => {
  const idSegments = route.params.id
  return Array.isArray(idSegments) ? idSegments.join('/') : idSegments
})

// Manual fetch replacing useAsyncData
const franchiseResponse = ref<any>(null)
const pending = ref(false)
const fetchError = ref<string | null>(null)

async function fetchFranchise() {
  if (!franchiseId.value) return
  pending.value = true
  fetchError.value = null
  try {
    franchiseResponse.value = await api<any>(`/franchise/${encodeURIComponent(franchiseId.value)}`)
  } catch (e: any) {
    fetchError.value = e.message
  } finally {
    pending.value = false
  }
}

onMounted(fetchFranchise)
watch(franchiseId, fetchFranchise)

const franchise = computed(() => franchiseResponse.value?.success ? franchiseResponse.value.data : null)
const loading = computed(() => pending.value)
const error = computed(() => {
  if (fetchError.value) return fetchError.value || 'Failed to load franchise'
  if (franchiseResponse.value && !franchiseResponse.value.success) return 'Failed to load franchise'
  return ''
})

// Filter state
const selectedGenres = ref<string[]>([])
const selectedTags = ref<string[]>([])
const selectedFormat = ref<'all' | 'anime' | 'manga'>('all')
const filterCounts = ref<any>({ genres: {}, tags: {} })
const loadingFilterCounts = ref(false)

// Sort state
const { sortBy, sortOrder, sortOptions, toggleSortOrder } = useSortable('year', 'desc')

// Card size state
const { cardSize, cardColSize, cardsPerRow, showYearMarkers } = useCardSize('small')

// Filters expansion panel state
const filtersOpen = ref(0)

// Check if franchise has both anime and manga formats
const hasBothAnimeAndManga = computed(() => {
  if (!baseFilteredEntries.value.length) return false

  const animeFormats = ['TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC']
  const mangaFormats = ['MANGA', 'NOVEL', 'ONE_SHOT', 'LIGHT_NOVEL']

  let hasAnime = false
  let hasManga = false

  baseFilteredEntries.value.forEach((entry: any) => {
    if (animeFormats.includes(entry.anime?.format)) {
      hasAnime = true
    } else if (mangaFormats.includes(entry.anime?.format)) {
      hasManga = true
    }
  })

  return hasAnime && hasManga
})

// Format options
const formatOptions = [
  { value: 'all', title: 'All' },
  { value: 'anime', title: 'Anime' },
  { value: 'manga', title: 'Manga' }
]

const getScoreColor = (score: number) => {
  if (score >= 80) return 'success'
  if (score >= 70) return 'primary'
  if (score >= 60) return 'warning'
  return 'error'
}

// Check if any filters are active
const hasActiveFilters = computed(() => {
  return selectedGenres.value.length > 0 || selectedTags.value.length > 0 || selectedFormat.value !== 'all'
})

// Base filtered entries (adult content filter)
const baseFilteredEntries = computed(() => {
  if (!franchise.value?.entries) return []
  return filterAdultContent(franchise.value.entries, includeAdult.value)
})

// Get filtered entries based on selected genres/tags/format
const getFilteredEntries = (entries: any[]) => {
  if (!hasActiveFilters.value) {
    return entries
  }

  return entries.filter((entry: any) => {
    const entryGenres = entry.anime?.genres || []
    const entryTags = entry.anime?.tags?.map((t: any) => t.name) || []
    const entryFormat = entry.anime?.format

    const genreMatch = selectedGenres.value.length === 0 ||
      selectedGenres.value.every(g => entryGenres.includes(g))

    const tagMatch = selectedTags.value.length === 0 ||
      selectedTags.value.every(t => entryTags.includes(t))

    // Format filter
    let formatMatch = true
    if (selectedFormat.value !== 'all') {
      const animeFormats = ['TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC']
      const mangaFormats = ['MANGA', 'NOVEL', 'ONE_SHOT']

      if (selectedFormat.value === 'anime') {
        formatMatch = animeFormats.includes(entryFormat)
      } else if (selectedFormat.value === 'manga') {
        formatMatch = mangaFormats.includes(entryFormat)
      }
    }

    return genreMatch && tagMatch && formatMatch
  })
}

// Apply filters and sort
const filteredEntries = computed(() => {
  const filtered = getFilteredEntries(baseFilteredEntries.value)
  return sortByYearAndScore(filtered, sortBy.value, sortOrder.value)
})

// Flatten with year markers
const activeEntries = computed(() => {
  if (sortBy.value !== 'year' || !showYearMarkers.value) return filteredEntries.value
  return flattenWithYearMarkers(filteredEntries.value, cardsPerRow.value)
})

// Virtual grid rows for entries
const { rows: entryRows } = useVirtualGrid(activeEntries, cardColSize)

// Counts
const totalEntriesCount = computed(() => baseFilteredEntries.value.length)
const filteredEntriesCount = computed(() => {
  if (!hasActiveFilters.value) return totalEntriesCount.value
  return getFilteredEntries(baseFilteredEntries.value).length
})

// Calculate filter counts
const calculateFilterCounts = () => {
  if (!franchise.value) {
    return
  }

  loadingFilterCounts.value = true

  try {
    const checkGenres = franchise.value.genreStats?.map((g: any) => g.name) || []
    const checkTags = franchise.value.tagStats?.map((t: any) => t.name) || []

    if (checkGenres.length === 0 && checkTags.length === 0) {
      loadingFilterCounts.value = false
      return
    }

    const counts: any = {
      genres: {},
      tags: {}
    }

    // Initialize all to 0
    checkGenres.forEach((g: string) => counts.genres[g] = 0)
    checkTags.forEach((t: string) => counts.tags[t] = 0)

    const filteredCount = getFilteredEntries(baseFilteredEntries.value).length
    const curSelectedGenres = selectedGenres.value
    const curSelectedTags = selectedTags.value
    const curFormat = selectedFormat.value

    // Pre-extract data per entry once
    const animeFormats = ['TV', 'MOVIE', 'OVA', 'ONA', 'SPECIAL', 'TV_SHORT', 'MUSIC']
    const mangaFormats = ['MANGA', 'NOVEL', 'ONE_SHOT']

    const prepared = baseFilteredEntries.value.map((entry: any) => {
      const genreSet = new Set<string>(entry.anime?.genres || [])
      const tagSet = new Set<string>(entry.anime?.tags?.map((t: any) => t.name) || [])
      const format = entry.anime?.format
      let formatMatch = true
      if (curFormat !== 'all') {
        if (curFormat === 'anime') {
          formatMatch = animeFormats.includes(format)
        } else if (curFormat === 'manga') {
          formatMatch = mangaFormats.includes(format)
        }
      }
      return { genreSet, tagSet, formatMatch }
    })

    // Pre-check which entries pass current genre/tag filters
    const passesGenreFilter = curSelectedGenres.length === 0
      ? null
      : prepared.map(p => curSelectedGenres.every(g => p.genreSet.has(g)))

    const passesTagFilter = curSelectedTags.length === 0
      ? null
      : prepared.map(p => curSelectedTags.every(t => p.tagSet.has(t)))

    // For each genre, simulate adding it and count results
    checkGenres.forEach((genre: string) => {
      if (curSelectedGenres.includes(genre)) {
        counts.genres[genre] = filteredCount
        return
      }

      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.formatMatch && (passesTagFilter === null || passesTagFilter[i]) && (passesGenreFilter === null || passesGenreFilter[i]) && p.genreSet.has(genre)) {
          count++
        }
      }
      counts.genres[genre] = count
    })

    // For each tag, simulate adding it and count results
    checkTags.forEach((tag: string) => {
      if (curSelectedTags.includes(tag)) {
        counts.tags[tag] = filteredCount
        return
      }

      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.formatMatch && (passesGenreFilter === null || passesGenreFilter[i]) && (passesTagFilter === null || passesTagFilter[i]) && p.tagSet.has(tag)) {
          count++
        }
      }
      counts.tags[tag] = count
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

// Clear all filters
const clearAllFilters = () => {
  selectedGenres.value = []
  selectedTags.value = []
  selectedFormat.value = 'all'
  calculateFilterCounts()
}


// Watch for filter/sort changes and recalculate counts
watch([selectedGenres, selectedTags, includeAdult], () => {
  if (franchise.value) {
    calculateFilterCounts()
  }
}, { deep: true })

// Watch for franchise data loading - calculate initial counts
watch(() => franchise.value, (newFranchise) => {
  if (newFranchise) {
    calculateFilterCounts()
  }
})

watchEffect(() => {
  document.title = (franchise.value?.title ? `${franchise.value.title} Franchise` : 'Franchise') + ' - Anigraph'
})
</script>

<style scoped>
.franchise-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px;
  padding-bottom: 40px;
}

.franchise-content {
  animation: fadeIn 0.6s ease-out;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.loading-container,
.error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  color: var(--color-text);
}

/* Hero Card */
.hero-card {
  background: linear-gradient(135deg, rgba(var(--color-primary-rgb), 0.2) 0%, rgba(var(--color-accent-rgb), 0.2) 100%);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-strong);
}

/* Filters Card */
.filters-card {
  background: linear-gradient(135deg, rgba(var(--color-surface-rgb), 0.7) 0%, rgba(var(--color-bg-rgb), 0.9) 100%);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-border);
}

.selected-genre-filter {
  background: var(--gradient-primary) !important;
  color: var(--color-text) !important;
  border-color: transparent !important;
  font-weight: 600;
  box-shadow: 0 0 12px rgba(var(--color-primary-rgb), 0.4);
}

.selected-tag-filter {
  background: var(--gradient-secondary) !important;
  color: var(--color-text) !important;
  font-weight: 600;
  box-shadow: 0 0 12px rgba(236, 72, 153, 0.4);
}

/* Entries Section */
.entries-section {
  animation: fadeIn 0.8s ease-out;
}

.spacer-col {
  padding-top: 12px !important;
  padding-bottom: 12px !important;
}

.year-marker-col {
  padding-top: 12px !important;
  padding-bottom: 12px !important;
}

/* Responsive */
@media (max-width: 960px) {
  .franchise-page {
    padding-top: 56px;
    padding-bottom: 30px;
  }

  .spacer-col {
    padding-top: 8px !important;
    padding-bottom: 8px !important;
  }

  .year-marker-col {
    padding-top: 8px !important;
    padding-bottom: 8px !important;
  }
}

@media (max-width: 600px) {
  .spacer-col,
  .year-marker-col {
    padding-top: 6px !important;
    padding-bottom: 6px !important;
  }
}
</style>
