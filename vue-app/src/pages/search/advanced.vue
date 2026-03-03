<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container fluid>
        <!-- Toolbar: Title + Filters + Card Size -->
        <ViewToolbar
          v-model:card-size="cardSize"
          :show-sort="false"
          :show-year-markers="false"
        >
          <template #left>
            <div class="d-flex align-center flex-wrap" style="gap: 12px;">
              <div class="page-title-section">
                <div class="d-flex align-center">
                  <v-icon color="primary" size="28" class="mr-2">mdi-magnify</v-icon>
                  <h1 class="text-h5 font-weight-bold mb-0">Advanced Search</h1>
                </div>
                <p v-if="!loading && searchResults.length > 0" class="text-caption text-medium-emphasis ml-9 mb-0">
                  Found {{ totalResults }} results{{ hasActiveFilters ? ' matching your filters' : '' }}
                </p>
                <p v-else class="text-caption text-medium-emphasis ml-9 mb-0">
                  Use filters to search anime and manga
                </p>
              </div>
              <v-divider vertical class="mx-2" style="height: 40px;"></v-divider>

              <!-- Text Search -->
              <v-text-field
                v-model="filters.textQuery"
                label="Search by title"
                prepend-inner-icon="mdi-magnify"
                variant="outlined"
                density="compact"
                clearable
                hide-details
                style="width: 200px;"
                @update:model-value="debouncedSearch"
              ></v-text-field>

              <!-- Format Filter -->
              <v-select
                v-model="filters.format"
                :items="formatOptions"
                label="Format"
                variant="outlined"
                density="compact"
                clearable
                hide-details
                style="width: 130px;"
                @update:model-value="performSearch"
              ></v-select>

              <!-- Season Filter -->
              <v-select
                v-model="filters.season"
                :items="seasonOptions"
                label="Season"
                variant="outlined"
                density="compact"
                clearable
                hide-details
                style="width: 120px;"
                @update:model-value="performSearch"
              ></v-select>

              <!-- Genres -->
              <v-autocomplete
                v-model="filters.genres"
                :items="availableGenres"
                label="Genres"
                variant="outlined"
                density="compact"
                multiple
                chips
                closable-chips
                clearable
                hide-details
                style="width: 200px;"
                @update:model-value="performSearch"
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
                v-model="filters.tags"
                :items="availableTags"
                label="Tags"
                variant="outlined"
                density="compact"
                multiple
                chips
                closable-chips
                clearable
                hide-details
                style="width: 200px;"
                @update:model-value="performSearch"
              >
                <template #chip="{ props, item }">
                  <v-chip
                    v-bind="props"
                    :text="item.title"
                    size="small"
                  ></v-chip>
                </template>
              </v-autocomplete>

              <!-- More Filters Button -->
              <v-btn
                variant="outlined"
                color="primary"
                density="compact"
                @click="showMoreFilters = !showMoreFilters"
              >
                <v-icon start>{{ showMoreFilters ? 'mdi-chevron-up' : 'mdi-filter-plus' }}</v-icon>
                More Filters
              </v-btn>

              <!-- Clear Filters Button -->
              <v-btn
                v-if="hasActiveFilters"
                variant="text"
                color="error"
                density="compact"
                @click="clearFilters"
              >
                <v-icon start>mdi-filter-remove</v-icon>
                Clear All
              </v-btn>
            </div>
          </template>
        </ViewToolbar>

        <!-- Expandable More Filters Panel -->
        <v-expand-transition>
          <v-card v-show="showMoreFilters" class="mb-4 mt-2">
            <v-card-text>
              <v-row dense>
                <!-- Year Range -->
                <v-col cols="12" sm="6" md="3">
                  <div class="mb-2">
                    <div class="text-subtitle-2 mb-2">
                      Year Range
                      <span class="text-caption text-medium-emphasis ml-2">
                        {{ yearRange[0] }} - {{ yearRange[1] }}
                      </span>
                    </div>
                    <v-range-slider
                      v-model="yearRange"
                      :min="1940"
                      :max="2030"
                      :step="1"
                      color="primary"
                      thumb-label
                      hide-details
                      @update:model-value="debouncedYearChange"
                    ></v-range-slider>
                  </div>
                </v-col>

                <!-- Score Range -->
                <v-col cols="12" sm="6" md="3">
                  <div class="mb-2">
                    <div class="text-subtitle-2 mb-2">
                      Score Range
                      <span class="text-caption text-medium-emphasis ml-2">
                        {{ scoreRange[0] }} - {{ scoreRange[1] }}
                      </span>
                    </div>
                    <v-range-slider
                      v-model="scoreRange"
                      :min="0"
                      :max="100"
                      :step="1"
                      color="primary"
                      thumb-label
                      hide-details
                      @update:model-value="debouncedScoreChange"
                    ></v-range-slider>
                  </div>
                </v-col>

                <!-- Episode Count Range -->
                <v-col cols="12" sm="6" md="3">
                  <div class="text-subtitle-2 mb-2">Episode Count</div>
                  <div class="d-flex align-center" style="gap: 8px;">
                    <v-text-field
                      v-model.number="filters.episodesMin"
                      label="Min"
                      type="number"
                      variant="outlined"
                      density="compact"
                      hide-details
                      :min="1"
                      @update:model-value="performSearch"
                    ></v-text-field>
                    <v-text-field
                      v-model.number="filters.episodesMax"
                      label="Max"
                      type="number"
                      variant="outlined"
                      density="compact"
                      hide-details
                      :min="1"
                      @update:model-value="performSearch"
                    ></v-text-field>
                  </div>
                </v-col>

                <!-- Sort -->
                <v-col cols="12" sm="6" md="3">
                  <div class="text-subtitle-2 mb-2">Sort by</div>
                  <div class="d-flex align-center" style="gap: 8px;">
                    <v-select
                      v-model="filters.sort"
                      :items="sortOptions"
                      variant="outlined"
                      density="compact"
                      hide-details
                      class="flex-grow-1"
                      @update:model-value="performSearch"
                    ></v-select>
                    <v-btn
                      icon
                      color="primary"
                      variant="tonal"
                      density="compact"
                      @click="toggleSortOrder"
                      :title="sortOrder === 'desc' ? 'Descending' : 'Ascending'"
                    >
                      <v-icon>{{ sortOrder === 'desc' ? 'mdi-arrow-down' : 'mdi-arrow-up' }}</v-icon>
                    </v-btn>
                  </div>
                </v-col>
              </v-row>
            </v-card-text>
          </v-card>
        </v-expand-transition>

        <!-- Loading State -->
        <v-row v-if="loading" class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-progress-circular
              indeterminate
              color="primary"
              size="64"
            ></v-progress-circular>
            <p class="text-h6 mt-4">Searching...</p>
          </v-col>
        </v-row>

        <!-- Results Grid -->
        <v-row v-else-if="searchResults.length > 0" class="mt-n2">
          <v-col
            v-for="anime in searchResults"
            :key="anime.id"
            cols="12"
            sm="6"
            md="4"
            :lg="cardColSize"
          >
            <AnimeCard :anime="anime" />
          </v-col>
        </v-row>

        <!-- No Results -->
        <v-row v-else-if="hasActiveFilters" class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-filter-remove-outline</v-icon>
            <p class="text-h6 mt-4">No results found</p>
            <p class="text-body-1 text-medium-emphasis mb-4">
              Try adjusting your filters
            </p>
            <v-btn color="primary" @click="clearFilters">
              Clear All Filters
            </v-btn>
          </v-col>
        </v-row>

        <!-- Initial State -->
        <v-row v-else class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="primary">mdi-database-search</v-icon>
            <p class="text-h6 mt-4">Search the Complete Database</p>
            <p class="text-body-1 text-medium-emphasis">
              Apply filters above to discover anime and manga tailored to your preferences
            </p>
          </v-col>
        </v-row>

        <!-- Pagination -->
        <v-row v-if="totalPages > 1">
          <v-col cols="12" class="d-flex justify-center py-6">
            <v-pagination
              v-model="currentPage"
              :length="totalPages"
              :total-visible="7"
              @update:model-value="onPageChange"
            ></v-pagination>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/utils/api'
