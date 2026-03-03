<template>
  <div>
    <!-- Loading state -->
    <v-skeleton-loader v-if="loading" type="avatar" />

    <!-- Not authenticated - Compact (navbar) -->
    <v-btn
      v-else-if="!isAuthenticated && compact"
      variant="text"
      class="nav-btn-compact"
      :to="loginUrl"
    >
      <v-icon start>mdi-login</v-icon>
      Sign In
    </v-btn>

    <!-- Not authenticated - Full size (login pages) -->
    <v-btn
      v-else-if="!isAuthenticated"
      variant="flat"
      color="primary"
      size="large"
      :to="loginUrl"
    >
      <v-icon start>mdi-google</v-icon>
      Sign in with Google
    </v-btn>

    <!-- Authenticated user menu -->
    <v-menu v-else>
      <template v-slot:activator="{ props }">
        <v-btn icon v-bind="props">
          <v-avatar v-if="user?.picture && !pictureError" size="32">
            <v-img :src="user.picture" :alt="user.name" @error="pictureError = true" />
          </v-avatar>
          <v-icon v-else>mdi-account-circle</v-icon>
        </v-btn>
      </template>

      <v-list>
        <!-- User info -->
        <v-list-item>
          <v-list-item-title>{{ user?.name }}</v-list-item-title>
          <v-list-item-subtitle>{{ user?.email }}</v-list-item-subtitle>
        </v-list-item>

        <v-divider />

        <!-- Favorites & Lists -->
        <v-list-item to="/favorites">
          <template v-slot:prepend>
            <v-icon>mdi-bookmark-multiple</v-icon>
          </template>
          <v-list-item-title>Favorites & Lists</v-list-item-title>
        </v-list-item>

        <!-- Taste Profile -->
        <v-list-item to="/taste-profile">
          <template v-slot:prepend>
            <v-icon>mdi-account-star</v-icon>
          </template>
          <v-list-item-title>Taste Profile</v-list-item-title>
        </v-list-item>

        <!-- Recommendations -->
        <v-list-item to="/recommendations">
          <template v-slot:prepend>
            <v-icon>mdi-heart-pulse</v-icon>
          </template>
          <v-list-item-title>Recommended For You</v-list-item-title>
        </v-list-item>

        <v-divider />

        <!-- Logout -->
        <v-list-item @click="logout">
          <template v-slot:prepend>
            <v-icon>mdi-logout</v-icon>
          </template>
          <v-list-item-title>Sign Out</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-menu>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch, computed } from 'vue';
import { useRoute } from 'vue-router';
import { useAuth } from '~/composables/useAuth';

interface Props {
  compact?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  compact: false
})

const { user, loading, isAuthenticated, fetchUser, loginWithGoogle, logout } = useAuth();
const route = useRoute();

const pictureError = ref(false);
watch(() => user.value?.picture, () => { pictureError.value = false; });

const loginUrl = computed(() => {
  const currentPath = route.fullPath;
  // Don't redirect back to login page itself
  if (currentPath === '/login' || currentPath.startsWith('/login?')) {
    return '/login';
  }
  return `/login?redirect=${encodeURIComponent(currentPath)}`;
});

onMounted(() => {
  fetchUser();
});
</script>

<style scoped>
.nav-btn-compact {
  color: rgba(var(--color-text-rgb), 0.9) !important;
  text-transform: none;
  font-weight: 500;
  letter-spacing: 0.5px;
  transition: all 0.3s ease;
}

.nav-btn-compact:hover {
  background: var(--color-primary-muted) !important;
  color: var(--color-text) !important;
}
</style>
