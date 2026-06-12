<template>
  <div class="rail-page">
    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <div class="environment-shell">
      <!-- Tab panels -->
      <main class="tab-panel">
      <!-- 概览 -->
      <div v-if="activeTab === 'overview'">
        <div class="ws-tabs env-secondary-tabs">
          <button type="button" class="ws-tab" :class="{ active: overviewPanelTab === 'topology' }" @click="overviewPanelTab = 'topology'">环境拓扑</button>
          <button type="button" class="ws-tab" :class="{ active: overviewPanelTab === 'details' }" @click="overviewPanelTab = 'details'">环境详情</button>
        </div>

        <div class="overview-panel">
        <section v-if="overviewPanelTab === 'topology'" class="overview-section component-focus overview-topology-primary">
          <div class="overview-section-head">
            <h2 class="overview-title">环境拓扑</h2>
            <span class="overview-subtitle">组件、平台工具、中间件和数据库的环境视图</span>
          </div>
          <div class="environment-topology-workspace environment-topology-workspace--primary">
            <div class="component-topology-canvas environment-topology-canvas environment-topology-canvas--primary" @contextmenu.prevent="openCanvasContextMenu">
              <div class="topology-controls">
                <button type="button" class="topology-control-btn" @click="zoomIn" title="放大">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 3.5a.5.5 0 0 1 .5.5v3.5H12a.5.5 0 0 1 0 1H8.5V12a.5.5 0 0 1-1 0V8.5H4a.5.5 0 0 1 0-1h3.5V4a.5.5 0 0 1 .5-.5z"/></svg>
                </button>
                <button type="button" class="topology-control-btn" @click="zoomOut" title="缩小">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M4 7.5a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 0 1h-7a.5.5 0 0 1-.5-.5z"/></svg>
                </button>
                <button type="button" class="topology-control-btn" @click="resetZoom" title="重置视图">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M4 8a4 4 0 1 1 8 0 4 4 0 0 1-8 0zm4-5a5 5 0 1 0 0 10 5 5 0 0 0 0-10z"/></svg>
                </button>
                <span class="topology-zoom-label">{{ Math.round(canvasZoom * 100) }}%</span>
              </div>
              <div class="component-canvas-stage" :style="{ width: `${environmentCanvasSize.width}px`, height: `${environmentCanvasSize.height}px`, transform: `scale(${canvasZoom})`, transformOrigin: 'top left' }" @pointerdown="startCanvasMarquee">
                <svg class="component-canvas-links" :viewBox="environmentCanvasViewBox" aria-hidden="true">
                  <defs>
                    <marker id="environment-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                      <path d="M 0 0 L 10 5 L 0 10 z" class="component-arrow-head" />
                    </marker>
                  </defs>
                  <path
                    v-for="edge in environmentCanvasEdges"
                    :key="`${edge.fromKey || edge.fromId}-${edge.toKey || edge.toId}`"
                    class="component-canvas-link environment-canvas-link"
                    :d="componentEdgePath(edge)"
                  />
                  <path
                    v-if="connectionDrag"
                    class="connection-drag-line"
                    :d="`M ${connectionDrag.startX} ${connectionDrag.startY} L ${connectionDrag.currentX} ${connectionDrag.currentY}`"
                    stroke="#3b82f6"
                    stroke-width="2"
                    stroke-dasharray="5,5"
                    fill="none"
                    marker-end="url(#environment-arrow)"
                  />
                  <rect
                    v-if="marqueeRect"
                    class="topology-marquee"
                    :x="marqueeRect.x"
                    :y="marqueeRect.y"
                    :width="marqueeRect.width"
                    :height="marqueeRect.height"
                  />
                </svg>
                <div v-if="environmentCanvasNodes.length === 0" class="component-canvas-empty-hint">
                  <strong>空环境</strong>
                  <span>在画布空白处右键创建组件、工具、数据库或中间件。</span>
                </div>
                <button
                  v-for="node in environmentCanvasNodes"
                  :key="node.topologyId || node.id || node.name"
                  type="button"
                  class="component-canvas-node component-topology-node"
                  :class="{ selected: selectedNodeKeys.includes(String(node.topologyId || node.id)), 'component-topology-node--service': node.topologyKind === 'service' }"
                  :style="componentNodeStyle(node)"
                  :data-node-key="node.topologyId || node.id"
                  @click="handleTopologyNodeClick($event, node)"
                  @pointerdown="startComponentNodeDrag($event, node)"
                  @pointerup="finishComponentNodePointer($event, node)"
                  @contextmenu.stop.prevent="openTopologyContextMenu($event, node)"
                >
                  <span class="node-type-icon" :class="componentNodeIconClass(node)">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="componentNodeIconPath(node)" /></svg>
                  </span>
                  <span class="node-status" :class="node.status"></span>
                  <strong>{{ node.name }}</strong>
                  <small>{{ environmentTopologyNodeSubtitle(node) }}</small>
                  <span
                    class="node-delete-action"
                    role="button"
                    tabindex="0"
                    :title="topologyDeleteTitle(node)"
                    @click.stop="deleteTopologyNode(node)"
                    @keydown.enter.stop.prevent="deleteTopologyNode(node)"
                    @keydown.space.stop.prevent="deleteTopologyNode(node)"
                    @pointerdown.stop
                  >
                    <svg focusable="false" width="14" height="14" viewBox="0 0 32 32" fill="currentColor"><path d="M12 12h2v12h-2zm6 0h2v12h-2z"/><path d="M4 6v2h2v20c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8h2V6H4zm4 22V8h16v20H8zM12 2h8v2h-8z"/></svg>
                  </span>
                  <div
                    v-for="side in connectorSides"
                    :key="side"
                    class="node-connector"
                    :class="`node-connector--${side}`"
                    @pointerdown.stop="startConnectionDrag($event, node, side)"
                    title="拖拽到其他节点建立连接"
                  ></div>
                </button>
              </div>
            </div>
          </div>
        </section>

        <div v-if="overviewPanelTab === 'details'" class="overview-detail-panel">
          <div class="overview-env-status">
            <span>{{ envStatusText(env.status) }}</span>
          </div>
          <div class="overview-stats">
            <button type="button" class="overview-stat" @click="setActiveTab('components')">
              <span class="stat-label">组件</span>
              <span class="stat-value">{{ components.length }}</span>
              <span class="stat-hint">{{ runningComponentCount }} 个运行中</span>
            </button>
            <button type="button" class="overview-stat" @click="scrollToCoreTools">
              <span class="stat-label">核心工具</span>
              <span class="stat-value">{{ installedTools.length }}</span>
              <span class="stat-hint">{{ runningToolCount }} 个运行中</span>
            </button>
            <button type="button" class="overview-stat" @click="setFirstCapabilityTab('infra')">
              <span class="stat-label">中间件</span>
              <span class="stat-value">{{ installedInfra.length }}</span>
              <span class="stat-hint">{{ runningInfraCount }} 个运行中</span>
            </button>
            <div class="overview-stat">
              <span class="stat-label">异常</span>
              <span class="stat-value" :class="{ danger: unhealthyServices.length > 0 }">{{ unhealthyServices.length }}</span>
              <span class="stat-hint">服务安装/运行问题</span>
            </div>
          </div>

          <section class="overview-section external-access-section">
            <div class="overview-section-head">
              <h2 class="overview-title">应用访问</h2>
              <button
                v-if="applicationFrontendURL"
                type="button"
                class="rail-btn rail-btn--primary"
                @click="openApplicationURL"
              >
                打开应用
              </button>
            </div>
            <div v-if="externalAccess.length" class="external-access-grid">
              <div v-for="group in externalAccessGroups" :key="group.key" class="external-access-group">
                <div class="external-access-group-head">
                  <span>{{ group.label }}</span>
                  <strong>{{ group.items.length }}</strong>
                </div>
                <a
                  v-for="item in group.items"
                  :key="`${item.namespace}-${item.kind}-${item.name}-${item.url}`"
                  class="external-access-row"
                  :href="item.url"
                  target="_blank"
                  rel="noreferrer"
                >
                  <span class="external-access-main">
                    <strong>{{ item.name }}</strong>
                    <small>{{ externalAccessSubtitle(item) }}</small>
                  </span>
                  <span class="rail-tag rail-tag--blue">{{ item.kind }}</span>
                </a>
              </div>
            </div>
            <div v-else class="overview-empty">
              <p>暂无应用访问入口。</p>
            </div>
          </section>

          <div class="overview-grid">
            <section class="overview-section">
              <div class="overview-section-head">
                <h2 class="overview-title">交付流程</h2>
                <span class="overview-subtitle">Source → Gitea → Jenkins/kpack → Registry → ArgoCD</span>
              </div>
              <div class="flow-list">
                <div v-for="step in deliverySteps" :key="step.key" class="flow-step" :class="step.state">
                  <span class="flow-dot" />
                  <div class="flow-body">
                    <div class="flow-name">{{ step.label }}</div>
                    <div class="flow-desc">{{ step.description }}</div>
                  </div>
                  <button v-if="step.targetTab" type="button" class="flow-link" @click="setActiveTab(step.targetTab)">查看</button>
                </div>
              </div>
            </section>

            <section ref="coreToolsSection" class="overview-section overview-section--anchor">
              <div class="overview-section-head">
                <h2 class="overview-title">核心工具</h2>
                <span class="overview-subtitle">点击进入真实工具工作台</span>
              </div>
              <div v-if="criticalTools.length" class="quick-list">
                <button v-for="svc in criticalTools" :key="svc.id" type="button" class="quick-row" @click="openServiceWorkspace(svc)">
                  <span class="quick-icon" :class="svc.serviceType">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path :d="serviceIconPath(svc.serviceType)"/></svg>
                  </span>
                  <span class="quick-main">
                    <span class="quick-name">{{ svcLabel(svc.serviceType) }}</span>
                    <span class="quick-desc">{{ typeLabel(svc.serviceType) }}</span>
                  </span>
                  <span class="bx--tag bx--tag--sm" :class="statusTagClass(svc.status)">{{ serviceStatusText(svc.status) }}</span>
                </button>
              </div>
              <div v-else class="overview-empty">
                <p>还没有安装 Git、CI、部署、监控或镜像仓库工具。</p>
                <button type="button" class="bx--btn bx--btn--primary" @click="openServiceModal">安装工具</button>
              </div>
            </section>
          </div>

          <section class="overview-section overview-wide">
            <div class="overview-section-head">
              <h2 class="overview-title">环境关注项</h2>
              <span class="overview-subtitle">异常服务和支撑资源</span>
            </div>
            <div class="overview-two-col">
              <div class="compact-list">
                <div class="compact-head">
                  <span>核心工具</span>
                  <button type="button" class="text-btn" @click="scrollToCoreTools">全部</button>
                </div>
                <button v-for="svc in criticalTools.slice(0, 5)" :key="svc.id" type="button" class="compact-row" @click="openServiceWorkspace(svc)">
                  <span class="compact-name">{{ svcLabel(svc.serviceType) }}</span>
                  <span class="rail-tag" :class="statusTagClass(svc.status)">{{ serviceStatusText(svc.status) }}</span>
                </button>
                <div v-if="criticalTools.length === 0" class="compact-empty">暂无核心工具</div>
              </div>
              <div class="compact-list">
                <div class="compact-head">
                  <span>需要关注</span>
                  <button type="button" class="text-btn" @click="setActiveTab(unhealthyServices[0] ? serviceCapability(unhealthyServices[0]).key : 'components')">定位</button>
                </div>
                <button v-for="svc in unhealthyServices.slice(0, 5)" :key="svc.id" type="button" class="compact-row" @click="openServiceWorkspace(svc)">
                  <span class="compact-name">{{ svcLabel(svc.serviceType) }}</span>
                  <span class="rail-tag" :class="statusTagClass(svc.status)">{{ serviceStatusText(svc.status) }}</span>
                </button>
                <div v-if="unhealthyServices.length === 0" class="compact-empty">暂无异常服务</div>
              </div>
            </div>
          </section>
        </div>
        </div>
      </div>
      <!-- 环境能力 -->
      <div v-else-if="activeCapabilityTab">
        <div v-if="activeCapabilityServices.length === 0" class="empty-state">
          <svg class="empty-illustration" width="96" height="96" viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="4" y="16" width="56" height="36" rx="0" stroke="#c6c6c6" stroke-width="2" fill="none"/>
            <line x1="4" y1="26" x2="60" y2="26" stroke="#e0e0e0" stroke-width="2"/>
            <circle cx="14" cy="21" r="2" fill="#c6c6c6"/>
            <circle cx="22" cy="21" r="2" fill="#c6c6c6"/>
            <rect x="10" y="32" width="18" height="14" fill="#e0e0e0" opacity="0.6"/>
            <rect x="34" y="32" width="20" height="6" fill="#e0e0e0" opacity="0.4"/>
            <rect x="34" y="42" width="12" height="4" fill="#e0e0e0" opacity="0.4"/>
          </svg>
          <h3 class="empty-title">暂无{{ activeCapabilityTab?.label }}</h3>
          <p class="empty-text">当前环境还没有安装对应能力。点击右上角安装工具或创建中间件。</p>
        </div>
        <div v-else class="capability-workspace">
          <div class="capability-workspace-bar">
            <div class="capability-workspace-title">
              <strong>{{ activeCapabilityTab?.label }}</strong>
              <span>{{ activeCapabilityService ? serviceCapabilityDescription(activeCapabilityService) : '-' }}</span>
            </div>
            <div class="capability-workspace-actions">
              <button
                v-for="svc in activeCapabilityServices"
                :key="svc.id"
                type="button"
                class="capability-service-pill"
                :class="{ active: activeCapabilityService?.id === svc.id }"
                @click="selectCapabilityService(svc)"
              >
                {{ capabilityServiceInstanceLabel(svc, templates) }}
              </button>
              <button v-if="activeCapabilityService" type="button" class="text-btn danger" @click="beginUninstallService(activeCapabilityService)">卸载</button>
              <a
                v-if="activeCapabilityService?.externalUrl"
                :href="activeCapabilityService.externalUrl"
                target="_blank"
                rel="noreferrer"
                class="text-btn capability-external-link"
              >
                打开外部控制台
              </a>
            </div>
          </div>

          <div v-if="capabilityWorkspaceError" class="page-error" role="alert">{{ capabilityWorkspaceError }}</div>
          <div v-else-if="capabilityWorkspaceMessage" class="workspace-message" role="status">{{ capabilityWorkspaceMessage }}</div>
          <div v-if="capabilityWorkspaceLoading" class="workspace-loading">工作台加载中...</div>

          <component
            v-if="activeCapabilityService && activeCapabilityWorkspaceReady && workspaceComponentForService(activeCapabilityService)"
            :key="capabilityWorkspaceKey"
            :is="workspaceComponentForService(activeCapabilityService)"
            :resources="activeCapabilityWorkspace.resources"
            :initial-subject-key="capabilityInitialSubjectKey"
            @action="(a: any, t?: string) => beginCapabilityWorkspaceAction(a, t)"
          />
          <div v-else-if="activeCapabilityService && workspaceComponentForService(activeCapabilityService)" class="empty-state compact-empty-state">
            <h3 class="empty-title">暂无真实工作台数据</h3>
            <p class="empty-text">正在读取服务工作台，未返回前不会展示本地构造的仓库或文件树。</p>
          </div>
          <div v-else class="empty-state compact-empty-state">
            <h3 class="empty-title">暂无工作台</h3>
            <p class="empty-text">当前能力暂未提供内置工作台。</p>
          </div>
        </div>
      </div>

      <!-- 组件 -->
      <!-- @ts-ignore -->
      <div v-else-if="activeTab === 'components'">
        <div class="component-workspace">
          <div class="component-header-row">
            <div class="ws-tabs component-secondary-tabs">
              <button type="button" class="ws-tab" :class="{ active: componentPanelTab === 'topology' }" @click="componentPanelTab = 'topology'">应用拓扑</button>
              <button type="button" class="ws-tab" :class="{ active: componentPanelTab === 'list' }" @click="componentPanelTab = 'list'">组件列表</button>
            </div>
            <button
              v-if="applicationFrontendURL"
              type="button"
              class="rail-btn rail-btn--primary rail-btn--sm"
              @click="openApplicationURL"
            >
              打开应用
            </button>
          </div>

          <section v-if="componentPanelTab === 'topology'" class="component-dependency-map component-dependency-map--canvas-only component-topology-primary">
            <div class="component-map-head">
              <div>
                <h2 class="component-map-title">应用拓扑</h2>
                <p class="component-map-desc">展示应用内部组件关系，以及组件连接到数据库、中间件的关系。</p>
              </div>
              <div class="component-map-tags">
                <button
                  type="button"
                  class="rail-btn rail-btn--secondary rail-btn--sm application-set-deploy-btn"
                  :class="{ 'application-set-deploy-btn--blocked': applicationSetDeployBlocked }"
                  :aria-disabled="applicationSetDeployBlocked ? 'true' : 'false'"
                  :title="applicationSetDeployHint"
                  @click="deployCanvasApplicationSet"
                >
                  应用集部署
                </button>
              </div>
            </div>
            <p v-if="applicationSetDeployBlocked" class="application-set-deploy-hint">
              {{ applicationSetDeployHint }}
            </p>
            <div class="component-topology-canvas component-topology-canvas--main" @contextmenu.prevent="openCanvasContextMenu">
              <div class="topology-controls">
                <button type="button" class="topology-control-btn" @click="zoomIn" title="放大">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 3.5a.5.5 0 0 1 .5.5v3.5H12a.5.5 0 0 1 0 1H8.5V12a.5.5 0 0 1-1 0V8.5H4a.5.5 0 0 1 0-1h3.5V4a.5.5 0 0 1 .5-.5z"/></svg>
                </button>
                <button type="button" class="topology-control-btn" @click="zoomOut" title="缩小">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M4 7.5a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 0 1h-7a.5.5 0 0 1-.5-.5z"/></svg>
                </button>
                <button type="button" class="topology-control-btn" @click="resetZoom" title="重置视图">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M4 8a4 4 0 1 1 8 0 4 4 0 0 1-8 0zm4-5a5 5 0 1 0 0 10 5 5 0 0 0 0-10z"/></svg>
                </button>
                <span class="topology-zoom-label">{{ Math.round(canvasZoom * 100) }}%</span>
              </div>
              <div class="component-canvas-stage" :style="{ width: `${componentCanvasSize.width}px`, height: `${componentCanvasSize.height}px`, transform: `scale(${canvasZoom})`, transformOrigin: 'top left' }" @pointerdown="startCanvasMarquee">
                <svg class="component-canvas-links" :viewBox="componentCanvasViewBox" aria-hidden="true">
                  <defs>
                    <marker id="component-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                      <path d="M 0 0 L 10 5 L 0 10 z" class="component-arrow-head" />
                    </marker>
                  </defs>
                  <path
                    v-for="edge in componentCanvasEdges"
                    :key="`${edge.fromKey || edge.fromId}-${edge.toKey || edge.toId}`"
                    class="component-canvas-link"
                    :class="{ active: componentEdgeHighlighted(edge) }"
                    :d="componentEdgePath(edge)"
                  />
                  <path
                    v-if="connectionDrag"
                    class="connection-drag-line"
                    :d="`M ${connectionDrag.startX} ${connectionDrag.startY} L ${connectionDrag.currentX} ${connectionDrag.currentY}`"
                    stroke="#3b82f6"
                    stroke-width="2"
                    stroke-dasharray="5,5"
                    fill="none"
                    marker-end="url(#component-arrow)"
                  />
                  <rect
                    v-if="marqueeRect"
                    class="topology-marquee"
                    :x="marqueeRect.x"
                    :y="marqueeRect.y"
                    :width="marqueeRect.width"
                    :height="marqueeRect.height"
                  />
                </svg>
                <div v-if="componentCanvasNodes.length === 0" class="component-canvas-empty-hint">
                  <strong>暂无组件</strong>
                  <span>在画布空白处右键创建前端、后端、数据库或中间件。</span>
                </div>
                <button
                  v-for="node in componentCanvasNodes"
                  :key="node.topologyId || node.id || node.name"
                  type="button"
                  class="component-canvas-node component-topology-node"
                  :class="{ active: componentNodeActive(node), selected: selectedNodeKeys.includes(String(node.topologyId || node.id)), 'component-topology-node--service': node.topologyKind === 'service' }"
                  :style="componentNodeStyle(node)"
                  :data-node-key="node.topologyId || node.id"
                  @click="handleTopologyNodeClick($event, node)"
                  @pointerdown="startComponentNodeDrag($event, node)"
                  @pointerup="finishComponentNodePointer($event, node)"
                  @contextmenu.stop.prevent="openTopologyContextMenu($event, node)"
                >
                  <span class="node-type-icon" :class="componentNodeIconClass(node)">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="componentNodeIconPath(node)" /></svg>
                  </span>
                  <span class="node-status" :class="node.status"></span>
                  <strong>{{ node.name }}</strong>
                  <small>{{ topologyNodeSubtitle(node) }}</small>
                  <span
                    class="node-delete-action"
                    role="button"
                    tabindex="0"
                    :title="topologyDeleteTitle(node)"
                    @click.stop="deleteTopologyNode(node)"
                    @keydown.enter.stop.prevent="deleteTopologyNode(node)"
                    @keydown.space.stop.prevent="deleteTopologyNode(node)"
                    @pointerdown.stop
                  >
                    <svg focusable="false" width="14" height="14" viewBox="0 0 32 32" fill="currentColor"><path d="M12 12h2v12h-2zm6 0h2v12h-2z"/><path d="M4 6v2h2v20c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8h2V6H4zm4 22V8h16v20H8zM12 2h8v2h-8z"/></svg>
                  </span>
                  <div
                    v-for="side in connectorSides"
                    :key="side"
                    class="node-connector"
                    :class="`node-connector--${side}`"
                    @pointerdown.stop="startConnectionDrag($event, node, side)"
                    title="拖拽到其他节点建立连接"
                  ></div>
                </button>
              </div>
            </div>
          </section>

          <section v-if="componentPanelTab === 'list'" class="component-table-panel">
            <div class="component-list-head">
              <div>
                <h2 class="component-map-title">组件列表</h2>
                <span class="component-count-label">{{ filteredComponents.length }} / {{ components.length }} 个组件</span>
              </div>
              <div class="component-filter-actions">
                <input id="component-search" v-model.trim="componentSearch" name="componentSearch" class="component-search-input" type="search" placeholder="搜索组件、镜像或仓库" />
                <select id="component-type-filter" v-model="componentTypeFilter" name="componentTypeFilter" class="component-type-select">
                  <option value="all">全部类型</option>
                  <option v-for="type in componentTypes" :key="type" :value="type">{{ compTypeText(type) }}</option>
                </select>
              </div>
            </div>
            <div class="component-table-wrap">
              <table class="component-table">
                <thead>
                  <tr>
                    <th>组件</th>
                    <th>类型</th>
                    <th>状态</th>
                    <th>副本</th>
                    <th>交付</th>
                    <th>目标</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="comp in filteredComponents"
                    :key="comp.id"
                    :class="{ active: selectedComponent?.id === comp.id }"
                    @click="selectComponent(comp.id)"
                  >
                    <td>
                      <strong>{{ comp.name }}</strong>
                      <small>{{ comp.identifier || comp.name }}</small>
                    </td>
                    <td>{{ compTypeText(comp.type) }}</td>
                    <td><span class="status-dot" :class="comp.status"></span>{{ comp.status || 'unknown' }}</td>
                    <td>{{ comp.replicas || 0 }}</td>
                    <td>{{ componentDeliveryModeLabel(comp) }}</td>
                    <td class="component-target-cell">{{ componentDeliveryTarget(comp) }}</td>
                    <td>
                      <div class="component-row-actions">
                        <button type="button" class="text-btn" @click.stop="openComponentConfigDrawer(comp)">配置</button>
                        <button type="button" class="text-btn" @click.stop="deployComponent(comp)">部署</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
              <div v-if="filteredComponents.length === 0" class="component-table-empty">没有匹配的组件。</div>
            </div>
          </section>
        </div>
      </div>

      </main>
    </div>

    <!-- Create Component Modal -->
    <Teleport to="body">
      <div v-if="showComponentModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="showComponentModal=false">
        <div class="modal-container" style="max-width:600px">
          <div class="modal-header">
            <div>
              <p class="modal-label">组件草稿</p>
              <p class="modal-heading">新建组件草稿</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="showComponentModal=false">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <div class="bx--form-item">
              <label class="bx--label">组件名称</label>
              <input v-model="compForm.name" class="bx--text-input" placeholder="例如：前端服务" />
            </div>
            <div class="bx--form-item">
              <label class="bx--label">组件类型</label>
              <div class="bx--select">
                <select v-model="compForm.type" class="bx--select-input">
                  <option value="frontend">前端服务</option>
                  <option value="backend">后端服务</option>
                  <option value="database">数据库</option>
                  <option value="middleware">中间件</option>
                  <option value="custom">自定义</option>
                </select>
                <svg class="bx--select__arrow" width="10" height="5" viewBox="0 0 10 5"><path d="M0 0l5 4.999L10 0z" fill-rule="evenodd"/></svg>
              </div>
            </div>
            <div class="bx--form-item">
              <label class="bx--label">交付方式</label>
              <div class="delivery-switch" role="radiogroup" aria-label="交付方式">
                <label class="delivery-option" :class="{ active: compForm.deliveryMode === 'image' }">
                  <input v-model="compForm.deliveryMode" type="radio" value="image" />
                  <span>镜像交付</span>
                </label>
                <label class="delivery-option" :class="{ active: compForm.deliveryMode === 'source' }">
                  <input v-model="compForm.deliveryMode" type="radio" value="source" />
                  <span>源码交付</span>
                </label>
              </div>
            </div>
            <div v-if="compForm.deliveryMode === 'image'" class="bx--form-item">
              <label class="bx--label">镜像地址</label>
              <input v-model="compForm.image" class="bx--text-input" placeholder="registry.example.com/app:v1.0.0" />
            </div>
            <template v-else>
              <div class="bx--form-item">
                <label class="bx--label">源码仓库</label>
                <input v-model="compForm.sourceRepoUrl" class="bx--text-input" placeholder="https://git.example.com/team/app.git" />
              </div>
              <div class="form-row">
                <div class="bx--form-item">
                  <label class="bx--label">源码分支</label>
                  <input v-model="compForm.sourceBranch" class="bx--text-input" placeholder="main" />
                </div>
                <div class="bx--form-item">
                  <label class="bx--label">构建上下文</label>
                  <input v-model="compForm.buildContext" class="bx--text-input" placeholder="." />
                </div>
              </div>
            </template>
            <div class="bx--form-item">
              <label class="bx--label">版本标签</label>
              <input v-model="compForm.version" class="bx--text-input" :placeholder="compForm.deliveryMode === 'source' ? '源码交付可留空' : 'v1.0.0'" />
              <div class="bx--form__helper-text">{{ compForm.deliveryMode === 'source' ? '源码交付可留空，Jenkins 首次构建会使用构建号；填写时不能使用 latest。' : '必须使用明确版本，不能使用 latest。' }}</div>
            </div>
            <div class="bx--form-item">
              <label class="bx--label">副本数量</label>
              <input type="number" v-model="compForm.replicas" class="bx--text-input" min="1" />
            </div>
            <p v-if="componentModalError" class="modal-error" role="alert">{{ componentModalError }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" @click="showComponentModal=false">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" @click="submitComponent">保存草稿</button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="configDrawer.visible" class="config-drawer-shell" role="dialog" aria-modal="true" @click.self="closeConfigDrawer">
        <aside class="config-drawer">
          <header class="config-drawer-header">
            <div>
              <p class="modal-label">{{ configDrawer.kind === 'service' ? '服务配置' : '组件配置' }}</p>
              <h3>{{ configDrawerTitle }}</h3>
              <small>{{ configDrawerSubtitle }}</small>
            </div>
            <div class="config-drawer-header-actions">
              <button
                v-if="configDrawer.kind === 'component'"
                type="button"
                class="bx--btn bx--btn--primary bx--btn--sm"
                :disabled="componentActionLoading"
                @click="deployDrawerComponent"
              >
                {{ componentActionLoading ? '部署中...' : '部署' }}
              </button>
              <button type="button" class="modal-close" aria-label="关闭" @click="closeConfigDrawer">
                <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
              </button>
            </div>
          </header>

          <div class="config-drawer-body">
            <section class="config-section">
              <div class="config-section-title">
                <span>运行入口</span>
              </div>
              <div v-if="configDrawer.kind === 'service'" class="service-access-stack">
                <div class="service-access-row">
                  <span>环境内</span>
                  <code>{{ serviceDrawerInternalEndpoint || '等待生成' }}</code>
                </div>
                <div class="service-access-row service-access-row--action">
                  <span>外部访问</span>
                  <code v-if="configDrawerExternalUrl">{{ configDrawerExternalUrl }}</code>
                  <strong v-else>未开启</strong>
                  <a v-if="configDrawerExternalUrl" :href="configDrawerExternalUrl" target="_blank" rel="noreferrer" class="text-btn">打开</a>
                  <button
                    v-if="serviceDrawerExternalAccessToggleVisible"
                    type="button"
                    class="bx--btn bx--btn--secondary bx--btn--sm"
                    :disabled="serviceExternalAccessLoading"
                    @click="toggleServiceExternalAccess"
                  >
                    {{ serviceDrawerExternalAccessLabel }}
                  </button>
                </div>
              </div>
              <div v-else class="service-access-row">
                <span>外部入口</span>
                <code v-if="configDrawerExternalUrl">{{ configDrawerExternalUrl }}</code>
                <strong v-else>未暴露</strong>
                <a v-if="configDrawerExternalUrl" :href="configDrawerExternalUrl" target="_blank" rel="noreferrer" class="text-btn">打开</a>
              </div>
            </section>

            <section v-if="configDrawer.kind === 'service' && serviceDrawerProfile.showDeploymentConfig" class="config-section">
              <div class="config-section-title"><span>部署配置</span></div>
              <div v-if="serviceDrawerVisibleConfigFields.length" class="config-form-grid">
                <label v-for="field in serviceDrawerVisibleConfigFields" :key="field.key" :class="{ 'config-form-wide': field.control === 'text' }">
                  <span>{{ field.label }}</span>
                  <select v-if="field.control === 'select'" v-model="serviceConfigForm[field.key]" class="bx--select-input">
                    <option v-for="option in field.options || []" :key="String(option.value)" :value="option.value">{{ option.label }}</option>
                  </select>
                  <select v-else-if="field.control === 'switch'" v-model="serviceConfigForm[field.key]" class="bx--select-input">
                    <option :value="false">关闭</option>
                    <option :value="true">开启</option>
                  </select>
                  <input
                    v-else-if="field.control === 'number'"
                    v-model.number="serviceConfigForm[field.key]"
                    type="number"
                    :min="field.min"
                    :max="field.max"
                    class="bx--text-input"
                    :placeholder="field.placeholder"
                  />
                  <input v-else v-model.trim="serviceConfigForm[field.key]" class="bx--text-input" :placeholder="field.placeholder" />
                </label>
              </div>
              <div v-else class="config-empty">当前服务类型暂未开放可编辑部署参数。</div>
              <div class="config-kv-grid service-summary-grid">
                <div v-for="row in serviceDrawerPrimaryRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
            </section>

            <section v-if="configDrawer.kind === 'service' && serviceDrawerProfile.showConnectionBindings" class="config-section">
              <div class="config-section-title"><span>连接参数</span></div>
              <div class="config-readonly-list">
                <div v-for="binding in serviceDrawerConnectionPreview.bindings" :key="binding.name" class="config-readonly-row">
                  <strong>{{ binding.name }}</strong>
                  <span>{{ binding.value }}</span>
                </div>
              </div>
            </section>

            <section v-if="configDrawer.kind === 'service' && serviceDrawerProfile.showTopology" class="config-section">
              <div class="config-section-title"><span>实例拓扑</span></div>
              <div v-if="serviceDrawerTopology.nodes.length" class="service-topology-list">
                <div class="service-topology-summary">
                  <span v-for="row in serviceDrawerTopology.summaryRows" :key="row.label">{{ row.label }} {{ row.value }}</span>
                </div>
                <div v-for="node in serviceDrawerTopology.nodes" :key="node.name" class="service-topology-node-row">
                  <strong>{{ node.name }}</strong>
                  <span>{{ node.role }}</span>
                  <span>{{ node.status }}</span>
                  <code>{{ node.address || '未采集地址' }}</code>
                  <small v-if="node.detail">{{ node.detail }}</small>
                </div>
              </div>
              <div v-else class="config-empty">{{ serviceDrawerWorkspaceLoading ? '拓扑采集中...' : '未采集到实例拓扑。' }}</div>
            </section>

            <section v-if="configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title">
                <span>镜像来源</span>
                <button type="button" class="text-btn" :disabled="registryWorkspaceLoading" @click="ensureRegistryWorkspaces">
                  {{ registryWorkspaceLoading ? '刷新中...' : '刷新' }}
                </button>
              </div>
              <div class="cds-image-fields">
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-repo">仓库</label>
                  <input
                    id="drawer-repo"
                    v-model.trim="configForm.repository"
                    class="cds-text-input"
                    list="component-registry-repositories"
                    placeholder="选择或输入仓库名"
                    @change="syncConfigVersionFromRepository"
                  />
                  <datalist id="component-registry-repositories">
                    <option v-for="repo in registryImageRepositories" :key="repo.repository" :value="repo.repository">{{ registryRepositoryDisplayName(repo.repository) }}</option>
                  </datalist>
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-tag">镜像:Tag</label>
                  <input
                    id="drawer-tag"
                    v-model.trim="configForm.version"
                    class="cds-text-input"
                    list="component-registry-tags"
                    placeholder="例如 v1.0.0"
                  />
                  <datalist id="component-registry-tags">
                    <option v-for="tag in selectedRegistryRepositoryTags" :key="tag" :value="tag">{{ tag }}</option>
                  </datalist>
                </div>
              </div>
              <div class="cds-image-preview" v-if="configForm.repository && configForm.version">
                <svg class="cds-image-preview__icon" width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm0 12.5a5.5 5.5 0 1 1 0-11 5.5 5.5 0 0 1 0 11zM8 4a.75.75 0 0 1 .75.75v2.5h2.5a.75.75 0 0 1 0 1.5h-2.5v2.5a.75.75 0 0 1-1.5 0v-2.5h-2.5a.75.75 0 0 1 0-1.5h2.5v-2.5A.75.75 0 0 1 8 4z"/></svg>
                <code class="cds-image-preview__text">{{ registryHostForDrawer || '环境镜像仓库' }}/{{ configForm.repository }}:{{ configForm.version }}</code>
              </div>
              <p class="cds-helper-text">左侧填写仓库名，右侧填写镜像 Tag；没有扫描到仓库时也可以直接填写仓库名和 Tag。</p>
              <p v-if="registryWorkspaceError" class="modal-error" role="alert">{{ registryWorkspaceError }}</p>
            </section>

            <section v-if="configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title"><span>规格</span></div>
              <div class="config-form-grid">
                <label>
                  <span>副本</span>
                  <input v-model.number="configForm.replicas" type="number" min="0" class="bx--text-input" />
                </label>
                <label>
                  <span>CPU</span>
                  <input v-model.trim="configForm.cpu" class="bx--text-input" placeholder="100m" />
                </label>
                <label>
                  <span>内存</span>
                  <input v-model.trim="configForm.memory" class="bx--text-input" placeholder="128Mi" />
                </label>
              </div>
            </section>

            <section class="config-section">
              <div class="config-section-title">
                <span>环境变量</span>
                <button v-if="configDrawer.kind === 'component'" type="button" class="text-btn" @click="addConfigEnv">添加</button>
              </div>
              <div v-if="configDrawer.kind === 'component'" class="config-env-list">
                <div v-for="(envItem, idx) in configForm.env" :key="idx" class="config-env-row">
                  <input v-model.trim="envItem.name" class="bx--text-input" placeholder="NAME" />
                  <select v-model="envItem.source" class="bx--select-input">
                    <option value="value">值</option>
                    <option value="configMap">ConfigMap</option>
                    <option value="secret">Secret</option>
                  </select>
                  <input v-if="envItem.source === 'value'" v-model="envItem.value" class="bx--text-input" placeholder="value" />
                  <template v-else>
                    <input v-model.trim="envItem.refName" class="bx--text-input" :placeholder="envItem.source === 'secret' ? 'secret name' : 'configmap name'" />
                    <input v-model.trim="envItem.refKey" class="bx--text-input" placeholder="key" />
                  </template>
                  <button type="button" class="text-btn danger" @click="removeConfigEnv(idx)">删除</button>
                </div>
                <div v-if="!configForm.env.length" class="config-empty">当前没有显式环境变量。</div>
              </div>
              <div v-else class="config-readonly-list">
                <div v-for="envItem in drawerRuntime.env || []" :key="envItem.name" class="config-readonly-row">
                  <strong>{{ envItem.name }}</strong>
                  <span>{{ runtimeEnvValue(envItem) }}</span>
                </div>
                <div v-if="!(drawerRuntime.env || []).length" class="config-empty">当前未发现显式环境变量。</div>
              </div>
            </section>

            <section class="config-section">
              <div class="config-section-title"><span>ConfigMap / Secret 引用</span></div>
              <div class="config-ref-grid">
                <div>
                  <span>ConfigMap</span>
                  <strong>{{ runtimeObjectSummary(drawerRuntime.configMaps) }}</strong>
                </div>
                <div>
                  <span>Secret</span>
                  <strong>{{ runtimeObjectSummary(drawerRuntime.secrets) }}</strong>
                </div>
                <div>
                  <span>envFrom</span>
                  <strong>{{ runtimeEnvFromSummary(drawerRuntime.envFrom) }}</strong>
                </div>
              </div>
            </section>

            <section v-if="configDrawer.kind === 'component'" class="config-section">
              <details>
                <summary>高级启动配置</summary>
                <label class="config-stack-field">
                  <span>Command</span>
                  <textarea v-model="configForm.commandText" class="bx--text-input" rows="2" placeholder="每行一个 command 片段"></textarea>
                </label>
                <label class="config-stack-field">
                  <span>Args</span>
                  <textarea v-model="configForm.argsText" class="bx--text-input" rows="3" placeholder="每行一个参数"></textarea>
                </label>
              </details>
            </section>

            <section class="config-section">
              <details>
                <summary>运行详情</summary>
                <div class="config-kv-grid">
                  <div v-for="row in serviceDrawerRuntimeRows" :key="row.label">
                    <span>{{ row.label }}</span>
                    <strong>{{ row.value }}</strong>
                  </div>
                </div>
              </details>
            </section>

            <p v-if="configDrawer.error" class="modal-error" role="alert">{{ configDrawer.error }}</p>
          </div>

          <footer class="config-drawer-footer">
            <button v-if="configDrawer.kind === 'component'" type="button" class="text-btn danger" :disabled="configDrawer.saving" @click="deleteDrawerComponent">删除组件</button>
            <button v-if="configDrawer.kind === 'service'" type="button" class="text-btn danger" :disabled="uninstallSubmitting" @click="deleteDrawerService">删除卡片</button>
            <button type="button" class="bx--btn bx--btn--secondary" @click="closeConfigDrawer">取消</button>
            <button v-if="configDrawer.kind === 'component'" type="button" class="bx--btn bx--btn--primary" :disabled="configDrawer.saving" @click="() => saveConfigDrawer()">
              {{ configDrawer.saving ? '保存中...' : '保存配置' }}
            </button>
            <button v-if="configDrawer.kind === 'service' && serviceDrawerProfile.showDeploymentConfig" type="button" class="bx--btn bx--btn--secondary" :disabled="configDrawer.saving || !serviceDrawerConfigurable" @click="saveServiceConfigDrawer">
              {{ configDrawer.saving ? '保存中...' : '保存配置' }}
            </button>
            <button v-if="configDrawer.kind === 'service'" type="button" class="bx--btn bx--btn--primary" :disabled="serviceDrawerDeployDisabled" @click="deployServiceFromDrawer">
              {{ serviceDrawerDeployLabel }}
            </button>
          </footer>
        </aside>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="componentContextMenu.visible"
        class="component-context-menu"
        :style="{ left: `${componentContextMenu.x}px`, top: `${componentContextMenu.y}px` }"
        @pointerdown.stop
        @click.stop
      >
        <button v-if="componentContextMenu.kind === 'canvas'" type="button" @mouseenter="openComponentSubmenu" @click="openComponentSubmenu">
          <span>创建组件</span>
          <small>前端、后端、自定义组件</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <button v-if="componentContextMenu.kind === 'canvas'" type="button" @mouseenter="openToolSubmenu" @click="openToolSubmenu">
          <span>添加工具</span>
          <small>Git、CI/CD、监控、日志工具</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <button v-if="componentContextMenu.kind === 'canvas'" type="button" @mouseenter="openInfraSubmenu" @click="openInfraSubmenu">
          <span>添加中间件</span>
          <small>数据库、缓存、消息队列</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <div v-if="componentContextMenu.kind === 'canvas'" class="context-menu-divider"></div>
        <button v-if="componentContextMenu.kind === 'canvas'" type="button" @click="adoptCanvasResource">
          <span>纳管已有资源</span>
          <small>接入集群现有资源</small>
        </button>
        <button v-if="componentContextMenu.kind !== 'canvas'" type="button" @click="configureContextNode">
          <span>配置</span>
          <small>{{ componentContextMenu.kind === 'service' ? '在右侧查看工具或中间件配置' : '在右侧配置环境变量、副本和启动参数' }}</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component'" type="button" @click="deployContextComponent">
          <span>部署</span>
          <small>提交 GitOps 并同步</small>
        </button>
        <button v-if="componentContextMenu.kind === 'service'" type="button" @click="deployContextService">
          <span>部署</span>
          <small>部署或应用当前服务配置</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component'" type="button" @click="deleteContextComponent">
          <span>删除</span>
          <small>删除组件草稿和运行态 CR</small>
        </button>
        <button v-if="componentContextMenu.kind === 'service'" type="button" @click="deleteContextService">
          <span>删除</span>
          <small>卸载工具、中间件或数据库并清理卡片</small>
        </button>
        <button v-if="componentContextMenu.kind !== 'canvas'" type="button" @click="openContextNodeMonitoring">
          <span>监控</span>
          <small>打开监控中心</small>
        </button>
        <button v-if="componentContextMenu.kind !== 'canvas'" type="button" @click="openContextNodeLogs">
          <span>日志</span>
          <small>打开日志中心</small>
        </button>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="contextSubmenu.visible"
        class="component-context-menu context-submenu"
        :style="{ left: `${contextSubmenu.x}px`, top: `${contextSubmenu.y}px` }"
        @pointerdown.stop
        @click.stop
      >
        <button v-for="tmpl in contextSubmenu.templates" :key="tmpl.type" type="button" :class="{ disabled: tmpl.disabled }" @click="selectSubmenuTemplate(tmpl)">
          <span>{{ tmpl.label }}</span>
          <small>{{ tmpl.disabled ? (tmpl.statusText || '已添加') : (tmpl.description || tmpl.type) }}</small>
        </button>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showAdoptResourceModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeAdoptResourceModal">
        <div class="modal-container relationship-modal">
          <div class="modal-header">
            <div>
              <p class="modal-label">接入资源</p>
              <p class="modal-heading">接入已有资源</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="closeAdoptResourceModal">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <p class="relationship-help">只列出当前环境命名空间内尚未被 PAAP 管理的真实工作负载。接入后会生成组件草稿，不会立即部署。</p>
            <p v-if="adoptResourceLoading" class="no-data">正在读取可接入资源...</p>
            <div v-else-if="adoptableResources.length" class="relationship-target-list">
              <label v-for="resource in adoptableResources" :key="resource.key" class="relationship-target">
                <input v-model="adoptResourceSelection" type="radio" name="adoptResourceSelection" :value="resource.key" />
                <span>
                  <strong>{{ resource.name }}</strong>
                  <small>{{ resource.kind }} · {{ resource.namespace }} · {{ compTypeText(resource.componentType) }}</small>
                </span>
              </label>
            </div>
            <p v-else class="no-data">当前没有可接入的真实资源。</p>
            <p v-if="adoptResourceError" class="modal-error" role="alert">{{ adoptResourceError }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" :disabled="adoptResourceSubmitting" @click="closeAdoptResourceModal">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" :disabled="adoptResourceLoading || adoptResourceSubmitting || !adoptResourceSelection" @click="submitAdoptResource">
              {{ adoptResourceSubmitting ? '接入中...' : '接入' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showRelationshipModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeRelationshipModal">
        <div class="modal-container relationship-modal">
          <div class="modal-header">
            <div>
              <p class="modal-label">组件关系</p>
              <p class="modal-heading">连接 {{ relationshipSourceComponent?.name || '组件' }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="closeRelationshipModal">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <p class="relationship-help">选择当前组件依赖的组件或中间件。保存后会写入组件运行配置，不会触发部署。</p>
            <div v-if="relationshipTargets.length" class="relationship-target-list">
              <label v-for="target in relationshipTargets" :key="target.key" class="relationship-target">
                <input v-model="relationshipSelectedKeys" type="checkbox" :value="target.key" />
                <span>
                  <strong>{{ target.name }}</strong>
                  <small>{{ target.kind }} · {{ target.type }}</small>
                </span>
              </label>
            </div>
            <p v-else class="no-data">当前没有可连接的组件或中间件。</p>
            <p v-if="relationshipError" class="modal-error" role="alert">{{ relationshipError }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" :disabled="relationshipSubmitting" @click="closeRelationshipModal">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" :disabled="relationshipSubmitting" @click="saveComponentRelationships">
              {{ relationshipSubmitting ? '保存中...' : '保存关系' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Install Service Modal -->
    <Teleport to="body">
      <div v-if="showServiceModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="showServiceModal=false">
        <div class="modal-container" :style="{ maxWidth: serviceModalMode === 'tool' ? '720px' : '600px' }">
          <div class="modal-header">
            <div>
              <p class="modal-label">{{ serviceModalMode === 'infra' ? '安装中间件' : '安装工具' }}</p>
              <p class="modal-heading">{{ serviceModalMode === 'infra' ? '选择要安装的中间件' : '选择要安装的工具' }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="showServiceModal=false">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <p v-if="serviceModalLoading" class="bx--type-body-short-01 no-data">模板加载中...</p>
            <p v-else-if="serviceModalError" class="bx--type-body-short-01 no-data error-text">{{ serviceModalError }}</p>
            <p v-else-if="serviceModalNotice" class="bx--type-body-short-01 no-data">{{ serviceModalNotice }}</p>
            <div v-else class="service-picker-summary">
              <span class="summary-pill">{{ serviceModalMode === 'infra' ? '中间件' : '工具' }}</span>
              <span class="summary-text">可添加 {{ selectableServiceCount }} 个，已添加、已安装或正在安装的模板会显示为不可选状态。</span>
            </div>
            <div class="service-select-grid">
              <div v-for="svc in availableServices" :key="svc.type"
                   :class="['service-select-card', {selected: serviceForm.serviceType===svc.type, disabled: svc.disabled}]"
                   @click="selectServiceTemplate(svc)">
                <div class="select-radio" :class="{selected: serviceForm.serviceType===svc.type, disabled: svc.disabled}">
                  <svg v-if="serviceForm.serviceType===svc.type && !svc.disabled" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 10.2L3.4 7.6 2 9l4 4 8-8-1.4-1.4z"/></svg>
                </div>
                <div>
                  <div class="service-name-row">
                    <h4 class="bx--type-productive-heading-02 service-name">{{ svc.name }}</h4>
                    <span class="bx--tag bx--tag--sm" :class="svc.disabled ? 'bx--tag--gray' : 'bx--tag--green'">
                      {{ svc.disabled ? (svc.statusText || '已添加') : '可添加' }}
                    </span>
                  </div>
                  <p class="bx--type-body-short-01 service-desc">{{ svc.description || '无描述' }}</p>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" @click="showServiceModal=false">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" :disabled="serviceModalLoading || serviceSubmitting || !serviceForm.serviceType" @click="submitService">{{ serviceSubmitting ? '安装中...' : '安装' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Uninstall Service Confirmation -->
    <Teleport to="body">
      <div v-if="pendingUninstallService" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeUninstallDialog">
        <div class="modal-container confirm-modal">
          <div class="modal-header">
            <div>
              <p class="modal-label">卸载服务</p>
              <p class="modal-heading">确认卸载 {{ svcLabel(pendingUninstallService.serviceType) }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="closeUninstallDialog">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <p class="confirm-text">卸载后，该服务的业务管理入口将从当前环境中移除。请确认不再需要这个服务。</p>
            <p v-if="uninstallError" class="modal-error" role="alert">{{ uninstallError }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" :disabled="uninstallSubmitting" @click="closeUninstallDialog">取消</button>
            <button type="button" class="bx--btn bx--btn--danger" :disabled="uninstallSubmitting" @click="confirmUninstallService">
              {{ uninstallSubmitting ? '卸载中...' : '确认卸载' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="pendingDeleteDialog" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeDeleteDialog">
        <div class="modal-container confirm-modal">
          <div class="modal-header">
            <div>
              <p class="modal-label">删除{{ pendingDeleteDialog.label }}</p>
              <p class="modal-heading">确认删除 {{ pendingDeleteDialog.name }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="closeDeleteDialog">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <p class="confirm-text">{{ pendingDeleteDialog.message }}</p>
            <p v-if="pendingDeleteDialog.error" class="modal-error" role="alert">{{ pendingDeleteDialog.error }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" :disabled="pendingDeleteDialog.submitting" @click="closeDeleteDialog">取消</button>
            <button type="button" class="bx--btn bx--btn--danger" :disabled="pendingDeleteDialog.submitting" @click="runPendingDelete">
              {{ pendingDeleteDialog.submitting ? '删除中...' : '确认删除' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="activeCapabilityAction" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeCapabilityActionDialog">
        <div class="modal-container capability-action-modal">
          <div class="modal-header">
            <div>
              <p class="modal-label">执行操作</p>
              <p class="modal-heading">{{ activeCapabilityAction.label }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" :disabled="capabilityWorkspaceLoading" @click="closeCapabilityActionDialog">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content capability-action-content">
            <p v-if="activeCapabilityAction.description" class="confirm-text">{{ activeCapabilityAction.description }}</p>
            <label v-for="field in activeCapabilityAction.fields || []" :key="field.name" class="config-stack-field capability-action-field">
              <span>{{ field.label }}</span>
              <textarea
                v-if="field.type === 'textarea'"
                v-model="activeCapabilityActionParams[field.name]"
                class="bx--text-input capability-action-textarea"
                :placeholder="field.placeholder"
                :required="field.required"
              />
              <input
                v-else-if="field.type === 'checkbox'"
                class="capability-action-checkbox"
                type="checkbox"
                :checked="activeCapabilityActionParams[field.name] === 'true'"
                @change="setCapabilityActionCheckboxParam(field.name, $event)"
              />
              <input
                v-else
                v-model="activeCapabilityActionParams[field.name]"
                class="bx--text-input"
                :type="field.type === 'number' ? 'number' : 'text'"
                :placeholder="field.placeholder"
                :required="field.required"
              />
            </label>
            <p v-if="capabilityWorkspaceError" class="modal-error" role="alert">{{ capabilityWorkspaceError }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" :disabled="capabilityWorkspaceLoading" @click="closeCapabilityActionDialog">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" :disabled="capabilityWorkspaceLoading" @click="submitCapabilityActionDialog">
              {{ capabilityWorkspaceLoading ? '执行中...' : '执行' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
const router = useRouter()
import { api } from '../api/client'
import { validateWorkspaceActionParams, type ServiceWorkspace, type WorkspaceAction, type WorkspaceResource } from './serviceWorkspace'
import {
  connectionBindingPreview,
  serviceConfigFieldVisible,
  serviceConfigFormFromInstallation,
  serviceConfigProfile,
  serviceConfigPrimaryRows,
  serviceInternalEndpoint,
  serviceTopologyFromWorkspace,
  serviceConfigValuesFromForm,
  serviceRuntimeDetailRows,
  type ServiceConfigForm,
} from './serviceAssetConfig'
import { numericRouteParam, routeEnvironmentKey } from './envDetailRouteState'
import { shouldPollTemplateInstallations, TEMPLATE_INSTALL_POLL_INTERVAL_MS, TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS } from './envInstallPolling'
import { buildPickerTemplates, createPickerSessionState, isServiceActive, pickerNotice } from './envDetailServicePicker'
import { buildCapabilityTabs, capabilityServiceInstanceLabel, knownCapabilityTabKeys, serviceCapability as resolveServiceCapability, serviceCategory as resolveServiceCategory, type CapabilityCategory, type CapabilityTab } from './envCapabilities'
import {
  buildComponentTopologyNodes,
  componentTopologyCanvasViewBox,
  componentTopologyEdgePath,
  findTopologyNodeAtPoint,
  hasComponentTopologyDragMoved,
  isNodeInMarquee,
  nextComponentTopologyDragPosition,
  nodeKey,
  parseComponentTopologyManualEdges,
  parseComponentTopologyPositions,
  serializeComponentTopologyManualEdges,
  serializeComponentTopologyPositions,
} from './componentTopology'
import MonitorWorkspace from '../components/workspaces/MonitorWorkspace.vue'
import ArgocdWorkspace from '../components/workspaces/ArgocdWorkspace.vue'
import GiteaWorkspace from '../components/workspaces/GiteaWorkspace.vue'
import LogWorkspace from '../components/workspaces/LogWorkspace.vue'
import PipelineWorkspace from '../components/workspaces/PipelineWorkspace.vue'
import DatabaseWorkspace from '../components/workspaces/DatabaseWorkspace.vue'
import RedisWorkspace from '../components/workspaces/RedisWorkspace.vue'
import MongoWorkspace from '../components/workspaces/MongoWorkspace.vue'
import RabbitWorkspace from '../components/workspaces/RabbitWorkspace.vue'
import KafkaWorkspace from '../components/workspaces/KafkaWorkspace.vue'
import MinioWorkspace from '../components/workspaces/MinioWorkspace.vue'
import RegistryWorkspace from '../components/workspaces/RegistryWorkspace.vue'

const route = useRoute()
const appId = computed(() => numericRouteParam(route.params.id))
const envId = computed(() => numericRouteParam(route.params.envId))
const envRouteKey = computed(() => routeEnvironmentKey(appId.value, envId.value))

const env = ref<any>(null)
const app = ref<any>(null)
const components = ref<any[]>([])
const services = ref<any[]>([])
const templates = ref<any[]>([])
const availableServices = ref<any[]>([])
const externalAccess = ref<any[]>([])

const showComponentModal = ref(false)
const showServiceModal = ref(false)
const serviceModalLoading = ref(false)
const serviceModalError = ref('')
const serviceModalNotice = ref('')
const serviceModalMode = ref<'tool' | 'infra'>('tool')
const serviceSubmitting = ref(false)
const defaultComponentForm = () => ({
  name: '',
  type: 'backend',
  deliveryMode: 'image' as 'image' | 'source',
  image: '',
  version: 'v1.0.0',
  replicas: 1,
  sourceRepoUrl: '',
  sourceBranch: 'main',
  buildContext: '.',
  dockerfilePath: '',
})
const compForm = ref(defaultComponentForm())
const componentModalError = ref('')
const pageError = ref('')
const pendingUninstallService = ref<any>(null)
const uninstallError = ref('')
const uninstallSubmitting = ref(false)
const serviceForm = ref({ serviceType:'deploy' })
const activeCapabilityServiceId = ref<number | null>(null)
const capabilityWorkspaceCache = ref<Record<number, ServiceWorkspace>>({})
const capabilityWorkspaceLoading = ref(false)
const capabilityWorkspaceError = ref('')
const capabilityWorkspaceMessage = ref('')
const capabilityInitialSubjectKey = ref('')
const activeCapabilityAction = ref<WorkspaceAction | null>(null)
const activeCapabilityActionTarget = ref<string | undefined>(undefined)
const activeCapabilityActionParams = ref<Record<string, string>>({})
const emptyCapabilityWorkspace: ServiceWorkspace = {
  kind: 'generic',
  title: '',
  description: '',
  actions: [],
  resources: [],
  config: [],
}
const componentSearch = ref('')
const componentTypeFilter = ref('all')
const overviewPanelTab = ref<'topology' | 'details'>('topology')
const componentPanelTab = ref<'topology' | 'list'>('topology')
const canvasZoom = ref(1)
const zoomIn = () => { canvasZoom.value = Math.min(2, canvasZoom.value + 0.1) }
const zoomOut = () => { canvasZoom.value = Math.max(0.5, canvasZoom.value - 0.1) }
const resetZoom = () => { canvasZoom.value = 1 }
const connectorSides = ['top', 'right', 'bottom', 'left'] as const
const selectedComponentId = ref<number | null>(null)
const selectedTopologyKey = ref<string | null>(null)
const coreToolsSection = ref<HTMLElement | null>(null)
const componentActionLoading = ref(false)
const componentContextMenu = ref<{ visible: boolean; x: number; y: number; kind: 'component' | 'service' | 'canvas'; component: any | null; service: any | null }>({
  visible: false,
  x: 0,
  y: 0,
  kind: 'component',
  component: null,
  service: null,
})
const contextSubmenu = ref<{ visible: boolean; x: number; y: number; mode: 'component' | 'tool' | 'infra'; templates: any[] }>({
  visible: false,
  x: 0,
  y: 0,
  mode: 'tool',
  templates: [],
})
const componentNodePositions = ref<Record<string, { x: number; y: number }>>({})
const manualCanvasEdges = ref<Array<{ fromKey: string; toKey: string }>>([])
const canvasCreatePoint = ref<{ x: number; y: number } | null>(null)
const componentNodeDrag = ref<{ keys: string[]; origins: Record<string, { x: number; y: number }>; startX: number; startY: number; moved: boolean; lastX: number; lastY: number } | null>(null)
const connectionDrag = ref<{ fromNode: any; startX: number; startY: number; currentX: number; currentY: number; stageEl: HTMLElement } | null>(null)
const suppressNextTopologyClick = ref(false)
const suppressTopologyClickKeys = ref<string[]>([])
const recentTopologyDrag = ref<{ key: string; at: number } | null>(null)
const selectedNodeKeys = ref<string[]>([])
const marqueeRect = ref<{ x: number; y: number; width: number; height: number } | null>(null)
const marqueeDrag = ref<{ startCanvasX: number; startCanvasY: number; currentCanvasX: number; currentCanvasY: number; stageEl: HTMLElement } | null>(null)
const defaultConfigForm = () => ({
  image: '',
  repository: '',
  version: '',
  replicas: 1,
  cpu: '',
  memory: '',
  env: [] as Array<{ name: string; source: 'value' | 'configMap' | 'secret'; value: string; refName: string; refKey: string }>,
  commandText: '',
  argsText: '',
})
const configForm = ref(defaultConfigForm())
const defaultServiceConfigForm = (): ServiceConfigForm => ({})
const serviceConfigForm = ref<ServiceConfigForm>(defaultServiceConfigForm())
const serviceDrawerWorkspaceLoading = ref(false)
const serviceExternalAccessLoading = ref(false)
const configDrawer = ref<{ visible: boolean; kind: 'component' | 'service'; component: any | null; service: any | null; saving: boolean; error: string }>({
  visible: false,
  kind: 'component',
  component: null,
  service: null,
  saving: false,
  error: '',
})
const registryWorkspaceLoading = ref(false)
const registryWorkspaceError = ref('')
type RegistryRepositoryOption = { repository: string; tags: string[]; resource: WorkspaceResource }
type PendingDeleteDialog = {
  kind: 'component'
  label: string
  name: string
  message: string
  target: any
  submitting: boolean
  error: string
}
const pendingDeleteDialog = ref<PendingDeleteDialog | null>(null)
const showRelationshipModal = ref(false)
const relationshipSourceComponent = ref<any | null>(null)
const relationshipSelectedKeys = ref<string[]>([])
const relationshipSubmitting = ref(false)
const relationshipError = ref('')
const showAdoptResourceModal = ref(false)
const adoptableResources = ref<any[]>([])
const adoptResourceSelection = ref('')
const adoptResourceLoading = ref(false)
const adoptResourceSubmitting = ref(false)
const adoptResourceError = ref('')

const initialEnvTab = () => {
  const tab = String(route.query.tab || '').trim()
  return tab || 'overview'
}
const activeTab = ref<string>(initialEnvTab())

const baseTabs = computed(() => [
  { key: 'overview', label: '概览', count: 0 },
  { key: 'components', label: '组件', count: components.value.length },
])

const tabs = computed(() => [
  ...baseTabs.value,
  ...capabilityTabs.value,
])

const replaceEnvTab = (tab: string) => {
  if (route.query.tab === tab) return
  router.replace({ query: { ...route.query, tab } })
}

const setActiveTab = (tab: string) => {
  const normalized = normalizeEnvTab(tab)
  activeTab.value = normalized
  replaceEnvTab(normalized)
}

const normalizeEnvTab = (tab: string) => {
  if (tab === 'overview' || tab === 'components') return tab
  if (tab === 'tools') return firstCapabilityTab('tool')?.key || 'overview'
  if (tab === 'infra') return firstCapabilityTab('infra')?.key || 'overview'
  if (knownCapabilityTabKeys.has(tab)) {
    return capabilityTabs.value.some(item => item.key === tab) ? tab : 'overview'
  }
  return tabs.value.some(item => item.key === tab) ? tab : 'overview'
}

const firstCapabilityTab = (category?: CapabilityCategory) =>
  capabilityTabs.value.find(tab => !category || tab.category === category)

const setFirstCapabilityTab = (category?: CapabilityCategory) => {
  const tab = firstCapabilityTab(category)
  if (tab) setActiveTab(tab.key)
  else setActiveTab('overview')
}

const scrollToCoreTools = async () => {
  setActiveTab('overview')
  await nextTick()
  coreToolsSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

watch(() => route.query.tab, () => {
  const normalized = normalizeEnvTab(initialEnvTab())
  if (activeTab.value !== normalized) activeTab.value = normalized
})

const handleDocumentPointerDown = () => closeComponentContextMenu()
const handleDocumentKeyDown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') closeComponentContextMenu()
}

const resetEnvironmentState = () => {
  env.value = null
  app.value = null
  components.value = []
  services.value = []
  externalAccess.value = []
  activeCapabilityServiceId.value = null
  capabilityWorkspaceCache.value = {}
  capabilityWorkspaceLoading.value = false
  capabilityWorkspaceError.value = ''
  capabilityWorkspaceMessage.value = ''
  capabilityInitialSubjectKey.value = ''
  activeTab.value = 'overview'
  pageError.value = ''
  selectedComponentId.value = null
  selectedTopologyKey.value = null
  componentPanelTab.value = 'topology'
  closeComponentContextMenu()
  capabilityWorkspaceLoadSeq++
}

let environmentLoadSeq = 0
let capabilityWorkspaceLoadSeq = 0
let templateInstallPollTimer: number | null = null
let templateInstallPollAttempts = 0

const stopTemplateInstallPolling = () => {
  if (templateInstallPollTimer) window.clearTimeout(templateInstallPollTimer)
  templateInstallPollTimer = null
  templateInstallPollAttempts = 0
}

const scheduleTemplateInstallPolling = () => {
  if (templateInstallPollTimer || !shouldPollTemplateInstallations(env.value, services.value)) return
  if (templateInstallPollAttempts >= TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS) return
  templateInstallPollAttempts += 1
  templateInstallPollTimer = window.setTimeout(async () => {
    templateInstallPollTimer = null
    try {
      await refreshServices()
    } finally {
      if (shouldPollTemplateInstallations(env.value, services.value)) {
        scheduleTemplateInstallPolling()
      } else {
        stopTemplateInstallPolling()
      }
    }
  }, TEMPLATE_INSTALL_POLL_INTERVAL_MS)
}

const loadEnvironmentDetail = async () => {
  const seq = ++environmentLoadSeq
  const targetAppId = appId.value
  const targetEnvId = envId.value
  stopTemplateInstallPolling()
  resetEnvironmentState()
  try {
    await loadComponentNodePositions()
    if (seq !== environmentLoadSeq) return
    const res = await api.getEnv(targetEnvId)
    if (seq !== environmentLoadSeq) return
    env.value = res.data.environment
    components.value = res.data.components || []
    services.value = res.data.services || []
    externalAccess.value = res.data.externalAccess || []
    const appRes = await api.getApp(targetAppId)
    if (seq !== environmentLoadSeq) return
    app.value = appRes.data?.application || appRes.data
    await loadServiceTemplates()
    if (seq !== environmentLoadSeq) return
    availableServices.value = filterTemplates(serviceModalMode.value)
    if (availableServices.value.length > 0) serviceForm.value.serviceType = availableServices.value[0].type
    activeTab.value = normalizeEnvTab(initialEnvTab())
    scheduleTemplateInstallPolling()
  } catch(e:any) {
    if (seq === environmentLoadSeq) pageError.value = '环境加载失败：' + (e?.message || '未知错误')
  }
}

onMounted(async () => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
  document.addEventListener('keydown', handleDocumentKeyDown)
  await loadEnvironmentDetail()
})

watch(envRouteKey, () => {
  void loadEnvironmentDetail()
})

onBeforeUnmount(() => {
  stopTemplateInstallPolling()
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
  document.removeEventListener('keydown', handleDocumentKeyDown)
})

const svcLabel = (type:string) => {
  const tmpl = templateFor(type)
  return tmpl?.name || type
}
const serviceIconPath = (type:string) => {
  const p: Record<string,string> = {
    deploy: 'M12 2l9 19H3L12 2z M12 8v6',
    git: 'M6 6a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm0 4v6a2 2 0 0 0 2 2h2a2 2 0 1 1 0 4 2 2 0 0 1 0-4H8a2 2 0 0 1-2-2v-6zm8-6a2 2 0 1 1 0 4 2 2 0 0 1 0-4z',
    log: 'M15.5 14h-.79l-.28-.27A6.47 6.47 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z',
    monitor: 'M3 14h4v-4H3v4zm6 0h4V7H9v7zm6 0h4v-9h-4v9z',
    registry: 'M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm0 16H5V5h14v14z',
    harbor: 'M12 2a3 3 0 0 0-3 3c0 1.5 1.1 2.8 2.5 3h-5v2h5v3H8v2h4.5v5c0 .6.4 1 1 1s1-.4 1-1v-5H18v-2h-4.5v-3h5V8h-5c1.4-.2 2.5-1.5 2.5-3a3 3 0 0 0-3-3z',
    ci: 'M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.488.488 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.484.484 0 0 0-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96a.488.488 0 0 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z',
    mysql: 'M12 2c-4.42 0-8 1.79-8 4v12c0 2.21 3.58 4 8 4s8-1.79 8-4V6c0-2.21-3.58-4-8-4zm0 2c3.31 0 6 1.34 6 3s-2.69 3-6 3-6-1.34-6-3 2.69-3 6-3z',
    postgresql: 'M12 2c-4.42 0-8 1.79-8 4v12c0 2.21 3.58 4 8 4s8-1.79 8-4V6c0-2.21-3.58-4-8-4zm0 2c3.31 0 6 1.34 6 3s-2.69 3-6 3-6-1.34-6-3 2.69-3 6-3z',
    mongodb: 'M12 2c-4.42 0-8 1.79-8 4v12c0 2.21 3.58 4 8 4s8-1.79 8-4V6c0-2.21-3.58-4-8-4zm0 2c3.31 0 6 1.34 6 3s-2.69 3-6 3-6-1.34-6-3 2.69-3 6-3z',
    redis: 'M12 2c-4.42 0-8 1.79-8 4v12c0 2.21 3.58 4 8 4s8-1.79 8-4V6c0-2.21-3.58-4-8-4zm0 2c3.31 0 6 1.34 6 3s-2.69 3-6 3-6-1.34-6-3 2.69-3 6-3z',
    rabbitmq: 'M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z',
    kafka: 'M4 15h2v-2H4v2zm4 0h2v-4H8v4zm4 0h2V7h-2v8zm4-8v8h2V7h-2z',
    minio: 'M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96z',
    infra: 'M4 5h16v4H4V5zm0 6h16v4H4v-4zm0 6h16v4H4v-4z',
    default: 'M12 2L3 7v10l9 5 9-5V7l-9-5zm0 2.2L18.8 8 12 11.8 5.2 8 12 4.2zM5 10.5l6 3.3v5.6l-6-3.3v-5.6zm14 0v5.6l-6 3.3v-5.6l6-3.3z'
  }
  return p[type] || p.default
}
const serviceStatusText = (status?: string) => ({ running: '运行中', installing: '安装中', failed: '安装失败', deleting: '删除中', pending: '未部署', error: '安装失败', draft: '未部署' }[String(status || '').toLowerCase()] || '未知')
const statusTagClass = (status?: string) => ({ running: 'bx--tag--green', installing: 'bx--tag--blue', failed: 'bx--tag--red', error: 'bx--tag--red', deleting: 'bx--tag--gray', pending: 'bx--tag--gray', draft: 'bx--tag--gray' }[String(status || '').toLowerCase()] || 'bx--tag--gray')
const serviceStatusValue = (svc:any) => String(svc?.status || '').toLowerCase()
const serviceStatusIsDraft = (svc:any) => ['draft', 'pending', ''].includes(serviceStatusValue(svc))
const serviceStatusHasRuntime = (svc:any) => ['running', 'installing'].includes(serviceStatusValue(svc))
const serviceStatusCanDeploy = (svc:any) => !['installing', 'deleting'].includes(serviceStatusValue(svc))
const typeLabel = (type:string) => {
  const labels: Record<string,string> = { deploy:'部署与持续交付', monitor:'监控与可观测性', log:'日志服务', ci:'持续集成', git:'代码仓库', registry:'轻量镜像仓库', harbor:'企业镜像仓库', postgresql:'关系型数据库', mysql:'关系型数据库', mongodb:'文档数据库', redis:'缓存服务', rabbitmq:'消息队列', kafka:'消息队列', minio:'对象存储', infra:'中间件', tool:'平台工具', custom:'自定义工具' }
  return labels[type] || type
}
const compTypeText = (type?:string) => ({ frontend:'前端服务', backend:'后端服务', database:'数据库', middleware:'中间件', custom:'自定义' }[type || ''] || type || 'custom')
const componentIsSourceDelivery = (comp:any) => comp?.deliveryMode === 'source' || Boolean(comp?.sourceRepoUrl || comp?.sourceMirrorRepoUrl || comp?.jenkinsJob)
const componentDeliveryModeLabel = (comp:any) => componentIsSourceDelivery(comp) ? '源码交付' : '镜像交付'
const componentDeliveryTarget = (comp:any) => comp?.registryImage || comp?.image || comp?.sourceRepoUrl || '-'
const appInfraServices = computed(() => services.value.filter((svc:any) => serviceCategory(svc) === 'infra'))
const componentTopologyAllNodes = computed(() => buildComponentTopologyNodes(components.value, appInfraServices.value))
const environmentTopologyAllNodes = computed(() => buildComponentTopologyNodes(components.value, services.value))
const componentTypes = computed(() =>
  Array.from(new Set(components.value.map((comp:any) => String(comp.type || 'custom')).filter(Boolean)))
)
const filteredComponents = computed(() => {
  const q = componentSearch.value.trim().toLowerCase()
  return components.value.filter((comp:any) => {
    const type = String(comp.type || 'custom')
    const text = [
      comp.name,
      comp.identifier,
      comp.image,
      comp.registryImage,
      comp.sourceRepoUrl,
      comp.sourceMirrorRepoUrl,
    ].filter(Boolean).join(' ').toLowerCase()
    const matchesSearch = !q || text.includes(q)
    const matchesType = componentTypeFilter.value === 'all' || type === componentTypeFilter.value
    return matchesSearch && matchesType
  })
})
const filteredTopologyNodes = computed(() => {
  const q = componentSearch.value.trim().toLowerCase()
  const visibleComponentIds = new Set(filteredComponents.value.map((comp:any) => `component:${comp.id}`))
  return componentTopologyAllNodes.value.filter((node:any) => {
    if (node.topologyKind === 'component') return visibleComponentIds.has(String(node.topologyId || ''))
    const text = [node.name, node.type, node.status].filter(Boolean).join(' ').toLowerCase()
    return !q || text.includes(q)
  })
})
const environmentTopologyNodeSubtitle = (node:any) => node?.topologyKind === 'service'
  ? `${typeLabel(node.type || node.serviceType || '')} · ${serviceCategory(node) === 'infra' ? '中间件/数据库' : '平台工具'}`
  : `${compTypeText(node.type)} · 应用组件`
const componentCanvasMetrics = {
  colWidth: 260,
  nodeWidth: 196,
  nodeHeight: 70,
  top: 48,
  left: 72,
  rowGap: 34,
}
const preferredTopologyDepth = (node:any) => {
  if (node?.topologyKind === 'service') return 2
  const type = String(node?.type || '').toLowerCase()
  if (type === 'frontend') return 0
  if (type === 'backend') return 1
  if (type === 'database' || type === 'middleware') return 2
  return 1
}
const layoutTopologyGraph = (nodes:any[], edges:any[]) => {
  const nodeKeys = new Set(nodes.map((node:any) => String(node.topologyId || node.id)))
  const incoming = new Map<string, number>()
  const outgoing = new Map<string, string[]>()
  for (const node of nodes) {
    const key = String(node.topologyId || node.id)
    incoming.set(key, 0)
    outgoing.set(key, [])
  }
  for (const edge of edges) {
    const from = String(edge.fromKey || edge.fromId)
    const to = String(edge.toKey || edge.toId)
    if (!nodeKeys.has(from) || !nodeKeys.has(to)) continue
    outgoing.get(from)?.push(to)
    incoming.set(to, (incoming.get(to) || 0) + 1)
  }
  const depth = new Map<string, number>()
  const queue = nodes
    .map((node:any) => String(node.topologyId || node.id))
    .filter(key => (incoming.get(key) || 0) === 0)
  if (!queue.length && nodes[0]) queue.push(String(nodes[0].topologyId || nodes[0].id))
  for (const key of queue) depth.set(key, 0)
  for (let i = 0; i < queue.length; i++) {
    const key = queue[i]
    const nextDepth = (depth.get(key) || 0) + 1
    for (const next of outgoing.get(key) || []) {
      if ((depth.get(next) ?? -1) < nextDepth) {
        depth.set(next, nextDepth)
        queue.push(next)
      }
    }
  }
  for (const node of nodes) {
    const key = String(node.topologyId || node.id)
    if (!depth.has(key)) depth.set(key, preferredTopologyDepth(node))
    else depth.set(key, Math.max(depth.get(key) || 0, preferredTopologyDepth(node)))
  }
  const buckets = new Map<number, any[]>()
  for (const node of nodes) {
    const key = String(node.topologyId || node.id)
    const d = depth.get(key) || 0
    const list = buckets.get(d) || []
    list.push(node)
    buckets.set(d, list)
  }
  return { buckets, maxDepth: Math.max(0, ...Array.from(buckets.keys())) }
}
const canvasStateRawJSON = (value: unknown, fallback: unknown) => {
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value ?? fallback)
  } catch {
    return JSON.stringify(fallback)
  }
}
const loadComponentNodePositions = async () => {
  componentNodePositions.value = {}
  manualCanvasEdges.value = []
  if (!envId.value) return
  const res = await api.getEnvironmentCanvasState(envId.value)
  componentNodePositions.value = parseComponentTopologyPositions(canvasStateRawJSON(res?.data?.positions, {}))
  manualCanvasEdges.value = parseComponentTopologyManualEdges(canvasStateRawJSON(res?.data?.edges, []))
}
const saveComponentNodePositions = async () => {
  if (!envId.value) return
  await api.saveEnvironmentCanvasState(envId.value, {
    positions: JSON.parse(serializeComponentTopologyPositions(componentNodePositions.value)),
    edges: JSON.parse(serializeComponentTopologyManualEdges(manualCanvasEdges.value)),
  })
}
const graphLayout = computed(() => {
  return layoutTopologyGraph(filteredTopologyNodes.value, [])
})
const environmentGraphLayout = computed(() => layoutTopologyGraph(environmentTopologyAllNodes.value, []))
const componentCanvasSize = computed(() => {
  const maxColumnNodes = Math.max(1, ...Array.from(graphLayout.value.buckets.values()).map(items => items.length))
  return {
    width: Math.max(1280, componentCanvasMetrics.left * 2 + (graphLayout.value.maxDepth + 1) * componentCanvasMetrics.colWidth + 220),
    height: Math.max(720, componentCanvasMetrics.top + maxColumnNodes * (componentCanvasMetrics.nodeHeight + componentCanvasMetrics.rowGap) + 220),
  }
})
const environmentCanvasSize = computed(() => {
  const maxColumnNodes = Math.max(1, ...Array.from(environmentGraphLayout.value.buckets.values()).map(items => items.length))
  return {
    width: Math.max(1280, componentCanvasMetrics.left * 2 + (environmentGraphLayout.value.maxDepth + 1) * componentCanvasMetrics.colWidth + 220),
    height: Math.max(680, componentCanvasMetrics.top + maxColumnNodes * (componentCanvasMetrics.nodeHeight + componentCanvasMetrics.rowGap) + 180),
  }
})
const componentCanvasViewBox = computed(() => {
  return componentTopologyCanvasViewBox(componentCanvasSize.value)
})
const environmentCanvasViewBox = computed(() => {
  return componentTopologyCanvasViewBox(environmentCanvasSize.value)
})
const componentCanvasNodes = computed(() =>
  Array.from(graphLayout.value.buckets.entries()).flatMap(([depth, items]) =>
    items.map((node:any, nodeIndex) => ({
      ...node,
      x: componentNodePosition(node, depth, nodeIndex).x,
      y: componentNodePosition(node, depth, nodeIndex).y,
      width: componentCanvasMetrics.nodeWidth,
      height: componentCanvasMetrics.nodeHeight,
    }))
  )
)
const environmentCanvasNodes = computed(() =>
  Array.from(environmentGraphLayout.value.buckets.entries()).flatMap(([depth, items]) =>
    items.map((node:any, nodeIndex) => ({
      ...node,
      x: componentNodePosition(node, depth, nodeIndex).x,
      y: componentNodePosition(node, depth, nodeIndex).y,
      width: componentCanvasMetrics.nodeWidth,
      height: componentCanvasMetrics.nodeHeight,
    }))
  )
)
const componentNodePosition = (node:any, laneIndex: number, nodeIndex: number) => {
  const key = String(node?.topologyId || node?.id || node?.name || '')
  const saved = componentNodePositions.value[key]
  if (saved) return saved
  return {
    x: componentCanvasMetrics.left + laneIndex * componentCanvasMetrics.colWidth,
    y: componentCanvasMetrics.top + nodeIndex * (componentCanvasMetrics.nodeHeight + componentCanvasMetrics.rowGap),
  }
}
const canvasEdgesForNodes = (nodes:any[], baseEdges:any[]) => {
  const nodeByKey = new Map(nodes.map((node:any) => [String(node.topologyId || node.id), node]))
  const nodeById = new Map(nodes.map((node:any) => [String(node.id), node]))
  const explicitEdges = baseEdges
    .map(edge => ({
      ...edge,
      fromNode: nodeByKey.get(edge.fromKey || '') || nodeById.get(String(edge.fromId)),
      toNode: nodeByKey.get(edge.toKey || '') || nodeById.get(String(edge.toId)),
    }))
    .filter(edge => edge.fromNode && edge.toNode)
  const seen = new Set(explicitEdges.map((edge:any) => `${edge.fromKey || edge.fromId}->${edge.toKey || edge.toId}`))
  const manualEdges = manualCanvasEdges.value
    .map(edge => {
      const fromNode = nodeByKey.get(edge.fromKey)
      const toNode = nodeByKey.get(edge.toKey)
      if (!fromNode || !toNode) return null
      const key = `${edge.fromKey}->${edge.toKey}`
      if (seen.has(key)) return null
      seen.add(key)
      return {
        from: String(fromNode.name || edge.fromKey),
        to: String(toNode.name || edge.toKey),
        fromId: Number(fromNode.id || 0),
        toId: Number(toNode.id || 0),
        fromKey: edge.fromKey,
        toKey: edge.toKey,
        fromNode,
        toNode,
      }
    })
    .filter(Boolean)
  return [...explicitEdges, ...manualEdges]
}
const componentCanvasEdges = computed(() => canvasEdgesForNodes(componentCanvasNodes.value, []))
const environmentCanvasEdges = computed(() => canvasEdgesForNodes(environmentCanvasNodes.value, []))
const componentNodeStyle = (node:any) => ({
  left: `${node.x}px`,
  top: `${node.y}px`,
  width: `${node.width}px`,
  height: `${node.height}px`,
})
const componentNodeIconClass = (node:any) => {
  if (node?.topologyKind === 'service') return `node-type-icon--${String(node.type || node.serviceType || 'service').toLowerCase()}`
  return `node-type-icon--${String(node.type || 'component').toLowerCase()}`
}
const componentNodeIconPath = (node:any) => {
  const type = String(node?.type || node?.serviceType || '').toLowerCase()
  const paths: Record<string, string> = {
    frontend: 'M4 5h16v10H4V5zm2 2v6h12V7H6zm3 12h6v2H9v-2zm-4-2h14v2H5v-2z',
    backend: 'M4 4h16v6H4V4zm2 2v2h12V6H6zm-2 8h16v6H4v-6zm2 2v2h12v-2H6z',
    database: 'M12 3c-4.4 0-8 1.6-8 3.5v11C4 19.4 7.6 21 12 21s8-1.6 8-3.5v-11C20 4.6 16.4 3 12 3zm0 2c3.4 0 5.7.9 6 1.5-.3.6-2.6 1.5-6 1.5s-5.7-.9-6-1.5C6.3 5.9 8.6 5 12 5zm0 6c2.4 0 4.6-.5 6-1.3V12c-.3.6-2.6 1.5-6 1.5S6.3 12.6 6 12V9.7c1.4.8 3.6 1.3 6 1.3zm0 8c-3.4 0-5.7-.9-6-1.5v-2.3c1.4.8 3.6 1.3 6 1.3s4.6-.5 6-1.3v2.3c-.3.6-2.6 1.5-6 1.5z',
    middleware: 'M5 5h6v6H5V5zm8 0h6v6h-6V5zM5 13h6v6H5v-6zm8 0h6v6h-6v-6z',
    redis: 'M12 3 3 7l9 4 9-4-9-4zm-7 8 7 3 7-3v2l-7 3-7-3v-2zm0 4 7 3 7-3v2l-7 3-7-3v-2z',
    postgresql: 'M12 3c-4.4 0-8 1.6-8 3.5v11C4 19.4 7.6 21 12 21s8-1.6 8-3.5v-11C20 4.6 16.4 3 12 3zm0 2c3.4 0 5.7.9 6 1.5-.3.6-2.6 1.5-6 1.5s-5.7-.9-6-1.5C6.3 5.9 8.6 5 12 5z',
    mysql: 'M12 3c-4.4 0-8 1.6-8 3.5v11C4 19.4 7.6 21 12 21s8-1.6 8-3.5v-11C20 4.6 16.4 3 12 3zm0 2c3.4 0 5.7.9 6 1.5-.3.6-2.6 1.5-6 1.5s-5.7-.9-6-1.5C6.3 5.9 8.6 5 12 5z',
    mongodb: 'M12 2c3 3 4 5.5 4 8.5 0 4-2.4 7.3-5 8.4V22h-2v-3.1c-2.6-1.1-5-4.4-5-8.4C4 6.5 7 3.5 12 2zm0 3.4c-3 2-4 4.1-4 6 0 2.2 1 4.1 3 5.2V9h2v7.6c2-1.1 3-3 3-5.2 0-1.9-1-4-4-6z',
    rabbitmq: 'M4 5h16v12H8l-4 4V5zm2 2v10l1.2-1.2H18V7H6zm3 2h2v2H9V9zm4 0h2v2h-2V9z',
    kafka: 'M6 7a3 3 0 1 1 4 2.8v2.4a3 3 0 1 1-2 0V9.8A3 3 0 0 1 6 7zm10-3a3 3 0 1 1-2.8 4H11V6h2.2A3 3 0 0 1 16 4zm0 10a3 3 0 1 1-2.8 4H11v-2h2.2a3 3 0 0 1 2.8-2z',
  }
  return paths[type] || paths.middleware
}
const componentEdgePath = (edge:any) => {
  return componentTopologyEdgePath(edge)
}
const selectedComponent = computed(() =>
  components.value.find((comp:any) => Number(comp.id) === Number(selectedComponentId.value))
  || filteredComponents.value[0]
  || components.value[0]
  || null
)
const singleApplicationManagedComponents = computed(() =>
  components.value.filter((comp:any) => String(comp?.argocdApp || '').trim())
)
const applicationSetDeployBlocked = computed(() => singleApplicationManagedComponents.value.length > 0)
const applicationSetDeployHint = computed(() => {
  if (!applicationSetDeployBlocked.value) return '将画布中的组件以 ApplicationSet 模式统一部署。'
  const names = singleApplicationManagedComponents.value
    .map((comp:any) => comp.name || comp.argocdApp)
    .filter(Boolean)
    .slice(0, 3)
    .join('、')
  const suffix = names ? ` 已有组件：${names}。` : ''
  return `当前环境已有单独 Application 管理的组件，不能直接使用应用集部署。${suffix}请先迁移为整体部署模式。`
})
const parseRuntimeConfig = (raw:any) => {
  if (!raw) return { env: [], dependencies: [] as string[], command: [] as string[], args: [] as string[] }
  if (typeof raw === 'object') return {
    env: Array.isArray(raw.env) ? raw.env : [],
    dependencies: Array.isArray(raw.dependencies) ? raw.dependencies.map((item:any) => String(item).trim()).filter(Boolean) : [],
    command: Array.isArray(raw.command) ? raw.command.map((item:any) => String(item).trim()).filter(Boolean) : [],
    args: Array.isArray(raw.args) ? raw.args.map((item:any) => String(item).trim()).filter(Boolean) : [],
  }
  try {
    const parsed = JSON.parse(String(raw))
    return {
      env: Array.isArray(parsed?.env) ? parsed.env : [],
      dependencies: Array.isArray(parsed?.dependencies) ? parsed.dependencies.map((item:any) => String(item).trim()).filter(Boolean) : [],
      command: Array.isArray(parsed?.command) ? parsed.command.map((item:any) => String(item).trim()).filter(Boolean) : [],
      args: Array.isArray(parsed?.args) ? parsed.args.map((item:any) => String(item).trim()).filter(Boolean) : [],
    }
  } catch {
    return { env: [], dependencies: [] as string[], command: [] as string[], args: [] as string[] }
  }
}
const relationshipTargets = computed(() => {
  const sourceId = Number(relationshipSourceComponent.value?.id)
  const componentTargets = components.value
    .filter((comp:any) => Number(comp.id) !== sourceId)
    .map((comp:any) => ({
      key: String(comp.name || comp.id),
      name: comp.name || `component-${comp.id}`,
      kind: '组件',
      type: compTypeText(comp.type),
    }))
  const serviceTargets = installedInfra.value.map((svc:any) => ({
    key: String(svc.serviceName || svc.name || svc.serviceType),
    name: svc.serviceName || svc.name || svc.serviceType,
    kind: '中间件',
    type: typeLabel(svc.serviceType),
  }))
  return [...componentTargets, ...serviceTargets]
})
const selectComponent = (compId?: string | number) => {
  const id = Number(compId)
  if (Number.isFinite(id)) {
    selectedComponentId.value = id
    selectedTopologyKey.value = `component:${id}`
  }
}
const componentNodeActive = (node:any) => {
  const key = String(node?.topologyId || node?.id || '')
  if (selectedTopologyKey.value) return selectedTopologyKey.value === key
  return node?.topologyKind !== 'service' && selectedComponent.value?.id === node?.id
}
const topologyNodeSubtitle = (node:any) => node?.topologyKind === 'service'
  ? `${typeLabel(node.type || 'infra')} · 已安装服务`
  : `${compTypeText(node.type)} · ${componentDeliveryModeLabel(node)}`
const selectTopologyNode = (node:any) => {
  if (!node) return
  selectedTopologyKey.value = String(node.topologyId || node.id || '')
  if (node.topologyKind === 'service') {
    openServiceConfigDrawer(node)
    return
  }
  selectComponent(node.id)
  openComponentConfigDrawer(node)
}
const handleTopologyNodeClick = (event: MouseEvent, node:any) => {
  const key = String(node?.topologyId || node?.id || node?.name || '')
  if (suppressNextTopologyClick.value && (suppressTopologyClickKeys.value.length === 0 || suppressTopologyClickKeys.value.includes(key))) {
    suppressNextTopologyClick.value = false
    suppressTopologyClickKeys.value = []
    event.preventDefault()
    event.stopPropagation()
    return
  }
  if (recentTopologyDrag.value?.key === key && Date.now() - recentTopologyDrag.value.at < 350) {
    event.preventDefault()
    event.stopPropagation()
    return
  }
  suppressTopologyClickKeys.value = []
  if (event.shiftKey || event.ctrlKey || event.metaKey) {
    const idx = selectedNodeKeys.value.indexOf(key)
    if (idx >= 0) {
      selectedNodeKeys.value = selectedNodeKeys.value.filter(k => k !== key)
    } else {
      selectedNodeKeys.value = [...selectedNodeKeys.value, key]
    }
    return
  }
  selectedNodeKeys.value = [key]
  selectTopologyNode(node)
}
const closeComponentContextMenu = () => {
  componentContextMenu.value = { visible: false, x: 0, y: 0, kind: 'component', component: null, service: null }
  contextSubmenu.value = { visible: false, x: 0, y: 0, mode: 'tool', templates: [] }
}
const componentTypeTemplates = [
  { type: 'frontend', label: '前端服务', description: '创建前端应用组件' },
  { type: 'backend', label: '后端服务', description: '创建后端服务组件' },
  { type: 'custom', label: '自定义组件', description: '创建自定义工作负载组件' },
]
const openComponentSubmenu = () => {
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y,
    mode: 'component',
    templates: componentTypeTemplates,
  }
}
const openToolSubmenu = () => {
  const toolTemplates = buildPickerTemplates(templates.value, services.value, 'tool')
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y + 40,
    mode: 'tool',
    templates: toolTemplates.map((t:any) => ({ ...t, label: t.serviceName || t.name || t.type })),
  }
}
const openInfraSubmenu = () => {
  const infraTemplates = buildPickerTemplates(templates.value, services.value, 'infra')
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y + 80,
    mode: 'infra',
    templates: infraTemplates.map((t:any) => ({ ...t, label: t.serviceName || t.name || t.type })),
  }
}
const selectSubmenuTemplate = async (tmpl: any) => {
  if (tmpl.disabled) return
  if (contextSubmenu.value.mode === 'component') {
    await createCanvasComponentDraft(tmpl.type)
    return
  }
  closeComponentContextMenu()
  await createCanvasServiceDraft(tmpl.type)
}
const openTopologyContextMenu = (event: MouseEvent, node: any) => {
  if (node?.topologyKind === 'service') {
    openServiceContextMenu(event, node)
    return
  }
  openComponentContextMenu(event, node)
}
const contextMenuPosition = (event: MouseEvent, menuWidth = 220, menuHeight = 220) => {
  const x = Math.min(event.clientX, window.innerWidth - menuWidth - 8)
  const y = Math.min(event.clientY, window.innerHeight - menuHeight - 8)
  return {
    x: Math.max(8, x),
    y: Math.max(8, y),
  }
}
const openCanvasContextMenu = (event: MouseEvent) => {
  selectedTopologyKey.value = null
  const stageEl = (event.target as HTMLElement | null)?.closest?.('.component-canvas-stage') as HTMLElement | null
  canvasCreatePoint.value = stageEl ? canvasPointFromEvent(event, stageEl) : null
  const pos = contextMenuPosition(event, 220, 288)
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'canvas',
    component: null,
    service: null,
  }
}
const openComponentContextMenu = (event: MouseEvent, comp: any) => {
  selectComponent(comp?.id)
  const pos = contextMenuPosition(event)
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'component',
    component: comp,
    service: null,
  }
}
const openServiceContextMenu = (event: MouseEvent, svc: any) => {
  const pos = contextMenuPosition(event, 220, 140)
  selectedTopologyKey.value = String(svc?.topologyId || svc?.id || '')
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'service',
    component: null,
    service: svc,
  }
}
const canvasNodesForStage = (stageEl: HTMLElement | null) => {
  const isEnvironmentCanvas = Boolean(stageEl?.closest('.environment-topology-canvas'))
  return isEnvironmentCanvas ? environmentCanvasNodes.value : componentCanvasNodes.value
}
const canvasPointFromEvent = (event: PointerEvent | MouseEvent, stageEl: HTMLElement) => {
  const stageRect = stageEl.getBoundingClientRect()
  return {
    x: (event.clientX - stageRect.left) / canvasZoom.value,
    y: (event.clientY - stageRect.top) / canvasZoom.value,
  }
}
const connectorPointFromNodeRect = (nodeRect: DOMRect, canvasRect: DOMRect, side: 'top' | 'right' | 'bottom' | 'left') => {
  const left = (nodeRect.left - canvasRect.left) / canvasZoom.value
  const top = (nodeRect.top - canvasRect.top) / canvasZoom.value
  const width = nodeRect.width / canvasZoom.value
  const height = nodeRect.height / canvasZoom.value
  if (side === 'top') return { x: left + width / 2, y: top }
  if (side === 'bottom') return { x: left + width / 2, y: top + height }
  if (side === 'left') return { x: left, y: top + height / 2 }
  return { x: left + width, y: top + height / 2 }
}
const startComponentNodeDrag = (event: PointerEvent, node:any) => {
  if (event.button !== 0) return
  closeComponentContextMenu()
  const key = String(node?.topologyId || node?.id || node?.name || '')
  if (!key) return
  suppressNextTopologyClick.value = false
  suppressTopologyClickKeys.value = []
  const isGroupDrag = selectedNodeKeys.value.includes(key) && selectedNodeKeys.value.length > 1
  const keys = isGroupDrag ? [...selectedNodeKeys.value] : [key]
  const origins: Record<string, { x: number; y: number }> = {}
  const stageEl = (event.currentTarget as HTMLElement | null)?.closest('.component-canvas-stage') as HTMLElement | null
  const currentCanvasNodes = canvasNodesForStage(stageEl)
  if (isGroupDrag) {
    for (const k of keys) {
      const n = currentCanvasNodes.find((n: any) => String(n.topologyId || n.id) === k)
      if (n) origins[k] = { x: n.x, y: n.y }
    }
  } else {
    origins[key] = { x: Number(node.x || 0), y: Number(node.y || 0) }
  }
  componentNodeDrag.value = {
    keys,
    origins,
    startX: event.clientX,
    startY: event.clientY,
    moved: false,
    lastX: event.clientX,
    lastY: event.clientY,
  }
  try {
    ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
  } catch {
    // setPointerCapture can fail if the target is already detached.
  }
  window.addEventListener('pointermove', onComponentNodeDrag)
  window.addEventListener('pointerup', stopComponentNodeDrag, { once: true })
}
const finishComponentNodePointer = (event: PointerEvent, node:any) => {
  const drag = componentNodeDrag.value
  if (!drag) return
  const key = String(node?.topologyId || node?.id || node?.name || '')
  if (!drag.keys.includes(key)) return
  if (hasComponentTopologyDragMoved({ startX: drag.startX, startY: drag.startY, currentX: event.clientX, currentY: event.clientY })) {
    suppressNextTopologyClick.value = true
    suppressTopologyClickKeys.value = [...drag.keys]
  }
}
const onComponentNodeDrag = (event: PointerEvent) => {
  const drag = componentNodeDrag.value
  if (!drag) return
  drag.lastX = event.clientX
  drag.lastY = event.clientY
  if (hasComponentTopologyDragMoved({ startX: drag.startX, startY: drag.startY, currentX: event.clientX, currentY: event.clientY })) drag.moved = true
  const updated: Record<string, { x: number; y: number }> = {}
  for (const k of drag.keys) {
    const origin = drag.origins[k]
    if (!origin) continue
    updated[k] = nextComponentTopologyDragPosition({
      originX: origin.x,
      originY: origin.y,
      startX: drag.startX,
      startY: drag.startY,
      currentX: event.clientX,
      currentY: event.clientY,
      zoom: canvasZoom.value,
    })
  }
  componentNodePositions.value = { ...componentNodePositions.value, ...updated }
}
const stopComponentNodeDrag = (event?: PointerEvent) => {
  window.removeEventListener('pointermove', onComponentNodeDrag)
  const drag = componentNodeDrag.value
  if (drag && event) {
    drag.lastX = event.clientX
    drag.lastY = event.clientY
    if (hasComponentTopologyDragMoved({ startX: drag.startX, startY: drag.startY, currentX: event.clientX, currentY: event.clientY })) {
      drag.moved = true
      const updated: Record<string, { x: number; y: number }> = {}
      for (const k of drag.keys) {
        const origin = drag.origins[k]
        if (!origin) continue
        updated[k] = nextComponentTopologyDragPosition({
          originX: origin.x,
          originY: origin.y,
          startX: drag.startX,
          startY: drag.startY,
          currentX: event.clientX,
          currentY: event.clientY,
          zoom: canvasZoom.value,
        })
      }
      componentNodePositions.value = { ...componentNodePositions.value, ...updated }
    }
  }
  if (drag?.moved) {
    suppressNextTopologyClick.value = true
    suppressTopologyClickKeys.value = [...drag.keys]
    recentTopologyDrag.value = { key: drag.keys[0] || '', at: Date.now() }
    void saveComponentNodePositions().catch((e:any) => {
      pageError.value = '保存画布布局失败：' + (e?.message || '未知错误')
    })
  }
  componentNodeDrag.value = null
}
const startCanvasMarquee = (event: PointerEvent) => {
  if (event.button !== 0) return
  if ((event.target as HTMLElement)?.closest?.('.component-topology-node')) return
  if ((event.target as HTMLElement)?.closest?.('.topology-controls')) return
  closeComponentContextMenu()
  const stageEl = (event.currentTarget as HTMLElement).closest('.component-canvas-stage') as HTMLElement | null
  if (!stageEl) return
  const point = canvasPointFromEvent(event, stageEl)
  const canvasX = point.x
  const canvasY = point.y
  marqueeDrag.value = { startCanvasX: canvasX, startCanvasY: canvasY, currentCanvasX: canvasX, currentCanvasY: canvasY, stageEl }
  marqueeRect.value = { x: canvasX, y: canvasY, width: 0, height: 0 }
  if (!(event.shiftKey || event.ctrlKey || event.metaKey)) {
    selectedNodeKeys.value = []
  }
  window.addEventListener('pointermove', onCanvasMarquee)
  window.addEventListener('pointerup', stopCanvasMarquee, { once: true })
}
const onCanvasMarquee = (event: PointerEvent) => {
  const md = marqueeDrag.value
  if (!md) return
  const point = canvasPointFromEvent(event, md.stageEl)
  md.currentCanvasX = point.x
  md.currentCanvasY = point.y
  marqueeRect.value = {
    x: Math.min(md.startCanvasX, md.currentCanvasX),
    y: Math.min(md.startCanvasY, md.currentCanvasY),
    width: Math.abs(md.currentCanvasX - md.startCanvasX),
    height: Math.abs(md.currentCanvasY - md.startCanvasY),
  }
}
const stopCanvasMarquee = (event?: PointerEvent) => {
  window.removeEventListener('pointermove', onCanvasMarquee)
  const md = marqueeDrag.value
  if (md && marqueeRect.value && (marqueeRect.value.width > 4 || marqueeRect.value.height > 4)) {
    const rect = marqueeRect.value
    const selected = canvasNodesForStage(md.stageEl)
      .filter((node: any) => isNodeInMarquee(node, rect))
      .map((node: any) => String(node.topologyId || node.id))
    if (event?.shiftKey || event?.ctrlKey || event?.metaKey) {
      selectedNodeKeys.value = [...new Set([...selectedNodeKeys.value, ...selected])]
    } else {
      selectedNodeKeys.value = selected
    }
  }
  marqueeDrag.value = null
  marqueeRect.value = null
}
const startConnectionDrag = (event: PointerEvent, node: any, side: 'top' | 'right' | 'bottom' | 'left' = 'right') => {
  event.stopPropagation()
  closeComponentContextMenu()
  const nodeEl = (event.currentTarget as HTMLElement).closest('.component-topology-node')
  const canvasEl = nodeEl?.closest('.component-canvas-stage') as HTMLElement | null
  if (!nodeEl || !canvasEl) return

  const nodeRect = nodeEl.getBoundingClientRect()
  const canvasRect = canvasEl.getBoundingClientRect()

  const start = connectorPointFromNodeRect(nodeRect, canvasRect, side)
  const current = canvasPointFromEvent(event, canvasEl)

  connectionDrag.value = {
    fromNode: node,
    startX: start.x,
    startY: start.y,
    currentX: current.x,
    currentY: current.y,
    stageEl: canvasEl,
  }
  window.addEventListener('pointermove', onConnectionDrag)
  window.addEventListener('pointerup', stopConnectionDrag, { once: true })
}
const onConnectionDrag = (event: PointerEvent) => {
  if (!connectionDrag.value) return
  const point = canvasPointFromEvent(event, connectionDrag.value.stageEl)
  connectionDrag.value.currentX = point.x
  connectionDrag.value.currentY = point.y
}
const stopConnectionDrag = (event: PointerEvent) => {
  window.removeEventListener('pointermove', onConnectionDrag)
  if (!connectionDrag.value) return

  const fromNodeKey = nodeKey(connectionDrag.value.fromNode)
  const dropPoint = canvasPointFromEvent(event, connectionDrag.value.stageEl)
  const targetNode = findTopologyNodeAtPoint(canvasNodesForStage(connectionDrag.value.stageEl), dropPoint, fromNodeKey)
  const toNodeKey = nodeKey(targetNode)

  if (toNodeKey && toNodeKey !== fromNodeKey) {
    void createComponentDependency(fromNodeKey, toNodeKey)
  }

  connectionDrag.value = null
}
const createComponentDependency = async (fromKey: string, toKey: string) => {
  const next = parseComponentTopologyManualEdges(JSON.stringify([
    ...manualCanvasEdges.value,
    { fromKey, toKey },
  ]))
  manualCanvasEdges.value = next
  try {
    await saveComponentNodePositions()
  } catch (e:any) {
    pageError.value = '保存画布连线失败：' + (e?.message || '未知错误')
  }
}
const componentEdgeHighlighted = (edge: { fromId: number; toId: number }) => {
  const edgeWithKeys = edge as any
  const key = selectedTopologyKey.value || (selectedComponent.value?.id ? `component:${selectedComponent.value.id}` : '')
  if (key) return edgeWithKeys.fromKey === key || edgeWithKeys.toKey === key
  const id = selectedComponent.value?.id
  return Boolean(id && (edge.fromId === id || edge.toId === id))
}
const templateFor = (type:string) => templates.value.find((item:any) => item.type === type)
const serviceCategory = (svc:any) => resolveServiceCategory(svc, templates.value)
const installedTools = computed(() => services.value.filter((svc:any) => serviceCategory(svc) !== 'infra'))
const installedInfra = computed(() => services.value.filter((svc:any) => serviceCategory(svc) === 'infra'))
const serviceCapability = (svc:any): CapabilityTab => resolveServiceCapability(svc, templates.value)
const capabilityTabs = computed<CapabilityTab[]>(() => buildCapabilityTabs(services.value, templates.value))
const activeCapabilityTab = computed(() => capabilityTabs.value.find(tab => tab.key === activeTab.value) || null)
const activeCapabilityServices = computed(() =>
  activeCapabilityTab.value
    ? services.value.filter((svc:any) => serviceCapability(svc).key === activeCapabilityTab.value?.key)
    : []
)
const activeCapabilityService = computed(() => {
  if (!activeCapabilityServices.value.length) return null
  const selected = activeCapabilityServices.value.find((svc:any) => svc.id === activeCapabilityServiceId.value)
  return selected || activeCapabilityServices.value[0]
})
const activeCapabilityWorkspace = computed<ServiceWorkspace>(() => {
  const svc = activeCapabilityService.value
  if (!svc) return emptyCapabilityWorkspace
  return capabilityWorkspaceCache.value[svc.id] || emptyCapabilityWorkspace
})
const activeCapabilityWorkspaceReady = computed(() => {
  const svc = activeCapabilityService.value
  return Boolean(svc?.id && capabilityWorkspaceCache.value[svc.id])
})
const capabilityWorkspaceKey = computed(() =>
  `${envRouteKey.value}:${activeCapabilityTab.value?.key || 'none'}:${activeCapabilityService.value?.id || 'none'}`
)
const registryServices = computed(() => services.value.filter((svc:any) => ['registry', 'harbor'].includes(String(svc?.serviceType || ''))))
const registryWorkspaces = computed(() =>
  registryServices.value
    .map((svc:any) => capabilityWorkspaceCache.value[svc.id])
    .filter(Boolean) as ServiceWorkspace[]
)
const registryHostForDrawer = computed(() => {
  for (const workspace of registryWorkspaces.value) {
    const trust = workspace.resources.find((x: WorkspaceResource) => x.type === 'Runtime Trust')
    const host = String(trust?.annotations?.registryHost || '').trim()
    if (host) return host
  }
  const configured = registryWorkspaces.value.flatMap(workspace => workspace.config || []).find(item => item.label === '外部访问地址')?.value || ''
  return String(configured).replace(/^https?:\/\//, '').replace(/\/$/, '')
})
const registryImageRepositories = computed<RegistryRepositoryOption[]>(() => {
  const resources = registryWorkspaces.value.flatMap(workspace => workspace.resources || [])
  return resources
    .filter((x: WorkspaceResource) => x.type === 'Image Repository' || x.type === 'Harbor Repository' || x.type === 'Repository')
    .map((resource: WorkspaceResource) => {
      const annotations = resource.annotations || {}
      const rawTags = annotations.tags || annotations.tagList || annotations.versions || []
      const tags = Array.isArray(rawTags)
        ? rawTags.map((item:any) => String(item).trim()).filter(Boolean)
        : String(rawTags).split(',').map(item => item.trim()).filter(Boolean)
      const imageRepository = splitImageRepositoryAndTag(String(annotations.image || '')).repository
      return {
        repository: registryRepositorySuffix(String(annotations.repository || imageRepository || resource.name || '').trim()),
        tags,
        resource,
      }
    })
    .filter(item => item.repository)
})
const selectedRegistryRepository = computed(() =>
  registryImageRepositories.value.find(item => item.repository === configForm.value.repository) || null
)
const selectedRegistryRepositoryTags = computed(() => selectedRegistryRepository.value?.tags || [])
const registryRepositorySuffix = (repository: string) => {
  const value = splitImageRepositoryAndTag(String(repository || '').trim()).repository.replace(/^\/+/, '')
  if (!value) return ''
  const host = String(registryHostForDrawer.value || '').trim().replace(/\/$/, '')
  if (host && value.startsWith(`${host}/`)) return value.slice(host.length + 1)
  const firstSegment = value.split('/')[0] || ''
  const hasRegistryHost = firstSegment === 'localhost' || firstSegment.includes('.') || firstSegment.includes(':')
  if (hasRegistryHost) {
    const suffix = value.split('/').slice(1).join('/')
    return suffix || value
  }
  return value
}
const normalizeRegistryRepository = (repository: string) => {
  const value = registryRepositorySuffix(repository)
  if (!value) return ''
  const host = String(registryHostForDrawer.value || '').trim().replace(/\/$/, '')
  return host ? `${host}/${value}` : value
}
const registryImageFromConfig = computed(() => {
  const repository = normalizeRegistryRepository(String(configForm.value.repository || ''))
  const version = String(configForm.value.version || '').trim()
  if (repository.includes(':') && repository.split('/').pop()?.includes(':')) return repository
  if (!repository || !version) return ''
  return `${repository}:${version}`
})
const registryRepositoryDisplayName = (repository: string) => {
  const value = String(repository || '').trim()
  const host = String(registryHostForDrawer.value || '').trim()
  if (host && value.startsWith(`${host}/`)) return value.slice(host.length + 1)
  return value
}
const ensureRegistryWorkspaces = async () => {
  const targets = registryServices.value.filter((svc:any) => svc?.id)
  if (!targets.length || registryWorkspaceLoading.value) return
  registryWorkspaceLoading.value = true
  registryWorkspaceError.value = ''
  try {
    for (const svc of targets) {
      const res = await api.getServiceWorkspace(envId.value, Number(svc.id))
      capabilityWorkspaceCache.value = { ...capabilityWorkspaceCache.value, [svc.id]: res.data }
    }
  } catch (e:any) {
    registryWorkspaceError.value = `镜像仓库刷新失败：${e?.message || '未知错误'}`
  } finally {
    registryWorkspaceLoading.value = false
  }
}
const syncConfigVersionFromRepository = () => {
  if (!selectedRegistryRepositoryTags.value.includes(configForm.value.version)) {
    configForm.value.version = selectedRegistryRepositoryTags.value[0] || ''
  }
}
const splitImageRepositoryAndTag = (image: string) => {
  const value = String(image || '').trim()
  const last = value.split('/').pop() || ''
  const tagAt = last.lastIndexOf(':')
  if (tagAt < 0) return { repository: value, version: '' }
  return {
    repository: value.slice(0, value.length - (last.length - tagAt)),
    version: last.slice(tagAt + 1),
  }
}

const workspaceComponentForService = (svc:any) => {
  switch (svc?.serviceType) {
    case 'monitor': return MonitorWorkspace
    case 'log': return LogWorkspace
    case 'deploy': return ArgocdWorkspace
    case 'git': return GiteaWorkspace
    case 'ci': return PipelineWorkspace
    case 'mysql': case 'postgresql': return DatabaseWorkspace
    case 'redis': return RedisWorkspace
    case 'mongodb': return MongoWorkspace
    case 'rabbitmq': return RabbitWorkspace
    case 'kafka': return KafkaWorkspace
    case 'minio': return MinioWorkspace
    case 'registry': case 'harbor': return RegistryWorkspace
    default: return null
  }
}
const selectCapabilityService = (svc:any) => {
  activeCapabilityServiceId.value = svc.id
  capabilityWorkspaceError.value = ''
  capabilityWorkspaceMessage.value = ''
  void loadCapabilityWorkspace(svc)
}
const loadCapabilityWorkspace = async (svc = activeCapabilityService.value) => {
  if (!svc?.id) return
  if (!serviceStatusHasRuntime(svc)) {
    capabilityWorkspaceMessage.value = '服务尚未部署，部署后会显示运行工作台。'
    return
  }
  const seq = ++capabilityWorkspaceLoadSeq
  const targetEnvId = envId.value
  const targetServiceId = svc.id
  capabilityWorkspaceLoading.value = true
  capabilityWorkspaceError.value = ''
  try {
    const res = await api.getServiceWorkspace(targetEnvId, targetServiceId)
    if (seq !== capabilityWorkspaceLoadSeq || targetEnvId !== envId.value) return
    capabilityWorkspaceCache.value = { ...capabilityWorkspaceCache.value, [targetServiceId]: res.data }
  } catch (e:any) {
    if (seq !== capabilityWorkspaceLoadSeq || targetEnvId !== envId.value) return
    capabilityWorkspaceError.value = `工作台刷新失败：${e?.message || '未知错误'}`
  } finally {
    if (seq === capabilityWorkspaceLoadSeq && targetEnvId === envId.value) {
      capabilityWorkspaceLoading.value = false
    }
  }
}
const beginCapabilityWorkspaceAction = async (action: WorkspaceAction, target?: string) => {
  if (action.fields?.length) {
    capabilityWorkspaceError.value = ''
    capabilityWorkspaceMessage.value = ''
    activeCapabilityAction.value = action
    activeCapabilityActionTarget.value = target
    const params: Record<string, string> = {}
    for (const field of action.fields) {
      params[field.name] = field.default || (field.type === 'checkbox' ? 'false' : '')
    }
    activeCapabilityActionParams.value = params
    return
  }
  await runCapabilityWorkspaceAction(action, target)
}
const closeCapabilityActionDialog = () => {
  if (capabilityWorkspaceLoading.value) return
  activeCapabilityAction.value = null
  activeCapabilityActionTarget.value = undefined
  activeCapabilityActionParams.value = {}
  capabilityWorkspaceError.value = ''
}
const setCapabilityActionCheckboxParam = (name: string, event: Event) => {
  activeCapabilityActionParams.value[name] = (event.target as HTMLInputElement).checked ? 'true' : 'false'
}
const submitCapabilityActionDialog = async () => {
  const action = activeCapabilityAction.value
  if (!action) return
  const validationMessage = validateWorkspaceActionParams(action.fields || [], activeCapabilityActionParams.value)
  if (validationMessage) {
    capabilityWorkspaceError.value = validationMessage
    return
  }
  await runCapabilityWorkspaceAction(action, activeCapabilityActionTarget.value, activeCapabilityActionParams.value)
  if (!capabilityWorkspaceError.value) {
    activeCapabilityAction.value = null
    activeCapabilityActionTarget.value = undefined
    activeCapabilityActionParams.value = {}
  }
}
const runCapabilityWorkspaceAction = async (action: WorkspaceAction, target?: string, params?: Record<string, string>) => {
  const svc = activeCapabilityService.value
  if (!svc?.id || !action?.key) return
  capabilityWorkspaceLoading.value = true
  capabilityWorkspaceError.value = ''
  capabilityWorkspaceMessage.value = ''
  try {
    const res = await api.runServiceWorkspaceAction(envId.value, svc.id, action.key, target, params)
    capabilityWorkspaceCache.value = { ...capabilityWorkspaceCache.value, [svc.id]: res.data }
    capabilityWorkspaceMessage.value = '执行完成，工作台已刷新。'
  } catch (e:any) {
    capabilityWorkspaceError.value = '执行失败：' + (e?.message || '未知错误')
  } finally {
    capabilityWorkspaceLoading.value = false
  }
}
watch(tabs, () => {
  const normalized = normalizeEnvTab(activeTab.value)
  if (normalized !== activeTab.value) {
    activeTab.value = normalized
    replaceEnvTab(normalized)
  }
})
watch(activeCapabilityService, (svc) => {
  if (!svc) return
  activeCapabilityServiceId.value = svc.id
  void loadCapabilityWorkspace(svc)
}, { immediate: true })
const serviceCapabilityDescription = (svc:any) => {
  const cap = serviceCapability(svc)
  const product = svcLabel(svc.serviceType)
  const descriptions: Record<string, string> = {
    'code-repository': `通过 ${product} 管理组件源码、GitOps 清单和仓库文件。`,
    'image-registry': `通过 ${product} 管理当前应用和环境隔离的镜像仓库、标签和运行时信任。`,
    'continuous-integration': `通过 ${product} 执行源码构建、镜像推送和交付触发。`,
    'continuous-deployment': `通过 ${product} 管理 Application、同步状态和集群拓扑。`,
    'monitoring-center': `通过 ${product} 查看组件、工具、数据库和中间件的监控面板。`,
    'logging-center': `通过 ${product} 查询组件、工具、数据库和中间件日志。`,
    databases: `通过 ${product} 管理数据库实例、库表对象和账号。`,
    cache: `通过 ${product} 管理缓存实例、键空间和连接配置。`,
    'message-queue': `通过 ${product} 管理消息队列、Topic、消费者和连接配置。`,
    middleware: `通过 ${product} 管理缓存、消息队列和中间件对象。`,
    'object-storage': `通过 ${product} 管理 Bucket、对象和访问配置。`,
  }
  return descriptions[cap.key] || templateFor(svc.serviceType)?.description || '当前环境已安装的能力入口。'
}
const runningToolCount = computed(() => installedTools.value.filter((svc:any) => svc.status === 'running').length)
const runningInfraCount = computed(() => installedInfra.value.filter((svc:any) => svc.status === 'running').length)
const runningComponentCount = computed(() => components.value.filter((comp:any) => ['running', 'deployed'].includes(String(comp.status || '').toLowerCase())).length)
const unhealthyServices = computed(() => services.value.filter((svc:any) => {
  const status = String(svc.status || '').toLowerCase()
  return Boolean(svc.errorMessage) || ['failed', 'error'].includes(status)
}))
const criticalTools = computed(() => {
  const priority = ['git', 'ci', 'registry', 'harbor', 'deploy', 'monitor', 'log']
  return installedTools.value
    .filter((svc:any) => priority.includes(svc.serviceType))
    .sort((a:any, b:any) => priority.indexOf(a.serviceType) - priority.indexOf(b.serviceType))
})
const externalAccessGroups = computed(() => {
  const groups = [
    { key: 'environment', label: '应用入口', items: [] as any[] },
  ]
  for (const item of externalAccess.value) {
    if (item?.scope !== 'service') groups[0].items.push(item)
  }
  return groups.filter(group => group.items.length > 0)
})
const applicationFrontendURL = computed(() => {
  // 优先查找前端组件的外部访问地址
  const frontendComponents = components.value.filter((comp:any) =>
    String(comp.type || '').toLowerCase() === 'frontend' && comp.externalUrl
  )
  if (frontendComponents.length > 0) {
    return frontendComponents[0].externalUrl
  }

  // 如果没有前端组件，返回第一个非service的外部访问地址
  for (const item of externalAccess.value) {
    if (item?.scope !== 'service' && item?.url) {
      return item.url
    }
  }

  return ''
})
const openApplicationURL = () => {
  if (applicationFrontendURL.value) {
    window.open(applicationFrontendURL.value, '_blank', 'noreferrer')
  }
}
const deployCanvasApplicationSet = () => {
  if (applicationSetDeployBlocked.value) {
    pageError.value = applicationSetDeployHint.value
    return
  }
  pageError.value = '应用集部署接口尚未启用，请先使用单卡片部署。'
}
const externalAccessSubtitle = (item:any) => {
  const parts = [item.namespace]
  if (item.serviceType) parts.push(svcLabel(item.serviceType))
  return parts.filter(Boolean).join(' · ')
}
const hasServiceType = (types: string[]) => services.value.some((svc:any) => types.includes(svc.serviceType) && ['running', 'installing'].includes(String(svc.status || '').toLowerCase()))
const tabForServiceTypes = (types: string[]) => {
  const svc = services.value.find((item:any) => types.includes(item.serviceType))
  return svc ? serviceCapability(svc).key : 'components'
}
const deliverySteps = computed(() => [
  {
    key: 'source',
    label: '组件源码',
    description: components.value.length ? `${components.value.length} 个组件已创建` : '创建 source 组件后进入交付流程',
    state: components.value.length ? 'ready' : 'pending',
    targetTab: 'components' as const,
  },
  {
    key: 'git',
    label: '代码仓库',
    description: hasServiceType(['git']) ? '代码仓库能力已安装' : '需要安装代码仓库工具',
    state: hasServiceType(['git']) ? 'ready' : 'missing',
    targetTab: tabForServiceTypes(['git']),
  },
  {
    key: 'ci',
    label: '持续集成',
    description: hasServiceType(['ci']) ? '持续集成能力已安装' : '需要安装持续集成工具并确认 kpack 已就绪',
    state: hasServiceType(['ci']) ? 'ready' : 'missing',
    targetTab: tabForServiceTypes(['ci']),
  },
  {
    key: 'registry',
    label: '镜像仓库',
    description: hasServiceType(['registry', 'harbor']) ? '镜像仓库能力已安装' : '需要安装镜像仓库',
    state: hasServiceType(['registry', 'harbor']) ? 'ready' : 'missing',
    targetTab: tabForServiceTypes(['registry', 'harbor']),
  },
  {
    key: 'deploy',
    label: '持续部署',
    description: hasServiceType(['deploy']) ? '持续部署能力已安装' : '需要安装持续部署工具',
    state: hasServiceType(['deploy']) ? 'ready' : 'missing',
    targetTab: tabForServiceTypes(['deploy']),
  },
])
const selectableServiceCount = computed(() => availableServices.value.filter((svc:any) => !svc.disabled).length)
const isActiveServiceInstalled = (serviceType:string) =>
  isServiceActive(services.value, serviceType)

const serviceTemplatesFromResponse = (res:any) => {
  if (Array.isArray(res?.data)) return res.data
  if (Array.isArray(res?.data?.data)) return res.data.data
  if (Array.isArray(res)) return res
  return []
}

const loadServiceTemplates = async () => {
  const tmplRes = await api.listServiceTemplates()
  templates.value = serviceTemplatesFromResponse(tmplRes)
}

const filterTemplates = (mode:'tool'|'infra') =>
  buildPickerTemplates(templates.value, services.value, mode)

const notifyEnvUpdated = () => {
  window.dispatchEvent(new CustomEvent('paap-env-updated', { detail: { envId: envId.value } }))
}

const refreshServices = async () => {
  const res = await api.getEnv(envId.value)
  env.value = res.data.environment
  components.value = res.data.components || []
  services.value = res.data.services || []
  externalAccess.value = res.data.externalAccess || []
  notifyEnvUpdated()
}

const prepareServicePicker = async (mode:'tool'|'infra') => {
  serviceModalMode.value = mode
  showServiceModal.value = true
  serviceModalError.value = ''
  const session = createPickerSessionState(templates.value, services.value, mode)
  availableServices.value = session.availableServices
  serviceForm.value.serviceType = session.selectedType
  serviceModalLoading.value = session.loading
  serviceModalNotice.value = session.notice
  serviceModalError.value = session.error
  try {
    await refreshServices()
    if (templates.value.length === 0) await loadServiceTemplates()
    availableServices.value = filterTemplates(mode)
    serviceForm.value.serviceType = availableServices.value.find((svc:any) => !svc.disabled)?.type || ''
    serviceModalNotice.value = pickerNotice(mode, availableServices.value.length, serviceForm.value.serviceType)
  } catch (e:any) {
    serviceModalError.value = '模板加载失败：' + (e?.message || '未知错误')
  } finally {
    serviceModalLoading.value = false
  }
}

const openServiceWorkspace = (svc:any) => {
  if (!svc?.id) return
  activeCapabilityServiceId.value = Number(svc.id)
  setActiveTab(serviceCapability(svc).key)
}
const componentDeployVersion = (comp:any) => {
  const version = String(comp?.version || '').trim()
  if (version) return version
  const image = String(comp?.image || comp?.registryImage || '').trim()
  const last = image.split('/').pop() || ''
  const colon = last.lastIndexOf(':')
  return colon >= 0 ? last.slice(colon + 1) : ''
}
const deployComponent = async (comp:any) => {
  if (!comp?.id || componentActionLoading.value) return
  const version = componentDeployVersion(comp)
  if (!version || version.toLowerCase() === 'latest') {
    pageError.value = '部署前需要在组件配置中填写明确版本，不能使用 latest。'
    return
  }
  componentActionLoading.value = true
  pageError.value = ''
  try {
    await api.deployComponent(Number(comp.id), { version })
    await refreshServices()
    selectComponent(comp.id)
  } catch (e:any) {
    pageError.value = '部署失败：' + (e?.message || '未知错误')
  } finally {
    componentActionLoading.value = false
    closeComponentContextMenu()
  }
}
const drawerRuntime = computed(() => configDrawer.value.component?.runtimeConfig || configDrawer.value.service?.runtimeConfig || {})
const drawerService = computed(() => configDrawer.value.kind === 'service' ? configDrawer.value.service : null)
const serviceDrawerWorkspace = computed<ServiceWorkspace | null>(() => {
  const svc = drawerService.value
  if (!svc?.id) return null
  return capabilityWorkspaceCache.value[svc.id] || null
})
const serviceDrawerProfile = computed(() => serviceConfigProfile(drawerService.value || {}))
const serviceDrawerConfigFields = computed(() => serviceDrawerProfile.value.fields)
const serviceDrawerVisibleConfigFields = computed(() => serviceDrawerConfigFields.value.filter((field) => serviceConfigFieldVisible(field, serviceConfigForm.value)))
const serviceDrawerPrimaryRows = computed(() => drawerService.value ? serviceConfigPrimaryRows(drawerService.value, serviceConfigForm.value) : [])
const serviceDrawerRuntimeRows = computed(() => drawerService.value || configDrawer.value.component
  ? serviceRuntimeDetailRows(drawerService.value || configDrawer.value.component)
  : [])
const serviceDrawerConnectionPreview = computed(() => connectionBindingPreview({ env: [] }, drawerService.value || {}))
const serviceDrawerTopology = computed(() => serviceTopologyFromWorkspace(drawerService.value || {}, serviceDrawerWorkspace.value?.resources || []))
const serviceDrawerConfigurable = computed(() => serviceDrawerProfile.value.showDeploymentConfig && serviceDrawerConfigFields.value.length > 0)
const serviceDrawerDeployDisabled = computed(() => configDrawer.value.saving || !serviceStatusCanDeploy(drawerService.value))
const serviceDrawerDeployLabel = computed(() => {
  if (configDrawer.value.saving) return '处理中...'
  const status = serviceStatusValue(drawerService.value)
  if (status === 'failed' || status === 'error') return '重新部署'
  if (status === 'running') return '应用配置'
  if (serviceStatusIsDraft(drawerService.value)) return '部署'
  return '部署'
})
const configDrawerTitle = computed(() => configDrawer.value.component?.name || configDrawer.value.service?.serviceName || configDrawer.value.service?.name || configDrawer.value.service?.serviceType || '-')
const configDrawerSubtitle = computed(() => {
  if (configDrawer.value.kind === 'service') return `${typeLabel(configDrawer.value.service?.serviceType || configDrawer.value.service?.type || '')} · ${serviceStatusText(configDrawer.value.service?.status)}`
  return `${compTypeText(configDrawer.value.component?.type)} · ${componentDeliveryModeLabel(configDrawer.value.component)}`
})
const configDrawerExternalUrl = computed(() => configDrawer.value.component?.externalUrl || configDrawer.value.service?.externalUrl || '')
const serviceDrawerInternalEndpoint = computed(() => drawerService.value && serviceStatusHasRuntime(drawerService.value) ? serviceInternalEndpoint(drawerService.value) : '')
const serviceDrawerExternalAccessEnabled = computed(() => Boolean(configDrawer.value.kind === 'service' && configDrawerExternalUrl.value))
const serviceDrawerExternalAccessToggleVisible = computed(() => configDrawer.value.kind === 'service' && serviceStatusHasRuntime(drawerService.value) && serviceDrawerProfile.value.showConnectionBindings)
const serviceDrawerExternalAccessLabel = computed(() => {
  if (serviceExternalAccessLoading.value) return serviceDrawerExternalAccessEnabled.value ? '关闭中...' : '开启中...'
  return serviceDrawerExternalAccessEnabled.value ? '关闭外部访问' : '开启外部访问'
})
const openComponentConfigDrawer = (comp:any) => {
  const actual = components.value.find((item:any) => Number(item.id) === Number(comp?.id)) || comp
  if (!actual) return
  selectComponent(actual.id)
  configDrawer.value = { visible: true, kind: 'component', component: actual, service: null, saving: false, error: '' }
  loadComponentConfigForm(actual)
  void ensureRegistryWorkspaces()
}
const openServiceConfigDrawer = (svc:any) => {
  const actual = services.value.find((item:any) => Number(item.id) === Number(svc?.id)) || svc
  if (!actual) return
  selectedTopologyKey.value = String(actual.topologyId || `service:${actual.id}`)
  configDrawer.value = { visible: true, kind: 'service', component: null, service: actual, saving: false, error: '' }
  configForm.value = defaultConfigForm()
  serviceConfigForm.value = serviceConfigFormFromInstallation(actual)
  void loadServiceDrawerWorkspace(actual)
}
const closeConfigDrawer = () => {
  if (configDrawer.value.saving) return
  configDrawer.value = { visible: false, kind: 'component', component: null, service: null, saving: false, error: '' }
  configForm.value = defaultConfigForm()
  serviceConfigForm.value = defaultServiceConfigForm()
}
const loadServiceDrawerWorkspace = async (svc:any) => {
  if (!svc?.id || serviceDrawerWorkspaceLoading.value || !serviceStatusHasRuntime(svc)) return
  serviceDrawerWorkspaceLoading.value = true
  try {
    const res = await api.getServiceWorkspace(envId.value, Number(svc.id))
    capabilityWorkspaceCache.value = { ...capabilityWorkspaceCache.value, [svc.id]: res.data }
  } catch {
    // The drawer remains usable for configuration even if live topology is not available yet.
  } finally {
    serviceDrawerWorkspaceLoading.value = false
  }
}
const loadComponentConfigForm = (comp:any) => {
  const cfg = parseRuntimeConfig(comp?.config)
  const runtime = comp?.runtimeConfig || {}
  const runtimeEnv = Array.isArray(runtime.env) ? runtime.env : []
  const envSource = cfg.env?.length ? cfg.env : runtimeEnv
  const image = String(comp?.image || comp?.registryImage || '')
  const imageParts = splitImageRepositoryAndTag(image)
  configForm.value = {
    image,
    repository: registryRepositorySuffix(imageParts.repository || image),
    version: String(comp?.version || imageParts.version || componentDeployVersion(comp) || ''),
    replicas: Number(runtime.replicas ?? comp?.replicas ?? 1),
    cpu: String(runtime.resources?.requests?.cpu || comp?.cpu || ''),
    memory: String(runtime.resources?.requests?.memory || comp?.memory || ''),
    env: envSource.map((item:any) => configEnvFromRuntime(item)),
    commandText: linesFromArray(cfg.command?.length ? cfg.command : runtime.command),
    argsText: linesFromArray(cfg.args?.length ? cfg.args : runtime.args),
  }
}
const configEnvFromRuntime = (item:any) => {
  if (item?.secretName || item?.secretKey) return { name: String(item.name || ''), source: 'secret' as const, value: '', refName: String(item.secretName || ''), refKey: String(item.secretKey || '') }
  if (item?.configMapName || item?.configMapKey) return { name: String(item.name || ''), source: 'configMap' as const, value: '', refName: String(item.configMapName || ''), refKey: String(item.configMapKey || '') }
  return { name: String(item?.name || ''), source: 'value' as const, value: String(item?.value || ''), refName: '', refKey: '' }
}
const linesFromArray = (items:any) => Array.isArray(items) ? items.map((item:any) => String(item).trim()).filter(Boolean).join('\n') : ''
const arrayFromLines = (text:string) => String(text || '').split('\n').map(item => item.trim()).filter(Boolean)
const addConfigEnv = () => {
  configForm.value.env.push({ name: '', source: 'value', value: '', refName: '', refKey: '' })
}
const removeConfigEnv = (idx:number) => {
  configForm.value.env.splice(idx, 1)
}
const configFormPayload = () => {
  const env = configForm.value.env
    .map((item:any) => {
      const name = String(item.name || '').trim()
      if (!name) return null
      if (item.source === 'secret') return { name, secretName: String(item.refName || '').trim(), secretKey: String(item.refKey || '').trim() }
      if (item.source === 'configMap') return { name, configMapName: String(item.refName || '').trim(), configMapKey: String(item.refKey || '').trim() }
      return { name, value: String(item.value || '') }
    })
    .filter(Boolean)
  const cfg = parseRuntimeConfig(configDrawer.value.component?.config)
  return {
    env,
    dependencies: cfg.dependencies || [],
    command: arrayFromLines(configForm.value.commandText),
    args: arrayFromLines(configForm.value.argsText),
  }
}
const saveConfigDrawer = async (options: { refresh?: boolean } = {}) => {
  const comp = configDrawer.value.component
  if (!comp?.id || configDrawer.value.saving) return
  const shouldRefresh = options.refresh !== false
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  try {
    const version = String(configForm.value.version || '').trim()
    if (version && version.toLowerCase() === 'latest') {
      configDrawer.value.error = '镜像 Tag 不能使用 latest。'
      return
    }
    const image = registryImageFromConfig.value || String(configForm.value.image || '').trim()
    const res = await api.updateComponent(Number(comp.id), {
      image,
      version,
      replicas: Number(configForm.value.replicas || 0),
      cpu: configForm.value.cpu,
      memory: configForm.value.memory,
      config: configFormPayload(),
    })
    const updated = res.data
    components.value = components.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    if (shouldRefresh) {
      await refreshServices()
    }
    const next = components.value.find((item:any) => Number(item.id) === Number(updated.id)) || updated
    configDrawer.value.component = next
    loadComponentConfigForm(next)
    selectComponent(updated.id)
    return updated
  } catch (e:any) {
    configDrawer.value.error = '保存配置失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const saveConfigDrawerIfComponent = async (options: { refresh?: boolean } = {}) => {
  if (configDrawer.value.kind !== 'component' || !configDrawer.value.component?.id) return true
  const saved = await saveConfigDrawer(options)
  return configDrawer.value.error ? false : (saved || true)
}
const saveServiceConfigDrawer = async () => {
  const svc = configDrawer.value.service
  if (!svc?.id || configDrawer.value.saving) return
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  try {
    const values = serviceConfigValuesFromForm(svc.serviceType, serviceConfigForm.value)
    const res = await api.updateService(envId.value, Number(svc.id), { values })
    const updated = res.data
    services.value = services.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    await refreshServices()
    const next = services.value.find((item:any) => Number(item.id) === Number(updated.id)) || updated
    configDrawer.value.service = next
    serviceConfigForm.value = serviceConfigFormFromInstallation(next)
    await loadServiceDrawerWorkspace(next)
  } catch (e:any) {
    configDrawer.value.error = '保存服务配置失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const deployServiceFromDrawer = async () => {
  const svc = configDrawer.value.service
  if (!svc?.id || configDrawer.value.saving || !serviceStatusCanDeploy(svc)) return
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  try {
    const values = serviceDrawerProfile.value.showDeploymentConfig
      ? serviceConfigValuesFromForm(svc.serviceType, serviceConfigForm.value)
      : {}
    const res = await api.installService(envId.value, { serviceType: svc.serviceType, values })
    const updated = res.data
    if (updated?.id) {
      services.value = services.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    }
    await refreshServices()
    const next = services.value.find((item:any) => Number(item.id) === Number(updated?.id || svc.id))
      || services.value.find((item:any) => item.serviceType === svc.serviceType)
      || updated
      || svc
    configDrawer.value.service = next
    serviceConfigForm.value = serviceConfigFormFromInstallation(next)
    await loadServiceDrawerWorkspace(next)
    notifyEnvUpdated()
  } catch (e:any) {
    configDrawer.value.error = '部署服务失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const toggleServiceExternalAccess = async () => {
  const svc = drawerService.value
  if (!svc?.id || serviceExternalAccessLoading.value) return
  serviceExternalAccessLoading.value = true
  configDrawer.value.error = ''
  try {
    const nextEnabled = !serviceDrawerExternalAccessEnabled.value
    const res = await api.setServiceExternalAccess(envId.value, Number(svc.id), nextEnabled)
    if (Array.isArray(res.externalAccess)) {
      externalAccess.value = res.externalAccess
    }
    const updated = res.data
    if (updated?.id) {
      services.value = services.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    } else {
      await refreshServices()
    }
    const next = services.value.find((item:any) => Number(item.id) === Number(svc.id)) || updated || svc
    configDrawer.value.service = next
    serviceConfigForm.value = serviceConfigFormFromInstallation(next)
    await loadServiceDrawerWorkspace(next)
    notifyEnvUpdated()
  } catch (e:any) {
    configDrawer.value.error = '外部访问切换失败：' + (e?.message || '未知错误')
  } finally {
    serviceExternalAccessLoading.value = false
  }
}
const deployDrawerComponent = async () => {
  const comp = configDrawer.value.component
  if (!comp?.id) return
  const saved = await saveConfigDrawerIfComponent({ refresh: false })
  if (!saved) return
  const next = saved === true ? (components.value.find((item:any) => Number(item.id) === Number(comp.id)) || comp) : saved
  await deployComponent(next)
  const refreshed = components.value.find((item:any) => Number(item.id) === Number(comp.id))
  if (refreshed) {
    configDrawer.value.component = refreshed
    loadComponentConfigForm(refreshed)
  }
}
const closeDeleteDialog = () => {
  if (pendingDeleteDialog.value?.submitting) return
  pendingDeleteDialog.value = null
}
const runPendingDelete = async () => {
  const dialog = pendingDeleteDialog.value
  if (!dialog || dialog.submitting) return
  dialog.submitting = true
  dialog.error = ''
  try {
    await performDeleteComponent(dialog.target)
    pendingDeleteDialog.value = null
  } catch (e:any) {
    dialog.error = e?.message || '删除失败'
  } finally {
    if (pendingDeleteDialog.value) pendingDeleteDialog.value.submitting = false
  }
}
const topologyDeleteTitle = (node:any) => node?.topologyKind === 'service' ? '删除卡片' : '删除组件'
const resolveTopologyService = (node:any) => {
  if (!node) return null
  const serviceId = Number(node.serviceId || node.id)
  if (Number.isFinite(serviceId) && serviceId > 0) {
    return services.value.find((item:any) => Number(item.id) === serviceId) || { ...node, id: serviceId }
  }
  const topologyId = String(node.topologyId || '')
  if (topologyId) {
    return services.value.find((item:any) => String(item.topologyId || `service:${item.id}`) === topologyId) || node
  }
  return node
}
const deleteTopologyNode = (node:any) => {
  if (node?.topologyKind === 'service') {
    const svc = resolveTopologyService(node)
    if (svc) beginUninstallService(svc)
    return
  }
  void deleteComponentById(node)
}
const deleteComponentById = async (comp:any) => {
  if (!comp?.id || componentActionLoading.value) return
  pendingDeleteDialog.value = {
    kind: 'component',
    label: '组件',
    name: String(comp.name || comp.id),
    message: '删除后会移除组件记录、画布位置、连线，并清理该组件在集群中的运行态资源。',
    target: comp,
    submitting: false,
    error: '',
  }
}
const performDeleteComponent = async (comp:any) => {
  if (!comp?.id || componentActionLoading.value) return
  componentActionLoading.value = true
  pageError.value = ''
  try {
    await api.deleteComponent(Number(comp.id))
    components.value = components.value.filter((item:any) => Number(item.id) !== Number(comp.id))
    const key = `component:${comp.id}`
    const nextPositions = { ...componentNodePositions.value }
    delete nextPositions[key]
    componentNodePositions.value = nextPositions
    manualCanvasEdges.value = manualCanvasEdges.value.filter(edge => edge.fromKey !== key && edge.toKey !== key)
    selectedComponentId.value = null
    selectedTopologyKey.value = null
    await saveComponentNodePositions()
    await refreshServices()
    notifyEnvUpdated()
  } catch (e:any) {
    throw new Error('删除组件失败：' + (e?.message || '未知错误'))
  } finally {
    componentActionLoading.value = false
    closeComponentContextMenu()
  }
}
const deleteDrawerComponent = async () => {
  const comp = configDrawer.value.component
  if (!comp?.id) return
  closeConfigDrawer()
  await deleteComponentById(comp)
}
const runtimeEnvValue = (envItem:any) => {
  if (envItem?.secretName) return `Secret ${envItem.secretName}/${envItem.secretKey || '-'}`
  if (envItem?.configMapName) return `ConfigMap ${envItem.configMapName}/${envItem.configMapKey || '-'}`
  return envItem?.value || '-'
}
const runtimeObjectSummary = (items:any[]) => Array.isArray(items) && items.length
  ? items.map((item:any) => `${item.name}${Array.isArray(item.keys) && item.keys.length ? ` (${item.keys.join(', ')})` : ''}`).join('；')
  : '未发现'
const runtimeEnvFromSummary = (items:any[]) => Array.isArray(items) && items.length
  ? items.map((item:any) => `${item.kind}:${item.name}`).join('；')
  : '未发现'
const configureContextNode = () => {
  const comp = componentContextMenu.value.component
  const svc = componentContextMenu.value.service
  closeComponentContextMenu()
  if (comp?.id) openComponentConfigDrawer(comp)
  else if (svc?.id) openServiceConfigDrawer(svc)
}
const deployContextComponent = () => {
  const comp = componentContextMenu.value.component
  if (comp) void deployComponent(comp)
}
const deployContextService = () => {
  const svc = componentContextMenu.value.service
  closeComponentContextMenu()
  if (!svc) return
  openServiceConfigDrawer(svc)
  void deployServiceFromDrawer()
}
const deleteContextComponent = () => {
  const comp = componentContextMenu.value.component
  if (comp) void deleteComponentById(comp)
}
const deleteContextService = () => {
  const svc = componentContextMenu.value.service
  closeComponentContextMenu()
  if (svc) beginUninstallService(svc)
}
const deleteDrawerService = () => {
  const svc = configDrawer.value.service
  closeConfigDrawer()
  if (svc) beginUninstallService(svc)
}
const closeRelationshipModal = () => {
  if (relationshipSubmitting.value) return
  showRelationshipModal.value = false
  relationshipSourceComponent.value = null
  relationshipSelectedKeys.value = []
  relationshipError.value = ''
}
const saveComponentRelationships = async () => {
  const comp = relationshipSourceComponent.value
  if (!comp?.id || relationshipSubmitting.value) return
  relationshipSubmitting.value = true
  relationshipError.value = ''
  try {
    const cfg = parseRuntimeConfig(comp.config)
    const dependencies = Array.from(new Set(relationshipSelectedKeys.value.map(item => item.trim()).filter(Boolean)))
    const res = await api.updateComponent(Number(comp.id), {
      config: {
        env: cfg.env,
        dependencies,
        command: cfg.command,
        args: cfg.args,
      },
    })
    const updated = res.data
    components.value = components.value.map((item:any) => Number(item.id) === Number(updated.id) ? updated : item)
    selectComponent(updated.id)
    showRelationshipModal.value = false
    relationshipSourceComponent.value = null
    relationshipSelectedKeys.value = []
  } catch (e:any) {
    relationshipError.value = '保存关系失败：' + (e?.message || '未知错误')
  } finally {
    relationshipSubmitting.value = false
  }
}
const openContextNodeMonitoring = () => {
  capabilityInitialSubjectKey.value = contextSubjectTargetKey('monitor', componentContextMenu.value.component, componentContextMenu.value.service)
  closeComponentContextMenu()
  setActiveTab('monitoring-center')
}
const openContextNodeLogs = () => {
  capabilityInitialSubjectKey.value = contextSubjectTargetKey('log', componentContextMenu.value.component, componentContextMenu.value.service)
  closeComponentContextMenu()
  setActiveTab('logging-center')
}
const subjectTargetKey = (type: string, kind: string, namespace: string, name: string) =>
  [type, kind, namespace, name].map(item => String(item || '').trim()).join(':')
const componentRuntimeIdentifier = (comp:any) => {
  const path = String(comp?.gitPath || '').trim()
  if (path.includes('/')) return path.split('/').filter(Boolean).pop() || ''
  const image = String(comp?.registryImage || comp?.image || '').trim()
  const imageName = image.split('/').pop()?.split(':')[0] || ''
  return imageName
}
const contextSubjectTargetKey = (type: 'monitor' | 'log', comp:any, svc:any) => {
  if (comp?.id) {
    if (type === 'monitor') return subjectTargetKey(type, 'component', env.value?.namespace || '', comp.name)
    return subjectTargetKey(type, 'pod-prefix', env.value?.namespace || '', componentRuntimeIdentifier(comp) || comp.name)
  }
  if (svc?.id) {
    return `${type}:service:${svc.id}`
  }
  return ''
}
const uniqueCanvasComponentName = (type: string) => {
  const prefix = type === 'frontend' ? 'frontend' : type === 'backend' ? 'backend' : 'component'
  const names = new Set(components.value.map((item:any) => String(item.name || '').toLowerCase()))
  for (let i = 1; i < 1000; i++) {
    const candidate = `${prefix}-${i}`
    if (!names.has(candidate.toLowerCase())) return candidate
  }
  return `${prefix}-${Date.now()}`
}
const createCanvasComponentDraft = async (type: string) => {
  const createPoint = canvasCreatePoint.value
  closeComponentContextMenu()
  pageError.value = ''
  try {
    const res = await api.createComponent(envId.value, {
      name: uniqueCanvasComponentName(type),
      type,
      replicas: 1,
      draftOnly: true,
    })
    await refreshServices()
    const created = res.data
    const actual = components.value.find((item:any) => Number(item.id) === Number(created?.id)) || created
    if (actual?.id && createPoint) {
      componentNodePositions.value = {
        ...componentNodePositions.value,
        [`component:${actual.id}`]: {
          x: Math.max(12, createPoint.x - componentCanvasMetrics.nodeWidth / 2),
          y: Math.max(46, createPoint.y - componentCanvasMetrics.nodeHeight / 2),
        },
      }
      await saveComponentNodePositions()
    }
    if (actual) {
      setActiveTab('components')
      openComponentConfigDrawer(actual)
    }
  } catch (e:any) {
    pageError.value = '创建组件草稿失败：' + (e?.message || '未知错误')
  }
}
const createCanvasServiceDraft = async (serviceType: string) => {
  const createPoint = canvasCreatePoint.value
  closeComponentContextMenu()
  pageError.value = ''
  try {
    const beforeIds = new Set(services.value.map((item:any) => Number(item.id)))
    await api.createServiceDraft(envId.value, { serviceType })
    await refreshServices()
    const installed = services.value.find((item:any) => !beforeIds.has(Number(item.id)) && item.serviceType === serviceType)
      || services.value.find((item:any) => item.serviceType === serviceType)
    if (installed) {
      if (createPoint) {
        const key = `service:${installed.id}`
        componentNodePositions.value = {
          ...componentNodePositions.value,
          [key]: {
            x: Math.max(12, createPoint.x - componentCanvasMetrics.nodeWidth / 2),
            y: Math.max(46, createPoint.y - componentCanvasMetrics.nodeHeight / 2),
          },
        }
        await saveComponentNodePositions()
      }
      setActiveTab('components')
      openServiceConfigDrawer(installed)
    }
  } catch (e:any) {
    pageError.value = '添加服务草稿失败：' + (e?.message || '未知错误')
  }
}
const adoptCanvasResource = async () => {
 closeComponentContextMenu()
  showAdoptResourceModal.value = true
  adoptResourceLoading.value = true
  adoptResourceSubmitting.value = false
  adoptResourceError.value = ''
  adoptResourceSelection.value = ''
  try {
    const res = await api.listAdoptableResources(envId.value)
    adoptableResources.value = res.data || []
    adoptResourceSelection.value = adoptableResources.value[0]?.key || ''
  } catch (e:any) {
    adoptableResources.value = []
    adoptResourceError.value = '读取可接入资源失败：' + (e?.message || '未知错误')
  } finally {
    adoptResourceLoading.value = false
  }
}
const closeAdoptResourceModal = () => {
  if (adoptResourceSubmitting.value) return
  showAdoptResourceModal.value = false
  adoptResourceError.value = ''
  adoptResourceSelection.value = ''
}
const submitAdoptResource = async () => {
  if (!adoptResourceSelection.value || adoptResourceSubmitting.value) return
  adoptResourceSubmitting.value = true
  adoptResourceError.value = ''
  try {
    const res = await api.adoptResource(envId.value, { key: adoptResourceSelection.value })
    await refreshServices()
    const adoptedId = res.data?.id
    const adopted = adoptedId
      ? components.value.find((item:any) => Number(item.id) === Number(adoptedId))
      : components.value.find((item:any) => item.name === res.data?.name)
    showAdoptResourceModal.value = false
    setActiveTab('components')
    if (adopted) openComponentConfigDrawer(adopted)
    notifyEnvUpdated()
  } catch (e:any) {
    adoptResourceError.value = '接入失败：' + (e?.message || '未知错误')
  } finally {
    adoptResourceSubmitting.value = false
  }
}

const openServiceModal = () => {
  void prepareServicePicker('tool')
}

const envStatusText = (status: string | undefined) => {
  const s = String(status || '').toLowerCase()
  if (s === 'running' || s === 'ready') return '运行中'
  if (s === 'creating') return '创建中'
  if (s === 'deleting') return '删除中'
  if (s === 'failed' || s === 'error') return '异常'
  if (!s) return '空环境'
  return s
}
const selectServiceTemplate = (svc:any) => {
  if (svc.disabled) {
    serviceModalError.value = `${svc.name} 已添加、已安装或正在安装。`
    return
  }
  serviceModalError.value = ''
  serviceForm.value.serviceType = svc.type
}

const submitComponent = async () => {
  const image = compForm.value.image.trim()
  const version = compForm.value.version.trim()
  const deliveryMode = compForm.value.deliveryMode === 'source' ? 'source' : 'image'
  const sourceRepoUrl = compForm.value.sourceRepoUrl.trim()
  const sourceBranch = compForm.value.sourceBranch.trim() || 'main'
  const buildContext = compForm.value.buildContext.trim() || '.'
  componentModalError.value = ''
  if (!compForm.value.name.trim()) { componentModalError.value = '请填写组件名称'; return }
  if (deliveryMode === 'image' && (!version || version.toLowerCase() === 'latest')) { componentModalError.value = '请填写明确版本号，不能使用 latest'; return }
  if (deliveryMode === 'source' && version && version.toLowerCase() === 'latest') { componentModalError.value = '源码交付版本不能使用 latest'; return }
  if (deliveryMode === 'image' && !image) { componentModalError.value = '请填写镜像地址'; return }
  if (deliveryMode === 'source' && !sourceRepoUrl) { componentModalError.value = '请填写源码仓库地址'; return }
  const payload = deliveryMode === 'source'
    ? { ...compForm.value, deliveryMode, sourceRepoUrl, sourceBranch, buildContext, image: '', version }
    : { ...compForm.value, deliveryMode, image, version }
  try {
    await api.createComponent(envId.value, payload)
    const res = await api.getEnv(envId.value)
    components.value = res.data.components || []
    showComponentModal.value = false
    compForm.value = defaultComponentForm()
    notifyEnvUpdated()
  }
  catch(e:any) { componentModalError.value = '创建失败：' + (e?.message || '未知错误') }
}
const submitService = async () => {
  if (!serviceForm.value.serviceType) return
  const selectedType = serviceForm.value.serviceType
  if (isActiveServiceInstalled(selectedType)) {
    serviceModalError.value = `${svcLabel(selectedType)} 已添加、已安装或正在安装。`
    availableServices.value = filterTemplates(serviceModalMode.value)
    serviceForm.value.serviceType = availableServices.value.find((svc:any) => !svc.disabled)?.type || ''
    return
  }
  serviceSubmitting.value = true
  serviceModalError.value = ''
  serviceModalNotice.value = ''
  try {
    const beforeIds = new Set(services.value.map((item:any) => Number(item.id)))
    await api.installService(envId.value, { serviceType: selectedType })
    await refreshServices()
    const installed = services.value.find((item:any) => !beforeIds.has(Number(item.id)) && item.serviceType === selectedType)
      || services.value.find((item:any) => item.serviceType === selectedType)
    availableServices.value = filterTemplates(serviceModalMode.value)
    serviceForm.value.serviceType = availableServices.value.find((svc:any) => !svc.disabled)?.type || ''
    serviceModalNotice.value = pickerNotice(serviceModalMode.value, availableServices.value.length, serviceForm.value.serviceType)
    showServiceModal.value = false
    if (installed) {
      setActiveTab('components')
      openServiceConfigDrawer(installed)
    }
  }
  catch(e:any) { serviceModalError.value = '安装失败：' + (e?.message || '未知错误') }
  finally { serviceSubmitting.value = false }
}

const beginUninstallService = (svc:any) => {
  pendingUninstallService.value = svc
  uninstallError.value = ''
  pageError.value = ''
}

const closeUninstallDialog = () => {
  if (uninstallSubmitting.value) return
  pendingUninstallService.value = null
  uninstallError.value = ''
}

const confirmUninstallService = async () => {
  const svc = pendingUninstallService.value
  if (!svc) return
  pageError.value = ''
  uninstallError.value = ''
  uninstallSubmitting.value = true
  try {
    await performDeleteService(svc)
    pendingUninstallService.value = null
  }
  catch(e:any) { uninstallError.value = '卸载失败：' + (e?.message || '未知错误') }
  finally { uninstallSubmitting.value = false }
}

const performDeleteService = async (svc:any) => {
  if (!svc?.id) return
  await api.uninstallService(envId.value, svc.id)
  const key = `service:${svc.id}`
  const nextPositions = { ...componentNodePositions.value }
  delete nextPositions[key]
  componentNodePositions.value = nextPositions
  manualCanvasEdges.value = manualCanvasEdges.value.filter(edge => edge.fromKey !== key && edge.toKey !== key)
  selectedTopologyKey.value = null
  await saveComponentNodePositions()
  await refreshServices()
  notifyEnvUpdated()
}
</script>

<style scoped>
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
  max-width: none;
}

/* Title bar */
.page-title-bar {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: var(--paap-space-6);
  margin-bottom: var(--paap-space-6);
  padding: var(--paap-space-7);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.page-title-bar::after {
  content: '';
  position: absolute;
  left: var(--paap-space-7);
  right: var(--paap-space-7);
  bottom: -1px;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(37,99,235,0.3), transparent);
}
.title-group { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.title-row { display: flex; align-items: center; gap: var(--paap-space-3); flex-wrap: wrap; }
.page-title { font-size: 28px; font-weight: 600; color: var(--paap-text); line-height: 1.2; letter-spacing: -0.02em; margin: 0; }
.title-id { font-family: var(--paap-mono); font-size: 12px; color: var(--paap-muted); letter-spacing: 0.02em; }
.status-badge {
  display: inline-flex; align-items: center; gap: 5px;
  font-size: 11px; font-weight: 500; padding: 2px 10px; border-radius: var(--paap-radius-full);
  background: #f3f4f6; color: var(--paap-muted);
}
.status-badge.running { background: var(--paap-success-soft); color: #059669; }
.status-badge.error { background: var(--paap-danger-soft); color: var(--paap-danger); }
.status-badge.creating { background: var(--paap-accent-soft); color: var(--paap-accent); }
.title-actions { flex-shrink: 0; display: flex; gap: var(--paap-space-2); align-self: center; flex-wrap: wrap; justify-content: flex-end; }
.page-error {
  border: 1px solid #fecaca;
  background: var(--paap-danger-soft);
  color: #991b1b;
  border-radius: var(--paap-radius);
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
  margin-bottom: var(--paap-space-4);
}

.environment-shell {
  min-width: 0;
}

/* Tab Panel */
.tab-panel {
  border: none;
  border-radius: 0;
  background: transparent;
  overflow: visible;
  min-width: 0;
}

/* Overview */
.overview-panel {
  display: grid;
  gap: var(--paap-space-6);
  padding: 0;
  background: transparent;
}
.overview-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--paap-space-3);
  margin-bottom: 0;
}
.overview-stat {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-height: 132px;
  padding: var(--paap-space-5);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  color: var(--paap-text);
  text-align: left;
}
button.overview-stat { cursor: pointer; transition: border-color 0.15s; }
button.overview-stat:hover { border-color: var(--paap-border-strong); }
.stat-label { color: var(--paap-muted); font-size: 13px; font-weight: 500; }
.stat-value { margin-top: var(--paap-space-4); color: var(--paap-text); font-size: 42px; font-weight: 700; line-height: 1; letter-spacing: -0.02em; }
.stat-value.danger { color: var(--paap-danger); }
.stat-hint { margin-top: var(--paap-space-3); color: var(--paap-muted); font-size: 12px; }
.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 420px);
  gap: var(--paap-space-6);
  margin-bottom: 0;
}
.overview-section {
  padding: var(--paap-space-5);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.overview-section--anchor { scroll-margin-top: 88px; }
.overview-wide { margin-bottom: 0; }
.overview-section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--paap-space-3); margin-bottom: var(--paap-space-4); }
.overview-title { margin: 0; color: var(--paap-text); font-size: 18px; font-weight: 600; }
.overview-subtitle { color: var(--paap-muted); font-size: 12px; max-width: 520px; text-align: right; line-height: 1.4; }
.external-access-section { order: -2; }
.external-access-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: var(--paap-space-4);
}
.external-access-group {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.external-access-group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--paap-muted);
  font-size: 12px;
  font-weight: 600;
}
.external-access-group-head strong { color: var(--paap-text); }
.external-access-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  text-decoration: none;
}
.external-access-row:hover {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
}
.external-access-main {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.external-access-main strong,
.external-access-main small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.external-access-main strong { font-size: 13px; font-weight: 650; }
.external-access-main small { color: var(--paap-muted); font-size: 12px; }
.component-focus { order: -1; }
.component-overview-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 340px);
  gap: var(--paap-space-4);
  align-items: stretch;
}
.component-overview-map {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: var(--paap-space-3);
  min-width: 0;
}
.component-overview-lane {
  display: grid;
  align-content: start;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: var(--paap-space-3);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}
.component-lane-label span {
  color: var(--paap-muted-2);
  font-weight: 600;
}
.component-overview-node {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 2px var(--paap-space-2);
  width: 100%;
  min-height: 44px;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  text-align: left;
  cursor: pointer;
}
.component-overview-node:hover {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
}
.component-overview-node strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: 12px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-overview-node small {
  grid-column: 2;
  color: var(--paap-muted);
  font-size: 11px;
}
.component-overview-summary {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
  padding: var(--paap-space-4);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}
.component-summary-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
  color: var(--paap-muted);
  font-size: 12px;
}
.component-summary-row strong {
  color: var(--paap-text);
  font-size: 14px;
  font-weight: 650;
}
.component-overview-edges {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.component-overview-edges button {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  min-height: 28px;
  padding: 0 9px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: inherit;
  font-size: 11px;
  cursor: pointer;
}
.component-overview-edges button:hover {
  border-color: var(--paap-accent);
  color: var(--paap-accent);
}
.component-overview-edges span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-list { display: flex; flex-direction: column; }
.flow-step { display: grid; grid-template-columns: 14px minmax(0, 1fr) auto; align-items: start; gap: var(--paap-space-3); padding: 13px 0; border-bottom: 1px solid #f3f4f6; }
.flow-step:last-child { border-bottom: none; }
.flow-dot { width: 9px; height: 9px; margin-top: 5px; border-radius: 50%; background: var(--paap-border-strong); }
.flow-step.ready .flow-dot { background: var(--paap-success); }
.flow-step.pending .flow-dot { background: var(--paap-warning); }
.flow-step.missing .flow-dot { background: var(--paap-danger); }
.flow-name { color: var(--paap-text); font-size: 13px; font-weight: 600; }
.flow-desc { margin-top: 2px; color: var(--paap-muted); font-size: 12px; line-height: 1.4; }
.flow-link, .text-btn {
  border: none;
  background: transparent;
  color: var(--paap-accent);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.flow-link:hover, .text-btn:hover { text-decoration: underline; }
.quick-list, .compact-list { display: flex; flex-direction: column; }
.quick-row, .compact-row {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  width: 100%;
  padding: 12px 0;
  border: none;
  border-bottom: 1px solid #f3f4f6;
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.quick-row:last-child, .compact-row:last-child { border-bottom: none; }
.quick-row:hover, .compact-row:hover { background: var(--paap-panel-subtle); }
.quick-icon { display: inline-flex; align-items: center; justify-content: center; width: 28px; color: var(--paap-muted); flex-shrink: 0; }
.quick-icon.git { color: #cc8888; }
.quick-icon.ci { color: #c88ba8; }
.quick-icon.registry, .quick-icon.harbor { color: #d4b072; }
.quick-icon.deploy { color: #8b9dc3; }
.quick-icon.monitor { color: #7bb896; }
.quick-icon.log { color: #a090c8; }
.quick-main { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.quick-name, .compact-name { color: var(--paap-text); font-size: 13px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.quick-desc { color: var(--paap-muted); font-size: 12px; margin-top: 2px; }
.overview-empty {
  display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-3);
  padding: var(--paap-space-4);
  border: 1px dashed var(--paap-border-strong); border-radius: var(--paap-radius);
  background: var(--paap-panel-subtle); color: var(--paap-muted); font-size: 13px;
}
.overview-two-col { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--paap-space-4); }
.compact-head { display: flex; align-items: center; justify-content: space-between; padding-bottom: var(--paap-space-2); color: var(--paap-muted); font-size: 12px; font-weight: 600; }
.compact-empty { padding: var(--paap-space-5) 0; color: var(--paap-muted-2); font-size: 13px; }

/* Capability workspace */
.capability-workspace {
  display: grid;
  gap: var(--paap-space-4);
}
.capability-workspace-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-4);
  padding: 0 2px var(--paap-space-1);
}
.capability-workspace-title {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.capability-workspace-title strong {
  color: var(--paap-text);
  font-size: 16px;
  font-weight: 600;
}
.capability-workspace-title span {
  color: var(--paap-muted);
  font-size: 12px;
  line-height: 1.4;
}
.capability-workspace-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.capability-service-pill {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.capability-service-pill.active {
  border-color: #bfdbfe;
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.capability-external-link {
  display: inline-flex;
  align-items: center;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel);
  color: var(--paap-accent);
  text-decoration: none;
  font-size: 12px;
  font-weight: 600;
}
.capability-external-link:hover {
  border-color: #bfdbfe;
  background: var(--paap-accent-soft);
  text-decoration: none;
}
.text-btn.danger { color: var(--paap-danger); }
.workspace-message {
  border: 1px solid #bbf7d0;
  background: var(--paap-success-soft);
  color: #047857;
  border-radius: var(--paap-radius);
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
}
.workspace-loading {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  padding: 10px 12px;
  font-size: 12px;
}
.compact-empty-state {
  padding: var(--paap-space-10) 0;
}

/* Service list */
.service-list {
  display: grid;
  gap: var(--paap-space-3);
}
.service-row {
  display: flex; align-items: center;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  transition: border-color 0.15s; cursor: pointer;
}
.service-row:hover { border-color: var(--paap-border-strong); }
.service-body {
  padding: var(--paap-space-5); flex: 1; display: flex; align-items: center; gap: var(--paap-space-4);
}
.service-main { flex: 1; min-width: 0; }
.service-header {
  display: flex; justify-content: space-between; align-items: flex-start;
  gap: var(--paap-space-3); margin-bottom: 4px;
}
.service-name-group { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; }
.service-name-group span:first-child { font-weight: 600; font-size: 15px; line-height: 1.4; color: var(--paap-text); }
.service-desc { color: var(--paap-muted); line-height: 1.4; font-size: 13px; }
.service-error { color: var(--paap-danger); line-height: 1.4; font-size: 12px; margin-top: 4px; max-width: 760px; overflow-wrap: anywhere; }
.service-meta { color: var(--paap-muted-2); font-size: 12px; margin-top: 4px; }

.service-action {
  width: 32px; height: 32px; display: flex; align-items: center; justify-content: center;
  background: transparent; border: none; color: var(--paap-muted-2); cursor: pointer;
  transition: all 0.15s; flex-shrink: 0; margin-top: -4px; border-radius: var(--paap-radius-xs);
  opacity: 0;
}
.service-row:hover .service-action { opacity: 1; }
.service-action:hover { background: var(--paap-danger-soft); color: var(--paap-danger); }

/* Component topology workspace */
.component-workspace { display: grid; gap: var(--paap-space-4); }
.component-filter-bar,
.component-dependency-map,
.component-detail-panel,
.component-table-panel {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.component-dependency-map--canvas-only {
  padding: 0;
  overflow: hidden;
}
.component-dependency-map--canvas-only .component-map-head {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-3);
  margin-bottom: 0;
  border-bottom: 1px solid var(--paap-border);
}
.component-filter-bar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--paap-space-4);
}
.component-filter-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.component-search-input,
.component-type-select {
  height: 34px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  font-size: 13px;
  outline: none;
}
.component-search-input {
  width: min(320px, 32vw);
  padding: 0 12px;
}
.component-type-select {
  min-width: 136px;
  padding: 0 32px 0 10px;
}
.component-search-input:focus,
.component-type-select:focus {
  border-color: var(--paap-accent);
  box-shadow: 0 0 0 3px rgba(37,99,235,0.1);
}
.component-topology-layout {
  display: block;
}
.component-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
  margin-bottom: var(--paap-space-4);
}
.component-map-head,
.component-list-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  margin-bottom: var(--paap-space-4);
}
.component-map-tags {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.application-set-deploy-btn {
  height: 32px;
  border-radius: 0;
}
.application-set-deploy-btn--blocked {
  border-color: var(--cds-border-subtle-01, #c6c6c6);
  background: var(--cds-layer-02, #ffffff);
  color: var(--cds-text-disabled, rgba(22, 22, 22, 0.25));
  cursor: not-allowed;
}
.application-set-deploy-hint {
  margin: -4px var(--paap-space-5) var(--paap-space-3);
  padding: var(--cds-spacing-03, 8px) var(--cds-spacing-04, 12px);
  border-left: 3px solid var(--cds-yellow-40, #d2a106);
  background: var(--cds-yellow-10, #fcf4d6);
  color: var(--cds-text-primary, #161616);
  font-size: var(--cds-helper-text-01-font-size, 12px);
  line-height: var(--cds-helper-text-01-line-height, 1.33333);
}
.component-map-title {
  color: var(--paap-text);
  font-size: 16px;
  font-weight: 600;
  line-height: 1.3;
  margin: 0;
}
.component-map-desc {
  color: var(--paap-muted);
  font-size: 12px;
  line-height: 1.5;
  margin-top: 3px;
}
.component-topology-canvas {
  position: relative;
  min-height: 320px;
  overflow-x: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background:
    radial-gradient(circle at 1px 1px, rgba(141, 141, 141, 0.34) 1px, transparent 0),
    #ffffff;
  background-size: 18px 18px;
}
.topology-controls {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 100;
  display: flex;
  gap: 4px;
  align-items: center;
  padding: 6px;
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid var(--paap-border);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}
.topology-control-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--paap-border);
  border-radius: 6px;
  background: white;
  color: var(--paap-text);
  cursor: pointer;
  transition: all 0.15s;
  font-family: inherit;
}
.topology-control-btn:hover {
  background: var(--paap-accent-soft);
  border-color: var(--paap-accent);
  color: var(--paap-accent);
}
.topology-zoom-label {
  margin-left: 4px;
  padding: 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--paap-muted);
  border-left: 1px solid var(--paap-border);
  min-width: 40px;
  text-align: center;
}
.component-topology-canvas--main {
  min-height: 720px;
  max-height: calc(100vh - 190px);
  border: 0;
  border-radius: 0;
}
.overview-detail-panel {
  display: grid;
  gap: var(--paap-space-5);
}
.environment-topology-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(240px, 0.28fr);
  gap: var(--paap-space-4);
  align-items: stretch;
}
.environment-topology-workspace--primary {
  display: block;
}
.environment-topology-canvas {
  min-height: 500px;
}
.environment-topology-canvas--primary {
  min-height: 800px;
  max-height: none;
}
.component-canvas-stage {
  position: relative;
}
.component-canvas-empty-hint {
  position: absolute;
  left: 96px;
  top: 96px;
  display: grid;
  gap: 8px;
  width: 360px;
  padding: 24px;
  border: 1px dashed var(--paap-border-strong);
  background: rgba(255, 255, 255, 0.88);
  color: var(--paap-muted);
  pointer-events: none;
}
.component-canvas-empty-hint strong {
  color: var(--paap-text);
  font-size: 16px;
}
.component-topology-lane {
  position: absolute;
  top: var(--paap-space-3);
  bottom: var(--paap-space-3);
  min-width: 0;
  padding: 10px 12px;
  border: 0;
  background: transparent;
  pointer-events: none;
}
.component-lane-label {
  color: var(--paap-muted);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.component-lane-label span {
  color: var(--paap-text);
  font-weight: 700;
}
.component-canvas-links {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}
.component-canvas-link {
  fill: none;
  stroke: #0f62fe;
  stroke-width: 2.2;
  stroke-dasharray: 7 5;
  stroke-linecap: round;
  stroke-linejoin: round;
  marker-end: url(#component-arrow);
  animation: dash-flow 20s linear infinite;
  transition: stroke 0.2s, stroke-width 0.2s;
}
.component-canvas-link:hover,
.component-canvas-link.active {
  stroke: #0043ce;
  stroke-width: 2.8;
}
@keyframes dash-flow {
  to { stroke-dashoffset: -240; }
}
.environment-canvas-link {
  marker-end: url(#environment-arrow);
}
.component-arrow-head {
  fill: #0f62fe;
}
.component-canvas-link.active {
  stroke: var(--paap-accent);
  stroke-width: 2.5;
}
.component-topology-node {
  position: absolute;
  display: grid;
  grid-template-columns: 36px auto minmax(0, 1fr);
  grid-template-rows: minmax(0, auto) minmax(0, auto);
  column-gap: 10px;
  row-gap: 2px;
  align-items: center;
  padding: 12px 14px;
  border: 2px solid #d0d7de;
  border-radius: 8px;
  background: linear-gradient(135deg, #ffffff 0%, #f9fafb 100%);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1), 0 1px 2px rgba(0, 0, 0, 0.06);
  box-sizing: border-box;
  text-align: left;
  cursor: grab;
  transition: border-color 0.2s ease, background 0.2s ease, box-shadow 0.2s ease;
  touch-action: none;
}
.component-topology-node:active {
  cursor: grabbing;
}
.component-topology-node:hover,
.component-topology-node.active {
  border-color: #0f62fe;
  background: linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%);
  box-shadow: 0 4px 12px rgba(15, 98, 254, 0.15), 0 0 0 3px rgba(15, 98, 254, 0.1);
  z-index: 10;
}
.component-topology-node strong { color: var(--paap-text); font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.component-topology-node small { grid-column: 3; color: var(--paap-muted); font-size: 11px; }
.node-delete-action {
  position: absolute;
  top: 6px;
  right: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--cds-gray-60, #6f6f6f);
  cursor: pointer;
  opacity: 0;
  transition: opacity var(--cds-motion-duration-fast, 110ms), background var(--cds-motion-duration-fast, 110ms), color var(--cds-motion-duration-fast, 110ms);
}
.component-topology-node:hover .node-delete-action,
.component-topology-node:focus .node-delete-action,
.node-delete-action:focus {
  opacity: 1;
}
.node-delete-action:hover,
.node-delete-action:focus {
  border-color: var(--cds-red-50, #fa4d56);
  background: var(--cds-red-10, #fff1f1);
  color: var(--cds-red-60, #da1e28);
  outline: none;
}
.component-topology-node--service {
  background: linear-gradient(135deg, #fef9e7 0%, #fefbf3 100%);
  border-color: #e5be8a;
}
.component-topology-node--service:hover,
.component-topology-node--service.active {
  border-color: #d4a574;
  background: linear-gradient(135deg, #fef5e0 0%, #fef0d8 100%);
  box-shadow: 0 4px 12px rgba(212, 165, 116, 0.2), 0 0 0 3px rgba(212, 165, 116, 0.08);
}
.component-topology-node.selected {
  border-color: #0f62fe;
  box-shadow: 0 0 0 2px rgba(15, 98, 254, 0.35), 0 4px 12px rgba(15, 98, 254, 0.12);
}
.topology-marquee {
  fill: rgba(15, 98, 254, 0.08);
  stroke: #0f62fe;
  stroke-width: 1.5;
  stroke-dasharray: 5 3;
  pointer-events: none;
}
.node-type-icon {
  grid-row: 1 / span 2;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: #edf5ff;
  color: var(--paap-accent);
}
.node-type-icon--frontend { background: #fff1f1; color: #da1e28; }
.node-type-icon--backend { background: #edf5ff; color: #0f62fe; }
.node-type-icon--database,
.node-type-icon--postgresql,
.node-type-icon--mysql,
.node-type-icon--mongodb { background: #f6f2ff; color: #8a3ffc; }
.node-type-icon--redis,
.node-type-icon--middleware,
.node-type-icon--rabbitmq,
.node-type-icon--kafka { background: #fcf4d6; color: #b28600; }
.node-status { width: 7px; height: 7px; border-radius: 50%; background: var(--paap-border-strong); }
.node-status.running { background: var(--paap-success); }
.node-status.error, .node-status.failed { background: var(--paap-danger); }
.node-status.creating, .node-status.deploying, .node-status.building, .node-status.installing { background: var(--paap-accent); }
.node-status.draft, .node-status.pending { background: var(--paap-border-strong); }
.component-edge-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
  margin-top: var(--paap-space-4);
}
.component-edge {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.component-edge:hover,
.component-edge.active {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.component-edge-empty { margin-top: var(--paap-space-4); color: var(--paap-muted-2); font-size: 12px; }
.component-detail-panel {
  display: grid;
  gap: var(--paap-space-4);
  position: sticky;
  top: var(--paap-space-4);
}
.component-detail-actions,
.component-row-actions {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.component-context-menu {
  position: fixed;
  z-index: 9500;
  display: grid;
  width: 220px;
  padding: 6px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.16);
}
.component-context-menu button {
  display: grid;
  gap: 2px;
  min-height: 44px;
  padding: 8px 10px;
  border: 0;
  border-radius: var(--paap-radius-xs);
  background: transparent;
  color: var(--paap-text);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
}
.component-context-menu button:hover { background: var(--paap-accent-soft); }
.component-context-menu span { font-size: 13px; font-weight: 650; }
.component-context-menu small { color: var(--paap-muted); font-size: 11px; }
.component-context-menu button { position: relative; }
.submenu-arrow {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--paap-muted);
}
.context-submenu {
  width: 240px;
  max-height: 400px;
  overflow-y: auto;
}
.component-context-menu button.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.component-context-menu button.disabled:hover {
  background: transparent;
}
.node-connector {
  position: absolute;
  width: 12px;
  height: 12px;
  border: 2px solid #3b82f6;
  border-radius: 50%;
  background: white;
  cursor: crosshair;
  opacity: 0;
  transition: opacity 0.2s;
}
.node-connector--top {
  top: -6px;
  left: 50%;
  transform: translateX(-50%);
}
.node-connector--right {
  right: -6px;
  top: 50%;
  transform: translateY(-50%);
}
.node-connector--bottom {
  bottom: -6px;
  left: 50%;
  transform: translateX(-50%);
}
.node-connector--left {
  left: -6px;
  top: 50%;
  transform: translateY(-50%);
}
.component-topology-node:hover .node-connector {
  opacity: 1;
}
.connection-drag-line {
  pointer-events: none;
}
.context-menu-divider {
  height: 1px;
  margin: 4px 0;
  background: var(--paap-border);
}
.context-menu-label {
  padding: 6px 10px 2px;
  color: var(--paap-muted);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.config-drawer-shell {
  position: fixed;
  inset: 0;
  z-index: 9400;
  display: flex;
  justify-content: flex-end;
  background: rgba(22, 22, 22, 0.22);
}
.config-drawer {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  width: min(560px, 100vw);
  height: 100%;
  border-left: 1px solid var(--paap-border);
  background: var(--paap-panel);
  box-shadow: -12px 0 30px rgba(15, 23, 42, 0.18);
}
.config-drawer-header,
.config-drawer-footer {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  padding: var(--paap-space-5);
  border-bottom: 1px solid var(--paap-border);
}
.config-drawer-footer {
  align-items: center;
  justify-content: flex-end;
  border-top: 1px solid var(--paap-border);
  border-bottom: 0;
}
.config-drawer-header h3 {
  margin: 2px 0;
  color: var(--paap-text);
  font-size: 18px;
  font-weight: 650;
}
.config-drawer-header-actions {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
}
.config-drawer-header small {
  color: var(--paap-muted);
  font-size: 12px;
}
.config-drawer-body {
  display: grid;
  align-content: start;
  gap: var(--paap-space-4);
  min-height: 0;
  overflow-y: auto;
  padding: var(--paap-space-5);
  background: var(--paap-bg);
}
.config-section {
  display: grid;
  gap: var(--paap-space-3);
  padding: var(--paap-space-4);
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.config-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
}
.config-section-title span,
.config-stack-field span,
.config-form-grid label span {
  color: var(--paap-muted-2);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}
.config-section-title a {
  color: var(--paap-accent);
  font-size: 12px;
  font-weight: 650;
  text-decoration: none;
}
.config-code {
  display: block;
  min-width: 0;
  padding: 8px 10px;
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  font-family: var(--paap-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.service-access-stack {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.service-access-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
}
.service-access-row > span {
  color: var(--paap-muted);
  font-size: 12px;
  font-weight: 650;
}
.service-access-row code,
.service-access-row strong {
  min-width: 0;
  color: var(--paap-text);
  font-family: var(--paap-mono);
  font-size: 12px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.service-access-row strong {
  color: var(--paap-muted);
  font-family: inherit;
}
.service-access-row .text-btn {
  white-space: nowrap;
}
.service-access-row--action .bx--btn {
  min-height: 32px;
  padding: 0 12px;
  white-space: nowrap;
}
.cds-image-fields {
  display: grid;
  grid-template-columns: 1fr 120px;
  gap: var(--cds-spacing-03, 8px);
}
.cds-image-field {
  display: flex;
  flex-direction: column;
  gap: var(--cds-spacing-02, 4px);
}
.cds-label {
  font-size: var(--cds-label-01-font-size, 12px);
  font-weight: var(--cds-label-01-font-weight, 400);
  line-height: var(--cds-label-01-line-height, 1.33333);
  letter-spacing: var(--cds-label-01-letter-spacing, 0.32px);
  color: var(--cds-text-secondary, #525252);
}
.cds-text-input {
  height: 32px;
  padding: 0 var(--cds-spacing-03, 8px);
  background: var(--cds-field-01, #f4f4f4);
  border: 1px solid var(--cds-border-strong-01, #8d8d8d);
  border-radius: 0;
  font-size: var(--cds-body-01-font-size, 14px);
  line-height: 1.4;
  color: var(--cds-text-primary, #161616);
  outline: none;
  transition: border-color 110ms;
}
.cds-text-input:focus {
  border-color: var(--cds-border-interactive, #0f62fe);
  box-shadow: inset 0 0 0 1px var(--cds-border-interactive, #0f62fe);
}
.cds-text-input::placeholder {
  color: var(--cds-text-placeholder, rgba(22,22,22,0.4));
}
.cds-image-preview {
  display: flex;
  align-items: center;
  gap: var(--cds-spacing-02, 4px);
  margin-top: var(--cds-spacing-02, 4px);
  padding: var(--cds-spacing-02, 4px) var(--cds-spacing-03, 8px);
  background: var(--cds-layer-01, #fff);
  border-left: 2px solid var(--cds-blue-60, #0f62fe);
}
.cds-image-preview__icon {
  flex-shrink: 0;
  color: var(--cds-blue-60, #0f62fe);
}
.cds-image-preview__text {
  font-family: var(--cds-font-family-mono, monospace);
  font-size: 12px;
  color: var(--cds-text-primary, #161616);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.config-kv-grid,
.config-ref-grid,
.config-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-3);
}
.config-kv-grid div,
.config-ref-grid div {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.config-kv-grid span,
.config-ref-grid span {
  color: var(--paap-muted-2);
  font-size: 11px;
}
.config-kv-grid strong,
.config-ref-grid strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: 12px;
  font-weight: 550;
  overflow-wrap: anywhere;
}
.service-summary-grid {
  margin-top: var(--paap-space-3);
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
}
.service-topology-list {
  display: grid;
  gap: var(--paap-space-2);
}
.service-topology-summary {
  display: flex;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
}
.service-topology-summary span {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: 11px;
  font-weight: 650;
}
.service-topology-node-row {
  display: grid;
  grid-template-columns: minmax(90px, 1fr) 70px 70px minmax(0, 1.2fr);
  gap: var(--paap-space-2);
  align-items: center;
  min-height: 38px;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}
.service-topology-node-row strong,
.service-topology-node-row span,
.service-topology-node-row code,
.service-topology-node-row small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}
.service-topology-node-row small {
  grid-column: 1 / -1;
  color: var(--paap-muted-2);
}
.config-form-grid label,
.config-stack-field {
  display: grid;
  gap: 6px;
}
.config-form-wide {
  grid-column: span 2;
}
.config-env-list,
.config-readonly-list {
  display: grid;
  gap: var(--paap-space-2);
}
.config-env-row {
  display: grid;
  grid-template-columns: minmax(100px, 1fr) 110px minmax(120px, 1.2fr) minmax(96px, 1fr) auto;
  gap: var(--paap-space-2);
  align-items: center;
}
.config-env-row .danger {
  color: var(--paap-danger);
}
.config-readonly-row {
  display: grid;
  grid-template-columns: minmax(120px, 0.8fr) minmax(0, 1.2fr);
  gap: var(--paap-space-3);
  padding: 8px 10px;
  background: var(--paap-panel-subtle);
}
.config-readonly-row strong,
.config-readonly-row span {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 12px;
}
.config-empty {
  color: var(--paap-muted-2);
  font-size: 12px;
}
.config-section details {
  display: grid;
  gap: var(--paap-space-3);
}
.config-section summary {
  cursor: pointer;
  color: var(--paap-text);
  font-weight: 650;
}
.config-section textarea {
  min-height: 56px;
  resize: vertical;
}
.relationship-modal { max-width: 560px; }
.relationship-help {
  margin: 0 0 var(--paap-space-4);
  color: var(--paap-muted);
  font-size: 13px;
  line-height: 1.5;
}
.relationship-target-list {
  display: grid;
  gap: var(--paap-space-2);
}
.relationship-target {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: var(--paap-space-3);
  min-height: 48px;
  padding: 9px 11px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}
.relationship-target input {
  margin: 0;
}
.relationship-target span {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.relationship-target strong,
.relationship-target small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.relationship-target strong { color: var(--paap-text); font-size: 13px; font-weight: 650; }
.relationship-target small { color: var(--paap-muted); font-size: 12px; }
.component-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-3);
}
.component-detail-head h3 {
  margin: 0;
  color: var(--paap-text);
  font-size: 16px;
  font-weight: 650;
  line-height: 1.3;
  overflow-wrap: anywhere;
}
.component-detail-head p {
  margin: 4px 0 0;
  color: var(--paap-muted);
  font-size: 12px;
}
.component-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
}
.component-detail-grid div {
  display: grid;
  gap: 3px;
  min-width: 0;
  padding: var(--paap-space-3);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}
.component-detail-grid span,
.component-detail-label {
  color: var(--paap-muted-2);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}
.component-detail-grid strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-flow-panel,
.component-relation-panel {
  display: grid;
  gap: var(--paap-space-2);
}
.component-flow-steps {
  display: flex;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
}
.component-flow-steps span {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 9px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: 11px;
  font-weight: 600;
}
.component-relation-panel button {
  min-height: 28px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  font-size: 12px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
}
.component-relation-panel button:hover { border-color: var(--paap-accent); color: var(--paap-accent); }
.component-relation-panel small { color: var(--paap-muted-2); font-size: 12px; }
.component-count-label { color: var(--paap-muted); font-size: 12px; }
.component-table-wrap {
  overflow: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
}
.component-table {
  width: 100%;
  min-width: 920px;
  border-collapse: collapse;
}
.component-table th,
.component-table td {
  padding: 11px 12px;
  border-bottom: 1px solid var(--paap-border);
  color: var(--paap-muted);
  font-size: 12px;
  text-align: left;
  vertical-align: middle;
}
.component-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted-2);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}
.component-table tr {
  cursor: pointer;
}
.component-table tbody tr:hover,
.component-table tbody tr.active {
  background: var(--paap-accent-soft);
}
.component-table tr:last-child td { border-bottom: 0; }
.component-table td strong {
  display: block;
  max-width: 220px;
  overflow: hidden;
  color: var(--paap-text);
  font-size: 13px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-table td small {
  display: block;
  max-width: 220px;
  overflow: hidden;
  color: var(--paap-muted-2);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-table td:nth-child(3) {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
}
.component-target-cell {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-table-empty {
  padding: var(--paap-space-6);
  color: var(--paap-muted);
  text-align: center;
}
.status-dot { width: 6px; height: 6px; background: var(--paap-border-strong); border-radius: 50%; }
.status-dot.running { background: var(--paap-success); }
.status-dot.stopped { background: var(--paap-danger); }
.status-dot.failed,
.status-dot.error { background: var(--paap-danger); }
.status-dot.creating,
.status-dot.deploying,
.status-dot.building { background: var(--paap-accent); }
.status-dot.draft { background: var(--paap-border-strong); }

/* Empty state */
.empty-state {
  padding: var(--paap-space-12) var(--paap-space-8); text-align: center; background: var(--paap-panel);
  display: flex; flex-direction: column; align-items: center;
  min-height: 200px; justify-content: center;
  border-radius: var(--paap-radius);
}
.empty-illustration { margin-bottom: var(--paap-space-4); }
.empty-title { margin-bottom: var(--paap-space-2); font-weight: 600; font-size: 18px; color: var(--paap-text); }
.empty-text { color: var(--paap-muted); line-height: 1.5; max-width: 420px; font-size: 14px; }

/* Modal */
.modal-overlay { position: fixed; inset: 0; z-index: 9000; display: flex; align-items: center; justify-content: center; background: rgba(17,19,24,0.46); backdrop-filter: blur(10px); padding: var(--paap-space-6); }
.modal-container { background: var(--paap-panel); width: 100%; max-height: 90vh; overflow-y: auto; border-radius: var(--paap-radius); border: 1px solid var(--paap-border); box-shadow: none; position: relative; }
.confirm-modal { max-width: 460px; }
.capability-action-modal { max-width: 520px; }
.modal-header { display: flex; justify-content: space-between; align-items: flex-start; padding: var(--paap-space-5) var(--paap-space-6); border-bottom: 1px solid var(--paap-border); }
.modal-label { font-size: 11px; color: var(--paap-muted); letter-spacing: 0.04em; margin-bottom: 4px; text-transform: uppercase; font-weight: 600; }
.modal-heading { font-size: 18px; font-weight: 600; color: var(--paap-text); line-height: 1.3; margin: 0; }
.modal-close { background: none; border: 1px solid var(--paap-border); color: var(--paap-muted); cursor: pointer; padding: 4px; line-height: 1; transition: all 0.15s; margin-top: -4px; border-radius: var(--paap-radius-sm); width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; }
.modal-close:hover { background: var(--paap-panel-subtle); color: var(--paap-text); }
.modal-content { padding: var(--paap-space-6); }
.capability-action-content { display: grid; gap: var(--paap-space-4); }
.capability-action-field { margin: 0; }
.capability-action-textarea { min-height: 140px; resize: vertical; }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--paap-space-2); padding: var(--paap-space-4) var(--paap-space-6); border-top: 1px solid var(--paap-border); }
.modal-error {
  border: 1px solid #fecaca;
  background: var(--paap-danger-soft);
  color: #991b1b;
  border-radius: var(--paap-radius);
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
  margin: 0;
}
.confirm-text {
  margin: 0;
  color: var(--paap-muted);
  font-size: 14px;
  line-height: 1.5;
}

/* Form */
.bx--form-item { margin-bottom: var(--paap-space-5); }
.bx--label { display: block; font-size: 12px; color: var(--paap-muted); margin-bottom: 6px; font-weight: 500; }
.bx--text-input {
  width: 100%; padding: 9px 12px; font-size: 14px;
  border: 1px solid var(--paap-border); border-radius: var(--paap-radius-sm);
  background: var(--paap-panel); color: var(--paap-text); outline: none;
  font-family: inherit; transition: border-color 0.15s, box-shadow 0.15s;
}
.bx--text-input:focus { border-color: var(--paap-accent); box-shadow: 0 0 0 3px rgba(37,99,235,0.1); }
.bx--text-input::placeholder { color: var(--paap-muted-2); }
.form-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-3);
}
.delivery-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
}
.delivery-option {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 38px;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  color: var(--paap-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  background: var(--paap-panel);
  transition: all 0.15s;
}
.delivery-option.active {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
  color: var(--paap-text);
}
.delivery-option input { margin: 0; }

.bx--select { position: relative; }
.bx--select-input {
  width: 100%; padding: 9px 36px 9px 12px; font-size: 14px;
  border: 1px solid var(--paap-border); border-radius: var(--paap-radius-sm);
  background: var(--paap-panel); color: var(--paap-text); outline: none;
  appearance: none; cursor: pointer; font-family: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.bx--select-input:focus { border-color: var(--paap-accent); box-shadow: 0 0 0 3px rgba(37,99,235,0.1); }
.bx--select__arrow { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); pointer-events: none; }

/* Service select grid in modal */
.no-data { color: var(--paap-muted); text-align: center; padding: var(--paap-space-8); }
.error-text { color: var(--paap-danger); }
.service-picker-summary { display: flex; align-items: center; gap: var(--paap-space-3); margin-bottom: var(--paap-space-4); padding: var(--paap-space-3) var(--paap-space-4); background: var(--paap-panel-subtle); border-left: 3px solid var(--paap-accent); border-radius: 0 var(--paap-radius-xs) var(--paap-radius-xs) 0; }
.summary-pill { display: inline-flex; align-items: center; justify-content: center; padding: 2px 8px; font-size: 11px; font-weight: 600; background: var(--paap-accent-soft); color: var(--paap-accent); border-radius: var(--paap-radius-xs); }
.summary-text { color: var(--paap-muted); font-size: 13px; line-height: 1.4; }
.service-select-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--paap-space-3); }
.service-select-card {
  display: flex; align-items: flex-start; gap: var(--paap-space-3);
  padding: var(--paap-space-4); border: 1px solid var(--paap-border);
  cursor: pointer; background: var(--paap-panel); transition: all 0.15s;
  border-radius: var(--paap-radius-sm);
}
.service-select-card:hover { border-color: var(--paap-accent); }
.service-select-card.selected { border-color: var(--paap-accent); background: var(--paap-accent-soft); }
.service-select-card.disabled { cursor: not-allowed; opacity: 0.5; }
.service-select-card.disabled:hover { border-color: var(--paap-border); background: var(--paap-panel); }

.select-radio { width: 18px; height: 18px; border: 1.5px solid var(--paap-border-strong); display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-top: 2px; background: var(--paap-panel); transition: all 0.15s; border-radius: 50%; }
.select-radio.selected { background: var(--paap-accent); border-color: var(--paap-accent); color: #ffffff; }
.select-radio.disabled { background: #f3f4f6; border-color: var(--paap-border-strong); color: var(--paap-muted-2); }

.service-name-row { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; }
.service-name { font-size: 15px; line-height: 1.4; font-weight: 600; }
.service-desc { color: var(--paap-muted); margin-top: 4px; line-height: 1.4; font-size: 13px; }

/* Buttons */
.bx--btn { display: inline-flex; align-items: center; justify-content: center; font-size: 13px; font-weight: 500; cursor: pointer; outline: none; border: none; height: 36px; padding: 0 14px; transition: all 0.15s; border-radius: var(--paap-radius-sm); gap: 6px; font-family: inherit; }
.bx--btn--primary { background: var(--cds-button-primary, var(--paap-accent)); color: var(--cds-text-on-color, #ffffff); }
.bx--btn--primary:hover { background: var(--cds-button-primary-hover, var(--paap-accent-hover)); }
.bx--btn--secondary { background: var(--paap-panel); color: var(--paap-accent); border: 1px solid var(--paap-accent); }
.bx--btn--secondary:hover { background: var(--paap-accent-soft); color: var(--paap-accent-hover); border-color: var(--paap-accent-hover); }
.bx--btn--danger { background: var(--cds-button-danger-primary, var(--paap-danger)); color: var(--cds-text-on-color, #ffffff); }
.bx--btn--danger:hover { background: var(--cds-button-danger-hover, #ba1b23); }
.bx--btn--ghost { background: transparent; color: var(--paap-muted); border: 1px solid var(--paap-border); }
.bx--btn--ghost:hover { background: var(--paap-panel-subtle); color: var(--paap-text); }
.bx--btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* Service icon wrap */
.service-icon-wrap { width: 32px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; align-self: center; background: transparent; color: var(--paap-muted-2); }
.service-icon-wrap.deploy   { color: #8b9dc3; }
.service-icon-wrap.git      { color: #cc8888; }
.service-icon-wrap.log      { color: #a090c8; }
.service-icon-wrap.monitor  { color: #7bb896; }
.service-icon-wrap.registry { color: #d4b072; }
.service-icon-wrap.harbor   { color: #8b9dc3; }
.service-icon-wrap.ci       { color: #c88ba8; }
.service-icon-wrap.mysql      { color: #7bb8ae; }
.service-icon-wrap.postgresql { color: #8b9dc3; }
.service-icon-wrap.mongodb    { color: #7bb896; }
.service-icon-wrap.redis      { color: #c88ba8; }
.service-icon-wrap.rabbitmq   { color: #d4b072; }
.service-icon-wrap.kafka      { color: #a090c8; }
.service-icon-wrap.minio      { color: #8b9dc3; }
.service-icon-wrap.infra      { color: var(--paap-muted); }

/* Tag status dot */
.tag-dot { width: 6px; height: 6px; border-radius: 50%; display: inline-block; margin-right: 5px; vertical-align: middle; margin-top: -1px; background: var(--paap-border-strong); }
.tag-dot.running   { background: #059669; }
.tag-dot.installing{ background: var(--paap-accent); }
.tag-dot.error     { background: var(--paap-danger); }
.tag-dot.failed    { background: var(--paap-danger); }
.tag-dot.deleting  { background: var(--paap-muted-2); }
.tag-dot.pending,
.tag-dot.draft     { background: var(--paap-border-strong); }

/* Tags */
.bx--tag { font-size: 11px; padding: 2px 8px; border-radius: var(--paap-radius-full); font-weight: 500; }
.bx--tag--sm { font-size: 10px; padding: 1px 6px; }
.bx--tag--blue { background: var(--paap-accent-soft); color: var(--paap-accent); }
.bx--tag--green { background: var(--paap-success-soft); color: #059669; }
.bx--tag--gray { background: var(--cds-tag-background-gray, var(--cds-gray-20, #e0e0e0)); color: var(--cds-tag-color-gray, var(--paap-text)); }
.bx--tag--red { background: var(--paap-danger-soft); color: var(--paap-danger); }

/* Responsive */
@media (max-width: 980px) {
  .environment-shell,
  .overview-grid,
  .overview-two-col,
  .component-topology-layout {
    grid-template-columns: 1fr;
  }
  .overview-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .overview-subtitle {
    text-align: left;
  }
}

@media (max-width: 768px) {
  .rail-page {
    padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10);
  }
  .page-title-bar {
    grid-template-columns: 1fr;
    padding: var(--paap-space-5);
  }
  .page-title { font-size: 24px; }
  .overview-stats { grid-template-columns: 1fr; }
  .component-filter-bar { align-items: stretch; flex-direction: column; }
  .component-filter-actions { justify-content: flex-start; }
  .component-search-input { width: 100%; }
  .component-detail-panel { position: static; }
  .service-select-grid { grid-template-columns: 1fr; }
  .form-row { grid-template-columns: 1fr; }
}
</style>
