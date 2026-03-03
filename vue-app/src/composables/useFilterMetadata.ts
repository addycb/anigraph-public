/**
 * Global composable for filter metadata
 * Loads once and caches across all pages
 * Supports both bitmap (integer IDs) and string formats
 */

import { ref, readonly } from 'vue'
import { api } from '@/utils/api'

// ── Types ──────────────────────────────────────────────

/** Anime metadata in bitmap format (compact integer IDs) */
interface BitmapAnimeMetadata {
  id: number
  s?: number[]   // studio IDs
  g?: number[]   // genre IDs
  t?: number[]   // tag IDs
}

/** Anime metadata in string format */
interface StringAnimeMetadata {
  id: number
  studios?: string[]
  genres?: string[]
  tags?: string[]
}

type AnimeMetadata = BitmapAnimeMetadata | StringAnimeMetadata

interface LookupTables {
  studios: string[]
  genres: string[]
  tags: string[]
}

interface FilterMetadataResponse {
  success: boolean
  data: AnimeMetadata[]
  count: number
  cached?: boolean
  useBitmaps?: boolean
  lookups?: LookupTables
}

interface FilterCounts {
  studios: Record<string, number>
  genres: Record<string, number>
  tags: Record<string, number>
}

// ── Global state — shared across all component instances ──

const filterMetadata = ref<AnimeMetadata[]>([])
const filterMetadataLoaded = ref(false)
const loadingFilterMetadata = ref(false)
const metadataError = ref<string | null>(null)
const useBitmaps = ref(false)
const lookupTables = ref<LookupTables | null>(null)

