export type ComponentFramework =
  | 'auto'
  | 'unknown'
  | 'springboot'
  | 'node'
  | 'python'
  | 'go'
  | 'nginx'
  | 'custom'

export type ComponentCapabilityKey =
  | 'web-entry'
  | 'api-service'
  | 'database-client'
  | 'redis-client'
  | 'message-client'
  | 'object-storage-client'
  | 'runtime-env'
  | 'config-files'

export interface ComponentFrameworkOption {
  value: ComponentFramework
  label: string
}

export interface ComponentConfigSuggestion {
  key: string
  description: string
}

export interface ComponentConfigPreset {
  key: string
  label: string
  description: string
  keys: string[]
  framework?: ComponentFramework
}

export interface ComponentDiscoveredConfigKey {
  name: string
  source: string
  sensitive: boolean
  refKind?: 'configMap' | 'secret'
  refName?: string
  refKey?: string
  mountPath?: string
  asFile?: boolean
}

export interface ComponentProfile {
  declaredType: string
  framework: ComponentFramework
  capabilities: ComponentCapabilityKey[]
  capabilityLabels: string[]
  webEntry: boolean
  apiService: boolean
  dataWorkload: boolean
  middlewareWorkload: boolean
  hasRuntimeDependencies: boolean
  configSourceSummary: string
  discoveredConfigKeys: ComponentDiscoveredConfigKey[]
  corpus: string
}

export type ComponentDrawerTabKey =
  | 'deploy'
  | 'autoscaling'
  | 'capabilities'
  | 'api'
  | 'dependencies'
  | 'data'
  | 'variables'
  | 'runtime'
  | 'logs'
  | 'console'
  | 'settings'

export interface ComponentDrawerTab {
  key: ComponentDrawerTabKey
  label: string
}

export type ComponentDrawerMode =
  | 'web-entry'
  | 'api-service'
  | 'data-workload'
  | 'middleware-workload'
  | 'generic-runtime'

export interface ComponentDrawerBlueprint {
  mode: ComponentDrawerMode
  label: string
  configStrategyLabel: string
  tabs: ComponentDrawerTab[]
}

export interface ComponentProfileInput {
  component?: any
  form?: {
    framework?: string
    env?: any[]
    configMaps?: any[]
    secrets?: any[]
    files?: any[]
    bindings?: any[]
  }
}

const capabilityLabels: Record<ComponentCapabilityKey, string> = {
  'web-entry': 'Web 入口',
  'api-service': 'API 服务',
  'database-client': '数据库客户端',
  'redis-client': 'Redis 客户端',
  'message-client': '消息队列客户端',
  'object-storage-client': '对象存储客户端',
  'runtime-env': '运行态环境变量',
  'config-files': '配置文件/敏感配置',
}

export const componentFrameworkOptions: ComponentFrameworkOption[] = [
  { value: 'auto', label: '自动/未声明' },
  { value: 'springboot', label: 'Spring Boot' },
  { value: 'node', label: 'Node.js / NestJS' },
  { value: 'python', label: 'Django / FastAPI' },
  { value: 'go', label: 'Go' },
  { value: 'nginx', label: 'Nginx / 静态站点' },
  { value: 'custom', label: '自定义' },
]

export function componentFrameworkLabel(framework: string): string {
  return componentFrameworkOptions.find((item) => item.value === framework)?.label || framework
}

