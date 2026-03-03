<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container>
        <!-- Search Bar -->
        <v-row class="mt-4 mb-8">
          <v-col cols="12">
            <SearchBar tracking-source="search-page" />
          </v-col>
        </v-row>

        <!-- Search Results Header -->
        <v-row v-if="searchQuery">
          <v-col cols="12">
            <h2 class="text-h4 font-weight-bold mb-2">
              Search Results
            </h2>
            <p class="text-body-1 text-medium-emphasis">
              {{ resultsCount }} results for "{{ searchQuery }}"
            </p>
          </v-col>
        </v-row>

        <!-- Filters -->
        <v-row v-if="searchResults.length > 0" class="mb-4">
          <v-col cols="12" md="3">
            <v-select
              v-model="selectedFormat"
              :items="formats"
              label="Format"
              clearable
              variant="outlined"
              density="compact"
            ></v-select>
          </v-col>
          <v-col cols="12" md="3">
            <v-select
              v-model="selectedYear"
              :items="years"
              label="Year"
              clearable
              variant="outlined"
              density="compact"
            ></v-select>
          </v-col>
          <v-col cols="12" md="3">
            <v-select
              v-model="sortBy"
              :items="sortOptions"
              label="Sort by"
              variant="outlined"
              density="compact"
            ></v-select>
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
        <v-row v-else-if="filteredResults.length > 0">
          <v-col
            v-for="anime in filteredResults"
            :key="anime.id"
            cols="12"
            sm="6"
            md="4"
            lg="3"
          >
            <AnimeCard :anime="anime" />
          </v-col>
        </v-row>

        <!-- No Results -->
        <v-row v-else-if="searchQuery">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-magnify-remove-outline</v-icon>
            <p class="text-h6 mt-4">No results found</p>
            <p class="text-body-1 text-medium-emphasis">
              Try a different search term
            </p>
          </v-col>
        </v-row>

        <!-- Initial State -->
        <v-row v-else>
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-magnify</v-icon>
            <p class="text-h6 mt-4">Start searching</p>
            <p class="text-body-1 text-medium-emphasis">
              Use the search bar above to find anime
            </p>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/utils/api'

const route = useRoute()
const router = useRouter()
const searchQuery = ref('')

const goBack = () => {
  router.back()
}
const loading = ref(false)
const searchResults = ref<any[]>([])
const selectedFormat = ref(null)
const selectedYear = ref(null)
const sortBy = ref('relevance')

const sortOptions = [
  { title: 'Relevance', value: 'relevance' },
  { title: 'Score (High to Low)', value: 'score_desc' },
  { title: 'Score (Low to High)', value: 'score_asc' },
  { title: 'Year (Newest)', value: 'year_desc' },
  { title: 'Year (Oldest)', value: 'year_asc' },
  { title: 'Title (A-Z)', value: 'title_asc' }
]

const resultsCount = computed(() => {
  return filteredResults.value.length
})

const formats = computed(() => {
  const uniqueFormats = [...new Set(searchResults.value.map(a => a.format).filter(Boolean))]
  return uniqueFormats.map(f => ({ title: f, value: f }))
})

const years = computed(() => {
  const uniqueYears = [...new Set(searchResults.value.map(a => a.seasonYear).filter(Boolean))]
  return uniqueYears.sort((a, b) => b - a).map(y => ({ title: y.toString(), value: y }))
})

const filteredResults = computed(() => {
  let results = [...searchResults.value]

  // Apply filters
  if (selectedFormat.value) {
    results = results.filter(a => a.format === selectedFormat.value)
  }
  if (selectedYear.value) {
    results = results.filter(a => a.seasonYear === selectedYear.value)
  }

  // Apply sorting
  switch (sortBy.value) {
    case 'score_desc':
      results.sort((a, b) => (b.averageScore || 0) - (a.averageScore || 0))
      break
    case 'score_asc':
      results.sort((a, b) => (a.averageScore || 0) - (b.averageScore || 0))
      break
    case 'year_desc':
      results.sort((a, b) => {
        // Sort by year DESC, then by season DESC (Fall -> Summer -> Spring -> Winter)
        const yearDiff = (b.seasonYear || 0) - (a.seasonYear || 0)
        if (yearDiff !== 0) return yearDiff

        const seasonOrder: Record<string, number> = { 'FALL': 4, 'SUMMER': 3, 'SPRING': 2, 'WINTER': 1 }
        const seasonA = seasonOrder[a.season as string] || 0
        const seasonB = seasonOrder[b.season as string] || 0
        return seasonB - seasonA
      })
      break
    case 'year_asc':
      results.sort((a, b) => {
        // Sort by year ASC, then by season ASC (Winter -> Spring -> Summer -> Fall)
        const yearDiff = (a.seasonYear || 0) - (b.seasonYear || 0)
        if (yearDiff !== 0) return yearDiff

        const seasonOrder: Record<string, number> = { 'WINTER': 1, 'SPRING': 2, 'SUMMER': 3, 'FALL': 4 }
        const seasonA = seasonOrder[a.season as string] || 0
        const seasonB = seasonOrder[b.season as string] || 0
        return seasonA - seasonB
      })
      break
    case 'title_asc':
      results.sort((a, b) => {
        const titleA = (a.titleEnglish || a.title || '').toLowerCase()
        const titleB = (b.titleEnglish || b.title || '').toLowerCase()
        return titleA.localeCompare(titleB)
      })
      break
  }

  return results
})

const performSearch = async (query: string) => {
  if (!query || query.length < 2) {
    searchResults.value = []
    return
  }

  loading.value = true
  try {
    const response = await api<any>('/anime/search', {
      params: { q: query, limit: '50' }
    })

    if (response.success) {
      searchResults.value = response.data
    }
  } catch (error) {
    console.error('Search error:', error)
    searchResults.value = []
  } finally {
    loading.value = false
  }
}

watch(() => route.query.q, (newQuery) => {
  if (newQuery) {
    searchQuery.value = newQuery as string
    performSearch(newQuery as string)
  }
}, { immediate: true })
</script>

<style scoped>
/* Add any custom styles if needed */
</style>
