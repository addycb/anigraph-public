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
                <v-icon color="primary" size="28" class="mr-2">mdi-heart</v-icon>
                <h1 class="text-h5 font-weight-bold mb-0">Favorites & Lists</h1>
              </div>
              <p class="text-caption text-medium-emphasis ml-9 mb-0">
                Your curated collections of anime
              </p>
            </div>
          </template>
          <template v-if="isAuthenticated" #right>
            <v-btn
              color="primary"
              variant="flat"
              size="small"
              @click="showCreateListDialog = true"
            >
              <v-icon start>mdi-plus</v-icon>
              Create List
            </v-btn>
          </template>
        </ViewToolbar>

        <!-- Not Logged In State -->
        <v-row v-if="!isAuthenticated">
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="grey">mdi-account-lock</v-icon>
            <h2 class="text-h4 mt-4">Login Required</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              Please log in with Google to access your favorites and lists
            </p>
            <AuthButton class="mt-4" />
          </v-col>
        </v-row>

        <!-- Tabs for Favorites and Lists -->
        <template v-else>
          <v-tabs
            v-model="activeTab"
            color="primary"
            class="mb-6"
          >
            <v-tab value="favorites">
              <v-icon start>mdi-heart</v-icon>
              Favorites
              <v-chip v-if="favoritesList" size="small" class="ml-2">
                {{ favoritesList.itemCount || 0 }}
              </v-chip>
            </v-tab>
            <v-tab
              v-for="list in customLists"
              :key="list.id"
              :value="`list-${list.id}`"
            >
              <v-icon start>mdi-bookmark</v-icon>
              {{ list.name }}
              <v-chip v-if="list.itemCount !== undefined" size="small" class="ml-2">
                {{ list.itemCount }}
              </v-chip>
            </v-tab>
          </v-tabs>

          <v-window v-model="activeTab">
            <!-- Favorites Tab -->
            <v-window-item value="favorites">
              <div v-if="favoritesList" class="mb-4 d-flex flex-column flex-sm-row align-start align-sm-center justify-space-between ga-2">
                <div>
                  <h2 class="text-h6 text-sm-h5">{{ favoritesList.name }}</h2>
                  <p class="text-body-2 text-medium-emphasis">
                    {{ favoritesList.description }}
                  </p>
                </div>
                <div class="d-flex ga-1 flex-wrap">
                  <!-- Privacy Toggle -->
                  <v-btn
                    :color="favoritesList.isPublic ? 'success' : 'grey'"
                    variant="tonal"
                    size="small"
                    @click="toggleListPrivacy(favoritesList)"
                  >
                    <v-icon start>{{ favoritesList.isPublic ? 'mdi-earth' : 'mdi-lock' }}</v-icon>
                    <span class="d-none d-sm-inline">{{ favoritesList.isPublic ? 'Public' : 'Private' }}</span>
                  </v-btn>

                  <!-- Share Button -->
                  <v-btn
                    v-if="favoritesList.isPublic && favoritesList.shareToken"
                    variant="tonal"
                    size="small"
                    @click="copyShareLink(favoritesList)"
                  >
                    <v-icon start>mdi-share-variant</v-icon>
                    <span class="d-none d-sm-inline">Share</span>
                  </v-btn>

                  <!-- Add Anime Button -->
                  <v-btn
                    color="secondary"
                    variant="tonal"
                    size="small"
                    @click="openAddAnimeDialog(favoritesList)"
                  >
                    <v-icon start>mdi-plus</v-icon>
                    <span class="d-none d-sm-inline">Add Anime</span>
                  </v-btn>
                </div>
              </div>

              <!-- Loading State -->
              <v-row v-if="loadingListItems[favoritesList?.id]">
                <v-col cols="12" class="text-center py-12">
                  <v-progress-circular
                    indeterminate
                    color="primary"
                    size="64"
                  ></v-progress-circular>
                  <p class="text-h6 mt-4">Loading your favorites...</p>
                </v-col>
              </v-row>

              <!-- Results List with Selection -->
              <div v-if="favoritesList && listItems[favoritesList.id] && listItems[favoritesList.id].length > 0">
                <!-- Selection Toolbar -->
                <v-toolbar density="compact" color="transparent" flat class="mb-4">
                  <v-checkbox
                    v-model="selectAllFavorites"
                    @update:model-value="toggleSelectAllFavorites"
                    hide-details
                    density="compact"
                    class="mr-2"
                  >
                    <template v-slot:label>
                      <span class="text-body-2">
                        {{ selectedFavorites.size === 0 ? 'Select All' : `${selectedFavorites.size} selected` }}
                      </span>
                    </template>
                  </v-checkbox>
                  <v-spacer></v-spacer>
                  <v-btn
                    v-if="selectedFavorites.size > 0"
                    color="error"
                    variant="flat"
                    @click="removeSelectedFavorites"
                    :loading="removingItems"
                  >
                    <v-icon start>mdi-delete</v-icon>
                    Remove Selected ({{ selectedFavorites.size }})
                  </v-btn>
                </v-toolbar>
                <v-list lines="two" class="anime-list">
                  <v-list-item
                    v-for="anime in listItems[favoritesList.id]"
                    :key="anime.id"
                    class="anime-list-item"
                    @click.stop="navigateToAnime(anime.anilistId)"
                  >
                    <template v-slot:prepend>
                      <v-checkbox
                        :model-value="selectedFavorites.has(anime.anilistId)"
                        @update:model-value="toggleSelection(selectedFavorites, anime.anilistId)"
                        @click.stop
                        hide-details
                        density="compact"
                        class="mr-2"
                      ></v-checkbox>
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
                  </v-list-item>
                </v-list>
              </div>

              <!-- Empty State -->
              <v-row v-else>
                <v-col cols="12" class="text-center py-12">
                  <v-icon size="80" color="grey">mdi-heart-outline</v-icon>
                  <h2 class="text-h4 mt-4">No favorites yet</h2>
                  <p class="text-h6 text-medium-emphasis mt-2">
                    Start favoriting anime to build your collection
                  </p>
                  <v-btn
                    color="primary"
                    size="large"
                    class="mt-4"
                    to="/home"
                  >
                    <v-icon start>mdi-compass</v-icon>
                    Browse Anime
                  </v-btn>
                </v-col>
              </v-row>
            </v-window-item>

            <!-- Custom List Tabs -->
            <v-window-item
              v-for="list in customLists"
              :key="list.id"
              :value="`list-${list.id}`"
            >
              <div class="mb-4 d-flex flex-column flex-sm-row align-start align-sm-center justify-space-between ga-2">
                <div>
                  <h2 class="text-h6 text-sm-h5">{{ list.name }}</h2>
                  <p v-if="list.description" class="text-body-2 text-sm-body-1 text-medium-emphasis">
                    {{ list.description }}
                  </p>
                </div>
                <div class="d-flex ga-1 flex-wrap">
                  <!-- Taste Profile Button -->
                  <v-btn
                    variant="tonal"
                    color="primary"
                    size="small"
                    @click="viewTasteProfile(list)"
                    :loading="loadingTasteProfile && currentAnalysisListId === list.id"
                  >
                    <v-icon start>mdi-chart-box</v-icon>
                    <span class="d-none d-sm-inline">Taste Profile</span>
                  </v-btn>

                  <!-- Recommendations Button -->
                  <v-btn
                    variant="tonal"
                    color="secondary"
                    size="small"
                    @click="viewRecommendations(list)"
                    :loading="loadingRecommendations && currentAnalysisListId === list.id"
                  >
                    <v-icon start>mdi-lightbulb</v-icon>
                    <span class="d-none d-sm-inline">Recommendations</span>
                  </v-btn>

                  <!-- Privacy Toggle -->
                  <v-btn
                    :color="list.isPublic ? 'success' : 'grey'"
                    variant="tonal"
                    size="small"
                    @click="toggleListPrivacy(list)"
                  >
                    <v-icon start>{{ list.isPublic ? 'mdi-earth' : 'mdi-lock' }}</v-icon>
                    <span class="d-none d-sm-inline">{{ list.isPublic ? 'Public' : 'Private' }}</span>
                  </v-btn>

                  <!-- Share Button -->
                  <v-btn
                    v-if="list.isPublic && list.shareToken"
                    variant="tonal"
                    size="small"
                    @click="copyShareLink(list)"
                  >
                    <v-icon start>mdi-share-variant</v-icon>
                    <span class="d-none d-sm-inline">Share</span>
                  </v-btn>

                  <!-- Add Anime Button -->
                  <v-btn
                    color="secondary"
                    variant="tonal"
                    size="small"
                    @click="openAddAnimeDialog(list)"
                  >
                    <v-icon start>mdi-plus</v-icon>
                    <span class="d-none d-sm-inline">Add Anime</span>
                  </v-btn>

                  <!-- Edit Button -->
                  <v-btn
                    variant="tonal"
                    size="small"
                    @click="editList(list)"
                  >
                    <v-icon>mdi-pencil</v-icon>
                  </v-btn>

                  <!-- Delete Button -->
                  <v-btn
                    color="error"
                    variant="tonal"
                    size="small"
                    @click="confirmDeleteList(list)"
                  >
                    <v-icon>mdi-delete</v-icon>
                  </v-btn>
                </div>
              </div>

              <!-- Loading State -->
              <v-row v-if="loadingListItems[list.id]">
                <v-col cols="12" class="text-center py-12">
                  <v-progress-circular
                    indeterminate
                    color="primary"
                    size="64"
                  ></v-progress-circular>
                  <p class="text-h6 mt-4">Loading list items...</p>
                </v-col>
              </v-row>

              <!-- Results List with Selection -->
              <div v-if="listItems[list.id] && listItems[list.id].length > 0">
                <!-- Selection Toolbar -->
                <v-toolbar density="compact" color="transparent" flat class="mb-4">
                  <v-checkbox
                    :model-value="getSelectAllState(list.id)"
                    @update:model-value="toggleSelectAllForList(list.id)"
                    hide-details
                    density="compact"
                    class="mr-2"
                  >
                    <template v-slot:label>
                      <span class="text-body-2">
                        {{ getSelectedCount(list.id) === 0 ? 'Select All' : `${getSelectedCount(list.id)} selected` }}
                      </span>
                    </template>
                  </v-checkbox>
                  <v-spacer></v-spacer>
                  <v-btn
                    v-if="getSelectedCount(list.id) > 0"
                    color="error"
                    variant="flat"
                    @click="removeSelectedFromList(list.id)"
                    :loading="removingItems"
                  >
                    <v-icon start>mdi-delete</v-icon>
                    Remove Selected ({{ getSelectedCount(list.id) }})
                  </v-btn>
                </v-toolbar>
                <v-list lines="two" class="anime-list">
                  <v-list-item
                    v-for="anime in listItems[list.id]"
                    :key="anime.id"
                    class="anime-list-item"
                    @click.stop="navigateToAnime(anime.anilistId)"
                  >
                    <template v-slot:prepend>
                      <v-checkbox
                        :model-value="isSelected(list.id, anime.anilistId)"
                        @update:model-value="toggleSelectionForList(list.id, anime.anilistId)"
                        @click.stop
                        hide-details
                        density="compact"
                        class="mr-2"
                      ></v-checkbox>
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
                  </v-list-item>
                </v-list>
              </div>

              <!-- Empty State -->
              <v-row v-else>
                <v-col cols="12" class="text-center py-12">
                  <v-icon size="80" color="grey">mdi-bookmark-outline</v-icon>
                  <h2 class="text-h4 mt-4">No items in this list</h2>
                  <p class="text-h6 text-medium-emphasis mt-2">
                    Start adding anime to this list
                  </p>
                </v-col>
              </v-row>
            </v-window-item>
          </v-window>
        </template>
      </v-container>
    </v-main>

    <!-- Create List Dialog -->
    <v-dialog v-model="showCreateListDialog" max-width="500">
      <v-card>
        <v-card-title>Create New List</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="newListName"
            label="List Name"
            variant="outlined"
            density="comfortable"
            autofocus
          ></v-text-field>
          <v-textarea
            v-model="newListDescription"
            label="Description (optional)"
            variant="outlined"
            density="comfortable"
            rows="3"
          ></v-textarea>
          <v-checkbox
            v-model="newListIsPublic"
            label="Make this list public"
            hide-details
          ></v-checkbox>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showCreateListDialog = false">Cancel</v-btn>
          <v-btn
            color="primary"
            variant="flat"
            @click="createNewList"
            :loading="creatingList"
            :disabled="!newListName.trim()"
          >
            Create
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Edit List Dialog -->
    <v-dialog v-model="showEditListDialog" max-width="500">
      <v-card v-if="editingList">
        <v-card-title>Edit List</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="editListName"
            label="List Name"
            variant="outlined"
            density="comfortable"
          ></v-text-field>
          <v-textarea
            v-model="editListDescription"
            label="Description (optional)"
            variant="outlined"
            density="comfortable"
            rows="3"
          ></v-textarea>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showEditListDialog = false">Cancel</v-btn>
          <v-btn
            color="primary"
            variant="flat"
            @click="saveListEdits"
            :loading="savingList"
            :disabled="!editListName.trim()"
          >
            Save
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Delete Confirmation Dialog -->
    <v-dialog v-model="showDeleteDialog" max-width="400">
      <v-card v-if="deletingList">
        <v-card-title>Delete List?</v-card-title>
        <v-card-text>
          Are you sure you want to delete "{{ deletingList.name }}"? This action cannot be undone.
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showDeleteDialog = false">Cancel</v-btn>
          <v-btn
            color="error"
            variant="flat"
            @click="deleteListConfirmed"
            :loading="deletingListLoading"
          >
            Delete
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Make Public Confirmation Dialog -->
    <v-dialog v-model="showMakePublicDialog" max-width="500">
      <v-card v-if="makingPublicList">
        <v-card-title>Make List Public?</v-card-title>
        <v-card-text>
          <p class="mb-3">
            Are you sure you want to make "{{ makingPublicList.name }}" public?
          </p>
          <v-alert type="info" variant="tonal" density="compact">
            <div class="text-body-2">
              <strong>Public lists:</strong>
              <ul class="ml-4 mt-1">
                <li>Can be viewed by anyone with the share link</li>
                <li>Will be visible in the public lists discovery page</li>
              </ul>
            </div>
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showMakePublicDialog = false">Cancel</v-btn>
          <v-btn
            color="success"
            variant="flat"
            @click="makePublicConfirmed"
            :loading="togglingPrivacy"
          >
            Make Public
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Add Anime to List Dialog -->
    <v-dialog v-model="showAddAnimeDialog" max-width="800">
      <v-card v-if="addingToList">
        <v-card-title>Add Anime to "{{ addingToList.name }}"</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="addAnimeSearch"
            placeholder="Search anime..."
            variant="outlined"
            density="comfortable"
            clearable
            prepend-inner-icon="mdi-magnify"
            autofocus
            @update:model-value="debouncedAddAnimeSearch"
            class="mb-4"
          ></v-text-field>

          <!-- Search Results -->
          <div v-if="addAnimeSearchLoading" class="text-center py-4">
            <v-progress-circular indeterminate color="primary"></v-progress-circular>
          </div>

          <div v-else-if="addAnimeSearchResults.length > 0" style="max-height: 400px; overflow-y: auto;">
            <v-row>
              <v-col
                v-for="anime in addAnimeSearchResults"
                :key="anime.anilistId"
                cols="6"
                sm="4"
                md="3"
              >
                <v-card
                  class="anime-search-card"
                  hover
                  @click="addAnimeToList(anime)"
                  :class="{ 'already-in-list': isInAddingList(anime.anilistId) }"
                >
                  <v-img
                    :src="anime.coverImage_large || anime.coverImage || '/placeholder-anime.jpg'"
                    aspect-ratio="0.7"
                    cover
                  >
                    <div v-if="isInAddingList(anime.anilistId)" class="already-added-overlay">
                      <v-icon size="48" color="success">mdi-check-circle</v-icon>
                    </div>
                  </v-img>
                  <v-card-title class="text-caption pa-2">
                    {{ anime.title }}
                  </v-card-title>
                </v-card>
              </v-col>
            </v-row>
          </div>

          <div v-else-if="addAnimeSearch" class="text-center py-4 text-medium-emphasis">
            No anime found
          </div>

          <div v-else class="text-center py-4 text-medium-emphasis">
            <v-icon size="48" color="grey">mdi-magnify</v-icon>
            <p class="mt-2">Search for anime to add to this list</p>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showAddAnimeDialog = false">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <Snackbar />

    <!-- Taste Profile Dialog -->
    <v-dialog v-model="showTasteProfileDialog" max-width="900">
      <v-card v-if="tasteProfile">
        <v-card-title>
          <v-icon start color="primary">mdi-chart-box</v-icon>
          Taste Profile for "{{ currentAnalysisList?.name }}"
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
          Recommendations Based on "{{ currentAnalysisList?.name }}"
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
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '~/composables/useAuth'
import { useLists } from '~/composables/useLists'
import { useSettings } from '~/composables/useSettings'
import { useCsrf } from '~/composables/useCsrf'
import { useSnackbar } from '~/composables/useSnackbar'
import { api } from '~/utils/api'

