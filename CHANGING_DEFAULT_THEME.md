# Changing the Default Theme

The default theme is what new users (or users with no saved preference) see on first load. It must be updated in **4 places** to avoid a flash of the old theme during SSR/hydration.

## Files to update

Replace `<old-theme>` with `<new-theme>` in each file:

### 1. `nuxt-app/composables/useTheme.ts`
The JS state fallback — used on both server and client.
```ts
return localStorage.getItem(STORAGE_KEY) || '<new-theme>'
// and the SSR branch below it:
return '<new-theme>'
```

### 2. `nuxt-app/plugins/theme-init.client.ts`
The client-side init that applies the theme on page load.
```ts
const themeId = localStorage.getItem(STORAGE_KEY) || '<new-theme>'
```

### 3. `nuxt-app/plugins/vuetify.ts`
The Vuetify SSR color defaults (the `light` theme colors). Copy the new theme's color values from `useTheme.ts`:
```ts
theme: {
  defaultTheme: 'light',
  themes: {
    light: {
      colors: {
        primary: '...',
        secondary: '...',
        // etc.
      }
    }
  }
}
```

### 4. `nuxt-app/assets/tokens.css`
The CSS variable defaults in `:root`. Copy all values from the new theme's `[data-theme="<new-theme>"]` block into the `:root` block at the top of the file.

## Available themes

Theme IDs can be found in `nuxt-app/composables/useTheme.ts` in the `allAppThemes` array.
Light themes: `healing`, `scholar-light`, `sakura-light`, `asiimov-light`, `strawberry`, `birthday`, `birthday2`
Dark themes: `midnight`, `slate`, `scholar`, and others.

> Note: The `[data-theme="<id>"]` block in `tokens.css` and the theme entry in `useTheme.ts` are the source of truth for a theme's colors. Always copy from there when updating the defaults.