export function buildComponentProfile(input: ComponentProfileInput): ComponentProfile {
  const component = input.component || {}
  const cfg = parseComponentConfig(component?.config)
  const runtime = component?.runtimeConfig || {}
  const form = input.form || {}
  const mergedConfig = {
    ...cfg,
    env: form.env || cfg.env || [],
    configMaps: form.configMaps || cfg.configMaps || [],
    secrets: form.secrets || cfg.secrets || [],
    files: form.files || cfg.files || [],
    bindings: form.bindings || cfg.bindings || [],
    framework: form.framework || cfg.framework || '',
  }
  const declaredType = String(component?.type || 'custom').toLowerCase()
  const corpus = componentCapabilityCorpus({ ...component, config: mergedConfig, runtimeConfig: runtime })
  const framework = inferComponentFramework(mergedConfig.framework, corpus)
  const capabilities = detectComponentCapabilities({
    declaredType,
    corpus,
    runtime,
    config: mergedConfig,
    framework,
  })
  const capSet = new Set(capabilities)

  return {
    declaredType,
    framework,
    capabilities,
    capabilityLabels: capabilities.map((key) => capabilityLabels[key]),
    webEntry: declaredType === 'frontend' || capSet.has('web-entry'),
    apiService: declaredType === 'backend' || capSet.has('api-service'),
    dataWorkload: declaredType === 'database',
    middlewareWorkload: declaredType === 'middleware',
    hasRuntimeDependencies:
      capSet.has('database-client') ||
      capSet.has('redis-client') ||
      capSet.has('message-client') ||
      capSet.has('object-storage-client'),
    configSourceSummary: configSourceSummary(mergedConfig, runtime),
    discoveredConfigKeys: collectComponentConfigKeys(mergedConfig, runtime),
    corpus,
  }
}

export function componentConfigKeySuggestions(profile: ComponentProfile): ComponentConfigSuggestion[] {
  const keys: ComponentConfigSuggestion[] = []
  const add = (key: string, description: string) => {
    if (!keys.some((item) => item.key === key)) keys.push({ key, description })
  }
  const caps = new Set(profile.capabilities)

  if (profile.framework === 'unknown' || profile.framework === 'custom' || profile.capabilities.length === 0) {
    add('PORT', '应用监听端口')
    add('LOG_LEVEL', '日志级别')
  }
  if (profile.framework === 'node') {
    add('NODE_ENV', 'Node.js 运行环境')
    add('PORT', 'HTTP 监听端口')
  }
  if (profile.framework === 'python' || profile.framework === 'go') {
    add('APP_ENV', '应用运行环境')
    add('PORT', 'HTTP 监听端口')
  }
  if (profile.webEntry) {
    add('BACKEND_URL', '后端服务地址')
    add('API_BASE_URL', '通用 API 地址')
    add('VITE_API_BASE_URL', 'Vite 前端 API 地址')
    add('NEXT_PUBLIC_API_URL', 'Next.js 前端 API 地址')
  }
  if (profile.apiService || profile.hasRuntimeDependencies || profile.framework === 'springboot') {
    add('SERVER_PORT', '应用监听端口')
    add('SPRING_PROFILES_ACTIVE', 'Spring Boot Profile')
  }
  if (caps.has('database-client')) {
    add('POSTGRES_HOST', 'PostgreSQL 主机')
    add('POSTGRES_PORT', 'PostgreSQL 端口')
    add('POSTGRES_USERNAME', 'PostgreSQL 用户名')
    add('POSTGRES_PASSWORD', 'PostgreSQL 密码')
    add('POSTGRES_DATABASE', 'PostgreSQL 数据库')
    add('MYSQL_HOST', 'MySQL 主机')
    add('MYSQL_PASSWORD', 'MySQL 密码')
  }
  if (caps.has('redis-client')) {
    add('REDIS_HOST', 'Redis 主机')
    add('REDIS_PORT', 'Redis 端口')
    add('REDIS_PASSWORD', 'Redis 密码')
    add('REDIS_SENTINEL_HOST', 'Redis Sentinel 主机')
    add('REDIS_SENTINEL_PORT', 'Redis Sentinel 端口')
    add('REDIS_SENTINEL_MASTER_NAME', 'Redis Sentinel master set')
    add('REDIS_CLUSTER_HOST', 'Redis Cluster 服务名')
    add('REDIS_CLUSTER_PORT', 'Redis Cluster 端口')
    add('REDIS_CLUSTER_NODES', 'Redis Cluster startup nodes')
  }
  if (caps.has('message-client')) {
    add('RABBITMQ_HOST', 'RabbitMQ 主机')
    add('RABBITMQ_PASSWORD', 'RabbitMQ 密码')
    add('KAFKA_BOOTSTRAP_SERVERS', 'Kafka Bootstrap Servers')
  }
  if (caps.has('object-storage-client')) {
    add('S3_ENDPOINT', 'S3 Endpoint')
    add('S3_BUCKET', 'S3 Bucket')
    add('MINIO_ACCESS_KEY', 'MinIO Access Key')
    add('MINIO_SECRET_KEY', 'MinIO Secret Key')
  }
  return keys
}

