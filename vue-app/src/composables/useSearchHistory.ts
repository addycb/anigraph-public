export const useSearchHistory = () => {
  const STORAGE_KEY = 'anigraph_search_history'
  const MAX_HISTORY = 5

  const getHistory = (): string[] => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY)
      return stored ? JSON.parse(stored) : []
    } catch {
      return []
    }
  }

  const addToHistory = (query: string) => {
    if (!query || query.trim().length < 2) return

    try {
      const history = getHistory()
      const normalized = query.trim()

      // Remove if already exists (to move to front)
      const filtered = history.filter(item => item !== normalized)

      // Add to front and limit to MAX_HISTORY
      const newHistory = [normalized, ...filtered].slice(0, MAX_HISTORY)

      localStorage.setItem(STORAGE_KEY, JSON.stringify(newHistory))
    } catch (error) {
      console.error('Failed to save search history:', error)
    }
  }

  const removeFromHistory = (query: string) => {
    try {
      const history = getHistory()
      const filtered = history.filter(item => item !== query)
      localStorage.setItem(STORAGE_KEY, JSON.stringify(filtered))
    } catch (error) {
      console.error('Failed to remove from search history:', error)
    }
  }

  const clearHistory = () => {
    try {
      localStorage.removeItem(STORAGE_KEY)
    } catch (error) {
      console.error('Failed to clear search history:', error)
    }
  }

  return {
    getHistory,
    addToHistory,
    removeFromHistory,
    clearHistory
  }
}
