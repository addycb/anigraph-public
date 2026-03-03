/**
 * Composable for syncing user preferences (theme, adult content) with the server.
 * Only active for authenticated (non-anonymous) users.
 * Anonymous users continue using localStorage only.
 */

import { ref } from 'vue'
import { api } from '@/utils/api'
import { useCsrf } from '@/composables/useCsrf'
import { useAppTheme } from '@/composables/useTheme'

// Module-level refs to directly set shared state without triggering server re-save
const appTheme = ref<string>(localStorage.getItem('anigraph_theme') || 'scholar-light')
const includeAdultState = ref<boolean>(localStorage.getItem('anigraph_includeAdult') === 'true')

export const useUserPreferences = () => {
  const { getCsrfHeaders } = useCsrf()

  /**
   * Load preferences from server and apply them locally.
   * Directly sets shared state to avoid triggering a server re-save.
   */
  const loadPreferences = async () => {
    try {
      const data = await api<{ theme: string; includeAdult: boolean }>('/user/preferences')

      // Apply theme: update shared state + localStorage + CSS vars
      appTheme.value = data.theme
      localStorage.setItem('anigraph_theme', data.theme)
      const { applyTheme } = useAppTheme()
      applyTheme(data.theme)

      // Apply adult content setting: update shared state + localStorage
      includeAdultState.value = data.includeAdult
      localStorage.setItem('anigraph_includeAdult', String(data.includeAdult))
    } catch {
      // Silently fail -- localStorage values remain as fallback
    }
  }

  /**
   * Save one or both preferences to the server (fire-and-forget).
   * Failures are silent -- localStorage already has the updated value.
   */
  const savePreferences = async (prefs: { theme?: string; includeAdult?: boolean }) => {
    try {
      const headers = await getCsrfHeaders()
      await api('/user/preferences', {
        method: 'PATCH',
        headers,
        body: JSON.stringify(prefs),
      })
    } catch {
      // Silently fail
    }
  }

  return { loadPreferences, savePreferences }
}
