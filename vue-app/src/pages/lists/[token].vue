<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container fluid>
        <!-- Loading State -->
        <v-row v-if="loading">
          <v-col cols="12" class="text-center py-12">
            <v-progress-circular
              indeterminate
              color="primary"
              size="64"
            ></v-progress-circular>
            <p class="text-h6 mt-4">Loading list...</p>
          </v-col>
        </v-row>

        <!-- Error State -->
        <v-row v-else-if="error">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="error">mdi-alert-circle</v-icon>
            <h2 class="text-h4 mt-4">List Not Found</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              This list doesn't exist or is no longer public
            </p>
            <v-btn
              color="primary"
              size="large"
              class="mt-4"
              to="/lists/public"
            >
              <v-icon start>mdi-arrow-left</v-icon>
              Browse Public Lists
            </v-btn>
          </v-col>
        </v-row>

        <!-- List Content -->
        <template v-else-if="list">
          <!-- Page Header -->
          <v-row class="mb-6">
            <v-col cols="12">
              <div class="d-flex align-center justify-space-between">
                <div>
                  <h1 class="text-h3 font-weight-bold mb-2">
                    <v-icon size="large" class="mr-2">mdi-bookmark-multiple</v-icon>
                    {{ list.name }}
                  </h1>
                  <p v-if="list.description" class="text-h6 text-medium-emphasis">
                    {{ list.description }}
                  </p>
                  <v-chip size="small" class="mt-2" variant="tonal" color="success">
                    <v-icon start size="small">mdi-earth</v-icon>
                    Public List
                  </v-chip>
                  <v-chip size="small" class="mt-2 ml-2" variant="tonal">
                    <v-icon start size="small">mdi-image-multiple</v-icon>
                    {{ listItems.length }} anime
                  </v-chip>
                </div>
                <div class="d-flex gap-2">
                  <!-- Taste Profile Button -->
                  <v-btn
                    variant="tonal"
                    color="primary"
                    @click="viewTasteProfile"
                    :loading="loadingTasteProfile"
                  >
                    <v-icon start>mdi-chart-box</v-icon>
                    View Taste Profile
                  </v-btn>

                  <!-- Recommendations Button -->
                  <v-btn
                    variant="tonal"
                    color="secondary"
                    @click="viewRecommendations"
                    :loading="loadingRecommendations"
                  >
                    <v-icon start>mdi-lightbulb</v-icon>
                    Get Recommendations
                  </v-btn>

                  <!-- Share Button -->
                  <v-btn
                    variant="outlined"
                    @click="copyShareLink"
                  >
                    <v-icon start>mdi-share-variant</v-icon>
                    Share
                  </v-btn>
                </div>
              </div>
            </v-col>
          </v-row>

          <!-- Anime List -->
          <div v-if="listItems.length > 0">
            <v-list lines="two" class="anime-list">
              <v-list-item
                v-for="anime in listItems"
                :key="anime.id"
                class="anime-list-item"
                @click="navigateToAnime(anime.anilistId)"
              >
                <template v-slot:prepend>
                  <div class="banner-thumbnail mr-4">
                    <v-img
                      :src="anime.bannerImage || anime.coverImage_large || anime.coverImage || '/placeholder-anime.jpg'"
                      :alt="anime.title"
                      cover
                      aspect-ratio="2.5"
                    />
                  </div>
                </template>

                <v-list-item-title class="text-h6 font-weight-medium">
                  {{ anime.title }}
                </v-list-item-title>

                <v-list-item-subtitle class="mt-1">
                  <v-chip
                    v-if="anime.format"
                    size="x-small"
                    variant="tonal"
                    class="mr-1"
                  >
                    {{ anime.format }}
                  </v-chip>
                  <v-chip
                    v-if="anime.seasonYear"
                    size="x-small"
                    variant="tonal"
                    class="mr-1"
                  >
                    {{ anime.seasonYear }}
                  </v-chip>
                  <v-chip
                    v-if="anime.averageScore"
                    size="x-small"
                    variant="tonal"
                    color="success"
                  >
                    <v-icon start size="x-small">mdi-star</v-icon>
                    {{ anime.averageScore }}
                  </v-chip>
                </v-list-item-subtitle>

                <!-- Action Buttons (only for authenticated users) -->
                <template v-if="isAuthenticated" v-slot:append>
                  <div class="action-buttons-bubble" @click.stop>
                    <v-btn
                      icon
                      class="favorite-bubble-btn"
                      :class="{ 'favorited': isFavorited(anime.anilistId) }"
                      @click="toggleFavorite(anime.anilistId, $event)"
                      size="small"
                      variant="flat"
                    >
                      <v-icon size="20" color="white">
                        {{ isFavorited(anime.anilistId) ? 'mdi-heart' : 'mdi-heart-outline' }}
                      </v-icon>
                    </v-btn>
                    <div class="button-divider"></div>
                    <ListButton :anime-id="anime.anilistId" bubble-mode size="small" />
                  </div>
                </template>
              </v-list-item>
            </v-list>
          </div>

          <!-- Empty List -->
          <v-row v-else>
            <v-col cols="12" class="text-center py-12">
              <v-icon size="80" color="grey">mdi-bookmark-outline</v-icon>
              <h2 class="text-h4 mt-4">No anime in this list</h2>
            </v-col>
          </v-row>
        </template>
      </v-container>
    </v-main>

    <!-- Taste Profile Dialog -->
    <v-dialog v-model="showTasteProfileDialog" max-width="900">
      <v-card v-if="tasteProfile">
        <v-card-title>
          <v-icon start color="primary">mdi-chart-box</v-icon>
          Taste Profile for "{{ list?.name }}"
        </v-card-title>
        <v-card-text>
          <TasteProfile :profile-data="tasteProfile" :show-title="false" />
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showTasteProfileDialog = false">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Recommendations Dialog -->
    <v-dialog v-model="showRecommendationsDialog" max-width="1200" scrollable>
      <v-card>
        <v-card-title>
          <v-icon start color="secondary">mdi-lightbulb</v-icon>
          Recommendations Based on "{{ list?.name }}"
        </v-card-title>
        <v-card-text style="max-height: 70vh;">
          <v-list v-if="recommendations.length > 0" lines="two" class="anime-list">
            <v-list-item
              v-for="anime in recommendations"
              :key="anime.id"
              class="anime-list-item"
              @click="navigateToAnime(anime.anilistId)"
            >
              <template v-slot:prepend>
                <div class="banner-thumbnail-small mr-3">
                  <v-img
                    :src="anime.bannerImage || anime.coverImage_large || anime.coverImage || '/placeholder-anime.jpg'"
                    :alt="anime.title"
                    cover
                    aspect-ratio="2.5"
                  />
                </div>
              </template>

              <v-list-item-title class="text-subtitle-1 font-weight-medium">
                {{ anime.title }}
              </v-list-item-title>

              <v-list-item-subtitle class="mt-1">
                <v-chip
                  v-if="anime.format"
                  size="x-small"
                  variant="tonal"
                  class="mr-1"
                >
                  {{ anime.format }}
                </v-chip>
                <v-chip
                  v-if="anime.seasonYear"
                  size="x-small"
                  variant="tonal"
                  class="mr-1"
                >
                  {{ anime.seasonYear }}
                </v-chip>
                <v-chip
                  v-if="anime.averageScore"
                  size="x-small"
                  variant="tonal"
                  color="success"
                >
                  <v-icon start size="x-small">mdi-star</v-icon>
                  {{ anime.averageScore }}
                </v-chip>
              </v-list-item-subtitle>

              <!-- Action Buttons (only for authenticated users) -->
              <template v-if="isAuthenticated" v-slot:append>
                <div class="action-buttons-bubble action-buttons-bubble-small" @click.stop>
                  <v-btn
                    icon
                    class="favorite-bubble-btn"
                    :class="{ 'favorited': isFavorited(anime.anilistId) }"
                    @click="toggleFavorite(anime.anilistId, $event)"
                    size="x-small"
                    variant="flat"
                  >
                    <v-icon size="16" color="white">
                      {{ isFavorited(anime.anilistId) ? 'mdi-heart' : 'mdi-heart-outline' }}
                    </v-icon>
                  </v-btn>
                  <div class="button-divider button-divider-small"></div>
                  <ListButton :anime-id="anime.anilistId" bubble-mode size="x-small" />
                </div>
              </template>
            </v-list-item>
          </v-list>
          <div v-else class="text-center py-12">
            <v-icon size="80" color="grey">mdi-lightbulb-outline</v-icon>
            <h3 class="text-h5 mt-4">No recommendations available</h3>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showRecommendationsDialog = false">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <Snackbar />
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/utils/api'
import { useCsrf } from '@/composables/useCsrf'
import { useAuth } from '@/composables/useAuth'
import { useFavorites } from '@/composables/useFavorites'
import { useSnackbar } from '@/composables/useSnackbar'
import { useSettings } from '@/composables/useSettings'

