<template>
  <RouterLink :to="animeLink" class="anime-card-link" @click.capture="handleCardClick">
      <v-card
        ref="cardRef"
        class="anime-card"
        :class="{ 'anime-card--no-hover': disableHover, 'touch-active': touchActive }"
        hover
        rounded="0"
        @touchstart="handleTouchStart"
        @touchmove="handleTouchMove"
        @touchend="handleTouchEnd"
      >
    <v-img
      :src="imageSource"
      :alt="anime.title"
      :aspect-ratio="imageAspectRatio"
      :cover="!isLandscapeImage"
      class="anime-card__image"
      :class="{ 'anime-card__image--landscape': isLandscapeImage }"
    >
      <!-- Glasspane overlay on hover -->
      <div class="anime-card__glasspane"></div>

      <!-- Top Left: New badge + shared staff chip -->
      <div class="anime-card__overlay-left">
        <v-chip
          v-if="isThisSeason"
          color="success"
          size="small"
          class="ma-2 anime-card__season"
        >
          <v-icon start>mdi-calendar-star</v-icon>
          New
        </v-chip>
        <v-chip
          v-if="(showStaffCount || (staffByRole && staffByRole.length > 0)) && anime.staffCount !== undefined"
          size="small"
          class="ma-2 anime-card__staff-chip"
        >
          <v-icon start>mdi-account-group</v-icon>
          {{ anime.staffCount }} {{ staffCountLabel }}
        </v-chip>
      </div>

      <!-- Hover overlay: shared staff by role, description, or action buttons -->
      <div v-if="hasOverlayContent" ref="overlayRef" class="anime-card__description-overlay">
        <!-- Action buttons in overlay -->
        <div v-if="anime.anilistId" class="anime-card__overlay-buttons" @click.stop.prevent>
          <v-btn
            icon
            class="favorite-btn"
            :class="{ 'favorited': isFavorited }"
            @click.stop.prevent="handleToggleFavorite"
            size="small"
            variant="flat"
          >
            <v-icon size="20">
              {{ isFavorited ? 'mdi-heart' : 'mdi-heart-outline' }}
            </v-icon>
          </v-btn>
          <div @click.stop.prevent>
            <ListButton :anime-id="anime.anilistId" bubble-mode size="small" />
          </div>
        </div>
        <div v-if="staffByRole && staffByRole.length > 0" ref="scrollableRef" class="staff-by-role-list">
          <div class="text-caption font-weight-bold mb-1">Shared Staff by Role:</div>
          <div v-for="item in staffByRole" :key="item.category" class="d-flex align-center ga-2 mb-1">
            <div class="staff-role-dot" :style="{ backgroundColor: item.color }"></div>
            <span class="text-caption">{{ item.category }}: {{ item.count }}</span>
          </div>
        </div>
        <p v-else-if="truncatedDescription" ref="scrollableRef" class="description-text" v-html="truncatedDescription"></p>
      </div>
    </v-img>

    <v-card-title class="anime-card__title" :title="displayTitle">
      {{ displayTitle }}
    </v-card-title>

    <!-- Compact Layout (for staff page) -->
    <template v-if="compactLayout">
      <!-- Role (left) | Season/Year (right) -->
      <v-card-subtitle class="anime-card__metadata text-caption pa-2 pt-0">
        <div class="d-flex justify-space-between align-center">
          <span
            v-if="role"
            class="anime-card__role-compact"
            :title="roleDisplay"
          >
            {{ roleDisplay }}
          </span>
          <span v-else class="text-medium-emphasis">—</span>
          <span class="anime-card__season-year-compact">
            {{ compactSeasonYearText }}
          </span>
        </div>
      </v-card-subtitle>

      <!-- Format (left) | Score (right) -->
      <v-card-text class="pa-2 pt-0">
        <div class="d-flex justify-space-between align-center">
          <span v-if="anime.format" class="text-caption text-medium-emphasis">
            {{ formatAnimeFormat(anime.format) }}
          </span>
          <span v-else class="text-caption text-medium-emphasis">—</span>
          <ScoreChip :score="anime.averageScore" style-variant="default" />
        </div>
      </v-card-text>
    </template>

    <!-- Default Layout -->
    <template v-else>
      <!-- Role line -->
      <div v-if="role" class="anime-card__role" :title="roleDisplay">
        {{ roleDisplay }}
      </div>

      <v-card-subtitle v-if="seasonFormatText && hasGenres" class="anime-card__metadata text-caption">
        {{ seasonFormatText }}
      </v-card-subtitle>

      <!-- Season/Format + Score chip (when no genres) -->
      <v-card-text v-if="!hasGenres && (seasonFormatText || anime.averageScore)" class="text-caption text-medium-emphasis pa-2 pt-0">
        <div class="d-flex justify-space-between align-center">
          <div>{{ seasonFormatText }}</div>
          <ScoreChip :score="anime.averageScore" style-variant="default" />
        </div>
      </v-card-text>

      <!-- Score + Genres row (only when genres are present) -->
      <div v-if="hasGenres" class="anime-card__tags">
        <div class="d-flex justify-space-between align-center flex-wrap ga-1">
          <div class="d-flex flex-wrap ga-1">
            <v-chip
              v-for="genre in anime.genres.slice(0, 2)"
              :key="genre"
              :color="getGenreColor(genre)"
              size="x-small"
              label
            >
              {{ genre }}
            </v-chip>
          </div>
          <ScoreChip :score="anime.averageScore" style-variant="default" />
        </div>
      </div>
    </template>

      </v-card>
  </RouterLink>
