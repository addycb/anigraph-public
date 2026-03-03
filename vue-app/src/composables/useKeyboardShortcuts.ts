import { onMounted, onBeforeUnmount } from 'vue'

type ShortcutHandlers = Record<string, (e: KeyboardEvent) => void>

/**
 * Register keyboard shortcuts that are automatically cleaned up on unmount.
 * Shortcuts are ignored when focus is inside an input, textarea, select, or
 * contenteditable element.
 */
export const useKeyboardShortcuts = (shortcuts: ShortcutHandlers) => {
  const handleKeyDown = (e: KeyboardEvent) => {
    const target = e.target as HTMLElement
    if (
      target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.tagName === 'SELECT' ||
      target.isContentEditable
    ) return

    // Ignore when modifier keys are held (e.g. Ctrl+F should not trigger 'f')
    if (e.ctrlKey || e.metaKey || e.altKey) return

    const handler = shortcuts[e.key]
    if (handler) handler(e)
  }

  onMounted(() => document.addEventListener('keydown', handleKeyDown))
  onBeforeUnmount(() => document.removeEventListener('keydown', handleKeyDown))
}
