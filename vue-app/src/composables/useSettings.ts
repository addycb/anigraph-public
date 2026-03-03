import { ref, readonly } from 'vue'

const includeAdult = ref<boolean>(localStorage.getItem('anigraph_includeAdult') === 'true')

export const useSettings = () => {
  const setIncludeAdult = (value: boolean) => {
    includeAdult.value = value
    localStorage.setItem('anigraph_includeAdult', value.toString())
  }

  return {
    includeAdult: readonly(includeAdult),
    setIncludeAdult
  }
}
