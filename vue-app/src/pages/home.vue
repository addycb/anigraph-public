<template>
  <v-app>
    <AppBar clickable-title @title-click="handleClearSearch" />

    <v-main class="masonry-page">
      <!-- Toolbar: Title + Filters + Card Size -->
      <v-container fluid class="toolbar-section">
        <ViewToolbar
          v-model:card-size="cardSize"
          :show-sort="false"
          :show-year-markers="false"
        >
          <template #left>
            <div class="d-flex align-center flex-wrap" style="gap: 12px;">
              <div class="page-title-section">
                <div class="d-flex align-center">
                  <v-icon color="primary" size="28" class="mr-2">mdi-view-grid</v-icon>
                  <h1 class="text-h5 font-weight-bold mb-0 page-title-gradient">{{ pageTitle }}</h1>
                </div>
                <p class="text-caption text-medium-emphasis ml-9 mb-0">
                  Discover anime and manga
                </p>
              </div>
              <v-divider vertical class="mx-2" style="height: 40px;"></v-divider>

              <!-- Era Filter -->
              <v-select
                v-model="selectedEra"
                :items="eraOptions"
                label="Era"
                clearable
                variant="outlined"
                density="compact"
                hide-details
                style="width: 145px;"
                @update:model-value="onFilterChange"
              ></v-select>

              <!-- Genres -->
              <v-autocomplete
                v-model="selectedGenres"
                :items="availableGenres"
                :loading="loadingFilters"
                label="Genres"
                variant="outlined"
                density="compact"
                multiple
                chips
                closable-chips
                clearable
                hide-details
                style="width: 200px;"
                @update:model-value="onFilterChange"
              >
                <template #chip="{ props, item }">
                  <v-chip
                    v-bind="props"
                    :text="item.title"
                    size="small"
                  ></v-chip>
                </template>
              </v-autocomplete>

              <!-- Tags -->
              <v-autocomplete
                v-model="selectedTags"
                :items="availableTags"
                :loading="loadingFilters"
                label="Tags"
                variant="outlined"
                density="compact"
                multiple
                chips
                closable-chips
                clearable
                hide-details
                style="width: 200px;"
                @update:model-value="onFilterChange"
              >
                <template #chip="{ props, item }">
                  <v-chip
                    v-bind="props"
                    :text="item.title"
                    size="small"
                  ></v-chip>
                </template>
              </v-autocomplete>

              <!-- Sort By -->
              <div class="d-flex align-center" style="gap: 4px;">
                <v-select
                  v-model="currentSort"
                  :items="sortOptions"
                  label="Sort by"
                  variant="outlined"
                  density="compact"
                  hide-details
                  style="width: 130px;"
                  @update:model-value="onFilterChange"
                ></v-select>
                <v-btn
                  v-if="currentSort !== 'random'"
                  icon
                  color="primary"
                  variant="tonal"
                  density="compact"
                  :title="sortOrder === 'desc' ? 'Descending' : 'Ascending'"
                  @click="toggleSortOrder"
                >
                  <v-icon>{{ sortOrder === 'desc' ? 'mdi-arrow-down' : 'mdi-arrow-up' }}</v-icon>
                </v-btn>
              </div>

              <!-- Custom Year Range (shown when Custom Range is selected) -->
              <template v-if="selectedEra === 'custom'">
                <v-text-field
                  v-model.number="customYearMin"
                  label="Year From"
                  type="number"
                  variant="outlined"
                  density="compact"
                  hide-details
                  :min="1940"
                  :max="2030"
                  style="width: 110px;"
                  @update:model-value="onFilterChange"
                ></v-text-field>
                <v-text-field
                  v-model.number="customYearMax"
                  label="Year To"
                  type="number"
                  variant="outlined"
                  density="compact"
                  hide-details
                  :min="1940"
                  :max="2030"
                  style="width: 110px;"
                  @update:model-value="onFilterChange"
                ></v-text-field>
              </template>

              <!-- Clear Filters Button -->
              <v-btn
                v-if="hasActiveFilters"
                variant="text"
                color="error"
                density="compact"
                @click="clearAllFilters"
              >
                <v-icon start>mdi-filter-remove</v-icon>
                Clear All
              </v-btn>
            </div>
          </template>
        </ViewToolbar>
      </v-container>

      <!-- No Results State -->
      <v-container v-if="!initialLoading && animeList.length === 0" class="empty-state-inline">
        <v-icon size="64" color="grey">mdi-animation-outline</v-icon>
        <p class="text-h6 mt-4">No anime found</p>
        <p class="text-body-1 text-medium-emphasis">
          Try different filters or search query
        </p>
      </v-container>

      <!-- Loading State (Initial Load) -->
      <v-container v-if="initialLoading" class="loading-container">
        <v-progress-circular
          indeterminate
          color="primary"
          size="64"
        ></v-progress-circular>
        <p class="text-h6 mt-4">Loading anime...</p>
      </v-container>

      <!-- Anime Grid -->
      <v-container v-if="!initialLoading && animeList.length > 0" fluid class="grid-section">
        <DynamicScroller
          :items="animeRows"
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
                  v-for="(anime, colIdx) in row.items"
                  :key="anime.id || anime.anilistId || `${row.id}-${colIdx}`"
                  cols="12" sm="6" md="4" :lg="cardColSize"
                >
                  <AnimeCard :anime="anime" />
                </v-col>
              </v-row>
            </DynamicScrollerItem>
          </template>
        </DynamicScroller>
      </v-container>

      <!-- Loading More Indicator / Infinite Scroll Sentinel -->
      <div ref="sentinel" class="loading-more">
        <v-progress-circular
          v-if="loadingMore"
          indeterminate
          color="primary"
          size="48"
        ></v-progress-circular>
      </div>

      <!-- Floating Search Bar -->
      <SearchBar
        floating
        show-arrow-button
        density="comfortable"
        hide-details
        label=""
        placeholder="Search works, staff, studios..."
        tracking-source="home"
        @search="handleSearch"
      />
    </v-main>

    <AppFooter />
  </v-app>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSettings } from '@/composables/useSettings'
