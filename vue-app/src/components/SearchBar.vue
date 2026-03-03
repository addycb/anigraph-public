<template>
  <v-autocomplete
    v-model="selectedItem"
    v-model:search="searchQuery"
    :items="displayItems"
    :loading="isLoading"
    item-title="displayTitle"
    item-value="id"
    :label="label"
    :placeholder="placeholder"
    prepend-inner-icon="mdi-magnify"
    :variant="variant"
    :density="density"
    :rounded="rounded"
    :class="searchBarClass"
    :hide-details="hideDetails"
    :autofocus="autofocus"
    clearable
    no-filter
    return-object
    @update:search="handleSearch"
    @update:model-value="onItemSelected"
    @keydown.enter="onEnterPress"
    @focus="showHistory = true"
  >
    <template #item="{ props, item }">
      <!-- Search History Items -->
      <v-list-item
        v-if="item.raw.type === 'history'"
        @click="onHistoryItemClick(item.raw.query)"
        @mousedown.prevent
        class="search-history-item"
      >
        <template #prepend>
          <v-icon color="grey-lighten-1">mdi-history</v-icon>
        </template>
        <v-list-item-title>{{ item.raw.query }}</v-list-item-title>
        <template #append>
          <v-btn
            icon="mdi-close"
            variant="text"
            size="x-small"
            @click.stop="removeHistory(item.raw.query)"
          ></v-btn>
        </template>
      </v-list-item>

      <!-- Regular Search Results -->
      <v-list-item
        v-else
        v-bind="props"
        :subtitle="item.raw.displaySubtitle"
        @mousedown.prevent
      >
        <template #prepend>
          <v-avatar v-if="item.raw.type === 'anime' && item.raw.coverImage" :image="item.raw.coverImage"></v-avatar>
          <v-avatar v-else-if="item.raw.type === 'staff' && item.raw.picture" :image="item.raw.picture"></v-avatar>
          <v-avatar v-else-if="item.raw.type === 'anime'" color="grey">
            <v-icon>mdi-filmstrip</v-icon>
          </v-avatar>
          <v-avatar v-else-if="item.raw.type === 'staff'" color="grey">
            <v-icon>mdi-account</v-icon>
          </v-avatar>
          <v-avatar v-else-if="item.raw.type === 'studio' && item.raw.imageUrl" :image="item.raw.imageUrl"></v-avatar>
          <v-avatar v-else color="grey">
            <v-icon>mdi-office-building</v-icon>
          </v-avatar>
        </template>
      </v-list-item>
    </template>

    <template #no-data>
      <v-list-item>
        <v-list-item-title v-if="searchQuery && searchQuery.length > 0">
          No results found
        </v-list-item-title>
        <v-list-item-title v-else>
          Start typing to search
        </v-list-item-title>
      </v-list-item>
    </template>

    <!-- Arrow button for floating variant -->
    <template v-if="showArrowButton" #append-inner>
      <v-btn
        v-if="searchQuery && searchQuery.trim() !== ''"
        icon
        size="small"
        variant="text"
        @click="onEnterPress"
      >
        <v-icon>mdi-arrow-right</v-icon>
      </v-btn>
    </template>
  </v-autocomplete>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/utils/api'
import { useAnalytics } from '@/composables/useAnalytics'
import { useSearchHistory } from '@/composables/useSearchHistory'

interface Props {
  variant?: 'solo' | 'outlined' | 'underlined' | 'filled' | 'plain'
  density?: 'default' | 'comfortable' | 'compact'
  rounded?: boolean | string
  label?: string
  placeholder?: string
  hideDetails?: boolean
  floating?: boolean
  showArrowButton?: boolean
  trackingSource?: string
  autofocus?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'solo',
  density: 'default',
  rounded: true,
  label: 'Search works, staff, studios...',
  placeholder: 'Start typing to search...',
  hideDetails: false,
  floating: false,
  showArrowButton: false,
  trackingSource: 'search',
  autofocus: false
})

const emit = defineEmits<{
  navigate: []
  search: [query: string]
}>()

