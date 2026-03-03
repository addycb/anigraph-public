<template>
  <v-container>
    <!-- Loading State -->
    <v-row v-if="loading || autoComputing" justify="center">
      <v-col cols="12" class="text-center">
        <v-progress-circular indeterminate color="primary" size="64" />
        <p class="mt-4">{{ autoComputing ? 'Computing your taste profile...' : 'Analyzing your taste...' }}</p>
      </v-col>
    </v-row>

    <!-- No Profile Yet -->
    <v-row v-else-if="!profile" justify="center">
      <v-col cols="12" md="8" class="text-center">
        <v-icon size="80" color="grey">mdi-chart-box-outline</v-icon>
        <h2 class="mt-4">Discover Your Anime DNA</h2>
        <p class="text-body-1 mt-2">
          Favorite at least 3 anime to unlock your personalized taste profile
          and discover patterns in your preferences.
        </p>
        <v-btn
          color="primary"
          size="large"
          class="mt-4"
          @click="$emit('start-favoriting')"
        >
          Start Favoriting Anime
        </v-btn>
      </v-col>
    </v-row>

    <!-- Taste Profile Display -->
    <v-row v-else>
      <!-- Summary Card -->
      <v-col cols="12">
        <v-card elevation="2">
          <v-card-title class="text-h5">
            <v-icon start>mdi-account-star</v-icon>
            Your Anime DNA
          </v-card-title>
          <v-card-text>
            <div class="d-flex align-center mb-4">
              <v-avatar color="primary" size="64" class="mr-4">
                <span class="text-h6">{{ profile.totalFavorites }}</span>
              </v-avatar>
              <div>
                <div class="text-h6">{{ profile.tasteSummary }}</div>
                <div class="text-caption">
                  Based on {{ profile.totalFavorites }} favorites
                </div>
              </div>
            </div>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Discovered Patterns -->
      <v-col cols="12" v-if="profile.hiddenPatterns && profile.hiddenPatterns.length > 0">
        <v-card elevation="2">
          <v-card-title class="text-h6">
            <v-icon start>mdi-lightbulb-on</v-icon>
            Patterns Discovered
          </v-card-title>
          <v-card-text>
            <v-expansion-panels variant="accordion">
              <v-expansion-panel
                v-for="(pattern, idx) in profile.hiddenPatterns"
                :key="idx"
              >
                <v-expansion-panel-title>
                  <div class="d-flex align-center justify-space-between w-100 pr-4">
                    <span>{{ pattern.pattern }}</span>
                    <v-chip
                      :color="getConfidenceColor(pattern.confidence)"
                      size="small"
                      label
                    >
                      {{ pattern.confidence }}% confidence
                    </v-chip>
                  </div>
                </v-expansion-panel-title>
                <v-expansion-panel-text>
                  <div class="mb-2">
                    <strong>Evidence:</strong>
                    <ul class="ml-4 mt-1">
                      <li v-for="(evidence, eidx) in pattern.evidence" :key="eidx">
                        {{ evidence }}
                      </li>
                    </ul>
                  </div>
                  <div class="mt-3">
                    <v-alert type="info" density="compact" variant="tonal">
                      {{ pattern.insight }}
                    </v-alert>
                  </div>
                </v-expansion-panel-text>
              </v-expansion-panel>
            </v-expansion-panels>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Preferred Genres -->
      <v-col cols="12" md="6">
        <v-card elevation="2">
          <v-card-title class="text-h6">
            <v-icon start>mdi-tag-multiple</v-icon>
            Your Top Genres
          </v-card-title>
          <v-card-text>
            <v-chip
              v-for="genre in profile.preferredGenres"
              :key="genre"
              class="ma-1"
              color="primary"
              variant="outlined"
            >
              {{ genre }}
            </v-chip>
            <div v-if="!profile.preferredGenres || profile.preferredGenres.length === 0" class="text-caption text-grey">
              No genre preferences detected yet
            </div>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Preferred Era -->
      <v-col cols="12" md="6">
        <v-card elevation="2">
          <v-card-title class="text-h6">
            <v-icon start>mdi-calendar-range</v-icon>
            Preferred Era
          </v-card-title>
          <v-card-text>
            <div class="text-h4 text-primary">{{ profile.preferredEra }}</div>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Top Staff -->
      <v-col cols="12" v-if="profile.topStaff && profile.topStaff.length > 0">
        <v-card elevation="2">
          <v-card-title class="text-h6">
            <v-icon start>mdi-account-group</v-icon>
            Creators You Love
          </v-card-title>
          <v-card-text>
            <v-list>
              <v-list-item
                v-for="staff in profile.topStaff"
                :key="staff.id"
                :to="`/staff/${staff.anilist_id}`"
              >
                <template v-slot:prepend>
                  <v-avatar v-if="staff.image_medium">
                    <v-img :src="staff.image_medium" />
                  </v-avatar>
                  <v-avatar v-else color="grey">
                    <v-icon>mdi-account</v-icon>
                  </v-avatar>
                </template>
                <v-list-item-title>{{ staff.name_en }}</v-list-item-title>
                <v-list-item-subtitle v-if="staff.primary_occupations">
                  {{ staff.primary_occupations.join(', ') }}
                </v-list-item-subtitle>
              </v-list-item>
            </v-list>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Top Studios -->
      <v-col cols="12" v-if="profile.topStudios && profile.topStudios.length > 0">
        <v-card elevation="2">
          <v-card-title class="text-h6">
            <v-icon start>mdi-office-building</v-icon>
            Your Favorite Studios
          </v-card-title>
          <v-card-text>
            <v-chip
              v-for="studio in profile.topStudios"
              :key="studio.name"
              class="ma-1"
              color="secondary"
              variant="outlined"
              :to="`/studio/${encodeURIComponent(studio.name)}`"
            >
              {{ studio.name }}
            </v-chip>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Refresh Button -->
      <v-col cols="12" class="text-center">
        <v-btn
          color="primary"
          variant="outlined"
          @click="refreshProfile"
          :loading="refreshing"
        >
          <v-icon start>mdi-refresh</v-icon>
          Refresh Profile
        </v-btn>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/utils/api'
