export type YearMarkerItem = { isYearMarker: true; year: number | string; count: number; continued?: boolean }
export type SpacerItem = { isSpacer: true }
export type SlotItem<T> = T | SpacerItem | YearMarkerItem

/**
 * Paginates a sorted list of items into pages of fixed slot count,
 * where year markers and spacers count as slots.
 * Each page has at most `slotsPerPage` slots (e.g. 24 = 4 rows of 6).
 */
export function paginateWithYearMarkers<T extends { anime?: { seasonYear?: number } }>(
  items: T[],
  cardsPerRow: number,
  slotsPerPage: number
): Array<Array<SlotItem<T>>> {
  if (items.length === 0) return []

  // Pre-compute year counts across all items
  const yearCounts = new Map<number | string, number>()
  for (const item of items) {
    const year = item.anime?.seasonYear || 'Unknown'
    yearCounts.set(year, (yearCounts.get(year) || 0) + 1)
  }

  const pages: Array<Array<SlotItem<T>>> = []
  let page: Array<SlotItem<T>> = []
  let pos = 0
  let currentYear: number | string | null = null

  for (const item of items) {
    const year = item.anime?.seasonYear || 'Unknown'

    if (year !== currentYear) {
      // New year: may need spacer + year marker + at least 1 production
      const posInRow = pos % cardsPerRow
      const needSpacer = pos > 0 && posInRow === cardsPerRow - 1
      const slotsNeeded = (needSpacer ? 1 : 0) + 1 + 1 // spacer? + marker + production

      // If it doesn't fit on current page, pad remaining row with spacers and start a new one
      if (pos > 0 && pos + slotsNeeded > slotsPerPage) {
        const remainder = pos % cardsPerRow
        if (remainder > 0) {
          for (let i = 0; i < cardsPerRow - remainder; i++) {
            page.push({ isSpacer: true })
          }
        }
        pages.push(page)
        page = []
        pos = 0
      }

      // Re-check spacer after potential page break
      if (pos > 0 && pos % cardsPerRow === cardsPerRow - 1) {
        page.push({ isSpacer: true })
        pos++
      }

      page.push({ isYearMarker: true, year, count: yearCounts.get(year)!, continued: false })
      pos++
      currentYear = year
    } else if (pos === 0 && pages.length > 0) {
      // Same year continuing on a new page
      page.push({ isYearMarker: true, year, count: yearCounts.get(year)!, continued: true })
      pos++
    }

    // Add the production
    page.push(item)
    pos++

    // If page is full, start a new page
    if (pos >= slotsPerPage) {
      pages.push(page)
      page = []
      pos = 0
    }
  }

  if (page.length > 0) {
    pages.push(page)
  }

  return pages
}

/**
 * Flattens a sorted list of items with inline year markers
 * Adds spacers to prevent year markers from appearing at the end of rows
 */
export function flattenWithYearMarkers<T extends { anime?: { seasonYear?: number } }>(
  items: T[],
  cardsPerRow: number,
  continuedFromYear?: number | string | null
): Array<T | { isSpacer: true } | { isYearMarker: true; year: number | string; count: number; continued?: boolean }> {
  const flattened: Array<T | { isSpacer: true } | { isYearMarker: true; year: number | string; count: number; continued?: boolean }> = []

  // Pre-compute year counts in a single pass
  const yearCounts = new Map<number | string, number>()
  for (const item of items) {
    const year = item.anime?.seasonYear || 'Unknown'
    yearCounts.set(year, (yearCounts.get(year) || 0) + 1)
  }

  let currentYear: number | string | null = null
  let position = 0

  for (const item of items) {
    const year = item.anime?.seasonYear || 'Unknown'

    if (year !== currentYear) {
      // Avoid orphaning year marker at end of row
      const positionInRow = position % cardsPerRow
      if (position > 0 && positionInRow === cardsPerRow - 1) {
        flattened.push({ isSpacer: true })
        position++
      }

      const continued = currentYear === null && continuedFromYear != null && year === continuedFromYear
      flattened.push({ isYearMarker: true, year, count: yearCounts.get(year)!, continued })
      position++
      currentYear = year
    }

    flattened.push(item)
    position++
  }

  return flattened
}
