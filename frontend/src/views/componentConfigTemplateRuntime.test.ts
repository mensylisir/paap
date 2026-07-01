import { describe, expect, it } from 'vitest'
import {
  componentConfigTemplateEffectiveType,
  componentConfigTemplateMatchesSelection,
  componentConfigTemplateMatchesComponent,
  componentConfigTemplateRecommendationScore,
  componentConfigTemplateSelectValue,
  componentTemplateFieldDefaultValue,
  componentTemplateFieldInputType,
  componentTemplateFieldKey,
  componentTemplateFieldLabel,
  componentTemplateFieldMatchesServiceRef,
  componentTemplateFieldTargetTokens,
  componentTemplateFieldType,
  componentTemplateInitialFieldValue,
  componentTemplateExistingFieldValue,
  componentTemplateFieldHidden,
  componentTemplateListItemFields,
  componentTemplateListRows,
  componentTemplateVisibleListItemFields,
  componentTemplateRenderTargetValue,
  componentTemplateRequiredFieldsComplete,
  componentTemplateServicePasswordFieldKeys,
  componentTemplateServiceTypeGroup,
  componentTemplateServiceTypeMatchesTargets,
  componentTemplateServiceUsernameFieldKeys,
  componentTemplateSplitEndpoint,
  defaultComponentTemplateListRow,
  resolveComponentConfigTemplateSelection,
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

  it('matches Spring Cloud gateway components to backend Spring templates even when exposed as frontend entrypoints', () => {
    const gatewayOptions = { componentName: 'gateway', componentType: 'frontend', framework: 'springboot' }

    expect(componentConfigTemplateEffectiveType(gatewayOptions)).toBe('backend')
    expect(componentConfigTemplateMatchesComponent(
      { framework: 'springboot', componentTypes: ['backend'], key: 'piggymetrics-gateway-runtime' },
      gatewayOptions,
    )).toBe(true)

    const gatewayScore = componentConfigTemplateRecommendationScore(
      { framework: 'springboot', componentTypes: ['backend'], key: 'piggymetrics-gateway-runtime', name: 'PiggyMetrics gateway 运行变量' },
      gatewayOptions,
    )
    const genericScore = componentConfigTemplateRecommendationScore(
      { framework: 'springboot', componentTypes: ['backend'], key: 'springboot-base', name: 'Spring Boot 基础配置' },
      gatewayOptions,
    )

    expect(gatewayScore).toBeGreaterThan(genericScore)
  })

  it('keeps component-specific templates visible before the framework is explicitly selected', () => {
    const gatewayOptions = { componentName: 'gateway', componentType: 'frontend', framework: 'auto' }

    expect(componentConfigTemplateMatchesComponent(
      { framework: 'springboot', componentTypes: ['backend'], key: 'piggymetrics-gateway-runtime' },
      gatewayOptions,
    )).toBe(true)

    expect(componentConfigTemplateMatchesComponent(
      { framework: 'springboot', componentTypes: ['backend'], key: 'springboot-base' },
      gatewayOptions,
    )).toBe(false)
  })

  it('resolves saved template metadata to the actual select option value', () => {
    const templates = [
      { id: 31, key: 'springboot-base', name: 'Spring Boot 基础配置' },
      { id: 42, key: 'piggymetrics-gateway-runtime', name: 'PiggyMetrics gateway 运行变量' },
    ]

    expect(componentConfigTemplateSelectValue(templates[1])).toBe('42')
    expect(componentConfigTemplateMatchesSelection(templates[1], 'piggymetrics-gateway-runtime')).toBe(true)
    expect(componentConfigTemplateMatchesSelection(templates[1], 'PiggyMetrics gateway 运行变量')).toBe(true)
    expect(resolveComponentConfigTemplateSelection(templates, 'PiggyMetrics gateway 运行变量')).toBe('42')
    expect(resolveComponentConfigTemplateSelection(templates, 'piggymetrics-gateway-runtime')).toBe('42')
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

    expect(componentTemplateExistingFieldValue(
      { key: 'CONFIG_SERVICE_PASSWORD', type: 'password' },
      {
        env: [],
        secrets: [{ data: { CONFIG_SERVICE_PASSWORD: '[[paap:CONFIG_SERVICE_PASSWORD default=cfg-pwd-2026]]' } }],
      },
    )).toBe('cfg-pwd-2026')
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
    expect(componentTemplateFieldDefaultValue({
      key: 'config.password',
      default: '[[paap:CONFIG_SERVICE_PASSWORD default=cfg-pwd-2026]]',
    })).toBe('cfg-pwd-2026')
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

  it('keeps hidden list item fields available for rendering but out of the editable UI', () => {
    const field = {
      key: 'locations',
      type: 'list',
      itemFields: [
        { key: 'path', label: '路径', default: '/api' },
        { key: 'target', label: '目标后端', type: 'serviceRef', target: 'backend' },
        { key: 'directives', label: '额外指令', type: 'textarea', hidden: true, default: 'proxy_pass http://backend;' },
      ],
    }

    expect(componentTemplateFieldHidden(field.itemFields[2])).toBe(true)
    expect(componentTemplateListItemFields(field).map(componentTemplateFieldKey)).toEqual(['path', 'target', 'directives'])
    expect(componentTemplateVisibleListItemFields(field).map(componentTemplateFieldKey)).toEqual(['path', 'target'])
    expect(defaultComponentTemplateListRow(field)).toEqual({
      path: '/api',
      target: '',
      directives: 'proxy_pass http://backend;',
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

  it('parses external capability URI endpoints before rendering template refs', () => {
    expect(componentTemplateSplitEndpoint('postgres://user:pass@db.example.com:5432/app', 5432))
      .toEqual(['db.example.com', 5432])
    expect(componentTemplateSplitEndpoint('redis://redis.example.com:6379/0', 6379))
      .toEqual(['redis.example.com', 6379])
    expect(componentTemplateSplitEndpoint('https://gitea.example.com/paap/repo', 3000))
      .toEqual(['gitea.example.com', 3000])

    expect(componentTemplateRenderTargetValue(
      { key: 'DATABASE_HOST', format: 'host' },
      { key: 'capability:9', kind: 'capability', type: 'postgresql' },
      { endpoint: 'postgres://user:pass@db.example.com:5432/app', defaultPort: 5432 },
    )).toBe('db.example.com')
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

  it('renders eureka service references as Spring Cloud defaultZone URLs', () => {
    const target = { key: 'service:5', kind: 'service', type: 'eureka' }

    expect(componentTemplateRenderTargetValue(
      { key: 'EUREKA_URL', type: 'serviceRef', target: 'eureka', format: 'eurekaUrl' },
      target,
      { endpoint: 'piggymetrics-dev-eureka.piggymetrics-dev-eureka.svc.cluster.local:8761', defaultPort: 8761 },
    )).toBe('http://piggymetrics-dev-eureka.piggymetrics-dev-eureka.svc.cluster.local:8761/eureka/')
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

  it('matches service references by logical service type aliases', () => {
    expect(componentTemplateServiceTypeGroup('postgresql-ha')).toBe('database')
    expect(componentTemplateServiceTypeMatchesTargets('postgresql-ha', ['postgresql'])).toBe(true)
    expect(componentTemplateServiceTypeMatchesTargets('mysql-galera', ['database'])).toBe(true)
    expect(componentTemplateServiceTypeMatchesTargets('redis-cluster', ['redis'])).toBe(true)
    expect(componentTemplateServiceTypeMatchesTargets('harbor', ['registry'])).toBe(true)
    expect(componentTemplateServiceTypeMatchesTargets('rabbitmq', ['redis'])).toBe(false)
  })

  it('finds username fields that should follow the selected service reference', () => {
    const fields = [
      { key: 'MONGODB_USERNAME', type: 'text' },
      { key: 'MONGODB_PASSWORD', type: 'password' },
      { key: 'RABBITMQ_USERNAME', type: 'text' },
      { key: 'CONFIG_SERVICE_PASSWORD', type: 'password' },
    ]

    expect(componentTemplateServiceUsernameFieldKeys(fields, 'mongodb')).toEqual(['MONGODB_USERNAME'])
    expect(componentTemplateServiceUsernameFieldKeys(fields, 'rabbitmq')).toEqual(['RABBITMQ_USERNAME'])
    expect(componentTemplateServiceUsernameFieldKeys(fields, 'mongodb')).not.toContain('MONGODB_PASSWORD')
  })
})
