<template>
  <v-container fluid class="section-carousel">
    <div class="section-header">
      <div class="header-left">
        <v-icon :size="28" :color="color">{{ icon }}</v-icon>
        <div class="header-text">
          <h2 class="section-title" :style="{ color: cssColor }">{{ title }}</h2>
          <p v-if="subtitle" class="section-subtitle">{{ subtitle }}</p>
        </div>
      </div>
      <v-btn
        variant="text"
        :color="color"
        @click="$emit('view-all')"
        @auxclick.prevent="$event.button === 1 && openViewAllNewTab()"
        class="view-all-btn"
      >
        View All
        <v-icon end size="20">mdi-arrow-right</v-icon>
      </v-btn>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="carousel-loading">
      <v-row>
        <v-col
          v-for="i in 6"
          :key="i"
          cols="6"
          sm="4"
          md="3"
          lg="2"
        >
          <v-skeleton-loader
            type="image, article"
            class="skeleton-card"
          ></v-skeleton-loader>
        </v-col>
      </v-row>
    </div>

    <!-- Carousel Content -->
    <div v-else-if="items.length > 0" class="carousel-container">
      <v-btn
        v-if="canScrollLeft"
        icon
        class="carousel-nav carousel-nav-left"
        :color="color"
        @click="scrollLeft"
        elevation="4"
      >
        <v-icon>mdi-chevron-left</v-icon>
      </v-btn>

      <div
        ref="scrollContainer"
        class="carousel-scroll"
        @scroll="updateScrollButtons"
      >
        <div class="carousel-track">
          <div
            v-for="(anime, index) in items"
            :key="anime.id || anime.anilistId"
            class="carousel-item"
            :style="{ animationDelay: `${index * 50}ms` }"
          >
            <AnimeCard :anime="anime" :staff-count-label="staffCountLabel" :show-staff-count="showStaffCount" />
          </div>
        </div>
      </div>

      <v-btn
        v-if="canScrollRight"
        icon
        class="carousel-nav carousel-nav-right"
        :color="color"
        @click="scrollRight"
        elevation="4"
      >
        <v-icon>mdi-chevron-right</v-icon>
      </v-btn>
    </div>

    <!-- Empty State -->
    <div v-else class="carousel-empty">
      <v-icon size="64" color="grey-darken-1">mdi-animation-outline</v-icon>
      <p class="empty-text">No anime found</p>
    </div>
  </v-container>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'

interface AnimeData {
  id?: string
  anilistId?: number
  title?: string
  titleEnglish?: string
  titleRomaji?: string
  coverImage?: string
  coverImage_large?: string
  coverImage_extraLarge?: string
  averageScore?: number
  season?: string
  seasonYear?: number
  format?: string
  description?: string
}

interface Props {
  title: string
  subtitle?: string
  items: AnimeData[]
  loading?: boolean
  icon?: string
  color?: string
  staffCountLabel?: string
  showStaffCount?: boolean
  viewAllHref?: string
}

const props = withDefaults(defineProps<Props>(), {
  subtitle: '',
  loading: false,
  icon: 'mdi-star',
  color: 'primary',
  staffCountLabel: 'shared',
  showStaffCount: false,
  viewAllHref: ''
})

defineEmits<{
  (e: 'view-all'): void
}>()

// Convert Vuetify named colors to CSS-safe values for inline styles
const vuetifyColorMap: Record<string, string> = {
  primary: 'rgb(var(--v-theme-primary))',
  secondary: 'rgb(var(--v-theme-secondary))',
  accent: 'rgb(var(--v-theme-accent))',
  error: 'rgb(var(--v-theme-error))',
  info: 'rgb(var(--v-theme-info))',
  success: 'rgb(var(--v-theme-success))',
  warning: 'rgb(var(--v-theme-warning))',
}
const cssColor = computed(() => vuetifyColorMap[props.color] || props.color)

const openViewAllNewTab = () => {
  if (props.viewAllHref) window.open(props.viewAllHref, '_blank')
}

const scrollContainer = ref<HTMLElement | null>(null)
const canScrollLeft = ref(false)
const canScrollRight = ref(false)

const updateScrollButtons = (): void => {
  if (!scrollContainer.value) return

  const { scrollLeft, scrollWidth, clientWidth } = scrollContainer.value
  canScrollLeft.value = scrollLeft > 0
  canScrollRight.value = scrollLeft < scrollWidth - clientWidth - 10
}

const scrollLeft = (): void => {
  if (!scrollContainer.value) return

  const scrollAmount = scrollContainer.value.clientWidth * 0.8
  scrollContainer.value.scrollBy({
    left: -scrollAmount,
    behavior: 'smooth'
  })
}

