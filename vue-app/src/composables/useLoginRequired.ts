import { ref } from 'vue'

const showLoginAlert = ref(false)
let dismissTimeout: ReturnType<typeof setTimeout> | null = null

export const useLoginRequired = () => {
  const requireLogin = () => {
    showLoginAlert.value = true
    if (dismissTimeout) clearTimeout(dismissTimeout)
    dismissTimeout = setTimeout(() => {
      showLoginAlert.value = false
    }, 1400)
  }

  return {
    showLoginAlert,
    requireLogin
  }
}
