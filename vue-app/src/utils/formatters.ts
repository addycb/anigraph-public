/**
 * Format anime season for display
 * Converts UPPER_CASE to Title Case
 */
export function formatSeason(season: string | null | undefined): string {
  if (!season) return ''

  const seasonMap: Record<string, string> = {
    'WINTER': 'Winter',
    'SPRING': 'Spring',
    'SUMMER': 'Summer',
    'FALL': 'Fall'
  }

  return seasonMap[season.toUpperCase()] || season
}

/**
 * Format anime format for display
 * Converts UPPER_CASE and SNAKE_CASE to human-readable format
 */
export function formatAnimeFormat(format: string | null | undefined): string {
  if (!format) return ''

  const formatMap: Record<string, string> = {
    'TV': 'TV',
    'TV_SHORT': 'TV Short',
    'MOVIE': 'Movie',
    'SPECIAL': 'Special',
    'OVA': 'OVA',
    'ONA': 'ONA',
    'MUSIC': 'Music',
    'MANGA': 'Manga',
    'NOVEL': 'Novel',
    'LIGHT_NOVEL': 'Light Novel',
    'ONE_SHOT': 'One Shot'
  }

  return formatMap[format.toUpperCase()] || format
}

/**
 * Format season and year together
 * Always includes a space between season and year
 */
export function formatSeasonYear(season: string | null | undefined, year: number | null | undefined): string {
  const parts: string[] = []

  if (season) {
    parts.push(formatSeason(season))
  }

  if (year) {
    parts.push(String(year))
  }

  return parts.join(' ')
}
