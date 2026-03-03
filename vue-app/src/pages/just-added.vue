<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container fluid>
        <!-- Toolbar: Title + Type Filter + Card Size -->
        <ViewToolbar
          v-model:card-size="cardSize"
          :show-sort="false"
          :show-year-markers="false"
        >
          <template #left>
            <div class="d-flex align-center" style="gap: 16px;">
              <div class="page-title-section">
                <div class="d-flex align-center">
                  <v-icon color="primary" size="28" class="mr-2">mdi-clock-plus-outline</v-icon>
                  <h1 class="text-h5 font-weight-bold mb-0">Just Added</h1>
                </div>
                <p class="text-caption text-medium-emphasis ml-9 mb-0">
                  Newest entries added to AniList
                </p>
              </div>
              <v-divider vertical class="mx-2" style="height: 40px;"></v-divider>
              <v-select
                v-model="selectedType"
                :items="typeOptions"
                label="Type"
                variant="outlined"
                density="compact"
                hide-details
                style="width: 100px;"
                @update:model-value="applyFilters"
              ></v-select>
            </div>
          </template>
        </ViewToolbar>

        <!-- Loading State -->
        <v-row v-if="loading" class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-progress-circular
              indeterminate
              color="primary"
              size="64"
            ></v-progress-circular>
            <p class="text-h6 mt-4">Loading just added...</p>
          </v-col>
        </v-row>

        <!-- Results Grid -->
        <v-row v-else-if="animeList.length > 0" class="mt-n2">
          <v-col
            v-for="anime in animeList"
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
        <v-row v-else class="mt-n2">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-filter-remove-outline</v-icon>
            <p class="text-h6 mt-4">No results found</p>
            <p class="text-body-1 text-medium-emphasis">
              Try adjusting your filters
            </p>
          </v-col>
        </v-row>

        <!-- Infinite Scroll Sentinel -->
        <div ref="sentinel" class="loading-more">
          <v-progress-circular
            v-if="loadingMore"
            indeterminate
            color="primary"
            size="48"
          ></v-progress-circular>
        </div>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCardSize } from '~/composables/useCardSize'
import { useSettings } from '~/composables/useSettings'
import { api } from '~/utils/api'

const loading = ref(true)
const loadingMore = ref(false)
const animeList = ref<any[]>([])
const hasMore = ref(true)
const offset = ref(0)
const limit = 24
const sentinel = ref<HTMLElement | null>(null)

// Composables
const { cardSize, cardColSize } = useCardSize()
const { includeAdult } = useSettings()
const route = useRoute()
const router = useRouter()

// Initialize filters from URL query params
const selectedType = ref((route.query.type as string) || 'all')

const typeOptions = [
  { title: 'All', value: 'all' },
  { title: 'Anime', value: 'anime' },
  { title: 'Manga', value: 'manga' }
]

const fetchNewProductions = async (append = false) => {
  if (append) {
    loadingMore.value = true
  } else {
    loading.value = true
    offset.value = 0
    animeList.value = []
  }

  try {
    const params: any = {
      sort: 'newest-id',
      limit,
      offset: offset.value
    }

    // Apply type filter
    if (selectedType.value !== 'all') {
      params.type = selectedType.value
    }

    // Apply adult content filter
    params.includeAdult = includeAdult.value

    const response = await api<any>('/anime/popular', { params })

    if (response.success) {
      const results = response.data

      if (append) {
        animeList.value = [...animeList.value, ...results]
      } else {
        animeList.value = results
      }

      // Check if there are more results
      hasMore.value = results.length >= limit
    }
  } catch (error) {
    console.error('Error fetching just added:', error)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const updateUrlQuery = () => {
  const query: Record<string, string> = {}
  if (selectedType.value !== 'all') query.type = selectedType.value
  router.replace({ query })
}

const applyFilters = () => {
  updateUrlQuery()
  fetchNewProductions(false)
}

const loadMore = () => {
  if (!hasMore.value || loadingMore.value) return
  offset.value += limit
  fetchNewProductions(true)
}

onMounted(async () => {
  let observer: IntersectionObserver | null = null

  // Register cleanup FIRST
  onBeforeUnmount(() => {
    if (observer) {
      observer.disconnect()
    }
  })

  // Initial fetch
  await fetchNewProductions()

  await nextTick()

  // Set up infinite scroll observer
  if (sentinel.value) {
    observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !loading.value && hasMore.value) {
          loadMore()
        }
      },
      { rootMargin: '400px' }
    )

    observer.observe(sentinel.value)
  }
})

document.title = 'Just Added - Anigraph'
</script>

<style scoped>
.loading-more {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 32px;
  width: 100%;
  min-height: 100px;
}

.page-title-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
</style>