</template>

<script setup lang="ts">
import DOMPurify from 'dompurify'
import { formatSeason, formatAnimeFormat, formatSeasonYear } from '~/utils/formatters'

interface AnimeCardProps {
  anime: {
    id?: string
    anilistId?: number | string
    title?: string
    titleEnglish?: string
    titleRomaji?: string
    coverImage?: string
    coverImage_extraLarge?: string
    coverImage_large?: string
    coverImage_medium?: string
    averageScore?: number
    season?: string
    seasonYear?: number
    format?: string
    staffCount?: number
    isFavorited?: boolean
    matchScore?: number
    matchReasons?: string[]
    description?: string
    genres?: string[]
  }
  searchContext?: {
    studios?: string[]
    genres?: string[]
    tags?: string[]
  }
  // Optional role information (for staff pages)
  role?: string | string[]
  // Optional layout variants
  compact?: boolean
  compactLayout?: boolean
  showBadges?: boolean
  // Show season (default: false, only year shown)
  showSeason?: boolean
  // Show year (default: true)
  showYear?: boolean
  // Shared staff breakdown for hover overlay (takes priority over description)
  staffByRole?: Array<{ category: string; count: number; color: string }>
  // Disable hover effects (glasspane and overlay buttons)
  disableHover?: boolean
  // Label shown after the staff count number in the chip (default: 'shared')
  staffCountLabel?: string
  // Explicitly opt-in to showing the staff count chip (without needing staffByRole)
  showStaffCount?: boolean
}

const props = withDefaults(defineProps<AnimeCardProps>(), {
  compact: false,
  compactLayout: false,
  showBadges: true,
  showSeason: false,
  showYear: true,
  disableHover: false,
  staffCountLabel: 'shared',
  showStaffCount: false
})

// Overlay scroll redirect: wheel events over the button area are forwarded to the
// description/staff list so the page doesn't scroll (desktop only — wheel is mouse-specific)
const overlayRef = ref<HTMLElement | null>(null)
const scrollableRef = ref<HTMLElement | null>(null)

const handleOverlayWheel = (e: WheelEvent) => {
  const scrollable = scrollableRef.value
  if (!scrollable) return
  // If cursor is over the scrollable content itself, let the browser handle it natively
  // (preserves smooth scrolling / momentum). Only intercept events from the button area.
  if (scrollable.contains(e.target as Node)) return
  // Always prevent page scroll; use scrollBy so the browser applies its own scroll
  // physics (smooth scroll, trackpad momentum, deltaMode handling).
  e.preventDefault()
  // Normalize deltaY to pixels based on deltaMode so speed matches native scrolling
  const lineHeight = 16
  const multiplier = e.deltaMode === 2 ? scrollable.clientHeight : e.deltaMode === 1 ? lineHeight : 1
  scrollable.scrollBy({ top: e.deltaY * multiplier, behavior: 'smooth' })
}

