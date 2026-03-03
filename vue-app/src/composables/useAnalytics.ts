/**
 * Analytics composable for Umami event tracking
 * Provides consistent tracking methods across the application
 */

// Event types for type safety
export type AnalyticsEvent =
  | 'search'
  | 'search_select'
  | 'filter_apply'
  | 'background_generate'
  | 'settings_change'

export interface AnalyticsEventData {
  [key: string]: string | number | boolean | undefined
}

export const useAnalytics = () => {
  /**
   * Track a custom event with Umami
   * @param eventName - The name of the event to track
   * @param data - Optional data to include with the event
   */
  const trackEvent = (eventName: AnalyticsEvent | string, data?: AnalyticsEventData) => {
    if (typeof window !== 'undefined' && (window as any).umami) {
      (window as any).umami.track(eventName, data)
    }
  }

  /**
   * Track a search event
   * @param query - The search query
   * @param source - Where the search originated (e.g., 'appbar', 'home', 'floating', 'advanced')
   */
  const trackSearch = (query: string, source: string = 'unknown') => {
    trackEvent('search', { query, source })
  }

  /**
   * Track when a user selects a search result
   * @param type - The type of result selected (anime, staff, studio)
   * @param id - The ID of the selected item
   * @param source - Where the selection occurred
   */
  const trackSearchSelect = (type: string, id: string | number, source: string = 'unknown') => {
    trackEvent('search_select', { type, id: String(id), source })
  }

  /**
   * Track filter application
   * @param filterType - The type of filter applied (genre, tag, format, etc.)
   * @param value - The filter value(s)
   * @param page - The page where the filter was applied
   */
  const trackFilterApply = (filterType: string, value: string | string[], page: string) => {
    const valueStr = Array.isArray(value) ? value.join(',') : value
    trackEvent('filter_apply', { filterType, value: valueStr, page })
  }

  /**
   * Track background generation
   * @param source - The source type (studio, staff, custom)
   * @param count - Number of anime in the background
   * @param tileSize - The tile size selected
   */
  const trackBackgroundGenerate = (source: string, count: number, tileSize: string) => {
    trackEvent('background_generate', { source, count, tileSize })
  }

  /**
   * Track settings changes
   * @param setting - The setting that was changed
   * @param value - The new value
   */
  const trackSettingsChange = (setting: string, value: string | boolean) => {
    trackEvent('settings_change', { setting, value: String(value) })
  }

  return {
    trackEvent,
    trackSearch,
    trackSearchSelect,
    trackFilterApply,
    trackBackgroundGenerate,
    trackSettingsChange
  }
}