const route = useRoute()
const router = useRouter()
const { csrfPost } = useCsrf()
const { isAuthenticated } = useAuth()
const { fetchFavorites, isFavorited: checkIsFavorited, toggleFavorite: toggleFavoriteCache } = useFavorites()
const shareToken = computed(() => route.params.token as string)

const loading = ref(true)
const error = ref(false)
const list = ref<any>(null)
const listItems = ref<any[]>([])
const loadingTasteProfile = ref(false)
const loadingRecommendations = ref(false)
const showTasteProfileDialog = ref(false)
const showRecommendationsDialog = ref(false)
const tasteProfile = ref<any>(null)
const recommendations = ref<any[]>([])
const snackbar = useSnackbar()

const fetchList = async () => {
  loading.value = true
  error.value = false

  try {
    const { includeAdult } = useSettings()
    const response = await api<any>(`/lists/share/${shareToken.value}`, {
      params: {
        includeAdult: String(includeAdult.value)
      }
    })

    if (response.success) {
      list.value = response.data.list
      listItems.value = response.data.items
    }
  } catch (err) {
    console.error('Error fetching list:', err)
    error.value = true
  } finally {
    loading.value = false
  }
}

const navigateToAnime = (animeId: number) => {
  router.push(`/anime/${animeId}`)
}