watchPostEffect((onCleanup) => {
  const el = overlayRef.value
  if (!el) return
  el.addEventListener('wheel', handleOverlayWheel, { passive: false })
  onCleanup(() => el.removeEventListener('wheel', handleOverlayWheel))
})

// Touch-active state to prevent hover flicker on mobile
const touchActive = ref(false)
const justActivated = ref(false)
const cardRef = ref<InstanceType<typeof import('vuetify/components').VCard> | null>(null)
let touchStartPos = { x: 0, y: 0 }
let touchMoved = false
const SWIPE_THRESHOLD = 10 // px — movement beyond this counts as a swipe, not a tap

const handleTouchStart = (e: TouchEvent) => {
  const touch = e.touches[0]
  touchStartPos = { x: touch.clientX, y: touch.clientY }
  touchMoved = false
}

const handleTouchMove = (e: TouchEvent) => {
  if (touchMoved) return
  const touch = e.touches[0]
  const dx = Math.abs(touch.clientX - touchStartPos.x)
  const dy = Math.abs(touch.clientY - touchStartPos.y)
  if (dx > SWIPE_THRESHOLD || dy > SWIPE_THRESHOLD) {
    touchMoved = true
  }
}

const handleTouchEnd = () => {
  // Only activate overlay on a genuine tap (no swipe)
  if (!touchMoved && !touchActive.value) {
    touchActive.value = true
    justActivated.value = true
  }
}

const handleCardClick = (e: MouseEvent) => {
  if (justActivated.value) {
    e.stopPropagation()
    e.preventDefault()
    justActivated.value = false
  }
}

const onDocumentTouchStart = (e: TouchEvent) => {
  const cardEl = cardRef.value?.$el as HTMLElement | undefined
  if (cardEl && !cardEl.contains(e.target as Node)) {
    touchActive.value = false
    justActivated.value = false
  }
}

onMounted(() => {
  document.addEventListener('touchstart', onDocumentTouchStart, { passive: true })
})

onUnmounted(() => {
  document.removeEventListener('touchstart', onDocumentTouchStart)
})

const { isCurrentOrNextSeason } = useSeason()
const { isAuthenticated } = useAuth()
const { requireLogin } = useLoginRequired()
const { isFavorited: checkIsFavorited, toggleFavorite: toggleFavoriteCache } = useFavorites()

// Track if the image has a landscape/wide aspect ratio
const isLandscapeImage = ref(false)

// Check image dimensions when the image source changes
const imageSource = computed(() =>
  props.anime.coverImage_extraLarge || props.anime.coverImage_large || props.anime.coverImage || '/placeholder-anime.jpg'
)

watch(imageSource, (newSrc) => {
  if (!newSrc) return

  const img = new Image()
  img.onload = () => {
    // Consider it landscape if width >= height
    isLandscapeImage.value = img.naturalWidth >= img.naturalHeight
  }
  img.src = newSrc
}, { immediate: true })

const imageAspectRatio = computed(() => {
  // Always use the same container height so card footers align consistently
  // Landscape images use object-fit: contain within this same-sized container
  return 0.7
})

const isThisSeason = computed(() => {
  return isCurrentOrNextSeason(props.anime.season, props.anime.seasonYear)
})

const isFavorited = computed(() => {
  const id = props.anime.anilistId || props.anime.id
  return id ? checkIsFavorited(Number(id)) : false
})


const handleToggleFavorite = async (e: MouseEvent) => {
  e.stopPropagation()

  if (!isAuthenticated.value) {
    requireLogin()
    return
  }

  const animeId = props.anime.anilistId || props.anime.id
  if (!animeId) return

  try {
    await toggleFavoriteCache(Number(animeId))
  } catch (error) {
    console.error('Error toggling favorite:', error)
  }
}

