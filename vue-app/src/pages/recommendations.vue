<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container fluid>
        <!-- Page Header -->
        <ViewToolbar v-model:card-size="cardSize">
          <template #left>
            <div class="page-title-section">
              <div class="d-flex align-center">
                <v-icon color="primary" size="28" class="mr-2">mdi-lightbulb</v-icon>
                <h1 class="text-h5 font-weight-bold mb-0">Recommended For You</h1>
              </div>
              <p v-if="favoritesCount > 0" class="text-caption text-medium-emphasis ml-9 mb-0">
                Based on {{ favoritesCount }} favorites
              </p>
            </div>
          </template>
        </ViewToolbar>

        <!-- Loading State -->
        <v-row v-if="loading || autoComputing">
          <v-col cols="12" class="text-center py-12">
            <v-progress-circular
              indeterminate
              color="primary"
              size="64"
            ></v-progress-circular>
            <p class="text-h6 mt-4">{{ autoComputing ? 'Computing your personalized recommendations...' : 'Loading recommendations...' }}</p>
          </v-col>
        </v-row>

        <!-- Results Grid -->
        <v-row v-else-if="recommendationsList.length > 0">
          <v-col
            v-for="anime in recommendationsList"
            :key="anime.id"
            cols="12"
            sm="6"
            md="4"
            :lg="cardColSize"
          >
            <AnimeCard :anime="anime" />
          </v-col>
        </v-row>

        <!-- Not Logged In State -->
        <v-row v-else-if="!isAuthenticated">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="grey">mdi-account-lock</v-icon>
            <h2 class="text-h4 mt-4">Login Required</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              Please log in with Google to get personalized recommendations
            </p>
            <AuthButton class="mt-4" />
          </v-col>
        </v-row>

        <!-- Empty State - Not Enough Favorites -->
        <v-row v-else-if="favoritesCount < 3">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="grey">mdi-heart-outline</v-icon>
            <h2 class="text-h4 mt-4">Favorite more anime to get recommendations</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              You need at least 3 favorites ({{ favoritesCount }}/3)
            </p>
            <v-btn
              color="primary"
              size="large"
              class="mt-4"
              to="/favorites"
            >
              <v-icon start>mdi-heart</v-icon>
              Go to Favorites
            </v-btn>
          </v-col>
        </v-row>

        <!-- Needs Computation State -->
        <v-row v-else-if="needsComputation">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="primary">mdi-auto-fix</v-icon>
            <h2 class="text-h4 mt-4">Generate Your Recommendations</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              Click below to generate personalized anime recommendations based on your {{ favoritesCount }} favorites
            </p>
            <v-btn
              color="primary"
              size="large"
              class="mt-4"
              @click="computeRecommendations"
              :loading="computing"
            >
              <v-icon start>mdi-sparkles</v-icon>
              Generate Recommendations
            </v-btn>
          </v-col>
        </v-row>

        <!-- Empty State - No Results -->
        <v-row v-else>
          <v-col cols="12" class="text-center py-12">
            <v-icon size="64" color="grey">mdi-archive-outline</v-icon>
            <p class="text-h6 mt-4">No recommendations available</p>
            <p class="text-body-1 text-medium-emphasis">
              Try favoriting more anime to improve recommendations
            </p>
          </v-col>
        </v-row>

        <!-- Load More Button -->
        <v-row v-if="!loading && recommendationsList.length > 0 && hasMore">
          <v-col cols="12" class="text-center py-6">
            <v-btn
              color="primary"
              size="large"
              variant="outlined"
              @click="loadMore"
              :loading="loadingMore"
            >
              Load More
              <v-icon end>mdi-chevron-down</v-icon>
            </v-btn>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useCardSize } from '~/composables/useCardSize'
import { useAuth } from '~/composables/useAuth'
import { useCsrf } from '~/composables/useCsrf'
import { api } from '~/utils/api'

const { cardSize, cardColSize } = useCardSize()
const { isAuthenticated } = useAuth()
const { csrfPost } = useCsrf()

const loading = ref(true)
const loadingMore = ref(false)
const recommendationsList = ref<any[]>([])
const hasMore = ref(true)
const offset = ref(0)
const limit = 24
const favoritesCount = ref(0)
const needsComputation = ref(false)
const computing = ref(false)
const autoComputing = ref(false)

const fetchRecommendations = async (append = false) => {
  if (!isAuthenticated.value) {
    loading.value = false
    return
  }
  if (append) {
    loadingMore.value = true
  } else {
    loading.value = true
    offset.value = 0
    recommendationsList.value = []
  }

  try {
    const params: any = {
      limit,
      offset: offset.value
    }

    const response = await api<any>('/user/recommendations', { params })

    // Update favorites count from response (avoids separate API call)
    if (response.favoritesCount !== undefined) {
      favoritesCount.value = response.favoritesCount
    }

    if (response.needsComputation) {
      needsComputation.value = true
      recommendationsList.value = []
      // Auto-compute if user has enough favorites
      if (favoritesCount.value >= 3) {
        autoComputing.value = true
        await computeRecommendations()
        autoComputing.value = false
      }
    } else if (response.success) {
      needsComputation.value = false
      if (append) {
        recommendationsList.value = [...recommendationsList.value, ...response.data]
      } else {
        recommendationsList.value = response.data
      }

      hasMore.value = response.data.length >= limit
    }
  } catch (error) {
    console.error('Error fetching recommendations:', error)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const computeRecommendations = async () => {
  computing.value = true
  try {
    // First, ensure taste profile exists
    const tasteProfileResponse = await api<any>('/user/taste-profile')

    if (!tasteProfileResponse.exists) {
      // Compute taste profile first
      const computeTasteResponse = await csrfPost<any>('/api/user/compute-taste-profile')

      if (!computeTasteResponse.success && computeTasteResponse.error) {
        throw new Error(computeTasteResponse.error)
      }
    }

    // Now compute recommendations
    const response = await csrfPost<any>('/api/user/compute-recommendations')

    if (response.success || response.data) {
      // Refresh recommendations after computing
      await fetchRecommendations()
    }
  } catch (error: any) {
    console.error('Error computing recommendations:', error)
    if (error.statusCode === 429) {
      alert('Rate limit exceeded. You can only compute recommendations 5 times per hour. Please try again later.')
    } else {
      alert(error.data?.message || 'Failed to compute recommendations. Please try again.')
    }
  } finally {
    computing.value = false
  }
}

const loadMore = () => {
  offset.value += limit
  fetchRecommendations(true)
}

onMounted(async () => {
  if (!isAuthenticated.value) {
    loading.value = false
    return
  }

  // Single call — favoritesCount is returned alongside recommendations
  await fetchRecommendations()
})
</script>

<style scoped>
/* Add any custom styles if needed */
</style>
