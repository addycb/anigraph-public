<template>
  <div class="studio-timeline-container">
    <v-card>
      <v-card-text>
        <!-- Loading State -->
        <div v-if="loading" class="text-center py-12">
          <v-progress-circular
            indeterminate
            color="primary"
            size="64"
          ></v-progress-circular>
          <p class="mt-4">Loading timeline...</p>
        </div>

        <!-- Timeline -->
        <div v-else-if="timelineData && timelineData.length > 0">
          <!-- Timeline Container -->
          <div class="timeline-container">
            <!-- Navigation Controls -->
            <div class="timeline-navigation">
              <v-btn
                icon
                size="small"
                variant="elevated"
                @click="scrollTimeline('left')"
                :disabled="!canScrollLeft"
              >
                <v-icon>mdi-chevron-left</v-icon>
              </v-btn>
              <v-btn
                icon
                size="small"
                variant="elevated"
                @click="scrollTimeline('right')"
                :disabled="!canScrollRight"
              >
                <v-icon>mdi-chevron-right</v-icon>
              </v-btn>
            </div>

            <div ref="timelineWrapper" class="timeline-wrapper" @scroll="updateScrollButtons">
              <div class="timeline-track">
                <div
                  v-for="(yearGroup, index) in timelineByYear"
                  :key="yearGroup.year"
                  class="timeline-year-section"
                >
                  <!-- Year marker -->
                  <div class="year-marker">
                    <v-chip color="primary" size="large" class="year-badge">
                      {{ yearGroup.year }}
                    </v-chip>
                    <div class="year-line"></div>
                  </div>

                  <!-- Anime cards for this year -->
                  <div class="year-cards">
                    <v-card
                      v-for="(production, idx) in yearGroup.productions"
                      :key="idx"
                      :to="`/anime/${encodeURIComponent(production.anilistId)}`"
                      class="timeline-anime-card"
                      hover
                      elevation="2"
                    >
                      <!-- Production type indicator (main vs supporting) -->
                      <v-tooltip location="top">
                        <template v-slot:activator="{ props: roleTooltipProps }">
                          <div
                            v-bind="roleTooltipProps"
                            class="production-indicator"
                            :style="{ backgroundColor: production.isMain ? 'var(--color-primary)' : 'var(--color-text-muted)' }"
                          >
                          </div>
                        </template>
                        <span>{{ production.isMain ? 'Main Studio' : 'Supporting Studio' }}</span>
                      </v-tooltip>

                      <!-- Tooltip for full title on image -->
                      <v-tooltip location="top">
                        <template v-slot:activator="{ props: titleTooltipProps }">
                          <v-img
                            v-bind="titleTooltipProps"
                            :src="production.coverImage || '/placeholder-anime.jpg'"
                            :alt="production.title"
                            aspect-ratio="0.7"
                            cover
                            class="timeline-card-image"
                          >
                          </v-img>
                        </template>
                        <span>{{ production.title }}</span>
                      </v-tooltip>

                      <v-card-title class="timeline-card-title">
                        {{ production.title }}
                      </v-card-title>

                      <v-card-subtitle v-if="production.season" class="timeline-card-subtitle">
                        {{ production.season }}
                      </v-card-subtitle>
                    </v-card>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Empty State -->
        <div v-else class="text-center py-8">
          <v-icon size="64" color="grey">mdi-timeline-outline</v-icon>
          <p class="text-subtitle-1 mt-4">No timeline data available</p>
          <p class="text-caption text-medium-emphasis">No production history with year data found</p>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useSettings } from '~/composables/useSettings'
import { api } from '~/utils/api'

const props = defineProps<{
  studioName: string
  selectedGenres?: string[]
  selectedTags?: string[]
}>()

const { includeAdult } = useSettings()

interface TimelineProduction {
  anilistId: string
  title: string
  year: number
  season: string | null
  coverImage?: string
  isMain: boolean
}

const loading = ref(true)
const productions = ref<any[]>([])

// Timeline scroll navigation
const timelineWrapper = ref<HTMLElement | null>(null)
const canScrollLeft = ref(false)
const canScrollRight = ref(false)
const checkIntervalId = ref<ReturnType<typeof setInterval> | null>(null)

