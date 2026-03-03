<template>
  <v-app>
    <AppBar />

    <v-main class="login-page">
      <v-container fluid class="fill-height">
        <v-row align="center" justify="center">
          <v-col cols="12" sm="8" md="6" lg="4">
            <v-card class="login-card" elevation="8">
              <v-card-text class="text-center pa-8">
                <!-- Logo/Icon -->
                <v-avatar color="primary" size="80" class="mb-4">
                  <v-icon size="48" color="white">mdi-chart-bubble</v-icon>
                </v-avatar>

                <!-- Title -->
                <h1 class="text-h4 font-weight-bold mb-2">Welcome to Anigraph</h1>
                <p class="text-h6 text-medium-emphasis mb-8">
                  Sign in to unlock personalized features
                </p>

                <!-- Features List -->
                <v-list class="feature-list mb-6" bg-color="transparent">
                  <v-list-item class="px-0">
                    <template v-slot:prepend>
                      <v-icon color="success">mdi-heart</v-icon>
                    </template>
                    <v-list-item-title>Save your favorite anime</v-list-item-title>
                  </v-list-item>

                  <v-list-item class="px-0">
                    <template v-slot:prepend>
                      <v-icon color="success">mdi-chart-box</v-icon>
                    </template>
                    <v-list-item-title>Discover your taste profile</v-list-item-title>
                  </v-list-item>

                  <v-list-item class="px-0">
                    <template v-slot:prepend>
                      <v-icon color="success">mdi-heart-pulse</v-icon>
                    </template>
                    <v-list-item-title>Get personalized recommendations</v-list-item-title>
                  </v-list-item>

                  <v-list-item class="px-0">
                    <template v-slot:prepend>
                      <v-icon color="success">mdi-sync</v-icon>
                    </template>
                    <v-list-item-title>Sync across all devices</v-list-item-title>
                  </v-list-item>
                </v-list>

                <!-- Sign In Button -->
                <v-btn
                  block
                  size="x-large"
                  color="primary"
                  class="google-signin-btn"
                  @click="handleGoogleSignIn"
                  :loading="signingIn"
                >
                  <v-icon start size="24">mdi-google</v-icon>
                  Sign in with Google
                </v-btn>

                <!-- Continue Browsing -->
                <v-divider class="my-6"></v-divider>
                <v-btn
                  variant="text"
                  color="primary"
                  to="/"
                >
                  Continue browsing without signing in
                  <v-icon end>mdi-arrow-right</v-icon>
                </v-btn>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const { loginWithGoogle, isAuthenticated } = useAuth()
const router = useRouter()
const route = useRoute()
const signingIn = ref(false)

const redirectPath = computed(() => {
  const redirect = route.query.redirect as string
  return redirect || '/'
})

// Redirect if already authenticated
onMounted(() => {
  if (isAuthenticated.value) {
    router.push(redirectPath.value)
  }
})

// Watch for authentication changes
watch(isAuthenticated, (newValue) => {
  if (newValue) {
    router.push(redirectPath.value)
  }
})

const handleGoogleSignIn = () => {
  signingIn.value = true
  loginWithGoogle(redirectPath.value)
}
</script>

<style scoped>
.login-page {
  background: var(--gradient-surface);
  min-height: 100vh;
}

.login-card {
  background: var(--gradient-surface-solid) !important;
  backdrop-filter: blur(20px);
  border: 1px solid var(--color-primary-border);
  border-radius: var(--radius-xl);
}

.google-signin-btn {
  text-transform: none;
  font-size: 1.1rem;
  font-weight: 600;
  letter-spacing: 0.5px;
  padding: 24px !important;
  border-radius: var(--radius-lg);
}

.feature-list {
  text-align: left;
}

.feature-list :deep(.v-list-item-title) {
  font-size: 1rem;
}
</style>