import { useSettings } from '@/composables/useSettings'
import { useAnalytics } from '@/composables/useAnalytics'
import { useCardSize } from '@/composables/useCardSize'

const route = useRoute()
const router = useRouter()
const { includeAdult } = useSettings()
const { trackSearch, trackFilterApply } = useAnalytics()

// Card size control
const { cardSize, cardColSize } = useCardSize('small')

const loading = ref(false)
const searchResults = ref<any[]>([])
const totalResults = ref(0)
const currentPage = ref(1)
const totalPages = ref(0)
const showMoreFilters = ref(false)

const filters = ref({
  textQuery: '',
  format: null as string | null,
  season: null as string | null,
  episodesMin: null as number | null,
  episodesMax: null as number | null,
  genres: [] as string[],
  tags: [] as string[],
  sort: 'score'
})

const yearRange = ref([1940, 2030])
const scoreRange = ref([0, 100])

const formatOptions = [
  { title: 'TV', value: 'TV' },
  { title: 'Movie', value: 'MOVIE' },
  { title: 'OVA', value: 'OVA' },
  { title: 'ONA', value: 'ONA' },
  { title: 'Special', value: 'SPECIAL' },
  { title: 'Manga', value: 'MANGA' },
  { title: 'Novel', value: 'NOVEL' },
  { title: 'One Shot', value: 'ONE_SHOT' }
]