// Filtered productions based on adult content setting, then selected genres and tags
const filteredProductions = computed(() => {
  // First, filter out adult anime if the setting is off
  let filtered = productions.value
  if (!includeAdult.value) {
    filtered = filtered.filter((entry: any) => !entry.anime?.isAdult)
  }

  // Then apply genre/tag filters if any are active
  const hasGenreTagFilters = (props.selectedGenres && props.selectedGenres.length > 0) ||
                              (props.selectedTags && props.selectedTags.length > 0)

  if (!hasGenreTagFilters) {
    return filtered
  }

  return filtered.filter((entry: any) => {
    const animeGenres = entry.anime?.genres || []
    const animeTags = entry.anime?.tags?.map((t: any) => t.name) || []

    const genreMatch = !props.selectedGenres || props.selectedGenres.length === 0 ||
      props.selectedGenres.every(g => animeGenres.includes(g))

    const tagMatch = !props.selectedTags || props.selectedTags.length === 0 ||
      props.selectedTags.every(t => animeTags.includes(t))

    return genreMatch && tagMatch
  })
})

// Build timeline data from productions
const timelineData = computed<TimelineProduction[]>(() => {
  if (!filteredProductions.value || filteredProductions.value.length === 0) return []

  const timeline = filteredProductions.value
    .filter((entry: any) => {
      // Only include entries with year data
      return entry.anime?.seasonYear
    })
    .map((entry: any) => ({
      anilistId: entry.anime.anilistId,
      title: entry.anime.title,
      year: entry.anime.seasonYear,
      season: entry.anime.season,
      coverImage: entry.anime.coverImage_extraLarge || entry.anime.coverImage_large || entry.anime.coverImage,
      isMain: entry.isMain
    }))
    .sort((a: TimelineProduction, b: TimelineProduction) => {
      if (a.year !== b.year) return a.year - b.year

      // Sort by season within same year (Spring, Summer, Fall, Winter)
      const seasonOrder: Record<string, number> = { SPRING: 0, SUMMER: 1, FALL: 2, WINTER: 3 }
      const aOrder = a.season ? seasonOrder[a.season] ?? 4 : 4
      const bOrder = b.season ? seasonOrder[b.season] ?? 4 : 4
      return aOrder - bOrder
    })

  return timeline
})

// Group timeline data by year for display
const timelineByYear = computed(() => {
  if (!timelineData.value || timelineData.value.length === 0) return []

  const grouped = new Map<number, TimelineProduction[]>()

  timelineData.value.forEach(production => {
    if (!grouped.has(production.year)) {
      grouped.set(production.year, [])
    }
    grouped.get(production.year)!.push(production)
  })

  // Convert to array and sort by year
  return Array.from(grouped.entries())
    .map(([year, productions]) => {
      // Sort productions by season within the year
      const seasonOrder: Record<string, number> = { SPRING: 0, SUMMER: 1, FALL: 2, WINTER: 3 }
      const sortedProductions = [...productions].sort((a, b) => {
        const aOrder = a.season ? seasonOrder[a.season] ?? 4 : 4
        const bOrder = b.season ? seasonOrder[b.season] ?? 4 : 4
        return aOrder - bOrder
      })
      return { year, productions: sortedProductions }
    })
    .sort((a, b) => a.year - b.year)
})

// Scroll timeline left or right
const scrollTimeline = (direction: 'left' | 'right') => {
  if (!timelineWrapper.value) return

  const scrollAmount = 600 // Scroll by 600px
  const currentScroll = timelineWrapper.value.scrollLeft
  const targetScroll = direction === 'left'
    ? currentScroll - scrollAmount
    : currentScroll + scrollAmount

  timelineWrapper.value.scrollTo({
    left: targetScroll,
    behavior: 'smooth'
  })
}

// Update scroll button states
const updateScrollButtons = () => {
  if (!timelineWrapper.value) return

  const { scrollLeft, scrollWidth, clientWidth } = timelineWrapper.value
  canScrollLeft.value = scrollLeft > 0
  canScrollRight.value = scrollLeft < scrollWidth - clientWidth - 10 // 10px threshold
}