const router = useRouter()
const { trackSearch, trackSearchSelect } = useAnalytics()
const { getHistory, addToHistory, removeFromHistory } = useSearchHistory()

const searchQuery = ref('')
const selectedItem = ref(null)
const searchResults = ref<any[]>([])
const isLoading = ref(false)
const showHistory = ref(false)

let searchTimeout: NodeJS.Timeout | null = null

const searchBarClass = computed(() => {
  return props.floating ? 'floating-search-input' : ''
})

// Combine search results with history when appropriate
const displayItems = computed(() => {
  // If there's a query or results, show search results
  if (searchQuery.value && searchQuery.value.length >= 2) {
    return searchResults.value
  }

  // If no query and history should be shown, display recent searches
  if (showHistory.value && !searchQuery.value) {
    const history = getHistory()
    return history.map((query, index) => ({
      id: `history-${index}`,
      type: 'history',
      query,
      displayTitle: query,
      displaySubtitle: 'Recent search'
    }))
  }

  return []
})

const onHistoryItemClick = (query: string) => {
  searchQuery.value = query
  // Trigger search immediately
  handleSearch(query)
}

const removeHistory = (query: string) => {
  removeFromHistory(query)
  // Force recompute by toggling showHistory
  showHistory.value = false
  nextTick(() => {
    showHistory.value = true
  })
}

const handleSearch = async (query: string) => {
  if (!query || query.length < 2) {
    searchResults.value = []
    return
  }

  // Debounce search
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }

  searchTimeout = setTimeout(async () => {
    isLoading.value = true
    try {
      const response = await api('/search/unified', {
        params: { q: query, limit: 5 }
      })

      if (response.success) {
        const allResults = []

        // Add anime results
        if (response.results.anime) {
          allResults.push(...response.results.anime.map((anime: any) => ({
            id: anime.id,
            type: 'anime',
            displayTitle: anime.titleEnglish || anime.titleRomaji || anime.title || '',
            displaySubtitle: getAnimeSubtitle(anime),
            coverImage: anime.coverImage,
            score: anime.score || 0,
            originalData: anime
          })))
        }

        // Add staff results
        if (response.results.staff) {
          allResults.push(...response.results.staff.map((staff: any) => ({
            id: staff.id,
            type: 'staff',
            displayTitle: staff.nameEn || staff.name || '',
            displaySubtitle: 'Staff',
            picture: staff.picture,
            score: staff.score || 0,
            originalData: staff
          })))
        }

        // Add studio results
        if (response.results.studios) {
          allResults.push(...response.results.studios.map((studio: any) => ({
            id: studio.studioId,
            name: studio.name,
            type: 'studio',
            displayTitle: studio.name,
            displaySubtitle: 'Studio',
            imageUrl: studio.imageUrl,
            score: studio.score || 0,
            originalData: studio
          })))
        }

        // Sort by score (descending) to get most relevant first
        allResults.sort((a, b) => b.score - a.score)

        // Get top 3 most relevant
        const top3 = allResults.slice(0, 3)
        const top3Ids = new Set(top3.map(r => `${r.type}-${r.id || r.name}`))

        // Get remaining results, deduplicated
        const remaining = allResults.slice(3).filter(r =>
          !top3Ids.has(`${r.type}-${r.id || r.name}`)
        )

        // Combine: top 3 first, then remaining
        searchResults.value = [...top3, ...remaining]
      }
    } catch (error) {
      console.error('Search error:', error)
      searchResults.value = []
    } finally {
      isLoading.value = false
    }
  }, 300)
}

const onItemSelected = (item: any) => {
  if (!item) return

  // Don't process history items here (handled by onHistoryItemClick)
  if (item.type === 'history') return

  // Save search query to history
  if (searchQuery.value && searchQuery.value.trim()) {
    addToHistory(searchQuery.value.trim())
  }

  // Track the search selection
  trackSearchSelect(item.type, item.id || item.name, props.trackingSource)

  // Navigate based on type
  if (item.type === 'anime') {
    router.push(`/anime/${encodeURIComponent(item.id)}`)
  } else if (item.type === 'staff') {
    router.push(`/staff/${encodeURIComponent(item.id)}`)
  } else if (item.type === 'studio') {
    // Studios use name for URL, not ID
    router.push(`/studio/${encodeURIComponent(item.name)}`)
  }

  emit('navigate')

  // Clear selection and search after navigating
  setTimeout(() => {
    selectedItem.value = null
    searchQuery.value = ''
    searchResults.value = []
    showHistory.value = false
  }, 100)
}