const seasonOptions = [
  { title: 'Winter', value: 'WINTER' },
  { title: 'Spring', value: 'SPRING' },
  { title: 'Summer', value: 'SUMMER' },
  { title: 'Fall', value: 'FALL' }
]

const sortOptions = [
  { title: 'Score', value: 'score' },
  { title: 'Year', value: 'year' },
  { title: 'Title', value: 'title' }
]

const sortOrder = ref<'asc' | 'desc'>('desc')

// Load genres and tags
const availableGenres = ref<string[]>([])
const availableTags = ref<string[]>([])

const hasActiveFilters = computed(() => {
  return filters.value.textQuery ||
    filters.value.format ||
    (yearRange.value[0] !== 1940 || yearRange.value[1] !== 2030) ||
    (scoreRange.value[0] !== 0 || scoreRange.value[1] !== 100) ||
    filters.value.season ||
    filters.value.episodesMin ||
    filters.value.episodesMax ||
    filters.value.genres.length > 0 ||
    filters.value.tags.length > 0
})

// Initialize filters from URL query params
const initializeFromURL = () => {
  if (route.query.q) filters.value.textQuery = route.query.q as string
  if (route.query.format) filters.value.format = route.query.format as string
  if (route.query.season) filters.value.season = route.query.season as string
  if (route.query.episodesMin) filters.value.episodesMin = parseInt(route.query.episodesMin as string)
  if (route.query.episodesMax) filters.value.episodesMax = parseInt(route.query.episodesMax as string)
  if (route.query.sort) filters.value.sort = route.query.sort as string
  if (route.query.order) sortOrder.value = route.query.order as 'asc' | 'desc'
  if (route.query.page) currentPage.value = parseInt(route.query.page as string)

  if (route.query.yearMin || route.query.yearMax) {
    yearRange.value = [
      route.query.yearMin ? parseInt(route.query.yearMin as string) : 1940,
      route.query.yearMax ? parseInt(route.query.yearMax as string) : 2030
    ]
  }

  if (route.query.scoreMin || route.query.scoreMax) {
    scoreRange.value = [
      route.query.scoreMin ? parseInt(route.query.scoreMin as string) : 0,
      route.query.scoreMax ? parseInt(route.query.scoreMax as string) : 100
    ]
  }

  if (route.query.genres) {
    filters.value.genres = (route.query.genres as string).split(',').filter(Boolean)
  }

  if (route.query.tags) {
    filters.value.tags = (route.query.tags as string).split(',').filter(Boolean)
  }
}

// Update URL with current filters
const updateURL = () => {
  const query: any = {}

  if (filters.value.textQuery) query.q = filters.value.textQuery
  if (filters.value.format) query.format = filters.value.format
  if (filters.value.season) query.season = filters.value.season
  if (filters.value.episodesMin) query.episodesMin = filters.value.episodesMin.toString()
  if (filters.value.episodesMax) query.episodesMax = filters.value.episodesMax.toString()
  if (filters.value.sort !== 'score') query.sort = filters.value.sort
  if (sortOrder.value !== 'desc') query.order = sortOrder.value
  if (currentPage.value > 1) query.page = currentPage.value.toString()

  if (yearRange.value[0] !== 1940) query.yearMin = yearRange.value[0].toString()
  if (yearRange.value[1] !== 2030) query.yearMax = yearRange.value[1].toString()

  if (scoreRange.value[0] !== 0) query.scoreMin = scoreRange.value[0].toString()
  if (scoreRange.value[1] !== 100) query.scoreMax = scoreRange.value[1].toString()

  if (filters.value.genres.length > 0) query.genres = filters.value.genres.join(',')
  if (filters.value.tags.length > 0) query.tags = filters.value.tags.join(',')

  router.replace({ query })
}

const loadGenresAndTags = async () => {
  try {
    const response = await api<any>('/anime/genres-tags', { params: { includeAdult: String(includeAdult.value) } })
    if (response.success) {
      availableGenres.value = response.genres || []
      availableTags.value = response.tags || []
    }
  } catch (error) {
    console.error('Error loading genres and tags:', error)
  }
}

