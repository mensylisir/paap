import type { WorkspaceResource } from '../../views/serviceWorkspace'

export function repositoryDefaultPath(repo: WorkspaceResource | null | undefined) {
  return String(repo?.annotations?.path || repo?.annotations?.defaultPath || '')
    .replace(/^\/+|\/+$/g, '')
}

export function repositoryIdentity(repo: WorkspaceResource) {
  const base = String(repo.annotations?.cloneURL || repo.annotations?.htmlURL || repo.externalUrl || repo.name)
  const defaultPath = repositoryDefaultPath(repo)
  return defaultPath ? `${base}#${defaultPath}` : base
}

export function repositoryInitialPath(repo: WorkspaceResource | null | undefined) {
  return repositoryDefaultPath(repo)
}

export function repositoryProxyBrowserUrl(repo: WorkspaceResource | null | undefined) {
  return String(repo?.annotations?.proxyURL || repo?.externalUrl || '').trim()
}

export function repositoryContentsUrl(repo: WorkspaceResource, path = '') {
  const browserUrl = repositoryProxyBrowserUrl(repo)
  const marker = '/proxy/'
  const idx = browserUrl.indexOf(marker)
  if (idx < 0) return ''
  const proxyBase = browserUrl.slice(0, idx + marker.length - 1)
  const repoPath = browserUrl.slice(idx + marker.length).replace(/^\/+|\/+$/g, '')
  if (!repoPath) return ''
  const branch = encodeURIComponent(String(repo.annotations?.branch || 'main'))
  const encodedPath = path.split('/').filter(Boolean).map(encodeURIComponent).join('/')
  const suffix = encodedPath ? `/${encodedPath}` : ''
  return `${proxyBase}/api/v1/repos/${repoPath}/contents${suffix}?ref=${branch}`
}

export function repositoryCloneUrl(repo: WorkspaceResource | null | undefined) {
  const explicitExternal = String(repo?.annotations?.externalCloneURL || '').trim()
  if (explicitExternal) return explicitExternal

  const externalUrl = String(repo?.externalUrl || '').trim()
  if (externalUrl && !externalUrl.includes('/proxy/')) {
    return `${externalUrl.replace(/\/+$/g, '')}.git`
  }

  return String(repo?.annotations?.cloneURL || '').trim()
}
