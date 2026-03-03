/**
 * Composable for managing user favorites (which is now just a special list)
 * This wraps useLists to provide a favorites-specific API
 */

import { computed } from 'vue'
import { useAuth } from '@/composables/useAuth'
import { useLists } from '@/composables/useLists'

export const useFavorites = () => {
  const { getUserId, isAuthenticated } = useAuth();
  const { userLists, fetchLists, listsLoading, listsLoaded, addToList, removeFromList } = useLists();

  /**
   * Get the favorites list (special list with listType='favorites')
   */
  const favoritesList = computed(() => {
    return userLists.value.find(list => list.listType === 'favorites');
  });

  /**
   * Set of favorited anime IDs for quick lookup
   */
  const favoritedAnimeIds = computed(() => {
    const list = favoritesList.value;
    if (!list || !list.items) return new Set<number>();
    return new Set(list.items);
  });

  /**
   * Check if an anime is favorited
   */
  const isFavorited = (animeId: number | string): boolean => {
    const id = typeof animeId === 'string' ? parseInt(animeId) : animeId;
    return favoritedAnimeIds.value.has(id);
  };

  /**
   * Add an anime to favorites
   */
  const addFavorite = async (animeId: number | string): Promise<boolean> => {
    const list = favoritesList.value;
    if (!list) {
      console.error('Favorites list not found');
      return false;
    }

    return await addToList(list.id, animeId);
  };

  /**
   * Remove an anime from favorites
   */
  const removeFavorite = async (animeId: number | string): Promise<boolean> => {
    const list = favoritesList.value;
    if (!list) {
      console.error('Favorites list not found');
      return false;
    }

    return await removeFromList(list.id, animeId);
  };

  /**
   * Toggle favorite status
   */
  const toggleFavorite = async (animeId: number | string): Promise<boolean> => {
    const favorited = isFavorited(animeId);
    return favorited ? removeFavorite(animeId) : addFavorite(animeId);
  };

  /**
   * Fetch favorites (delegates to fetchLists)
   */
  const fetchFavorites = async (force = false) => {
    await fetchLists(force);
  };

  /**
   * Clear favorites cache (delegates to lists)
   */
  const clearCache = () => {
    const { clearCache: clearListsCache } = useLists();
    clearListsCache();
  };

  return {
    favoritesList,
    favoritedAnimeIds,
    favoritesLoaded: listsLoaded,
    favoritesLoading: listsLoading,
    isFavorited,
    addFavorite,
    removeFavorite,
    toggleFavorite,
    fetchFavorites,
    clearCache,
  };
};