const onEnterPress = () => {
  // If there's a search query and no item is selected, emit search or navigate
  if (searchQuery.value && searchQuery.value.length >= 2 && !selectedItem.value) {
    // Save to history
    addToHistory(searchQuery.value.trim())

    // Track the search query
    trackSearch(searchQuery.value, props.trackingSource)

    // Emit search event (for pages that handle it)
    emit('search', searchQuery.value.trim())

    // Also navigate to home with search query (default behavior)
    router.push(`/home?q=${encodeURIComponent(searchQuery.value)}`)
    emit('navigate')

    // Clear after navigating
    setTimeout(() => {
      selectedItem.value = null
      searchQuery.value = ''
      searchResults.value = []
      showHistory.value = false
    }, 100)
  }
}

const formatLabels: Record<string, string> = {
  'TV': 'TV Series',
  'MOVIE': 'Movie',
  'OVA': 'OVA',
  'ONA': 'ONA',
  'SPECIAL': 'Special',
  'TV_SHORT': 'TV Short',
  'MUSIC': 'Music',
  'MANGA': 'Manga',
  'NOVEL': 'Light Novel',
  'ONE_SHOT': 'One Shot'
}

const getAnimeSubtitle = (anime: any) => {
  const parts = []
  if (anime.seasonYear) parts.push(anime.seasonYear)
  if (anime.format) {
    const label = formatLabels[anime.format] || anime.format
    parts.push(label)
  }
  if (anime.averageScore) parts.push(`\u2605 ${anime.averageScore}`)
  return parts.join(' \u2022 ') || 'Work'
}
</script>

<style scoped>
/* Floating variant styles */
.floating-search-input {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  width: 90%;
  max-width: 700px;
  z-index: 1000;
}

.floating-search-input :deep(.v-field) {
  background: rgba(var(--color-surface-rgb), 0.8) !important;
  backdrop-filter: blur(10px);
  border-radius: 28px !important;
  box-shadow: var(--shadow-lg);
}

.floating-search-input :deep(.v-field__overlay) {
  background: transparent !important;
}

.floating-search-input :deep(.v-input__control) {
  border-radius: 28px !important;
}

.floating-search-input :deep(.v-field__input) {
  color: var(--color-text);
  padding: 8px 16px;
  font-size: 1rem;
}

.floating-search-input :deep(.v-field__input::placeholder) {
  color: rgba(var(--color-text-rgb), 0.6);
}

.floating-search-input :deep(.v-icon) {
  color: rgba(var(--color-text-rgb), 0.8);
}

/* Style the autocomplete menu for floating variant */
.floating-search-input :deep(.v-overlay__content) {
  background: rgba(var(--color-surface-rgb), 0.95) !important;
  backdrop-filter: blur(10px);
  border-radius: 12px;
  margin-top: 8px;
}

.floating-search-input :deep(.v-list) {
  background: transparent !important;
}

.floating-search-input :deep(.v-list-item) {
  color: var(--color-text);
}

.floating-search-input :deep(.v-list-item:hover) {
  background: var(--color-primary-medium);
}

.floating-search-input :deep(.v-list-item-title) {
  color: var(--color-text);
}

.floating-search-input :deep(.v-list-item-subtitle) {
  color: rgba(var(--color-text-rgb), 0.7);
}

/* Search history item styling */
.search-history-item {
  opacity: 0.9;
}

.search-history-item:hover {
  opacity: 1;
}

/* Responsive */
@media (max-width: 600px) {
  .floating-search-input {
    bottom: 16px;
    width: 95%;
  }

  .floating-search-input :deep(.v-field__input) {
    font-size: 0.9rem;
  }
}
</style>
