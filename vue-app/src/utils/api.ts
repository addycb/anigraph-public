const API_BASE = import.meta.env.VITE_API_BASE || '/api'

export async function api<T>(path: string, options?: RequestInit & { params?: Record<string, string> }): Promise<T> {
  const url = new URL(API_BASE + path, window.location.origin)
  if (options?.params) {
    for (const [k, v] of Object.entries(options.params)) {
      url.searchParams.set(k, v)
    }
  }
  const { params: _, ...fetchOptions } = options || {} as any
  const res = await fetch(url.toString(), {
    ...fetchOptions,
    headers: { 'Content-Type': 'application/json', ...fetchOptions?.headers },
  })
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new Error(`API ${res.status}: ${body}`)
  }
  return res.json()
}