const { isAuthenticated, fetchUser } = useAuth()
const { userLists, fetchLists, createList, updateList, deleteList: deleteListApi, addToList, removeFromList } = useLists()
const { includeAdult } = useSettings()
const { csrfPost } = useCsrf()
const router = useRouter()

const activeTab = ref('favorites')

// Selection state
const selectedFavorites = ref<Set<number>>(new Set())
const selectedByList = ref<Record<number, Set<number>>>({})
const selectAllFavorites = ref(false)
const removingItems = ref(false)

// Separate favorites list from custom lists
const favoritesList = computed(() =>
  userLists.value.find(list => list.listType === 'favorites')
)

const customLists = computed(() =>
  userLists.value.filter(list => list.listType !== 'favorites')
)

// Lists state
const listItems = ref<Record<number, any[]>>({})
const loadingListItems = ref<Record<number, boolean>>({})

// Create list state
const showCreateListDialog = ref(false)
const newListName = ref('')
const newListDescription = ref('')
const newListIsPublic = ref(false)
const creatingList = ref(false)

// Edit list state
const showEditListDialog = ref(false)
const editingList = ref<any>(null)
const editListName = ref('')
const editListDescription = ref('')
const savingList = ref(false)

// Delete list state
const showDeleteDialog = ref(false)
const deletingList = ref<any>(null)
const deletingListLoading = ref(false)