// Check if an anime is favorited
const isFavorited = (animeId: number) => {
  return checkIsFavorited(animeId)
}

// Toggle favorite for an anime
const toggleFavorite = async (animeId: number, event: Event) => {
  event.stopPropagation() // Prevent navigation when clicking favorite button

  const success = await toggleFavoriteCache(animeId)

  if (!success) {
    // Error already logged in composable
  }
}

const copyShareLink = () => {
  const url = window.location.href
  navigator.clipboard.writeText(url)
  snackbar.showSuccess('Share link copied to clipboard!')
}

const viewTasteProfile = async () => {
  if (listItems.value.length === 0) return

  loadingTasteProfile.value = true
  try {
    // Compute taste profile based on list items
    const animeIds = listItems.value.map(item => item.anilistId)
    const response = await csrfPost<any>('/api/compute-list-taste-profile', {
      animeIds
    })

    if (response.success && response.data) {
      tasteProfile.value = response.data
      showTasteProfileDialog.value = true
    } else if (response.error) {
      snackbar.showError(response.error)
    }
  } catch (error: any) {
    console.error('Error computing taste profile:', error)
    if (error.statusCode === 429) {
      snackbar.showError('Rate limit exceeded. Please try again later')
    } else {
      snackbar.showError(error.data?.message || 'Failed to compute taste profile')
    }
  } finally {
    loadingTasteProfile.value = false
  }
}

const viewRecommendations = async () => {
  if (listItems.value.length === 0) return
  if (listItems.value.length < 3) {
    snackbar.showError(`Need at least 3 anime in this list to compute recommendations (${listItems.value.length}/3)`)
    return
  }

  loadingRecommendations.value = true
  try {
    // Get recommendations based on list items
    const animeIds = listItems.value.map(item => item.anilistId)
    const response = await csrfPost<any>('/api/compute-list-recommendations', {
      animeIds,
      limit: 20
    })

    if (response.success && response.data) {
      recommendations.value = response.data
      showRecommendationsDialog.value = true
    } else if (response.error) {
      snackbar.showError(response.error)
    }
  } catch (error: any) {
    console.error('Error getting recommendations:', error)
    if (error.statusCode === 429) {
      snackbar.showError('Rate limit exceeded. Please try again later')
    } else {
      snackbar.showError(error.data?.message || 'Failed to get recommendations')
    }
  } finally {
    loadingRecommendations.value = false
  }
}

onMounted(async () => {
  await fetchList()
  // Fetch favorites if authenticated
  if (isAuthenticated.value) {
    await fetchFavorites()
  }
})

watchEffect(() => {
  document.title = (list.value ? `${list.value.name} - Public List` : 'Public List') + ' - Anigraph'
})
</script>

<style scoped>
.anime-list {
  background: transparent;
}

.anime-list-item {
  cursor: pointer;
  border-bottom: 1px solid rgba(var(--color-text-rgb), 0.05);
  transition: background-color 0.2s ease;
}

.anime-list-item:hover {
  background-color: rgba(var(--color-text-rgb), 0.03);
}

.banner-thumbnail {
  width: 180px;
  height: 72px;
  border-radius: 8px;
  overflow: hidden;
  flex-shrink: 0;
}

.banner-thumbnail-small {
  width: 135px;
  height: 54px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
}

.action-buttons-bubble {
  display: flex;
  align-items: center;
  background-color: rgba(var(--color-overlay-rgb), 0.85) !important;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(var(--color-text-rgb), 0.2);
  border-radius: var(--radius-pill);
  padding: 4px;
  box-shadow: var(--shadow-md);
  transition: all var(--transition-base);
}

.action-buttons-bubble:hover {
  border-color: rgba(var(--color-text-rgb), 0.3);
  box-shadow: var(--shadow-lg);
}

.action-buttons-bubble-small {
  border-radius: 20px;
  padding: 3px;
}

.button-divider {
  width: 1px;
  height: 32px;
  background: rgba(var(--color-text-rgb), 0.2);
  margin: 0 4px;
}

.button-divider-small {
  height: 24px;
  margin: 0 3px;
}

.favorite-bubble-btn {
  background-color: transparent !important;
  box-shadow: none !important;
  transition: all 0.2s ease;
}

.favorite-bubble-btn:hover {
  background-color: rgba(var(--color-text-rgb), 0.1) !important;
}

.favorite-bubble-btn :deep(.v-icon) {
  color: var(--color-text) !important;
}

.favorite-bubble-btn.favorited :deep(.v-icon) {
  color: var(--color-error) !important;
}
</style>