import { useCardSize } from '@/composables/useCardSize'
import { useVirtualGrid } from '@/composables/useVirtualGrid'
import { api } from '@/utils/api'

const route = useRoute()
const router = useRouter()
const { includeAdult } = useSettings()

const animeList = shallowRef<any[]>([])
const initialLoading = ref(true)
const loadingMore = ref(false)
const isSearchMode = ref(false)
const currentSearchQuery = ref('')
const sentinel = ref<HTMLElement | null>(null)
const currentOffset = ref(0)
const hasMoreResults = ref(true)

// Card size
const { cardSize, cardColSize } = useCardSize('small')

// Virtual grid rows for anime list
const { rows: animeRows } = useVirtualGrid(animeList, cardColSize)

// Sort, type, and format from URL
const currentSort = ref<string>('random')
const sortOrder = ref<'asc' | 'desc'>('desc')
const currentType = ref<string | null>(null) // null means all types
const currentFormat = ref<string | null>(null)

// Mobile detection
const isMobile = ref(false)
const updateMobileStatus = () => {
  isMobile.value = window.innerWidth < 600
}
updateMobileStatus()
window.addEventListener('resize', updateMobileStatus)

// Initialize includeAdult from URL query if present
if (route.query.includeAdult === 'true') {
  const { setIncludeAdult } = useSettings()
  setIncludeAdult(true)
}

// Filters
const selectedEra = ref<string | null>(null)
const customYearMin = ref<number | null>(null)
const customYearMax = ref<number | null>(null)
const selectedGenres = ref<string[]>([])
const selectedTags = ref<string[]>([])
const availableGenres = ref<string[]>([])
const availableTags = ref<string[]>([])
const loadingFilters = ref(false)

const sortOptions = [
  { title: 'Random', value: 'random' },
  { title: 'Score', value: 'score' },
  { title: 'Year', value: 'year' },
  { title: 'Title', value: 'title' },
]

const eraOptions = [
  { title: 'Pre-1960s', value: 'pre-1960' },
  { title: '1960s-1980s', value: '1960s-1980s' },
  { title: '1990s-2000s', value: '1990s-2000s' },
  { title: '2010s', value: '2010s' },
  { title: '2020s', value: '2020s' },
  { title: 'Custom Range', value: 'custom' }
]