// Make public state
const showMakePublicDialog = ref(false)
const makingPublicList = ref<any>(null)
const togglingPrivacy = ref(false)

// Add anime to list state
const showAddAnimeDialog = ref(false)
const addingToList = ref<any>(null)
const addAnimeSearch = ref('')
const addAnimeSearchResults = ref<any[]>([])
const addAnimeSearchLoading = ref(false)

const snackbar = useSnackbar()

// Taste profile and recommendations state
const showTasteProfileDialog = ref(false)
const showRecommendationsDialog = ref(false)
const loadingTasteProfile = ref(false)
const loadingRecommendations = ref(false)
const tasteProfile = ref<any>(null)
const recommendations = ref<any[]>([])
const currentAnalysisList = ref<any>(null)
const currentAnalysisListId = ref<number | null>(null)

const createNewList = async () => {
  if (!newListName.value.trim()) return

  creatingList.value = true
  try {
    const list = await createList(
      newListName.value.trim(),
      newListDescription.value.trim() || undefined,
      newListIsPublic.value
    )

    newListName.value = ''
    newListDescription.value = ''
    newListIsPublic.value = false
    showCreateListDialog.value = false

    // Switch to the new list tab
    activeTab.value = `list-${list.id}`
  } catch (error: any) {
    snackbar.showError(error.data?.message || error.message || 'Failed to create list')
  } finally {
    creatingList.value = false
  }
}

