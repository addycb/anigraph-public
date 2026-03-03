import DOMPurify from 'dompurify'

/**
 * Shared HTML sanitization composable for AniList descriptions and Wikipedia content.
 * Used across anime, staff, and studio pages.
 */
export const useSanitizeHtml = () => {
  /**
   * Sanitize AniList HTML descriptions (anime descriptions).
   * Allows only basic formatting tags.
   */
  const sanitizeDescription = (desc: string) => {
    return DOMPurify.sanitize(desc, {
      ALLOWED_TAGS: ['br', 'i', 'b', 'em', 'strong', 'p'],
      ALLOWED_ATTR: [],
      KEEP_CONTENT: true,
    })
  }

  /**
   * Convert AniList markdown-style bio text to sanitized HTML.
   * AniList staff bios use: __bold__, [text](url), *italic*, ''italic'', - list items, newlines
   */
  const sanitizeBio = (bio: string) => {
    let html = bio

    // Convert __text__ to <strong>text</strong>
    html = html.replace(/__([^_]+?)__/g, '<strong>$1</strong>')

    // Convert *text* to <em>text</em> (but not ** which would be bold in some markdown)
    html = html.replace(/(?<!\*)\*(?!\*)([^*]+?)(?<!\*)\*(?!\*)/g, '<em>$1</em>')

    // Convert ''text'' to <em>text</em> (wiki-style italics)
    html = html.replace(/''([^']+?)''/g, '<em>$1</em>')

    // Convert [text](url) to <a href="url">text</a>
    html = html.replace(/\[([^\]]+?)\]\(([^)]+?)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')

    // Convert lines starting with - to list items
    html = html.replace(/(?:^|\n)- (.+)/g, '\n<li>$1</li>')
    // Wrap consecutive <li> in <ul>
    html = html.replace(/((?:<li>.*?<\/li>\s*)+)/g, '<ul>$1</ul>')

    // Convert double newlines to paragraph breaks
    html = html.replace(/\n\n+/g, '</p><p>')
    // Convert single newlines to <br>
    html = html.replace(/\n/g, '<br>')
    // Wrap in <p> tags
    html = `<p>${html}</p>`
    // Clean up empty paragraphs
    html = html.replace(/<p>\s*<\/p>/g, '')

    return DOMPurify.sanitize(html, {
      ALLOWED_TAGS: ['br', 'i', 'b', 'em', 'strong', 'p', 'a', 'ul', 'li'],
      ALLOWED_ATTR: ['href', 'target', 'rel'],
      KEEP_CONTENT: true,
    })
  }

  /**
   * Sanitize Wikipedia HTML content.
   * Rewrites relative links, strips edit buttons, template artifacts, and infoboxes.
   *
   * @param html - Raw Wikipedia HTML
   * @param options.wikipediaUrl - Full Wikipedia article URL for anchor link rewriting
   * @param options.stripTopHeading - Remove top-level h2 elements (default: true)
   */
  const sanitizeWikipediaHtml = (
    html: string,
    options: { wikipediaUrl?: string; stripTopHeading?: boolean } = {}
  ) => {
    const { wikipediaUrl, stripTopHeading = true } = options

    // Rewrite relative Wikipedia links to absolute URLs
    let processed = html.replace(/href="\/wiki\//g, 'href="https://en.wikipedia.org/wiki/')
    processed = processed.replace(/href="\/w\//g, 'href="https://en.wikipedia.org/w/')

    // Rewrite same-page anchor links to point to the Wikipedia article
    if (wikipediaUrl) {
      processed = processed.replace(/href="#/g, `href="${wikipediaUrl}#`)
    }

    // Strip infobox tables
    processed = processed.replace(/<table[^>]*class="[^"]*infobox[^"]*"[^>]*>[\s\S]*?<\/table>/gi, '')

    // Strip cite error messages (class-based)
    processed = processed.replace(/<span[^>]*class="[^"]*mw-ext-cite-error[^"]*"[^>]*>[\s\S]*?<\/span>/gi, '')

    // Strip Wikipedia error messages about missing templates
    processed = processed.replace(/<li[^>]*>.*?references will not show.*?<\/li>/gi, '')
    processed = processed.replace(/<div[^>]*class="[^"]*mw-references-wrap[^"]*"[^>]*>[\s\S]*?<\/div>/gi, '')

    // Strip Cite error paragraphs/list items
    processed = processed.replace(/<li[^>]*>.*?Cite error.*?<\/li>/gi, '')
    processed = processed.replace(/<p[^>]*>.*?Cite error.*?<\/p>/gi, '')

    // Strip empty paragraphs
    processed = processed.replace(/<p>\s*<br\s*\/?>\s*<\/p>/gi, '')

    // Strip unrendered Wikipedia template artifacts
    processed = processed.replace(/\{\{[^{}]*(?:\{[^{}]*(?:\{[^{}]*\}[^{}]*)?\}[^{}]*)*\}\}/g, '')

    // Strip any remaining orphaned template error text
    processed = processed.replace(/\(see the help page\)\.?/g, '')

    // Strip [edit] sections and optionally top-level headings
    DOMPurify.addHook('beforeSanitizeElements', (node) => {
      if (node instanceof Element) {
        if (node.classList?.contains('mw-editsection')) {
          node.parentNode?.removeChild(node)
        }
        if (stripTopHeading && node.tagName === 'H2') {
          node.parentNode?.removeChild(node)
        }
      }
    })

    // Make all links open in a new tab
    DOMPurify.addHook('afterSanitizeAttributes', (node) => {
      if (node.tagName === 'A' && node.getAttribute('href')) {
        node.setAttribute('target', '_blank')
        node.setAttribute('rel', 'noopener noreferrer')
      }
    })

    const result = DOMPurify.sanitize(processed, {
      ALLOWED_TAGS: ['p', 'br', 'b', 'i', 'em', 'strong', 'ul', 'ol', 'li', 'h2', 'h3', 'h4', 'a', 'span', 'sup', 'sub', 'blockquote', 'cite', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'caption', 'dl', 'dt', 'dd'],
      ALLOWED_ATTR: ['href', 'title', 'class', 'target', 'rel'],
      KEEP_CONTENT: true,
      ADD_ATTR: ['target'],
      FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input'],
    })

    DOMPurify.removeAllHooks()
    return result
  }

  return {
    sanitizeDescription,
    sanitizeBio,
    sanitizeWikipediaHtml,
  }
}
