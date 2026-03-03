<template>
  <v-app>
    <AppBar title="Advanced Search" />

    <v-main>
      <v-container>
        <!-- Active Filters -->
        <v-row class="mt-4 mb-2" v-if="hasActiveFilters">
          <v-col cols="12">
            <h3 class="text-h6 mb-3">Active Filters</h3>
            <div class="d-flex flex-wrap gap-2">
              <!-- Studio Chips -->
              <v-chip
                v-for="studio in activeStudios"
                :key="`studio-${studio}`"
                closable
                @click:close="removeStudio(studio)"
                color="primary"
                variant="flat"
              >
                <v-icon start>mdi-domain</v-icon>
                {{ studio }}
              </v-chip>

              <!-- Genre Chips -->
              <v-chip
                v-for="genre in activeGenres"
                :key="`genre-${genre}`"
                closable
                @click:close="removeGenre(genre)"
                color="secondary"
                variant="flat"
              >
                <v-icon start>mdi-tag</v-icon>
                {{ genre }}
              </v-chip>

              <!-- Tag Chips -->
              <v-chip
                v-for="tag in activeTags"
                :key="`tag-${tag}`"
                closable
                @click:close="removeTag(tag)"
                color="accent"
                variant="flat"
              >
                <v-icon start>mdi-label</v-icon>
                {{ tag }}
              </v-chip>

              <!-- Clear All Button -->
              <v-btn
                variant="outlined"
                size="small"
                @click="clearAllFilters"
                class="ml-2"
              >
                Clear All
              </v-btn>
            </div>
          </v-col>
        </v-row>

        <v-divider v-if="hasActiveFilters" class="my-4"></v-divider>

        <!-- Results Header -->
        <v-row v-if="hasActiveFilters && !loading">
          <v-col cols="12" md="6">
            <h2 class="text-h5 font-weight-bold mb-2">
              Search Results
            </h2>
            <p class="text-body-1 text-medium-emphasis">
              {{ total }} anime found
              <span v-if="totalPages > 1"> (Page {{ currentPage }} of {{ totalPages }})</span>
            </p>
          </v-col>
          <v-col cols="12" md="6" class="d-flex align-center justify-end gap-2">
            <v-select
              v-model="sortBy"
              :items="sortOptions"
              variant="outlined"
              density="compact"
              hide-details
              style="max-width: 200px;"
              @update:model-value="onSortChange"
            ></v-select>
            <v-btn
              icon
              variant="outlined"
              @click="toggleSortOrder"
              :title="sortOrder === 'desc' ? 'Descending' : 'Ascending'"
            >
              <v-icon>{{ sortOrder === 'desc' ? 'mdi-arrow-down' : 'mdi-arrow-up' }}</v-icon>
            </v-btn>
          </v-col>
        </v-row>

        <!-- Loading State -->
        <v-row v-if="loading">
          <v-col cols="12" class="text-center py-12">
            <v-progress-circular
              indeterminate
              color="primary"
              size="64"
            ></v-progress-circular>
            <p class="text-h6 mt-4">Searching...</p>
          </v-col>
        </v-row>

        <!-- Search Results Grid -->
        <v-row v-else-if="searchResults.length > 0" class="mt-4">
          <v-col
            v-for="anime in searchResults"
            :key="anime.anilistId"
            cols="12"
            sm="6"
            md="4"
            lg="2"
          >
            <AnimeCard :anime="anime" />
          </v-col>
        </v-row>

        <!-- No Results -->
        <v-row v-else-if="hasActiveFilters && !loading">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-emoticon-sad-outline</v-icon>
            <p class="text-h6 mt-4">No anime found</p>
            <p class="text-body-1 text-medium-emphasis">
              Try adjusting your filters to find more results
            </p>
          </v-col>
        </v-row>

        <!-- Initial State (No Filters) -->
        <v-row v-else-if="!hasActiveFilters && !loading">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-filter-variant</v-icon>
            <p class="text-h6 mt-4">Advanced Anime Search</p>
            <p class="text-body-1 text-medium-emphasis">
              Browse anime details and click the + icon next to studios, genres, or tags to add them to your search
            </p>
          </v-col>
        </v-row>

        <!-- Pagination -->
        <v-row v-if="totalPages > 1 && !loading" class="mt-4">
          <v-col cols="12" class="d-flex justify-center">
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
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '~/utils/api'

const route = useRoute()
const router = useRouter()

const goBack = () => {
  router.back()
}

const loading = ref(false)
const searchResults = ref<any[]>([])
const total = ref(0)
const currentPage = ref(1)
const totalPages = ref(0)

