<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

// Redirect to Samurai Champloo with tutorial mode enabled
onMounted(() => {
  router.replace('/anime/205?tutorial=true')
})
</script>

<template>
  <div></div>
</template>