export function componentConfigPresets(profile: ComponentProfile): ComponentConfigPreset[] {
  const presets: ComponentConfigPreset[] = []
  const add = (preset: ComponentConfigPreset) => {
    if (!presets.some((item) => item.key === preset.key)) presets.push(preset)
  }
  const caps = new Set(profile.capabilities)

  if (profile.webEntry) {
    add({
      key: 'frontend-api',
      label: '前端 API 地址',
      description: '面向静态前端和 SSR 前端的后端入口变量。',
      keys: ['BACKEND_URL', 'API_BASE_URL', 'VITE_API_BASE_URL', 'NEXT_PUBLIC_API_URL'],
    })
  }
  if (profile.webEntry && (profile.framework === 'nginx' || profile.framework === 'unknown' || profile.framework === 'custom')) {
    add({
      key: 'nginx-api-proxy',
      label: 'Nginx 代理路由',
      description: '生成 default.conf，由用户填写匹配路径并选择或输入转发地址。',
      keys: ['default.conf', 'LOCATION_LIST'],
      framework: 'nginx',
    })
  }
  if (profile.framework === 'springboot') {
    add({
      key: 'springboot-runtime',
      label: 'Spring Boot 运行配置',
      description: 'Profile、端口和外部配置文件入口。',
      keys: ['SPRING_PROFILES_ACTIVE', 'SERVER_PORT', 'SPRING_CONFIG_ADDITIONAL_LOCATION'],
      framework: 'springboot',
    })
  }
  if (profile.framework === 'node') {
    add({
      key: 'node-runtime',
      label: 'Node.js 运行配置',
      description: 'Node/Nest/Next/Vite 常用运行变量。',
      keys: ['NODE_ENV', 'PORT', 'API_BASE_URL'],
      framework: 'node',
    })
  }
  if (profile.framework === 'python') {
    add({
      key: 'python-runtime',
      label: 'Python Web 运行配置',
      description: 'Django/FastAPI 常用运行变量。',
      keys: ['APP_ENV', 'PORT', 'DATABASE_URL', 'REDIS_URL'],
      framework: 'python',
    })
  }
  if (profile.framework === 'go') {
    add({
      key: 'go-runtime',
      label: 'Go 服务运行配置',
      description: 'Go API 服务常用运行变量。',
      keys: ['APP_ENV', 'PORT', 'DATABASE_URL', 'REDIS_URL'],
      framework: 'go',
    })
  }
  if (caps.has('database-client')) {
    add({
      key: 'database-client',
      label: '数据库连接',
      description: 'PostgreSQL/MySQL 客户端常用变量。',
      keys: ['POSTGRES_HOST', 'POSTGRES_PORT', 'POSTGRES_USERNAME', 'POSTGRES_PASSWORD', 'POSTGRES_DATABASE', 'MYSQL_HOST', 'MYSQL_PASSWORD'],
    })
  }
  if (caps.has('redis-client')) {
    add({
      key: 'redis-client',
      label: 'Redis 连接',
      description: 'Redis 客户端常用变量。',
      keys: ['REDIS_HOST', 'REDIS_PORT', 'REDIS_PASSWORD', 'REDIS_URL', 'REDIS_SENTINEL_HOST', 'REDIS_SENTINEL_PORT', 'REDIS_SENTINEL_MASTER_NAME', 'REDIS_CLUSTER_HOST', 'REDIS_CLUSTER_PORT', 'REDIS_CLUSTER_NODES'],
    })
  }
  if (caps.has('message-client')) {
    add({
      key: 'message-client',
      label: '消息队列连接',
      description: 'RabbitMQ/Kafka 客户端常用变量。',
      keys: ['RABBITMQ_HOST', 'RABBITMQ_PORT', 'RABBITMQ_USERNAME', 'RABBITMQ_PASSWORD', 'RABBITMQ_URL', 'KAFKA_BROKERS'],
    })
  }
  if (caps.has('object-storage-client')) {
    add({
      key: 'object-storage-client',
      label: '对象存储连接',
      description: 'S3/MinIO 客户端常用变量。',
      keys: ['MINIO_ENDPOINT', 'MINIO_ACCESS_KEY', 'MINIO_SECRET_KEY', 'S3_BUCKET'],
    })
  }
  if (!presets.length) {
    add({
      key: 'generic-runtime',
      label: '通用运行配置',
      description: '未知组件的最小运行契约。',
      keys: ['PORT', 'LOG_LEVEL'],
    })
  }
  return presets
}

