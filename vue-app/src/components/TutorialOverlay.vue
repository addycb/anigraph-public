<template>
  <teleport to="body">
    <transition name="tutorial-fade">
      <div v-if="tutorialActive" class="tutorial-overlay-container">
        <!-- Backdrop with spotlight effect -->
        <div
          class="tutorial-backdrop"
          :class="{ 'backdrop-interactive': step?.interactive }"
          @click="handleBackdropClick"
        >
          <!-- SVG mask for spotlight effect -->
          <svg class="tutorial-spotlight-svg">
            <defs>
              <mask id="spotlight-mask">
                <rect x="0" y="0" width="100%" height="100%" fill="white" />
                <rect
                  v-if="spotlightRect"
                  :x="spotlightRect.x"
                  :y="spotlightRect.y"
                  :width="spotlightRect.width"
                  :height="spotlightRect.height"
                  :rx="12"
                  fill="black"
                />
              </mask>
            </defs>
            <rect
              x="0"
              y="0"
              width="100%"
              height="100%"
              fill="rgba(var(--color-overlay-rgb), 0.75)"
              mask="url(#spotlight-mask)"
            />
          </svg>
        </div>

        <!-- Tutorial card -->
        <transition name="tutorial-slide">
          <v-card
            v-if="step"
            class="tutorial-card"
            :style="cardStyle"
            elevation="24"
          >
            <!-- Progress indicator -->
            <div class="tutorial-progress">
              <div
                v-for="(s, idx) in steps.length"
                :key="idx"
                class="tutorial-progress-dot"
                :class="{ active: idx === currentStep, completed: idx < currentStep }"
              ></div>
            </div>

            <v-card-title
              class="tutorial-card-title"
              @mousedown="handleDragStart"
            >
              <v-icon v-if="currentStep === 0" color="primary" size="large" class="mr-2">
                mdi-chart-timeline-variant
              </v-icon>
              {{ step.title }}
            </v-card-title>

            <v-card-text class="tutorial-card-text">
              {{ step.description }}
            </v-card-text>

            <v-card-actions class="tutorial-card-actions">
              <div class="tutorial-step-counter">
                {{ currentStep + 1 }} / {{ steps.length }}
              </div>
              <v-spacer></v-spacer>
              <v-btn
                v-if="currentStep > 0"
                variant="text"
                @click="handlePrev"
              >
                Back
              </v-btn>
              <v-btn
                v-if="currentStep < steps.length - 1"
                color="primary"
                variant="flat"
                @click="handleNext"
              >
                Next
              </v-btn>
              <v-btn
                v-else
                color="primary"
                variant="flat"
                @click="handleFinish"
              >
                Start Exploring
              </v-btn>
              <v-btn
                v-if="currentStep < steps.length - 1"
                variant="text"
                @click="handleSkip"
                class="ml-2"
              >
                Exit Tutorial
              </v-btn>
            </v-card-actions>
          </v-card>
        </transition>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTutorial } from '@/composables/useTutorial'

const router = useRouter()
const { tutorialActive, currentStep, steps, nextStep, prevStep, endTutorial, getCurrentStep } = useTutorial()

const step = computed(() => getCurrentStep())
const spotlightRect = ref<{ x: number; y: number; width: number; height: number } | null>(null)
const cardStyle = ref({})
const previousHighlight = ref<string | null>(null)
const isTransitioning = ref(false)

// Drag state
const isDragging = ref(false)
const dragStart = ref({ x: 0, y: 0 })
const dragOffset = ref({ x: 0, y: 0 })
const cardPosition = ref<{ top: number; left: number } | null>(null)
const rafId = ref<number | null>(null)

// Emit events for parent to handle
const emit = defineEmits(['expand-stats', 'expand-search'])

