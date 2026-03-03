<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container fluid>
        <!-- Page Header -->
        <v-row class="mb-6">
          <v-col cols="12">
            <h1 class="text-h3 font-weight-bold mb-2">Public Lists</h1>
            <p class="text-h6 text-medium-emphasis">
              Discover curated anime collections from the community
            </p>
          </v-col>
        </v-row>

        <!-- Search and Filters -->
        <v-row class="mb-4">
          <v-col cols="12" md="6">
            <v-text-field
              v-model="searchQuery"
              placeholder="Search lists..."
              variant="outlined"
              density="comfortable"
              clearable
              prepend-inner-icon="mdi-magnify"
              @update:model-value="debouncedSearch"
            ></v-text-field>
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
            <p class="text-h6 mt-4">Loading public lists...</p>
          </v-col>
        </v-row>

        <!-- Lists Grid -->
        <v-row v-else-if="publicLists.length > 0">
          <v-col
            v-for="list in publicLists"
            :key="list.id"
            cols="12"
            sm="6"
            md="4"
          >
            <v-card
              hover
              @click="viewList(list)"
              class="list-card"
            >
              <!-- Preview Images -->
              <div class="list-preview">
                <div
                  v-for="(preview, idx) in list.previews"
                  :key="idx"
                  class="preview-image"
                  :style="{ backgroundImage: `url(${preview})` }"
                ></div>
                <div v-if="list.itemCount > 4" class="preview-count">
                  +{{ list.itemCount - 4 }}
                </div>
              </div>

              <v-card-title>
                <v-icon start>mdi-bookmark-multiple</v-icon>
                {{ list.name }}
              </v-card-title>

              <v-card-subtitle v-if="list.description" class="text-wrap">
                {{ list.description }}
              </v-card-subtitle>

              <v-card-text>
                <div class="d-flex align-center justify-space-between">
                  <v-chip size="small" variant="tonal">
                    <v-icon start size="small">mdi-image-multiple</v-icon>
                    {{ list.itemCount }} {{ list.itemCount === 1 ? 'anime' : 'anime' }}
                  </v-chip>
                  <v-chip size="small" variant="tonal" color="success">
                    <v-icon start size="small">mdi-earth</v-icon>
                    Public
                  </v-chip>
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <!-- Empty State -->
        <v-row v-else>
          <v-col cols="12" class="text-center py-12">
            <v-icon size="80" color="grey">mdi-bookmark-outline</v-icon>
            <h2 class="text-h4 mt-4">No public lists found</h2>
            <p class="text-h6 text-medium-emphasis mt-2">
              {{ searchQuery ? 'Try a different search term' : 'Be the first to create a public list!' }}
            </p>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/utils/api'
import { useSettings } from '@/composables/useSettings'

const router = useRouter()
const { includeAdult } = useSettings()

const searchQuery = ref('')
const publicLists = ref<any[]>([])
const loading = ref(true)

const fetchPublicLists = async () => {
  loading.value = true

  try {
    const params: Record<string, string> = {
      limit: '50',
      includeAdult: String(includeAdult.value)
    }

    if (searchQuery.value) {
      params.search = searchQuery.value
    }

    const response = await api<any>('/lists/public', { params })

    if (response.success) {
      publicLists.value = response.data
    }
  } catch (error) {
    console.error('Error fetching public lists:', error)
  } finally {
    loading.value = false
  }
}

let searchTimeout: ReturnType<typeof setTimeout> | null = null
const debouncedSearch = () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    fetchPublicLists()
  }, 300)
}

const viewList = (list: any) => {
  router.push(`/lists/${list.shareToken}`)
}

onMounted(() => {
  fetchPublicLists()
})

// Refresh lists when includeAdult setting changes
watch(includeAdult, () => {
  fetchPublicLists()
})

document.title = 'Public Lists - Anime Collections - Anigraph'
</script>

<style scoped>
.list-card {
  transition: all var(--transition-base);
  height: 100%;
  display: flex;
  flex-direction: column;
}

.list-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-glow);
  cursor: pointer;
}

.list-preview {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  height: 200px;
  position: relative;
  overflow: hidden;
}

.preview-image {
  background-size: cover;
  background-position: center;
  background-color: var(--color-surface-alt);
}

.preview-count {
  position: absolute;
  bottom: 8px;
  right: 8px;
  background: rgba(var(--color-overlay-rgb), 0.8);
  color: var(--color-text);
  padding: 4px 12px;
  border-radius: var(--radius-lg);
  font-weight: bold;
  font-size: var(--text-sm);
}
</style>
