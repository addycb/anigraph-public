/**
 * Composable for working with anime seasons
 */
export const useSeason = () => {
  /**
   * Get the current anime season based on the current date
   * Returns { season: 'WINTER' | 'SPRING' | 'SUMMER' | 'FALL', year: number }
   */
  const getCurrentSeason = () => {
    const now = new Date()
    const month = now.getMonth() + 1 // 1-12
    const year = now.getFullYear()

    let season: 'WINTER' | 'SPRING' | 'SUMMER' | 'FALL'

    if (month >= 1 && month <= 3) {
      season = 'WINTER'
    } else if (month >= 4 && month <= 6) {
      season = 'SPRING'
    } else if (month >= 7 && month <= 9) {
      season = 'SUMMER'
    } else {
      season = 'FALL'
    }

    return { season, year }
  }

  /**
   * Get the next anime season after the given one
   */
  const getNextSeason = (season: string, year: number) => {
    const order: Array<'WINTER' | 'SPRING' | 'SUMMER' | 'FALL'> = ['WINTER', 'SPRING', 'SUMMER', 'FALL']
    const idx = order.indexOf(season as typeof order[number])
    if (idx === 3) {
      return { season: 'WINTER' as const, year: year + 1 }
    }
    return { season: order[idx + 1], year }
  }

  /**
   * Check if an anime is from the current season
   */
  const isCurrentSeason = (animeSeason?: string, animeYear?: number) => {
    if (!animeSeason || !animeYear) return false

    const current = getCurrentSeason()
    return animeSeason.toUpperCase() === current.season && animeYear === current.year
  }

  /**
   * Check if an anime is from the current or next season
   */
  const isCurrentOrNextSeason = (animeSeason?: string, animeYear?: number) => {
    if (!animeSeason || !animeYear) return false

    const upper = animeSeason.toUpperCase()
    const current = getCurrentSeason()
    if (upper === current.season && animeYear === current.year) return true

    const next = getNextSeason(current.season, current.year)
    return upper === next.season && animeYear === next.year
  }

  return {
    getCurrentSeason,
    getNextSeason,
    isCurrentSeason,
    isCurrentOrNextSeason
  }
}
