<template>
  <v-app>
    <AppBar clickable-title />

    <v-main class="main-page">
      <!-- Two Column Layout -->
      <div class="two-column-container">
        <!-- Left Column: Hero & Search -->
        <div class="left-column">
          <div class="title-with-logo">
            <v-icon size="58" class="title-icon" color="primary">mdi-chart-bubble</v-icon>
            <h1 class="main-title">Anigraph</h1>
          </div>
          <p class="subtitle">Discover Anime Connections</p>

          <div class="search-wrapper">
            <SearchBar
              floating
              show-arrow-button
              density="comfortable"
              hide-details
              label=""
              placeholder="Search works, staff, studios..."
              tracking-source="index"
              @search="handleSearch"
            />
          </div>

          <v-btn
            color="primary"
            block
            rounded="lg"
            class="enter-button"
            height="auto"
            style="padding: 10px 16px;"
            @click="navigateToHome"
          >
            Enter Anigraph
            <v-icon end>mdi-arrow-right</v-icon>
          </v-btn>

          <v-btn
            variant="outlined"
            color="primary"
            block
            rounded="lg"
            class="tutorial-button"
            height="auto"
            style="padding: 10px 16px; margin-top: -16px;"
            to="/tutorial"
          >
            <v-icon start size="small">mdi-school</v-icon>
            Take the Interactive Tour
          </v-btn>
        </div>

        <!-- Right Column: How It Works -->
        <div class="right-column">
          <h2 class="section-title">How It Works</h2>
          <div class="cards-grid">
            <div class="info-card">
              <div class="card-icon">
                <v-icon size="40" color="primary">mdi-magnify</v-icon>
              </div>
              <h3 class="card-title">Search Any Anime</h3>
              <p class="card-description">
                Find any anime from our extensive database and dive into its creative team
              </p>
            </div>
            <div class="info-card">
              <div class="card-icon">
                <v-icon size="40" color="primary">mdi-connection</v-icon>
              </div>
              <h3 class="card-title">Staff & Studio Connections</h3>
              <p class="card-description">
                Discover which directors, writers, and studios connect different anime
              </p>
            </div>
            <div class="info-card">
              <div class="card-icon">
                <v-icon size="40" color="primary">mdi-tag-multiple</v-icon>
              </div>
              <h3 class="card-title">Tag & Genre Similarity</h3>
              <p class="card-description">
                Find similar anime through tag-based matching and genre analysis
              </p>
            </div>
          </div>
        </div>
      </div>
    </v-main>

    <AppFooter />
  </v-app>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSettings } from '@/composables/useSettings'

const router = useRouter()
const { includeAdult } = useSettings()

const handleSearch = (query: string) => {
  // Analytics tracked in home.vue to avoid duplicates
  router.push({
    path: '/overview',
    query: {
      q: query,
      includeAdult: includeAdult.value ? 'true' : undefined
    }
  })
}

const navigateToHome = () => {
  router.push('/overview')
}

onMounted(() => {
  document.title = 'Discover Anime Connections - Anigraph'
})
</script>

<style scoped>
.main-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 80px 24px 40px 24px;
}

/* Two Column Container */
.two-column-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
  max-width: 1400px;
  width: 100%;
  align-items: stretch;
  background-color: rgba(var(--color-surface-rgb), 0.4);
  border-radius: var(--radius-xl);
  overflow: hidden;
  box-shadow: var(--shadow-md);
}

/* Left Column */
.left-column {
  display: flex;
  flex-direction: column;
  gap: 32px;
  background-color: rgba(var(--color-surface-rgb), 0.6);
  padding: 56px 48px;
  text-align: center;
  align-items: center;
}

.title-with-logo {
  display: flex;
  align-items: center;
}

.title-icon {
  flex-shrink: 0;
  margin-left: -8px;
}

.main-title {
  font-size: 4rem;
  font-weight: 700;
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
  line-height: 1.1;
  letter-spacing: -0.02em;
}

