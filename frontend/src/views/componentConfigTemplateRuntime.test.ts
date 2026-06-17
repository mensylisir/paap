import { describe, expect, it } from 'vitest'
import {
  componentConfigTemplateMatchesComponent,
  componentTemplateFieldDefaultValue,
  componentTemplateFieldInputType,
  componentTemplateFieldKey,
  componentTemplateFieldLabel,
  componentTemplateFieldMatchesServiceRef,
  componentTemplateFieldTargetTokens,
  componentTemplateFieldType,
  componentTemplateInitialFieldValue,
  componentTemplateExistingFieldValue,
  componentTemplateListItemFields,
  componentTemplateListRows,
  componentTemplateRenderTargetValue,
  componentTemplateRequiredFieldsComplete,
  componentTemplateServicePasswordFieldKeys,
  defaultComponentTemplateListRow,
} from './componentConfigTemplateRuntime'

describe('component config template runtime helpers', () => {
  it('keeps explicitly matching component templates visible even when framework differs', () => {
    expect(componentConfigTemplateMatchesComponent(
      { framework: 'nginx', componentTypes: ['frontend'] },
      { componentType: 'frontend', framework: 'node' },
    )).toBe(true)
    expect(componentConfigTemplateMatchesComponent(
      { framework: 'springboot', componentTypes: ['backend'] },
      { componentType: 'frontend', framework: 'node' },
    )).toBe(false)
  })

  it('prefills password fields from existing env or secret names', () => {
    expect(componentTemplateExistingFieldValue(
      { key: 'DATABASE_PASSWORD', type: 'password' },
      {
        env: [{ name: 'POSTGRES_PASSWORD', value: 'from-env' }],
        secrets: [{ data: { SPRING_DATASOURCE_PASSWORD: 'from-secret' } }],
      },
    )).toBe('from-env')

    expect(componentTemplateExistingFieldValue(
      { key: 'REDIS_PASSWORD', type: 'password' },
      {
        env: [],
        secrets: [{ data: { REDIS_PASSWORD: 'redis-secret' } }],
      },
    )).toBe('redis-secret')
  })

  it('normalizes field metadata without depending on the component view', () => {
    const field = {
      key: 'database.password',
      label: '数据库密码',
      type: 'password',
      target: 'postgresql|mysql',
    }

    expect(componentTemplateFieldKey(field)).toBe('database.password')
    expect(componentTemplateFieldLabel(field)).toBe('数据库密码')
    expect(componentTemplateFieldType(field)).toBe('password')
    expect(componentTemplateFieldDefaultValue(field)).toBe('')
    expect(componentTemplateFieldInputType(field)).toBe('password')
    expect(componentTemplateFieldTargetTokens(field)).toEqual(['postgresql', 'mysql'])
  })

  it('builds list rows from item field defaults', () => {
    const field = {
      key: 'locations',
      type: 'list',
      itemFields: [
        { key: 'path', label: '路径', default: '/api' },
        { key: 'proxyPass', label: '转发地址' },
      ],
    }

    expect(componentTemplateListItemFields(field).map(componentTemplateFieldKey)).toEqual(['path', 'proxyPass'])
    expect(defaultComponentTemplateListRow(field)).toEqual({
      path: '/api',
      proxyPass: '',
    })
  })

  it('initializes booleans, service refs, and lists from injected UI state', () => {
    expect(componentTemplateInitialFieldValue({ key: 'enabled', type: 'boolean', default: 'yes' })).toBe(true)
    expect(componentTemplateInitialFieldValue(
      { key: 'database.service', type: 'serviceref' },
      { existingTargetKey: 'svc-postgres', firstOptionValue: 'svc-mysql' },
    )).toBe('svc-postgres')
    expect(componentTemplateInitialFieldValue({ key: 'locations', type: 'list', itemFields: [{ key: 'path', default: '/' }] })).toEqual([
      { path: '/' },
    ])
  })

  it('checks required fields including lists from field values', () => {
    const fields = [
      { key: 'database.url', required: true },
      { key: 'locations', type: 'list', required: true },
    ]

    expect(componentTemplateRequiredFieldsComplete(fields, {
      'database.url': 'jdbc:postgresql://postgres:5432/postgres',
      locations: [{ path: '/api' }],
    })).toBe(true)
    expect(componentTemplateRequiredFieldsComplete(fields, {
      'database.url': '',
      locations: [],
    })).toBe(false)
    expect(componentTemplateListRows(fields[1], { locations: [{ path: '/api' }] })).toEqual([{ path: '/api' }])
  })

  it('requires filled required item fields inside required list fields', () => {
    const fields = [
      {
        key: 'locations',
        type: 'list',
        required: true,
        itemFields: [
          { key: 'path', required: true },
          { key: 'proxyPass', required: true },
        ],
      },
    ]

    expect(componentTemplateRequiredFieldsComplete(fields, {
      locations: [{ path: '/api', proxyPass: '' }],
    })).toBe(false)
    expect(componentTemplateRequiredFieldsComplete(fields, {
      locations: [{ path: '/api', proxyPass: 'http://backend:8080' }],
    })).toBe(true)
  })

  it('renders service reference values from endpoint and credentials', () => {
    const target = { key: 'service:1', kind: 'service', type: 'postgresql' }
    const credentials = [
      { key: 'postgres-username', value: 'app' },
      { key: 'postgres-password', value: 'secret pass' },
    ]

    expect(componentTemplateRenderTargetValue(
      { key: 'database.jdbcUrl' },
      target,
      { endpoint: 'postgres.dev.svc.cluster.local:5432', defaultPort: 5432, credentials },
    )).toBe('jdbc:postgresql://postgres.dev.svc.cluster.local:5432/postgres')
    expect(componentTemplateRenderTargetValue(
      { key: 'database.url' },
      target,
      { endpoint: 'postgres.dev.svc.cluster.local:5432', defaultPort: 5432, credentials },
    )).toBe('postgresql://app:secret%20pass@postgres.dev.svc.cluster.local:5432/postgres')
  })

  it('renders service references from ordinary template keys and inferred formats', () => {
    const target = { key: 'service:1', kind: 'service', type: 'postgresql' }

    expect(componentTemplateRenderTargetValue(
      { key: 'JDBC_URL', format: 'jdbcUrl' },
      target,
      { endpoint: 'postgres.dev.svc.cluster.local:5432', defaultPort: 5432 },
    )).toBe('jdbc:postgresql://postgres.dev.svc.cluster.local:5432/postgres')
    expect(componentTemplateRenderTargetValue(
      { key: 'DATABASE_HOST', format: 'host' },
      target,
      { endpoint: 'postgres.dev.svc.cluster.local:5432', defaultPort: 5432 },
    )).toBe('postgres.dev.svc.cluster.local')
    expect(componentTemplateRenderTargetValue(
      { key: 'DATABASE_PORT', format: 'port' },
      target,
      { endpoint: 'postgres.dev.svc.cluster.local:5432', defaultPort: 5432 },
    )).toBe('5432')
  })

  it('matches ordinary password fields to selected database and redis service refs', () => {
    const fields = [
      { key: 'JDBC_URL', type: 'serviceRef', target: 'postgresql|mysql', format: 'jdbcUrl' },
      { key: 'DATABASE_PASSWORD', type: 'password' },
      { key: 'REDIS_HOST', type: 'serviceRef', target: 'redis', format: 'host' },
      { key: 'REDIS_PASSWORD', type: 'password' },
      { key: 'JWT_SECRET', type: 'password' },
    ]

    expect(componentTemplateFieldMatchesServiceRef(fields[1], fields[0])).toBe(true)
    expect(componentTemplateFieldMatchesServiceRef(fields[3], fields[2])).toBe(true)
    expect(componentTemplateFieldMatchesServiceRef(fields[4], fields[0])).toBe(false)
    expect(componentTemplateServicePasswordFieldKeys(fields, 'postgresql')).toEqual(['DATABASE_PASSWORD'])
    expect(componentTemplateServicePasswordFieldKeys(fields, 'redis')).toEqual(['REDIS_PASSWORD'])
  })
})
