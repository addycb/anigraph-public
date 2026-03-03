<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container fluid>
        <!-- Page Header -->
        <ViewToolbar>
          <template #left>
            <div class="page-title-section">
              <div class="d-flex align-center">
                <v-icon color="primary" size="28" class="mr-2">mdi-chart-box</v-icon>
                <h1 class="text-h5 font-weight-bold mb-0">Your Taste Profile</h1>
              </div>
              <p class="text-caption text-medium-emphasis ml-9 mb-0">
                Discover the patterns in your anime preferences
              </p>
            </div>
          </template>
        </ViewToolbar>

        <!-- Not Logged In State -->
        <v-row v-if="!isAuthenticated">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="grey">mdi-account-lock</v-icon>
            <h2 class="text-h4 mt-4">Login Required</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              Please log in with Google to view your taste profile
            </p>
            <AuthButton class="mt-4" />
          </v-col>
        </v-row>

        <!-- Taste Profile Component -->
        <TasteProfile
          v-else
          @start-favoriting="navigateToFavorites"
        />
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import TasteProfile from '@/components/TasteProfile.vue'

const { isAuthenticated } = useAuth()
const router = useRouter()

const navigateToFavorites = () => {
  router.push('/favorites')
}
</script>

<style scoped>
/* Add any custom styles if needed */
</style>
