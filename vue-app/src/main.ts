import { createApp, nextTick } from 'vue'
import { RecycleScroller, DynamicScroller, DynamicScrollerItem } from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'

import App from './App.vue'
import router from './router'
import { vuetify } from './plugins/vuetify'
import './assets/tokens.css'

const app = createApp(App)

app.use(router)
app.use(vuetify)

// Virtual scroller components
app.component('RecycleScroller', RecycleScroller)
app.component('DynamicScroller', DynamicScroller)
app.component('DynamicScrollerItem', DynamicScrollerItem)

// Chunk error handler — reload on stale dynamic imports
router.onError((err) => {
  if (err.message.includes('Failed to fetch dynamically imported module') ||
      err.message.includes('Importing a module script failed')) {
    window.location.reload()
  }
})

// Track client-side route changes for Umami analytics
let isFirstLoad = true
router.afterEach((to) => {
  if (isFirstLoad) {
    isFirstLoad = false
    return
  }
  nextTick(() => {
    if ((window as any).umami) {
      (window as any).umami.track((props: any) => ({ ...props, url: to.fullPath }))
    }
  })
})

// Global error handler — log full details for unhandled component errors
app.config.errorHandler = (err, instance, info) => {
  console.error('[Vue Error]', info, err)
  console.error('[Vue Error] Component:', instance?.$options?.name || instance?.$options?.__name || instance)
  console.error('[Vue Error] Stack:', (err as Error)?.stack)
}

app.mount('#app')