export function componentDrawerBlueprint(profile: ComponentProfile): ComponentDrawerBlueprint {
  const tabs: ComponentDrawerTab[] = [
    { key: 'deploy', label: '部署' },
    { key: 'autoscaling', label: '伸缩' },
    { key: 'variables', label: '配置' },
    { key: 'runtime', label: '指标' },
    { key: 'logs', label: '日志' },
    { key: 'console', label: '控制台' },
  ]

  if (profile.webEntry) {
    return {
      mode: 'web-entry',
      label: 'Web 入口组件',
      configStrategyLabel: '代理路由 + 静态运行变量',
      tabs,
    }
  }

  if (profile.apiService || profile.hasRuntimeDependencies) {
    return {
      mode: 'api-service',
      label: 'API/运行服务组件',
      configStrategyLabel: '运行依赖 + 框架配置 + Secret 引用',
      tabs,
    }
  }

  if (profile.dataWorkload || profile.middlewareWorkload) {
    return {
      mode: profile.dataWorkload ? 'data-workload' : 'middleware-workload',
      label: profile.dataWorkload ? '纳管数据库工作负载' : '纳管中间件工作负载',
      configStrategyLabel: '运行态资源只读 + 通用部署参数',
      tabs,
    }
  }

  return {
    mode: 'generic-runtime',
    label: '自定义工作负载',
    configStrategyLabel: '通用部署参数 + 发现到的配置键',
    tabs,
  }
}

export function componentCapabilityCorpus(component: any): string {
  const cfg = parseComponentConfig(component?.config)
  const runtime = component?.runtimeConfig || {}
  return [
    component?.name,
    component?.type,
    component?.image,
    component?.registryImage,
    component?.sourceRepoUrl,
    component?.sourceMirrorRepoUrl,
    component?.gitRepoUrl,
    component?.gitPath,
    cfg.framework,
    ...(cfg.env || []).flatMap((item: any) => [item.name, item.value, item.configMapName, item.configMapKey, item.secretName, item.secretKey]),
    ...(cfg.configMaps || []).flatMap((item: any) => [item.name, ...Object.keys(item.data || {}), ...Object.values(item.data || {})]),
    ...(cfg.secrets || []).flatMap((item: any) => [item.name, ...Object.keys(item.data || {})]),
    ...(cfg.files || []).flatMap((item: any) => [item.name, item.configMapName, item.key, item.mountPath]),
    ...(cfg.bindings || []).flatMap((item: any) => [item.targetName, item.targetType, item.role, ...Object.keys(item.generated || {})]),
    ...(cfg.dependencies || []),
    ...(runtime.env || []).flatMap((item: any) => [item.name, item.value, item.configMapName, item.configMapKey, item.secretName, item.secretKey]),
    ...(runtime.envFrom || []).flatMap((item: any) => [item.kind, item.name]),
    ...(runtime.configMaps || []).flatMap((item: any) => [item.name, ...(item.keys || [])]),
    ...(runtime.secrets || []).flatMap((item: any) => [item.name, ...(item.keys || [])]),
    ...(runtime.files || []).flatMap((item: any) => [item.kind, item.objectName, item.key, item.path, item.mountPath]),
    runtime.workloadName,
    runtime.serviceName,
  ].filter(Boolean).join(' ').toLowerCase()
}

