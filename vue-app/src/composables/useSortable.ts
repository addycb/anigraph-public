/**
 * Composable for managing sortable state (sort by, sort order)
 */
import { ref, computed } from 'vue'

export function useSortable(
  defaultSortBy: 'year' | 'score' = 'year',
  defaultSortOrder: 'asc' | 'desc' = 'desc'
) {
  const sortBy = ref<'year' | 'score'>(defaultSortBy)
  const sortOrder = ref<'asc' | 'desc'>(defaultSortOrder)

  const sortOptions = [
    { value: 'year', title: 'Year' },
    { value: 'score', title: 'Score' }
  ]

  const toggleSortOrder = () => {
    sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  }

  return {
    sortBy,
    sortOrder,
    sortOptions,
    toggleSortOrder
  }
}
