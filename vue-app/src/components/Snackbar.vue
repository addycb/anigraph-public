<template>
  <v-snackbar
    v-model="showSnackbar"
    :color="snackbarColor"
    :timeout="1000"
    location="top"
    class="centered-snackbar"
  >
    {{ snackbarMessage }}
  </v-snackbar>
</template>

<script setup lang="ts">
import { useSnackbar } from '@/composables/useSnackbar'

const { showSnackbar, snackbarMessage, snackbarColor } = useSnackbar()
</script>

<style scoped>
.centered-snackbar :deep(.v-snackbar__content) {
  text-align: center;
}
</style>