function inferComponentFramework(explicitFramework: unknown, corpus: string): ComponentFramework {
  const declared = String(explicitFramework || '').trim().toLowerCase()
  if (declared && declared !== 'auto') return normalizeFramework(declared)
  if (containsAny(corpus, ['spring', 'java', 'jdbc', 'application.yml', 'application.properties'])) return 'springboot'
  if (containsAny(corpus, ['nginx', 'default.conf', 'static', 'index.html'])) return 'nginx'
  if (containsAny(corpus, ['node', 'nestjs', 'next', 'react', 'vue', 'vite', 'npm', 'yarn', 'pnpm'])) return 'node'
  if (containsAny(corpus, ['fastapi', 'django', 'python', 'uvicorn', 'gunicorn'])) return 'python'
  if (containsAny(corpus, ['gin', 'golang', '/go', ' go '])) return 'go'
  return 'unknown'
}

function normalizeFramework(value: string): ComponentFramework {
  if (value.includes('spring')) return 'springboot'
  if (value.includes('node') || value.includes('next') || value.includes('nest')) return 'node'
  if (value.includes('python') || value.includes('django') || value.includes('fastapi')) return 'python'
  if (value.includes('golang') || value === 'go') return 'go'
  if (value.includes('nginx') || value.includes('static')) return 'nginx'
  if (value === 'custom') return 'custom'
  return 'unknown'
}