export const useFilterMetadata = () => {
  const loadFilterMetadata = async (force = false) => {
    if ((filterMetadataLoaded.value || loadingFilterMetadata.value) && !force) {
      return
    }

    loadingFilterMetadata.value = true
    metadataError.value = null

    try {
      const response = await api<FilterMetadataResponse>('/anime/filter-metadata')
      if (response.success) {
        filterMetadata.value = response.data
        useBitmaps.value = response.useBitmaps || false
        lookupTables.value = response.lookups || null
        filterMetadataLoaded.value = true
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to load filter metadata'
      metadataError.value = message
    } finally {
      loadingFilterMetadata.value = false
    }
  }

  const getFilteredMetadata = (excludeAnimeId?: number) => {
    if (!filterMetadata.value) return []
    if (!excludeAnimeId) {
      return filterMetadata.value
    }
    return filterMetadata.value.filter(anime => anime.id !== excludeAnimeId)
  }

  /** Get the studio/genre/tag array from an anime metadata item, handling both formats */
  const getAnimeStudios = (anime: AnimeMetadata): (number | string)[] =>
    useBitmaps.value ? (anime as BitmapAnimeMetadata).s || [] : (anime as StringAnimeMetadata).studios || []

  const getAnimeGenres = (anime: AnimeMetadata): (number | string)[] =>
    useBitmaps.value ? (anime as BitmapAnimeMetadata).g || [] : (anime as StringAnimeMetadata).genres || []

  const getAnimeTags = (anime: AnimeMetadata): (number | string)[] =>
    useBitmaps.value ? (anime as BitmapAnimeMetadata).t || [] : (anime as StringAnimeMetadata).tags || []

  /** Resolve a display name to its bitmap ID (or pass through in string mode) */
  const resolveId = (name: string, table: string[]): number | string => {
    if (!useBitmaps.value || !lookupTables.value) return name
    return table.indexOf(name)
  }

  const resolveIds = (names: string[], table: string[]): (number | string)[] => {
    if (!useBitmaps.value || !lookupTables.value) return names
    return names.map(name => table.indexOf(name)).filter(id => id !== -1)
  }

  const calculateFilterCounts = (
    currentFilters: { studios: string[], genres: string[], tags: string[] },
    checkStudios: string[],
    checkGenres: string[],
    checkTags: string[],
    excludeAnimeId?: number
  ): FilterCounts => {
    if (!filterMetadataLoaded.value) {
      return { studios: {}, genres: {}, tags: {} }
    }

    const counts: FilterCounts = {
      studios: {},
      genres: {},
      tags: {}
    }

    // Initialize all to 0
    checkStudios.forEach(s => counts.studios[s] = 0)
    checkGenres.forEach(g => counts.genres[g] = 0)
    checkTags.forEach(t => counts.tags[t] = 0)

    const tables = lookupTables.value
    const currentStudioIds = resolveIds(currentFilters.studios, tables?.studios || [])
    const currentGenreIds = resolveIds(currentFilters.genres, tables?.genres || [])
    const currentTagIds = resolveIds(currentFilters.tags, tables?.tags || [])

    const hasNoFilters = currentFilters.studios.length === 0 &&
                         currentFilters.genres.length === 0 &&
                         currentFilters.tags.length === 0

    if (hasNoFilters) {
      // Build reverse lookup maps: ID → display name
      const studioIdToName = new Map<number | string, string>()
      const genreIdToName  = new Map<number | string, string>()
      const tagIdToName    = new Map<number | string, string>()

      checkStudios.forEach(name => {
        const id = resolveId(name, tables?.studios || [])
        if (id !== -1) studioIdToName.set(id, name)
      })
      checkGenres.forEach(name => {
        const id = resolveId(name, tables?.genres || [])
        if (id !== -1) genreIdToName.set(id, name)
      })
      checkTags.forEach(name => {
        const id = resolveId(name, tables?.tags || [])
        if (id !== -1) tagIdToName.set(id, name)
      })

      filterMetadata.value.forEach(anime => {
        if (excludeAnimeId && anime.id === excludeAnimeId) return

        getAnimeStudios(anime).forEach(id => {
          const name = studioIdToName.get(id)
          if (name !== undefined) counts.studios[name]++
        })
        getAnimeGenres(anime).forEach(id => {
          const name = genreIdToName.get(id)
          if (name !== undefined) counts.genres[name]++
        })
        getAnimeTags(anime).forEach(id => {
          const name = tagIdToName.get(id)
          if (name !== undefined) counts.tags[name]++
        })
      })

      return counts
    }

    // Filtered path: only count anime that match current filters
    const matchingAnime = filterMetadata.value.filter(anime => {
      if (excludeAnimeId && anime.id === excludeAnimeId) return false

      const animeStudios = getAnimeStudios(anime)
      const animeGenres = getAnimeGenres(anime)
      const animeTags = getAnimeTags(anime)

      const studioMatch = currentStudioIds.length === 0 ||
        currentStudioIds.every(id => animeStudios.includes(id))

      const genreMatch = currentGenreIds.length === 0 ||
        currentGenreIds.every(id => animeGenres.includes(id))

      const tagMatch = currentTagIds.length === 0 ||
        currentTagIds.every(id => animeTags.includes(id))

      return studioMatch && genreMatch && tagMatch
    })

    matchingAnime.forEach(anime => {
      const animeStudios = getAnimeStudios(anime)
      const animeGenres = getAnimeGenres(anime)
      const animeTags = getAnimeTags(anime)

      checkStudios.forEach(studioName => {
        const id = resolveId(studioName, tables?.studios || [])
        if (animeStudios.includes(id)) counts.studios[studioName]++
      })

      checkGenres.forEach(genreName => {
        const id = resolveId(genreName, tables?.genres || [])
        if (animeGenres.includes(id)) counts.genres[genreName]++
      })

      checkTags.forEach(tagName => {
        const id = resolveId(tagName, tables?.tags || [])
        if (animeTags.includes(id)) counts.tags[tagName]++
      })
    })

    return counts
  }

  return {
    filterMetadata: readonly(filterMetadata),
    filterMetadataLoaded: readonly(filterMetadataLoaded),
    loadingFilterMetadata: readonly(loadingFilterMetadata),
    metadataError: readonly(metadataError),
    useBitmaps: readonly(useBitmaps),
    lookupTables: readonly(lookupTables),
    loadFilterMetadata,
    getFilteredMetadata,
    calculateFilterCounts
  }
}
