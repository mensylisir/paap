import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  withEmbeddedProxyAuthToken,
  serviceAccessUrl,
  serviceProxyUrl,
  serviceWorkspaceKind,
  validateWorkspaceActionParams,
} from './serviceWorkspace'

describe('serviceWorkspace', () => {
  it('keeps service workspace data backend-owned instead of synthesizing resources locally', () => {
    const source = readFileSync(new URL('./serviceWorkspace.ts', import.meta.url), 'utf8')

    expect(source).not.toContain('buildServiceWorkspace')
    expect(source).not.toContain('repositoryWorkspace')
    expect(source).not.toContain('gitopsWorkspace')
    expect(source).not.toContain('observabilityWorkspace')
    expect(source).not.toContain('pipelineWorkspace')
    expect(source).not.toContain('registryWorkspace')
    expect(source).not.toContain('dataWorkspace')
    expect(source).not.toContain('genericWorkspace')
    expect(source).not.toContain('Runtime Trust')
    expect(source).not.toContain('Grafana Loki Panel')
    expect(source).not.toContain('Dashboard')
    expect(source).not.toContain('Connection')
  })

  it('derives deterministic proxy and in-cluster access URLs', () => {
    expect(serviceAccessUrl('billing-dev-git', 'git')).toBe('http://billing-dev-git.billing-dev-git.svc.cluster.local:3000')
    expect(serviceAccessUrl('billing-prod-deploy', 'deploy')).toBe('http://billing-prod-deploy-argocd-server.billing-prod-deploy.svc.cluster.local')
    expect(serviceAccessUrl('billing-prod-registry', 'registry')).toBe('https://billing-prod-registry.billing-prod-registry.svc.cluster.local:5000')
    expect(serviceProxyUrl(1, 9, '/api/health')).toBe('/api/v1/environments/1/services/9/proxy/api/health')
    expect(serviceProxyUrl(undefined, 9, '/api/health')).toBe('')
  })

  it('adds auth tokens only to embedded same-origin service proxy URLs', () => {
    expect(withEmbeddedProxyAuthToken('/api/v1/environments/1/services/9/proxy/d/node?orgId=1#view', 'signed.jwt.token'))
      .toBe('/api/v1/environments/1/services/9/proxy/d/node?orgId=1&paap_token=signed.jwt.token#view')
    expect(withEmbeddedProxyAuthToken('/api/v1/environments/1/services/9/runtime-metrics', 'signed.jwt.token'))
      .toBe('/api/v1/environments/1/services/9/runtime-metrics')
    expect(withEmbeddedProxyAuthToken('https://grafana.example.com/d/node?orgId=1', 'signed.jwt.token'))
      .toBe('https://grafana.example.com/d/node?orgId=1')
  })

  it('maps service types to workspace renderers without generating workspace contents', () => {
    expect(serviceWorkspaceKind('git')).toBe('repository')
    expect(serviceWorkspaceKind('deploy')).toBe('gitops')
    expect(serviceWorkspaceKind('monitor')).toBe('observability')
    expect(serviceWorkspaceKind('log')).toBe('observability')
    expect(serviceWorkspaceKind('redis')).toBe('data')
    expect(serviceWorkspaceKind('custom-tool')).toBe('generic')
  })

  it('validates required workspace action fields without browser alerts', () => {
    const message = validateWorkspaceActionParams(
      [{ name: 'database', label: '数据库名', required: true }],
      { database: '   ' },
    )

    expect(message).toBe('请填写：数据库名')
    expect(validateWorkspaceActionParams([{ name: 'database', label: '数据库名', required: true }], { database: 'appdb' })).toBe('')
  })
})