const editList = (list: any) => {
  editingList.value = list
  editListName.value = list.name
  editListDescription.value = list.description || ''
  showEditListDialog.value = true
}

const saveListEdits = async () => {
  if (!editingList.value || !editListName.value.trim()) return

  savingList.value = true
  try {
    await updateList(editingList.value.id, {
      name: editListName.value.trim(),
      description: editListDescription.value.trim() || undefined
    })

    showEditListDialog.value = false
    editingList.value = null
  } catch (error: any) {
    snackbar.showError(error.data?.message || error.message || 'Failed to update list')
  } finally {
    savingList.value = false
  }
}

const toggleListPrivacy = (list: any) => {
  // If making public, show confirmation dialog
  if (!list.isPublic) {
    makingPublicList.value = list
    showMakePublicDialog.value = true
  } else {
    // If making private, no confirmation needed
    makePrivate(list)
  }
}

const makePublicConfirmed = async () => {
  if (!makingPublicList.value) return

  togglingPrivacy.value = true
  try {
    await updateList(makingPublicList.value.id, {
      isPublic: true
    })

    showMakePublicDialog.value = false
    makingPublicList.value = null
  } catch (error: any) {
    snackbar.showError(error.data?.message || error.message || 'Failed to make list public')
  } finally {
    togglingPrivacy.value = false
  }
}