import { useCsrf } from '@/composables/useCsrf'

const { csrfPost } = useCsrf();

interface HiddenPattern {
  pattern: string;
  confidence: number;
  evidence: string[];
  insight: string;
  surprise: 'high' | 'medium' | 'low';
}

interface TasteProfileData {
  tasteSummary: string;
  totalFavorites: number;
  preferredGenres: string[];
  preferredTags: string[];
  preferredEra: string;
  hiddenPatterns: HiddenPattern[];
  topStaff: any[];
  topStudios: any[];
  lastComputed?: string;
}

const props = defineProps<{
  profileData?: TasteProfileData | null;
  showTitle?: boolean;
}>();

const emit = defineEmits(['start-favoriting']);

const loading = ref(false);
const refreshing = ref(false);
const fetchedProfile = ref<TasteProfileData | null>(null);
const autoComputing = ref(false);
const favoritesCount = ref(0);

const profile = computed(() => props.profileData || fetchedProfile.value);

const loadProfile = async () => {
  loading.value = true;
  try {
    const response = await api('/user/taste-profile');
    if (response.exists) {
      fetchedProfile.value = response.profile;
    } else {
      // Check if user has favorites and auto-compute if they do
      await checkAndAutoCompute();
    }
  } catch (error) {
    console.error('Error loading taste profile:', error);
  } finally {
    loading.value = false;
  }
};

const checkAndAutoCompute = async () => {
  try {
    // Get favorites count from user_lists (which includes favorites list)
    const listsResponse = await api('/user/lists');
    if (listsResponse.success && listsResponse.data) {
      const favoritesList = listsResponse.data.find((list: any) => list.listType === 'favorites');
      if (favoritesList && favoritesList.itemCount >= 3) {
        favoritesCount.value = favoritesList.itemCount;
        // Auto-compute the profile
        console.log('Auto-computing taste profile with', favoritesCount.value, 'favorites...');
        autoComputing.value = true;
        try {
          await refreshProfile();
        } catch (err) {
          console.error('Error auto-computing profile:', err);
        }
        autoComputing.value = false;
      } else {
        console.log('Not enough favorites to compute profile. Count:', favoritesList?.itemCount || 0);
      }
    }
  } catch (error) {
    console.error('Error checking favorites:', error);
    autoComputing.value = false;
  }
};

const refreshProfile = async () => {
  refreshing.value = true;
  try {
    await csrfPost('/api/user/compute-taste-profile');
    await loadProfile();
  } catch (error) {
    console.error('Error refreshing profile:', error);
  } finally {
    refreshing.value = false;
  }
};

const getConfidenceColor = (confidence: number) => {
  if (confidence >= 80) return 'success';
  if (confidence >= 60) return 'info';
  if (confidence >= 40) return 'warning';
  return 'grey';
};

onMounted(() => {
  // Only load user's profile if no profile prop was provided
  if (!props.profileData) {
    loadProfile();
  }
});
</script>
