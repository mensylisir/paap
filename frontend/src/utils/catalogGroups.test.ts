import { describe, expect, it } from 'vitest'
import * as catalogGroups from './catalogGroups'
import { catalogGroupForTemplate, compareCatalogGroupMeta } from './catalogGroups'

describe('catalog group helpers', () => {
  it('maps infrastructure templates into product categories', () => {
    expect(catalogGroupForTemplate({ category: 'tool', type: 'jenkins' })).toMatchObject({ category: 'ci', label: 'CI服务' })
    expect(catalogGroupForTemplate({ category: 'tool', type: 'deploy' })).toMatchObject({ category: 'cd', label: 'CD服务' })
    expect(catalogGroupForTemplate({ category: 'tool', type: 'monitor' })).toMatchObject({ category: 'monitor', label: '监控服务' })
    expect(catalogGroupForTemplate({ category: 'tool', type: 'log' })).toMatchObject({ category: 'log', label: '日志服务' })
    expect(catalogGroupForTemplate({ category: 'middleware', type: 'registry' })).toMatchObject({ category: 'middleware', label: '中间件服务' })
    expect(catalogGroupForTemplate({ category: 'middleware', type: 'harbor' })).toMatchObject({ category: 'middleware', label: '中间件服务' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'postgresql' })).toMatchObject({ category: 'database', label: '数据库服务' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'mysql' })).toMatchObject({ category: 'database', label: '数据库服务' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'mongodb' })).toMatchObject({ category: 'database', label: '数据库服务' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'redis' })).toMatchObject({ category: 'middleware', label: '中间件服务' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'rabbitmq' })).toMatchObject({ category: 'middleware', label: '中间件服务' })
    expect(catalogGroupForTemplate({ category: 'environment', type: 'environment:dev' })).toMatchObject({ category: 'environment', label: '环境服务' })
    expect(catalogGroupForTemplate({ category: 'kubevirt', type: 'kubevirt-postgresql' })).toMatchObject({ category: 'virtualMachine', label: '虚拟机服务' })
  })

  it('keeps catalog groups in a stable product order', () => {
    const groups = [
      catalogGroupForTemplate({ type: 'redis' }),
      catalogGroupForTemplate({ type: 'deploy', category: 'tool' }),
      catalogGroupForTemplate({ type: 'rabbitmq' }),
      catalogGroupForTemplate({ type: 'postgresql' }),
      catalogGroupForTemplate({ type: 'monitor' }),
      catalogGroupForTemplate({ type: 'log' }),
    ]

    expect(groups.sort(compareCatalogGroupMeta).map(group => group.category)).toEqual([
      'cd',
      'monitor',
      'log',
      'database',
      'middleware',
      'middleware',
    ])
  })

  it('matches search queries against product group labels', () => {
    const matchesQuery = (catalogGroups as any).catalogTemplateMatchesQuery

    expect(matchesQuery).toBeTypeOf('function')
    expect(matchesQuery({ category: 'infra', type: 'redis', name: 'Redis' }, '中间件服务')).toBe(true)
    expect(matchesQuery({ category: 'infra', type: 'rabbitmq', name: 'RabbitMQ' }, '中间件服务')).toBe(true)
    expect(matchesQuery({ category: 'infra', type: 'postgresql', name: 'PostgreSQL' }, '数据库服务')).toBe(true)
    expect(matchesQuery({ category: 'environment', type: 'environment:dev', name: '开发环境' }, '环境服务')).toBe(true)
    expect(matchesQuery({ category: 'infra', type: 'redis', name: 'Redis' }, '数据库')).toBe(false)
  })
})