const makePrivate = async (list: any) => {
  try {
    await updateList(list.id, {
      isPublic: false
    })
  } catch (error: any) {
    snackbar.showError(error.data?.message || error.message || 'Failed to make list private')
  }
}

const copyShareLink = (list: any) => {
  const url = `${window.location.origin}/lists/${list.shareToken}`
  navigator.clipboard.writeText(url)
  snackbar.showSuccess('Share link copied to clipboard!')
}

const confirmDeleteList = (list: any) => {
  deletingList.value = list
  showDeleteDialog.value = true
}

const deleteListConfirmed = async () => {
  if (!deletingList.value) return

  deletingListLoading.value = true
  const success = await deleteListApi(deletingList.value.id)
  deletingListLoading.value = false

  if (success) {
    showDeleteDialog.value = false
    deletingList.value = null

    // Switch back to favorites tab
    activeTab.value = 'favorites'
  }
}

const exportList = (list: any) => {
  const items = listItems.value[list.id] || []
  const data = items.map(anime => ({
    id: anime.anilistId,
    title: anime.title,
    format: anime.format,
    seasonYear: anime.seasonYear,
    averageScore: anime.averageScore
  }))

  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `anigraph-${list.name.toLowerCase().replace(/\s+/g, '-')}-${Date.now()}.json`
  a.click()
  URL.revokeObjectURL(url)
}

