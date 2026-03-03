<template>
  <v-app-bar app flat class="app-bar-modern" elevation="0">
    <!-- Logo/Brand -->
    <v-app-bar-title>
      <RouterLink to="/" class="brand-link">
        <v-icon size="32" color="primary">mdi-chart-bubble</v-icon>
        <span class="brand-text">Anigraph</span>
      </RouterLink>
    </v-app-bar-title>

    <v-spacer></v-spacer>

    <!-- Navigation -->
    <template v-if="!mobile">
      <!-- Browse Dropdown -->
      <v-menu open-on-hover offset-y transition="slide-y-transition">
        <template v-slot:activator="{ props }">
          <v-btn
            v-bind="props"
            variant="text"
            class="nav-btn"
          >
            Browse
            <v-icon end size="small">mdi-chevron-down</v-icon>
          </v-btn>
        </template>
        <v-card class="dropdown-panel" elevation="8">
          <v-list class="py-0">
            <!-- Anime Section -->
            <v-list-subheader class="dropdown-header">
              <v-icon start color="primary">mdi-television-classic</v-icon>
              Anime
            </v-list-subheader>
            <v-list-item
              v-for="format in animeFormats"
              :key="format.value"
              :to="format.to"
              class="dropdown-item"
            >
              <template v-slot:prepend>
                <v-icon :icon="format.icon"></v-icon>
              </template>
              <v-list-item-title>{{ format.label }}</v-list-item-title>
            </v-list-item>

            <v-divider class="my-2"></v-divider>

            <!-- Manga Section -->
            <v-list-subheader class="dropdown-header">
              <v-icon start color="secondary">mdi-book-open-variant</v-icon>
              Manga
            </v-list-subheader>
            <v-list-item
              v-for="format in mangaFormats"
              :key="format.value"
              :to="format.to"
              class="dropdown-item"
            >
              <template v-slot:prepend>
                <v-icon :icon="format.icon"></v-icon>
              </template>
              <v-list-item-title>{{ format.label }}</v-list-item-title>
            </v-list-item>

          </v-list>
        </v-card>
      </v-menu>

      <!-- Discover Dropdown -->
      <v-menu open-on-hover offset-y transition="slide-y-transition">
        <template v-slot:activator="{ props }">
          <v-btn
            v-bind="props"
            variant="text"
            class="nav-btn"
          >
            Discover
            <v-icon end size="small">mdi-chevron-down</v-icon>
          </v-btn>
        </template>
        <v-card class="dropdown-panel" elevation="8">
          <v-list class="py-0">
            <v-list-item
              v-for="item in discoverItems"
              :key="item.value"
              :to="item.to"
              @click="item.action === 'random' ? goToRandom() : undefined"
              :class="['dropdown-item', { 'tour-highlight': item.value === 'tutorial' && !tutorialCompleted }]"
            >
              <template v-slot:prepend>
                <v-icon :icon="item.icon"></v-icon>
              </template>
              <v-list-item-title>{{ item.label }}</v-list-item-title>
              <v-list-item-subtitle v-if="item.subtitle">{{ item.subtitle }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>


      <!-- Search Button -->
      <v-btn
        icon
        variant="text"
        @click="searchDialog = true"
        class="nav-btn-icon"
      >
        <v-icon>mdi-magnify</v-icon>
      </v-btn>

      <!-- Theme Picker -->
      <v-menu :close-on-content-click="false" offset-y transition="slide-y-transition">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" icon variant="text" class="nav-btn-icon">
            <v-icon>mdi-palette</v-icon>
          </v-btn>
        </template>
        <v-card class="dropdown-panel" elevation="8">
          <v-card-text class="pa-3">
            <div class="text-caption text-medium-emphasis mb-2">Theme</div>
            <div class="theme-options">
              <button
                v-for="theme in appThemes"
                :key="theme.id"
                class="theme-swatch"
                :class="{ active: currentAppTheme === theme.id }"
                @click="handleThemeChange(theme.id)"
                :title="theme.name"
              >
                <span class="swatch-color" :style="{ background: theme.primary }"></span>
                <span class="swatch-label">{{ theme.name }}</span>
              </button>
            </div>
          </v-card-text>
        </v-card>
      </v-menu>

      <!-- Settings Button -->
      <v-btn
        to="/settings"
        icon
        variant="text"
        class="nav-btn-icon"
        active-class=""
      >
        <v-icon>mdi-cog</v-icon>
      </v-btn>

      <!-- Info Button -->
      <v-btn
        to="/status"
        icon
        variant="text"
        class="nav-btn-icon"
        active-class=""
      >
        <v-icon>mdi-information</v-icon>
      </v-btn>

      <!-- Auth Button -->
      <AuthButton compact />
    </template>

    <!-- Mobile Menu Button -->
    <template v-else>
      <v-app-bar-nav-icon @click="drawer = !drawer"></v-app-bar-nav-icon>
    </template>
  </v-app-bar>

  <!-- Search Dialog -->
  <v-dialog
    v-model="searchDialog"
    max-width="700"
    transition="fade-transition"
    scrim="transparent"
    class="search-dialog-overlay"
  >
    <div class="search-dialog-content" @click="searchDialog = false">
      <SearchBar
        tracking-source="appbar"
        placeholder="Search works, staff, studios..."
        label=""
        autofocus
        @click.stop
        @navigate="searchDialog = false"
      />
    </div>
  </v-dialog>

  <!-- Mobile Navigation Drawer -->
  <v-navigation-drawer
    v-if="mobile"
    v-model="drawer"
    temporary
    location="right"
    class="mobile-drawer"
  >
    <v-list class="py-0">
      <!-- Mobile Search -->
      <v-list-item class="pa-4">
        <div @click.stop>
          <SearchBar />
        </div>
      </v-list-item>

      <v-divider></v-divider>

      <!-- Browse Section -->
      <v-list-group value="browse">
        <template v-slot:activator="{ props }">
          <v-list-item v-bind="props" title="Browse">
            <template v-slot:prepend>
              <v-icon>mdi-view-grid</v-icon>
            </template>
          </v-list-item>
        </template>

        <v-list-subheader>Anime</v-list-subheader>
        <v-list-item
          v-for="format in animeFormats"
          :key="format.value"
          :to="format.to"
          @click="drawer = false"
        >
          <template v-slot:prepend>
            <v-icon :icon="format.icon" size="small"></v-icon>
          </template>
          <v-list-item-title>{{ format.label }}</v-list-item-title>
        </v-list-item>

        <v-divider class="my-2"></v-divider>

        <v-list-subheader>Manga</v-list-subheader>
        <v-list-item
          v-for="format in mangaFormats"
          :key="format.value"
          :to="format.to"
          @click="drawer = false"
        >
          <template v-slot:prepend>
            <v-icon :icon="format.icon" size="small"></v-icon>
          </template>
          <v-list-item-title>{{ format.label }}</v-list-item-title>
        </v-list-item>
      </v-list-group>

      <v-divider></v-divider>

      <!-- Discover Section -->
      <v-list-group value="discover">
        <template v-slot:activator="{ props }">
          <v-list-item v-bind="props" title="Discover">
            <template v-slot:prepend>
              <v-icon>mdi-compass</v-icon>
            </template>
          </v-list-item>
        </template>

        <v-list-item
          v-for="item in discoverItems"
          :key="item.value"
          :to="item.to"
          @click="item.action === 'random' ? goToRandom() : drawer = false"
          :class="{ 'tour-highlight': item.value === 'tutorial' && !tutorialCompleted }"
        >
          <template v-slot:prepend>
            <v-icon :icon="item.icon" size="small"></v-icon>
          </template>
          <v-list-item-title>{{ item.label }}</v-list-item-title>
        </v-list-item>
      </v-list-group>

      <v-divider></v-divider>


      <v-divider></v-divider>

      <!-- Theme -->
      <v-list-item class="pa-4">
        <template v-slot:prepend>
          <v-icon>mdi-palette</v-icon>
        </template>
        <v-list-item-title class="mb-2">Theme</v-list-item-title>
        <div class="mobile-theme-swatches">
          <button
            v-for="theme in appThemes"
            :key="theme.id"
            class="mobile-swatch"
            :class="{ active: currentAppTheme === theme.id }"
            @click="handleThemeChange(theme.id)"
            :title="theme.name"
          >
            <span class="mobile-swatch-dot" :style="{ background: theme.primary }"></span>
          </button>
        </div>
      </v-list-item>

      <v-divider></v-divider>

      <!-- Settings -->
      <v-list-item to="/settings" active-class="" @click="drawer = false">
        <template v-slot:prepend>
          <v-icon>mdi-cog</v-icon>
        </template>
        <v-list-item-title>Settings</v-list-item-title>
      </v-list-item>

      <!-- Info / Status -->
      <v-list-item to="/status" active-class="" @click="drawer = false">
        <template v-slot:prepend>
          <v-icon>mdi-information</v-icon>
        </template>
        <v-list-item-title>System Status</v-list-item-title>
      </v-list-item>

      <v-divider></v-divider>

      <!-- Auth Button (Mobile) -->
      <v-list-item class="pa-4">
        <AuthButton compact />
      </v-list-item>
    </v-list>
  </v-navigation-drawer>
</template>

<script setup lang="ts">
import { useDisplay } from 'vuetify'
import { useRouter } from 'vue-router'
import { api } from '~/utils/api'

// Define props and emits
defineProps<{
  clickableTitle?: boolean
}>()

const emit = defineEmits<{
  titleClick: []
}>()

const router = useRouter()
const { includeAdult } = useSettings()
const { currentTheme: currentAppTheme, themes: allAppThemes, setTheme: setAppTheme } = useAppTheme()
const { isAuthenticated } = useAuth()
const { savePreferences } = useUserPreferences()
const { tutorialCompleted } = useTutorial()

const handleThemeChange = (themeId: string) => {
  setAppTheme(themeId)
  if (isAuthenticated.value) savePreferences({ theme: themeId })
}
const appThemes = computed(() => allAppThemes.filter(t => ['midnight', 'slate', 'healing', 'sakura-light', 'scholar-light', 'asiimov-light', 'strawberry'].includes(t.id)))

// Responsive - use Vuetify's display composable
const { mobile } = useDisplay()
const drawer = ref(false)
const searchDialog = ref(false)

// Navigation Items
const animeFormats = [
  { label: 'All Anime', value: 'all', icon: 'mdi-animation', to: '/home?type=anime' },
  { label: 'TV Series', value: 'tv', icon: 'mdi-television', to: '/home?type=anime&format=TV' },
  { label: 'Movies', value: 'movie', icon: 'mdi-movie', to: '/home?type=anime&format=MOVIE' },
  { label: 'OVA', value: 'ova', icon: 'mdi-disc', to: '/home?type=anime&format=OVA' },
  { label: 'ONA', value: 'ona', icon: 'mdi-web', to: '/home?type=anime&format=ONA' },
  { label: 'Specials', value: 'special', icon: 'mdi-star', to: '/home?type=anime&format=SPECIAL' },
  { label: 'TV Shorts', value: 'tv_short', icon: 'mdi-television-classic', to: '/home?type=anime&format=TV_SHORT' },
  { label: 'Music', value: 'music', icon: 'mdi-music', to: '/home?type=anime&format=MUSIC' },
]

const mangaFormats = [
  { label: 'All Manga', value: 'all', icon: 'mdi-book-open-page-variant', to: '/home?type=manga' },
  { label: 'Manga', value: 'manga', icon: 'mdi-book', to: '/home?type=manga&format=MANGA' },
  { label: 'Light Novels', value: 'novel', icon: 'mdi-book-alphabet', to: '/home?type=manga&format=NOVEL' },
  { label: 'One Shots', value: 'one_shot', icon: 'mdi-book-outline', to: '/home?type=manga&format=ONE_SHOT' },
]

const discoverItems = [
  // { label: 'Public Lists', value: 'public-lists', icon: 'mdi-bookmark-multiple', to: '/lists/public', subtitle: 'Community anime collections' },
{ label: 'New Productions', value: 'new-productions', icon: 'mdi-new-box', to: '/new-productions', subtitle: 'Recently released works' },
  { label: 'Just Added', value: 'just-added', icon: 'mdi-clock-plus-outline', to: '/just-added', subtitle: 'Latest database entries' },
  { label: 'Top Rated', value: 'top', icon: 'mdi-trophy', to: '/top-rated', subtitle: 'Best of all time' },
  { label: 'Advanced Search', value: 'advanced', icon: 'mdi-filter-variant', to: '/search/advanced', subtitle: 'Find exactly what you want' },
  { label: 'Random', value: 'random', icon: 'mdi-shuffle-variant', action: 'random', subtitle: 'Discover something new' },
  { label: 'Interactive Tour', value: 'tutorial', icon: 'mdi-school', to: '/tutorial', subtitle: 'Learn how to use AniGraph' },
]

// Random navigation
const goToRandom = async () => {
  try {
    // Fetch a random anime/manga
    const response = await api('/anime/popular', {
      params: {
        limit: 1,
        sort: 'random',
        includeAdult: includeAdult.value
      }
    })

    if (response.success && response.data && response.data.length > 0) {
      const randomItem = response.data[0]
      // Navigate to the anime/manga page
      await router.push(`/anime/${randomItem.id}`)
      drawer.value = false
    }
  } catch (error) {
    console.error('Error fetching random anime:', error)
  }
}

</script>

<style scoped>
.app-bar-modern {
  background: var(--gradient-surface) !important;
  backdrop-filter: blur(20px);
  border-bottom: 1px solid var(--color-primary-border);
  box-shadow: var(--shadow-md);
}

.brand-link {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
  color: var(--color-text);
  transition: all var(--transition-base);
}

.brand-link:hover {
  transform: none;
}

.brand-text {
  font-size: 1.5rem;
  font-weight: 700;
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.nav-btn {
  color: rgba(var(--color-text-rgb), 0.9) !important;
  text-transform: none;
  font-weight: 500;
  letter-spacing: 0.5px;
  margin: 0 4px;
  transition: all 0.3s ease;
}

.nav-btn:hover {
  background: var(--color-primary-muted) !important;
  color: var(--color-text) !important;
}

.nav-btn-icon {
  color: rgba(var(--color-text-rgb), 0.9) !important;
  transition: all 0.3s ease;
}

.nav-btn-icon:hover {
  background: var(--color-primary-muted) !important;
  color: var(--color-text) !important;
  transform: translateY(-2px);
}

/* Dropdown Panels */
.dropdown-panel {
  background: var(--gradient-surface-solid) !important;
  backdrop-filter: blur(20px);
  border: 1px solid var(--color-primary-border);
  margin-top: 8px;
  min-width: 250px;
}

.dropdown-header {
  color: rgba(var(--color-text-rgb), 0.7);
  font-weight: 600;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 1px;
  padding: 12px 16px 8px;
  background: var(--color-primary-faint);
}

.dropdown-item {
  color: rgba(var(--color-text-rgb), 0.9);
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
}

.dropdown-item:hover {
  background: var(--color-primary-muted) !important;
  border-left-color: var(--color-primary-border-accent);
  transform: translateX(4px);
}

.dropdown-item :deep(.v-list-item-title) {
  font-weight: 500;
}

.dropdown-item :deep(.v-list-item-subtitle) {
  font-size: 0.75rem;
  color: rgba(var(--color-text-rgb), 0.6);
}

/* Search Dialog */
.search-dialog-overlay :deep(.v-overlay__scrim) {
  opacity: 0 !important;
}

.search-dialog-content {
  padding: 16px;
  margin-top: 80px;
}

.search-dialog-content :deep(.v-autocomplete) {
  background: transparent;
}

.search-dialog-content :deep(.v-field) {
  background: rgba(var(--color-surface-rgb), 0.95) !important;
  backdrop-filter: blur(20px);
  border-radius: 28px !important;
  box-shadow: var(--shadow-lg), 0 0 0 1px var(--color-primary-strong);
  transition: all var(--transition-base);
}

.search-dialog-content :deep(.v-field--focused) {
  box-shadow: var(--shadow-xl), 0 0 0 2px var(--color-primary-border-focus);
}

.search-dialog-content :deep(.v-field__input) {
  color: var(--color-text);
  padding: 12px 16px;
  font-size: 1.1rem;
}

.search-dialog-content :deep(.v-field__input::placeholder) {
  color: rgba(var(--color-text-rgb), 0.6);
}

.search-dialog-content :deep(.v-icon) {
  color: rgba(var(--color-text-rgb), 0.8);
}

/* Style the autocomplete menu */
.search-dialog-content :deep(.v-overlay__content) {
  background: rgba(var(--color-surface-rgb), 0.98) !important;
  backdrop-filter: blur(20px);
  border-radius: 16px;
  margin-top: 8px;
  box-shadow: var(--shadow-lg);
}

.search-dialog-content :deep(.v-list) {
  background: transparent !important;
}

.search-dialog-content :deep(.v-list-item) {
  color: var(--color-text);
}

.search-dialog-content :deep(.v-list-item:hover) {
  background: var(--color-primary-medium) !important;
}

/* Mobile Drawer */
.mobile-drawer {
  background: var(--gradient-surface-solid) !important;
  backdrop-filter: blur(20px);
}

.mobile-drawer :deep(.v-list-item) {
  color: rgba(var(--color-text-rgb), 0.9);
}

.mobile-drawer :deep(.v-list-item:hover) {
  background: var(--color-primary-muted) !important;
}

.mobile-drawer :deep(.v-list-subheader) {
  color: rgba(var(--color-text-rgb), 0.7);
  font-weight: 600;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 1px;
}

/* Tour Highlight */
.tour-highlight {
  background: var(--color-primary-muted) !important;
  border-left-color: var(--color-primary-border-accent) !important;
  position: relative;
}

.tour-highlight::after {
  content: 'NEW';
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 0.6rem;
  font-weight: 700;
  letter-spacing: 0.5px;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--color-primary);
  color: var(--color-on-primary, #fff);
}

/* Animations */
@keyframes slideDown {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.dropdown-panel {
  animation: slideDown 0.2s ease;
}

/* Mobile Theme Swatches */
.mobile-theme-swatches {
  display: flex;
  gap: 8px;
  padding-top: 4px;
}

.mobile-swatch {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 2px solid transparent;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.mobile-swatch.active {
  border-color: var(--color-primary);
}

.mobile-swatch-dot {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
}

/* Inline Theme Picker */
.theme-options {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.theme-swatch {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 2px solid transparent;
  background: transparent;
  cursor: pointer;
  transition: all 0.2s ease;
  width: 100%;
  text-align: left;
}

.theme-swatch:hover {
  background: rgba(var(--color-text-rgb), 0.06);
}

.theme-swatch.active {
  border-color: var(--color-primary);
  background: rgba(var(--color-text-rgb), 0.08);
}

.swatch-color {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  flex-shrink: 0;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
}

.swatch-label {
  font-size: 0.85rem;
  color: rgba(var(--color-text-rgb), 0.9);
  white-space: nowrap;
}
</style>