// Update spotlight and card position when step changes
watch(currentStep, async () => {
  const currentStepData = step.value
  const currentHighlight = currentStepData.highlight || null

  // Reset dragged position so card repositions automatically for each step
  cardPosition.value = null

  // Disable position updates during transition to prevent jerky behavior in Chromium
  isTransitioning.value = true

  // Only clear spotlight if the highlight area is changing
  // This prevents flicker when consecutive steps highlight the same element
  if (currentHighlight !== previousHighlight.value) {
    spotlightRect.value = null
  }

  previousHighlight.value = currentHighlight

  await nextTick()

  // Handle step actions first (expand panels)
  if (currentStepData.action === 'expand-stats') {
    emit('expand-stats')
  } else if (currentStepData.action === 'expand-search') {
    emit('expand-search')
  }

  // For step 2 (Basic Info) and step 3 (Quick Add Search), scroll sticky-card to top
  if (currentStep.value === 1 || currentStep.value === 2) {
    const stickyCard = document.querySelector('.sticky-card')
    if (stickyCard) {
      stickyCard.scrollTo({ top: 0, behavior: 'auto' })
      // Force layout recalculation
      void stickyCard.scrollHeight
    }
  }

  // Wait for panel expansion animations
  await nextTick()
  await new Promise(resolve => setTimeout(resolve, 200))

  // ALWAYS scroll to highlighted element if there is one
  const targetElement = currentStepData.highlight
    ? document.querySelector(currentStepData.highlight)
    : currentStepData.target ? document.querySelector(currentStepData.target) : null

  if (targetElement) {
    // Scroll in scrollable parent containers first (instant scroll)
    scrollToElementInContainer(targetElement)

    // Wait for Chromium to process container scroll and update layout
    // Double RAF ensures scroll position and layout are fully settled
    await new Promise(resolve => requestAnimationFrame(() => {
      requestAnimationFrame(resolve)
    }))

    // Then scroll main window if needed (smooth scroll)
    targetElement.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' })

    // Wait for window scroll to FULLY complete (smooth scroll takes ~500-600ms)
    await new Promise(resolve => setTimeout(resolve, 800))

    // Wait for Chromium's layout/paint cycle to complete
    // Double RAF ensures layout is fully settled before measuring positions
    await new Promise(resolve => requestAnimationFrame(() => {
      requestAnimationFrame(resolve)
    }))

    // NOW update spotlight - element is in final position
    updateSpotlight()
    updateCardPosition()

    // Final refinement after layout is stable
    await new Promise(resolve => requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        updateSpotlight()
        updateCardPosition()
        resolve()
      })
    }))

    // Re-enable position updates now that transition is complete
    isTransitioning.value = false
  } else {
    // No target, update immediately (for centered steps)
    // Wait for layout to be stable before updating
    await new Promise(resolve => requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        updateSpotlight()
        updateCardPosition()
        resolve()
      })
    }))

    // Re-enable position updates now that transition is complete
    isTransitioning.value = false
  }
}, { immediate: true })

// Helper function to scroll element into view within its scrollable container
const scrollToElementInContainer = (element: Element) => {
  // Find the scrollable parent container
  let parent = element.parentElement
  while (parent) {
    const overflow = window.getComputedStyle(parent).overflowY
    if (overflow === 'auto' || overflow === 'scroll') {
      // Found a scrollable container
      const parentRect = parent.getBoundingClientRect()
      const elementRect = element.getBoundingClientRect()

      // Special handling for sticky-card: scroll to top to show all content
      if (parent.classList.contains('sticky-card')) {
        parent.scrollTo({
          top: 0,
          behavior: 'auto' // Instant scroll to prevent timing issues
        })
        // Force layout recalculation for Chromium
        void parent.offsetHeight
      } else {
        // Check if element is fully visible within the scrollable container
        const isAboveView = elementRect.top < parentRect.top
        const isBelowView = elementRect.bottom > parentRect.bottom

        if (isAboveView || isBelowView) {
          // Scroll the container to center the element
          const scrollTop = element.offsetTop - parent.offsetTop - (parent.clientHeight / 2) + (element.clientHeight / 2)
          parent.scrollTo({
            top: scrollTop,
            behavior: 'auto' // Instant scroll to prevent timing issues
          })
          // Force layout recalculation for Chromium
          void parent.offsetHeight
        }
      }
      break
    }
    parent = parent.parentElement
  }
}