// Add anime to list functions
const openAddAnimeDialog = (list: any) => {
  addingToList.value = list
  addAnimeSearch.value = ''
  addAnimeSearchResults.value = []
  showAddAnimeDialog.value = true
}

let addAnimeSearchTimeout: ReturnType<typeof setTimeout> | null = null
const debouncedAddAnimeSearch = () => {
  if (addAnimeSearchTimeout) clearTimeout(addAnimeSearchTimeout)
  addAnimeSearchTimeout = setTimeout(() => {
    performAddAnimeSearch()
  }, 300)
}

const performAddAnimeSearch = async () => {
  if (!addAnimeSearch.value || addAnimeSearch.value.length < 2) {
    addAnimeSearchResults.value = []
    return
  }

  addAnimeSearchLoading.value = true

  try {
    const response = await api<any>('/anime/search', {
      params: {
        q: addAnimeSearch.value,
        limit: 24,
        includeAdult: includeAdult.value
      }
    })

    if (response.success && response.data) {
      addAnimeSearchResults.value = response.data
    }
  } catch (error) {
    console.error('Error searching anime:', error)
    addAnimeSearchResults.value = []
  } finally {
    addAnimeSearchLoading.value = false
  }
}

const isInAddingList = (animeId: number | string) => {
  if (!addingToList.value) return false
  return addingToList.value.items?.includes(parseInt(String(animeId))) || false
}

const addAnimeToList = async (anime: any) => {
  if (!addingToList.value || isInAddingList(anime.anilistId)) return

  const success = await addToList(addingToList.value.id, anime.anilistId)

  if (success) {
    // Refresh the list items
    await fetchListItems(addingToList.value.id)
  }
}

