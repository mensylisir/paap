export interface ComponentLike {
  id?: string | number
  name?: string
  type?: string
  image?: string
  version?: string
  replicas?: number
  cpu?: string
  memory?: string
  status?: string
  errorMessage?: string
  gitRepoUrl?: string
  gitPath?: string
  argocdApp?: string
  deliveryMode?: string
  sourceRepoUrl?: string
  sourceMirrorRepoUrl?: string
  sourceBranch?: string
  jenkinsJob?: string
  registryImage?: string
  pipelineStatus?: string
  config?: unknown
  events?: ComponentEventItem[]
}

export interface ComponentSummaryItem {
  label: string
  value: string
  mono?: boolean
}

export interface ComponentEventItem {
  time: string
  type: 'Normal' | 'Warning'
  reason: string
  message: string
}

export interface DeliveryChainStep {
  key: 'source' | 'image' | 'gitea-source' | 'ci' | 'registry' | 'gitea-deploy' | 'argocd' | 'cluster'
  label: string
  value: string
  detail: string
  status: 'ready' | 'running' | 'pending' | 'error'
  statusText: string
  mono?: boolean
}

export interface ComponentResourceTopologyNode {
  key: string
  kind: 'Component' | 'GitOps Path' | 'Deployment Manifest' | 'Service Manifest' | 'ArgoCD Application' | 'Cluster' | 'SecretKeyRef' | 'ConfigMapKeyRef' | 'Dependency'
  name: string
  status: 'ready' | 'pending'
  detail: string
  parentKey?: string
}

const empty = '-'

export function componentTypeText(type?: string) {
  return ({ frontend: '前端服务', backend: '后端服务', database: '数据库', middleware: '中间件', custom: '自定义' }[type || ''] || type || '未知')
}

export function componentStatusText(status?: string) {
  return ({ running: '运行中', syncing: '同步中', deploying: '部署中', building: '构建中', stopped: '已停止', pending: '等待中', error: '异常' }[status || ''] || status || '未知')
}

export function componentHealthDescription(component?: ComponentLike | null) {
  if (!component) return '等待组件数据加载。'
  if (component.status === 'running') return '组件运行正常，可以接收业务流量。'
  if (component.status === 'syncing') return '组件部署已提交，正在等待 GitOps 同步结果刷新。'
  if (component.status === 'pending') return '组件正在准备发布，请等待状态刷新。'
  if (component.status === 'error') return component.errorMessage || '组件发布或运行异常，请查看事件和发布配置。'
  if (component.status === 'stopped') return '组件当前已停止，不会接收业务流量。'
  return '组件状态未知，请刷新后查看。'
}

export function buildComponentSummary(component?: ComponentLike | null): ComponentSummaryItem[] {
  return [
    { label: '镜像', value: component?.image || empty, mono: true },
    { label: '版本', value: component?.version || empty, mono: true },
    { label: '实例规模', value: `${component?.replicas || 0} 个实例` },
    { label: '资源规格', value: `${component?.cpu || empty} / ${component?.memory || empty}` },
    { label: '代码路径', value: component?.gitPath || component?.gitRepoUrl || empty, mono: true },
    { label: '源码镜像仓', value: component?.sourceMirrorRepoUrl || empty, mono: true },
    { label: '发布应用', value: component?.argocdApp || empty, mono: true },
  ]
}

export function buildComponentEvents(component?: ComponentLike | null): ComponentEventItem[] {
  return component?.events || []
}

function deliveryStatusText(status: DeliveryChainStep['status']) {
  return ({ ready: '就绪', running: '进行中', pending: '待配置', error: '异常' }[status])
}

function pipelineStepStatus(component?: ComponentLike | null): DeliveryChainStep['status'] {
  const status = String(component?.pipelineStatus || '').toLowerCase()
  if (status === 'running' || status === 'building' || status === 'syncing') return 'running'
  if (status === 'error' || status === 'failed' || status === 'failure') return 'error'
  if (status === 'success' || status === 'ready' || component?.jenkinsJob) return 'ready'
  return 'pending'
}

function clusterStepStatus(status?: string): DeliveryChainStep['status'] {
  if (status === 'running') return 'ready'
  if (status === 'pending') return 'running'
  if (status === 'error') return 'error'
  return 'pending'
}