const updateSpotlight = () => {
  const currentStepData = step.value
  if (!currentStepData.highlight) {
    spotlightRect.value = null
    return
  }

  const element = document.querySelector(currentStepData.highlight)
  if (!element) {
    spotlightRect.value = null
    return
  }

  const rect = element.getBoundingClientRect()
  const padding = 12

  spotlightRect.value = {
    x: rect.left - padding,
    y: rect.top - padding,
    width: rect.width + padding * 2,
    height: rect.height + padding * 2
  }
}

const updateCardPosition = () => {
  const currentStepData = step.value

  if (!currentStepData.target || currentStepData.position === 'center') {
    // Center the card
    cardStyle.value = {
      position: 'fixed',
      top: '50%',
      left: '50%',
      transform: 'translate(-50%, -50%)',
      maxWidth: '500px',
      width: '90%',
      maxHeight: 'calc(100vh - 40px)',
      overflowY: 'auto'
    }
    return
  }

  const element = document.querySelector(currentStepData.target)
  if (!element) {
    // Fallback to center
    cardStyle.value = {
      position: 'fixed',
      top: '50%',
      left: '50%',
      transform: 'translate(-50%, -50%)',
      maxWidth: '500px',
      width: '90%',
      maxHeight: 'calc(100vh - 40px)',
      overflowY: 'auto'
    }
    return
  }

  const rect = element.getBoundingClientRect()
  const cardWidth = 400
  const estimatedCardHeight = 250 // Estimate card height with padding
  const gap = 20
  const viewportPadding = 20

  let top = 0
  let left = 0
  let preferredPosition = currentStepData.position

  // Calculate initial position based on preference
  switch (preferredPosition) {
    case 'right':
      top = rect.top + rect.height / 2 - estimatedCardHeight / 2
      left = rect.right + gap
      // Check if there's enough space on the right
      if (left + cardWidth + viewportPadding > window.innerWidth) {
        // Try left instead
        preferredPosition = 'left'
        left = rect.left - cardWidth - gap
      }
      break
    case 'left':
      top = rect.top + rect.height / 2 - estimatedCardHeight / 2
      left = rect.left - cardWidth - gap
      // Check if there's enough space on the left
      if (left < viewportPadding) {
        // Try right instead
        preferredPosition = 'right'
        left = rect.right + gap
      }
      break
    case 'top':
      top = rect.top - estimatedCardHeight - gap
      left = rect.left + rect.width / 2 - cardWidth / 2
      // Check if there's enough space on top
      if (top < viewportPadding) {
        // Try bottom instead
        preferredPosition = 'bottom'
        top = rect.bottom + gap
      }
      break
    case 'bottom':
      top = rect.bottom + gap
      left = rect.left + rect.width / 2 - cardWidth / 2
      // Check if there's enough space on bottom
      if (top + estimatedCardHeight + viewportPadding > window.innerHeight) {
        // Try top instead
        preferredPosition = 'top'
        top = rect.top - estimatedCardHeight - gap
      }
      break
  }

  // Final bounds check - ensure card stays within viewport
  const maxTop = window.innerHeight - estimatedCardHeight - viewportPadding
  const maxLeft = window.innerWidth - cardWidth - viewportPadding

  top = Math.max(viewportPadding, Math.min(top, maxTop))
  left = Math.max(viewportPadding, Math.min(left, maxLeft))

  cardStyle.value = {
    position: 'fixed',
    top: `${top}px`,
    left: `${left}px`,
    maxWidth: `${cardWidth}px`,
    width: 'auto',
    maxHeight: `calc(100vh - 40px)`,
    overflowY: 'auto'
  }
}

const handleNext = async () => {
  nextStep()
}

const handlePrev = () => {
  prevStep()
}

const handleFinish = () => {
  endTutorial()
  router.push('/anime/205')
}

const handleSkip = () => {
  endTutorial()
}

const handleBackdropClick = () => {
  // Don't close on backdrop click - require explicit action
}

// Drag handlers
const handleDragStart = (e: MouseEvent) => {
  isDragging.value = true
  dragStart.value = { x: e.clientX, y: e.clientY }

  // Get current card position
  const card = (e.target as HTMLElement).closest('.tutorial-card') as HTMLElement
  if (card) {
    const rect = card.getBoundingClientRect()
    cardPosition.value = { top: rect.top, left: rect.left }
    // Reset drag offset
    dragOffset.value = { x: 0, y: 0 }
  }

  e.preventDefault() // Prevent text selection while dragging
}

