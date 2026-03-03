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
            <div class="d-flex align-center flex-wrap" style="gap: 8px;">
              <div class="page-title-section">
                <div class="d-flex align-center">
                  <v-icon color="primary" size="28" class="mr-2">mdi-trophy</v-icon>
                  <h1 class="text-h5 font-weight-bold mb-0">Top Rated</h1>
                </div>
                <p class="text-caption text-medium-emphasis ml-9 mb-0">
                  Highest rated anime and manga of all time
                </p>
              </div>
              <v-divider vertical class="mx-2 d-none d-sm-flex" style="height: 40px;"></v-divider>

              <!-- Type Filter -->
              <v-select
                v-model="selectedType"
                :items="typeOptions"
                label="Type"
                variant="outlined"
                density="compact"
                hide-details
                style="min-width: 100px; max-width: 100px;"
                @update:model-value="applyFilters"
              ></v-select>

              <!-- Format Filter -->
              <v-select
                v-model="selectedFormat"
                :items="formatOptions"
                label="Format"
                clearable
                variant="outlined"
                density="compact"
                hide-details
                style="min-width: 115px; max-width: 150px;"
                @update:model-value="applyFilters"
              ></v-select>

              <!-- Era Filter -->
              <v-select
                v-model="selectedEra"
                :items="eraOptions"
                label="Era"
                clearable
                variant="outlined"
                density="compact"
                hide-details
                style="min-width: 145px; max-width: 180px;"
                @update:model-value="applyFilters"
              ></v-select>

              <!-- Genres -->
              <v-autocomplete
                v-model="selectedGenres"
                :items="availableGenres"
                label="Genres"
                variant="outlined"
                density="compact"
                multiple
                chips
                closable-chips
                clearable
                hide-details
                style="min-width: 180px; max-width: 250px;"
                @update:model-value="applyFilters"
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
                label="Tags"
                variant="outlined"
                density="compact"
                multiple
                chips
                closable-chips
                clearable
                hide-details
                style="min-width: 180px; max-width: 250px;"
                @update:model-value="applyFilters"
              >
                <template #chip="{ props, item }">
                  <v-chip
                    v-bind="props"
                    :text="item.title"
                    size="small"
                  ></v-chip>
                </template>
              </v-autocomplete>

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
                  style="min-width: 110px; max-width: 120px;"
                  @update:model-value="applyFilters"
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
                  style="min-width: 110px; max-width: 120px;"
                  @update:model-value="applyFilters"
                ></v-text-field>
              </template>
            </div>
          </template>
        </ViewToolbar>

        <!-- Loading State -->
        <v-row v-if="loading" class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-progress-circular indeterminate color="primary" size="64"></v-progress-circular>
            <p class="text-h6 mt-4">Loading top rated...</p>
          </v-col>
        </v-row>

        <!-- Results Grid (Virtual Scroll) -->
        <template v-else-if="animeList.length > 0">
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
        </template>

        <!-- No Results -->
        <v-row v-else class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-filter-remove-outline</v-icon>
            <p class="text-h6 mt-4">No results found</p>
            <p class="text-body-1 text-medium-emphasis">Try adjusting your filters</p>
          </v-col>
        </v-row>
      </v-container>

      <!-- Infinite Scroll Sentinel -->
      <div ref="sentinel" class="loading-more">
        <v-progress-circular
          v-if="loadingMore"
          indeterminate
          color="primary"
          size="48"
        ></v-progress-circular>
      </div>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCardSize } from '@/composables/useCardSize'
import { useSettings } from '@/composables/useSettings'
import { useVirtualGrid } from '@/composables/useVirtualGrid'
import { api } from '@/utils/api'

// Composables
const { cardSize, cardColSize } = useCardSize('small')
const { includeAdult } = useSettings()
const route = useRoute()
const router = useRouter()

const sentinel = ref<HTMLElement | null>(null)
const loading = ref(true)
const loadingMore = ref(false)
const animeList = ref<any[]>([])
const hasMore = ref(true)
const offset = ref(0)
const limit = 24

// Virtual grid rows
const { rows: animeRows } = useVirtualGrid(animeList, cardColSize)

// Initialize filters from URL query params
const selectedType = ref((route.query.type as string) || 'all')
const selectedFormat = ref<string | null>((route.query.format as string) || null)
const selectedEra = ref<string | null>((route.query.era as string) || null)
const customYearMin = ref<number | null>(route.query.yearMin ? Number(route.query.yearMin) : null)
const customYearMax = ref<number | null>(route.query.yearMax ? Number(route.query.yearMax) : null)
const selectedGenres = ref<string[]>(route.query.genres ? (route.query.genres as string).split(',') : [])
const selectedTags = ref<string[]>(route.query.tags ? (route.query.tags as string).split(',') : [])

// Available options for genres and tags
const availableGenres = ref<string[]>([])
const availableTags = ref<string[]>([])

