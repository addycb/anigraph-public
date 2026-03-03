<template>
  <v-card class="anime-preview-card" elevation="8">
    <v-img
      :src="anime.coverImage || anime.coverImage_large || anime.coverImage_extraLarge || anime.image || '/placeholder-anime.jpg'"
      class="anime-preview-card__image"
    >
      <!-- Top Left: New badge -->
      <div v-if="isThisSeason" class="preview-badge-overlay-left">
        <v-chip
          color="success"
          size="small"
          class="ma-2 preview-season-badge"
        >
          <v-icon start>mdi-calendar-star</v-icon>
          New
        </v-chip>
      </div>

      <!-- Top Right: Rating chip -->
      <div v-if="anime.averageScore" class="preview-badge-overlay-right">
        <v-chip
          color="primary"
          size="small"
          class="ma-2 preview-rating"
        >
          <v-icon start>mdi-star</v-icon>
          {{ Math.round(anime.averageScore) }}
        </v-chip>
      </div>
    </v-img>

    <v-card-title class="anime-preview-card__title">
      {{ displayTitle }}
    </v-card-title>

    <v-card-subtitle v-if="anime.format || seasonDisplay" class="anime-preview-card__metadata">
      <div class="d-flex justify-space-between align-center">
        <span v-if="anime.format" class="format-badge">{{ anime.format }}</span>
        <span v-if="seasonDisplay" class="year-badge">{{ seasonDisplay }}</span>
      </div>
    </v-card-subtitle>
  </v-card>
</template>

<script setup lang="ts">
interface AnimePreviewProps {
  anime: {
    // ID fields
    id?: string | number
    anilistId?: number | string
    // Title fields
    label?: string
    title?: string
    titleEnglish?: string
    titleRomaji?: string
    // Image fields
    image?: string
    coverImage?: string
    coverImage_large?: string
    coverImage_extraLarge?: string
    // Metadata
    averageScore?: number | null
    season?: string | null
    seasonYear?: number | null
    format?: string | null
  }
}

const props = defineProps<AnimePreviewProps>()

const { isCurrentOrNextSeason } = useSeason()

const displayTitle = computed(() => {
  return props.anime.titleEnglish ||
         props.anime.title ||
         props.anime.label ||
         props.anime.titleRomaji ||
         'Unknown Anime'
})

const isThisSeason = computed(() => {
  return isCurrentOrNextSeason(props.anime.season, props.anime.seasonYear)
})

const seasonDisplay = computed(() => {
  if (props.anime.season && props.anime.seasonYear) {
    return `${formatSeasonName(props.anime.season)} ${props.anime.seasonYear}`
  } else if (props.anime.seasonYear) {
    return String(props.anime.seasonYear)
  }
  return null
})

// Format season name (WINTER -> Winter)
const formatSeasonName = (season: string | null | undefined) => {
  if (!season) return ''
  return season.charAt(0) + season.slice(1).toLowerCase()
}
</script>

<style scoped>
.anime-preview-card {
  width: 180px;
  max-width: 250px;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-lg);
  overflow: hidden;
  box-shadow: var(--shadow-glow);
}

.anime-preview-card__image {
  position: relative;
}

.anime-preview-card__title {
  font-size: 0.95rem;
  line-height: 1.4;
  font-weight: 500;
  word-wrap: break-word;
  white-space: normal;
  padding-bottom: 8px;
}

.anime-preview-card__metadata {
  padding-top: 0;
  padding-bottom: 12px;
}

.preview-badge-overlay-left {
  position: absolute;
  top: 0;
  left: 0;
  z-index: 2;
}

.preview-badge-overlay-right {
  position: absolute;
  top: 0;
  right: 0;
  z-index: 2;
}

.preview-rating {
  background-color: rgba(var(--color-bg-rgb), 0.8) !important;
  backdrop-filter: blur(8px);
  font-weight: 700;
  box-shadow: var(--shadow-sm);
}

.preview-rating .v-icon {
  color: var(--color-score) !important;
  -webkit-text-stroke: 0.5px rgba(0, 0, 0, 0.3);
  paint-order: stroke fill;
}

.preview-season-badge {
  background-color: rgba(var(--color-bg-rgb), 0.8) !important;
  backdrop-filter: blur(8px);
  font-weight: 700;
  box-shadow: var(--shadow-sm);
}

.format-badge {
  font-size: 0.875rem;
  font-weight: 600;
  text-transform: uppercase;
  opacity: 0.7;
  letter-spacing: 0.5px;
}

.year-badge {
  font-size: 0.875rem;
  font-weight: 700;
  color: rgb(var(--v-theme-primary));
  opacity: 1;
  flex-shrink: 0;
}
</style>