const handleDragMove = (e: MouseEvent) => {
  if (!isDragging.value || !cardPosition.value) return

  const deltaX = e.clientX - dragStart.value.x
  const deltaY = e.clientY - dragStart.value.y

  // Update offset for smooth RAF-based rendering
  dragOffset.value = { x: deltaX, y: deltaY }

  // Use RAF for smooth updates
  if (rafId.value) return // Already scheduled

  rafId.value = requestAnimationFrame(() => {
    rafId.value = null

    if (!cardPosition.value) return

    const newLeft = cardPosition.value.left + dragOffset.value.x
    const newTop = cardPosition.value.top + dragOffset.value.y

    // Update card style with new position using transform for better performance
    const currentStyle = cardStyle.value as any
    cardStyle.value = {
      ...currentStyle,
      position: 'fixed',
      left: `${newLeft}px`,
      top: `${newTop}px`,
      transform: 'none',
      transition: 'none' // Disable transitions during drag
    }
  })
}

const handleDragEnd = () => {
  if (isDragging.value) {
    isDragging.value = false
    dragOffset.value = { x: 0, y: 0 }
    if (rafId.value) {
      cancelAnimationFrame(rafId.value)
      rafId.value = null
    }
  }
}

// Event handler wrappers that respect the transitioning flag
const handleResize = () => {
  if (!isTransitioning.value) {
    updateSpotlight()
    updateCardPosition()
  }
}

const handleScroll = () => {
  if (!isTransitioning.value) {
    updateSpotlight()
    updateCardPosition()
  }
}

// Update on window resize and scroll
onMounted(() => {
  window.addEventListener('resize', handleResize)
  window.addEventListener('scroll', handleScroll, true) // Use capture phase to catch all scroll events
  window.addEventListener('mousemove', handleDragMove)
  window.addEventListener('mouseup', handleDragEnd)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('scroll', handleScroll, true)
  window.removeEventListener('mousemove', handleDragMove)
  window.removeEventListener('mouseup', handleDragEnd)
})
</script>

<style scoped>
.tutorial-overlay-container {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 9999;
  pointer-events: none;
}

.tutorial-backdrop {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: all;
  transition: opacity 0.3s ease;
}

.tutorial-backdrop.backdrop-interactive {
  pointer-events: none;
}

.tutorial-spotlight-svg {
  width: 100%;
  height: 100%;
  transition: opacity 0.2s ease;
}

.tutorial-card {
  pointer-events: all;
  z-index: 10000;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.tutorial-progress {
  display: flex;
  gap: 8px;
  padding: 16px 24px 8px;
  justify-content: center;
}

.tutorial-progress-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary-medium);
  transition: all 0.3s ease;
}

.tutorial-progress-dot.active {
  background: var(--color-primary);
  transform: scale(1.3);
}

.tutorial-progress-dot.completed {
  background: var(--color-primary);
}

.tutorial-card-title {
  font-size: 1.5rem;
  font-weight: 600;
  padding: 8px 24px 12px;
  display: flex;
  align-items: center;
  cursor: grab;
  user-select: none;
}

.tutorial-card-title:active {
  cursor: grabbing;
}

.tutorial-card-text {
  font-size: 1rem;
  line-height: 1.6;
  padding: 0 24px 16px;
  color: rgba(var(--color-text-rgb), 0.87);
}

.tutorial-card-actions {
  padding: 12px 24px 16px;
}

.tutorial-step-counter {
  font-size: 0.875rem;
  color: rgba(var(--color-text-rgb), 0.6);
  font-weight: 500;
}

/* Animations */
.tutorial-fade-enter-active,
.tutorial-fade-leave-active {
  transition: opacity 0.3s ease;
}

.tutorial-fade-enter-from,
.tutorial-fade-leave-to {
  opacity: 0;
}

.tutorial-slide-enter-active,
.tutorial-slide-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.tutorial-slide-enter-from {
  opacity: 0;
  transform: translateY(20px);
}

.tutorial-slide-leave-to {
  opacity: 0;
  transform: translateY(-20px);
}
</style>