const typeOptions = [
  { title: 'Anime', value: 'anime' },
  { title: 'Manga', value: 'manga' },
  { title: 'All', value: 'all' }
]

const animeFormatOptions = [
  { title: 'TV Series', value: 'TV' },
  { title: 'Movie', value: 'MOVIE' },
  { title: 'OVA', value: 'OVA' },
  { title: 'ONA', value: 'ONA' },
  { title: 'Special', value: 'SPECIAL' },
  { title: 'TV Short', value: 'TV_SHORT' },
  { title: 'Music', value: 'MUSIC' }
]

const mangaFormatOptions = [
  { title: 'Manga', value: 'MANGA' },
  { title: 'Novel', value: 'NOVEL' },
  { title: 'One Shot', value: 'ONE_SHOT' }
]

const formatOptions = computed(() => {
  if (selectedType.value === 'anime') {
    return animeFormatOptions
  } else if (selectedType.value === 'manga') {
    return mangaFormatOptions
  } else {
    return [...animeFormatOptions, ...mangaFormatOptions]
  }
})

const eraOptions = [
  { title: 'Pre-1960s', value: 'pre-1960' },
  { title: '1960s-1980s', value: '1960s-1980s' },
  { title: '1990s-2000s', value: '1990s-2000s' },
  { title: '2010s', value: '2010s' },
  { title: '2020s', value: '2020s' },
  { title: 'Custom Range', value: 'custom' }
]

const loadGenresAndTags = async () => {
  try {
    const response = await api('/anime/genres-tags', { params: { includeAdult: includeAdult.value } })
    if (response.success) {
      availableGenres.value = response.genres || []
      availableTags.value = response.tags || []
    }
  } catch (error) {
    console.error('Error loading genres and tags:', error)
  }
}

const fetchTopRated = async (append = false) => {
  if (append) {
    loadingMore.value = true
  } else {
    loading.value = true
    offset.value = 0
    animeList.value = []
  }

  try {
    const params: any = {
      sort: 'top',
      limit,
      offset: offset.value
    }

    if (selectedType.value !== 'all') params.type = selectedType.value
    if (selectedFormat.value) params.format = selectedFormat.value
    if (selectedEra.value) {
      if (selectedEra.value === 'custom') {
        if (customYearMin.value) params.yearMin = customYearMin.value
        if (customYearMax.value) params.yearMax = customYearMax.value
      } else {
        params.eras = selectedEra.value
      }
    }
    if (selectedGenres.value.length > 0) params.genres = selectedGenres.value.join(',')
    if (selectedTags.value.length > 0) params.tags = selectedTags.value.join(',')
    params.includeAdult = includeAdult.value

    const response = await api('/anime/popular', { params })

    if (response.success) {
      const results = response.data
      if (append) {
        animeList.value = [...animeList.value, ...results]
      } else {
        animeList.value = results
      }
      hasMore.value = results.length >= limit
    }
  } catch (error) {
    console.error('Error fetching top rated:', error)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const updateUrlQuery = () => {
  const query: Record<string, string> = {}
  if (selectedType.value !== 'all') query.type = selectedType.value
  if (selectedFormat.value) query.format = selectedFormat.value
  if (selectedEra.value) query.era = selectedEra.value
  if (selectedEra.value === 'custom') {
    if (customYearMin.value) query.yearMin = String(customYearMin.value)
    if (customYearMax.value) query.yearMax = String(customYearMax.value)
  }
  if (selectedGenres.value.length > 0) query.genres = selectedGenres.value.join(',')
  if (selectedTags.value.length > 0) query.tags = selectedTags.value.join(',')
  router.replace({ query })
}

const applyFilters = () => {
  updateUrlQuery()
  fetchTopRated(false)
}

const loadMore = () => {
  offset.value += limit
  fetchTopRated(true)
}

// Watch for type changes and clear format if not available
watch(selectedType, () => {
  if (selectedFormat.value) {
    const availableFormatValues = formatOptions.value.map(opt => opt.value)
    if (!availableFormatValues.includes(selectedFormat.value)) {
      selectedFormat.value = null
    }
  }
})

// Watch for era changes and clear custom year range if switching away from custom
watch(selectedEra, (newEra) => {
  if (newEra !== 'custom') {
    customYearMin.value = null
    customYearMax.value = null
  }
})

onMounted(async () => {
  let observer: IntersectionObserver | null = null

  onBeforeUnmount(() => {
    if (observer) observer.disconnect()
  })

  loadGenresAndTags()
  await fetchTopRated()
  await nextTick()

  if (sentinel.value) {
    observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !loading.value && !loadingMore.value && hasMore.value) {
          loadMore()
        }
      },
      { rootMargin: '400px' }
    )
    observer.observe(sentinel.value)
  }
})
</script>

<style scoped>
.page-title-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.loading-more {
  display: flex;
  justify-content: center;
  padding: 24px 0;
  min-height: 48px;
}
</style>
