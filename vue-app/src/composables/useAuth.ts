/**
 * Composable for authentication state management
 */

import { ref, computed } from 'vue'
import { api } from '@/utils/api'
import { useCsrf } from '@/composables/useCsrf'
import { useUserPreferences } from '@/composables/useUserPreferences'

interface User {
  userId: string;
  email: string;
  name: string;
  picture: string;
  isAnonymous: boolean;
  createdAt: string;
}

const user = ref<User | null>(null);
const loading = ref(false);

export const useAuth = () => {
  const { csrfPost } = useCsrf();
  const isAuthenticated = computed(() => user.value !== null && !user.value.isAnonymous);
  const isAnonymous = computed(() => user.value?.isAnonymous ?? true);

  /**
   * Fetch current user info
   */
  const fetchUser = async () => {
    loading.value = true;
    try {
      const response = await api<any>('/auth/me');
      if (response.authenticated && response.user) {
        user.value = response.user;
        // Load server-stored preferences for authenticated (non-anonymous) users
        if (!response.user.isAnonymous) {
          const { loadPreferences } = useUserPreferences();
          await loadPreferences();
        }
      } else {
        user.value = null;
      }
    } catch (error) {
      console.error('Failed to fetch user:', error);
      user.value = null;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Login with Google (redirects to OAuth flow)
   */
  const loginWithGoogle = (returnUrl?: string) => {
    // Store current anonymous user ID if exists
    const anonymousId = localStorage.getItem('anigraph_user_id');
    if (anonymousId) {
      document.cookie = `anigraph_anonymous_id=${anonymousId}; path=/; max-age=3600`; // 1 hour
    }

    // Store the return URL so we can redirect back after login
    const url = returnUrl || window.location.pathname + window.location.search;
    document.cookie = `oauth_return_url=${encodeURIComponent(url)}; path=/; max-age=3600`;

    // Redirect to Google OAuth
    window.location.href = '/api/auth/google/login';
  };

  /**
   * Logout
   */
  const logout = async () => {
    try {
      await csrfPost('/api/auth/logout');
      user.value = null;

      // Create new anonymous user ID
      const newAnonymousId = crypto.randomUUID();
      localStorage.setItem('anigraph_user_id', newAnonymousId);

      // Redirect back to current page
      window.location.href = window.location.pathname + window.location.search;
    } catch (error) {
      console.error('Logout failed:', error);
    }
  };

  /**
   * Get current user ID (authenticated or anonymous)
   */
  const getUserId = (): string => {
    if (user.value && !user.value.isAnonymous) {
      return user.value.userId;
    }

    // Fall back to anonymous localStorage ID
    let anonymousId = localStorage.getItem('anigraph_user_id');
    if (!anonymousId) {
      anonymousId = crypto.randomUUID();
      localStorage.setItem('anigraph_user_id', anonymousId);
    }
    return anonymousId;
  };

  return {
    user,
    loading,
    isAuthenticated,
    isAnonymous,
    fetchUser,
    loginWithGoogle,
    logout,
    getUserId,
  };
};
