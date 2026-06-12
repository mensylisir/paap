/// <reference types="node" />

import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

describe('Vue view markup', () => {
  it('does not contain nested style tags inside a style block', () => {
    const viewSources = import.meta.glob('./*.vue', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    const offenders = Object.entries(viewSources).flatMap(([file, content]) => {
      const styleBlocks = content.match(/<style\b[^>]*>[\s\S]*?<\/style>/g) || []
      const hasNestedStyle = styleBlocks.some((block) => {
        const body = block.replace(/^<style\b[^>]*>/, '').replace(/<\/style>$/, '')
        return /<style\b/i.test(body)
      })
      return hasNestedStyle ? [file.replace('./', '')] : []
    })

    expect(offenders).toEqual([])
  })

  it('makes environment overview the first tab instead of using tools as the landing view', () => {
    const viewSources = import.meta.glob('./*.vue', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    const envDetail = viewSources['./EnvDetailView.vue']

    expect(envDetail).toContain("initialEnvTab")
    expect(envDetail).toContain("key: 'overview'")
    expect(envDetail).toContain("v-if=\"activeTab === 'overview'\"")
    expect(envDetail).toContain('交付流程')
    expect(envDetail).toContain('核心工具')
    expect(envDetail).toContain('环境关注项')
    expect(envDetail).toContain('deliverySteps')
    expect(envDetail).toContain('criticalTools')
    expect(envDetail).toContain('unhealthyServices')
    expect(envDetail).toContain('@click="scrollToCoreTools"')
    expect(envDetail).toContain('ref="coreToolsSection"')
    expect(envDetail).toContain('const scrollToCoreTools')
    expect(envDetail).not.toContain('<button type="button" class="overview-stat" @click="setFirstCapabilityTab(\'tool\')">')
  })

  it('keeps empty environments on the canvas instead of routing users to an old environment page', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const appOverview = await import('./AppOverviewView.vue?raw')

    expect(envDetail.default).toContain('environmentCanvasNodes.length === 0')
    expect(envDetail.default).toContain('在画布空白处右键创建组件、工具、数据库或中间件。')
    expect(envDetail.default).not.toContain('v-if="environmentTopologyAllNodes.length"')
    expect(envDetail.default).not.toContain('当前环境还没有业务组件，先创建组件再进入源码/镜像到集群的交付流程。')

    expect(appOverview.default).toContain('openCreateEnvironmentModal')
    expect(appOverview.default).toContain('submitEnvironment')
    expect(appOverview.default).toContain('api.createEnv')
    expect(appOverview.default).not.toContain('environments?create=true')
  })

  it('runs embedded workspace form actions in the environment page instead of opening the old service detail page', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('activeCapabilityAction')
    expect(envDetail.default).toContain('submitCapabilityActionDialog')
    expect(envDetail.default).toContain('validateWorkspaceActionParams')
    expect(envDetail.default).toContain('api.runServiceWorkspaceAction(envId.value, svc.id, action.key, target, params)')
    expect(envDetail.default).not.toContain('/services/${svc.id}?tab=workspace')
  })

  it('keeps global navigation focused on applications and templates', async () => {
    const mainLayout = await import('../layouts/MainLayout.vue?raw')

    expect(mainLayout.default).toContain('<span>应用</span>')
    expect(mainLayout.default).toContain('<span>模板</span>')
    expect(mainLayout.default).not.toContain('<span>仓库</span>')
    expect(mainLayout.default).not.toContain('to="/registries"')
  })

  it('uses Railway-like app and environment context switching with modal creation', async () => {
    const appList = await import('./AppListView.vue?raw')
    const environmentsView = await import('./AppEnvironmentsView.vue?raw')
    const appLayout = await import('../layouts/AppLayout.vue?raw')

    expect(appList.default).toContain('showCreateAppModal')
    expect(appList.default).toContain('submitApp')
    expect(appList.default).toContain('goToDefaultWorkspace')
    expect(appList.default).toContain('创建应用')
    expect(appList.default).not.toContain("router.push('/apps/create')")
    expect(appList.default).not.toContain('environments?create=true')
    expect(appList.default).toContain('overview?createEnvironment=true')
    expect(environmentsView.default).toContain('autoOpenCreateEnvironment')
    expect(environmentsView.default).toContain('goToDefaultEnvironment')
    expect(environmentsView.default).toContain("mode: 'empty' as string")
    expect(environmentsView.default).toContain("mode: 'empty'")
    expect(appLayout.default).toContain('context-bar')
    expect(appLayout.default.indexOf('<main class="main">')).toBeLessThan(appLayout.default.indexOf('context-bar'))
    expect(appLayout.default).toContain('workspace-switcher')
    expect(appLayout.default).toContain('context-switcher-button app-context-switcher')
    expect(appLayout.default).toContain('context-switcher-button env-context-switcher')
    expect(appLayout.default).toContain('context-popover')
    expect(appLayout.default).toContain('toggleAppSwitcher')
    expect(appLayout.default).toContain('toggleEnvSwitcher')
    expect(appLayout.default).toContain('goToSelectedApp')
    expect(appLayout.default).toContain('goToSelectedEnv')
    expect(appLayout.default).not.toContain('app-context-select')
    expect(appLayout.default).not.toContain('env-context-select')
    expect(appList.default).toContain('@click="goToAppHome(app)"')
    expect(appList.default).toContain("router.push(`/apps/${app.id}/overview`)")
    expect(appLayout.default).toContain("router.push(`/apps/${nextAppId}/overview`)")
  })

  it('exposes delete actions from application and environment list cards without navigating into cards', async () => {
    const appList = await import('./AppListView.vue?raw')
    const environmentsView = await import('./AppEnvironmentsView.vue?raw')
    const appOverview = await import('./AppOverviewView.vue?raw')

    expect(appList.default).toContain('@click.stop="openDeleteApplicationDialog(app)"')
    expect(appList.default).toContain('删除应用')
    expect(appList.default).toContain('pendingDeleteApp')
    expect(appList.default).toContain('performDeleteApplication')
    expect(appList.default).toContain('api.deleteApp')
    expect(appList.default).toContain('await loadApps()')
    expect(environmentsView.default).toContain('@click.stop="openDeleteEnvironmentDialog(env)"')
    expect(environmentsView.default).toContain('env-delete-btn')
    expect(environmentsView.default).toContain('删除环境')
    expect(environmentsView.default).toContain('pendingDeleteEnv')
    expect(environmentsView.default).toContain('performDeleteEnvironment')
    expect(environmentsView.default).toContain('api.deleteEnv')
    expect(environmentsView.default).toContain('await loadEnvs()')
    expect(appOverview.default).toContain('@click.stop="openDeleteEnvironmentDialog(env)"')
    expect(appOverview.default).toContain('删除环境')
    expect(appOverview.default).toContain('pendingDeleteEnv')
    expect(appOverview.default).toContain('performDeleteEnvironment')
    expect(appOverview.default).toContain('api.deleteEnv')
  })

  it('does not expose application or environment deletion inside the environment detail status summary', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).not.toContain('overview-danger-actions')
    expect(envDetail.default).not.toContain('@click="deleteCurrentEnvironment"')
    expect(envDetail.default).not.toContain('@click="deleteCurrentApplication"')
  })

  it('does not show deploy ci and monitor as app-level navigation entries', async () => {
    const appLayout = await import('../layouts/AppLayout.vue?raw')

    expect(appLayout.default).toContain('<span>概览</span>')
    expect(appLayout.default).toContain('<span>环境</span>')
    expect(appLayout.default).not.toContain('<span>部署</span>')
    expect(appLayout.default).not.toContain('<span>CI</span>')
    expect(appLayout.default).not.toContain('<span>监控</span>')
    expect(appLayout.default).not.toContain("active === 'deploy'")
    expect(appLayout.default).not.toContain("active === 'ci'")
    expect(appLayout.default).not.toContain("active === 'monitor'")
  })

  it('uses a wide working canvas for dense operational pages', () => {
    const globalStyles = readFileSync(new URL('../style.scss', import.meta.url), 'utf8')
    const mainEntry = readFileSync(new URL('../main.ts', import.meta.url), 'utf8')

    expect(mainEntry).toContain("import './styles/carbon-theme.css'")
    expect(globalStyles).toContain('--paap-content-max: 9999px')
    expect(globalStyles).toContain('--paap-content-gutter: 20px')
    expect(globalStyles).toContain('width: 100%')
    expect(globalStyles).toContain('padding: var(--paap-space-5)')
  })

  it('renders environment service tabs as installed capability areas and persists tab query state', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const appLayout = await import('../layouts/AppLayout.vue?raw')
    const envCapabilities = await import('./envCapabilities.ts?raw')

    expect(envDetail.default).toContain('capabilityTabs')
    expect(envDetail.default).toContain('serviceCapability')
    expect(envDetail.default).toContain('environment-shell')
    expect(envDetail.default).not.toContain('env-side-nav')
    expect(envDetail.default).not.toContain('capability-menu')
    expect(appLayout.default).toContain('环境菜单')
    expect(appLayout.default).toContain('envMenuItems')
    expect(appLayout.default).toContain('activeEnvMenuKey')
    expect(appLayout.default).toContain('paap-env-updated')
    expect(envCapabilities.default).toContain('代码仓库')
    expect(envCapabilities.default).toContain('镜像仓库')
    expect(envCapabilities.default).toContain('持续集成')
    expect(envCapabilities.default).toContain('持续部署')
    expect(envCapabilities.default).toContain('监控中心')
    expect(envCapabilities.default).toContain('日志中心')
    expect(envDetail.default).toContain('replaceEnvTab')
    expect(envDetail.default).toContain('route.query.tab')
    expect(envDetail.default).not.toContain('<div class="tab-bar">')
    expect(envDetail.default).toContain('activeCapabilityWorkspace')
    expect(envDetail.default).toContain('workspaceComponentForService')
    expect(envDetail.default).toContain('loadCapabilityWorkspace')
    expect(envDetail.default).toContain(':is="workspaceComponentForService(activeCapabilityService)"')
    expect(envDetail.default).not.toContain('class="service-row" @click="goToService(svc.id)"')
  })

  it('renders empty environments as 空环境 and reads wrapped environment lists', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const appLayout = await import('../layouts/AppLayout.vue?raw')

    expect(envDetail.default).toContain('envStatusText')
    expect(envDetail.default).toContain("if (!s) return '空环境'")
    expect(envDetail.default).toContain('{{ envStatusText(env.status) }}')
    expect(envDetail.default).not.toContain(": env.status }}")
    expect(appLayout.default).toContain('normalizeListPayload(envsRes.data)')
    expect(appLayout.default).toContain('normalizeListPayload(res.data)')
  })

  it('renders real external access entries from environment details', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('应用访问')
    expect(envDetail.default).toContain('externalAccess')
    expect(envDetail.default).toContain('res.data.externalAccess')
    expect(envDetail.default).toContain('externalAccessGroups')
    expect(envDetail.default).toContain("item?.scope !== 'service'")
    expect(envDetail.default).toContain(':href="item.url"')
    expect(envDetail.default).not.toContain('工具与中间件入口')
  })

  it('uses canvas creation wording for new application resources and canvas tools', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('创建组件')
    expect(envDetail.default).toContain('安装工具')
    expect(envDetail.default).toContain('添加中间件')
    expect(envDetail.default).toContain('createCanvasComponentDraft')
    expect(envDetail.default).toContain('installCanvasService')
    expect(envDetail.default).toContain('接入已有资源')
  })

  it('removes the old service detail page entry and keeps service workspaces inside environment tabs', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const router = await import('../router/index.ts?raw')

    expect(router.default).not.toContain('ServiceInstanceDetailView')
    expect(router.default).not.toContain('environments/:envId/services/:serviceId')
    expect(envDetail.default).toContain('openServiceWorkspace')
    expect(envDetail.default).not.toContain('goToService(')
    expect(envDetail.default).not.toContain('/services/${svc.id}')
  })

  it('groups environment capabilities under tool and middleware sidebar sections', async () => {
    const appLayout = await import('../layouts/AppLayout.vue?raw')

    expect(appLayout.default).toContain('env-menu-section')
    expect(appLayout.default).toContain('env-menu-group-toggle')
    expect(appLayout.default).toContain('@click="toggleEnvMenuGroup(\'tool\')"')
    expect(appLayout.default).toContain('@click="toggleEnvMenuGroup(\'infra\')"')
    expect(appLayout.default).toContain('openEnvMenuGroups.tool')
    expect(appLayout.default).toContain('openEnvMenuGroups.infra')
    expect(appLayout.default).toContain('toolEnvMenuItems')
    expect(appLayout.default).toContain('infraEnvMenuItems')
    expect(appLayout.default).toContain('工具')
    expect(appLayout.default).toContain('中间件')
    expect(appLayout.default).toContain('v-if="toolEnvMenuItems.length"')
    expect(appLayout.default).toContain('v-if="infraEnvMenuItems.length"')
    expect(appLayout.default).toContain('v-show="openEnvMenuGroups.tool"')
    expect(appLayout.default).toContain('v-show="openEnvMenuGroups.infra"')
    expect(appLayout.default).not.toContain('v-for="item in envMenuItems"')
  })

  it('opens component repositories through the environment service proxy instead of cluster DNS', async () => {
    const componentDetail = await import('./ComponentDetailView.vue?raw')

    expect(componentDetail.default).toContain('serviceProxyUrl')
    expect(componentDetail.default).toContain('gitServiceId')
    expect(componentDetail.default).toContain('componentRepoUrl')
    expect(componentDetail.default).toContain('repoProxyPath')
    expect(componentDetail.default).toContain(':href="componentRepoUrl"')
    expect(componentDetail.default).not.toContain(':href="component.gitRepoUrl"')
  })

  it('keeps component detail back navigation and opens ArgoCD topology in the environment deployment tab', async () => {
    const componentDetail = await import('./ComponentDetailView.vue?raw')

    expect(componentDetail.default).toContain('router.back()')
    expect(componentDetail.default).toContain('tab=continuous-deployment')
    expect(componentDetail.default).not.toContain('services/${deployServiceId.value}')
  })

  it('tells registry users to hand self-signed CA certificates to cluster administrators', async () => {
    const registryWorkspace = await import('../components/workspaces/RegistryWorkspace.vue?raw')

    expect(registryWorkspace.default).toContain('内网自签 CA')
    expect(registryWorkspace.default).toContain('交给集群管理员')
    expect(registryWorkspace.default).toContain('配置节点运行时信任')
    expect(registryWorkspace.default).toContain('证书下载地址')
    expect(registryWorkspace.default).toContain('内网默认按自签或企业 CA 处理')
    expect(registryWorkspace.default).toContain('kpack build pod')
  })

  it('counts registry overview resources using the real workspace resource types', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const registryWorkspace = await import('../components/workspaces/RegistryWorkspace.vue?raw')

    expect(envDetail.default).toContain("x.type === 'Image Repository'")
    expect(envDetail.default).toContain("x.type === 'Runtime Trust'")
    expect(registryWorkspace.default).toContain("r.type === 'Repository' || r.type === 'Image Repository' || r.type === 'Harbor Repository'")
  })

  it('renders an ArgoCD-like topology with selectable resource details and resource actions', async () => {
    const argocdWorkspace = await import('../components/workspaces/ArgocdWorkspace.vue?raw')

    expect(argocdWorkspace.default).toContain('appRepoDisplayURL')
    expect(argocdWorkspace.default).toContain('resourceDescription')
    expect(argocdWorkspace.default).toContain('externalRepoURL')
    expect(argocdWorkspace.default).toContain('app?.annotations?.externalRepoURL')
    expect(argocdWorkspace.default).toContain("resource.type !== 'Application'")
    expect(argocdWorkspace.default).toContain("`Source: ${source}`")
    expect(argocdWorkspace.default).toContain('resourceDescription(r)')
    expect(argocdWorkspace.default).toContain("'server'")
    expect(argocdWorkspace.default).toContain("'proxyURL'")
    expect(argocdWorkspace.default).toContain('打开 ArgoCD 拓扑')
    expect(argocdWorkspace.default).toContain(':href="selectedApp.externalUrl"')
    expect(argocdWorkspace.default).not.toContain('{{ selectedApp.annotations?.repoURL || \'-\' }}')
    expect(argocdWorkspace.default).not.toContain('{{ selectedResourceView.description }}')
    expect(argocdWorkspace.default).not.toContain('<td class="cell-desc">{{ r.description }}</td>')
    expect(argocdWorkspace.default).toContain('资源详情')
    expect(argocdWorkspace.default).toContain('资源级操作')
    expect(argocdWorkspace.default).toContain('selectedResource')
    expect(argocdWorkspace.default).toContain('@click=\"selectResource')
    expect(argocdWorkspace.default).toContain('argocd-topology')
    expect(argocdWorkspace.default).toContain('topology-lanes')
    expect(argocdWorkspace.default).toContain('topology-node')
    expect(argocdWorkspace.default).toContain('argocdTreeLayout')
    expect(argocdWorkspace.default).toContain('treeNodes')
    expect(argocdWorkspace.default).toContain('treeEdges')
    expect(argocdWorkspace.default).toContain('buildArgoCDTreeLayout')
    expect(argocdWorkspace.default).toContain('argocdEdgePath')
  })

  it('renders a Gitea-like repository detail with selectable files, commits, and repo actions', async () => {
    const giteaWorkspace = await import('../components/workspaces/GiteaWorkspace.vue?raw')

    expect(giteaWorkspace.default).toContain('repo-home')
    expect(giteaWorkspace.default).toContain('ws-toolbar')
    expect(giteaWorkspace.default).toContain('ws-toolbar-actions')
    expect(giteaWorkspace.default).toContain('创建代码仓')
    expect(giteaWorkspace.default).toContain('配置公钥')
    expect(giteaWorkspace.default).not.toContain('创建/修复代码仓')
    expect(giteaWorkspace.default).toContain('git remote add origin')
    expect(giteaWorkspace.default).toContain('v-if="exampleCloneUrl"')
    expect(giteaWorkspace.default).toContain('暂无可推送的真实仓库')
    expect(giteaWorkspace.default).not.toContain('<strong>Deploy Key</strong>')
    expect(giteaWorkspace.default).not.toContain('deployKeyAction')
    expect(giteaWorkspace.default).not.toContain('component-source')
    expect(giteaWorkspace.default).toContain('文件详情')
    expect(giteaWorkspace.default).toContain('最近提交')
    expect(giteaWorkspace.default).toContain('仓库操作')
    expect(giteaWorkspace.default).toContain('selectedFile')
    expect(giteaWorkspace.default).toContain('selectedFile.value = null')
    expect(giteaWorkspace.default).toContain('readmePreview')
    expect(giteaWorkspace.default).toContain('loadRepositoryDirectory')
    expect(giteaWorkspace.default).toContain('repositoryContentsUrl')
    expect(giteaWorkspace.default).toContain('README')
    expect(giteaWorkspace.default).toContain('annotations?.commits')
    expect(giteaWorkspace.default).toContain('@click=\"selectFile')
    expect(giteaWorkspace.default).toContain('file-editor-main')
    expect(giteaWorkspace.default).toContain('textarea')
    expect(giteaWorkspace.default).toContain('editedFileContent')
    expect(giteaWorkspace.default).toContain('编辑')
    expect(giteaWorkspace.default).toContain('background: var(--paap-panel); color: var(--paap-text)')
    expect(giteaWorkspace.default).not.toContain('.file-editor-textarea { display: block; width: 100%; min-height: 620px; border: 0; resize: vertical; padding: var(--paap-space-4); outline: none; background: #0f1117')
  })

  it('keeps CI build details as the primary pane and contains long log lines', async () => {
    const pipelineWorkspace = await import('../components/workspaces/PipelineWorkspace.vue?raw')

    expect(pipelineWorkspace.default).toContain('grid-template-columns: minmax(260px, 320px) minmax(720px, 1fr)')
    expect(pipelineWorkspace.default).toContain('max-width: 100%')
    expect(pipelineWorkspace.default).toContain('white-space: pre-wrap')
    expect(pipelineWorkspace.default).toContain('overflow-wrap: anywhere')
    expect(pipelineWorkspace.default).not.toContain('white-space: nowrap')
  })

  it('renders a monitor workbench with Grafana panels as the primary view and secondary tabs', async () => {
    const monitorWorkspace = await import('../components/workspaces/MonitorWorkspace.vue?raw')

    expect(monitorWorkspace.default).toContain('Grafana 面板')
    expect(monitorWorkspace.default).toContain('iframe')
    expect(monitorWorkspace.default).toContain('grafana-frame')
    expect(monitorWorkspace.default).not.toContain('grafana-panel-head')
    expect(monitorWorkspace.default).toContain("theme', 'light'")
    expect(monitorWorkspace.default).toContain("kiosk', ''")
    expect(monitorWorkspace.default).toContain("embed', 'true'")
    expect(monitorWorkspace.default).toContain("paap_embed', '1'")
    expect(monitorWorkspace.default).toContain('dashboard-frame-shell')
    expect(monitorWorkspace.default).not.toContain("toSoloDashboardURL")
    expect(monitorWorkspace.default).not.toContain("'/d-solo/'")
    expect(monitorWorkspace.default).not.toContain("panelId")
    expect(monitorWorkspace.default).not.toContain('viewPanel')
    expect(monitorWorkspace.default).toContain('var-workload')
    expect(monitorWorkspace.default).toContain('grafanaFrameSource')
    expect(monitorWorkspace.default).toContain("annotations?.proxyURL")
    expect(monitorWorkspace.default).toContain('监控对象')
    expect(monitorWorkspace.default).toContain('selectedSubject')
    expect(monitorWorkspace.default).toContain('@click=\"selectSubject')
    expect(monitorWorkspace.default).toContain("const servicePrefix = 'monitor:service:'")
    expect(monitorWorkspace.default).toContain("annotations?.serviceId")
    expect(monitorWorkspace.default).toContain('max-height: calc(100vh - 180px)')
    expect(monitorWorkspace.default).toContain('height: calc(100vh - 220px)')
    expect(monitorWorkspace.default).toContain('Targets')
    expect(monitorWorkspace.default).toContain('Alerts')
    expect(monitorWorkspace.default).toContain('Rules')
  })

  it('renders a log workbench with logs as the primary view and selectable subjects', async () => {
    const logWorkspace = await import('../components/workspaces/LogWorkspace.vue?raw')

    expect(logWorkspace.default).toContain('日志对象')
    expect(logWorkspace.default).toContain('grafanaLogFrames')
    expect(logWorkspace.default).toContain('iframe')
    expect(logWorkspace.default).toContain('loki-frame')
    expect(logWorkspace.default).toContain('loki-frame-shell')
    expect(logWorkspace.default).not.toContain('loki-panel-head')
    expect(logWorkspace.default).toContain("theme', 'light'")
    expect(logWorkspace.default).toContain("kiosk', ''")
    expect(logWorkspace.default).toContain("embed', 'true'")
    expect(logWorkspace.default).toContain("paap_embed', '1'")
    expect(logWorkspace.default).toContain('toEmbeddedGrafanaURL')
    expect(logWorkspace.default).toContain('grafanaFrameSource')
    expect(logWorkspace.default).toContain("annotations?.proxyURL")
    expect(logWorkspace.default).toContain("from: 'now-24h'")
    expect(logWorkspace.default).toContain('logQuery')
    expect(logWorkspace.default).toContain("const servicePrefix = 'log:service:'")
    expect(logWorkspace.default).toContain("annotations?.serviceId")
    expect(logWorkspace.default).toContain("parsed.pathname.includes('/explore')")
    expect(logWorkspace.default).toContain('selectedSubject')
    expect(logWorkspace.default).toContain('@click=\"selectSubject')
    expect(logWorkspace.default).toContain('max-height: calc(100vh - 180px)')
    expect(logWorkspace.default).not.toContain('transform: translate(-72px, -56px)')
    expect(logWorkspace.default).toContain('height: calc(100vh - 220px)')
    expect(logWorkspace.default).not.toContain('<div class="stream-list">')
    expect(logWorkspace.default).not.toContain('<div class="terminal">')
    expect(logWorkspace.default).not.toContain('最近日志')
  })

  it('renders a Jenkins-like pipeline workbench with stages, logs, and image artifacts', async () => {
    const pipelineWorkspace = await import('../components/workspaces/PipelineWorkspace.vue?raw')

    expect(pipelineWorkspace.default).toContain('构建详情')
    expect(pipelineWorkspace.default).toContain('阶段视图')
    expect(pipelineWorkspace.default).toContain('构建日志')
    expect(pipelineWorkspace.default).toContain('镜像产物')
    expect(pipelineWorkspace.default).toContain('真实 Jenkins 数据')
    expect(pipelineWorkspace.default).toContain('暂无真实构建日志')
    expect(pipelineWorkspace.default).toContain('realBuildLogLines')
    expect(pipelineWorkspace.default).toContain('grid-template-columns: minmax(260px, 320px) minmax(720px, 1fr)')
    expect(pipelineWorkspace.default).toContain('selectedJob')
    expect(pipelineWorkspace.default).toContain('@click=\"selectJob')
    expect(pipelineWorkspace.default).toContain('min-height: 360px')
    expect(pipelineWorkspace.default).toContain('max-height: 520px')
    expect(pipelineWorkspace.default).not.toContain('const buildLogLines = (job: WorkspaceResource) => [')
    expect(pipelineWorkspace.default).not.toContain('const buildNumber = (job: WorkspaceResource) => {')
  })

  it('shows environment components as components with a dependency map for dense environments', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const workspaceFrame = await import('../components/workspaces/ToolWorkspaceFrame.vue?raw')
    const globalStyles = readFileSync(new URL('../style.scss', import.meta.url), 'utf8')

    expect(envDetail.default).toContain('overviewPanelTab')
    expect(envDetail.default).toContain('<div class="ws-tabs env-secondary-tabs">')
    expect(envDetail.default).toContain('<div class="component-header-row">')
    expect(envDetail.default).toContain('<div class="ws-tabs component-secondary-tabs">')
    expect(envDetail.default).toContain('overview-topology-primary')
    expect(envDetail.default).toContain('overview-detail-panel')
    expect(envDetail.default).toContain("overviewPanelTab === 'topology'")
    expect(envDetail.default).toContain("overviewPanelTab === 'details'")
    expect(envDetail.default).toContain('component-topology-primary')
    expect(envDetail.default).toContain('应用拓扑</h2>')
    expect(envDetail.default).toContain('component-dependency-map')
    expect(envDetail.default).toContain('componentTopologyAllNodes')
    expect(envDetail.default).toContain('environmentTopologyAllNodes')
    expect(envDetail.default).toContain('component-topology-canvas')
    expect(envDetail.default).toContain('component-canvas-links')
    expect(envDetail.default).toContain('component-canvas-node')
    expect(envDetail.default).toContain('componentCanvasNodes')
    expect(envDetail.default).toContain('componentCanvasEdges')
    expect(envDetail.default).toContain('componentCanvasViewBox')
    expect(envDetail.default).toContain('componentNodeStyle')
    expect(envDetail.default).toContain('componentEdgePath')
    expect(envDetail.default).toContain('component-filter-bar')
    expect(envDetail.default).toContain('component-table-panel')
    expect(envDetail.default).toContain('component-detail-panel')
    expect(envDetail.default).toContain('component-overview-workspace')
    expect(envDetail.default).toContain('component-overview-map')
    expect(envDetail.default).toContain('selectComponent')
    expect(envDetail.default).toContain('openComponentConfigDrawer')
    expect(envDetail.default).toContain('deployComponent')
    expect(envDetail.default).toContain('selectedComponent')
    expect(envDetail.default).toContain('filteredComponents')
    expect(envDetail.default).toContain('componentCanvasEdges')
    expect(envDetail.default).toContain('min-height: 720px')
    expect(envDetail.default).toContain('min-height: 800px')
    expect(envDetail.default).not.toContain('component-focus-grid')
    expect(envDetail.default).not.toContain('component-focus-card')
    expect(envDetail.default).not.toContain('环境组件</h2>')
    expect(envDetail.default).not.toContain('component-panel-tabs')
    expect(globalStyles).toContain('.ws-tabs')
    expect(globalStyles).toContain('.ws-tab.active')
    expect(workspaceFrame.default).not.toContain('.ws-tabs')
  })

  it('renders ArgoCD applications as a topology map with details below the graph', async () => {
    const argocdWorkspace = await import('../components/workspaces/ArgocdWorkspace.vue?raw')

    expect(argocdWorkspace.default).toContain('argocd-topology')
    expect(argocdWorkspace.default).toContain('topology-lanes')
    expect(argocdWorkspace.default).toContain('topology-node')
    expect(argocdWorkspace.default).toContain('resourceDetailRows')
    expect(argocdWorkspace.default).toContain('资源详情')
    expect(argocdWorkspace.default).toContain('grid-template-columns: minmax(260px, 320px) minmax(720px, 1fr)')
    expect(argocdWorkspace.default).toContain(':width="argocdTreeLayout.width"')
    expect(argocdWorkspace.default).toContain(':height="argocdTreeLayout.height"')
    expect(argocdWorkspace.default).toContain('flex: 0 0 auto')
    expect(argocdWorkspace.default).toContain('width: 100%')
    expect(argocdWorkspace.default).toContain('height: 100%')
    expect(argocdWorkspace.default).not.toContain('grid-template-columns: minmax(0, 1fr) minmax(280px, 360px)')
    expect(argocdWorkspace.default).not.toContain('min-width: 100%;')
  })

  it('renders the component source to cluster delivery chain', async () => {
    const componentDetail = await import('./ComponentDetailView.vue?raw')

    expect(componentDetail.default).toContain('交付链路')
    expect(componentDetail.default).toContain('从源码、代码仓、构建、镜像、GitOps 到集群运行态')
    expect(componentDetail.default).toContain('buildComponentDeliveryChain')
    expect(componentDetail.default).toContain('deliveryChain')
    expect(componentDetail.default).toContain('资源拓扑')
    expect(componentDetail.default).toContain('buildComponentResourceTopology')
    expect(componentDetail.default).toContain('component-argocd-topology')
    expect(componentDetail.default).toContain('component-resource-links')
    expect(componentDetail.default).toContain('selectedResourceNode')
    expect(componentDetail.default).toContain('selectResourceNode')
    expect(componentDetail.default).toContain('resourceDetailRows')
    expect(componentDetail.default).toContain('资源详情')
    expect(componentDetail.default).toContain('不会根据状态推断未返回的 Pod 或 ReplicaSet')
  })

  it('lets component detail configure runtime env before explicit deploy', async () => {
    const componentDetail = await import('./ComponentDetailView.vue?raw')
    const client = await import('../api/client.ts?raw')

    expect(componentDetail.default).toContain('运行配置')
    expect(componentDetail.default).toContain('保存配置不会部署')
    expect(componentDetail.default).toContain('SecretKeyRef')
    expect(componentDetail.default).toContain('ConfigMapKeyRef')
    expect(componentDetail.default).toContain('saveComponentConfig')
    expect(componentDetail.default).toContain('deployCurrentComponent')
    expect(componentDetail.default).toContain('api.updateComponent')
    expect(client.default).toContain('updateComponent')
  })

  it('lets users connect component and middleware relationships from the canvas context menu', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('component-context-menu')
    expect(envDetail.default).toContain('startConnectionDrag')
    expect(envDetail.default).toContain('createComponentDependency')
    expect(envDetail.default).toContain('manualCanvasEdges')
    expect(envDetail.default).toContain('componentCanvasEdges = computed(() => canvasEdgesForNodes(componentCanvasNodes.value, []))')
    expect(envDetail.default).toContain('environmentCanvasEdges = computed(() => canvasEdgesForNodes(environmentCanvasNodes.value, []))')
    expect(envDetail.default).toContain('api.saveEnvironmentCanvasState')
    expect(envDetail.default).toContain('canvasNodesForStage')
  })

  it('keeps topology card geometry stable so zoomed canvas links stay attached', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('height: `${node.height}px`')
    expect(envDetail.default).toContain('box-sizing: border-box;')
    expect(envDetail.default).not.toContain('transform: translateY(-2px);')
    expect(envDetail.default).not.toContain('transform: scale(1.02);')
  })

  it('lets users create source-delivery components from the environment page', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('源码交付')
    expect(envDetail.default).toContain('镜像交付')
    expect(envDetail.default).toContain('sourceRepoUrl')
    expect(envDetail.default).toContain('sourceBranch')
    expect(envDetail.default).toContain('buildContext')
    expect(envDetail.default).toContain('deliveryMode')
    expect(envDetail.default).toContain('源码交付可留空')
    expect(envDetail.default).toContain("deliveryMode === 'image' && (!version || version.toLowerCase() === 'latest')")
    expect(envDetail.default).toContain("deliveryMode === 'source' && version && version.toLowerCase() === 'latest'")
  })

  it('creates components as drafts and exposes explicit canvas deploy actions', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')
    const client = await import('../api/client.ts?raw')

    expect(envDetail.default).toContain('保存草稿')
    expect(envDetail.default).toContain('@contextmenu.stop.prevent="openTopologyContextMenu')
    expect(envDetail.default).toContain('component-topology-node--service')
    expect(envDetail.default).toContain('component-context-menu')
    expect(envDetail.default).toContain('deployComponent(')
    expect(envDetail.default).toContain('配置')
    expect(envDetail.default).toContain('部署')
    expect(envDetail.default).toContain('监控')
    expect(envDetail.default).toContain('日志')
    expect(envDetail.default).toContain('contextSubjectTargetKey')
    expect(envDetail.default).toContain(':initial-subject-key="capabilityInitialSubjectKey"')
    expect(client.default).toContain('deployComponent')
  })

  it('blocks canvas ApplicationSet deploy when single Applications already manage cards', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('应用集部署')
    expect(envDetail.default).toContain('applicationSetDeployBlocked')
    expect(envDetail.default).toContain('deployCanvasApplicationSet')
    expect(envDetail.default).toContain('当前环境已有单独 Application 管理的组件，不能直接使用应用集部署')
  })

  it('exposes canvas context actions for creating app resources installing tools and configuring service nodes', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('@contextmenu.prevent="openCanvasContextMenu')
    expect(envDetail.default).toContain('@contextmenu.stop.prevent="openTopologyContextMenu')
    expect(envDetail.default).toContain('createCanvasComponentDraft')
    expect(envDetail.default).toContain('installCanvasService')
    expect(envDetail.default).toContain('api.saveEnvironmentCanvasState')
    expect(envDetail.default).toContain('configureContextNode')
    expect(envDetail.default).toContain("componentContextMenu.kind === 'canvas'")
    expect(envDetail.default).toContain("componentContextMenu.kind === 'service'")
    expect(envDetail.default).toContain('<span>安装工具</span>')
    expect(envDetail.default).toContain('<span>添加中间件</span>')
    expect(envDetail.default).toContain('connectorSides')
    expect(envDetail.default).toContain('canvasNodesForStage')
    expect(envDetail.default).toContain('node-connector--left')
    expect(envDetail.default).toContain('接入已有资源')
    expect(envDetail.default).toContain('showAdoptResourceModal')
    expect(envDetail.default).toContain('api.listAdoptableResources')
    expect(envDetail.default).toContain('api.adoptResource')
    expect(envDetail.default).toContain('adoptResourceSelection')
    expect(envDetail.default).toContain('component-canvas-empty-hint')
    expect(envDetail.default).not.toContain('v-if="components.length > 0" class="component-workspace"')
    expect(envDetail.default).not.toContain('后续会接到资源发现接口')
  })

  it('shows tool install and middleware create actions next to create component on the environment page', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('安装工具')
    expect(envDetail.default).toContain('添加中间件')
    expect(envDetail.default).toContain('@click="openServiceModal"')
    expect(envDetail.default).toContain('openInfraSubmenu')
    expect(envDetail.default).not.toContain("activeCapabilityTab && activeCapabilityTab.category !== 'infra'")
    expect(envDetail.default).not.toContain("activeCapabilityTab?.category === 'infra'")
    expect(envDetail.default).not.toContain('添加基础设施')
  })

  it('does not render synthetic service workspaces before the backend workspace API returns', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('打开外部控制台')
    expect(envDetail.default).toContain('activeCapabilityService.externalUrl')
    expect(envDetail.default).toContain('activeCapabilityWorkspaceReady')
    expect(envDetail.default).toContain('emptyCapabilityWorkspace')
    expect(envDetail.default).not.toContain('activeCapabilityWorkspace.config.length')
    expect(envDetail.default).not.toContain('workspace-config-grid capability-config-grid')
    expect(envDetail.default).toContain('.capability-action-modal { max-width: 520px; }')
    expect(envDetail.default).not.toContain('buildServiceWorkspace')
    expect(envDetail.default).not.toContain('fallbackCapabilityWorkspace')
  })

  it('uses Carbon blue for primary rail buttons instead of black', () => {
    const styles = readFileSync(new URL('../style.scss', import.meta.url), 'utf8')

    expect(styles).toContain('.rail-btn--primary')
    expect(styles).toContain('background: var(--cds-button-primary, var(--paap-accent));')
    expect(styles).not.toContain('.rail-btn--primary {\n  background: var(--paap-text);')
  })

  it('uses Carbon brand color for shell logo and application switcher icons', async () => {
    const appLayout = await import('../layouts/AppLayout.vue?raw')
    const mainLayout = await import('../layouts/MainLayout.vue?raw')

    for (const source of [appLayout.default, mainLayout.default]) {
      expect(source).toContain('.logo-icon')
      expect(source).toContain('background: var(--cds-background-brand, var(--paap-accent));')
      expect(source).toContain('color: var(--cds-icon-on-color, #fff);')
      expect(source).not.toContain('background: var(--paap-text);\n  color: #fff;')
    }

    expect(appLayout.default).toContain('.context-avatar')
    expect(appLayout.default).toContain('color: var(--cds-icon-interactive, var(--paap-accent));')
  })

  it('tears down stale capability workspaces when switching environments', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('activeTab.value = \'overview\'')
    expect(envDetail.default).toContain('capabilityTabs.value.some(item => item.key === tab) ? tab : \'overview\'')
    expect(envDetail.default).toContain(':key="capabilityWorkspaceKey"')
    expect(envDetail.default).toContain('capabilityWorkspaceLoadSeq++')
    expect(envDetail.default).toContain('targetEnvId !== envId.value')
    expect(envDetail.default).toContain('targetServiceId')
  })

  it('keeps lazy Gitea file loads bound to the selected repository', async () => {
    const giteaWorkspace = await import('../components/workspaces/GiteaWorkspace.vue?raw')

    expect(giteaWorkspace.default).toContain('const requestRepoKey = repoIdentity(selectedRepo.value)')
    expect(giteaWorkspace.default).toContain('repoIdentity(selectedRepo.value) !== requestRepoKey')
    expect(giteaWorkspace.default).toContain('clearRepositorySelection')
    expect(giteaWorkspace.default).toContain('resetRepositoryLoadState')
    expect(giteaWorkspace.default).toContain('loadedDirectories.value = {}')
    expect(giteaWorkspace.default).toContain('loadedRepoTrees.value = {}')
  })

  it('keeps service workspace headers focused on the external console action', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('打开外部控制台')
    expect(envDetail.default).toContain('activeCapabilityService.externalUrl')
    expect(envDetail.default).not.toContain('activeCapabilityWorkspace.config')
    expect(envDetail.default).not.toContain('v-for="item in activeCapabilityWorkspace.config"')
    expect(envDetail.default).not.toContain('workspace-config-link')
  })

  it('renders data and middleware workspaces with selectable object details and object actions', async () => {
    const workspaceModules = [
      '../components/workspaces/DatabaseWorkspace.vue?raw',
      '../components/workspaces/RedisWorkspace.vue?raw',
      '../components/workspaces/MongoWorkspace.vue?raw',
      '../components/workspaces/RabbitWorkspace.vue?raw',
      '../components/workspaces/KafkaWorkspace.vue?raw',
      '../components/workspaces/MinioWorkspace.vue?raw',
    ] as const

    for (const path of workspaceModules) {
      const mod = await import(path)
      expect(mod.default).toContain('对象详情')
      expect(mod.default).toContain('对象级操作')
      expect(mod.default).toContain('selectedResource')
      expect(mod.default).toContain('@click=\"selectResource')
    }
  })

  it('configures component images from real registry workspace repositories and tags', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('registryImageRepositories')
    expect(envDetail.default).toContain('registryHostForDrawer')
    expect(envDetail.default).toContain('仓库')
    expect(envDetail.default).toContain('镜像:Tag')
    expect(envDetail.default).toContain('registryHostForDrawer ||')
    expect(envDetail.default).toContain('ensureRegistryWorkspaces')
    expect(envDetail.default).toContain("x.type === 'Image Repository'")
    expect(envDetail.default).toContain("x.type === 'Harbor Repository'")
    expect(envDetail.default).toContain("x.type === 'Repository'")
    expect(envDetail.default).toContain('没有扫描到仓库时也可以直接填写仓库名和 Tag。')
    expect(envDetail.default).toContain('registryImageFromConfig')
    expect(envDetail.default).toContain('configForm.repository')
    expect(envDetail.default).toContain('v-model.trim="configForm.repository"')
    expect(envDetail.default).toContain('v-model.trim="configForm.version"')
    expect(envDetail.default).toContain('normalizeRegistryRepository')
    expect(envDetail.default).toContain('list="component-registry-repositories"')
    expect(envDetail.default).toContain('list="component-registry-tags"')
    expect(envDetail.default).toContain('<datalist id="component-registry-repositories">')
    expect(envDetail.default).toContain('<datalist id="component-registry-tags">')
  })

  it('keeps canvas-created middleware on the topology and exposes service deletion', async () => {
    const envDetail = await import('./EnvDetailView.vue?raw')

    expect(envDetail.default).toContain('deleteContextService')
    expect(envDetail.default).toContain('deleteDrawerService')
    expect(envDetail.default).toContain('deleteTopologyNode')
    expect(envDetail.default).toContain('performDeleteService')
    expect(envDetail.default).toContain('api.uninstallService')
    expect(envDetail.default).toContain('topologyDeleteTitle')
    expect(envDetail.default).toContain("'删除卡片'")
    expect(envDetail.default).toContain("setActiveTab('components')")
    expect(envDetail.default).not.toContain("setActiveTab(mode === 'infra' ? 'components' : tabForServiceTypes([serviceType]))")
  })
})
