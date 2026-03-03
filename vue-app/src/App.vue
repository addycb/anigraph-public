<template>
  <Transition name="login-alert-slide">
    <div v-if="showLoginAlert" class="login-required-alert">
      <span>Please <a href="#" class="login-required-alert__link" @click.prevent="loginWithGoogle">log in</a> to favorite or bookmark anime</span>
      <button class="login-required-alert__close" @click="showLoginAlert = false">&#x2715;</button>
    </div>
  </Transition>
  <router-view />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useLoginRequired } from '@/composables/useLoginRequired'
import { useAuth } from '@/composables/useAuth'
import { useAppTheme } from '@/composables/useTheme'
import { useFilterMetadata } from '@/composables/useFilterMetadata'

const { showLoginAlert } = useLoginRequired()
const { loginWithGoogle } = useAuth()

// Theme init (replaces theme-init.client.ts plugin)
const { applyTheme } = useAppTheme()
const savedTheme = localStorage.getItem('anigraph_theme') || 'scholar-light'
applyTheme(savedTheme)

// Preload filter metadata (replaces preloadFilterMetadata.client.ts plugin)
onMounted(() => {
  const { loadFilterMetadata, filterMetadataLoaded, loadingFilterMetadata } = useFilterMetadata()
  if (!filterMetadataLoaded.value && !loadingFilterMetadata.value) {
    if ('requestIdleCallback' in window) {
      window.requestIdleCallback(() => { loadFilterMetadata() }, { timeout: 500 })
    } else {
      setTimeout(() => { loadFilterMetadata() }, 300)
    }
  }
})
</script>

<style>
.login-required-alert {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10000;
  background: var(--color-error);
  color: var(--color-text);
  padding: 12px 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 0.95rem;
  box-shadow: var(--shadow-sm);
}

.login-required-alert__link {
  color: var(--color-text);
  font-weight: 700;
  text-decoration: underline;
  cursor: pointer;
}

.login-required-alert__link:hover {
  opacity: 0.85;
}

.login-required-alert__close {
  background: none;
  border: none;
  color: var(--color-text);
  font-size: 1.1rem;
  cursor: pointer;
  opacity: 0.7;
  margin-left: 12px;
  line-height: 1;
  padding: 0;
}

.login-required-alert__close:hover {
  opacity: 1;
}

.login-alert-slide-enter-active {
  transition: transform 0.3s ease;
}

.login-alert-slide-enter-from {
  transform: translateY(-100%);
}

.login-alert-slide-leave-active {
  transition: opacity 0.3s ease;
}

.login-alert-slide-leave-to {
  opacity: 0;
}
</style>
