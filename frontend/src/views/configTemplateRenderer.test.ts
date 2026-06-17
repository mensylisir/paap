import { describe, expect, it } from 'vitest'
import { renderPaapTemplateValue } from './configTemplateRenderer'

describe('config template renderer', () => {
  it('renders list blocks with item fields', () => {
    const rendered = renderPaapTemplateValue(
      [
        'server {',
        '[[paap:for LOCATION_LIST]]',
        'location [[paap:item.API_PATH]] {',
        '  proxy_pass [[paap:item.PROXY_PASS]];',
        '}',
        '[[paap:end LOCATION_LIST]]',
        '}',
      ].join('\n'),
      {
        fieldValues: {
          LOCATION_LIST: [
            { API_PATH: '/api/', PROXY_PASS: 'http://backend' },
            { API_PATH: '/admin/', PROXY_PASS: 'http://admin' },
          ],
        },
      },
    )

    expect(rendered).toContain('location /api/')
    expect(rendered).toContain('proxy_pass http://backend;')
    expect(rendered).toContain('location /admin/')
    expect(rendered).toContain('proxy_pass http://admin;')
    expect(rendered).not.toContain('[[paap:for LOCATION_LIST]]')
  })

  it('skips false condition blocks and renders true condition blocks', () => {
    const rendered = renderPaapTemplateValue(
      [
        'spring:',
        '[[paap:if REDIS_ENABLED]]',
        '  redis:',
        '    host: [[paap:REDIS_HOST default=redis-master]]',
        '[[paap:end REDIS_ENABLED]]',
        '[[paap:if MQ_ENABLED]]',
        '  rabbitmq:',
        '    host: [[paap:MQ_HOST]]',
        '[[paap:end MQ_ENABLED]]',
      ].join('\n'),
      {
        fieldValues: {
          REDIS_ENABLED: true,
          MQ_ENABLED: false,
        },
      },
    )

    expect(rendered).toContain('redis:')
    expect(rendered).toContain('host: redis-master')
    expect(rendered).not.toContain('rabbitmq:')
  })
})
