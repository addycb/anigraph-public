<template>
  <v-card class="mb-4">
    <v-card-title class="d-flex align-center justify-space-between">
      <span>Sakuga Clips</span>
      <div class="d-flex align-center gap-2">
        <v-chip v-if="filteredPosts.length > 0" size="small" variant="tonal" color="primary">
          {{ filteredPosts.length }} clips
        </v-chip>
        <a
          v-if="sakugabooruTag"
          :href="`https://www.sakugabooru.com/post?tags=${encodeURIComponent(sakugabooruTag)}`"
          target="_blank"
          rel="noopener noreferrer"
          class="text-decoration-none"
        >
          <v-chip size="small" variant="outlined" color="primary" class="cursor-pointer">
            <v-icon start size="small">mdi-open-in-new</v-icon>
            Sakugabooru
          </v-chip>
        </a>
      </div>
    </v-card-title>
    <v-card-text>
      <!-- Adult content filter notice -->
      <v-chip
        v-if="!includeAdult && hiddenCount > 0 && !adultClipsDismissed"
        :to="'/settings'"
        size="small"
        color="warning"
        variant="tonal"
        prepend-icon="mdi-eye-off-outline"
        closable
        class="mb-3"
        @click:close.prevent="dismissAdultClipsBanner"
      >
        {{ hiddenCount }} adult clip{{ hiddenCount === 1 ? '' : 's' }} hidden
      </v-chip>

      <!-- No clips state -->
      <div v-if="filteredPosts.length === 0" class="text-center py-8">
        <v-icon size="48" color="grey">mdi-filmstrip-off</v-icon>
        <p class="text-body-1 text-medium-emphasis mt-3">No sakuga clips available.</p>
      </div>

      <!-- Clips grid -->
      <v-row v-else>
        <v-col
          v-for="clip in paginatedClips"
          :key="clip.postId"
          cols="12"
          sm="6"
          md="4"
          lg="3"
        >
          <v-card class="sakuga-clip-card" elevation="2" @click="openLightbox(clip)">
            <!-- Thumbnail: preview image for videos, file itself for images -->
            <img
              :src="getThumbnail(clip)"
              loading="lazy"
              class="sakuga-thumbnail"
            />
            <!-- Play icon overlay for videos -->
            <div v-if="isVideoClip(clip)" class="play-overlay">
              <v-icon size="48" color="white">mdi-play-circle-outline</v-icon>
            </div>
            <a
              :href="`https://www.sakugabooru.com/post/show/${clip.postId}`"
              target="_blank"
              rel="noopener noreferrer"
              class="sakuga-external-link"
              @click.stop
            >
              <v-icon size="16" color="white">mdi-open-in-new</v-icon>
            </a>
          </v-card>
        </v-col>
      </v-row>

      <!-- Pagination -->
      <v-row v-if="clipsTotalPages > 1" class="mt-4">
        <v-col cols="12" class="d-flex justify-center align-center flex-column">
          <v-pagination
            v-model="clipsPage"
            :length="clipsTotalPages"
            :total-visible="7"
          ></v-pagination>
          <p class="text-caption text-medium-emphasis mt-2">
            Page {{ clipsPage }} of {{ clipsTotalPages }}
          </p>
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>

  <!-- Fullscreen Lightbox -->
  <Teleport to="body">
    <transition name="lightbox-fade">
      <div v-if="lightboxClip" class="lightbox-backdrop" @click="closeLightbox">
        <v-btn
          icon
          class="lightbox-close"
          variant="flat"
          size="small"
          color="primary"
          @click.stop="closeLightbox"
        >
          <v-icon size="24" color="white">mdi-close</v-icon>
        </v-btn>

        <a
          :href="`https://www.sakugabooru.com/post/show/${lightboxClip.postId}`"
          target="_blank"
          rel="noopener noreferrer"
          class="lightbox-external-link"
          @click.stop
        >
          <v-icon size="18" color="white" class="mr-1">mdi-open-in-new</v-icon>
          Sakugabooru
        </a>

        <div class="lightbox-content" @click.stop>
          <!-- Video — sakugabooru encodes at 480p height (various aspect ratios) -->
          <video
            v-if="isVideoClip(lightboxClip)"
            ref="lightboxVideoRef"
            :poster="lightboxClip.previewUrl || undefined"
            :src="lightboxClip.fileUrl"
            preload="none"
            controls
            loop
            playsinline
            class="lightbox-media lightbox-video"
          />
          <!-- Image / GIF -->
          <img
            v-else
            :src="lightboxClip.fileUrl"
            class="lightbox-media"
          />
        </div>

        <!-- Nav arrows -->
        <v-btn
          v-if="lightboxIndex > 0"
          icon
          class="lightbox-nav lightbox-nav-prev"
          variant="flat"
          size="small"
          color="primary"
          @click.stop="navigateLightbox(-1)"
        >
          <v-icon size="28" color="white">mdi-chevron-left</v-icon>
        </v-btn>
        <v-btn
          v-if="lightboxIndex < filteredPosts.length - 1"
          icon
          class="lightbox-nav lightbox-nav-next"
          variant="flat"
          size="small"
          color="primary"
          @click.stop="navigateLightbox(1)"
        >
          <v-icon size="28" color="white">mdi-chevron-right</v-icon>
        </v-btn>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useSettings } from '~/composables/useSettings'

interface Clip {
  postId: number | string
  fileUrl?: string
  fileExt?: string | null
  previewUrl?: string
  rating?: string | null
}