const animeLink = computed(() => {
  // Use anilistId if id is not available
  const animeId = props.anime.id || props.anime.anilistId
  const link: { path: string; query?: Record<string, string> } = {
    path: `/anime/${encodeURIComponent(animeId)}`
  }

  // Add search context as query params if provided
  if (props.searchContext) {
    const query: Record<string, string> = {}

    if (props.searchContext.studios && props.searchContext.studios.length > 0) {
      query.searchStudios = props.searchContext.studios.join(',')
    }
    if (props.searchContext.genres && props.searchContext.genres.length > 0) {
      query.searchGenres = props.searchContext.genres.join(',')
    }
    if (props.searchContext.tags && props.searchContext.tags.length > 0) {
      query.searchTags = props.searchContext.tags.join(',')
    }

    if (Object.keys(query).length > 0) {
      link.query = query
    }
  }

  return link
})

const displayTitle = computed(() => {
  return props.anime.titleEnglish || props.anime.title || props.anime.titleRomaji || 'Unknown Anime'
})

const hasGenres = computed(() => props.anime.genres && props.anime.genres.length > 0)

const genreColors = [
  'red', 'pink', 'purple', 'deep-purple', 'indigo', 'blue', 'light-blue', 'cyan',
  'teal', 'green', 'light-green', 'lime', 'yellow', 'amber', 'orange', 'deep-orange',
  'brown', 'blue-grey', 'grey'
]

const genreColorMap = new Map<string, string>()

const getGenreColor = (genre: string): string => {
  if (!genreColorMap.has(genre)) {
    const index = genreColorMap.size % genreColors.length
    genreColorMap.set(genre, genreColors[index])
  }
  return genreColorMap.get(genre)!
}

const roleDisplay = computed(() => {
  if (!props.role) return ''
  return Array.isArray(props.role) ? props.role.join(', ') : props.role
})

const truncatedDescription = computed(() => {
  if (!props.anime.description) return ''

  // Sanitize HTML to allow only basic formatting tags, let CSS line-clamp handle truncation
  return DOMPurify.sanitize(props.anime.description, {
    ALLOWED_TAGS: ['br', 'i', 'b', 'em', 'strong', 'p'],
    ALLOWED_ATTR: [],
    KEEP_CONTENT: true,
  })
})

const seasonFormatText = computed(() => {
  const parts: string[] = []
  const timeParts: string[] = []

  if (props.showSeason && props.anime.season) {
    timeParts.push(formatSeason(props.anime.season))
  }

  if (props.showYear && props.anime.seasonYear) {
    timeParts.push(String(props.anime.seasonYear))
  }

  if (timeParts.length) parts.push(timeParts.join(' '))
  if (props.anime.format) parts.push(formatAnimeFormat(props.anime.format))
  return parts.join(' • ')
})

const compactSeasonYearText = computed(() => {
  const parts: string[] = []

  if (props.showSeason && props.anime.season) {
    parts.push(formatSeason(props.anime.season))
  }

  if (props.showYear && props.anime.seasonYear) {
    parts.push(String(props.anime.seasonYear))
  }

  return parts.join(' ')
})

const hasOverlayContent = computed(() => {
  return (props.staffByRole && props.staffByRole.length > 0) || !!truncatedDescription.value || !!props.anime.anilistId
})
</script>

<style scoped>
.anime-card-link {
  text-decoration: none;
  color: inherit;
  display: block;
  height: 100%;
}

.anime-card {
  height: 100%;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
  background: var(--color-surface);
  overflow: hidden;
}

.anime-card:hover {
  box-shadow: var(--shadow-card-hover);
}

.anime-card__image {
  position: relative;
  overflow: hidden;
}

.anime-card__image--landscape {
  background: rgba(var(--color-overlay-rgb), 0.8);
}

.anime-card__image--landscape :deep(img) {
  object-fit: contain !important;
}

.anime-card__glasspane {
  display: none;
}

.anime-card__image :deep(.v-img__img) {
  transition: filter 0.3s ease;
}

.anime-card:hover .anime-card__image :deep(.v-img__img) {
  filter: brightness(0.4) blur(2px);
}

.anime-card__overlay-left {
  position: absolute;
  top: 0;
  left: 0;
  z-index: 2;
  transition: opacity 0.3s ease;
}

.anime-card:hover .anime-card__overlay-left {
  opacity: 0;
}

.anime-card__overlay-buttons {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 4px;
  z-index: 3;
}

.anime-card__overlay-buttons :deep(.list-button-wrapper) {
  display: flex;
}