// Format label mapping (matches AppBar browse menu labels)
const formatLabels: Record<string, string> = {
  'TV': 'TV Series',
  'MOVIE': 'Movies',
  'OVA': 'OVA',
  'ONA': 'ONA',
  'SPECIAL': 'Specials',
  'TV_SHORT': 'TV Shorts',
  'MUSIC': 'Music',
  'MANGA': 'Manga',
  'NOVEL': 'Light Novels',
  'ONE_SHOT': 'One Shots'
}

// Check if we're on base /home with no URL params (default view)
const isBaseHomePage = computed(() => {
  const q = route.query
  return !q.type && !q.format && !q.q && !q.sort && !q.era && !q.yearMin && !q.yearMax && !q.genres && !q.tags
})

// Computed page title based on sort, type, and search mode
const pageTitle = computed(() => {
  if (isSearchMode.value) return 'Search Results'

  // Determine type label
  let typeLabel = 'Anime & Manga' // Default to both on home page
  if (currentType.value === 'manga') {
    typeLabel = currentFormat.value ? '' : 'All Manga'
  } else if (currentType.value === 'anime') {
    typeLabel = currentFormat.value ? '' : 'All Anime'
  }

  // Get formatted label for the format
  const formatLabel = currentFormat.value ? formatLabels[currentFormat.value] || currentFormat.value : ''

  // Build final title
  const contentLabel = formatLabel || typeLabel

  switch (currentSort.value) {
    case 'top':
      return `Top Rated ${contentLabel}`
    case 'new':
      return `New ${contentLabel}`
    case 'trending':
      return `Trending ${contentLabel}`
    case 'popular':
      return `Popular ${contentLabel}`
    case 'score':
      return `Top Rated ${contentLabel}`
    case 'year':
      return `Newest ${contentLabel}`
    case 'title':
      return `${contentLabel} A\u2013Z`
    default:
      return contentLabel
  }
})

// Fetch available genres and tags
const fetchFilters = async () => {
  loadingFilters.value = true
  try {
    const response = await api('/anime/genres-tags', { params: { includeAdult: includeAdult.value } })
    if (response.success) {
      availableGenres.value = response.genres || []
      availableTags.value = response.tags || []
    }
  } catch (error) {
    // Silently fail - filters won't be available
  } finally {
    loadingFilters.value = false
  }
}


const fetchRandomAnime = async (limit: number = 18, offset: number = 0) => {
  try {
    const params: any = {
      limit,
      offset,
      includeAdult: includeAdult.value,
      sort: currentSort.value,
      order: sortOrder.value
    }

    // On base /home page (no URL params), default to both anime and manga with >2 staff and ratings
    if (isBaseHomePage.value) {
      params.minStaff = 3
      params.hasRating = true
    } else if (currentType.value) {
      // Otherwise use URL-specified type
      params.type = currentType.value
    }

    // Add format filter if present
    if (currentFormat.value) {
      params.format = currentFormat.value
    }

    // Add era filter
    if (selectedEra.value) {
      if (selectedEra.value === 'custom') {
        // Use custom year range
        if (customYearMin.value) {
          params.yearMin = customYearMin.value
        }
        if (customYearMax.value) {
          params.yearMax = customYearMax.value
        }
      } else {
        // Use preset era
        params.eras = [selectedEra.value]
      }
    }

    // Add genre filters (comma-separated)
    if (selectedGenres.value.length > 0) {
      params.genres = selectedGenres.value.join(',')
    }

    // Add tag filters (comma-separated)
    if (selectedTags.value.length > 0) {
      params.tags = selectedTags.value.join(',')
    }

    const response = await api('/anime/popular', { params })
    return response.success && response.data ? response.data : []
  } catch {
    return []
  }
}

const toggleSortOrder = () => {
  sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  onFilterChange()
}

const clearAllFilters = async () => {
  selectedEra.value = null
  customYearMin.value = null
  customYearMax.value = null
  selectedGenres.value = []
  selectedTags.value = []
  await onFilterChange()
}

