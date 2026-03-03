/**
 * Filters adult content from a list of items based on user settings
 * Works with productions, filmography, or any items that have anime.isAdult
 */
export function filterAdultContent<T extends { anime?: { isAdult?: boolean } }>(
  items: T[],
  includeAdult: boolean
): T[] {
  if (includeAdult) {
    return items
  }
  return items.filter((item) => !item.anime?.isAdult)
}
