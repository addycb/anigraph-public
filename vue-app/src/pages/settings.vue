<template>
  <v-app>
    <AppBar />

    <v-main class="settings-page">
      <v-container class="settings-container">
        <h1 class="text-h4 font-weight-bold mb-6">Settings</h1>

        <v-card class="settings-card">
          <!-- Theme -->
          <div class="settings-row">
            <div class="settings-row-left">
              <v-icon color="primary" size="20">mdi-palette</v-icon>
              <div>
                <div class="text-body-1 font-weight-medium">Theme</div>
                <div class="text-caption text-medium-emphasis">App color scheme</div>
              </div>
            </div>
            <div class="theme-options">
              <button
                v-for="theme in visibleThemes"
                :key="theme.id"
                class="theme-chip"
                :class="{ active: currentAppTheme === theme.id }"
                @click="handleThemeChange(theme.id)"
              >
                <span class="theme-dot" :style="{ background: theme.primary }"></span>
                {{ theme.name }}
              </button>
            </div>
          </div>

          <v-divider />

          <!-- Adult Content -->
          <div class="settings-row">
            <div class="settings-row-left">
              <v-icon color="primary" size="20">mdi-eye-off-outline</v-icon>
              <div>
                <div class="text-body-1 font-weight-medium">Adult Content</div>
                <div class="text-caption text-medium-emphasis">Show in search and browsing</div>
              </div>
            </div>
            <v-switch
              :model-value="includeAdult"
              @update:model-value="handleToggle"
              color="primary"
              hide-details
              density="compact"
            />
          </div>
        </v-card>

        <div class="text-caption text-medium-emphasis mt-4 d-flex align-center justify-center">
          <v-icon size="14" class="mr-1">mdi-information-outline</v-icon>
          {{ isAuthenticated ? 'Settings are synced to your account.' : 'Settings are saved locally in your browser.' }}
        </div>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSettings } from '@/composables/useSettings'
import { useAnalytics } from '@/composables/useAnalytics'
import { useAppTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'
import { useUserPreferences } from '@/composables/useUserPreferences'

const { includeAdult, setIncludeAdult } = useSettings()
const { trackSettingsChange } = useAnalytics()
const { currentTheme: currentAppTheme, themes: allThemes, setTheme: setAppTheme } = useAppTheme()
const { isAuthenticated } = useAuth()
const { savePreferences } = useUserPreferences()

const visibleThemes = computed(() => allThemes)

const handleThemeChange = (themeId: string) => {
  setAppTheme(themeId)
  if (isAuthenticated.value) savePreferences({ theme: themeId })
}

const handleToggle = (value: boolean) => {
  setIncludeAdult(value)
  trackSettingsChange('includeAdult', value)
  if (isAuthenticated.value) savePreferences({ includeAdult: value })
}

document.title = 'Settings - Anigraph'
</script>

<style scoped>
.settings-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px;
}

.settings-container {
  max-width: 560px !important;
  padding-top: 40px;
}

.settings-card {
  background: var(--gradient-surface-card) !important;
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-border) !important;
  border-radius: 12px !important;
  overflow: visible;
}

.settings-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  gap: 16px;
}

.settings-row-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.theme-options {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: flex-end;
}

.theme-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 20px;
  border: 1.5px solid rgba(var(--color-text-rgb), 0.1);
  background: transparent;
  color: rgba(var(--color-text-rgb), 0.8);
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.theme-chip:hover {
  border-color: rgba(var(--color-text-rgb), 0.25);
  background: rgba(var(--color-text-rgb), 0.05);
}

.theme-chip.active {
  border-color: var(--color-primary);
  color: var(--color-text);
}

.theme-dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  flex-shrink: 0;
}
</style>
