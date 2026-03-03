import { ref, readonly } from 'vue'
import { useTheme as useVuetifyTheme } from 'vuetify'

export interface ThemeDefinition {
  id: string
  name: string
  primary: string
  secondary: string
  accent: string
  background: string
  surface: string
}

export const themes: ThemeDefinition[] = [
  { id: 'midnight', name: 'Indigo', primary: '#667eea', secondary: '#ec4899', accent: '#764ba2', background: '#0F172A', surface: '#1E293B' },
  { id: 'sakura', name: 'Sakura', primary: '#f472b6', secondary: '#a78bfa', accent: '#c084fc', background: '#1a0a14', surface: '#2d1525' },
  { id: 'emerald', name: 'Emerald', primary: '#34d399', secondary: '#60a5fa', accent: '#a78bfa', background: '#0a1a14', surface: '#132d22' },
  { id: 'amber', name: 'Amber', primary: '#f59e0b', secondary: '#fb923c', accent: '#ef4444', background: '#1a1408', surface: '#2d2410' },
  { id: 'slate', name: 'Slate', primary: '#94a3b8', secondary: '#a78bfa', accent: '#6366f1', background: '#111318', surface: '#1e2028' },
  { id: 'asiimov', name: 'Asiimov', primary: '#ff6a00', secondary: '#facc15', accent: '#ff4500', background: '#0c0c0e', surface: '#1a1a1e' },
  { id: 'healing', name: 'Healing', primary: '#546E7A', secondary: '#78909C', accent: '#607D8B', background: '#f0f4f5', surface: '#ffffff' },
  { id: 'scholar', name: 'Scholar', primary: '#C4B5A0', secondary: '#A09080', accent: '#8D7B68', background: '#16130f', surface: '#231f19' },
  { id: 'scholar-light', name: 'Scholar Light', primary: '#8D7B68', secondary: '#A09080', accent: '#6B5B4E', background: '#f5f0ea', surface: '#ffffff' },
  { id: 'sakura-light', name: 'Sakura Light', primary: '#d84a8a', secondary: '#8b5cf6', accent: '#a855f7', background: '#fdf2f6', surface: '#ffffff' },
  { id: 'asiimov-light', name: 'Asiimov Light', primary: '#e05500', secondary: '#d4a017', accent: '#d44000', background: '#f5f3f0', surface: '#ffffff' },
  { id: 'strawberry', name: 'Strawberry', primary: '#ee342a', secondary: '#d4628a', accent: '#e02a20', background: '#f7f6ef', surface: '#ffffff' },
  { id: 'birthday', name: 'Birthday', primary: '#c026d3', secondary: '#0891b2', accent: '#e040fb', background: '#fdf4ff', surface: '#ffffff' },
  { id: 'birthday2', name: 'Birthday 2', primary: '#2563eb', secondary: '#dc2626', accent: '#16a34a', background: '#f5f7ff', surface: '#ffffff' },
]

const LIGHT_THEMES = new Set(['healing', 'sakura-light', 'scholar-light', 'asiimov-light', 'strawberry', 'birthday', 'birthday2'])
const isLightTheme = (themeId: string) => LIGHT_THEMES.has(themeId)

const STORAGE_KEY = 'anigraph_theme'

// Module-level singleton: captured once in a component setup context, then reused
let _vuetifyRef: ReturnType<typeof useVuetifyTheme> | null = null

const currentTheme = ref<string>(localStorage.getItem(STORAGE_KEY) || 'scholar-light')

export const useAppTheme = () => {
  // Try to capture the Vuetify ref if we don't have it yet.
  if (!_vuetifyRef) {
    try {
      _vuetifyRef = useVuetifyTheme()
    } catch {
      // Not in component context (plugin, async fn) -- will be picked up on next component mount
    }
  }

  // Sync Vuetify colors to match the current theme state.
  if (_vuetifyRef) {
    const saved = themes.find(t => t.id === currentTheme.value)
    if (saved) {
      const vuetifyName = isLightTheme(saved.id) ? 'light' : 'dark'
      _vuetifyRef.change(vuetifyName)
      const target = _vuetifyRef.themes.value[vuetifyName]
      target.colors.primary = saved.primary
      target.colors.secondary = saved.secondary
      target.colors.accent = saved.accent
      target.colors.background = saved.background
      target.colors.surface = saved.surface
    }
  }

  const applyTheme = (themeId: string) => {
    const theme = themes.find(t => t.id === themeId)
    if (!theme) return

    // Set data-theme attribute (drives CSS custom property overrides)
    document.documentElement.setAttribute('data-theme', themeId)

    // Sync Vuetify theme colors so color="primary" props update
    if (_vuetifyRef) {
      const vuetifyName = isLightTheme(themeId) ? 'light' : 'dark'
      _vuetifyRef.change(vuetifyName)
      const target = _vuetifyRef.themes.value[vuetifyName]
      target.colors.primary = theme.primary
      target.colors.secondary = theme.secondary
      target.colors.accent = theme.accent
      target.colors.background = theme.background
      target.colors.surface = theme.surface
    }
  }

  const setTheme = (themeId: string) => {
    currentTheme.value = themeId
    localStorage.setItem(STORAGE_KEY, themeId)
    applyTheme(themeId)
  }

  return {
    currentTheme: readonly(currentTheme),
    themes,
    setTheme,
    applyTheme,
  }
}
