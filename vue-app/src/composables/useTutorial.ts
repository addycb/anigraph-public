import { ref } from 'vue'

const tutorialActive = ref(false)
const currentStep = ref(0)

// Use localStorage instead of useCookie
const TUTORIAL_COMPLETED_KEY = 'anigraph_tutorial_completed'
const tutorialCompleted = ref(localStorage.getItem(TUTORIAL_COMPLETED_KEY) === 'true')

export const useTutorial = () => {
  const steps = [
    {
      title: 'Welcome to AniGraph',
      description: 'AniGraph shows you connections between anime, staff, and studios. Let\'s walk through the key features using Samurai Champloo.',
      target: null,
      position: 'center'
    },
    {
      title: 'Basic Info',
      description: 'Cover art, rating, format, and season. All the essential details at a glance.',
      target: '.sticky-card',
      position: 'right',
      highlight: '.sticky-card'
    },
    {
      title: 'Quick Add Search',
      description: 'Click the magnifying glass next to studios, genres, or tags to instantly build a search. Fast way to find anime with similar attributes.',
      target: '.sticky-card .info-section',
      position: 'right',
      highlight: '.sticky-card .info-section'
    },
    {
      title: 'Related Works',
      description: 'See sequels, prequels, and adaptations. Click the franchise link to view the complete series.',
      target: '.related-works-section',
      position: 'left',
      highlight: '.related-works-section'
    },
    {
      title: 'Network Graph',
      description: 'The core feature. Visualize how anime, staff, and studios connect. Try clicking any node to navigate, or drag nodes to rearrange.',
      target: '.graph-panel',
      position: 'top',
      highlight: '.graph-panel',
      scroll: true,
      interactive: true
    },
    {
      title: 'Graph Controls',
      description: 'Try the controls. Toggle between graph and staff list views. Filter by format (TV, Movie, OVA), select staff groups, and filter by genre/tag to focus.',
      target: '.graph-panel',
      position: 'top',
      highlight: '.graph-panel',
      scroll: true,
      interactive: true
    },
    {
      title: 'Similar Anime',
      description: 'Browse recommendations based on shared staff from the graph, all shared staff, or similarity metrics. Click to explore.',
      target: '.recommendations-section',
      position: 'top',
      highlight: '.recommendations-section',
      scroll: true,
      interactive: true
    },
    {
      title: 'Performance Context',
      description: 'See how this anime ranks among the studio\'s work. Filter by genre to focus the timeline charts and explore trends over time.',
      target: '.studio-stats-section',
      position: 'top',
      highlight: '.studio-stats-section',
      scroll: true,
      action: 'expand-stats',
      interactive: true
    },
    {
      title: 'Search From This Anime',
      description: 'Click + icons to instantly search using this anime\'s studios, genres, and tags. Mix and match attributes to find similar works. Live counts show results.',
      target: '.advanced-search-section',
      position: 'top',
      highlight: '.advanced-search-section',
      scroll: true,
      action: 'expand-search',
      interactive: true
    },
    {
      title: 'That\'s It',
      description: 'Explore anime through their connections. Click around and discover.',
      target: null,
      position: 'center'
    }
  ]

  const startTutorial = () => {
    tutorialActive.value = true
    currentStep.value = 0
  }

  const nextStep = () => {
    if (currentStep.value < steps.length - 1) {
      currentStep.value++
      return true
    }
    return false
  }

  const prevStep = () => {
    if (currentStep.value > 0) {
      currentStep.value--
      return true
    }
    return false
  }

  const endTutorial = () => {
    tutorialActive.value = false
    currentStep.value = 0
    tutorialCompleted.value = true
    localStorage.setItem(TUTORIAL_COMPLETED_KEY, 'true')
  }

  const getCurrentStep = () => {
    return steps[currentStep.value]
  }

  return {
    tutorialActive,
    currentStep,
    steps,
    startTutorial,
    nextStep,
    prevStep,
    tutorialCompleted,
    endTutorial,
    getCurrentStep
  }
}