const scrollRight = (): void => {
  if (!scrollContainer.value) return

  const scrollAmount = scrollContainer.value.clientWidth * 0.8
  scrollContainer.value.scrollBy({
    left: scrollAmount,
    behavior: 'smooth'
  })
}

onMounted(() => {
  if (scrollContainer.value) {
    updateScrollButtons()

    // Update on window resize
    window.addEventListener('resize', updateScrollButtons)

    onBeforeUnmount(() => {
      window.removeEventListener('resize', updateScrollButtons)
    })
  }
})

// Watch items to update scroll buttons when data loads
watch(() => props.items, () => {
  nextTick(() => {
    updateScrollButtons()
  })
}, { deep: true })
</script>

<style scoped>
.section-carousel {
  padding: 24px 24px;
  max-width: 1600px;
  margin: 0 auto;
  position: relative;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.section-title {
  font-size: 2rem;
  font-weight: 600;
  margin: 0;
  letter-spacing: -0.01em;
}

.section-subtitle {
  font-size: 0.875rem;
  color: rgba(var(--color-text-rgb), 0.65);
  margin: 0;
  font-weight: 400;
}

.view-all-btn {
  text-transform: none;
  font-size: 1rem;
  font-weight: 500;
}

/* Loading State */
.carousel-loading {
  opacity: 0.6;
}

.skeleton-card {
  background: rgba(var(--color-surface-rgb), 0.4);
  border-radius: 12px;
}

/* Carousel Container */
.carousel-container {
  position: relative;
  padding: 0 48px;
}

.carousel-scroll {
  overflow-x: auto;
  overflow-y: hidden;
  scroll-behavior: smooth;
  scrollbar-width: none;
  -ms-overflow-style: none;
  padding: 8px 0 16px;
}

.carousel-scroll::-webkit-scrollbar {
  display: none;
}

.carousel-track {
  display: flex;
  gap: 20px;
  padding: 4px;
}

.carousel-item {
  flex: 0 0 auto;
  width: 200px;
  animation: fadeInUp 0.6s ease-out forwards;
  opacity: 0;
  height: 100%;
}

.carousel-item :deep(.anime-card) {
  height: 100%;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Navigation Buttons */
.carousel-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  background: rgba(var(--color-primary-rgb), 0.95) !important;
  backdrop-filter: blur(8px);
  transition: all var(--transition-base);
  width: 48px !important;
  height: 48px !important;
  box-shadow: var(--shadow-md),
              0 0 0 2px rgba(var(--color-text-rgb), 0.1) !important;
}

.carousel-nav :deep(.v-icon) {
  color: var(--color-text) !important;
  font-size: 28px !important;
}

.carousel-nav:hover {
  transform: translateY(-50%) scale(1.15);
  background: var(--color-primary) !important;
  box-shadow: var(--shadow-glow-strong),
              0 0 0 2px rgba(var(--color-text-rgb), 0.2) !important;
}

.carousel-nav-left {
  left: 0;
}

.carousel-nav-right {
  right: 0;
}

/* Empty State */
.carousel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 64px 24px;
  color: rgba(var(--color-text-rgb), 0.5);
}

.empty-text {
  font-size: 1.125rem;
  margin-top: 16px;
  margin-bottom: 0;
}

/* Responsive Design */
@media (max-width: 960px) {
  .section-carousel {
    padding: 16px 16px;
  }

  .section-title {
    font-size: 1.75rem;
  }

  .section-subtitle {
    font-size: 0.8125rem;
  }

  .carousel-container {
    padding: 0 40px;
  }

  .carousel-item {
    width: 180px;
  }

  .carousel-track {
    gap: 16px;
  }
}

@media (max-width: 600px) {
  .section-carousel {
    padding: 12px 8px;
  }

  .section-title {
    font-size: 1.5rem;
  }

  .section-subtitle {
    font-size: 0.75rem;
  }

  .view-all-btn {
    font-size: 0.875rem;
    padding: 4px 8px;
  }

  .carousel-container {
    padding: 0;
  }

  .carousel-nav {
    display: none;
  }

  .carousel-scroll {
    padding: 16px 8px 24px;
  }

  .carousel-item {
    width: 160px;
  }

  .carousel-track {
    gap: 12px;
  }
}

/* Smooth scroll behavior for touch devices */
@media (hover: none) and (pointer: coarse) {
  .carousel-scroll {
    -webkit-overflow-scrolling: touch;
  }
}
</style>
