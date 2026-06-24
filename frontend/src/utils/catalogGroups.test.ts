import { describe, expect, it } from 'vitest'
import { catalogGroupForTemplate, compareCatalogGroupMeta } from './catalogGroups'

describe('catalog group helpers', () => {
  it('maps infrastructure templates into product categories', () => {
    expect(catalogGroupForTemplate({ category: 'infra', type: 'postgresql' })).toMatchObject({ category: 'database', label: '数据库' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'mysql' })).toMatchObject({ category: 'database', label: '数据库' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'mongodb' })).toMatchObject({ category: 'database', label: '数据库' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'redis' })).toMatchObject({ category: 'cache', label: '缓存' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'rabbitmq' })).toMatchObject({ category: 'mq', label: '消息队列' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'kafka' })).toMatchObject({ category: 'mq', label: '消息队列' })
    expect(catalogGroupForTemplate({ category: 'infra', type: 'minio' })).toMatchObject({ category: 'objectStorage', label: '对象存储' })
  })

  it('keeps catalog groups in a stable product order', () => {
    const groups = [
      catalogGroupForTemplate({ type: 'redis' }),
      catalogGroupForTemplate({ type: 'deploy', category: 'tool' }),
      catalogGroupForTemplate({ type: 'rabbitmq' }),
      catalogGroupForTemplate({ type: 'postgresql' }),
      catalogGroupForTemplate({ type: 'minio' }),
    ]

    expect(groups.sort(compareCatalogGroupMeta).map(group => group.category)).toEqual([
      'tool',
      'database',
      'cache',
      'mq',
      'objectStorage',
    ])
  })
})