const activeStudios = ref<string[]>([])
const activeGenres = ref<string[]>([])
const activeTags = ref<string[]>([])

const sortBy = ref('score')
const sortOrder = ref<'asc' | 'desc'>('desc')

const sortOptions = [
  { value: 'relevance', title: 'Relevance' },
  { value: 'score', title: 'Score' },
  { value: 'year', title: 'Year' },
  { value: 'title', title: 'Title' }
]

const hasActiveFilters = computed(() => {
  return activeStudios.value.length > 0 ||
         activeGenres.value.length > 0 ||
         activeTags.value.length > 0
})

const currentSearchContext = computed(() => ({
  studios: activeStudios.value,
  genres: activeGenres.value,
  tags: activeTags.value
}))

// Initialize filters from URL query params
const initializeFiltersFromQuery = () => {
  if (route.query.studios) {
    activeStudios.value = (route.query.studios as string).split(',').filter(Boolean)
  }
  if (route.query.genres) {
    activeGenres.value = (route.query.genres as string).split(',').filter(Boolean)
  }
  if (route.query.tags) {
    activeTags.value = (route.query.tags as string).split(',').filter(Boolean)
  }
  if (route.query.page) {
    currentPage.value = parseInt(route.query.page as string) || 1
  }
  if (route.query.sortBy) {
    sortBy.value = route.query.sortBy as string
  }
  if (route.query.sortOrder) {
    sortOrder.value = route.query.sortOrder as 'asc' | 'desc'
  }
}

// Update URL with current filters
const updateURL = () => {
  const query: any = {}

  if (activeStudios.value.length > 0) {
    query.studios = activeStudios.value.join(',')
  }
  if (activeGenres.value.length > 0) {
    query.genres = activeGenres.value.join(',')
  }
  if (activeTags.value.length > 0) {
    query.tags = activeTags.value.join(',')
  }
  if (currentPage.value > 1) {
    query.page = currentPage.value.toString()
  }
  if (sortBy.value !== 'score') {
    query.sortBy = sortBy.value
  }
  if (sortOrder.value !== 'desc') {
    query.sortOrder = sortOrder.value
  }

  router.push({ query })
}

// Remove individual filters
const removeStudio = (studio: string) => {
  activeStudios.value = activeStudios.value.filter(s => s !== studio)
  currentPage.value = 1
  updateURL()
}

const removeGenre = (genre: string) => {
  activeGenres.value = activeGenres.value.filter(g => g !== genre)
  currentPage.value = 1
  updateURL()
}

const removeTag = (tag: string) => {
  activeTags.value = activeTags.value.filter(t => t !== tag)
  currentPage.value = 1
  updateURL()
}

const clearAllFilters = () => {
  activeStudios.value = []
  activeGenres.value = []
  activeTags.value = []
  currentPage.value = 1
  searchResults.value = []
  total.value = 0
  totalPages.value = 0
  router.push({ query: {} })
}

const onPageChange = (page: number) => {
  currentPage.value = page
  updateURL()
  // Scroll to top
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const toggleSortOrder = () => {
  sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  currentPage.value = 1
  updateURL()
}

const onSortChange = () => {
  currentPage.value = 1
  updateURL()
}

const performSearch = async () => {
  if (!hasActiveFilters.value) {
    searchResults.value = []
    total.value = 0
    totalPages.value = 0
    return
  }

  loading.value = true
  try {
    const params: any = {
      page: currentPage.value,
      limit: 18,
      sort: sortBy.value,
      order: sortOrder.value
    }

    if (activeStudios.value.length > 0) {
      params.studios = activeStudios.value.join(',')
    }
    if (activeGenres.value.length > 0) {
      params.genres = activeGenres.value.join(',')
    }
    if (activeTags.value.length > 0) {
      params.tags = activeTags.value.join(',')
    }

    const response = await api<any>('/anime/advanced-search', { params })

    if (response.success) {
      searchResults.value = response.data
      total.value = response.total
      totalPages.value = response.totalPages
    }
  } catch (error) {
    console.error('Advanced search error:', error)
    searchResults.value = []
    total.value = 0
    totalPages.value = 0
  } finally {
    loading.value = false
  }
}

// Watch for URL query changes
watch(() => route.query, () => {
  initializeFiltersFromQuery()
  performSearch()
}, { immediate: true, deep: true })

// Set page title
document.title = 'Advanced Search - Anigraph'
</script>

<style scoped>
.gap-2 {
  gap: 8px;
}
</style>
