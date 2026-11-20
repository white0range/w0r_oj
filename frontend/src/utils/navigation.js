export function safeReturnTo(value, fallback = '/') {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) return fallback
  try {
    const url = new URL(value, window.location.origin)
    if (url.origin === window.location.origin && !/^\/(?:login|register)(?:\/|$)/.test(url.pathname)) {
      return url.pathname + url.search + url.hash
    }
  } catch {
    return fallback
  }
  return fallback
}
