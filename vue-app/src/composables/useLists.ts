/**
 * Composable for managing user lists with caching
 */

import { ref, computed } from 'vue'
import { api } from '@/utils/api'
import { useAuth } from '@/composables/useAuth'
import { useCsrf } from '@/composables/useCsrf'
import { useSettings } from '@/composables/useSettings'

interface UserList {
  id: number;
  userId: string;
  name: string;
  description?: string;
  isPublic: boolean;
  shareToken?: string;
  listType?: string; // 'custom' or 'favorites'
  createdAt: string;
  updatedAt: string;
  itemCount?: number;
  items?: number[]; // anime IDs
}

const userLists = ref<UserList[]>([]);
const listsLoading = ref(false);
const listsLoaded = ref(false);
let fetchPromise: Promise<void> | null = null;

export const useLists = () => {
  const { getUserId, isAuthenticated } = useAuth();
  const { csrfPost, csrfPatch, csrfDelete } = useCsrf();

  /**
   * Fetch and cache user's lists
   */
  const fetchLists = async (force = false, includeAdultOverride?: boolean) => {
    if (!isAuthenticated.value) {
      userLists.value = [];
      listsLoaded.value = false;
      return;
    }

    // Don't refetch if already loaded unless forced
    if (listsLoaded.value && !force) {
      return;
    }

    // If already fetching, return the existing promise to deduplicate calls
    if (fetchPromise && !force) {
      return fetchPromise;
    }

    const doFetch = async () => {
      listsLoading.value = true;
      try {
        const { includeAdult } = useSettings();
        const showAdult = includeAdultOverride !== undefined ? includeAdultOverride : includeAdult.value;

        const response = await api<any>('/user/lists', {
          params: {
            includeAdult: String(showAdult)
          }
        });

        if (response.success && response.data) {
          userLists.value = response.data;
          listsLoaded.value = true;
        }
      } catch (error) {
        console.error('Error fetching lists:', error);
      } finally {
        listsLoading.value = false;
        fetchPromise = null;
      }
    };

    fetchPromise = doFetch();
    return fetchPromise;
  };

  /**
   * Create a new list
   * @throws Error with message if creation fails
   */
  const createList = async (name: string, description?: string, isPublic = false): Promise<UserList> => {
    const response = await csrfPost<any>('/api/user/lists', {
      name,
      description,
      isPublic
    });

    if (response.success && response.data) {
      userLists.value.push(response.data);
      return response.data;
    }
    throw new Error('Failed to create list');
  };

  /**
   * Update a list
   * @throws Error with message if update fails
   */
  const updateList = async (listId: number, updates: Partial<Pick<UserList, 'name' | 'description' | 'isPublic'>>): Promise<void> => {
    const response = await csrfPatch<any>(`/api/user/lists/${listId}`, updates);

    if (response.success && response.data) {
      const index = userLists.value.findIndex(l => l.id === listId);
      if (index !== -1) {
        userLists.value[index] = response.data;
      }
      return;
    }
    throw new Error('Failed to update list');
  };

  /**
   * Delete a list
   */
  const deleteList = async (listId: number): Promise<boolean> => {
    try {
      const response = await csrfDelete<any>(`/api/user/lists/${listId}`);

      if (response.success) {
        userLists.value = userLists.value.filter(l => l.id !== listId);
        return true;
      }
      return false;
    } catch (error) {
      console.error('Error deleting list:', error);
      return false;
    }
  };

  /**
   * Check if an anime is in a specific list
   */
  const isInList = (listId: number, animeId: number | string): boolean => {
    const list = userLists.value.find(l => l.id === listId);
    if (!list || !list.items) return false;
    const id = typeof animeId === 'string' ? parseInt(animeId) : animeId;
    return list.items.includes(id);
  };

  /**
   * Get all lists that contain an anime
   */
  const getListsForAnime = (animeId: number | string): UserList[] => {
    const id = typeof animeId === 'string' ? parseInt(animeId) : animeId;
    return userLists.value.filter(list => list.items?.includes(id) || false);
  };

  /**
   * Add anime to a list
   */
  const addToList = async (listId: number, animeId: number | string): Promise<boolean> => {
    const id = typeof animeId === 'string' ? parseInt(animeId) : animeId;

    // Optimistic update
    const list = userLists.value.find(l => l.id === listId);
    if (list) {
      if (!list.items) list.items = [];
      if (!list.items.includes(id)) {
        list.items.push(id);
        if (list.itemCount !== undefined) list.itemCount++;
      }
    }

    try {
      const response = await csrfPost<any>(`/api/user/lists/${listId}/items`, {
        animeId: id
      });

      return response.success;
    } catch (error) {
      console.error('Error adding to list:', error);
      // Rollback
      if (list && list.items) {
        const index = list.items.indexOf(id);
        if (index !== -1) {
          list.items.splice(index, 1);
          if (list.itemCount !== undefined) list.itemCount--;
        }
      }
      return false;
    }
  };

  /**
   * Remove anime from a list
   */
  const removeFromList = async (listId: number, animeId: number | string): Promise<boolean> => {
    const id = typeof animeId === 'string' ? parseInt(animeId) : animeId;

    // Optimistic update
    const list = userLists.value.find(l => l.id === listId);
    if (list && list.items) {
      const index = list.items.indexOf(id);
      if (index !== -1) {
        list.items.splice(index, 1);
        if (list.itemCount !== undefined) list.itemCount--;
      }
    }

    try {
      const response = await csrfDelete<any>(`/api/user/lists/${listId}/items`, {
        animeId: id
      });

      return response.success;
    } catch (error) {
      console.error('Error removing from list:', error);
      // Rollback
      if (list) {
        if (!list.items) list.items = [];
        if (!list.items.includes(id)) {
          list.items.push(id);
          if (list.itemCount !== undefined) list.itemCount++;
        }
      }
      return false;
    }
  };

  /**
   * Toggle anime in a list
   */
  const toggleInList = async (listId: number, animeId: number | string): Promise<boolean> => {
    const inList = isInList(listId, animeId);
    return inList ? removeFromList(listId, animeId) : addToList(listId, animeId);
  };

  /**
   * Clear lists cache (e.g., on logout)
   */
  const clearCache = () => {
    userLists.value = [];
    listsLoaded.value = false;
  };

  return {
    userLists: computed(() => userLists.value),
    listsLoading: computed(() => listsLoading.value),
    listsLoaded: computed(() => listsLoaded.value),
    fetchLists,
    createList,
    updateList,
    deleteList,
    isInList,
    getListsForAnime,
    addToList,
    removeFromList,
    toggleInList,
    clearCache,
  };
};
