import { ref } from 'vue'

const showSnackbar = ref(false)
const snackbarMessage = ref('')
const snackbarColor = ref('success')

export const useSnackbar = () => {
  const show = (message: string, color: string = 'success') => {
    snackbarMessage.value = message
    snackbarColor.value = color
    showSnackbar.value = true
  }

  return {
    showSnackbar,
    snackbarMessage,
    snackbarColor,
    show,
    showSuccess: (msg: string) => show(msg, 'success'),
    showError: (msg: string) => show(msg, 'error'),
  }
}
