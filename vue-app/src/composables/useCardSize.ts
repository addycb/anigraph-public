import { ref, watch, computed } from 'vue'

const CARD_COL_SIZES: Record<string, string> = {
  small: '2',
  medium: '3',
  large: '4',
  xlarge: '6'
}

const CARD_SIZE_KEY = 'anigraph_cardSize'
const YEAR_MARKERS_KEY = 'anigraph_yearMarkers'

export function useCardSize(defaultSize: 'small' | 'medium' | 'large' | 'xlarge' = 'small') {
  const storedSize = localStorage.getItem(CARD_SIZE_KEY)
  const initialSize = (storedSize && CARD_COL_SIZES[storedSize]) ? storedSize as 'small' | 'medium' | 'large' | 'xlarge' : defaultSize

  const cardSize = ref<'small' | 'medium' | 'large' | 'xlarge'>(initialSize)

  const storedMarkers = localStorage.getItem(YEAR_MARKERS_KEY)
  const showYearMarkers = ref<boolean>(storedMarkers === null ? true : storedMarkers === 'true')

  watch(cardSize, (val) => {
    localStorage.setItem(CARD_SIZE_KEY, val)
  })

  watch(showYearMarkers, (val) => {
    localStorage.setItem(YEAR_MARKERS_KEY, val.toString())
  })

  const cardColSize = computed(() => CARD_COL_SIZES[cardSize.value])
  const cardsPerRow = computed(() => 12 / Number(cardColSize.value))

  return { cardSize, cardColSize, cardsPerRow, showYearMarkers }
}
