import { describe, expect, it } from 'vitest'
import {
  mergeComponentBinding,
  nginxRouteRowsFromComponentConfig,
  nginxRouteRowsToTemplateListRows,
  nginxTemplateListRowsWithGeneratedDirectives,
  nginxTemplateListRowsToRouteRows,
} from './componentNginxRoutes'

describe('componentNginxRoutes', () => {
  it('recovers proxy route paths from generated nginx config when binding metadata was overwritten', () => {
    const rows = nginxRouteRowsFromComponentConfig({
      bindings: [{
        targetKey: 'component:38',
        targetName: 'backend-1',
        targetType: 'backend',
        role: 'backend',
        generated: { templateField: 'PROXY_PASS' },
      }],
      configMaps: [{
        name: 'frontend-1-config',
        data: {
          'default.conf': [
            'server {',
            '  location / {',
            '    try_files $uri $uri/ /index.html;',
            '  }',
            '',
            '  location /api {',
            '    proxy_pass http://backend-1;',
            '  }',
            '}',
          ].join('\n'),
        },
      }],
      backendTargets: [{ key: 'component:38', name: 'backend-1', serviceName: 'backend-1' }],
    })

    expect(rows).toEqual([{ path: '/api', targetKey: 'component:38', targetUrl: 'http://backend-1' }])
  })

  it('preserves route metadata when a later template binding targets the same backend', () => {
    const bindings = mergeComponentBinding([{
      targetKey: 'component:38',
      targetName: 'backend-1',
      targetType: 'backend',
      role: 'backend',
      generated: { locationPath: '/api', proxyPass: 'http://backend-1', PROXY_ROUTE_1: '/api -> http://backend-1' },
    }], {
      targetKey: 'component:38',
      targetName: 'backend-1',
      targetType: 'backend',
      role: 'backend',
      generated: { templateField: 'PROXY_PASS' },
    })

    expect(bindings[0].generated).toMatchObject({
      locationPath: '/api',
      proxyPass: 'http://backend-1',
      templateField: 'PROXY_PASS',
    })
  })

  it('maps recovered nginx routes into template list field rows', () => {
    const rows = nginxRouteRowsToTemplateListRows([{
      path: '/api',
      targetKey: 'component:38',
      targetUrl: 'http://backend-1',
    }], {
      key: 'LOCATION_LIST',
      type: 'list',
      itemFields: [
        { key: 'PATH', label: '匹配路径', type: 'text' },
        { key: 'PROXY_PASS', label: '转发地址', type: 'serviceRef', target: 'backend' },
      ],
    })

    expect(rows).toEqual([{ PATH: '/api', PROXY_PASS: 'component:38' }])
  })

  it('maps template list field rows back into nginx route rows before save', () => {
    const rows = nginxTemplateListRowsToRouteRows([{
      PATH: '/api',
      PROXY_PASS: 'component:38',
    }], {
      key: 'LOCATION_LIST',
      type: 'list',
      itemFields: [
        { key: 'PATH', label: '匹配路径', type: 'text' },
        { key: 'PROXY_PASS', label: '转发地址', type: 'serviceRef', target: 'backend' },
      ],
    }, [{ key: 'component:38', name: 'backend-1', serviceName: 'backend-1' }])

    expect(rows).toEqual([{ path: '/api', targetKey: 'component:38', targetUrl: 'http://backend-1' }])
  })

  it('supports generic location directive list fields', () => {
    const field = {
      key: 'LOCATION_LIST',
      type: 'list',
      itemFields: [
        { key: 'MATCH', label: '匹配规则', type: 'text' },
        { key: 'DIRECTIVES', label: '指令块', type: 'textarea' },
      ],
    }

    const generated = nginxRouteRowsToTemplateListRows([{
      path: '/api',
      targetKey: '',
      targetUrl: 'http://backend-1',
    }], field)

    expect(generated[0]).toMatchObject({ MATCH: '/api' })
    expect(generated[0].DIRECTIVES).toContain('proxy_pass http://backend-1;')

    const recovered = nginxTemplateListRowsToRouteRows(generated, field, [
      { key: 'component:38', name: 'backend-1', serviceName: 'backend-1' },
    ])

    expect(recovered).toEqual([{ path: '/api', targetKey: 'component:38', targetUrl: 'http://backend-1' }])
  })

  it('generates hidden nginx directives from a selected backend service reference', () => {
    const field = {
      key: 'LOCATION_LIST',
      type: 'list',
      itemFields: [
        { key: 'MATCH', label: '匹配规则', type: 'text' },
        { key: 'PROXY_PASS', label: '目标后端', type: 'serviceRef', target: 'backend' },
        { key: 'DIRECTIVES', label: '额外指令', type: 'textarea', hidden: true },
      ],
    }

    const rows = nginxTemplateListRowsWithGeneratedDirectives([{
      MATCH: '/api',
      PROXY_PASS: 'component:38',
      DIRECTIVES: 'proxy_pass http://backend:8080;',
    }], field, [
      { key: 'component:38', name: 'backend-1', serviceName: 'backend-1' },
    ])

    expect(rows).toEqual([expect.objectContaining({
      MATCH: '/api',
      PROXY_PASS: 'http://backend-1',
    })])
    expect(rows[0].DIRECTIVES).toContain('proxy_pass http://backend-1;')
    expect(rows[0].DIRECTIVES).not.toContain('backend:8080')
  })
})
