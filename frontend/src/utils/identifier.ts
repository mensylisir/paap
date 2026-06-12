const nonDnsLabel = /[^a-z0-9-]+/g

export function toIdentifier(input: string, fallback = 'item', maxLen = 50) {
  let value = String(input || '')
    .trim()
    .toLowerCase()
    .replace(/_/g, '-')
    .replace(nonDnsLabel, '-')
    .replace(/^-+|-+$/g, '')

  if (!value || !/^[a-z]/.test(value)) {
    value = fallback
  }

  if (maxLen > 0 && value.length > maxLen) {
    value = value.slice(0, maxLen).replace(/-+$/g, '')
  }

  return value || fallback
}
