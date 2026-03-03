import { computed, type ComputedRef, type Ref } from 'vue'
import { useDisplay } from 'vuetify'

export function useVirtualGrid(
  items: ComputedRef<any[]> | Ref<any[]>,
  cardColSize: ComputedRef<string> | Ref<string>
) {
  const { smAndUp, mdAndUp, lgAndUp } = useDisplay()

  const columnsCount = computed(() => {
    if (lgAndUp.value) return 12 / Number(cardColSize.value)
    if (mdAndUp.value) return 3 // md="4" -> 12/4
    if (smAndUp.value) return 2 // sm="6" -> 12/6
    return 1 // cols="12"
  })

  const rows = computed(() => {
    const cols = columnsCount.value
    const result: { id: string; items: any[] }[] = []
    const arr = items.value
    for (let i = 0; i < arr.length; i += cols) {
      result.push({
        id: `row-${i}`,
        items: arr.slice(i, i + cols)
      })
    }
    return result
  })

  return { rows, columnsCount }
}
