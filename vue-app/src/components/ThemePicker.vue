<template>
  <v-menu :close-on-content-click="false" offset-y transition="slide-y-transition">
    <template #activator="{ props }">
      <v-btn v-bind="props" icon variant="text" class="theme-picker-btn">
        <v-icon>mdi-palette</v-icon>
      </v-btn>
    </template>
    <v-card class="theme-menu" elevation="8">
      <v-card-text class="pa-3">
        <div class="text-caption text-medium-emphasis mb-2">Theme</div>
        <div class="theme-options">
          <button
            v-for="theme in themes"
            :key="theme.id"
            class="theme-swatch"
            :class="{ active: currentTheme === theme.id }"
            @click="setTheme(theme.id)"
            :title="theme.name"
          >
            <span class="swatch-color" :style="{ background: theme.primary }"></span>
            <span class="swatch-label">{{ theme.name }}</span>
          </button>
        </div>
      </v-card-text>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { useAppTheme } from '@/composables/useAppTheme'

const { currentTheme, themes, setTheme } = useAppTheme()
</script>

<style scoped>
.theme-picker-btn {
  color: rgba(var(--color-text-rgb), 0.9) !important;
  transition: all 0.3s ease;
}

.theme-picker-btn:hover {
  background: var(--color-primary-muted) !important;
  color: var(--color-text) !important;
  transform: translateY(-2px);
}

.theme-menu {
  background: var(--gradient-surface-solid) !important;
  backdrop-filter: blur(20px);
  border: 1px solid var(--color-primary-border);
  min-width: 180px;
}

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
