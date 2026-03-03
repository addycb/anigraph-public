/**
 * Sort items by year and/or score with configurable order
 * Works with productions, filmography, or any items that have anime.seasonYear and anime.averageScore
 */
export function sortByYearAndScore<T extends { anime?: any }>(
  items: T[],
  sortBy: 'year' | 'score',
  sortOrder: 'asc' | 'desc' = 'desc'
): T[] {
  return [...items].sort((a, b) => {
    if (sortBy === 'score') {
      // Sort by score
      const scoreA = a.anime?.averageScore || 0
      const scoreB = b.anime?.averageScore || 0
      const scoreComparison = sortOrder === 'desc' ? scoreB - scoreA : scoreA - scoreB
      if (scoreComparison !== 0) return scoreComparison
    }

    // Sort by year (primary for 'year' sort, secondary for 'score' sort)
    const yearA = a.anime?.seasonYear || 0
    const yearB = b.anime?.seasonYear || 0
    const yearComparison = sortOrder === 'desc' ? yearB - yearA : yearA - yearB
    if (yearComparison !== 0) return yearComparison

    // Then by season (Winter=1, Spring=2, Summer=3, Fall=4)
    const seasonOrder: Record<string, number> = {
      'WINTER': 1,
      'SPRING': 2,
      'SUMMER': 3,
      'FALL': 4
    }
    const seasonA = seasonOrder[a.anime?.season] || 0
    const seasonB = seasonOrder[b.anime?.season] || 0
    const seasonComparison = sortOrder === 'desc' ? seasonB - seasonA : seasonA - seasonB
    if (seasonComparison !== 0) return seasonComparison

    // Finally by title alphabetically
    const titleA = a.anime?.title || ''
    const titleB = b.anime?.title || ''
    return titleA.localeCompare(titleB)
  })
}