const onFilterChange = async () => {
  // Track filter change event in Umami
  if (typeof window !== 'undefined' && (window as any).umami) {
    const filterData: Record<string, any> = {}
    if (selectedEra.value) filterData.era = selectedEra.value
    if (customYearMin.value) filterData.yearMin = customYearMin.value
    if (customYearMax.value) filterData.yearMax = customYearMax.value
    if (selectedGenres.value.length > 0) filterData.genres = selectedGenres.value.join(',')
    if (selectedTags.value.length > 0) filterData.tags = selectedTags.value.join(',')

    // Only track if there are active filters
    if (Object.keys(filterData).length > 0) {
      (window as any).umami.track('filter', filterData)
    }
  }

  // If in search mode, re-search with the same query but updated filters
  if (isSearchMode.value && currentSearchQuery.value) {
    router.push({
      query: {
        q: currentSearchQuery.value,
        sort: currentSort.value && currentSort.value !== 'random' ? currentSort.value : undefined,
        order: sortOrder.value !== 'desc' ? sortOrder.value : undefined,
        type: currentType.value || undefined,
        format: currentFormat.value || undefined,
        era: selectedEra.value || undefined,
        yearMin: customYearMin.value || undefined,
        yearMax: customYearMax.value || undefined,
        genres: selectedGenres.value.length > 0 ? selectedGenres.value.join(',') : undefined,
        tags: selectedTags.value.length > 0 ? selectedTags.value.join(',') : undefined,
        includeAdult: includeAdult.value ? 'true' : undefined
      }
    })

    initialLoading.value = true
    animeList.value = await searchAnime(currentSearchQuery.value)
    initialLoading.value = false
    return
  }

  // Update route with filter parameters (browse mode)
  router.push({
    query: {
      sort: currentSort.value || undefined,
      order: sortOrder.value !== 'desc' ? sortOrder.value : undefined,
      type: currentType.value || undefined,
      format: currentFormat.value || undefined,
      era: selectedEra.value || undefined,
      yearMin: customYearMin.value || undefined,
      yearMax: customYearMax.value || undefined,
      genres: selectedGenres.value.length > 0 ? selectedGenres.value.join(',') : undefined,
      tags: selectedTags.value.length > 0 ? selectedTags.value.join(',') : undefined,
      includeAdult: includeAdult.value ? 'true' : undefined
    }
  })

  initialLoading.value = true
  currentOffset.value = 0 // Reset offset when filters change
  hasMoreResults.value = true // Reset hasMoreResults flag
  const results = await fetchRandomAnime(18, 0)
  animeList.value = results
  currentOffset.value = results.length // Update offset for next load
  hasMoreResults.value = results.length >= 18 // If we got less than requested, we're at the end
  initialLoading.value = false
}

const searchAnime = async (query: string) => {
  try {
    const params: any = {
      q: query,
      limit: 18,
      includeAdult: includeAdult.value
    }

    // Add type filter if present (null = all types)
    if (currentType.value) {
      params.type = currentType.value
    }

    // Add format filter
    if (currentFormat.value) {
      params.format = currentFormat.value
    }

    // Add era filter
    if (selectedEra.value) {
      if (selectedEra.value === 'custom') {
        if (customYearMin.value) params.yearMin = customYearMin.value
        if (customYearMax.value) params.yearMax = customYearMax.value
      } else {
        params.eras = [selectedEra.value]
      }
    }

    // Add sort
    if (currentSort.value && currentSort.value !== 'random') {
      params.sort = currentSort.value
      params.order = sortOrder.value
    }

    const response = await api('/search/unified', { params })
    return response.success && response.results?.anime ? response.results.anime : []
  } catch {
    return []
  }
}

const loadInitialAnime = async () => {
  initialLoading.value = true
  currentOffset.value = 0
  hasMoreResults.value = true
  const results = await fetchRandomAnime(18, 0)
  animeList.value = results
  currentOffset.value = shouldUseOffset.value ? results.length : 0 // Track offset if not random mode
  hasMoreResults.value = results.length >= 18 // If we got less than requested, we're at the end
  initialLoading.value = false
}

const hasActiveFilters = computed(() => {
  return selectedEra.value !== null || customYearMin.value !== null || customYearMax.value !== null || selectedGenres.value.length > 0 || selectedTags.value.length > 0
})