.anime-card__overlay-buttons :deep(.list-button) {
  width: 36px !important;
  height: 36px !important;
}

.anime-card__title {
  font-size: 0.875rem;
  line-height: 1.4;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  padding: 8px;
}

.anime-card__season {
  background-color: rgba(var(--color-overlay-rgb), 0.75) !important;
  backdrop-filter: blur(8px);
  font-weight: 700;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
}

.anime-card__season :deep(.v-chip__content) {
  color: var(--color-success) !important;
}

.anime-card__staff-chip {
  background-color: rgba(var(--color-overlay-rgb), 0.75) !important;
  backdrop-filter: blur(8px);
  font-weight: 700;
  color: var(--color-text) !important;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
}

.anime-card__staff-chip .v-icon {
  color: var(--color-primary-hover) !important;
}

.anime-card__description-overlay {
  position: absolute;
  inset: 0;
  background: rgba(var(--color-overlay-rgb), 0.75);
  backdrop-filter: blur(4px);
  padding: 56px 0 16px 16px;
  display: flex;
  align-items: flex-start;
  opacity: 0;
  transition: opacity 0.3s ease;
  z-index: 2;
  pointer-events: none;
}

.anime-card:hover .anime-card__description-overlay {
  opacity: 1;
  pointer-events: auto;
}

.description-text {
  color: var(--color-text);
  font-size: 0.85rem;
  line-height: 1.6;
  margin: 0;
  font-weight: 400;
  text-align: center;
  overflow-y: auto;
  max-height: 100%;
  width: 100%;
  padding-right: 8px;

}

.description-text :deep(p) {
  margin: 0 0 0.5em 0;
}

.description-text :deep(p:last-child) {
  margin-bottom: 0;
}

.staff-by-role-list {
  color: var(--color-text);
  overflow-y: auto;
  max-height: 100%;
  width: 100%;
  padding-right: 8px;
}

.staff-role-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.favorite-btn {
  background-color: transparent !important;
  width: 36px !important;
  height: 36px !important;
  transition: all 0.2s ease;
}

.favorite-btn:hover {
  background-color: rgba(var(--color-text-rgb), 0.1) !important;
}

.favorite-btn.favorited :deep(.v-icon) {
  color: var(--color-error) !important;
}

.anime-card__metadata {
  padding: 0 8px 4px;
}

.anime-card__role {
  padding: 0 8px 2px;
  font-size: 0.75rem;
  opacity: 0.85;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.anime-card__role-compact {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 60%;
  opacity: 0.85;
}

.anime-card__season-year-compact {
  white-space: nowrap;
  margin-left: auto;
  padding-left: 8px;
  opacity: 0.85;
}

.anime-card__tags {
  padding: 0 8px 8px;
}

.anime-card :deep(.v-chip) {
  color: rgba(var(--color-text-rgb), 0.95) !important;
}

/* Disable hover effects when disableHover prop is true (but keep lift animation) */
.anime-card--no-hover:hover .anime-card__image :deep(.v-img__img) {
  filter: none !important;
}

.anime-card--no-hover:hover .anime-card__description-overlay {
  opacity: 0 !important;
  pointer-events: none !important;
}

.anime-card--no-hover:hover .anime-card__overlay-left {
  opacity: 1 !important;
}

/* On touch devices, disable CSS :hover overlay so only JS class controls it */
@media (hover: none) {
  .anime-card:hover .anime-card__image :deep(.v-img__img) {
    filter: none;
  }

  .anime-card:hover .anime-card__overlay-left {
    opacity: 1;
  }

  .anime-card:hover .anime-card__description-overlay {
    opacity: 0;
    pointer-events: none;
  }

.anime-card:hover {
    box-shadow: none;
  }
}

/* Touch-active rules AFTER media query so they win on equal specificity */
.anime-card.touch-active .anime-card__image :deep(.v-img__img) {
  filter: brightness(0.4) blur(2px);
}

.anime-card.touch-active .anime-card__overlay-left {
  opacity: 0;
}

.anime-card.touch-active .anime-card__description-overlay {
  opacity: 1;
  pointer-events: auto;
}

/* Prevent scroll leaking from overlay content to the page */
.description-text,
.staff-by-role-list {
  overscroll-behavior: contain;
}
</style>