// Handle keyboard navigation
const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    scrollTimeline('left')
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    scrollTimeline('right')
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    const response = await api(`/studio/${encodeURIComponent(props.studioName)}`)

    if (response.success) {
      productions.value = response.data.productions || []
    }
  } catch (error) {
    console.error('Error fetching studio timeline data:', error)
  } finally {
    loading.value = false
    // Update scroll buttons after data is loaded and DOM is updated
    nextTick(() => {
      updateScrollButtons()
    })
  }
}

onMounted(() => {
  fetchData()

  // Also set up a resize observer to update scroll buttons on window resize
  window.addEventListener('resize', updateScrollButtons)
  window.addEventListener('keydown', handleKeydown)

  // Poll for visibility and update scroll buttons
  // This handles the case where the tab is not initially active
  let attemptCount = 0
  const maxAttempts = 20
  checkIntervalId.value = setInterval(() => {
    attemptCount++
    if (timelineWrapper.value) {
      const { scrollWidth, clientWidth } = timelineWrapper.value
      // Only update if the element has actual dimensions (is visible)
      if (scrollWidth > 0 && clientWidth > 0) {
        updateScrollButtons()
        if (checkIntervalId.value) clearInterval(checkIntervalId.value)
        checkIntervalId.value = null
      }
    }
    if (attemptCount >= maxAttempts) {
      if (checkIntervalId.value) clearInterval(checkIntervalId.value)
      checkIntervalId.value = null
    }
  }, 100)
})

// Cleanup on unmount
onUnmounted(() => {
  window.removeEventListener('resize', updateScrollButtons)
  window.removeEventListener('keydown', handleKeydown)

  // Clean up polling interval
  if (checkIntervalId.value) {
    clearInterval(checkIntervalId.value)
    checkIntervalId.value = null
  }
})

// Update scroll buttons when timeline data changes
watch(timelineData, () => {
  // Wait for DOM to update
  nextTick(() => {
    updateScrollButtons()
  })
})

// Watch for genre/tag filter changes
watch([() => props.selectedGenres, () => props.selectedTags], () => {
  nextTick(() => {
    updateScrollButtons()
  })
}, { deep: true })

// Refetch when studio name changes
watch(() => props.studioName, () => {
  fetchData()
})
</script>

<style scoped>
.studio-timeline-container {
  width: 100%;
}

.timeline-container {
  width: 100%;
  position: relative;
  padding: 16px 0;
}

.timeline-navigation {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 16px;
}

.timeline-wrapper {
  width: 100%;
  overflow-x: auto;
  overflow-y: visible;
  padding: 24px 16px;
}

.timeline-track {
  display: flex;
  flex-direction: row;
  gap: 48px;
  min-width: min-content;
}

.timeline-year-section {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: max-content;
}

.year-marker {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.year-badge {
  flex-shrink: 0;
  z-index: 2;
}

.year-line {
  width: 3px;
  height: 60px;
  background: var(--gradient-timeline);
  border-radius: 2px;
}

.year-cards {
  display: flex;
  flex-direction: row;
  gap: 16px;
  padding-bottom: 16px;
}

.timeline-anime-card {
  width: 160px;
  height: 100%;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  transition: transform 0.2s ease-in-out, box-shadow 0.2s ease-in-out;
  overflow: hidden;
}

.timeline-anime-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.2) !important;
}

.timeline-card-image {
  position: relative;
}

.production-indicator {
  width: 100%;
  height: 6px;
  cursor: help;
  transition: height 0.2s ease-in-out;
  border-radius: 4px 4px 0 0;
}

.production-indicator:hover {
  height: 8px;
}

.timeline-card-title {
  font-size: 0.875rem;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  padding: 12px 12px 4px 12px;
}

.timeline-card-subtitle {
  font-size: 0.75rem;
  padding: 0 12px 12px 12px;
}

/* Responsive adjustments */
@media (max-width: 960px) {
  .timeline-anime-card {
    width: 140px;
  }

  .timeline-track {
    gap: 32px;
  }

  .year-cards {
    gap: 12px;
  }
}

@media (max-width: 600px) {
  .timeline-anime-card {
    width: 120px;
  }

  .timeline-track {
    gap: 24px;
  }

  .year-cards {
    gap: 10px;
  }

  .timeline-wrapper {
    padding: 16px 8px;
  }
}
</style>