const props = defineProps<{
  posts: Clip[]
  sakugabooruTag?: string | null
}>()

const { includeAdult } = useSettings()

// Filter out questionable/explicit clips unless adult content is enabled
const filteredPosts = computed(() => {
  if (includeAdult.value) return props.posts
  return props.posts.filter(clip => clip.rating === 's' || !clip.rating)
})

const hiddenCount = computed(() => props.posts.length - filteredPosts.value.length)

const adultClipsDismissed = ref(false)

onMounted(() => {
  adultClipsDismissed.value = localStorage.getItem('adult_clips_banner_dismissed') === 'true'
})

const dismissAdultClipsBanner = () => {
  adultClipsDismissed.value = true
  localStorage.setItem('adult_clips_banner_dismissed', 'true')
}

const CLIPS_PER_PAGE = 12
const clipsPage = ref(1)

const clipsTotalPages = computed(() => Math.max(1, Math.ceil(filteredPosts.value.length / CLIPS_PER_PAGE)))

const paginatedClips = computed(() => {
  const start = (clipsPage.value - 1) * CLIPS_PER_PAGE
  return filteredPosts.value.slice(start, start + CLIPS_PER_PAGE)
})

// Reset page when posts change
watch(() => filteredPosts.value, () => {
  clipsPage.value = 1
})

function isVideoClip(clip: Clip) {
  const ext = clip.fileExt || clip.fileUrl?.split('.').pop()?.toLowerCase()
  return ext === 'mp4' || ext === 'webm'
}

function getThumbnail(clip: Clip) {
  if (isVideoClip(clip)) {
    return clip.previewUrl || ''
  }
  return clip.fileUrl || ''
}

// Lightbox
const lightboxClip = ref<Clip | null>(null)
const lightboxIndex = ref(-1)
const lightboxVideoRef = ref<HTMLVideoElement | null>(null)

function openLightbox(clip: Clip) {
  lightboxClip.value = clip
  lightboxIndex.value = filteredPosts.value.findIndex(p => p.postId === clip.postId)
}

function closeLightbox() {
  // Pause video before closing
  if (lightboxVideoRef.value) {
    lightboxVideoRef.value.pause()
  }
  lightboxClip.value = null
  lightboxIndex.value = -1
}

function navigateLightbox(direction: number) {
  // Pause current video
  if (lightboxVideoRef.value) {
    lightboxVideoRef.value.pause()
  }
  const newIndex = lightboxIndex.value + direction
  if (newIndex >= 0 && newIndex < filteredPosts.value.length) {
    lightboxIndex.value = newIndex
    lightboxClip.value = filteredPosts.value[newIndex]
  }
}

// Keyboard navigation
function onKeydown(e: KeyboardEvent) {
  if (!lightboxClip.value) return
  if (e.key === 'Escape') closeLightbox()
  else if (e.key === 'ArrowLeft') navigateLightbox(-1)
  else if (e.key === 'ArrowRight') navigateLightbox(1)
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.sakuga-clip-card {
  overflow: hidden;
  border: 1px solid var(--color-primary-medium);
  transition: all 0.3s ease;
  position: relative;
  cursor: pointer;
}

.sakuga-clip-card:hover {
  border-color: var(--color-primary-border-focus);
  box-shadow: var(--shadow-glow);
  transform: translateY(-2px);
}

.sakuga-thumbnail {
  width: 100%;
  display: block;
  aspect-ratio: 16 / 9;
  object-fit: cover;
  background: var(--color-surface);
}

.play-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.3);
  pointer-events: none;
  transition: background 0.2s ease;
}

.sakuga-clip-card:hover .play-overlay {
  background: rgba(0, 0, 0, 0.15);
}

.sakuga-external-link {
  position: absolute;
  top: 4px;
  right: 4px;
  background: rgba(0, 0, 0, 0.6);
  border-radius: 4px;
  padding: 2px 4px;
  opacity: 0;
  transition: opacity 0.2s ease;
  text-decoration: none;
  line-height: 1;
  z-index: 1;
}

.sakuga-clip-card:hover .sakuga-external-link {
  opacity: 1;
}

.cursor-pointer {
  cursor: pointer;
}

/* Lightbox */
.lightbox-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.92);
  display: flex;
  align-items: center;
  justify-content: center;
}

.lightbox-close {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 2;
}

.lightbox-external-link {
  position: absolute;
  top: 20px;
  left: 16px;
  z-index: 2;
  color: white;
  text-decoration: none;
  display: flex;
  align-items: center;
  font-size: 14px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.lightbox-external-link:hover {
  opacity: 1;
}

.lightbox-content {
  max-width: 90vw;
  max-height: 85vh;
  display: flex;
  align-items: center;
  justify-content: center;
}

.lightbox-media {
  max-width: 90vw;
  max-height: 85vh;
  object-fit: contain;
  border-radius: 4px;
}

.lightbox-video {
  /* Lock height to 480p (sakugabooru standard) so poster->playback doesn't cause a size jump.
     Width adapts naturally to the video's actual aspect ratio (16:9 or 4:3). */
  height: 480px;
  background: #000;
}

.lightbox-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 2;
}

.lightbox-nav-prev {
  left: 16px;
}

.lightbox-nav-next {
  right: 16px;
}

/* Transitions */
.lightbox-fade-enter-active,
.lightbox-fade-leave-active {
  transition: opacity 0.2s ease;
}

.lightbox-fade-enter-from,
.lightbox-fade-leave-to {
  opacity: 0;
}
</style>