export function buildComponentDeliveryChain(component?: ComponentLike | null): DeliveryChainStep[] {
  const isSourceDelivery = component?.deliveryMode === 'source' || !!component?.sourceRepoUrl || !!component?.sourceMirrorRepoUrl || !!component?.jenkinsJob
  const ciStatus = pipelineStepStatus(component)
  const registryValue = component?.registryImage || component?.image || empty
  const clusterStatus = clusterStepStatus(component?.status)
  const deployManifestValue = component?.gitPath
    ? `${component.gitRepoUrl || 'Gitea'} · ${component.gitPath}`
    : component?.gitRepoUrl || empty

  const sourceSteps: DeliveryChainStep[] = [
    {
      key: 'source',
      label: 'Source',
      value: component?.sourceRepoUrl || empty,
      detail: component?.sourceBranch ? `源码分支 ${component.sourceBranch}` : '组件源码输入',
      status: component?.sourceRepoUrl ? 'ready' : 'pending',
      statusText: '',
      mono: true,
    },
    {
      key: 'gitea-source',
      label: 'Gitea Source Mirror',
      value: component?.sourceMirrorRepoUrl || empty,
      detail: '环境内源码镜像仓，Jenkins/kpack 从这里构建',
      status: component?.sourceMirrorRepoUrl ? 'ready' : 'pending',
      statusText: '',
      mono: true,
    },
    {
      key: 'ci',
      label: 'Jenkins/kpack',
      value: component?.jenkinsJob || '等待 Jenkins Job',
      detail: 'Jenkins 触发 kpack Buildpacks 构建并推送镜像',
      status: ciStatus,
      statusText: '',
      mono: true,
    },
    {
      key: 'registry',
      label: 'Registry',
      value: registryValue,
      detail: '构建产物推送到当前环境镜像仓库',
      status: registryValue === empty ? 'pending' : 'ready',
      statusText: '',
      mono: true,
    },
  ]

  const imageSteps: DeliveryChainStep[] = [
    {
      key: 'image',
      label: 'Image',
      value: component?.image || component?.registryImage || empty,
      detail: '直接使用已有镜像，不经过源码构建',
      status: component?.image || component?.registryImage ? 'ready' : 'pending',
      statusText: '',
      mono: true,
    },
  ]

  const deploySteps: DeliveryChainStep[] = [
    {
      key: 'gitea-deploy',
      label: 'Gitea Deploy YAML',
      value: deployManifestValue,
      detail: 'PAAP 生成并推送 Kubernetes 部署清单',
      status: deployManifestValue === empty ? 'pending' : 'ready',
      statusText: '',
      mono: true,
    },
    {
      key: 'argocd',
      label: 'ArgoCD',
      value: component?.argocdApp || empty,
      detail: '同步 GitOps 清单到目标命名空间',
      status: component?.argocdApp ? 'ready' : 'pending',
      statusText: '',
      mono: true,
    },
    {
      key: 'cluster',
      label: 'Cluster',
      value: componentStatusText(component?.status),
      detail: componentHealthDescription(component),
      status: clusterStatus,
      statusText: '',
    },
  ]

  const steps = isSourceDelivery ? [...sourceSteps, ...deploySteps] : [...imageSteps, ...deploySteps]
  return steps.map((step) => ({ ...step, statusText: deliveryStatusText(step.status) }))
}

function parseComponentConfig(config: unknown): { env?: any[]; dependencies?: string[] } {
  if (!config) return {}
  if (typeof config === 'object') return config as any
  try {
    return JSON.parse(String(config))
  } catch {
    return {}
  }
}

export function buildComponentResourceTopology(component?: ComponentLike | null): ComponentResourceTopologyNode[] {
  if (!component) return []
  const componentKey = `component:${component.id || component.name || 'current'}`
  const nodes: ComponentResourceTopologyNode[] = [{
    key: componentKey,
    kind: 'Component',
    name: component.name || '-',
    status: 'ready',
    detail: componentTypeText(component.type),
  }]

  if (component.gitPath || component.gitRepoUrl) {
    nodes.push({
      key: 'gitops-path',
      kind: 'GitOps Path',
      name: component.gitPath || component.gitRepoUrl || '-',
      status: 'ready',
      detail: 'Gitea 中的组件交付目录',
      parentKey: componentKey,
    })
  }

  nodes.push({
    key: 'deployment-manifest',
    kind: 'Deployment Manifest',
    name: component.gitPath ? `${component.gitPath}/deployment.yaml` : 'deployment.yaml',
    status: component.gitPath || component.image || component.registryImage ? 'ready' : 'pending',
    detail: '由 PAAP 生成并交给 ArgoCD 同步',
    parentKey: component.gitPath || component.gitRepoUrl ? 'gitops-path' : componentKey,
  })
  nodes.push({
    key: 'service-manifest',
    kind: 'Service Manifest',
    name: component.gitPath ? `${component.gitPath}/service.yaml` : 'service.yaml',
    status: component.gitPath ? 'ready' : 'pending',
    detail: '组件在环境 namespace 内的 ClusterIP 服务',
    parentKey: component.gitPath || component.gitRepoUrl ? 'gitops-path' : componentKey,
  })

  if (component.argocdApp) {
    nodes.push({
      key: 'argocd-app',
      kind: 'ArgoCD Application',
      name: component.argocdApp,
      status: 'ready',
      detail: '环境 AppProject 约束下的 GitOps 应用',
      parentKey: 'deployment-manifest',
    })
  }

  nodes.push({
    key: 'cluster',
    kind: 'Cluster',
    name: componentStatusText(component.status),
    status: component.status === 'running' ? 'ready' : 'pending',
    detail: componentHealthDescription(component),
    parentKey: component.argocdApp ? 'argocd-app' : 'deployment-manifest',
  })

  const cfg = parseComponentConfig(component.config)
  for (const item of cfg.env || []) {
    if (item?.secretName && item?.secretKey) {
      nodes.push({
        key: `secret:${item.secretName}:${item.secretKey}:${item.name || ''}`,
        kind: 'SecretKeyRef',
        name: `${item.secretName}/${item.secretKey}`,
        status: 'ready',
        detail: item.name ? `${item.name} 从 Secret 读取` : '从 Secret 读取环境变量',
        parentKey: 'deployment-manifest',
      })
    }
    if (item?.configMapName && item?.configMapKey) {
      nodes.push({
        key: `configmap:${item.configMapName}:${item.configMapKey}:${item.name || ''}`,
        kind: 'ConfigMapKeyRef',
        name: `${item.configMapName}/${item.configMapKey}`,
        status: 'ready',
        detail: item.name ? `${item.name} 从 ConfigMap 读取` : '从 ConfigMap 读取环境变量',
        parentKey: 'deployment-manifest',
      })
    }
  }
  for (const dep of cfg.dependencies || []) {
    const name = String(dep || '').trim()
    if (!name) continue
    nodes.push({
      key: `dependency:${name}`,
      kind: 'Dependency',
      name,
      status: 'ready',
      detail: '组件配置中显式声明的依赖',
      parentKey: componentKey,
    })
  }

  return nodes
}