const fetchListItems = async (listId: number) => {
  loadingListItems.value[listId] = true

  try {
    const response = await api<any>(`/user/lists/${listId}/items`, {
      params: {
        includeAdult: includeAdult.value
      }
    })

    if (response.success) {
      listItems.value[listId] = response.data
    }
  } catch (error) {
    console.error('Error fetching list items:', error)
  } finally {
    loadingListItems.value[listId] = false
  }
}

// Selection functions
const toggleSelection = (selectedSet: Set<number>, animeId: number) => {
  if (selectedSet.has(animeId)) {
    selectedSet.delete(animeId)
  } else {
    selectedSet.add(animeId)
  }
}

const toggleSelectAllFavorites = (value: boolean) => {
  if (!favoritesList.value) return
  const items = listItems.value[favoritesList.value.id] || []

  if (value) {
    selectedFavorites.value = new Set(items.map((a: any) => a.anilistId))
  } else {
    selectedFavorites.value.clear()
  }
  selectAllFavorites.value = value
}

const toggleSelectAllForList = (listId: number) => {
  const items = listItems.value[listId] || []

  if (!selectedByList.value[listId]) {
    selectedByList.value[listId] = new Set()
  }

  const currentCount = selectedByList.value[listId].size

  if (currentCount === items.length) {
    selectedByList.value[listId].clear()
  } else {
    selectedByList.value[listId] = new Set(items.map((a: any) => a.anilistId))
  }
}

const toggleSelectionForList = (listId: number, animeId: number) => {
  if (!selectedByList.value[listId]) {
    selectedByList.value[listId] = new Set()
  }
  toggleSelection(selectedByList.value[listId], animeId)
}

const isSelected = (listId: number, animeId: number) => {
  return selectedByList.value[listId]?.has(animeId) || false
}

const getSelectedCount = (listId: number) => {
  return selectedByList.value[listId]?.size || 0
}

const getSelectAllState = (listId: number) => {
  const items = listItems.value[listId] || []
  const selected = selectedByList.value[listId]?.size || 0
  return selected > 0 && selected === items.length
}

const navigateToAnime = (animeId: number) => {
  router.push(`/anime/${animeId}`)
}

// Remove functions
const removeSelectedFavorites = async () => {
  if (!favoritesList.value || selectedFavorites.value.size === 0) return

  removingItems.value = true
  const listId = favoritesList.value.id
  const toRemove = Array.from(selectedFavorites.value)

  try {
    // Remove all selected items
    await Promise.all(toRemove.map(animeId => removeFromList(listId, animeId)))

    // Refresh the list
    await fetchListItems(listId)

    // Clear selection
    selectedFavorites.value.clear()
    selectAllFavorites.value = false
  } catch (error) {
    console.error('Error removing items:', error)
    snackbar.showError('Failed to remove some items')
  } finally {
    removingItems.value = false
  }
}

const removeSelectedFromList = async (listId: number) => {
  if (!selectedByList.value[listId] || selectedByList.value[listId].size === 0) return

  removingItems.value = true
  const toRemove = Array.from(selectedByList.value[listId])

  try {
    // Remove all selected items
    await Promise.all(toRemove.map(animeId => removeFromList(listId, animeId)))

    // Refresh the list
    await fetchListItems(listId)

    // Clear selection
    selectedByList.value[listId].clear()
  } catch (error) {
    console.error('Error removing items:', error)
    snackbar.showError('Failed to remove some items')
  } finally {
    removingItems.value = false
  }
}

// Watch active tab to load list items when needed
watch(activeTab, (newTab) => {
  if (newTab === 'favorites' && favoritesList.value && !listItems.value[favoritesList.value.id]) {
    fetchListItems(favoritesList.value.id)
  } else if (newTab.startsWith('list-')) {
    const listId = parseInt(newTab.replace('list-', ''))
    if (!listItems.value[listId]) {
      fetchListItems(listId)
    }
  }
})