function detectComponentCapabilities(input: {
  declaredType: string
  corpus: string
  runtime: any
  config: any
  framework: ComponentFramework
}): ComponentCapabilityKey[] {
  const caps: ComponentCapabilityKey[] = []
  const add = (key: ComponentCapabilityKey) => {
    if (!caps.includes(key)) caps.push(key)
  }
  const bindingTypes = new Set<string>(
    (input.config.bindings || [])
      .map((item: any) => String(item?.targetType || '').toLowerCase())
      .filter((item: string) => Boolean(item))
  )
  const configKeys = [
    ...(input.config.env || []).map((item: any) => item.name),
    ...(input.config.configMaps || []).flatMap((item: any) => Object.keys(item.data || {})),
    ...(input.config.secrets || []).flatMap((item: any) => Object.keys(item.data || {})),
    ...(input.config.files || []).flatMap((item: any) => [item.key, item.mountPath]),
    ...(input.runtime.env || []).map((item: any) => item.name || item.value),
    ...(input.runtime.configMaps || []).flatMap((item: any) => item.keys || []),
    ...(input.runtime.secrets || []).flatMap((item: any) => item.keys || []),
    ...(input.runtime.files || []).flatMap((item: any) => [item.key, item.mountPath]),
  ]
  const envNames = configKeys.map((item: any) => String(item || '').toUpperCase())

  const looksLikeWeb =
    input.declaredType === 'frontend' ||
    input.framework === 'nginx' ||
    containsAny(input.corpus, ['frontend', 'web', 'nginx', 'react', 'vue', 'vite', 'next_public', 'index.html'])
  const looksLikeApi =
    input.declaredType === 'backend' ||
    (!looksLikeWeb && (
      containsAny(input.corpus, ['backend', 'server', 'fastapi', 'spring', 'nestjs', 'django', 'gin']) ||
      containsSeparatedToken(input.corpus, 'api')
    ))

  if (looksLikeWeb) add('web-entry')
  if (looksLikeApi) add('api-service')
  const mentionsDatabase =
    hasBinding(bindingTypes, ['postgresql', 'mysql', 'mongodb']) ||
    envNames.some((name) => containsAny(name, ['POSTGRES', 'MYSQL', 'MONGO', 'JDBC', 'DATABASE', 'DATASOURCE'])) ||
    containsAny(input.corpus, ['postgresql', 'postgres://', 'mysql://', 'mongodb://', 'jdbc:', 'spring.datasource', 'datasource', 'hibernate', 'jpa'])
  const mentionsRedis =
    bindingTypes.has('redis') ||
    envNames.some((name) => name.includes('REDIS')) ||
    containsAny(input.corpus, ['redis://', 'redis:', 'spring.data.redis', 'spring.redis', 'redisson', 'lettuce', 'jedis'])
  const mentionsMessageQueue =
    hasBinding(bindingTypes, ['rabbitmq', 'kafka']) ||
    envNames.some((name) => containsAny(name, ['RABBIT', 'KAFKA', 'MQ', 'AMQP'])) ||
    containsAny(input.corpus, ['amqp://', 'rabbitmq', 'spring.rabbitmq', 'kafka', 'bootstrap.servers', 'spring.kafka'])
  const mentionsObjectStorage =
    bindingTypes.has('minio') ||
    envNames.some((name) => containsAny(name, ['S3', 'MINIO', 'OBJECT_STORAGE'])) ||
    containsAny(input.corpus, ['s3://', 'minio', 'aws.s3', 'object-storage'])

  if (mentionsDatabase) add('database-client')
  if (mentionsRedis) add('redis-client')
  if (mentionsMessageQueue) add('message-client')
  if (mentionsObjectStorage) add('object-storage-client')
  if (Array.isArray(input.runtime.env) && input.runtime.env.length) add('runtime-env')
  if (
    (input.config.configMaps || []).length ||
    (input.config.secrets || []).length ||
    (input.config.files || []).length ||
    (input.runtime.configMaps || []).length ||
    (input.runtime.secrets || []).length ||
    (input.runtime.envFrom || []).length ||
    (input.runtime.files || []).length
  ) add('config-files')

  return caps
}

function configSourceSummary(config: any, runtime: any = {}): string {
  const parts: string[] = []
  const envCount = (config.env || []).length + (runtime.env || []).length
  const configMapCount = (config.configMaps || []).length + (runtime.configMaps || []).length
  const secretCount = (config.secrets || []).length + (runtime.secrets || []).length
  const fileCount = (config.files || []).length + (runtime.files || []).length
  if (envCount) parts.push(`${envCount} 运行参数`)
  if (configMapCount) parts.push(`${configMapCount} 普通配置`)
  if (secretCount) parts.push(`${secretCount} 敏感配置`)
  if (fileCount) parts.push(`${fileCount} 配置文件`)
  return parts.join(' / ') || '未声明'
}

