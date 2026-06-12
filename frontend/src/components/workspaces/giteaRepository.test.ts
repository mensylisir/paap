import { describe, expect, it } from 'vitest'
import { repositoryCloneUrl, repositoryContentsUrl, repositoryIdentity, repositoryInitialPath } from './giteaRepository'
import type { WorkspaceResource } from '../../views/serviceWorkspace'

describe('giteaRepository helpers', () => {
  const baseRepo = {
    name: 'test-staging-components',
    type: 'Repository',
    status: 'Ready',
    description: 'component repo root',
    externalUrl: '/api/v1/environments/1/services/3/proxy/paap/test-staging-components',
    annotations: {
      cloneURL: 'http://gitea/paap/test-staging-components.git',
      componentPaths: ['components/backend-3', 'components/source-smoke'],
    },
  } satisfies WorkspaceResource

  it('opens GitOps repositories at the real repository root', () => {
    expect(repositoryInitialPath(baseRepo)).toBe('')
  })

  it('keeps source mirrors distinct from the GitOps repository', () => {
    const sourceMirror = {
      ...baseRepo,
      name: 'test-staging-source-smoke-source',
      externalUrl: '/api/v1/environments/1/services/3/proxy/paap/paap-source-smoke-go',
      annotations: {
        cloneURL: 'http://gitea/paap/paap-source-smoke-go.git',
        repositoryRole: 'source',
      },
    } satisfies WorkspaceResource

    expect(repositoryIdentity(baseRepo)).not.toBe(repositoryIdentity(sourceMirror))
  })

  it('loads repository contents through the same-origin proxy when externalUrl is browser reachable', () => {
    const repo = {
      ...baseRepo,
      externalUrl: 'http://172.18.0.2:31236/paap/test-staging-components',
      annotations: {
        ...baseRepo.annotations,
        branch: 'main',
        proxyURL: '/api/v1/environments/1/services/3/proxy/paap/test-staging-components',
      },
    } satisfies WorkspaceResource

    expect(repositoryContentsUrl(repo)).toBe('/api/v1/environments/1/services/3/proxy/api/v1/repos/paap/test-staging-components/contents?ref=main')
    expect(repositoryContentsUrl(repo, 'components/source-smoke')).toBe('/api/v1/environments/1/services/3/proxy/api/v1/repos/paap/test-staging-components/contents/components/source-smoke?ref=main')
  })

  it('shows a browser-reachable clone URL instead of the cluster-internal clone URL', () => {
    const repo = {
      ...baseRepo,
      externalUrl: 'http://172.18.0.2:31236/paap/test-staging-components',
      annotations: {
        ...baseRepo.annotations,
        cloneURL: 'http://test-staging-git:3000/paap/test-staging-components.git',
      },
    } satisfies WorkspaceResource

    expect(repositoryCloneUrl(repo)).toBe('http://172.18.0.2:31236/paap/test-staging-components.git')
  })
})
