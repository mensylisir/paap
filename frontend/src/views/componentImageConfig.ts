export interface ImageRepositoryAndTag {
  repository: string
  tag: string
}

function normalizeRegistryHost(host: string) {
  return String(host || '').trim().replace(/^https?:\/\//, '').replace(/\/$/, '')
}

export function splitImageRepositoryAndTag(image: string): ImageRepositoryAndTag {
  const value = String(image || '').trim()
  const last = value.split('/').pop() || ''
  const tagAt = last.lastIndexOf(':')
  if (tagAt < 0) return { repository: value, tag: '' }
  return {
    repository: value.slice(0, value.length - (last.length - tagAt)),
    tag: last.slice(tagAt + 1),
  }
}

export function registryHostFromImage(image: string) {
  const value = String(image || '').trim().replace(/^https?:\/\//, '')
  const parts = value.split('/')
  if (parts.length < 2) return ''
  const first = parts[0] || ''
  return first === 'localhost' || first.includes('.') || first.includes(':') ? first : ''
}

export function stripRegistryHost(image: string, registryHost = '') {
  const value = String(image || '').trim().replace(/^\/+/, '')
  if (!value) return ''
  const preferredHost = normalizeRegistryHost(registryHost)
  if (preferredHost && value.startsWith(`${preferredHost}/`)) {
    return value.slice(preferredHost.length + 1)
  }
  const detectedHost = registryHostFromImage(value)
  if (detectedHost) return value.slice(detectedHost.length + 1)
  return value
}

export function registryRepositorySuffix(repository: string, registryHost = '') {
  return stripRegistryHost(splitImageRepositoryAndTag(repository).repository, registryHost)
}

export function registryHostForImageField(image: string, fallbackHost = '') {
  return registryHostFromImage(image) || normalizeRegistryHost(fallbackHost)
}

export function imageTagForImageField(image: string, fallbackHost = '') {
  return stripRegistryHost(image, registryHostFromImage(image) || fallbackHost)
}

export function imageTagVersion(imageTag: string) {
  return splitImageRepositoryAndTag(stripRegistryHost(imageTag)).tag
}

export function imageRefFromRegistryFields(registryHost: string, imageTag: string) {
  const trimmedImageTag = String(imageTag || '').trim().replace(/^\/+/, '')
  if (!trimmedImageTag) return ''
  if (registryHostFromImage(trimmedImageTag)) return trimmedImageTag
  const host = normalizeRegistryHost(registryHost)
  return host ? `${host}/${trimmedImageTag}` : trimmedImageTag
}