function collectComponentConfigKeys(config: any, runtime: any): ComponentDiscoveredConfigKey[] {
  const keys: ComponentDiscoveredConfigKey[] = []
  const seen = new Set<string>()
  const add = (item: ComponentDiscoveredConfigKey) => {
    const name = String(item.name || '').trim()
    if (!name) return
    const source = String(item.source || '').trim() || '配置'
    const semanticKey = [
      name,
      item.refKind || '',
      item.refName || '',
      item.refKey || '',
      item.mountPath || '',
      item.asFile ? 'file' : 'value',
    ].join(':')
    const dedupeKey = item.refKind || item.refName || item.refKey || item.mountPath ? semanticKey : `${name}:${source}`
    if (seen.has(dedupeKey)) return
    seen.add(dedupeKey)
    keys.push({ ...item, name, source, sensitive: Boolean(item.sensitive) || configKeyLooksSensitive(name) })
  }

  for (const item of [...(config.env || []), ...(runtime.env || [])]) {
    const name = String(item?.name || '').trim()
    if (!name) continue
    if (item?.secretName || item?.secretKey) {
      add({
        name,
        source: '敏感配置',
        sensitive: true,
        refKind: 'secret',
        refName: item.secretName,
        refKey: item.secretKey || name,
      })
    } else if (item?.configMapName || item?.configMapKey) {
      add({
        name,
        source: '普通配置',
        sensitive: false,
        refKind: 'configMap',
        refName: item.configMapName,
        refKey: item.configMapKey || name,
      })
    } else {
      add({ name, source: '环境变量', sensitive: configKeyLooksSensitive(name) })
    }
  }

  for (const item of config.configMaps || []) {
    for (const key of Object.keys(item?.data || {})) {
      add({
        name: key,
        source: '普通配置',
        sensitive: false,
        refKind: 'configMap',
        refName: item.name,
        refKey: key,
      })
    }
  }
  for (const item of config.secrets || []) {
    for (const key of Object.keys(item?.data || {})) {
      add({
        name: key,
        source: '敏感配置',
        sensitive: true,
        refKind: 'secret',
        refName: item.name,
        refKey: key,
      })
    }
  }
  for (const item of config.files || []) {
    const key = String(item?.key || '').trim()
    if (!key) continue
    add({
      name: key,
      source: '配置文件',
      sensitive: false,
      refKind: 'configMap',
      refName: item.configMapName,
      refKey: key,
      mountPath: item.mountPath,
      asFile: true,
    })
  }
  for (const item of runtime.configMaps || []) {
    for (const key of item?.keys || []) {
      add({
        name: key,
        source: '普通配置',
        sensitive: false,
        refKind: 'configMap',
        refName: item.name,
        refKey: key,
      })
    }
  }
  for (const item of runtime.secrets || []) {
    for (const key of item?.keys || []) {
      add({
        name: key,
        source: '敏感配置',
        sensitive: true,
        refKind: 'secret',
        refName: item.name,
        refKey: key,
      })
    }
  }
  for (const item of runtime.files || []) {
    const key = String(item?.key || '').trim()
    if (!key) continue
    const kind: 'configMap' | 'secret' = String(item?.kind || '').toLowerCase() === 'secret' ? 'secret' : 'configMap'
    const objectName = item?.objectName || item?.name
    add({
      name: key,
      source: kind === 'secret' ? '敏感配置文件' : '配置文件',
      sensitive: kind === 'secret' || configKeyLooksSensitive(key),
      refKind: kind,
      refName: objectName,
      refKey: key,
      mountPath: item.mountPath,
      asFile: true,
    })
  }

  return keys.slice(0, 40)
}

function configKeyLooksSensitive(key: string): boolean {
  const upper = String(key || '').toUpperCase()
  return upper.includes('PASSWORD') || upper.includes('SECRET') || upper.includes('TOKEN') || upper.includes('ACCESS_KEY')
}

function parseComponentConfig(config: unknown): any {
  if (!config) return {}
  if (typeof config === 'object') return config
  if (typeof config !== 'string') return {}
  const trimmed = config.trim()
  if (!trimmed || trimmed === '{}') return {}
  try {
    return JSON.parse(trimmed)
  } catch {
    return {}
  }
}

function containsAny(text: string, needles: string[]): boolean {
  return needles.some((needle) => text.includes(needle))
}

function hasBinding(bindings: Set<string>, types: string[]): boolean {
  return types.some((type) => bindings.has(type))
}

function containsSeparatedToken(text: string, token: string): boolean {
  const escaped = token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`(^|[\\s./-])${escaped}($|[\\s./-])`).test(text)
}