// Watch includeAdult setting changes to refetch lists and items
watch(() => includeAdult.value, async (newValue) => {
  if (!isAuthenticated.value) return

  // Refetch lists to get updated counts with the new includeAdult value
  await fetchLists(true, newValue) // force refetch with explicit includeAdult value

  // Refetch items for the current active tab
  if (activeTab.value === 'favorites' && favoritesList.value) {
    await fetchListItems(favoritesList.value.id)
  } else if (activeTab.value.startsWith('list-')) {
    const listId = parseInt(activeTab.value.replace('list-', ''))
    await fetchListItems(listId)
  }
})

// Taste profile and recommendations functions
const viewTasteProfile = async (list: any) => {
  const items = listItems.value[list.id]
  if (!items || items.length === 0) {
    snackbar.showError('This list is empty. Add some anime first!')
    return
  }

  currentAnalysisList.value = list
  currentAnalysisListId.value = list.id
  loadingTasteProfile.value = true

  try {
    // Compute taste profile based on list items
    const animeIds = items.map((item: any) => item.anilistId)
    const response = await csrfPost<any>('/api/compute-list-taste-profile', {
      animeIds,
      listId: list.id  // Pass list ID to enable database storage
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
    currentAnalysisListId.value = null
  }
}

const viewRecommendations = async (list: any) => {
  const items = listItems.value[list.id]
  if (!items || items.length === 0) {
    snackbar.showError('This list is empty. Add some anime first!')
    return
  }
  if (items.length < 3) {
    snackbar.showError(`Need at least 3 anime in this list to compute recommendations (${items.length}/3)`)
    return
  }

  currentAnalysisList.value = list
  currentAnalysisListId.value = list.id
  loadingRecommendations.value = true

  try {
    // Get recommendations based on list items
    const animeIds = items.map((item: any) => item.anilistId)
    const response = await csrfPost<any>('/api/compute-list-recommendations', {
      animeIds,
      listId: list.id,  // Pass list ID to enable database storage
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
    currentAnalysisListId.value = null
  }
}

// Load lists when authenticated
const loadUserLists = async () => {
  if (!isAuthenticated.value) return

  // Fetch list metadata and favorites items in parallel
  // /api/user/favorites resolves by list_type, no ID needed
  const [, favResponse] = await Promise.all([
    fetchLists(),
    api<any>('/user/favorites', {
      params: { includeAdult: includeAdult.value }
    }).catch(() => null)
  ])

  // Store favorites items using the list ID from fetchLists
  if (favoritesList.value && favResponse?.success) {
    listItems.value[favoritesList.value.id] = favResponse.data
  }
}

// Watch for authentication changes
watch(() => isAuthenticated.value, (authenticated) => {
  if (authenticated) {
    loadUserLists()
  }
})

onMounted(async () => {
  // Fetch user authentication state first
  await fetchUser()

  // Then load lists if authenticated
  if (isAuthenticated.value) {
    await loadUserLists()
  }
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

.anime-list-item :deep(.v-list-item__content) {
  overflow: hidden;
}

.anime-list-item :deep(.v-list-item-title) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 599px) {
  .anime-list-item :deep(.v-list-item-title) {
    font-size: 0.875rem !important;
  }
}

.banner-thumbnail {
  width: 140px;
  height: 56px;
  border-radius: var(--radius-md);
  overflow: hidden;
  flex-shrink: 0;
}

.banner-thumbnail-small {
  width: 120px;
  height: 48px;
  border-radius: var(--radius-md);
  overflow: hidden;
  flex-shrink: 0;
}

@media (min-width: 600px) {
  .banner-thumbnail {
    width: 280px;
    height: 112px;
  }

  .banner-thumbnail-small {
    width: 200px;
    height: 80px;
  }
}

/* Remove forced uppercase from tabs */
:deep(.v-tab) {
  text-transform: none !important;
}

.anime-search-card {
  transition: all 0.2s ease;
  cursor: pointer;
}

.anime-search-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.anime-search-card.already-in-list {
  opacity: 0.6;
  cursor: default;
}

.anime-search-card.already-in-list:hover {
  transform: none;
}

.already-added-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(var(--color-overlay-rgb), 0.6);
}
</style>