.subtitle {
  font-size: 1.5rem;
  font-weight: 400;
  color: rgba(var(--color-text-rgb), 0.7);
  margin: -8px 0 0 0;
}

.search-wrapper {
  margin-top: 16px;
  width: 100%;
}

/* Override FloatingSearchBar fixed positioning */
.search-wrapper :deep(.floating-search-input) {
  position: static !important;
  transform: none !important;
  width: 100% !important;
  max-width: none !important;
}

.enter-button {
  margin-top: 8px;
  font-size: 1.0625rem;
  font-weight: 600;
  letter-spacing: 0.01em;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.enter-button:hover {
  transform: translateY(-2px);
}

.enter-button :deep(.v-btn__content) {
  padding: 0;
}

.enter-button :deep(.v-btn__overlay),
.enter-button :deep(.v-btn__underlay) {
  padding: 8px 16px;
}

.tutorial-button {
  font-size: 0.9375rem;
  font-weight: 500;
  letter-spacing: 0.01em;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  border-width: 2px;
}

.tutorial-button:hover {
  transform: translateY(-2px);
  background-color: var(--color-primary-faint);
}

/* Right Column */
.right-column {
  display: flex;
  flex-direction: column;
  background-color: rgba(var(--color-surface-rgb), 0.3);
  padding: 56px 48px;
}

.section-title {
  font-size: 2rem;
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: 32px;
  letter-spacing: -0.01em;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.info-card {
  background-color: rgba(var(--color-surface-rgb), 0.5);
  border-radius: var(--radius-lg);
  padding: 28px 24px;
  text-align: center;
  box-shadow: var(--shadow-sm);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.info-card:hover {
  transform: translateY(-6px);
  background-color: rgba(var(--color-surface-rgb), 0.7);
  box-shadow: var(--shadow-lg),
              0 0 20px rgba(var(--color-primary-rgb), 0.15);
}

.card-icon {
  margin-bottom: 12px;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: 10px;
  letter-spacing: -0.01em;
}

.card-description {
  font-size: 0.875rem;
  color: rgba(var(--color-text-rgb), 0.65);
  line-height: 1.6;
  margin: 0;
}

/* Responsive Design */
@media (max-width: 1200px) {
  .two-column-container {
    gap: 40px;
    padding: 40px;
  }

  .title-icon {
    font-size: 70px !important;
  }

  .main-title {
    font-size: 3.5rem;
  }

  .subtitle {
    font-size: 1.6rem;
  }
}

@media (max-width: 960px) {
  .two-column-container {
    grid-template-columns: 1fr;
    gap: 32px;
    padding: 40px 32px;
  }

  .main-page {
    padding: 60px 24px 40px 24px;
  }

  .title-icon {
    font-size: 64px !important;
  }

  .main-title {
    font-size: 3rem;
  }

  .subtitle {
    font-size: 1.4rem;
  }

  .section-title {
    font-size: 1.8rem;
  }

  .cards-grid {
    grid-template-columns: 1fr;
  }

  .left-column {
    padding: 40px 32px;
  }

  .right-column {
    text-align: center;
    padding: 40px 32px;
  }
}

@media (max-width: 600px) {
  .two-column-container {
    padding: 32px 24px;
    gap: 24px;
  }

  .main-page {
    padding: 60px 16px 40px 16px;
  }

  .title-icon {
    font-size: 56px !important;
  }

  .main-title {
    font-size: 2.5rem;
  }

  .subtitle {
    font-size: 1.1rem;
  }

  .section-title {
    font-size: 1.5rem;
  }

  .cards-grid {
    gap: 16px;
  }

  .left-column {
    gap: 24px;
    padding: 32px 24px;
  }

  .right-column {
    padding: 32px 24px;
  }

  .search-wrapper {
    width: 100%;
  }

  .enter-button {
    font-size: 0.95rem;
  }

  .v-toolbar-title {
    font-size: 1.5rem !important;
  }
}
</style>