// Check if we should use offset-based pagination (not random sort)
const shouldUseOffset = computed(() => {
  return currentSort.value !== 'random' || hasActiveFilters.value || currentFormat.value !== null
})

const activeFilterCount = computed(() => {
  let count = 0
  if (selectedEra.value) count++
  if (selectedGenres.value.length > 0) count += selectedGenres.value.length
  if (selectedTags.value.length > 0) count += selectedTags.value.length
  return count
})

const loadMore = async () => {
  // Don't load more if in search mode, already loading, no more results, or at cap
  if (loadingMore.value || isSearchMode.value || !hasMoreResults.value || animeList.value.length >= 500) return

  loadingMore.value = true
  // Use offset for sorted/filtered queries, 0 for random
  const offset = shouldUseOffset.value ? currentOffset.value : 0
  const newAnime = await fetchRandomAnime(18, offset)
  if (newAnime.length > 0) {
    animeList.value = [...animeList.value, ...newAnime]
    // Update offset if using offset pagination
    if (shouldUseOffset.value) {
      currentOffset.value += newAnime.length
    }
    // Check if we got fewer results than requested
    hasMoreResults.value = newAnime.length >= 18
  } else {
    // No more results
    hasMoreResults.value = false
  }
  loadingMore.value = false
}

const handleSearch = async (query: string) => {
  // Skip if already searching for the same query (prevents duplicate analytics)
  if (isSearchMode.value && currentSearchQuery.value === query) return

  initialLoading.value = true
  isSearchMode.value = true
  currentSearchQuery.value = query

  // Clear filters when searching
  selectedEra.value = null
  customYearMin.value = null
  customYearMax.value = null
  selectedGenres.value = []
  selectedTags.value = []

  router.push({
    query: {
      q: query,
      type: currentType.value || undefined,
      format: currentFormat.value || undefined,
      includeAdult: includeAdult.value ? 'true' : undefined
    }
  })

  // Track search event in Umami
  if (typeof window !== 'undefined' && (window as any).umami) {
    (window as any).umami.track('search', { query })
  }

  animeList.value = await searchAnime(query)
  initialLoading.value = false
}

const handleClearSearch = () => {
  isSearchMode.value = false
  currentSearchQuery.value = ''

  router.push({
    query: {
      sort: currentSort.value || undefined,
      order: sortOrder.value !== 'desc' ? sortOrder.value : undefined,
      type: currentType.value || undefined,
      format: currentFormat.value || undefined,
      era: selectedEra.value || undefined,
      yearMin: customYearMin.value || undefined,
      yearMax: customYearMax.value || undefined,
      genres: selectedGenres.value.length > 0 ? selectedGenres.value.join(',') : undefined,
      tags: selectedTags.value.length > 0 ? selectedTags.value.join(',') : undefined,
      includeAdult: includeAdult.value ? 'true' : undefined
    }
  })

  loadInitialAnime()
}

onMounted(async () => {
  document.title = 'Discover Anime - Anigraph'

  const searchQuery = route.query.q as string
  let observer: IntersectionObserver | null = null

  // Register cleanup FIRST, before any await
  onBeforeUnmount(() => {
    if (observer) {
      observer.disconnect()
    }
    window.removeEventListener('resize', updateMobileStatus)
  })

  // Fetch available genres/tags for filtering
  await fetchFilters()

  // Initialize sort, type, and format from URL
  if (route.query.sort) {
    currentSort.value = route.query.sort as string
  }
  if (route.query.order) {
    sortOrder.value = route.query.order as 'asc' | 'desc'
  }
  if (route.query.type) {
    currentType.value = route.query.type as string
  }
  if (route.query.format) {
    currentFormat.value = route.query.format as string
  }

  // Restore filters from URL query parameters
  if (route.query.era) {
    selectedEra.value = route.query.era as string
  }
  if (route.query.yearMin) {
    customYearMin.value = parseInt(route.query.yearMin as string)
  }
  if (route.query.yearMax) {
    customYearMax.value = parseInt(route.query.yearMax as string)
  }
  if (route.query.genres) {
    selectedGenres.value = (route.query.genres as string).split(',')
  }
  if (route.query.tags) {
    selectedTags.value = (route.query.tags as string).split(',')
  }

  if (searchQuery) {
    await handleSearch(searchQuery)
  } else {
    await loadInitialAnime()
  }

  await nextTick()

  // Now set up the observer
  if (sentinel.value) {
    observer = new IntersectionObserver(
      ([entry]) => {
        // Only trigger if not in search mode, not initially loading, and has more results
        if (entry.isIntersecting && !isSearchMode.value && !initialLoading.value && hasMoreResults.value) {
          loadMore()
        }
      },
      { rootMargin: '400px' }
    )

    observer.observe(sentinel.value)
  }
})

