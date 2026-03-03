<template>
  <v-app>
    <AppBar />

    <v-main class="status-page">
      <v-container class="status-container">
        <h1 class="text-h4 font-weight-bold mb-6">System Status</h1>

        <div v-if="loading" class="text-center py-8">
          <v-progress-circular indeterminate color="primary" size="48"></v-progress-circular>
          <p class="text-caption text-medium-emphasis mt-3">Checking services...</p>
        </div>

        <template v-else>
          <!-- Services -->
          <v-card class="status-card mb-4">
            <div class="status-row" v-for="service in services" :key="service.name">
              <div class="status-row-left">
                <v-icon size="20">{{ service.icon }}</v-icon>
                <div>
                  <div class="text-body-1 font-weight-medium">{{ service.name }}</div>
                  <div class="text-caption text-medium-emphasis">{{ service.detail }}</div>
                </div>
              </div>
              <v-chip
                v-if="service.status !== undefined"
                :color="service.status ? 'success' : 'error'"
                size="small"
                variant="flat"
              >
                {{ service.status ? 'Connected' : 'Offline' }}
              </v-chip>
            </div>
          </v-card>

          <!-- Tour -->
          <v-btn
            to="/tutorial"
            color="primary"
            variant="tonal"
            block
            class="mb-4"
            prepend-icon="mdi-school"
          >
            Take the Interactive Tour
          </v-btn>

          <!-- Credits -->
          <v-card class="status-card">
            <div class="status-row justify-center">
              <span class="text-body-2 text-medium-emphasis">By Addison Baum</span>
              <v-btn
                href="https://linkedin.com/in/addycb"
                target="_blank"
                variant="text"
                size="small"
                icon
              >
                <v-icon size="18">mdi-linkedin</v-icon>
              </v-btn>
              <v-btn
                href="https://bdayatk.com"
                target="_blank"
                variant="text"
                size="small"
                icon
              >
                <span style="font-size: 18px;">&#127874;</span>
              </v-btn>
            </div>
          </v-card>
        </template>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/utils/api'

const loading = ref(true)
const status = ref<Record<string, any>>({})

const services = computed(() => [
  { name: 'Reverse Proxy', detail: 'Nginx', icon: 'mdi-server' },
  { name: 'API Server', detail: 'Go (ConnectRPC + Chi)', icon: 'mdi-language-go', status: status.value.server === 'go' },
  { name: 'Frontend', detail: 'Vue 3 + Vuetify (SPA)', icon: 'mdi-vuejs' },
  { name: 'Relational Database', detail: 'PostgreSQL 16', icon: 'mdi-database', status: status.value.postgres?.connected },
  { name: 'Search Engine', detail: 'Elasticsearch 8.11', icon: 'mdi-magnify', status: status.value.elasticsearch?.connected },
  { name: 'Graph Database', detail: 'Neo4j 5 (pipeline)', icon: 'mdi-graph', status: status.value.neo4j?.connected },
  { name: 'AI Integration', detail: 'OpenAI (franchise naming)', icon: 'mdi-brain' },
  { name: 'Orchestration', detail: 'Docker Compose', icon: 'mdi-docker' },
])

onMounted(async () => {
  document.title = 'System Status - Anigraph'
  try {
    const response = await api('/status')
    status.value = response
  } catch (error) {
    console.error('Failed to fetch status:', error)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.status-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px;
}

.status-container {
  max-width: 560px !important;
  padding-top: 40px;
}

.status-card {
  background: var(--gradient-surface-card) !important;
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-border) !important;
  border-radius: 12px !important;
  overflow: hidden;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  gap: 16px;
}

.status-row + .status-row {
  border-top: 1px solid rgba(var(--color-text-rgb), 0.06);
}

.status-row-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
