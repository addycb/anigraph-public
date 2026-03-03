import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('@/pages/index.vue') },
    { path: '/home', component: () => import('@/pages/home.vue') },
    { path: '/overview', component: () => import('@/pages/overview.vue') },
    { path: '/top-rated', component: () => import('@/pages/top-rated.vue') },
    { path: '/new-productions', component: () => import('@/pages/new-productions.vue') },
    { path: '/just-added', component: () => import('@/pages/just-added.vue') },
    { path: '/anime/:id', component: () => import('@/pages/anime/[id].vue') },
    { path: '/studio/:id', component: () => import('@/pages/studio/[id].vue') },
    { path: '/staff/:id', component: () => import('@/pages/staff/[id].vue') },
    { path: '/franchise/:id+', component: () => import('@/pages/franchise/[...id].vue') },
    { path: '/favorites', component: () => import('@/pages/favorites.vue') },
    { path: '/recommendations', component: () => import('@/pages/recommendations.vue') },
    { path: '/taste-profile', component: () => import('@/pages/taste-profile.vue') },
    { path: '/lists/public', component: () => import('@/pages/lists/public.vue') },
    { path: '/lists/:token', component: () => import('@/pages/lists/[token].vue') },
    { path: '/search', component: () => import('@/pages/search/index.vue') },
    { path: '/search/advanced', component: () => import('@/pages/search/advanced.vue') },
    { path: '/advanced-search', component: () => import('@/pages/advanced-search.vue') },
    { path: '/login', component: () => import('@/pages/login.vue') },
    { path: '/settings', component: () => import('@/pages/settings.vue') },
    { path: '/tutorial', component: () => import('@/pages/tutorial.vue') },
    { path: '/privacy-policy', component: () => import('@/pages/privacy-policy.vue') },
    { path: '/terms-of-service', component: () => import('@/pages/terms-of-service.vue') },
    { path: '/status', component: () => import('@/pages/status.vue') },
  ],
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.path === from.path) return false
    return { top: 0 }
  },
})

export default router