const performSearchWithoutURLUpdate = async (resetPage = true) => {
  if (!hasActiveFilters.value) {
    searchResults.value = []
    totalResults.value = 0
    totalPages.value = 0
    return
  }

  loading.value = true
  if (resetPage) currentPage.value = 1

  try {
    // Invert order for title sort so descending (down arrow) shows A-Z
    const effectiveOrder = filters.value.sort === 'title'
      ? (sortOrder.value === 'desc' ? 'asc' : 'desc')
      : sortOrder.value

    const params: Record<string, string> = {
      page: String(currentPage.value),
      limit: '18',
      sort: filters.value.sort,
      order: effectiveOrder,
      includeAdult: String(includeAdult.value)
    }

    if (filters.value.textQuery) params.q = filters.value.textQuery
    if (filters.value.format) params.format = filters.value.format
    if (yearRange.value[0] !== 1940) params.yearMin = String(yearRange.value[0])
    if (yearRange.value[1] !== 2030) params.yearMax = String(yearRange.value[1])
    if (scoreRange.value[0] !== 0) params.scoreMin = String(scoreRange.value[0])
    if (scoreRange.value[1] !== 100) params.scoreMax = String(scoreRange.value[1])
    if (filters.value.season) params.season = filters.value.season
    if (filters.value.episodesMin) params.episodesMin = String(filters.value.episodesMin)
    if (filters.value.episodesMax) params.episodesMax = String(filters.value.episodesMax)
    if (filters.value.genres.length > 0) params.genres = filters.value.genres.join(',')
    if (filters.value.tags.length > 0) params.tags = filters.value.tags.join(',')

    const response = await api<any>('/anime/advanced-search', { params })

    if (response.success) {
      searchResults.value = response.data.map((anime: any) => ({
        id: anime.anilistId,
        anilistId: anime.anilistId,
        title: anime.title_english || anime.title,
        titleEnglish: anime.title_english,
        titleRomaji: anime.title_romaji,
        description: anime.description,
        coverImage: anime.coverImage,
        coverImage_extraLarge: anime.coverImage_extraLarge,
        coverImage_large: anime.coverImage_large,
        coverImage_medium: anime.coverImage_medium,
        format: anime.format,
        seasonYear: anime.seasonYear,
        season: anime.season,
        averageScore: anime.averageScore,
        episodes: anime.episodes,
        studios: anime.studios,
        genres: anime.genres
      }))
      totalResults.value = response.total || 0
      totalPages.value = response.totalPages || 0
    }
  } catch (error) {
    console.error('Search error:', error)
    searchResults.value = []
  } finally {
    loading.value = false
  }
}

const performSearch = async () => {
  if (!hasActiveFilters.value) {
    router.push({ query: {} })
    searchResults.value = []
    totalResults.value = 0
    totalPages.value = 0
    return
  }

  // Track search/filter usage
  if (filters.value.textQuery) {
    trackSearch(filters.value.textQuery, 'advanced')
  }
  if (filters.value.genres.length > 0) {
    trackFilterApply('genres', filters.value.genres, 'advanced')
  }
  if (filters.value.tags.length > 0) {
    trackFilterApply('tags', filters.value.tags, 'advanced')
  }
  if (filters.value.format) {
    trackFilterApply('format', filters.value.format, 'advanced')
  }

  currentPage.value = 1
  updateURL()
  await performSearchWithoutURLUpdate(false)
}

const toggleSortOrder = () => {
  sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  currentPage.value = 1
  updateURL()
  performSearchWithoutURLUpdate(false)
}

let searchTimeout: ReturnType<typeof setTimeout> | null = null
const debouncedSearch = () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    performSearch()
  }, 500)
}

let yearTimeout: ReturnType<typeof setTimeout> | null = null
const debouncedYearChange = () => {
  if (yearTimeout) clearTimeout(yearTimeout)
  yearTimeout = setTimeout(() => {
    performSearch()
  }, 500)
}

let scoreTimeout: ReturnType<typeof setTimeout> | null = null
const debouncedScoreChange = () => {
  if (scoreTimeout) clearTimeout(scoreTimeout)
  scoreTimeout = setTimeout(() => {
    performSearch()
  }, 500)
}

const onPageChange = async (page: number) => {
  if (!hasActiveFilters.value) return

  currentPage.value = page
  updateURL()
  window.scrollTo({ top: 0, behavior: 'smooth' })
  await performSearchWithoutURLUpdate(false)
}

const clearFilters = () => {
  filters.value = {
    textQuery: '',
    format: null,
    season: null,
    episodesMin: null,
    episodesMax: null,
    genres: [],
    tags: [],
    sort: 'score'
  }
  yearRange.value = [1940, 2030]
  scoreRange.value = [0, 100]
  searchResults.value = []
  totalResults.value = 0
  totalPages.value = 0
  currentPage.value = 1
  router.push({ query: {} })
}

// Watch for route query changes (browser back/forward)
watch(() => route.query, async () => {
  initializeFromURL()
  if (hasActiveFilters.value) {
    await performSearchWithoutURLUpdate(false)
  } else {
    searchResults.value = []
    totalResults.value = 0
    totalPages.value = 0
  }
})

onMounted(() => {
  loadGenresAndTags()
  initializeFromURL()
  if (hasActiveFilters.value) {
    performSearchWithoutURLUpdate(false)
  }
})
</script>

<style scoped>
.page-title-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
</style>