// Watch for era changes and clear custom year range if switching away from custom
watch(selectedEra, (newEra) => {
  if (newEra !== 'custom') {
    customYearMin.value = null
    customYearMax.value = null
  }
})

// Watch for route query changes to update filters when navigating between filtered views
watch(() => route.query, async (newQuery, oldQuery) => {
  // Only react if query actually changed (avoid infinite loops)
  if (JSON.stringify(newQuery) === JSON.stringify(oldQuery)) return

  // Update filters from new query parameters
  currentSort.value = (newQuery.sort as string) || 'random'
  sortOrder.value = (newQuery.order as 'asc' | 'desc') || 'desc'
  currentType.value = (newQuery.type as string) || null
  currentFormat.value = (newQuery.format as string) || null
  selectedEra.value = (newQuery.era as string) || null
  customYearMin.value = newQuery.yearMin ? parseInt(newQuery.yearMin as string) : null
  customYearMax.value = newQuery.yearMax ? parseInt(newQuery.yearMax as string) : null
  selectedGenres.value = newQuery.genres ? (newQuery.genres as string).split(',') : []
  selectedTags.value = newQuery.tags ? (newQuery.tags as string).split(',') : []

  // Handle search vs browse mode
  const searchQuery = newQuery.q as string
  if (searchQuery) {
    await handleSearch(searchQuery)
  } else {
    // Reset to browse mode
    isSearchMode.value = false
    animeList.value = []
    currentOffset.value = 0
    hasMoreResults.value = true
    await loadInitialAnime()
  }
}, { deep: true })
</script>

<style scoped>
.masonry-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px; /* Top padding for app-bar */
  padding-bottom: 120px; /* Bottom for search bar */
}

.page-title-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.page-title-gradient {
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
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

@media (max-width: 600px) {
  .masonry-page {
    padding-top: 64px; /* Adjusted top padding for mobile app-bar */
    padding-bottom: 100px;
  }
}

.toolbar-section {
  position: relative;
  z-index: 1;
}

.grid-section {
  position: relative;
  z-index: 2;
  overflow: visible;
}

.grid-section :deep(.vue-recycle-scroller),
.grid-section :deep(.vue-recycle-scroller__item-wrapper),
.grid-section :deep(.vue-recycle-scroller__item-view) {
  overflow: visible !important;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 80vh;
  color: var(--color-text);
}

.empty-state-inline {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  color: var(--color-text);
  max-width: 1200px;
  margin: 0 auto;
  background-color: rgba(var(--color-surface-rgb), 0.3);
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-primary-border);
}

.loading-more {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 32px;
  width: 100%;
}

/* Glow for search bar on homepage */
.masonry-page :deep(.floating-search-input .v-field) {
  box-shadow: 0 0 20px rgba(var(--color-primary-rgb), 0.5), 0 0 40px rgba(var(--color-primary-rgb), 0.3), 0 8px 32px rgba(0, 0, 0, 0.4) !important;
  border: 1px solid rgba(var(--color-primary-rgb), 0.4);
  transition: box-shadow 0.3s ease, border-color 0.3s ease;
}

/* Enhanced glow on hover and focus */
.masonry-page :deep(.floating-search-input .v-field:hover),
.masonry-page :deep(.floating-search-input .v-field--focused) {
  box-shadow: 0 0 25px rgba(var(--color-primary-rgb), 0.7), 0 0 50px rgba(var(--color-primary-rgb), 0.4), 0 8px 32px rgba(0, 0, 0, 0.4) !important;
  border: 1px solid rgba(var(--color-primary-rgb), 0.6);
}

</style>
