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
              <div class="component-topology-zone-legend" aria-label="画布分区">
                <span>本环境</span>
                <span>平台公共</span>
                <span>集群外部</span>
              </div>
              <div class="component-canvas-stage" :style="{ width: `${environmentCanvasSize.width}px`, height: `${environmentCanvasSize.height}px`, transform: `scale(${canvasZoom})`, transformOrigin: 'top left' }" @pointerdown="startCanvasMarquee">
                <div
                  v-for="zone in environmentCanvasZones"
                  :key="zone.key"
                  class="component-topology-zone"
                  :class="[`component-topology-zone--${zone.key}`, { 'component-topology-zone--collapsed': zone.collapsed }]"
                  :style="componentTopologyZoneStyle(zone)"
                  @pointerdown="startTopologyZoneDrag($event, zone)"
                >
                  <button
                    type="button"
                    class="component-topology-zone-toggle"
                    :aria-expanded="!isTopologyZoneCollapsed(zone.key)"
                    :title="topologyZoneToggleTitle(zone)"
                    @click.stop="toggleTopologyZone(zone.key)"
                    @pointerdown.stop
                  >
                    <span>{{ zone.label }}</span>
                    <small>{{ zone.nodes.length }} 个资源</small>
                    <svg focusable="false" width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                      <path :d="isTopologyZoneCollapsed(zone.key) ? 'M6 4l4 4-4 4V4z' : 'M4 6l4 4 4-4H4z'" />
                    </svg>
                  </button>
                  <button
                    v-for="handle in topologyZoneResizeHandles"
                    :key="handle.key"
                    v-show="!zone.collapsed"
                    type="button"
                    class="component-topology-zone-resize-handle"
                    :class="`component-topology-zone-resize-handle--${handle.key}`"
                    :aria-label="topologyZoneResizeHandleTitle(zone, handle)"
                    :title="topologyZoneResizeHandleTitle(zone, handle)"
                    @click.stop.prevent
                    @pointerdown.stop.prevent="startTopologyZoneResize($event, zone, handle.edges)"
                  ></button>
                </div>
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
                    :class="componentEdgeClasses(edge)"
                    :d="componentEdgePath(edge)"
                  />
                  <path
                    v-for="edge in environmentManualCanvasEdges"
                    :key="`hit-${edge.fromKey}-${edge.toKey}`"
                    class="component-canvas-link-hit"
                    :d="componentEdgePath(edge)"
                    @pointerdown.stop
                    @click.stop="selectManualCanvasEdge(edge)"
                    @contextmenu.stop.prevent="openManualEdgeContextMenu($event, edge)"
                  />
                  <path
                    v-if="connectionDrag"
                    class="connection-drag-line"
                    :d="`M ${connectionDrag.startX} ${connectionDrag.startY} L ${connectionDrag.currentX} ${connectionDrag.currentY}`"
                    stroke="currentColor"
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
                <div v-if="environmentCanvasAllNodes.length === 0" class="component-canvas-empty-hint">
                  <strong>空环境</strong>
                  <span>{{ isSystemSharedEnvironment ? '在画布空白处右键添加工具或中间件。' : '在画布空白处右键创建组件、工具、数据库或中间件。' }}</span>
                  <div v-if="missingRequiredEnvironmentCapabilities.length" class="component-canvas-empty-actions">
                    <button
                      v-for="item in missingRequiredEnvironmentCapabilities"
                      :key="item.key"
                      type="button"
                      @click.stop="openFoundationCapability(item)"
                    >
                      安装{{ item.label }}
                    </button>
                  </div>
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
                  @dblclick.stop.prevent="startRenameNode(node)"
                  @pointerdown="startComponentNodeDrag($event, node)"
                  @pointerup="finishComponentNodePointer($event, node)"
                  @contextmenu.stop.prevent="openTopologyContextMenu($event, node)"
                >
                  <span class="node-type-icon" :class="componentNodeIconClass(node)">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="componentNodeIconPath(node)" /></svg>
                  </span>
                  <span class="node-status" :class="node.status"></span>
                  <input
                    v-if="renamingNodeKey === (node.topologyId || String(node.id))"
                    class="node-rename-input"
                    type="text"
                    :value="renamingNodeValue"
                    @input="renamingNodeValue = ($event.target as HTMLInputElement).value"
                    @keydown.enter.stop.prevent="submitRenameNode"
                    @keydown.escape.stop.prevent="cancelRenameNode"
                    @blur="submitRenameNode"
                    @pointerdown.stop
                    @click.stop
                  />
                  <strong v-else>{{ node.name }}</strong>
                  <small>{{ environmentTopologyNodeSubtitle(node) }}</small>
                  <span
                    v-if="topologySourceBadge(node)"
                    class="node-source-badge"
                    :class="`node-source-badge--${topologySourceBadge(node)?.tone}`"
                  >
                    {{ topologySourceBadge(node)?.label }}
                  </span>
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
                <span class="overview-subtitle">Source → Gitea → Registry → ArgoCD → Monitor / Logs</span>
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
                <p>还没有安装环境必需的代码仓库、镜像仓库、部署、监控或日志工具。</p>
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
            <rect x="4" y="16" width="56" height="36" rx="0" stroke="var(--paap-text-04)" stroke-width="2" fill="none"/>
            <line x1="4" y1="26" x2="60" y2="26" stroke="var(--paap-gray-20)" stroke-width="2"/>
            <circle cx="14" cy="21" r="2" fill="var(--paap-text-04)"/>
            <circle cx="22" cy="21" r="2" fill="var(--paap-text-04)"/>
            <rect x="10" y="32" width="18" height="14" fill="var(--paap-gray-20)" opacity="0.6"/>
            <rect x="34" y="32" width="20" height="6" fill="var(--paap-gray-20)" opacity="0.4"/>
            <rect x="34" y="42" width="12" height="4" fill="var(--paap-gray-20)" opacity="0.4"/>
          </svg>
          <h3 class="empty-title">暂无{{ activeCapabilityTab?.label }}</h3>
          <p class="empty-text">{{ activeCapabilityEmptyText }}</p>
          <div class="capability-install-panel">
            <div class="capability-install-head">
              <span>{{ activeCapabilityInstallLabel }}</span>
              <small>{{ activeCapabilityInstallHint }}</small>
            </div>
            <div v-if="capabilityInlineInstallLoading" class="workspace-loading">服务加载中...</div>
            <div v-else-if="capabilityInlineInstallError" class="page-error" role="alert">{{ capabilityInlineInstallError }}</div>
            <div v-else-if="activeCapabilityInstallTemplates.length" class="capability-install-grid">
              <button
                v-for="tmpl in activeCapabilityInstallTemplates"
                :key="tmpl.type"
                type="button"
                class="capability-install-card"
                :class="{ disabled: tmpl.disabled }"
                :disabled="tmpl.disabled || capabilityInlineInstallingType === tmpl.type"
                @click="installCapabilityTemplate(tmpl)"
              >
                <span class="service-icon-wrap" :class="tmpl.type">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path :d="serviceIconPath(tmpl.type)" /></svg>
                </span>
                <span>
                  <strong>{{ tmpl.name || tmpl.type }}</strong>
                  <small>{{ capabilityInlineInstallingType === tmpl.type ? '安装中...' : (tmpl.disabled ? (tmpl.statusText || '已添加') : (tmpl.description || tmpl.type)) }}</small>
                </span>
              </button>
            </div>
            <div v-else class="config-empty">当前没有可添加的服务。</div>
          </div>
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
          <div v-if="activeCapabilityWorkspaceActions.length && !activeCapabilityWorkspaceUsesInternalActions" class="workspace-action-strip" aria-label="工作台操作">
            <button
              v-for="action in activeCapabilityWorkspaceActions"
              :key="action.key"
              type="button"
              :class="workspaceActionButtonClass(action)"
              :title="action.description"
              :disabled="capabilityWorkspaceLoading"
              @click="beginCapabilityWorkspaceAction(action, action.target)"
            >
              {{ action.label }}
            </button>
          </div>
          <div v-if="activeCapabilityAction && !activeWorkspaceActionBelongsToDrawer" class="workspace-action-inline" role="region" aria-label="工作台操作参数">
            <header>
              <div>
                <span>执行操作</span>
                <strong>{{ activeCapabilityAction.label }}</strong>
              </div>
              <button type="button" class="text-btn" :disabled="capabilityWorkspaceLoading" @click="closeWorkspaceActionInline">取消</button>
            </header>
            <p v-if="activeCapabilityAction.description">{{ activeCapabilityAction.description }}</p>
            <div class="workspace-action-inline-form">
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
                  @change="setWorkspaceActionCheckboxParam(field.name, $event)"
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
            </div>
            <div class="workspace-action-inline-actions">
              <button type="button" class="bx--btn bx--btn--secondary" :disabled="capabilityWorkspaceLoading" @click="closeWorkspaceActionInline">取消</button>
              <button type="button" class="bx--btn bx--btn--primary" :disabled="capabilityWorkspaceLoading" @click="submitWorkspaceActionInline">
                {{ capabilityWorkspaceLoading ? '执行中...' : '执行' }}
              </button>
            </div>
          </div>
          <div v-if="capabilityWorkspaceLoading" class="workspace-loading">工作台加载中...</div>

          <component
            v-if="activeCapabilityService && activeCapabilityWorkspaceReady && workspaceComponentForService(activeCapabilityService)"
            :key="capabilityWorkspaceKey"
            :is="workspaceComponentForService(activeCapabilityService)"
            :resources="activeCapabilityWorkspace.resources"
            :workspace-actions="activeCapabilityWorkspaceActions"
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
                    :class="componentEdgeClasses(edge)"
                    :d="componentEdgePath(edge)"
                  />
                  <path
                    v-for="edge in componentManualCanvasEdges"
                    :key="`hit-${edge.fromKey}-${edge.toKey}`"
                    class="component-canvas-link-hit"
                    :d="componentEdgePath(edge)"
                    @pointerdown.stop
                    @click.stop="selectManualCanvasEdge(edge)"
                    @contextmenu.stop.prevent="openManualEdgeContextMenu($event, edge)"
                  />
                  <path
                    v-if="connectionDrag"
                    class="connection-drag-line"
                    :d="`M ${connectionDrag.startX} ${connectionDrag.startY} L ${connectionDrag.currentX} ${connectionDrag.currentY}`"
                    stroke="currentColor"
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
                  <span>{{ isSystemSharedEnvironment ? '在画布空白处右键添加工具或中间件。' : '在画布空白处右键创建前端、后端、数据库或中间件。' }}</span>
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
                  @dblclick.stop.prevent="startRenameNode(node)"
                  @pointerdown="startComponentNodeDrag($event, node)"
                  @pointerup="finishComponentNodePointer($event, node)"
                  @contextmenu.stop.prevent="openTopologyContextMenu($event, node)"
                >
                  <span class="node-type-icon" :class="componentNodeIconClass(node)">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="componentNodeIconPath(node)" /></svg>
                  </span>
                  <span class="node-status" :class="node.status"></span>
                  <input
                    v-if="renamingNodeKey === (node.topologyId || String(node.id))"
                    class="node-rename-input"
                    type="text"
                    :value="renamingNodeValue"
                    @input="renamingNodeValue = ($event.target as HTMLInputElement).value"
                    @keydown.enter.stop.prevent="submitRenameNode"
                    @keydown.escape.stop.prevent="cancelRenameNode"
                    @blur="submitRenameNode"
                    @pointerdown.stop
                    @click.stop
                  />
                  <strong v-else>{{ node.name }}</strong>
                  <small>{{ topologyNodeSubtitle(node) }}</small>
                  <span
                    v-if="topologySourceBadge(node)"
                    class="node-source-badge"
                    :class="`node-source-badge--${topologySourceBadge(node)?.tone}`"
                  >
                    {{ topologySourceBadge(node)?.label }}
                  </span>
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
                        <button type="button" class="text-btn" @click.stop="openComponentConfigDrawer(comp, 'variables')">配置</button>
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
      <div v-if="showComponentModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeComponentDraftModal">
        <div class="modal-container" style="max-width:600px">
          <div class="modal-header">
            <div>
              <p class="modal-label">组件草稿</p>
              <p class="modal-heading">新建组件草稿</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="closeComponentDraftModal">
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
                <div class="bx--form-item">
                  <label class="bx--label">构建模块</label>
                  <input v-model="compForm.buildModule" class="bx--text-input" placeholder="多模块项目填写模块目录" />
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
            <button type="button" class="bx--btn bx--btn--secondary" @click="closeComponentDraftModal">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" @click="submitComponent">保存草稿</button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="configDrawer.visible" class="config-drawer-shell" role="dialog" aria-modal="true" @click.self="closeConfigDrawer">
        <aside class="config-drawer" @pointerdown.stop="enterDrawerContext">
          <header class="config-drawer-header">
            <div class="config-drawer-title-block">
              <span class="config-drawer-avatar" :class="`config-drawer-avatar--${configDrawer.kind}`">
                {{ configDrawerAvatarLabel }}
              </span>
              <div>
              <p class="modal-label">{{ configDrawer.kind === 'capability' ? '能力配置' : (configDrawer.kind === 'service' ? '服务配置' : '组件配置') }}</p>
              <h3>{{ configDrawerTitle }}</h3>
              <small>{{ configDrawerSubtitle }}</small>
              </div>
            </div>
            <div class="config-drawer-header-actions">
              <button type="button" class="modal-close" aria-label="关闭" @click="closeConfigDrawer">
                <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
              </button>
            </div>
          </header>

          <nav class="config-drawer-tabs" aria-label="资源配置视图">
            <button
              v-for="tab in configDrawerTabs"
              :key="tab.key"
              type="button"
              :class="{ active: configDrawerTab === tab.key }"
              @click="configDrawerTab = tab.key"
            >
              {{ tab.label }}
            </button>
          </nav>

          <div class="config-drawer-body">
            <div v-if="activeCapabilityAction && activeWorkspaceActionBelongsToDrawer && !serviceDrawerWorkspaceActive" class="workspace-action-inline workspace-action-inline--drawer" role="region" aria-label="当前卡片操作参数">
              <header>
                <div>
                  <span>执行操作</span>
                  <strong>{{ activeCapabilityAction.label }}</strong>
                </div>
                <button type="button" class="text-btn" :disabled="serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading" @click="closeWorkspaceActionInline">取消</button>
              </header>
              <p v-if="activeCapabilityAction.description">{{ activeCapabilityAction.description }}</p>
              <div class="workspace-action-inline-form">
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
                    @change="setWorkspaceActionCheckboxParam(field.name, $event)"
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
              </div>
              <div class="workspace-action-inline-actions">
                <button type="button" class="bx--btn bx--btn--secondary" :disabled="serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading" @click="closeWorkspaceActionInline">取消</button>
                <button type="button" class="bx--btn bx--btn--primary" :disabled="serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading" @click="submitWorkspaceActionInline">
                  {{ serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading ? '执行中...' : '执行' }}
                </button>
              </div>
            </div>
            <section v-if="configDrawerTab === 'deploy'" class="config-section config-section--hero">
              <div class="config-section-title">
                <span>部署状态</span>
              </div>
              <div class="config-deployment-card">
                <div class="config-deployment-main">
                  <span class="status-dot" :class="drawerStatusValue"></span>
                  <div>
                    <strong>{{ drawerDeploymentTitle }}</strong>
                    <small>{{ drawerDeploymentSubtitle }}</small>
                  </div>
                  <span class="rail-tag" :class="statusTagClass(drawerStatusValue)">{{ drawerStatusLabel }}</span>
                </div>
                <div v-if="drawerDeploymentRows.length" class="config-deployment-meta">
                  <div v-for="row in drawerDeploymentRows" :key="row.label">
                    <span>{{ row.label }}</span>
                    <strong>{{ row.value }}</strong>
                  </div>
                </div>
                <div v-if="configDrawer.kind !== 'capability'" class="config-deployment-actions">
                  <button type="button" class="rail-btn rail-btn--secondary rail-btn--sm" @click="openDrawerLogs">查看日志</button>
                  <button type="button" class="rail-btn rail-btn--secondary rail-btn--sm" @click="openDrawerMonitoring">查看监控</button>
                </div>
              </div>
              <div v-if="resourceSourceSummaryRows.length" class="source-semantics-card">
                <div v-for="row in resourceSourceSummaryRows" :key="row.label" class="source-semantics-row">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                  <small v-if="row.hint">{{ row.hint }}</small>
                </div>
              </div>
              <div v-if="configDrawer.kind === 'capability'" class="service-access-stack capability-access-stack">
                <div class="service-access-row">
                  <span>来源</span>
                  <strong>{{ capabilitySourceLabel(drawerCapability?.source || '') }}</strong>
                </div>
                <div class="service-access-row">
                  <span>能力</span>
                  <code>{{ capabilityLabel(drawerCapability?.capability || '') }}</code>
                </div>
                <div v-if="drawerCapability?.source === 'shared'" class="service-access-row">
                  <span>共享服务</span>
                  <code>{{ drawerCapability?.refService?.serviceName || drawerCapability?.refService?.serviceType || drawerCapability?.serviceType || '未关联' }}</code>
                </div>
                <div v-if="drawerCapability?.source === 'external'" class="service-access-row">
                  <span>外部地址</span>
                  <code>{{ drawerCapability?.externalEndpoint || '未配置' }}</code>
                </div>
              </div>
              <div v-else-if="configDrawer.kind === 'service'" class="service-access-stack">
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
              <div v-else-if="configDrawer.kind === 'component'" class="service-access-stack">
                <div class="service-access-row">
                  <span>内部地址</span>
                  <code>{{ componentDrawerServiceEndpoint || '等待生成' }}</code>
                </div>
                <div class="service-access-row service-access-row--action">
                  <span>Ingress 入口</span>
                  <code v-if="componentDrawerIngressUrl">{{ componentDrawerIngressUrl }}</code>
                  <strong v-else>未启用</strong>
                  <a v-if="componentDrawerIngressUrl" :href="componentDrawerIngressUrl" target="_blank" rel="noreferrer" class="text-btn">打开</a>
                  <button
                    v-if="componentDrawerExternalAccessToggleVisible"
                    type="button"
                    class="bx--btn bx--btn--secondary bx--btn--sm"
                    :disabled="componentExternalAccessLoading"
                    @click="toggleComponentExternalAccess"
                  >
                    {{ componentDrawerExternalAccessLabel }}
                  </button>
                </div>
                <div class="service-access-row service-access-row--action">
                  <span>NodePort 端口</span>
                  <code v-if="componentDrawerNodePortUrl">{{ componentDrawerNodePortUrl }}</code>
                  <strong v-else>未启用</strong>
                  <a v-if="componentDrawerNodePortUrl" :href="componentDrawerNodePortUrl" target="_blank" rel="noreferrer" class="text-btn">打开</a>
                  <button
                    v-if="componentDrawerExternalAccessToggleVisible"
                    type="button"
                    class="bx--btn bx--btn--secondary bx--btn--sm"
                    :disabled="componentNodePortLoading"
                    @click="toggleComponentNodePortAccess"
                  >
                    {{ componentDrawerNodePortEnabled ? '关闭 NodePort' : '开启 NodePort' }}
                  </button>
                </div>
              </div>
            </section>

            <section v-if="configDrawerTab === 'deploy' && configDrawer.kind === 'capability'" class="config-section capability-config-section">
              <div class="config-section-title">
                <span>能力配置</span>
                <small v-if="drawerCapability?.source === 'shared'">共享资源只读</small>
              </div>
              <div class="config-kv-grid service-summary-grid capability-summary-grid">
                <div v-for="row in drawerCapabilityRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
              <div v-if="drawerCapability?.source === 'shared'" class="shared-capability-readonly">
                <div class="config-section-title">
                  <span>共享资源连接信息</span>
                  <small>当前业务环境只读引用，连接信息由共享资源池提供。</small>
                </div>
                <div v-if="sharedCapabilityConnectionRows.length" class="config-kv-grid service-summary-grid capability-summary-grid">
                  <div v-for="row in sharedCapabilityConnectionRows" :key="row.label" :class="{ 'capability-secret-row': row.secretKey }">
                    <span>{{ row.label }}</span>
                    <strong>{{ row.value }}</strong>
                    <button
                      v-if="row.secretKey"
                      type="button"
                      class="password-visible-toggle capability-row-secret-toggle"
                      :aria-label="sharedCapabilitySecretVisible(row.secretKey) ? '隐藏密码' : '显示密码'"
                      :title="sharedCapabilitySecretVisible(row.secretKey) ? '隐藏密码' : '显示密码'"
                      :disabled="sharedCapabilityCredentialLoading"
                      @click="toggleSharedCapabilitySecret(row.secretKey)"
                    >
                      <svg v-if="sharedCapabilitySecretVisible(row.secretKey)" focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                        <path d="M16 6C7 6 2 16 2 16s5 10 14 10 14-10 14-10S25 6 16 6zm0 18c-6.4 0-10.5-5.8-11.7-8C5.5 13.8 9.6 8 16 8s10.5 5.8 11.7 8c-1.2 2.2-5.3 8-11.7 8z"/>
                        <path d="M16 10a6 6 0 1 0 0 12 6 6 0 0 0 0-12zm0 10a4 4 0 1 1 0-8 4 4 0 0 1 0 8z"/>
                      </svg>
                      <svg v-else focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                        <path d="m3.3 2 26.7 26.7-1.4 1.4-5.1-5.1A15 15 0 0 1 16 26C7 26 2 16 2 16a25 25 0 0 1 6.2-7.5L1.9 3.4 3.3 2zm6.4 8A22.7 22.7 0 0 0 4.3 16C5.5 18.2 9.6 24 16 24c2.1 0 4-.6 5.7-1.5l-3-3A6 6 0 0 1 12.5 13l-2.8-3z"/>
                        <path d="M16 6c9 0 14 10 14 10a24.9 24.9 0 0 1-4.6 6.1L24 20.7c1.7-1.6 3-3.5 3.7-4.7C26.5 13.8 22.4 8 16 8c-1.5 0-2.8.3-4.1.8L10.4 7.3A14 14 0 0 1 16 6z"/>
                      </svg>
                    </button>
                  </div>
                </div>
                <div v-else class="config-empty">未读取到共享资源连接信息。</div>
                <div v-if="sharedCapabilityCredentialError" class="config-inline-note error-text">{{ sharedCapabilityCredentialError }}</div>
              </div>
              <div v-if="drawerCapability?.source === 'shared'" class="config-inline-note">
                只读引用不会改变共享资源池中的服务本体；部署、编辑和删除由平台管理员在共享环境完成。
              </div>
              <div v-if="drawerCapability?.source === 'external'" class="external-capability-form">
                <div class="config-section-title">
                  <span>外部资源配置</span>
                  <small>连接信息保存到 Kubernetes Secret，数据库只保存 credentialSecretRef。</small>
                </div>
                <div class="config-form-grid">
                  <label class="config-form-wide">
                    <span>外部地址</span>
                    <input v-model.trim="capabilityForm.externalEndpoint" class="bx--text-input" :placeholder="externalCapabilityPlaceholder(drawerCapability)" />
                  </label>
                  <label>
                    <span>认证方式</span>
                    <select v-model="capabilityForm.authType" class="bx--select-input">
                      <option value="none">不需要认证</option>
                      <option value="basic">用户名密码</option>
                      <option value="token">Token</option>
                      <option value="existingSecret">已有 Secret</option>
                    </select>
                  </label>
                  <label v-if="capabilityForm.authType === 'basic'">
                    <span>用户名</span>
                    <input v-model.trim="capabilityForm.username" class="bx--text-input" autocomplete="username" />
                  </label>
                  <label v-if="capabilityForm.authType === 'basic'">
                    <span>密码</span>
                    <span class="password-field-wrap capability-secret-field">
                      <input
                        v-model="capabilityForm.password"
                        class="bx--text-input password-field-input"
                        :type="capabilitySecretVisible('password') ? 'text' : 'password'"
                        autocomplete="new-password"
                        placeholder="留空则沿用已保存 Secret"
                      />
                      <button
                        type="button"
                        class="password-visible-toggle"
                        :aria-label="capabilitySecretVisible('password') ? '隐藏密码' : '显示密码'"
                        :title="capabilitySecretVisible('password') ? '隐藏密码' : '显示密码'"
                        :disabled="capabilityCredentialLoading"
                        @click="toggleCapabilitySecret('password')"
                      >
                        <svg v-if="capabilitySecretVisible('password')" focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                          <path d="M16 6C7 6 2 16 2 16s5 10 14 10 14-10 14-10S25 6 16 6zm0 18c-6.4 0-10.5-5.8-11.7-8C5.5 13.8 9.6 8 16 8s10.5 5.8 11.7 8c-1.2 2.2-5.3 8-11.7 8z"/>
                          <path d="M16 10a6 6 0 1 0 0 12 6 6 0 0 0 0-12zm0 10a4 4 0 1 1 0-8 4 4 0 0 1 0 8z"/>
                        </svg>
                        <svg v-else focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                          <path d="m3.3 2 26.7 26.7-1.4 1.4-5.1-5.1A15 15 0 0 1 16 26C7 26 2 16 2 16a25 25 0 0 1 6.2-7.5L1.9 3.4 3.3 2zm6.4 8A22.7 22.7 0 0 0 4.3 16C5.5 18.2 9.6 24 16 24c2.1 0 4-.6 5.7-1.5l-3-3A6 6 0 0 1 12.5 13l-2.8-3z"/>
                          <path d="M16 6c9 0 14 10 14 10a24.9 24.9 0 0 1-4.6 6.1L24 20.7c1.7-1.6 3-3.5 3.7-4.7C26.5 13.8 22.4 8 16 8c-1.5 0-2.8.3-4.1.8L10.4 7.3A14 14 0 0 1 16 6z"/>
                        </svg>
                      </button>
                    </span>
                  </label>
                  <label v-if="capabilityForm.authType === 'token'" class="config-form-wide">
                    <span>Token</span>
                    <span class="password-field-wrap capability-secret-field">
                      <input
                        v-model="capabilityForm.token"
                        class="bx--text-input password-field-input"
                        :type="capabilitySecretVisible('token') ? 'text' : 'password'"
                        autocomplete="off"
                        placeholder="留空则沿用已保存 Secret"
                      />
                      <button
                        type="button"
                        class="password-visible-toggle"
                        :aria-label="capabilitySecretVisible('token') ? '隐藏 Token' : '显示 Token'"
                        :title="capabilitySecretVisible('token') ? '隐藏 Token' : '显示 Token'"
                        :disabled="capabilityCredentialLoading"
                        @click="toggleCapabilitySecret('token')"
                      >
                        <svg v-if="capabilitySecretVisible('token')" focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                          <path d="M16 6C7 6 2 16 2 16s5 10 14 10 14-10 14-10S25 6 16 6zm0 18c-6.4 0-10.5-5.8-11.7-8C5.5 13.8 9.6 8 16 8s10.5 5.8 11.7 8c-1.2 2.2-5.3 8-11.7 8z"/>
                          <path d="M16 10a6 6 0 1 0 0 12 6 6 0 0 0 0-12zm0 10a4 4 0 1 1 0-8 4 4 0 0 1 0 8z"/>
                        </svg>
                        <svg v-else focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                          <path d="m3.3 2 26.7 26.7-1.4 1.4-5.1-5.1A15 15 0 0 1 16 26C7 26 2 16 2 16a25 25 0 0 1 6.2-7.5L1.9 3.4 3.3 2zm6.4 8A22.7 22.7 0 0 0 4.3 16C5.5 18.2 9.6 24 16 24c2.1 0 4-.6 5.7-1.5l-3-3A6 6 0 0 1 12.5 13l-2.8-3z"/>
                          <path d="M16 6c9 0 14 10 14 10a24.9 24.9 0 0 1-4.6 6.1L24 20.7c1.7-1.6 3-3.5 3.7-4.7C26.5 13.8 22.4 8 16 8c-1.5 0-2.8.3-4.1.8L10.4 7.3A14 14 0 0 1 16 6z"/>
                        </svg>
                      </button>
                    </span>
                  </label>
                  <label v-if="capabilityForm.authType === 'existingSecret'" class="config-form-wide">
                    <span>credentialSecretRef</span>
                    <input v-model.trim="capabilityForm.credentialSecretRef" class="bx--text-input" placeholder="namespace/name" />
                  </label>
                  <label class="capability-checkbox-field">
                    <input v-model="capabilityForm.tlsInsecureSkipVerify" type="checkbox" />
                    <span>跳过 TLS 证书校验</span>
                  </label>
                </div>
                <p v-if="capabilityCredentialError" class="modal-error" role="alert">{{ capabilityCredentialError }}</p>
              </div>
            </section>

            <section v-if="configDrawerTab === 'deploy' && configDrawer.kind === 'service' && serviceTypeVersions.length" class="config-section">
              <div class="config-section-title">
                <span>版本</span>
              </div>
              <div class="config-form-grid">
                <label class="service-config-field">
                  <span>{{ serviceStatusHasRuntime(drawerService) ? '当前应用版本' : '选择应用版本' }}</span>
                  <select v-if="!serviceStatusHasRuntime(drawerService)" v-model="selectedChartVersion" class="bx--select-input">
                    <option v-for="tv in serviceTypeVersions" :key="tv.chartVersion" :value="tv.chartVersion">
                      {{ serviceVersionOptionLabel(tv) }}
                    </option>
                  </select>
                  <strong v-else class="service-version-installed">{{ serviceVersionOptionLabel(serviceTemplateForInstallation(drawerService)) }}</strong>
                </label>
              </div>
            </section>

            <section v-if="configDrawerTab === 'deploy' && configDrawer.kind === 'service' && serviceDrawerProfile.showDeploymentConfig" class="config-section">
              <div class="config-section-title">
                <span>部署参数可编辑</span>
                <small>保存后写入服务部署参数；运行中的服务应用后会更新 ServiceInstance。</small>
              </div>
              <div v-if="serviceDrawerVisibleConfigFields.length" class="config-form-grid service-config-form-grid">
                <label v-for="field in serviceDrawerVisibleConfigFields" :key="field.key" class="service-config-field" :class="{ 'config-form-wide': field.control === 'text' }">
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
            </section>

            <section v-if="configDrawerTab === 'deploy' && configDrawer.kind === 'service' && serviceDrawerVolumeFields.length" class="config-section">
              <div class="config-section-title">
                <span>存储卷</span>
                <small>保存后写入当前服务部署参数；运行中的服务会触发更新。</small>
              </div>
              <div class="service-volume-grid">
                <article
                  v-for="volume in serviceDrawerVolumeFields"
                  :key="volume.sizeKey"
                  class="service-volume-card"
                  :class="{ disabled: volume.enabledKey && !serviceConfigForm[volume.enabledKey] }"
                >
                  <div class="service-volume-card__head">
                    <div>
                      <strong>{{ volume.label }}</strong>
                      <small>{{ volume.description }}</small>
                    </div>
                    <label v-if="volume.enabledKey" class="service-volume-toggle">
                      <input
                        type="checkbox"
                        :checked="Boolean(serviceConfigForm[volume.enabledKey])"
                        @change="setServiceVolumeEnabled(volume, $event)"
                      />
                      <span>{{ serviceConfigForm[volume.enabledKey] ? '已启用持久化' : '启用持久化' }}</span>
                    </label>
                  </div>
                  <label class="service-volume-size">
                    <span>容量</span>
                    <input
                      v-model.trim="serviceConfigForm[volume.sizeKey]"
                      class="bx--text-input"
                      :disabled="Boolean(volume.enabledKey) && !serviceConfigForm[volume.enabledKey]"
                      :placeholder="volume.placeholder"
                    />
                  </label>
                  <div class="service-volume-presets" aria-label="常用容量">
                    <button
                      v-for="preset in serviceVolumeSizePresets"
                      :key="`${volume.sizeKey}-${preset}`"
                      type="button"
                      class="service-volume-preset"
                      :class="{ active: String(serviceConfigForm[volume.sizeKey] || '') === preset }"
                      :disabled="Boolean(volume.enabledKey) && !serviceConfigForm[volume.enabledKey]"
                      @click="setServiceVolumeSize(volume, preset)"
                    >
                      {{ preset }}
                    </button>
                  </div>
                </article>
              </div>
            </section>

            <section v-if="serviceDrawerWorkspaceActive" class="config-section config-section--workspace">
              <div class="drawer-workspace-head">
                <div>
                  <div class="config-section-title"><span>{{ serviceDrawerWorkspaceTitle }}</span></div>
                  <p>{{ serviceDrawerWorkspaceDescription }}</p>
                </div>
                <div class="drawer-workspace-actions">
                  <button type="button" class="text-btn" :disabled="serviceDrawerWorkspaceLoading" @click="loadServiceDrawerWorkspace(drawerService)">
                    {{ serviceDrawerWorkspaceLoading ? '刷新中...' : '刷新' }}
                  </button>
                  <a
                    v-if="configDrawerExternalUrl"
                    :href="configDrawerExternalUrl"
                    target="_blank"
                    rel="noreferrer"
                    class="text-btn capability-external-link"
                  >
                    打开外部控制台
                  </a>
                </div>
              </div>
              <div v-if="serviceDrawerWorkspaceActions.length && !serviceDrawerWorkspaceUsesInternalActions" class="workspace-action-strip workspace-action-strip--drawer" aria-label="当前卡片操作">
                <button
                  v-for="action in serviceDrawerWorkspaceActions"
                  :key="action.key"
                  type="button"
                  :class="workspaceActionButtonClass(action)"
                  :title="action.description"
                  :disabled="serviceDrawerWorkspaceLoading"
                  @click="beginDrawerWorkspaceAction(action, action.target)"
                >
                  {{ action.label }}
                </button>
              </div>
              <div v-if="activeCapabilityAction && activeWorkspaceActionBelongsToDrawer && !serviceDrawerWorkspaceEmbedsActions" class="workspace-action-inline workspace-action-inline--drawer" role="region" aria-label="当前卡片操作参数">
                <header>
                  <div>
                    <span>执行操作</span>
                    <strong>{{ activeCapabilityAction.label }}</strong>
                  </div>
                  <button type="button" class="text-btn" :disabled="serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading" @click="closeWorkspaceActionInline">取消</button>
                </header>
                <p v-if="activeCapabilityAction.description">{{ activeCapabilityAction.description }}</p>
                <div class="workspace-action-inline-form">
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
                      @change="setWorkspaceActionCheckboxParam(field.name, $event)"
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
                </div>
                <div class="workspace-action-inline-actions">
                  <button type="button" class="bx--btn bx--btn--secondary" :disabled="serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading" @click="closeWorkspaceActionInline">取消</button>
                  <button type="button" class="bx--btn bx--btn--primary" :disabled="serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading" @click="submitWorkspaceActionInline">
                    {{ serviceDrawerWorkspaceLoading || capabilityWorkspaceLoading ? '执行中...' : '执行' }}
                  </button>
                </div>
              </div>
              <div v-if="serviceDrawerWorkspaceLoading" class="workspace-loading">工作台加载中...</div>
              <component
                v-else-if="serviceDrawerWorkspaceComponent && serviceDrawerWorkspaceReady"
                :key="serviceDrawerWorkspaceKey"
                :is="serviceDrawerWorkspaceComponent"
                :resources="serviceDrawerWorkspaceData.resources"
                :workspace-actions="serviceDrawerWorkspaceActions"
                :initial-subject-key="capabilityInitialSubjectKey"
                v-bind="serviceDrawerWorkspaceEmbeddedActionProps"
                @action="(a: any, t?: string) => beginDrawerWorkspaceAction(a, t)"
              />
              <div v-else-if="serviceDrawerWorkspaceComponent" class="empty-state compact-empty-state">
                <h3 class="empty-title">暂无真实工作台数据</h3>
                <p class="empty-text">{{ serviceDrawerWorkspaceEmptyText }}</p>
              </div>
              <div v-else class="empty-state compact-empty-state">
                <h3 class="empty-title">暂无工作台</h3>
                <p class="empty-text">当前服务类型还没有内置工作台。</p>
              </div>
            </section>

            <section v-if="configDrawerTab === 'variables' && configDrawer.kind === 'service'" class="config-section">
              <div class="config-section-title">
                <span>接入变量只读</span>
                <div class="config-section-actions">
                  <button type="button" class="text-btn" @click="showServiceRawVariables">查看原始参数</button>
                </div>
              </div>
              <div class="config-inline-note">服务接入变量由模板、敏感配置和运行态发现生成；业务组件在配置模板里选择服务后引用这些变量。</div>
              <div v-if="serviceDrawerVariableRows.length" class="config-variable-list">
                <div v-for="row in serviceDrawerVariableRows" :key="row.name" class="config-variable-row">
                  <span class="config-variable-name">{{ row.name }}</span>
                  <span class="config-variable-value">
                    <code :class="{ masked: row.secret && !serviceDrawerSecretVisible(row.name) }">{{ row.secret ? serviceDrawerSecretDisplayValue(row) : row.value }}</code>
                    <button
                      v-if="row.secret"
                      type="button"
                      class="service-secret-reveal-btn"
                      :disabled="serviceDrawerSecretLoadingKey === row.name"
                      :aria-label="serviceDrawerSecretVisible(row.name) ? '隐藏密码' : '显示密码'"
                      :title="serviceDrawerSecretVisible(row.name) ? '隐藏密码' : '显示密码'"
                      @click="toggleServiceDrawerSecret(row)"
                    >
                      <svg v-if="serviceDrawerSecretVisible(row.name)" focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                        <path d="M16 6C7 6 2 16 2 16s5 10 14 10 14-10 14-10S25 6 16 6zm0 18c-6.4 0-10.5-5.8-11.7-8C5.5 13.8 9.6 8 16 8s10.5 5.8 11.7 8c-1.2 2.2-5.3 8-11.7 8z"/>
                        <path d="M16 10a6 6 0 1 0 0 12 6 6 0 0 0 0-12zm0 10a4 4 0 1 1 0-8 4 4 0 0 1 0 8z"/>
                      </svg>
                      <svg v-else focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
                        <path d="m3.3 2 26.7 26.7-1.4 1.4-5.1-5.1A15 15 0 0 1 16 26C7 26 2 16 2 16a25 25 0 0 1 6.2-7.5L1.9 3.4 3.3 2zm6.4 8A22.7 22.7 0 0 0 4.3 16C5.5 18.2 9.6 24 16 24c2.1 0 4-.6 5.7-1.5l-3-3A6 6 0 0 1 12.5 13l-2.8-3z"/>
                        <path d="M16 6c9 0 14 10 14 10a24.9 24.9 0 0 1-4.6 6.1L24 20.7c1.7-1.6 3-3.5 3.7-4.7C26.5 13.8 22.4 8 16 8c-1.5 0-2.8.3-4.1.8L10.4 7.3A14 14 0 0 1 16 6z"/>
                      </svg>
                    </button>
                  </span>
                  <small>{{ row.hint }}</small>
                </div>
              </div>
              <div v-else class="config-empty">当前服务还没有可复制的接入变量。</div>
            </section>

            <section v-if="configDrawerTab === 'variables' && configDrawer.kind === 'service' && serviceDrawerProfile.showConnectionBindings" class="config-section">
              <div class="config-section-title"><span>连接参数</span></div>
              <div class="config-readonly-list">
                <div v-for="binding in serviceDrawerConnectionPreview.bindings" :key="binding.name" class="config-readonly-row">
                  <strong>{{ binding.name }}</strong>
                  <span>{{ binding.value }}</span>
                </div>
              </div>
            </section>

            <section v-if="configDrawerTab === 'runtime' && configDrawer.kind === 'service' && serviceDrawerProfile.showTopology" class="config-section">
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

            <section v-if="configDrawerTab === 'database' && configDrawer.kind === 'service'" class="config-section">
              <div class="config-section-title"><span>数据库</span></div>
              <div class="config-kv-grid service-summary-grid">
                <div v-for="row in serviceDrawerDatabaseRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
              <div class="config-inline-note">应用连接数据库时优先引用连接串或用户名密码，不需要手动记忆长 key。</div>
            </section>

            <section v-if="configDrawerTab === 'backups' && configDrawer.kind === 'service'" class="config-section">
              <div class="config-section-title">
                <span>备份</span>
                <div class="config-section-actions">
                  <button type="button" class="text-btn" :disabled="serviceDrawerWorkspaceLoading" @click="loadServiceDrawerWorkspace(drawerService)">
                    {{ serviceDrawerWorkspaceLoading ? '刷新中...' : '刷新' }}
                  </button>
                  <button type="button" class="rail-btn rail-btn--primary rail-btn--sm" :disabled="serviceDrawerWorkspaceLoading" @click="beginDrawerWorkspaceAction(serviceDrawerBackupAction)">
                    创建备份
                  </button>
                </div>
              </div>
              <div class="config-kv-grid service-summary-grid">
                <div>
                  <span>策略</span>
                  <strong>手动快照</strong>
                </div>
                <div>
                  <span>存储</span>
                  <strong>{{ serviceDrawerBackupStorage }}</strong>
                </div>
                <div>
                  <span>备份数</span>
                  <strong>{{ serviceDrawerBackups.length }}</strong>
                </div>
                <div>
                  <span>默认库</span>
                  <strong>{{ serviceDrawerDefaultDatabase }}</strong>
                </div>
              </div>
              <div v-if="serviceDrawerWorkspaceLoading" class="workspace-loading">正在读取备份列表...</div>
              <div v-else-if="serviceDrawerBackups.length" class="backup-list">
                <div v-for="backup in serviceDrawerBackups" :key="backup.name" class="backup-row">
                  <div>
                    <strong>{{ backup.annotations?.database || backup.name }}</strong>
                    <small>{{ backup.annotations?.createdAt || backup.description }}</small>
                  </div>
                  <span>{{ backup.annotations?.tables || 0 }} tables</span>
                  <span>{{ backup.annotations?.rows || 0 }} rows</span>
                  <code>{{ backup.annotations?.size || '-' }}</code>
                  <small>{{ backupStorageLabel(backup) }}</small>
                </div>
              </div>
              <div v-else class="config-empty">当前数据库还没有备份。点击“创建备份”会导出真实表结构和数据，并保存为平台备份。</div>
            </section>

            <section v-if="configDrawerTab === 'data' && configDrawer.kind === 'service'" class="config-section">
              <div class="config-section-title"><span>Redis 数据</span></div>
              <div class="config-kv-grid service-summary-grid">
                <div v-for="row in serviceDrawerDataRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
              <div v-if="serviceDrawerWorkspaceReady" class="config-inline-note">上方工作台展示当前 Redis 的真实 keyspace、实例信息和对象级操作；刷新会重新读取运行态。</div>
              <div v-else class="config-empty">部署成功后这里会展示 keyspace、内存、过期键和对象级操作。</div>
            </section>

            <section v-if="configDrawerTab === 'queues' && configDrawer.kind === 'service'" class="config-section">
              <div class="config-section-title"><span>{{ serviceDrawerType === 'kafka' ? '主题' : '队列' }}</span></div>
              <div class="config-kv-grid service-summary-grid">
                <div v-for="row in serviceDrawerQueueRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
              <div v-if="serviceDrawerWorkspaceReady" class="config-inline-note">上方工作台展示当前 {{ serviceDrawerType === 'kafka' ? 'Kafka Topic' : 'RabbitMQ 队列、Exchange、VHost 和绑定' }} 的真实对象和操作。</div>
              <div v-else class="config-empty">部署成功后这里会展示队列、Topic、消费者和对象级操作。</div>
            </section>

            <section v-if="configDrawerTab === 'buckets' && configDrawer.kind === 'service'" class="config-section">
              <div class="config-section-title"><span>存储桶</span></div>
              <div class="config-kv-grid service-summary-grid">
                <div v-for="row in serviceDrawerBucketRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
              <div v-if="serviceDrawerWorkspaceReady" class="config-inline-note">上方工作台展示当前 Bucket、对象和访问连接状态，所有对象操作都会调用真实 MinIO 接口。</div>
              <div v-else class="config-empty">部署成功后这里会展示 Bucket、对象和访问密钥状态。</div>
            </section>

            <section v-if="configDrawerTab === 'logs'" class="config-section">
              <div class="config-section-title">
                <span>运行日志</span>
                <button type="button" class="text-btn" :disabled="runtimeLogsLoading" @click="loadDrawerRuntimeLogs(true)">
                  {{ runtimeLogsLoading ? '刷新中...' : '刷新' }}
                </button>
              </div>
              <div v-if="runtimeLogsLoading" class="workspace-loading">正在读取当前卡片的真实日志...</div>
              <div v-else-if="runtimeLogsError" class="modal-error" role="alert">{{ runtimeLogsError }}</div>
              <div v-else-if="runtimeLogs" class="runtime-logs-panel">
                <div v-if="!runtimeLogs.available" class="config-inline-note">
                  当前卡片还没有返回日志：{{ runtimeLogs.error || '暂无可读取日志' }}
                </div>
                <div class="runtime-log-meta">
                  <span>{{ runtimeLogSamples.length }} 个运行实例</span>
                  <span>tail {{ runtimeLogs.tailLines || 200 }}</span>
                  <span>{{ runtimeLogs.updatedAt || '-' }}</span>
                </div>
                <div v-if="runtimeLogSamples.length" class="runtime-log-list">
                  <article v-for="(sample, idx) in runtimeLogSamples" :key="`${sample.pod}:${sample.container}`" class="runtime-log-sample">
                    <header>
                      <strong>运行实例 {{ Number(idx) + 1 }}</strong>
                      <small>{{ sample.status || 'Unknown' }}</small>
                    </header>
                    <pre v-if="sample.text" class="runtime-log-output">{{ sample.text }}</pre>
                    <div v-else-if="sample.error" class="modal-error" role="alert">{{ sample.error }}</div>
                    <div v-else class="config-empty">这个运行实例暂无日志。</div>
                  </article>
                </div>
                <div v-else class="config-empty">当前没有可展示的日志流。</div>
              </div>
              <div v-else class="config-empty">点击刷新读取当前卡片日志。</div>
            </section>

            <section v-if="configDrawerTab === 'console'" class="config-section">
              <div class="config-section-title">
                <span>控制台</span>
                <div class="config-section-actions">
                  <button type="button" class="text-btn" :disabled="runtimeConsoleConnecting || runtimeConsoleConnected" @click="connectDrawerConsole">
                    {{ runtimeConsoleConnecting ? '连接中...' : '连接' }}
                  </button>
                  <button type="button" class="text-btn" :disabled="!runtimeConsoleConnected && !runtimeConsoleConnecting" @click="disconnectDrawerConsole">
                    断开
                  </button>
                </div>
              </div>
              <div class="runtime-console-panel">
                <div class="runtime-console-status" :class="{ connected: runtimeConsoleConnected, connecting: runtimeConsoleConnecting }">
                  <span>{{ runtimeConsoleStatusText }}</span>
                  <small>{{ drawerConsoleTargetLabel }}</small>
                </div>
                <div ref="runtimeConsoleView" class="runtime-console-output"></div>
                <p v-if="!runtimeConsoleConnected && !runtimeConsoleConnecting" class="runtime-console-hint">点击“连接”进入当前卡片的运行实例；如果应用镜像缺少命令工具，PAAP 会自动准备调试环境。</p>
                <p v-if="runtimeConsoleError" class="modal-error" role="alert">{{ runtimeConsoleError }}</p>
              </div>
            </section>

            <section v-if="configDrawerTab === 'capabilities' && configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title"><span>识别与声明</span></div>
              <div class="config-form-grid">
                <label>
                  <span>组件用途</span>
                  <select v-model="componentDrawerRole" class="bx--select-input">
                    <option v-for="option in componentDrawerRoleOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                  </select>
                </label>
                <label>
                  <span>应用框架</span>
                  <select v-model="configForm.framework" class="bx--select-input" @change="syncGeneratedSpringConfig">
                    <option v-for="option in configFrameworkOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                  </select>
                </label>
              </div>
              <div class="config-kv-grid service-summary-grid">
                <div v-for="row in componentDrawerCapabilityRows" :key="row.label">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
              <div v-if="componentDrawerSuggestedActions.length" class="component-capability-actions">
                <button
                  v-for="action in componentDrawerSuggestedActions"
                  :key="action.key"
                  type="button"
                  class="rail-btn rail-btn--secondary rail-btn--sm"
                  @click="runComponentDrawerSuggestion(action.key)"
                >
                  {{ action.label }}
                </button>
              </div>
              <div class="config-inline-note">PAAP 不假设业务框架。识别结果来自镜像、源码字段、运行态端口、环境变量、配置引用和用户在画布上的连接。</div>
            </section>

            <section v-if="configDrawerTab === 'dependencies' && configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title"><span>运行依赖</span></div>
              <div class="config-binding-form">
                <select v-model="configForm.bindingTargetKey" class="bx--select-input">
                  <option value="">选择数据库、缓存、消息队列或对象存储</option>
                  <option v-for="target in componentDrawerDependencyTargets" :key="target.key" :value="target.key">
                    {{ targetOptionLabel(target) }}
                  </option>
                </select>
                <button type="button" class="bx--btn bx--btn--primary bx--btn--sm" :disabled="configDrawer.saving || !selectedConnectionTarget" @click="applySelectedConfigBinding">
                  应用连接
                </button>
              </div>
              <div v-if="configForm.bindings.length" class="config-binding-list">
                <div v-for="(binding, idx) in configForm.bindings" :key="`${binding.targetKey || binding.targetName}-${idx}`" class="config-binding-row">
                  <span>
                    <strong>{{ binding.targetName }}</strong>
                    <small>{{ typeLabel(binding.targetType) || binding.targetType }} · {{ binding.mode || 'env' }}</small>
                  </span>
                  <code>{{ Object.keys(binding.generated || {}).join(', ') || binding.role }}</code>
                  <button type="button" class="text-btn danger" @click="removeConfigBinding(idx)">移除</button>
                </div>
              </div>
              <div v-else class="config-empty">当前后端还没有绑定数据库、缓存或中间件。</div>
            </section>


            <section v-if="configDrawerTab === 'deploy' && configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title">
                <span>交付方式</span>
                <button type="button" class="text-btn" :disabled="registryWorkspaceLoading" @click="ensureRegistryWorkspaces">
                  {{ registryWorkspaceLoading ? '刷新中...' : '刷新' }}
                </button>
              </div>
              <div class="delivery-switch component-delivery-switch">
                <label class="delivery-option" :class="{ active: configForm.deliveryMode === 'image' }">
                  <input v-model="configForm.deliveryMode" type="radio" value="image" />
                  <span>镜像交付</span>
                </label>
                <label class="delivery-option" :class="{ active: configForm.deliveryMode === 'source' }">
                  <input v-model="configForm.deliveryMode" type="radio" value="source" />
                  <span>源码交付</span>
                </label>
              </div>
              <p class="cds-helper-text">
                {{ componentDrawerUsesSourceDelivery ? '源码交付无需 Dockerfile，由平台通过 Buildpacks/kpack 识别源码运行时并构建镜像。' : '镜像交付使用已构建镜像，镜像:Tag 必须填写明确 Tag。' }}
              </p>
              <div v-if="!componentDrawerUsesSourceDelivery" class="cds-image-fields">
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-registry-target">镜像仓库</label>
                  <select
                    id="drawer-registry-target"
                    v-model="configForm.registryTargetKey"
                    class="cds-text-input"
                    @change="syncRegistryTargetSelection"
                  >
                    <option value="">请选择环境镜像仓库</option>
                    <option v-for="target in registryTargetOptions" :key="target.key" :value="target.key">
                      {{ target.label }}
                    </option>
                  </select>
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-repo">仓库地址</label>
                  <input
                    id="drawer-repo"
                    v-model.trim="configForm.repository"
                    class="cds-text-input"
                    :readonly="selectedRegistryTarget?.source !== 'external'"
                    :title="configForm.repository || '等待镜像仓库地址'"
                    placeholder="等待镜像仓库地址"
                  />
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-tag">镜像:Tag</label>
                  <input
                    id="drawer-tag"
                    v-model.trim="configForm.imageTag"
                    class="cds-text-input"
                    list="component-registry-image-tags"
                    placeholder="例如 paap/orders-api:v1.0.0"
                    @change="syncConfigVersionFromImageTag"
                  />
                  <datalist id="component-registry-image-tags">
                    <option v-for="option in registryImageTagOptions" :key="option" :value="option">{{ option }}</option>
                  </datalist>
                </div>
              </div>
              <div v-else class="cds-image-fields">
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-source-registry-target">镜像仓库</label>
                  <select
                    id="drawer-source-registry-target"
                    v-model="configForm.registryTargetKey"
                    class="cds-text-input"
                    @change="syncRegistryTargetSelection"
                  >
                    <option value="">请选择构建镜像仓库</option>
                    <option v-for="target in registryTargetOptions" :key="target.key" :value="target.key">
                      {{ target.label }}
                    </option>
                  </select>
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-source-repo-host">目标仓库地址</label>
                  <input
                    id="drawer-source-repo-host"
                    v-model.trim="configForm.repository"
                    class="cds-text-input"
                    :readonly="selectedRegistryTarget?.source !== 'external'"
                    :title="configForm.repository || '等待镜像仓库地址'"
                    placeholder="等待镜像仓库地址"
                  />
                </div>
                <div class="cds-image-field cds-image-field--full">
                  <label class="cds-label" for="drawer-source-repo">源码仓库</label>
                  <input
                    id="drawer-source-repo"
                    v-model.trim="configForm.sourceRepoUrl"
                    class="cds-text-input"
                    placeholder="https://git.example.com/team/app.git"
                  />
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-source-branch">源码分支</label>
                  <input
                    id="drawer-source-branch"
                    v-model.trim="configForm.sourceBranch"
                    class="cds-text-input"
                    placeholder="main"
                  />
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-build-context">构建上下文</label>
                  <input
                    id="drawer-build-context"
                    v-model.trim="configForm.buildContext"
                    class="cds-text-input"
                    placeholder="."
                  />
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-build-module">构建模块</label>
                  <input
                    id="drawer-build-module"
                    v-model.trim="configForm.buildModule"
                    class="cds-text-input"
                    placeholder="例如 gateway"
                  />
                </div>
                <div class="cds-image-field">
                  <label class="cds-label" for="drawer-source-version">版本标签</label>
                  <input
                    id="drawer-source-version"
                    v-model.trim="configForm.version"
                    class="cds-text-input"
                    placeholder="v1.0.0"
                  />
                </div>
              </div>
              <div class="cds-image-preview" v-if="!componentDrawerUsesSourceDelivery && registryImageFromConfig">
                <svg class="cds-image-preview__icon" width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm0 12.5a5.5 5.5 0 1 1 0-11 5.5 5.5 0 0 1 0 11zM8 4a.75.75 0 0 1 .75.75v2.5h2.5a.75.75 0 0 1 0 1.5h-2.5v2.5a.75.75 0 0 1-1.5 0v-2.5h-2.5a.75.75 0 0 1 0-1.5h2.5v-2.5A.75.75 0 0 1 8 4z"/></svg>
                <span class="cds-image-preview__label">完整镜像</span>
                <code class="cds-image-preview__text">{{ registryImageFromConfig }}</code>
              </div>
              <div class="cds-image-preview" v-if="componentDrawerUsesSourceDelivery && sourceDeliveryImagePreview">
                <svg class="cds-image-preview__icon" width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm0 12.5a5.5 5.5 0 1 1 0-11 5.5 5.5 0 0 1 0 11zM8 4a.75.75 0 0 1 .75.75v2.5h2.5a.75.75 0 0 1 0 1.5h-2.5v2.5a.75.75 0 0 1-1.5 0v-2.5h-2.5a.75.75 0 0 1 0-1.5h2.5v-2.5A.75.75 0 0 1 8 4z"/></svg>
                <span class="cds-image-preview__label">构建镜像</span>
                <code class="cds-image-preview__text">{{ sourceDeliveryImagePreview }}</code>
              </div>
              <p v-if="!componentDrawerUsesSourceDelivery" class="cds-helper-text">仓库地址来自当前环境镜像仓库；镜像:Tag 填写镜像名和明确 Tag。</p>
              <p v-else class="cds-helper-text">源码构建完成后会推送到所选镜像仓库；本环境或共享资源使用集群内地址完成构建推送。</p>
              <p v-if="registryWorkspaceError" class="modal-error" role="alert">{{ registryWorkspaceError }}</p>
            </section>

            <section v-if="configDrawerTab === 'deploy' && configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title"><span>规格</span></div>
              <div class="config-form-grid">
                <label>
                  <span>副本</span>
                  <input v-model.number="configForm.replicas" type="number" min="0" class="bx--text-input" />
                </label>
                <label>
                  <span>容器端口</span>
                  <input v-model.number="configForm.containerPort" type="number" min="1" max="65535" class="bx--text-input" @input="configForm.containerPortSource = 'user'" />
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
              <details class="config-advanced-details">
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

            <section v-if="configDrawerTab === 'variables' && configDrawer.kind === 'component'" class="config-section">
              <div class="config-section-title"><span>配置</span></div>
              <div class="component-config-flow component-config-flow--guided">
                <div class="component-template-picker">
                  <label class="component-template-select">
                    <span>配置模板</span>
                    <select v-model="selectedComponentConfigTemplateId" class="bx--select-input">
                      <option value="">{{ componentConfigTemplatesLoading ? '配置模板加载中...' : '请选择配置模板' }}</option>
                      <option
                        v-for="template in componentSelectableConfigTemplates"
                        :key="componentTemplateOptionValue(template)"
                        :value="componentTemplateOptionValue(template)"
                      >
                        {{ template.name }}
                      </option>
                    </select>
                  </label>
                  <p v-if="selectedComponentConfigTemplate" class="component-template-helper">
                    {{ componentTemplateInlineHelp(selectedComponentConfigTemplate) }}
                  </p>
                  <p v-else class="component-template-helper">
                    未选择模板时，直接编辑当前组件已有的环境变量、配置文件和手工覆盖项。
                  </p>
                </div>

                <ComponentConfigTemplateFields
                  v-if="componentTemplateFields.length"
                  v-model:field-values="componentTemplateFieldValues"
                  :fields="componentTemplateFields"
                  :required-for-user="componentTemplateFieldRequiredForUser"
                  :field-hint="componentTemplateFieldHint"
                  :field-options="componentTemplateFieldOptions"
                  :field-uses-target-select="componentTemplateFieldUsesTargetSelect"
                  :field-placeholder="componentTemplateFieldPlaceholder"
                />
                <div v-else-if="selectedComponentConfigTemplate" class="config-empty">这个模板不需要额外填写，直接应用即可。</div>

                <p v-if="selectedComponentConfigTemplate && componentTemplateRequiredFieldsComplete" class="component-template-save-note">
                  保存配置时会按当前字段生成运行配置并建立服务连接。
                </p>
                <p v-else-if="selectedComponentConfigTemplate" class="component-config-warning">请先补全必填项。</p>

                <div v-if="componentCurrentConfigRows.length" class="component-current-config-panel">
                  <div class="component-current-config-head">
                    <span>当前配置</span>
                    <small>{{ componentCurrentConfigRows.length }} 项</small>
                  </div>
                  <div class="component-current-config-list">
                    <div v-for="row in componentCurrentConfigRows" :key="row.key" class="component-current-config-row">
                      <span>
                        <strong>{{ row.name }}</strong>
                        <small>{{ row.source }}</small>
                      </span>
                      <code>{{ row.value }}</code>
                    </div>
                  </div>
                </div>
                <div v-else-if="!selectedComponentConfigTemplate" class="config-empty">当前组件没有业务配置，可直接保存部署参数。</div>

                <details
                  v-if="!selectedComponentConfigTemplate || configForm.files.length || configForm.env.length || componentNginxRouteEditorVisible"
                  class="component-template-custom-config component-template-advanced-config"
                  :open="componentAdvancedConfigOpenByDefault"
                >
                  <summary>
                    <span>高级配置</span>
                    <small>配置文件挂载、代理路由和手工覆盖项</small>
                  </summary>
                  <div v-if="configForm.files.length" class="component-file-mount-panel">
                    <div class="component-file-mount-head">
                      <span>配置文件挂载</span>
                    </div>
                    <div class="component-file-mount-list">
                      <div v-for="(file, idx) in configForm.files" :key="`${file.configMapName}-${file.key}-${idx}`" class="component-file-mount-row">
                        <div class="component-file-mount-source">
                          <strong>{{ file.key || file.name }}</strong>
                          <small>{{ file.configMapName }}</small>
                        </div>
                        <input v-model.trim="file.mountPath" class="bx--text-input" placeholder="挂载路径" aria-label="配置文件挂载路径" />
                        <label class="component-file-readonly">
                          <input v-model="file.readOnly" type="checkbox" />
                          <span>只读</span>
                        </label>
                      </div>
                    </div>
                  </div>
                  <div v-if="componentNginxRouteEditorVisible" class="nginx-route-panel">
                    <div class="nginx-route-head">
                      <span>代理路由</span>
                      <button type="button" class="icon-btn icon-btn--compact" aria-label="添加代理路由" title="添加代理路由" @click="addNginxRoute">
                        <svg focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M17 15V6h-2v9H6v2h9v9h2v-9h9v-2z"/></svg>
                      </button>
                    </div>
                    <div v-if="nginxRouteRows.length" class="nginx-route-list">
                      <div v-for="(route, idx) in nginxRouteRows" :key="idx" class="nginx-route-row">
                        <input v-model.trim="route.path" class="bx--text-input" placeholder="匹配路径" aria-label="匹配路径" />
                        <input
                          v-model.trim="route.targetUrl"
                          class="bx--text-input"
                          list="nginx-route-target-options"
                          placeholder="转发地址"
                          aria-label="转发地址"
                          @change="syncNginxRouteTargetUrl(route)"
                        />
                        <button type="button" class="icon-btn icon-btn--danger icon-btn--compact" aria-label="删除代理路由" title="删除代理路由" @click="removeNginxRoute(idx)">
                          <svg focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M12 12h2v12h-2zm6 0h2v12h-2z"/><path d="M4 6v2h2v20c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8h2V6H4zm4 22V8h16v20H8zM12 2h8v2h-8z"/></svg>
                        </button>
                      </div>
                      <datalist id="nginx-route-target-options">
                        <option v-for="target in componentDrawerBackendTargets" :key="target.key" :value="nginxTargetUrl(target)">
                          {{ target.name }}
                        </option>
                      </datalist>
                      <button type="button" class="rail-btn rail-btn--secondary rail-btn--sm nginx-route-apply" @click="() => applyNginxRoutes()">更新代理配置</button>
                    </div>
                  </div>
                  <div class="component-advanced-tools">
                    <button type="button" class="text-btn" @click="addConfigEnv">添加环境变量</button>
                  </div>
                  <div v-if="configForm.env.length" class="config-env-list">
                    <div v-for="(envItem, idx) in configForm.env" :key="idx" class="config-env-row" :class="{ 'config-env-row--managed': envItem.managed }">
                      <template v-if="envItem.managed">
                        <strong class="config-env-managed-name">{{ envItem.name }}</strong>
                        <span class="config-env-managed-secret">平台注入</span>
                        <span class="config-env-managed-value">{{ managedEnvDisplay(envItem) }}</span>
                      </template>
                      <template v-else>
                        <input v-model.trim="envItem.name" class="bx--text-input" list="component-config-key-suggestions" placeholder="NAME" />
                        <select v-model="envItem.source" class="bx--select-input" @change="normalizeConfigEnvSource(envItem)">
                          <option value="value">直接填写</option>
                          <option value="configMap">应用配置</option>
                          <option value="secret">敏感项</option>
                        </select>
                        <input v-if="envItem.source === 'value'" v-model="envItem.value" class="bx--text-input" placeholder="value" />
                        <template v-else-if="envItem.source === 'configMap'">
                          <span class="config-env-managed-secret">平台管理</span>
                          <input v-model.trim="envItem.refKey" class="bx--text-input" :placeholder="envItem.name || '配置项'" />
                        </template>
                        <template v-else>
                          <span class="config-env-managed-secret">平台管理</span>
                          <input v-model.trim="envItem.refKey" class="bx--text-input" :placeholder="envItem.name || '敏感项'" />
                        </template>
                      </template>
                      <button type="button" class="text-btn danger" @click="removeConfigEnv(idx)">删除</button>
                    </div>
                    <datalist id="component-config-key-suggestions">
                      <option v-for="item in componentDrawerConfigKeySuggestions" :key="item.key" :value="item.key">{{ item.description }}</option>
                    </datalist>
                  </div>
                </details>
              </div>
            </section>

            <section v-if="configDrawerTab === 'variables' && configDrawer.kind === 'service' && (drawerRuntime.env || []).length" class="config-section">
              <div class="config-section-title">
                <span>运行参数</span>
              </div>
              <div class="config-readonly-list">
                <div v-for="envItem in drawerRuntime.env || []" :key="envItem.name" class="config-readonly-row">
                  <strong>{{ envItem.name }}</strong>
                  <span>{{ runtimeEnvValue(envItem) }}</span>
                </div>
                <div v-if="!(drawerRuntime.env || []).length" class="config-empty">当前未发现显式环境变量。</div>
              </div>
            </section>


            <section v-if="configDrawerTab === 'runtime'" class="config-section">
              <div class="config-section-title">
                <span>运行指标</span>
                <button type="button" class="text-btn" :disabled="runtimeMetricsLoading" @click="loadDrawerRuntimeMetrics(true)">
                  {{ runtimeMetricsLoading ? '刷新中...' : '刷新' }}
                </button>
              </div>
              <div v-if="runtimeMetricsLoading" class="workspace-loading">正在读取当前卡片的 CPU 和内存占用...</div>
              <div v-else-if="runtimeMetricsError" class="modal-error" role="alert">{{ runtimeMetricsError }}</div>
              <div v-else-if="runtimeMetrics" class="runtime-metrics-panel">
                <div v-if="!runtimeMetrics.available" class="config-inline-note">
                  当前暂时拿不到运行指标：{{ runtimeMetrics.error || '平台没有返回运行指标' }}
                </div>
                <div class="runtime-metric-cards">
                  <div v-for="card in runtimeMetricCards" :key="card.label" class="runtime-metric-card">
                    <span>{{ card.label }}</span>
                    <strong>{{ card.value }}</strong>
                    <small>{{ card.hint }}</small>
                  </div>
                </div>
                <div v-if="runtimeMetricSamples.length" class="runtime-metric-chart-grid">
                  <div v-for="chart in runtimeMetricCharts" :key="chart.key" class="runtime-metric-chart">
                    <header>
                      <span>{{ chart.label }}</span>
                      <strong>{{ chart.value }}</strong>
                    </header>
                    <svg viewBox="0 0 320 120" role="img" :aria-label="chart.label">
                      <line x1="12" y1="24" x2="308" y2="24" />
                      <line x1="12" y1="64" x2="308" y2="64" />
                      <line x1="12" y1="104" x2="308" y2="104" />
                      <path :d="chart.areaPath" />
                      <polyline :points="chart.points" />
                      <circle v-for="point in chart.pointList" :key="point.key" :cx="point.x" :cy="point.y" r="3" />
                    </svg>
                    <footer>
                      <span>{{ chart.minLabel }}</span>
                      <span>{{ chart.maxLabel }}</span>
                    </footer>
                  </div>
                </div>
                <div v-if="runtimeMetricSamples.length" class="runtime-metric-list">
                  <div v-for="(sample, idx) in runtimeMetricSamples" :key="`${sample.pod}:${sample.container}`" class="runtime-metric-row">
                    <div class="runtime-metric-row__head">
                      <span>
                        <strong>运行实例 {{ Number(idx) + 1 }}</strong>
                        <small>{{ sample.status || 'Unknown' }}</small>
                      </span>
                      <small>重启 {{ sample.restarts ?? 0 }}</small>
                    </div>
                    <div class="runtime-metric-bars">
                      <div>
                        <span>CPU</span>
                        <strong>{{ sample.cpu || '-' }}</strong>
                        <i><b :style="runtimeMetricBarStyle(sample, 'cpu')"></b></i>
                      </div>
                      <div>
                        <span>Memory</span>
                        <strong>{{ sample.memory || '-' }}</strong>
                        <i><b :style="runtimeMetricBarStyle(sample, 'memory')"></b></i>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-else class="config-empty">当前没有可展示的运行指标。</div>
              </div>
            </section>

            <p v-if="configDrawer.message" class="workspace-message" role="status">{{ configDrawer.message }}</p>
            <p v-if="configDrawer.error" class="modal-error" role="alert">{{ configDrawer.error }}</p>
          </div>

          <footer class="config-drawer-footer">
            <button v-if="configDrawer.kind === 'component'" type="button" class="text-btn danger" :disabled="configDrawer.saving" @click="deleteDrawerComponent">删除组件</button>
            <button v-if="configDrawer.kind === 'service'" type="button" class="text-btn danger" :disabled="uninstallSubmitting" @click="deleteDrawerService">卸载服务</button>
            <button v-if="configDrawer.kind === 'capability'" type="button" class="text-btn danger" :disabled="configDrawer.saving" @click="deleteDrawerCapability">{{ capabilityRemovalActionLabel(drawerCapability) }}</button>
            <button type="button" class="bx--btn bx--btn--secondary" @click="closeConfigDrawer">取消</button>
            <button v-if="configDrawer.kind === 'component'" type="button" class="bx--btn bx--btn--secondary" :disabled="configDrawer.saving" @click="() => saveConfigDrawer()">
              {{ configDrawer.saving ? '保存中...' : '保存配置' }}
            </button>
            <button v-if="configDrawer.kind === 'component'" type="button" class="bx--btn bx--btn--primary" :disabled="configDrawer.saving || componentActionLoading" @click="deployDrawerComponent">
              {{ componentActionLoading ? '部署中...' : '部署' }}
            </button>
            <button v-if="configDrawer.kind === 'service' && serviceDrawerProfile.showDeploymentConfig" type="button" class="bx--btn bx--btn--secondary" :disabled="configDrawer.saving || !serviceDrawerConfigurable" @click="saveServiceConfigDrawer">
              {{ configDrawer.saving ? '保存中...' : '保存部署配置' }}
            </button>
            <button v-if="configDrawer.kind === 'capability' && drawerCapability?.source === 'external'" type="button" class="bx--btn bx--btn--secondary" :disabled="configDrawer.saving || capabilityValidationLoading" @click="validateCapabilityConfigDrawer">
              {{ capabilityValidationLoading ? '验证中...' : '验证连接' }}
            </button>
            <button v-if="configDrawer.kind === 'capability' && drawerCapability?.source === 'external'" type="button" class="bx--btn bx--btn--secondary" :disabled="configDrawer.saving" @click="saveCapabilityConfigDrawer">
              {{ configDrawer.saving ? '保存中...' : '保存外部配置' }}
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
        <button v-if="componentContextMenu.kind === 'canvas' && !isSystemSharedEnvironment" type="button" @mouseenter="openComponentSubmenu" @click="openComponentSubmenu">
          <MenuIconComponent class="menu-icon" />
          <span>创建组件</span>
          <small>前端、后端、自定义组件</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <button v-if="componentContextMenu.kind === 'canvas'" type="button" @mouseenter="openToolSubmenu" @click="openToolSubmenu">
          <MenuIconTool class="menu-icon" />
          <span>添加工具</span>
          <small>Git、镜像仓库、部署、监控、日志工具</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <button v-if="componentContextMenu.kind === 'canvas'" type="button" @mouseenter="openInfraSubmenu" @click="openInfraSubmenu">
          <MenuIconMiddleware class="menu-icon" />
          <span>添加中间件</span>
          <small>数据库、缓存、消息队列</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <button v-if="componentContextMenu.kind === 'canvas' && !isSystemSharedEnvironment" type="button" @mouseenter="openSharedCapabilitySubmenu" @click="openSharedCapabilitySubmenu">
          <MenuIconShared class="menu-icon" />
          <span>添加共享资源</span>
          <small>引用平台共享资源池中的工具或中间件</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <button v-if="componentContextMenu.kind === 'canvas' && !isSystemSharedEnvironment" type="button" @mouseenter="openExternalCapabilitySubmenu" @click="openExternalCapabilitySubmenu">
          <MenuIconExternal class="menu-icon" />
          <span>添加外部资源</span>
          <small>接入外部代码、镜像、数据库、中间件或观测资源</small>
          <svg class="submenu-arrow" width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
        </button>
        <div v-if="componentContextMenu.kind === 'canvas' && !isSystemSharedEnvironment" class="context-menu-divider"></div>
        <button v-if="componentContextMenu.kind === 'canvas' && !isSystemSharedEnvironment" type="button" @click="adoptCanvasResource">
          <MenuIconAdopt class="menu-icon" />
          <span>纳管已有资源</span>
          <small>接入集群现有资源</small>
        </button>
        <button v-if="componentContextMenu.kind === 'edge'" type="button" @click="deleteContextEdge">
          <MenuIconDelete class="menu-icon" />
          <span>删除连线</span>
          <small>只删除这条手动画布连线</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component' || componentContextMenu.kind === 'service' || componentContextMenu.kind === 'capability'" type="button" @click="configureContextNode">
          <MenuIconConfigure class="menu-icon" />
          <span>配置</span>
          <small>{{ componentContextMenu.kind === 'capability' ? '在右侧查看或配置能力来源' : (componentContextMenu.kind === 'service' ? '在右侧查看工具或中间件配置' : '在右侧配置环境变量、副本和启动参数') }}</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component' || componentContextMenu.kind === 'service' || componentContextMenu.kind === 'capability'" type="button" @click="renameContextNode">
          <MenuIconRename class="menu-icon" />
          <span>重命名</span>
          <small>修改画布上显示的卡片名称</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component'" type="button" @click="deployContextComponent">
          <MenuIconDeploy class="menu-icon" />
          <span>部署</span>
          <small>提交 GitOps 并同步</small>
        </button>
        <button v-if="componentContextMenu.kind === 'service'" type="button" @click="deployContextService">
          <MenuIconDeploy class="menu-icon" />
          <span>部署</span>
          <small>部署或应用当前服务配置</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component'" type="button" @click="deleteContextComponent">
          <MenuIconDelete class="menu-icon" />
          <span>删除</span>
          <small>删除组件草稿和运行态 CR</small>
        </button>
        <button v-if="componentContextMenu.kind === 'service'" type="button" @click="deleteContextService">
          <MenuIconDelete class="menu-icon" />
          <span>删除</span>
          <small>卸载工具、中间件或数据库并清理卡片</small>
        </button>
        <button v-if="componentContextMenu.kind === 'capability'" type="button" @click="deleteContextCapability">
          <MenuIconDelete class="menu-icon" />
          <span>删除</span>
          <small>移除共享或外部资源卡片</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component' || componentContextMenu.kind === 'service'" type="button" @click="openContextNodeMonitoring">
          <MenuIconMonitor class="menu-icon" />
          <span>监控</span>
          <small>打开监控中心</small>
        </button>
        <button v-if="componentContextMenu.kind === 'component' || componentContextMenu.kind === 'service'" type="button" @click="openContextNodeLogs">
          <MenuIconLogs class="menu-icon" />
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
          <component :is="submenuTemplateIcon(tmpl)" class="menu-icon" />
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
      <div v-if="showServiceModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeServiceInstallModal">
        <div class="modal-container" :style="{ maxWidth: serviceModalMode === 'tool' ? '720px' : '600px' }">
          <div class="modal-header">
            <div>
              <p class="modal-label">{{ serviceModalMode === 'infra' ? '安装中间件' : '安装工具' }}</p>
              <p class="modal-heading">{{ serviceModalMode === 'infra' ? '选择要安装的中间件' : '选择要安装的工具' }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="closeServiceInstallModal">
              <svg focusable="false" width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-content">
            <p v-if="serviceModalLoading" class="bx--type-body-short-01 no-data">服务加载中...</p>
            <p v-else-if="serviceModalError" class="bx--type-body-short-01 no-data error-text">{{ serviceModalError }}</p>
            <p v-else-if="serviceModalNotice" class="bx--type-body-short-01 no-data">{{ serviceModalNotice }}</p>
            <div v-else class="service-picker-summary">
              <span class="summary-pill">{{ serviceModalMode === 'infra' ? '中间件' : '工具' }}</span>
              <span class="summary-text">先选择使用方式，再选择服务产品；不可用路径不会进入安装表单。</span>
            </div>
            <div v-if="!serviceModalLoading && !serviceModalError" class="service-provision-mode-grid" aria-label="服务使用方式">
              <button
                v-for="mode in serviceProvisionModeOptions"
                :key="mode.key"
                type="button"
                class="service-provision-mode-card"
                :class="{ selected: serviceProvisionMode === mode.key, disabled: !mode.enabled }"
                :disabled="!mode.enabled"
                @click="selectServiceProvisionMode(mode.key)"
              >
                <strong>{{ mode.label }}</strong>
                <small>{{ mode.description }}</small>
              </button>
            </div>
            <div v-if="!serviceModalLoading && !serviceModalError" class="service-picker-summary">
              <span class="summary-pill">{{ serviceProvisionModeLabel }}</span>
              <span class="summary-text">可选 {{ selectableServiceCount }} 个，已添加、已安装或正在安装的服务会显示为不可选状态。</span>
            </div>
            <div class="service-select-grid">
              <div v-for="svc in visibleServiceOptions" :key="svc.type"
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
            <div v-if="serviceProvisionMode === 'shared' && serviceForm.serviceType" class="shared-service-choice">
              <div class="config-section-title">
                <span>选择平台公共服务</span>
                <small>只引用共享资源池实例，不修改共享服务本体。</small>
              </div>
              <div v-if="matchingSharedResources.length" class="service-select-grid service-select-grid--compact">
                <div
                  v-for="resource in matchingSharedResources"
                  :key="resource.id || resource.capabilityKey || resource.serviceName"
                  :class="['service-select-card', { selected: selectedSharedResourceId === String(resource.id) }]"
                  @click="selectedSharedResourceId = String(resource.id)"
                >
                  <div class="select-radio" :class="{ selected: selectedSharedResourceId === String(resource.id) }"></div>
                  <div>
                    <div class="service-name-row">
                      <h4 class="bx--type-productive-heading-02 service-name">{{ resource.serviceName || resource.serviceType }}</h4>
                      <span class="bx--tag bx--tag--sm bx--tag--blue">{{ resource.status || 'shared' }}</span>
                    </div>
                    <p class="bx--type-body-short-01 service-desc">{{ [resource.namespace, resource.provider].filter(Boolean).join(' · ') || '平台共享资源' }}</p>
                  </div>
                </div>
              </div>
              <p v-else class="bx--type-body-short-01 no-data">共享资源池中没有匹配的 {{ svcLabel(serviceForm.serviceType) }} 实例。</p>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="bx--btn bx--btn--secondary" @click="closeServiceInstallModal">取消</button>
            <button type="button" class="bx--btn bx--btn--primary" :disabled="serviceSubmitDisabled" @click="submitService">{{ serviceSubmitting ? '处理中...' : serviceSubmitLabel }}</button>
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
              <p class="modal-label">{{ pendingDeleteDialog.actionLabel || `删除${pendingDeleteDialog.label}` }}</p>
              <p class="modal-heading">确认{{ pendingDeleteDialog.actionLabel || '删除' }} {{ pendingDeleteDialog.name }}</p>
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
              {{ pendingDeleteDialog.submitting ? '处理中...' : `确认${pendingDeleteDialog.actionLabel || '删除'}` }}
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
import ComponentConfigTemplateFields from '../components/ComponentConfigTemplateFields.vue'
import { validateWorkspaceActionParams, type ServiceWorkspace, type WorkspaceAction, type WorkspaceActionField, type WorkspaceResource } from './serviceWorkspace'
import { renderPaapTemplateValue, templatePlaceholderDefault } from './configTemplateRenderer'
import {
  componentConfigTemplateMatchesComponent,
  componentConfigTemplateRecommendationScore,
  componentConfigTemplateMatchesSelection,
  componentConfigTemplateSelectValue,
  componentTemplateFieldDefaultValue,
  componentTemplateCredentialPasswordKeys,
  componentTemplateCredentialUsernameKeys,
  componentTemplateDefaultCredentialUsername,
  componentTemplateExistingFieldValue,
  componentTemplateFieldKey,
  componentTemplateFieldLabel,
  componentTemplateFieldMatchesServiceRef,
  componentTemplateFieldRequired,
  componentTemplateFieldTargetTokens,
  componentTemplateFieldType,
  componentTemplateListItemFields,
  componentTemplateInitialFieldValue as templateInitialFieldValue,
  componentTemplateRenderTargetValue,
  componentTemplateRequiredFieldsComplete as templateRequiredFieldsComplete,
  componentTemplateServicePasswordFieldKeys,
  componentTemplateServiceTypeMatchesTargets,
  componentTemplateServiceUsernameFieldKeys,
  componentTemplateSplitEndpoint,
  resolveComponentConfigTemplateSelection,
} from './componentConfigTemplateRuntime'
import {
  connectionBindingPreview,
  serviceConfigFieldVisible,
  serviceConfigFormFromInstallation,
  serviceConfigProfile,
  serviceConfigType,
  serviceInternalEndpoint,
  serviceTopologyFromWorkspace,
  serviceConfigValues,
  serviceConfigValuesFromForm,
  type ServiceConfigField,
  type ServiceConfigForm,
} from './serviceAssetConfig'
import { numericRouteParam, routeEnvironmentKey } from './envDetailRouteState'
import { shouldPollTemplateInstallations, TEMPLATE_INSTALL_POLL_INTERVAL_MS, TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS } from './envInstallPolling'
import { runtimeConsoleWebSocketProtocols } from './runtimeConsoleAuth'
import { buildPickerTemplates, createPickerSessionState, pickerNotice } from './envDetailServicePicker'
import { buildEnvironmentCapabilityTabs, capabilityServiceInstanceLabel, knownCapabilityTabByKey, knownCapabilityTabKeys, requiredEnvironmentCapabilities, serviceCapability as resolveServiceCapability, serviceCategory as resolveServiceCategory, type CapabilityCategory, type CapabilityTab } from './envCapabilities'
import { effectiveEnvironmentStatus, environmentStatusLabel } from './appSummary'
import {
  hasExplicitNonLatestImageTag,
  imageRefFromRegistryFields,
  imageTagForImageField,
  imageTagVersion,
  registryHostForImageField,
  registryRepositorySuffix,
  splitImageRepositoryAndTag,
} from './componentImageConfig'
import {
  mergeComponentBinding,
  nginxRouteRowsToTemplateListRows,
  nginxRouteRowsFromComponentConfig,
  nginxTemplateListFieldSupportsRoutes,
  nginxTemplateListRowsToRouteRows,
  type NginxRouteRow,
} from './componentNginxRoutes'
import {
  componentTemplateFileMountPath,
  mergeComponentConfigFile,
  normalizeComponentTemplateFiles,
  type ComponentConfigFileRow,
  type ComponentConfigTemplateFileRow,
} from './componentConfigTemplateFiles'
import { mergeCreatedCanvasResource, selectCreatedCanvasResource } from './envDetailCanvasResources'
import {
  buildComponentDependencyEdges,
  buildComponentTopologyNodes,
  buildComponentTopologyZones,
  componentTopologyCanvasSizeWithSavedBounds,
  componentTopologyCanvasViewBox,
  componentTopologyEdgePath,
  componentTopologyUnionBounds,
  componentTopologyZoneKey,
  expandComponentTopologyZoneBounds,
  findTopologyNodeAtPoint,
  hasComponentTopologyDragMoved,
  isNodeInMarquee,
  nextComponentTopologyDragPosition,
  nextComponentTopologyZoneResizeBounds,
  nodeKey,
  parseComponentTopologyDisplayNames,
  parseComponentTopologyManualEdges,
  parseComponentTopologyPositions,
  removeComponentTopologyManualEdge,
  serializeComponentTopologyDisplayNames,
  serializeComponentTopologyManualEdges,
  serializeComponentTopologyPositions,
} from './componentTopology'
import {
  buildComponentProfile,
  componentDrawerBlueprint,
  componentConfigKeySuggestions,
  componentFrameworkLabel,
  componentFrameworkOptions,
} from './componentProfile'
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
import MenuIconComponent from '@carbon/icons-vue/es/cube/16'
import MenuIconTool from '@carbon/icons-vue/es/tool-box/16'
import MenuIconMiddleware from '@carbon/icons-vue/es/datastore/16'
import MenuIconShared from '@carbon/icons-vue/es/share/16'
import MenuIconExternal from '@carbon/icons-vue/es/plug/16'
import MenuIconAdopt from '@carbon/icons-vue/es/connect/16'
import MenuIconConfigure from '@carbon/icons-vue/es/settings/16'
import MenuIconRename from '@carbon/icons-vue/es/edit/16'
import MenuIconDeploy from '@carbon/icons-vue/es/rocket/16'
import MenuIconDelete from '@carbon/icons-vue/es/trash-can/16'
import MenuIconMonitor from '@carbon/icons-vue/es/chart--line/16'
import MenuIconLogs from '@carbon/icons-vue/es/list/16'
import SubmenuIconFrontend from '@carbon/icons-vue/es/application/16'
import SubmenuIconBackend from '@carbon/icons-vue/es/api/16'
import SubmenuIconGit from '@carbon/icons-vue/es/branch/16'
import SubmenuIconRegistry from '@carbon/icons-vue/es/container-registry/16'
import SubmenuIconCI from '@carbon/icons-vue/es/continuous-integration/16'
import SubmenuIconCD from '@carbon/icons-vue/es/continuous-deployment/16'
import SubmenuIconObjectStorage from '@carbon/icons-vue/es/object-storage/16'
import SubmenuIconMQ from '@carbon/icons-vue/es/message-queue/16'
import SubmenuIconCache from '@carbon/icons-vue/es/flash/16'
import SubmenuIconStatus from '@carbon/icons-vue/es/circle-dash/16'
import SubmenuIconFallback from '@carbon/icons-vue/es/information/16'

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
const environmentCapabilities = ref<any[]>([])
const sharedCapabilityResources = ref<any[]>([])
const isSystemSharedEnvironment = computed(() =>
  Boolean(env.value?.isSystem) ||
  (String(app.value?.identifier || '').toLowerCase() === 'default' && String(env.value?.identifier || '').toLowerCase() === 'shared')
)

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
  buildModule: '',
  dockerfilePath: '',
})
const compForm = ref(defaultComponentForm())
const componentModalError = ref('')
const pageError = ref('')
const pendingUninstallService = ref<any>(null)
const uninstallError = ref('')
const uninstallSubmitting = ref(false)
const serviceForm = ref({ serviceType:'deploy' })
type ServiceProvisionMode = 'managed' | 'shared' | 'external' | 'kubevirt'
const serviceProvisionMode = ref<ServiceProvisionMode>('managed')
const selectedSharedResourceId = ref('')
const externalCapabilityOptions = [
  { capability: 'git', label: '代码仓库', serviceType: 'git', provider: 'gitlab', externalPlaceholder: 'https://gitlab.example.com' },
  { capability: 'registry', label: '镜像仓库', serviceType: 'registry', provider: 'harbor', externalPlaceholder: 'https://harbor.example.com' },
  { capability: 'ci', label: '持续集成', serviceType: 'ci', provider: 'jenkins', externalPlaceholder: 'https://jenkins.example.com' },
  { capability: 'cd', label: '持续部署', serviceType: 'deploy', provider: 'argocd', externalPlaceholder: 'https://argocd.example.com' },
  { capability: 'database', label: '数据库', serviceType: 'database', provider: 'postgresql', externalPlaceholder: 'postgres://user:password@db.example.com:5432/app' },
  { capability: 'cache', label: '缓存', serviceType: 'cache', provider: 'redis', externalPlaceholder: 'redis://redis.example.com:6379/0' },
  { capability: 'mq', label: '消息中间件', serviceType: 'mq', provider: 'kafka', externalPlaceholder: 'kafka://kafka.example.com:9092' },
  { capability: 'objectStorage', label: '对象存储', serviceType: 'object-storage', provider: 's3', externalPlaceholder: 'https://s3.example.com' },
  { capability: 'monitor', label: '监控', serviceType: 'monitor', provider: 'prometheus', externalPlaceholder: 'https://prometheus.example.com' },
  { capability: 'logging', label: '日志', serviceType: 'log', provider: 'loki', externalPlaceholder: 'https://loki.example.com' },
  { capability: 'custom', label: '自定义外部资源', serviceType: 'custom', provider: 'custom', externalPlaceholder: 'https://service.example.com' },
]
const serviceProvisionModes: Array<{ key: ServiceProvisionMode; label: string; description: string }> = [
  { key: 'managed', label: '环境内创建', description: '使用服务产品在当前环境安装一份独立实例。' },
  { key: 'shared', label: '使用平台公共服务', description: '引用共享资源池中的实例，不修改共享服务本体。' },
  { key: 'external', label: '接入外部连接', description: '保存外部 endpoint 和凭据引用，PAAP 不拥有外部系统。' },
  { key: 'kubevirt', label: 'KubeVirt 模板交付', description: '通过虚拟机模板交付具体数据库或缓存服务。' },
]
const parseServiceFeatures = (raw: unknown) => {
  const fallback = [
    { key: 'managed', enabled: true },
    { key: 'shared', enabled: true },
  ]
  if (Array.isArray(raw)) {
    return raw
      .map((item:any) => ({ key: String(item?.key || '').trim(), enabled: item?.enabled !== false }))
      .filter(item => item.key)
  }
  if (typeof raw === 'string' && raw.trim()) {
    try {
      return parseServiceFeatures(JSON.parse(raw))
    } catch {
      return fallback
    }
  }
  return fallback
}
const serviceSupportsProvisionMode = (svc:any, mode: ServiceProvisionMode) => {
  if (mode === 'managed') return true
  const features = parseServiceFeatures(svc?.features)
  return features.some(item => item.key === mode && item.enabled)
}
const externalCapabilityForService = (svc:any) => {
  const type = String(svc?.type || svc?.serviceType || '').toLowerCase()
  const mapping: Record<string, { capability: string; provider: string; serviceType: string }> = {
    git: { capability: 'git', provider: 'gitlab', serviceType: 'git' },
    gitea: { capability: 'git', provider: 'gitea', serviceType: 'git' },
    registry: { capability: 'registry', provider: 'registry', serviceType: 'registry' },
    harbor: { capability: 'registry', provider: 'harbor', serviceType: 'harbor' },
    ci: { capability: 'ci', provider: 'jenkins', serviceType: 'ci' },
    jenkins: { capability: 'ci', provider: 'jenkins', serviceType: 'ci' },
    deploy: { capability: 'cd', provider: 'argocd', serviceType: 'deploy' },
    argocd: { capability: 'cd', provider: 'argocd', serviceType: 'deploy' },
    postgresql: { capability: 'database', provider: 'postgresql', serviceType: 'postgresql' },
    mysql: { capability: 'database', provider: 'mysql', serviceType: 'mysql' },
    mongodb: { capability: 'database', provider: 'mongodb', serviceType: 'mongodb' },
    redis: { capability: 'cache', provider: 'redis', serviceType: 'redis' },
    rabbitmq: { capability: 'mq', provider: 'rabbitmq', serviceType: 'rabbitmq' },
    kafka: { capability: 'mq', provider: 'kafka', serviceType: 'kafka' },
    minio: { capability: 'objectStorage', provider: 'minio', serviceType: 'minio' },
    monitor: { capability: 'monitor', provider: 'prometheus', serviceType: 'monitor' },
    log: { capability: 'logging', provider: 'loki', serviceType: 'log' },
    loki: { capability: 'logging', provider: 'loki', serviceType: 'log' },
  }
  const mapped = mapping[type]
  if (!mapped) return null
  const base = externalCapabilityOptions.find(item => item.capability === mapped.capability) || externalCapabilityOptions[externalCapabilityOptions.length - 1]
  return { ...base, ...mapped, label: base.label }
}
const defaultCapabilityForm = () => ({
  externalEndpoint: '',
  authType: 'none',
  username: '',
  password: '',
  token: '',
  credentialSecretRef: '',
  tlsInsecureSkipVerify: false,
})
const capabilityForm = ref(defaultCapabilityForm())
const capabilitySecretVisibleKeys = ref<Set<string>>(new Set())
const capabilityCredentialLoading = ref(false)
const capabilityCredentialError = ref('')
const capabilityValidationLoading = ref(false)
const sharedCapabilitySecretVisibleKeys = ref<Set<string>>(new Set())
const sharedCapabilityCredentialLoading = ref(false)
const sharedCapabilityCredentialError = ref('')
const sharedCapabilityCredentialCapability = ref('')
const sharedCapabilityCredentials = ref<any[]>([])
const activeCapabilityServiceId = ref<number | null>(null)
const capabilityWorkspaceCache = ref<Record<number, ServiceWorkspace>>({})
const capabilityWorkspaceLoading = ref(false)
const capabilityWorkspaceError = ref('')
const capabilityWorkspaceMessage = ref('')
const capabilityInlineInstallLoading = ref(false)
const capabilityInlineInstallError = ref('')
const capabilityInlineInstallingType = ref('')
const capabilityInitialSubjectKey = ref('')
const activeCapabilityAction = ref<WorkspaceAction | null>(null)
const activeCapabilityActionTarget = ref<string | undefined>(undefined)
const activeCapabilityActionParams = ref<Record<string, string>>({})
const activeWorkspaceActionServiceId = ref<number | null>(null)
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
const selectedManualEdge = ref<{ fromKey: string; toKey: string } | null>(null)
const coreToolsSection = ref<HTMLElement | null>(null)
const componentActionLoading = ref(false)
const componentContextMenu = ref<{ visible: boolean; x: number; y: number; kind: 'component' | 'service' | 'capability' | 'canvas' | 'edge'; component: any | null; service: any | null; capability: any | null; edge: any | null }>({
  visible: false,
  x: 0,
  y: 0,
  kind: 'component',
  component: null,
  service: null,
  capability: null,
  edge: null,
})
const contextSubmenu = ref<{ visible: boolean; x: number; y: number; mode: 'component' | 'tool' | 'infra' | 'shared-capability' | 'external-capability'; templates: any[] }>({
  visible: false,
  x: 0,
  y: 0,
  mode: 'tool',
  templates: [],
})
type ComponentConfigEnvRow = {
  name: string
  source: 'value' | 'configMap' | 'secret'
  value: string
  refName: string
  refKey: string
  managed?: boolean
  managedLabel?: string
}
type ComponentConfigObjectRow = { name: string; data: Record<string, string> }
type ServiceVolumeField = {
  label: string
  description: string
  enabledKey: string
  sizeKey: string
  placeholder: string
}
type UserComponentConfigTemplate = {
  id: string | number
  key?: string
  name: string
  description: string
  framework: string
  bindingMode: string
  componentTypes: string[]
  fields: any[]
  syntax: string
  env: ComponentConfigEnvRow[]
  configMaps: ComponentConfigObjectRow[]
  secrets: ComponentConfigObjectRow[]
  files: ComponentConfigTemplateFileRow[]
  command: string[]
  args: string[]
  isBuiltin?: boolean
}
const normalizeTemplateConfigObjectRows = (items:any[]): ComponentConfigObjectRow[] => Array.isArray(items)
  ? items.map((item:any) => ({
    name: String(item?.name || '').trim(),
    data: Object.fromEntries(Object.entries(item?.data || {}).map(([key, value]) => [String(key).trim(), String(value ?? '')]).filter(([key]) => key)),
  })).filter((item:ComponentConfigObjectRow) => Object.keys(item.data).length)
  : []
const normalizeTemplateConfigFiles = normalizeComponentTemplateFiles
type TopologyCanvasPosition = { x: number; y: number; width?: number; height?: number }
const componentNodePositions = ref<Record<string, TopologyCanvasPosition>>({})
const manualCanvasEdges = ref<Array<{ fromKey: string; toKey: string }>>([])
const componentDisplayNames = ref<Record<string, string>>({})
const renamingNodeKey = ref<string | null>(null)
const renamingNodeValue = ref('')
const canvasCreatePoint = ref<{ x: number; y: number } | null>(null)
type CanvasPositionScope = 'component' | 'environment'
const canvasCreateScope = ref<CanvasPositionScope>('component')
type TopologyDragBounds = { minX?: number; minY?: number; maxX?: number; maxY?: number }
const componentNodeDrag = ref<{ keys: string[]; positionKeys: Record<string, string>; origins: Record<string, { x: number; y: number }>; bounds?: Record<string, TopologyDragBounds>; stageEl: HTMLElement | null; startX: number; startY: number; moved: boolean; lastX: number; lastY: number } | null>(null)
type TopologyZoneDragEdge = 'left' | 'right' | 'top' | 'bottom'
const topologyZoneResizeHandles: { key: string; label: string; edges: TopologyZoneDragEdge[] }[] = [
  { key: 'top', label: '上边', edges: ['top'] },
  { key: 'right', label: '右边', edges: ['right'] },
  { key: 'bottom', label: '下边', edges: ['bottom'] },
  { key: 'left', label: '左边', edges: ['left'] },
  { key: 'top-left', label: '左上角', edges: ['top', 'left'] },
  { key: 'top-right', label: '右上角', edges: ['top', 'right'] },
  { key: 'bottom-right', label: '右下角', edges: ['bottom', 'right'] },
  { key: 'bottom-left', label: '左下角', edges: ['bottom', 'left'] },
]
const topologyZoneDrag = ref<{ key: string; mode: 'move' | 'resize'; edges: TopologyZoneDragEdge[]; origins: Record<string, { x: number; y: number }>; originBounds: { left: number; top: number; width: number; height: number }; stageEl: HTMLElement; startX: number; startY: number; moved: boolean } | null>(null)
const connectionDrag = ref<{ fromNode: any; startX: number; startY: number; currentX: number; currentY: number; stageEl: HTMLElement } | null>(null)
const suppressNextTopologyClick = ref(false)
const suppressTopologyClickKeys = ref<string[]>([])
const recentTopologyDrag = ref<{ key: string; at: number } | null>(null)
const selectedNodeKeys = ref<string[]>([])
const marqueeRect = ref<{ x: number; y: number; width: number; height: number } | null>(null)
const marqueeDrag = ref<{ startCanvasX: number; startCanvasY: number; currentCanvasX: number; currentCanvasY: number; stageEl: HTMLElement } | null>(null)
const defaultConfigForm = () => ({
  framework: 'auto',
  deliveryMode: 'image' as 'image' | 'source',
  image: '',
  registryTargetKey: '',
  repository: '',
  imageTag: '',
  sourceRepoUrl: '',
  sourceBranch: 'main',
  buildContext: '.',
  buildModule: '',
  dockerfilePath: '',
  version: '',
  replicas: 1,
  containerPort: 0,
  containerPortSource: 'default' as 'saved' | 'detected' | 'default' | 'user',
  cpu: '',
  memory: '',
  env: [] as ComponentConfigEnvRow[],
  configMaps: [] as ComponentConfigObjectRow[],
  secrets: [] as ComponentConfigObjectRow[],
  files: [] as ComponentConfigFileRow[],
  bindings: [] as Array<{ targetKey: string; targetKind: string; targetName: string; targetType: string; role: string; mode: string; confidence: string; source: string; generated: Record<string, string> }>,
  bindingTargetKey: '',
  bindingMode: 'recommended',
  commandText: '',
  argsText: '',
})
const configForm = ref(defaultConfigForm())
const normalizeUserComponentConfigTemplate = (raw:any): UserComponentConfigTemplate | null => {
  const name = String(raw?.name || '').trim()
  if (!name) return null
  const env = Array.isArray(raw?.env)
    ? raw.env.map((item:any) => ({
      name: String(item?.name || '').trim(),
      source: ['configMap', 'secret'].includes(String(item?.source || '')) ? String(item.source) as 'configMap' | 'secret' : 'value',
      value: String(item?.value || ''),
      refName: String(item?.refName || ''),
      refKey: String(item?.refKey || ''),
    })).filter((item:ComponentConfigEnvRow) => item.name)
    : []
  return {
    id: String(raw?.id || `component-template-${Date.now().toString(36)}`),
    key: String(raw?.key || '').trim(),
    name,
    description: String(raw?.description || '').trim(),
    framework: String(raw?.framework || 'auto'),
    bindingMode: String(raw?.bindingMode || 'recommended'),
    componentTypes: Array.isArray(raw?.componentTypes) ? raw.componentTypes.map((item:any) => String(item).trim()).filter(Boolean) : [],
    fields: Array.isArray(raw?.fields) ? raw.fields : [],
    syntax: String(raw?.syntax || ''),
    env,
    configMaps: normalizeTemplateConfigObjectRows(raw?.configMaps || []),
    secrets: normalizeTemplateConfigObjectRows(raw?.secrets || []),
    files: normalizeTemplateConfigFiles(raw?.files || []),
    command: Array.isArray(raw?.command) ? raw.command.map((item:any) => String(item).trim()).filter(Boolean) : [],
    args: Array.isArray(raw?.args) ? raw.args.map((item:any) => String(item).trim()).filter(Boolean) : [],
    isBuiltin: Boolean(raw?.isBuiltin),
  }
}
const loadUserComponentConfigTemplates = async () => {
  componentConfigTemplatesLoading.value = true
  try {
    const res = await api.listComponentConfigTemplates()
    const parsed = res.data || []
    componentUserConfigTemplates.value = Array.isArray(parsed)
      ? parsed.map(normalizeUserComponentConfigTemplate).filter(Boolean) as UserComponentConfigTemplate[]
      : []
  } catch (e:any) {
    configDrawer.value.error = configDrawer.value.visible ? `加载配置模板失败：${e?.message || '未知错误'}` : configDrawer.value.error
    componentUserConfigTemplates.value = []
  } finally {
    componentConfigTemplatesLoading.value = false
  }
}
const componentConfigTemplatesLoading = ref(false)
const componentUserConfigTemplates = ref<UserComponentConfigTemplate[]>([])
const selectedComponentConfigTemplateId = ref('')
const componentTemplateFieldValues = ref<Record<string, any>>({})
const nginxRouteRows = ref<NginxRouteRow[]>([])
const defaultServiceConfigForm = (): ServiceConfigForm => ({})
const serviceConfigForm = ref<ServiceConfigForm>(defaultServiceConfigForm())
const componentDrawerRole = ref('custom')
const serviceDrawerWorkspaceLoading = ref(false)
const serviceExternalAccessLoading = ref(false)
const componentExternalAccessLoading = ref(false)
const componentNodePortLoading = ref(false)
const serviceDrawerRevealedSecrets = ref<Set<string>>(new Set())
const serviceDrawerSecretValues = ref<Record<string, string>>({})
const serviceDrawerSecretLoadingKey = ref('')
const runtimeMetrics = ref<any | null>(null)
const runtimeMetricsLoading = ref(false)
const runtimeMetricsError = ref('')
const runtimeLogs = ref<any | null>(null)
const runtimeLogsLoading = ref(false)
const runtimeLogsError = ref('')
const runtimeConsoleSocket = ref<WebSocket | null>(null)
const runtimeConsoleConnected = ref(false)
const runtimeConsoleConnecting = ref(false)
const runtimeConsoleError = ref('')
const runtimeConsoleView = ref<HTMLElement | null>(null)
let runtimeConsoleTerm: import('@xterm/xterm').Terminal | null = null
let runtimeConsoleFitAddon: import('@xterm/addon-fit').FitAddon | null = null
let runtimeConsoleResizeObserver: ResizeObserver | null = null
type ConfigDrawerTab = 'deploy' | 'workspace' | 'capabilities' | 'api' | 'dependencies' | 'database' | 'data' | 'queues' | 'buckets' | 'backups' | 'variables' | 'runtime' | 'logs' | 'console'
const configDrawerTab = ref<ConfigDrawerTab>('deploy')
const configDrawer = ref<{ visible: boolean; kind: 'component' | 'service' | 'capability'; component: any | null; service: any | null; capability: any | null; saving: boolean; error: string; message: string }>({
  visible: false,
  kind: 'component',
  component: null,
  service: null,
  capability: null,
  saving: false,
  error: '',
  message: '',
})
const configDrawerTabs = computed<Array<{ key: ConfigDrawerTab; label: string }>>(() => {
  if (configDrawer.value.kind === 'capability') {
    return [{ key: 'deploy', label: '配置' }]
  }
  if (configDrawer.value.kind === 'component') {
    return componentDrawerBlueprintModel.value.tabs as Array<{ key: ConfigDrawerTab; label: string }>
  }
  const kind = serviceDrawerProfile.value.kind
  if (kind === 'database') {
    return [
      { key: 'deploy', label: '部署' },
      { key: 'database', label: '数据库' },
      { key: 'backups', label: '备份' },
	      { key: 'variables', label: '接入' },
	      { key: 'runtime', label: '指标' },
	      { key: 'logs', label: '日志' },
	      { key: 'console', label: '控制台' },
    ]
  }
  if (kind === 'redis') {
    return [
      { key: 'deploy', label: '部署' },
      { key: 'data', label: '数据' },
	      { key: 'variables', label: '接入' },
	      { key: 'runtime', label: '指标' },
	      { key: 'logs', label: '日志' },
	      { key: 'console', label: '控制台' },
    ]
  }
  if (kind === 'message-queue') {
    return [
      { key: 'deploy', label: '部署' },
      { key: 'queues', label: serviceDrawerType.value === 'kafka' ? '主题' : '队列' },
	      { key: 'variables', label: '接入' },
	      { key: 'runtime', label: '指标' },
	      { key: 'logs', label: '日志' },
	      { key: 'console', label: '控制台' },
    ]
  }
  if (kind === 'object-storage') {
    return [
      { key: 'deploy', label: '部署' },
      { key: 'buckets', label: '存储桶' },
	      { key: 'variables', label: '接入' },
	      { key: 'runtime', label: '指标' },
	      { key: 'logs', label: '日志' },
	      { key: 'console', label: '控制台' },
    ]
  }
  return [
    { key: 'deploy', label: '部署' },
    { key: 'workspace', label: serviceDrawerWorkspaceTabLabel.value },
	    { key: 'variables', label: '接入' },
	    { key: 'runtime', label: '指标' },
	    { key: 'logs', label: '日志' },
	    { key: 'console', label: '控制台' },
  ]
})
const registryWorkspaceLoading = ref(false)
const registryWorkspaceError = ref('')
type RegistryRepositoryOption = { repository: string; tags: string[]; resource: WorkspaceResource }
type RegistryTargetOption = { key: string; label: string; source: 'managed' | 'shared' | 'external'; host: string; service?: any; capability?: any; workspace?: ServiceWorkspace | null }
type PendingDeleteDialog = {
  kind: 'component' | 'capability'
  label: string
  actionLabel?: string
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
    return tab
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
const isEditableKeyboardTarget = (target: EventTarget | null) => {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = String(el.tagName || '').toLowerCase()
  return el.isContentEditable || tag === 'input' || tag === 'textarea' || tag === 'select'
}
const handleDocumentKeyDown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    closeComponentContextMenu()
    selectedManualEdge.value = null
    return
  }
  if ((event.key === 'Delete' || event.key === 'Backspace') && !isEditableKeyboardTarget(event.target)) {
    if (pendingDeleteDialog.value || pendingUninstallService.value || showComponentModal.value || showServiceModal.value || showRelationshipModal.value || showAdoptResourceModal.value) return
    if (deleteSelectedCanvasItem()) {
      event.preventDefault()
      event.stopPropagation()
    }
  }
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
  selectedManualEdge.value = null
  componentPanelTab.value = 'topology'
  closeComponentContextMenu()
  capabilityWorkspaceLoadSeq++
}

let environmentLoadSeq = 0
let capabilityWorkspaceLoadSeq = 0
let templateInstallPollTimer: number | null = null
let templateInstallPollAttempts = 0
let componentStatusPollTimer: number | null = null
let componentStatusPollAttempts = 0
let componentStatusPollComponentId: number | null = null

const stopTemplateInstallPolling = () => {
  if (templateInstallPollTimer) window.clearTimeout(templateInstallPollTimer)
  templateInstallPollTimer = null
  templateInstallPollAttempts = 0
}

const componentNeedsStatusPolling = (componentId: number) => {
  const comp = components.value.find((item:any) => Number(item.id) === Number(componentId))
  return ['syncing', 'deploying', 'building', 'pending'].includes(String(comp?.status || '').toLowerCase())
}

const stopComponentStatusPolling = () => {
  if (componentStatusPollTimer) window.clearTimeout(componentStatusPollTimer)
  componentStatusPollTimer = null
  componentStatusPollAttempts = 0
  componentStatusPollComponentId = null
}

const scheduleComponentStatusPolling = (componentId: number) => {
  if (!componentId) return
  if (componentStatusPollComponentId !== componentId) {
    if (componentStatusPollTimer) window.clearTimeout(componentStatusPollTimer)
    componentStatusPollTimer = null
    componentStatusPollAttempts = 0
    componentStatusPollComponentId = componentId
  }
  if (componentStatusPollTimer || componentStatusPollAttempts >= TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS) return
  componentStatusPollAttempts += 1
  componentStatusPollTimer = window.setTimeout(async () => {
    componentStatusPollTimer = null
    try {
      await refreshServices()
    } finally {
      if (componentNeedsStatusPolling(componentId)) {
        scheduleComponentStatusPolling(componentId)
      } else {
        stopComponentStatusPolling()
      }
    }
  }, TEMPLATE_INSTALL_POLL_INTERVAL_MS)
}

const scheduleTemplateInstallPolling = () => {
  if (templateInstallPollTimer || !shouldPollTemplateInstallations(env.value, services.value, components.value)) return
  if (templateInstallPollAttempts >= TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS) return
  templateInstallPollAttempts += 1
  templateInstallPollTimer = window.setTimeout(async () => {
    templateInstallPollTimer = null
    try {
      await refreshServices()
    } finally {
      if (shouldPollTemplateInstallations(env.value, services.value, components.value)) {
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
  stopComponentStatusPolling()
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
    await Promise.all([loadEnvironmentCapabilities(), loadSharedCapabilityResources()])
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
  await Promise.all([loadUserComponentConfigTemplates(), loadEnvironmentDetail()])
})

watch(envRouteKey, () => {
  void loadEnvironmentDetail()
})

onBeforeUnmount(() => {
  stopTemplateInstallPolling()
  stopComponentStatusPolling()
  disconnectDrawerConsole()
  destroyRuntimeConsoleTerm()
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
const serviceStatusText = (status?: string) => ({ running: '运行中', linked: '已关联', installing: '安装中', syncing: '同步中', deploying: '部署中', building: '构建中', failed: '安装失败', deleting: '删除中', pending: '未部署', error: '安装失败', draft: '未部署' }[String(status || '').toLowerCase()] || '未知')
const statusTagClass = (status?: string) => ({ running: 'bx--tag--green', linked: 'bx--tag--green', installing: 'bx--tag--blue', syncing: 'bx--tag--blue', deploying: 'bx--tag--blue', building: 'bx--tag--blue', failed: 'bx--tag--red', error: 'bx--tag--red', deleting: 'bx--tag--gray', pending: 'bx--tag--gray', draft: 'bx--tag--gray' }[String(status || '').toLowerCase()] || 'bx--tag--gray')
const serviceStatusValue = (svc:any) => String(svc?.status || '').toLowerCase()
const serviceStatusIsDraft = (svc:any) => ['draft', 'pending', ''].includes(serviceStatusValue(svc))
const serviceStatusHasRuntime = (svc:any) => ['running', 'installing'].includes(serviceStatusValue(svc))
const serviceStatusCanDeploy = (svc:any) => !['installing', 'deleting'].includes(serviceStatusValue(svc))
const typeLabel = (type:string) => {
  const labels: Record<string,string> = { deploy:'部署与持续交付', monitor:'监控与可观测性', log:'日志服务', ci:'持续集成', git:'代码仓库', registry:'轻量镜像仓库', harbor:'企业镜像仓库', postgresql:'关系型数据库', mysql:'关系型数据库', mongodb:'文档数据库', redis:'缓存服务', rabbitmq:'消息队列', kafka:'消息队列', minio:'对象存储', infra:'中间件', tool:'平台工具', custom:'自定义工具' }
  return labels[type] || type
}
const capabilityLabel = (capability:string) => ({
  git: '代码仓库',
  registry: '镜像仓库',
  cd: '持续部署',
  ci: '持续集成',
  monitor: '监控',
  logging: '日志',
  database: '数据库',
  cache: '缓存',
  mq: '消息队列',
  objectStorage: '对象存储',
  custom: '自定义资源',
}[capability] || capability)
const capabilitySourceLabel = (source:string) => ({
  managed: '本环境',
  shared: '共享资源',
  external: '外部资源',
  deferred: '稍后配置',
}[source] || source)
const capabilityRemovalActionLabel = (cap:any) => {
  if (cap?.source === 'shared') return '断开引用'
  if (cap?.source === 'external') return '断开外部连接'
  return '删除卡片'
}
const capabilityRemovalMessage = (cap:any) => {
  if (cap?.source === 'external') {
    return '断开后只会移除当前环境中的外部资源连接记录和本地凭据，不会删除外部系统。'
  }
  if (cap?.source === 'shared') {
    return '断开后只会移除当前环境对共享资源的引用，不会删除共享资源池中的服务。'
  }
  return '删除后只会移除当前环境中的资源卡片。'
}
const topologySourceBadge = (node:any): { label: string; tone: string } | null => {
  if (!node) return null
  if (node.topologyKind === 'capability') {
    if (node.source === 'shared') return { label: '平台共享', tone: 'shared' }
    if (node.source === 'external') return { label: '外部资源', tone: 'external' }
    if (node.source === 'managed') return { label: '环境内资源', tone: 'managed' }
    return { label: capabilitySourceLabel(node.source || 'deferred'), tone: 'deferred' }
  }
  if (node.topologyKind === 'service') return { label: '环境内资源', tone: 'managed' }
  if (node.topologyKind === 'component') return { label: '应用组件', tone: 'component' }
  return null
}
const capabilityDisplayName = (cap:any) => {
  if (cap?.source === 'shared' && cap?.refService) return `${capabilityLabel(cap.capability)} · ${cap.refService.serviceName || cap.refService.serviceType}`
  if (cap?.source === 'external' && cap?.externalEndpoint) return `${capabilityLabel(cap.capability)} · 外部`
  return `${capabilityLabel(cap?.capability || '')} · ${capabilitySourceLabel(cap?.source || '')}`
}
const capabilityRequestKey = (cap:any) => String(cap?.capabilityKey || cap?.capability || '').trim()
const capabilityNodeStatus = (cap:any) => {
	if (cap?.source === 'shared') return 'linked'
	if (cap?.source === 'external') return cap?.validationStatus || (cap?.externalEndpoint ? 'pending' : 'draft')
	return cap?.validationStatus || 'draft'
}
const compTypeText = (type?:string) => ({ frontend:'前端服务', backend:'后端服务', database:'数据库', middleware:'中间件', custom:'自定义' }[type || ''] || type || 'custom')
const componentIsSourceDelivery = (comp:any) => comp?.deliveryMode === 'source' || Boolean(comp?.sourceRepoUrl || comp?.sourceMirrorRepoUrl || comp?.jenkinsJob)
const componentDeliveryModeLabel = (comp:any) => componentIsSourceDelivery(comp) ? '源码交付' : '镜像交付'
const componentDeliveryTarget = (comp:any) => comp?.registryImage || comp?.image || comp?.sourceRepoUrl || '-'
const isApplicationTopologyService = (svc:any) => {
  const type = String(serviceProductKey(svc) || svc?.serviceType || svc?.type || '').toLowerCase()
  return ['postgresql', 'mysql', 'mongodb', 'redis', 'rabbitmq', 'kafka', 'minio'].includes(type)
}
const appCanvasServices = computed(() => services.value.filter(isApplicationTopologyService))
const environmentCapabilityNodes = computed(() => environmentCapabilities.value.map((cap:any) => {
  const key = `capability:${cap.id || cap.capability}`
  const displayName = componentDisplayNames.value[key] || capabilityDisplayName(cap)
  return {
    ...cap,
    id: cap.id || cap.capability,
    name: displayName,
    type: `capability-${cap.source || 'external'}`,
    status: capabilityNodeStatus(cap),
    topologyKind: 'capability',
    topologyId: key,
  }
}))
const componentTopologyAllNodes = computed(() => [
  ...buildComponentTopologyNodes(components.value, appCanvasServices.value, componentDisplayNames.value),
  ...environmentCapabilityNodes.value,
])
const environmentTopologyAllNodes = computed(() => [
  ...buildComponentTopologyNodes(components.value, services.value, componentDisplayNames.value),
  ...environmentCapabilityNodes.value,
])
const componentTopologyEdges = computed(() => buildComponentDependencyEdges(componentTopologyAllNodes.value))
const environmentTopologyEdges = computed(() => buildComponentDependencyEdges(environmentTopologyAllNodes.value))
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
const environmentTopologyNodeSubtitle = (node:any) => {
  if (node?.topologyKind === 'capability') return `${capabilitySourceLabel(node.source)} · ${node.externalEndpoint || node.refService?.namespace || node.credentialSecretRef || '待配置'}`
  if (node?.topologyKind === 'service') return `${typeLabel(node.type || node.serviceType || '')}`
  return `${compTypeText(node.type)} · 应用组件`
}
const componentCanvasMetrics = {
  colWidth: 260,
  nodeWidth: 196,
  nodeHeight: 70,
  top: 48,
  left: 72,
  rowGap: 34,
}
const topologyZoneMetrics = {
  paddingX: 12,
  paddingTop: 24,
  paddingBottom: 12,
  canvasEdgePadding: 16,
  minWidth: 360,
  minHeight: 220,
  collapsedWidth: 180,
  collapsedHeight: 44,
}
const topologyZonePaddingOptions = () => ({
  paddingX: topologyZoneMetrics.paddingX,
  paddingTop: topologyZoneMetrics.paddingTop,
  paddingBottom: topologyZoneMetrics.paddingBottom,
  minLeft: 12,
  minTop: 16,
})
const topologyNodeKey = (node:any) => String(node?.topologyId || node?.id || node?.name || '')
const scopedCanvasPositionKey = (scope: CanvasPositionScope, key: string) => `${scope}:${key}`
const nodeCanvasPositionKey = (scope: CanvasPositionScope, node:any) => scopedCanvasPositionKey(scope, topologyNodeKey(node))
const canvasScopeForStage = (stageEl: HTMLElement | null): CanvasPositionScope =>
  stageEl?.closest('.environment-topology-canvas') ? 'environment' : 'component'
const deleteCanvasPositionKeys = (positions: Record<string, TopologyCanvasPosition>, key: string) => {
  delete positions[key]
  delete positions[scopedCanvasPositionKey('component', key)]
  delete positions[scopedCanvasPositionKey('environment', key)]
}
const savedNodeCanvasPosition = (scope: CanvasPositionScope, node:any) => {
  const key = topologyNodeKey(node)
  if (!key) return null
  const scoped = componentNodePositions.value[scopedCanvasPositionKey(scope, key)]
  if (scoped) return scoped
  if (scope === 'component') return componentNodePositions.value[key] || null
  return null
}
const preferredTopologyDepth = (node:any) => {
  if (node?.topologyKind === 'capability') return 3
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
  const maxAutoDepth = Math.max(1, nodes.length + 2)
  for (let i = 0; i < queue.length; i++) {
    const key = queue[i]
    const nextDepth = Math.min((depth.get(key) || 0) + 1, maxAutoDepth)
    for (const next of outgoing.get(key) || []) {
      if ((depth.get(next) ?? -1) < nextDepth) {
        depth.set(next, nextDepth)
        if (nextDepth < maxAutoDepth) queue.push(next)
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
  componentDisplayNames.value = {}
  if (!envId.value) return
  const res = await api.getEnvironmentCanvasState(envId.value)
  componentNodePositions.value = parseComponentTopologyPositions(canvasStateRawJSON(res?.data?.positions, {}))
  manualCanvasEdges.value = parseComponentTopologyManualEdges(canvasStateRawJSON(res?.data?.edges, []))
  componentDisplayNames.value = parseComponentTopologyDisplayNames(canvasStateRawJSON(res?.data?.names, {}))
}
const saveComponentNodePositions = async () => {
  if (!envId.value) return
  await api.saveEnvironmentCanvasState(envId.value, {
    positions: JSON.parse(serializeComponentTopologyPositions(componentNodePositions.value)),
    edges: JSON.parse(serializeComponentTopologyManualEdges(manualCanvasEdges.value)),
    names: JSON.parse(serializeComponentTopologyDisplayNames(componentDisplayNames.value)),
  })
}
const canvasLayoutSaveErrorPrefix = '保存画布布局失败：'
const clearCanvasLayoutSaveError = () => {
  if (String(pageError.value || '').startsWith(canvasLayoutSaveErrorPrefix)) pageError.value = ''
}
const saveCanvasLayoutAfterDrag = () =>
  saveComponentNodePositions()
    .then(clearCanvasLayoutSaveError)
    .catch((e:any) => {
      pageError.value = canvasLayoutSaveErrorPrefix + (e?.message || '未知错误')
    })
const graphLayout = computed(() => {
  return layoutTopologyGraph(filteredTopologyNodes.value, componentTopologyEdges.value)
})
const environmentGraphLayout = computed(() => layoutTopologyGraph(environmentTopologyAllNodes.value, environmentTopologyEdges.value))
const componentCanvasSize = computed(() => {
  const maxColumnNodes = Math.max(1, ...Array.from(graphLayout.value.buckets.values()).map(items => items.length))
  return {
    width: Math.max(1280, componentCanvasMetrics.left * 2 + (graphLayout.value.maxDepth + 1) * componentCanvasMetrics.colWidth + 220),
    height: Math.max(720, componentCanvasMetrics.top + maxColumnNodes * (componentCanvasMetrics.nodeHeight + componentCanvasMetrics.rowGap) + 220),
  }
})
const environmentCanvasSize = computed(() => {
  const maxColumnNodes = Math.max(1, ...Array.from(environmentGraphLayout.value.buckets.values()).map(items => items.length))
  let savedRight = 0
  let savedBottom = 0
  for (const [key, pos] of Object.entries(componentNodePositions.value)) {
    if (!key.startsWith('environment:')) continue
    const x = Number(pos?.x || 0)
    const y = Number(pos?.y || 0)
    const width = Number(pos?.width || componentCanvasMetrics.nodeWidth)
    const height = Number(pos?.height || componentCanvasMetrics.nodeHeight)
    savedRight = Math.max(savedRight, x + width)
    savedBottom = Math.max(savedBottom, y + height)
  }
  return componentTopologyCanvasSizeWithSavedBounds({
    width: Math.max(1280, componentCanvasMetrics.left * 2 + (environmentGraphLayout.value.maxDepth + 1) * componentCanvasMetrics.colWidth + 220),
    height: Math.max(680, componentCanvasMetrics.top + maxColumnNodes * (componentCanvasMetrics.nodeHeight + componentCanvasMetrics.rowGap) + 180),
  }, { right: savedRight, bottom: savedBottom }, topologyZoneMetrics.canvasEdgePadding)
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
      x: componentNodePosition('component', node, depth, nodeIndex).x,
      y: componentNodePosition('component', node, depth, nodeIndex).y,
      width: componentCanvasMetrics.nodeWidth,
      height: componentCanvasMetrics.nodeHeight,
    }))
  )
)
const environmentCanvasAllNodes = computed(() =>
  Array.from(environmentGraphLayout.value.buckets.entries()).flatMap(([depth, items]) =>
    items.map((node:any, nodeIndex) => ({
      ...node,
      x: componentNodePosition('environment', node, depth, nodeIndex).x,
      y: componentNodePosition('environment', node, depth, nodeIndex).y,
      width: componentCanvasMetrics.nodeWidth,
      height: componentCanvasMetrics.nodeHeight,
    }))
  )
)
const collapsedTopologyZones = ref<Set<string>>(new Set())
const isTopologyZoneCollapsed = (zoneKey:string) => collapsedTopologyZones.value.has(zoneKey)
const toggleTopologyZone = (zoneKey:string) => {
  const next = new Set(collapsedTopologyZones.value)
  if (next.has(zoneKey)) next.delete(zoneKey)
  else next.add(zoneKey)
  collapsedTopologyZones.value = next
}
const topologyZoneToggleTitle = (zone:any) => {
  const action = isTopologyZoneCollapsed(zone.key) ? '展开' : '折叠'
  const label = zone.key === 'environment' ? '本环境' : zone.label
  return `${action}${label}分区`
}
const topologyZoneResizeHandleTitle = (zone:any, handle:any) => `调整${zone?.label || '分区'}${handle?.label || '边框'}`
const environmentCanvasVisibleNodes = computed(() =>
  environmentCanvasAllNodes.value.filter((node:any) => !isTopologyZoneCollapsed(componentTopologyZoneKey(node)))
)
const environmentCanvasNodes = environmentCanvasVisibleNodes
const legacyTopologyZonePositionKey = (zoneKey:string) => `zone:${zoneKey}`
const topologyZonePositionKey = (zoneKey:string) => scopedCanvasPositionKey('environment', legacyTopologyZonePositionKey(zoneKey))
const topologyZoneContentMinimum = (bounds:any, nodes:any[]) => {
  const left = Number(bounds?.left || 0)
  const top = Number(bounds?.top || 0)
  const right = Math.max(...nodes.map((node:any) => Number(node.x || 0) + Number(node.width || componentCanvasMetrics.nodeWidth)), left)
  const bottom = Math.max(...nodes.map((node:any) => Number(node.y || 0) + Number(node.height || componentCanvasMetrics.nodeHeight)), top)
  return {
    width: Math.max(0, right - left + topologyZoneMetrics.paddingX),
    height: Math.max(0, bottom - top + topologyZoneMetrics.paddingBottom),
  }
}
const topologyZoneBounds = (zoneKey:string, nodes:any[], canvasSize:any) => {
  const canvasWidth = Number(canvasSize.width || 0)
  const canvasHeight = Number(canvasSize.height || 0)
  const contentLeft = Math.max(16, Math.min(...nodes.map((node:any) => Number(node.x || 0))) - topologyZoneMetrics.paddingX)
  const contentTop = Math.max(16, Math.min(...nodes.map((node:any) => Number(node.y || 0))) - topologyZoneMetrics.paddingTop)
  const contentRight = Math.min(
    canvasWidth - 16,
    Math.max(...nodes.map((node:any) => Number(node.x || 0) + Number(node.width || componentCanvasMetrics.nodeWidth))) + topologyZoneMetrics.paddingX,
  )
  const contentBottom = Math.min(
    canvasHeight - 16,
    Math.max(...nodes.map((node:any) => Number(node.y || 0) + Number(node.height || componentCanvasMetrics.nodeHeight))) + topologyZoneMetrics.paddingBottom,
  )
  const width = Math.max(0, contentRight - contentLeft)
  const height = Math.max(0, contentBottom - contentTop)
  const saved = componentNodePositions.value[topologyZonePositionKey(zoneKey)]
    || componentNodePositions.value[legacyTopologyZonePositionKey(zoneKey)]
  const left = saved ? Number(saved.x || 0) : contentLeft
  const top = saved ? Number(saved.y || 0) : contentTop
  const savedWidth = Number(saved?.width || 0)
  const savedHeight = Number(saved?.height || 0)
  const union = componentTopologyUnionBounds(
    { left: Math.max(16, left), top: Math.max(16, top), width: Number.isFinite(savedWidth) ? savedWidth : 0, height: Number.isFinite(savedHeight) ? savedHeight : 0 },
    { left: contentLeft, top: contentTop, width, height },
  )
  const minForContent = topologyZoneContentMinimum({ left: union.left, top: union.top }, nodes)
  return {
    left: union.left,
    top: union.top,
    width: Math.max(union.width, minForContent.width),
    height: Math.max(union.height, minForContent.height),
  }
}
const collapsedTopologyZoneBounds = (bounds:any) => ({
  left: Number(bounds.left || 0),
  top: Number(bounds.top || 0),
  width: Math.min(Number(bounds.width || topologyZoneMetrics.collapsedWidth), topologyZoneMetrics.collapsedWidth),
  height: topologyZoneMetrics.collapsedHeight,
})
const ensureTopologyZonePosition = (zone:any) => {
  const key = topologyZonePositionKey(zone.key)
  if (componentNodePositions.value[key]) return
  componentNodePositions.value = {
    ...componentNodePositions.value,
    [key]: {
      x: Number(zone.expandedBounds?.left || zone.bounds?.left || 16),
      y: Number(zone.expandedBounds?.top || zone.bounds?.top || 16),
      width: Number(zone.expandedBounds?.width || zone.bounds?.width || topologyZoneMetrics.minWidth),
      height: Number(zone.expandedBounds?.height || zone.bounds?.height || topologyZoneMetrics.minHeight),
    },
  }
}
const environmentCanvasZones = computed(() =>
  buildComponentTopologyZones(environmentCanvasAllNodes.value).map((zone:any) => {
    const expandedBounds = topologyZoneBounds(zone.key, zone.nodes, environmentCanvasSize.value)
    const collapsed = isTopologyZoneCollapsed(zone.key)
    return {
      ...zone,
      collapsed,
      expandedBounds,
      bounds: collapsed ? collapsedTopologyZoneBounds(expandedBounds) : expandedBounds,
    }
  })
)
const componentNodePosition = (scope: CanvasPositionScope, node:any, laneIndex: number, nodeIndex: number) => {
  const saved = savedNodeCanvasPosition(scope, node)
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
      source: edge.source || 'auto',
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
        source: 'manual',
        fromNode,
        toNode,
      }
    })
    .filter(Boolean)
  return [...explicitEdges, ...manualEdges]
}
const componentCanvasEdges = computed(() => canvasEdgesForNodes(componentCanvasNodes.value, componentTopologyEdges.value))
const environmentCanvasEdges = computed(() => canvasEdgesForNodes(environmentCanvasNodes.value, environmentTopologyEdges.value))
const isManualCanvasEdge = (edge:any) => edge.source === 'manual'
const componentManualCanvasEdges = computed(() => componentCanvasEdges.value.filter(isManualCanvasEdge))
const environmentManualCanvasEdges = computed(() => environmentCanvasEdges.value.filter(isManualCanvasEdge))
const componentNodeStyle = (node:any) => ({
  left: `${node.x}px`,
  top: `${node.y}px`,
  width: `${node.width}px`,
  height: `${node.height}px`,
})
const componentTopologyZoneStyle = (zone:any) => ({
  left: `${zone.bounds.left}px`,
  top: `${zone.bounds.top}px`,
  width: `${zone.bounds.width}px`,
  height: `${zone.bounds.height}px`,
})
const clampDragBounds = (bounds: TopologyDragBounds): TopologyDragBounds => ({
  ...bounds,
  maxX: Number.isFinite(bounds.maxX) && Number.isFinite(bounds.minX) ? Math.max(Number(bounds.minX), Number(bounds.maxX)) : bounds.maxX,
  maxY: Number.isFinite(bounds.maxY) && Number.isFinite(bounds.minY) ? Math.max(Number(bounds.minY), Number(bounds.maxY)) : bounds.maxY,
})
const topologyNodeDragBounds = (stageEl: HTMLElement | null, node:any): TopologyDragBounds => {
  const stageWidth = Number(stageEl?.offsetWidth || 0)
  const stageHeight = Number(stageEl?.offsetHeight || 0)
  const nodeWidth = Number(node?.width || componentCanvasMetrics.nodeWidth)
  const nodeHeight = Number(node?.height || componentCanvasMetrics.nodeHeight)
  if (stageEl?.closest('.environment-topology-canvas')) {
    return clampDragBounds({
      minX: 12,
      minY: 46,
    })
  }
  return clampDragBounds({
    minX: 12,
    minY: 46,
    maxX: stageWidth ? stageWidth - nodeWidth - 16 : undefined,
    maxY: stageHeight ? stageHeight - nodeHeight - 16 : undefined,
  })
}
const componentNodeIconClass = (node:any) => {
  if (node?.topologyKind === 'capability') return `node-type-icon--capability node-type-icon--capability-${String(node.source || 'external').toLowerCase()}`
  if (node?.topologyKind === 'service') return `node-type-icon--${String(node.type || node.serviceType || 'service').toLowerCase()}`
  return `node-type-icon--${String(node.type || 'component').toLowerCase()}`
}
const componentNodeIconPath = (node:any) => {
  if (node?.topologyKind === 'capability') return 'M12 2 3 7v10l9 5 9-5V7l-9-5zm0 2.2L18.8 8 12 11.8 5.2 8 12 4.2zM5 10.5l6 3.3v5.6l-6-3.3v-5.6zm14 0v5.6l-6 3.3v-5.6l6-3.3z'
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
const manualEdgeKey = (edge:any) => `${String(edge?.fromKey || '').trim()}->${String(edge?.toKey || '').trim()}`
const selectedManualEdgeKey = computed(() => selectedManualEdge.value ? manualEdgeKey(selectedManualEdge.value) : '')
const manualEdgeSelected = (edge:any) => isManualCanvasEdge(edge) && selectedManualEdgeKey.value === manualEdgeKey(edge)
const componentEdgeClasses = (edge:any) => ({
  active: selectedManualEdge.value ? manualEdgeSelected(edge) : componentEdgeHighlighted(edge),
  'component-canvas-link--manual': isManualCanvasEdge(edge),
  'component-canvas-link--selected': manualEdgeSelected(edge),
})
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
  const emptyConfig = {
    framework: '',
    configTemplateId: 0,
    configTemplateKey: '',
    configTemplateName: '',
    configTemplate: null as any,
    registryTarget: null as any,
    env: [] as any[],
    configMaps: [] as any[],
    secrets: [] as any[],
    files: [] as any[],
    bindings: [] as any[],
    dependencies: [] as string[],
    command: [] as string[],
    args: [] as string[],
  }
  if (!raw) return emptyConfig
  if (typeof raw === 'object') return {
    framework: String(raw.framework || ''),
    configTemplateId: Number(raw.configTemplateId || raw.configTemplate?.id || 0),
    configTemplateKey: String(raw.configTemplateKey || raw.configTemplate?.key || ''),
    configTemplateName: String(raw.configTemplateName || raw.configTemplate?.name || ''),
    configTemplate: raw.configTemplate || null,
    registryTarget: raw.registryTarget || null,
    env: Array.isArray(raw.env) ? raw.env : [],
    configMaps: Array.isArray(raw.configMaps) ? raw.configMaps : [],
    secrets: Array.isArray(raw.secrets) ? raw.secrets : [],
    files: Array.isArray(raw.files) ? raw.files : [],
    bindings: Array.isArray(raw.bindings) ? raw.bindings : [],
    dependencies: Array.isArray(raw.dependencies) ? raw.dependencies.map((item:any) => String(item).trim()).filter(Boolean) : [],
    command: Array.isArray(raw.command) ? raw.command.map((item:any) => String(item).trim()).filter(Boolean) : [],
    args: Array.isArray(raw.args) ? raw.args.map((item:any) => String(item).trim()).filter(Boolean) : [],
  }
  try {
    const parsed = JSON.parse(String(raw))
    return {
      framework: String(parsed?.framework || ''),
      configTemplateId: Number(parsed?.configTemplateId || parsed?.configTemplate?.id || 0),
      configTemplateKey: String(parsed?.configTemplateKey || parsed?.configTemplate?.key || ''),
      configTemplateName: String(parsed?.configTemplateName || parsed?.configTemplate?.name || ''),
      configTemplate: parsed?.configTemplate || null,
      registryTarget: parsed?.registryTarget || null,
      env: Array.isArray(parsed?.env) ? parsed.env : [],
      configMaps: Array.isArray(parsed?.configMaps) ? parsed.configMaps : [],
      secrets: Array.isArray(parsed?.secrets) ? parsed.secrets : [],
      files: Array.isArray(parsed?.files) ? parsed.files : [],
      bindings: Array.isArray(parsed?.bindings) ? parsed.bindings : [],
      dependencies: Array.isArray(parsed?.dependencies) ? parsed.dependencies.map((item:any) => String(item).trim()).filter(Boolean) : [],
      command: Array.isArray(parsed?.command) ? parsed.command.map((item:any) => String(item).trim()).filter(Boolean) : [],
      args: Array.isArray(parsed?.args) ? parsed.args.map((item:any) => String(item).trim()).filter(Boolean) : [],
    }
  } catch {
    return emptyConfig
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
    selectedManualEdge.value = null
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
  ? `${typeLabel(node.type || 'infra')}`
  : `${compTypeText(node.type)} · ${componentDeliveryModeLabel(node)}`
const selectTopologyNode = (node:any) => {
  if (!node) return
  selectedTopologyKey.value = String(node.topologyId || node.id || '')
  if (node.topologyKind === 'capability') {
    openCapabilityConfigDrawer(node)
    return
  }
  if (node.topologyKind === 'service') {
    openServiceConfigDrawer(node)
    return
  }
  selectComponent(node.id)
  openComponentConfigDrawer(node, 'variables')
}
const handleTopologyNodeClick = (event: MouseEvent, node:any) => {
  if (renamingNodeKey.value) return
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
  selectedManualEdge.value = null
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
const closeCanvasMenus = () => {
  componentContextMenu.value = { visible: false, x: 0, y: 0, kind: 'component', component: null, service: null, capability: null, edge: null }
  contextSubmenu.value = { visible: false, x: 0, y: 0, mode: 'tool', templates: [] }
}
const closeCanvasDialogs = () => {
  showComponentModal.value = false
  componentModalError.value = ''
  if (!serviceSubmitting.value) {
    showServiceModal.value = false
    serviceModalError.value = ''
    serviceModalNotice.value = ''
  }
  if (!uninstallSubmitting.value) {
    pendingUninstallService.value = null
    uninstallError.value = ''
  }
  if (!pendingDeleteDialog.value?.submitting) pendingDeleteDialog.value = null
  if (!relationshipSubmitting.value) {
    showRelationshipModal.value = false
    relationshipSourceComponent.value = null
    relationshipSelectedKeys.value = []
    relationshipError.value = ''
  }
  if (!adoptResourceSubmitting.value) {
    showAdoptResourceModal.value = false
    adoptResourceError.value = ''
    adoptResourceSelection.value = ''
  }
}
const closeComponentContextMenu = closeCanvasMenus
const enterDrawerContext = () => {
  closeCanvasMenus()
  closeCanvasDialogs()
}
const enterCanvasContextMenu = (event: MouseEvent, _kind: 'canvas' | 'component' | 'service' = 'canvas') => {
  event.preventDefault()
  closeConfigDrawer()
  closeCanvasDialogs()
  closeCanvasMenus()
}
const enterModalContext = () => {
  closeCanvasMenus()
  closeConfigDrawer()
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
const openToolSubmenu = async () => {
  if (templates.value.length === 0) {
    contextSubmenu.value = {
      visible: true,
      x: componentContextMenu.value.x + 230,
      y: componentContextMenu.value.y + 40,
      mode: 'tool',
      templates: [{ type: 'loading', label: '服务加载中...', description: '正在读取服务目录', disabled: true }],
    }
    try {
      await loadServiceTemplates()
    } catch (e:any) {
      contextSubmenu.value = {
        visible: true,
        x: componentContextMenu.value.x + 230,
        y: componentContextMenu.value.y + 40,
        mode: 'tool',
        templates: [{ type: 'error', label: '服务加载失败', description: e?.message || '请稍后重试', disabled: true }],
      }
      return
    }
  }
  const toolTemplates = buildPickerTemplates(templates.value, services.value, 'tool')
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y + 40,
    mode: 'tool',
    templates: toolTemplates.map((t:any) => ({ ...t, label: t.serviceName || t.name || t.type })),
  }
}
const openInfraSubmenu = async () => {
  if (templates.value.length === 0) {
    contextSubmenu.value = {
      visible: true,
      x: componentContextMenu.value.x + 230,
      y: componentContextMenu.value.y + 80,
      mode: 'infra',
      templates: [{ type: 'loading', label: '服务加载中...', description: '正在读取服务目录', disabled: true }],
    }
    try {
      await loadServiceTemplates()
    } catch (e:any) {
      contextSubmenu.value = {
        visible: true,
        x: componentContextMenu.value.x + 230,
        y: componentContextMenu.value.y + 80,
        mode: 'infra',
        templates: [{ type: 'error', label: '服务加载失败', description: e?.message || '请稍后重试', disabled: true }],
      }
      return
    }
  }
  const infraTemplates = buildPickerTemplates(templates.value, services.value, 'infra')
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y + 80,
    mode: 'infra',
    templates: infraTemplates.map((t:any) => ({ ...t, label: t.serviceName || t.name || t.type })),
  }
}
const sharedCapabilityTemplateLabel = (item:any) => [capabilityLabel(item.capability), item.serviceName || item.serviceType].filter(Boolean).join(' · ')
const externalCapabilityMenuDescription = (item:any) => {
  const labels: Record<string, string> = {
    git: '接入外部 Git 或代码托管服务',
    registry: '接入外部镜像仓库',
    ci: '接入外部持续集成服务',
    cd: '接入外部持续部署服务',
    database: '接入外部数据库服务',
    cache: '接入外部缓存服务',
    mq: '接入外部消息中间件',
    objectStorage: '接入外部对象存储',
    monitor: '接入外部监控服务',
    logging: '接入外部日志服务',
    custom: '接入自定义外部资源',
  }
  return labels[String(item?.capability || '')] || '接入外部资源'
}
const openSharedCapabilitySubmenu = async () => {
  await loadSharedCapabilityResources()
  const templates = sharedCapabilityResources.value.map((item:any) => ({
    ...item,
    type: String(item.capability || item.serviceType || item.id),
    label: sharedCapabilityTemplateLabel(item),
    description: [item.namespace, item.status].filter(Boolean).join(' · ') || '共享资源',
  }))
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y + 120,
    mode: 'shared-capability',
    templates: templates.length ? templates : [{ type: 'none', label: '暂无共享资源', description: '平台管理员需要先在共享资源池创建资源', disabled: true }],
  }
}
const openExternalCapabilitySubmenu = () => {
  contextSubmenu.value = {
    visible: true,
    x: componentContextMenu.value.x + 230,
    y: componentContextMenu.value.y + 160,
    mode: 'external-capability',
    templates: externalCapabilityOptions.map((item) => ({
      ...item,
      type: item.capability,
      description: externalCapabilityMenuDescription(item),
    })),
  }
}
const submenuTemplateIcon = (tmpl: any) => {
  const raw = String(tmpl?.type || tmpl?.serviceType || tmpl?.capability || '').toLowerCase()
  if (raw === 'loading' || raw === 'error' || raw === 'none') return SubmenuIconStatus
  const has = (...keys: string[]) => keys.some((k) => raw.includes(k))
  if (has('frontend', 'web', 'application')) return SubmenuIconFrontend
  if (has('backend', 'api', 'custom', 'service')) return SubmenuIconBackend
  if (has('git', 'code', 'repository')) return SubmenuIconGit
  if (has('registry', 'harbor', 'image')) return SubmenuIconRegistry
  if (has('ci', 'continuous-integration', 'jenkins', 'build')) return SubmenuIconCI
  if (has('cd', 'continuous-deployment', 'deploy', 'argocd')) return SubmenuIconCD
  if (has('monitor', 'prometheus', 'metric')) return MenuIconMonitor
  if (has('log', 'loki')) return MenuIconLogs
  if (has('cache', 'redis')) return SubmenuIconCache
  if (has('mq', 'message', 'queue', 'kafka', 'rabbit')) return SubmenuIconMQ
  if (has('object', 'storage', 'minio', 's3')) return SubmenuIconObjectStorage
  if (has('database', 'databases', 'postgres', 'mysql', 'mongo', 'galera')) return MenuIconMiddleware
  return SubmenuIconFallback
}
const selectSubmenuTemplate = async (tmpl: any) => {
  if (tmpl.disabled) return
  if (contextSubmenu.value.mode === 'component') {
    await createCanvasComponentDraft(tmpl.type)
    return
  }
  if (contextSubmenu.value.mode === 'shared-capability') {
    await createSharedCapabilityReference(tmpl)
    return
  }
  if (contextSubmenu.value.mode === 'external-capability') {
    await createExternalCapabilityDraft(tmpl)
    return
  }
  closeComponentContextMenu()
  await createCanvasServiceDraft(tmpl.type)
}
const openTopologyContextMenu = (event: MouseEvent, node: any) => {
  if (node?.topologyKind === 'capability') {
    openCapabilityContextMenu(event, node)
    return
  }
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
  enterCanvasContextMenu(event, 'canvas')
  selectedTopologyKey.value = null
  selectedManualEdge.value = null
  const stageEl = (event.target as HTMLElement | null)?.closest?.('.component-canvas-stage') as HTMLElement | null
  canvasCreatePoint.value = stageEl ? canvasPointFromEvent(event, stageEl) : null
  canvasCreateScope.value = canvasScopeForStage(stageEl)
  const pos = contextMenuPosition(event, 220, 288)
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'canvas',
    component: null,
    service: null,
    capability: null,
    edge: null,
  }
}
const openComponentContextMenu = (event: MouseEvent, comp: any) => {
  enterCanvasContextMenu(event, 'component')
  selectedManualEdge.value = null
  selectComponent(comp?.id)
  const pos = contextMenuPosition(event)
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'component',
    component: comp,
    service: null,
    capability: null,
    edge: null,
  }
}
const openServiceContextMenu = (event: MouseEvent, svc: any) => {
  enterCanvasContextMenu(event, 'service')
  selectedManualEdge.value = null
  const pos = contextMenuPosition(event, 220, 200)
  selectedTopologyKey.value = String(svc?.topologyId || svc?.id || '')
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'service',
    component: null,
    service: svc,
    capability: null,
    edge: null,
  }
}
const openCapabilityContextMenu = (event: MouseEvent, capability: any) => {
  enterCanvasContextMenu(event, 'component')
  selectedManualEdge.value = null
  const pos = contextMenuPosition(event, 220, 164)
  selectedTopologyKey.value = String(capability?.topologyId || '')
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'capability',
    component: null,
    service: null,
    capability,
    edge: null,
  }
}
const selectManualCanvasEdge = (edge:any) => {
  if (!isManualCanvasEdge(edge)) return
  closeComponentContextMenu()
  selectedTopologyKey.value = null
  selectedComponentId.value = null
  selectedNodeKeys.value = []
  selectedManualEdge.value = { fromKey: String(edge.fromKey || '').trim(), toKey: String(edge.toKey || '').trim() }
}
const openManualEdgeContextMenu = (event: MouseEvent, edge:any) => {
  if (!isManualCanvasEdge(edge)) return
  event.preventDefault()
  closeConfigDrawer()
  closeCanvasDialogs()
  closeCanvasMenus()
  selectManualCanvasEdge(edge)
  const pos = contextMenuPosition(event, 220, 92)
  componentContextMenu.value = {
    visible: true,
    x: pos.x,
    y: pos.y,
    kind: 'edge',
    component: null,
    service: null,
    capability: null,
    edge,
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
  const key = String(node?.topologyId || node?.id || node?.name || '')
  closeComponentContextMenu()
  selectedManualEdge.value = null
  if (!key) return
  suppressNextTopologyClick.value = false
  suppressTopologyClickKeys.value = []
  const isGroupDrag = selectedNodeKeys.value.includes(key) && selectedNodeKeys.value.length > 1
  const keys = isGroupDrag ? [...selectedNodeKeys.value] : [key]
  const origins: Record<string, { x: number; y: number }> = {}
  const bounds: Record<string, TopologyDragBounds> = {}
  const positionKeys: Record<string, string> = {}
  const stageEl = (event.currentTarget as HTMLElement | null)?.closest('.component-canvas-stage') as HTMLElement | null
  const scope = canvasScopeForStage(stageEl)
  const currentCanvasNodes = canvasNodesForStage(stageEl)
  if (stageEl?.closest('.environment-topology-canvas')) {
    for (const draggedKey of keys) {
      const draggedNode = currentCanvasNodes.find((n: any) => String(n.topologyId || n.id) === draggedKey)
      if (!draggedNode) continue
      const zone = environmentCanvasZones.value.find((item:any) => item.key === componentTopologyZoneKey(draggedNode))
      if (zone) ensureTopologyZonePosition(zone)
    }
  }
  if (isGroupDrag) {
    for (const k of keys) {
      const n = currentCanvasNodes.find((n: any) => String(n.topologyId || n.id) === k)
      if (n) {
        origins[k] = { x: n.x, y: n.y }
        bounds[k] = topologyNodeDragBounds(stageEl, n)
        positionKeys[k] = nodeCanvasPositionKey(scope, n)
      }
    }
  } else {
    origins[key] = { x: Number(node.x || 0), y: Number(node.y || 0) }
    bounds[key] = topologyNodeDragBounds(stageEl, node)
    positionKeys[key] = nodeCanvasPositionKey(scope, node)
  }
  componentNodeDrag.value = {
    keys,
    positionKeys,
    origins,
    bounds,
    stageEl,
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
const expandedEnvironmentZonePositionUpdates = (positionsByTopologyKey: Record<string, { x: number; y: number }>) => {
  const updates: Record<string, TopologyCanvasPosition> = {}
  const movedNodes = Object.entries(positionsByTopologyKey)
    .map(([key, pos]) => {
      const current = environmentCanvasAllNodes.value.find((node:any) => topologyNodeKey(node) === key)
      return current ? { ...current, x: pos.x, y: pos.y } : null
    })
    .filter(Boolean) as any[]
  for (const zone of environmentCanvasZones.value) {
    const nodes = movedNodes.filter((node:any) => componentTopologyZoneKey(node) === zone.key)
    if (!nodes.length) continue
    const bounds = zone.expandedBounds || zone.bounds
    const expanded = expandComponentTopologyZoneBounds(
      {
        left: Number(bounds?.left || 0),
        top: Number(bounds?.top || 0),
        width: Number(bounds?.width || 0),
        height: Number(bounds?.height || 0),
      },
      nodes,
      topologyZonePaddingOptions(),
    )
    updates[topologyZonePositionKey(zone.key)] = {
      x: expanded.left,
      y: expanded.top,
      width: expanded.width,
      height: expanded.height,
    }
  }
  return updates
}
const componentNodeDragPositionUpdates = (drag: NonNullable<typeof componentNodeDrag.value>, event: PointerEvent) => {
  const byTopologyKey: Record<string, { x: number; y: number }> = {}
  const updates: Record<string, TopologyCanvasPosition> = {}
  for (const k of drag.keys) {
    const origin = drag.origins[k]
    if (!origin) continue
    const bound = drag.bounds?.[k] || {}
    const next = nextComponentTopologyDragPosition({
      originX: origin.x,
      originY: origin.y,
      startX: drag.startX,
      startY: drag.startY,
      currentX: event.clientX,
      currentY: event.clientY,
      zoom: canvasZoom.value,
      minX: bound.minX,
      minY: bound.minY,
      maxX: bound.maxX,
      maxY: bound.maxY,
    })
    byTopologyKey[k] = next
    updates[drag.positionKeys[k] || k] = next
  }
  if (canvasScopeForStage(drag.stageEl) === 'environment') {
    Object.assign(updates, expandedEnvironmentZonePositionUpdates(byTopologyKey))
  }
  return updates
}
const onComponentNodeDrag = (event: PointerEvent) => {
  const drag = componentNodeDrag.value
  if (!drag) return
  drag.lastX = event.clientX
  drag.lastY = event.clientY
  if (hasComponentTopologyDragMoved({ startX: drag.startX, startY: drag.startY, currentX: event.clientX, currentY: event.clientY })) drag.moved = true
  const updated = componentNodeDragPositionUpdates(drag, event)
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
      const updated = componentNodeDragPositionUpdates(drag, event)
      componentNodePositions.value = { ...componentNodePositions.value, ...updated }
    }
  }
  if (drag?.moved) {
    suppressNextTopologyClick.value = true
    suppressTopologyClickKeys.value = [...drag.keys]
    recentTopologyDrag.value = { key: drag.keys[0] || '', at: Date.now() }
    void saveCanvasLayoutAfterDrag()
  }
  componentNodeDrag.value = null
}
const topologyZoneResizeEdges = (event: PointerEvent, target: HTMLElement): TopologyZoneDragEdge[] => {
  const rect = target.getBoundingClientRect()
  const threshold = 8
  const edges: TopologyZoneDragEdge[] = []
  if (event.clientX - rect.left <= threshold) edges.push('left')
  if (rect.right - event.clientX <= threshold) edges.push('right')
  if (event.clientY - rect.top <= threshold) edges.push('top')
  if (rect.bottom - event.clientY <= threshold) edges.push('bottom')
  return edges
}
const topologyZoneContentBoundsFromOrigins = (origins: Record<string, { x: number; y: number }>) => {
  const values = Object.values(origins)
  if (!values.length) return null
  return {
    left: Math.min(...values.map((origin) => origin.x)) - topologyZoneMetrics.paddingX,
    top: Math.min(...values.map((origin) => origin.y)) - topologyZoneMetrics.paddingTop,
    right: Math.max(...values.map((origin) => origin.x + componentCanvasMetrics.nodeWidth)) + topologyZoneMetrics.paddingX,
    bottom: Math.max(...values.map((origin) => origin.y + componentCanvasMetrics.nodeHeight)) + topologyZoneMetrics.paddingBottom,
  }
}
const nextTopologyZoneResizeBounds = (drag: NonNullable<typeof topologyZoneDrag.value>, event: PointerEvent) => {
  const dx = (event.clientX - drag.startX) / canvasZoom.value
  const dy = (event.clientY - drag.startY) / canvasZoom.value
  const content = topologyZoneContentBoundsFromOrigins(drag.origins)
  return nextComponentTopologyZoneResizeBounds({
    originBounds: drag.originBounds,
    contentBounds: content,
    edges: drag.edges,
    dx,
    dy,
    minLeft: 16,
    minTop: 16,
    minWidth: topologyZoneMetrics.minWidth,
    minHeight: topologyZoneMetrics.minHeight,
  })
}
const beginTopologyZoneDrag = (event: PointerEvent, zone:any, mode: 'move' | 'resize', edges: TopologyZoneDragEdge[] = []) => {
  if (event.button !== 0) return
  event.stopPropagation()
  closeComponentContextMenu()
  selectedManualEdge.value = null
  const stageEl = (event.currentTarget as HTMLElement | null)?.closest('.component-canvas-stage') as HTMLElement | null
  if (!stageEl || !zone?.bounds || !Array.isArray(zone.nodes) || !zone.nodes.length) return
  if (mode === 'resize' && (zone.collapsed || !edges.length)) return
  ensureTopologyZonePosition(zone)
  const dragBounds = zone.expandedBounds || zone.bounds
  const origins: Record<string, { x: number; y: number }> = {}
  for (const node of zone.nodes) {
    const key = String(node?.topologyId || node?.id || node?.name || '')
    if (!key) continue
    origins[key] = { x: Number(node.x || 0), y: Number(node.y || 0) }
  }
  if (!Object.keys(origins).length) return
  topologyZoneDrag.value = {
    key: String(zone.key || ''),
    mode,
    edges,
    origins,
    originBounds: {
      left: Number(dragBounds.left || 0),
      top: Number(dragBounds.top || 0),
      width: Number(dragBounds.width || 0),
      height: Number(dragBounds.height || 0),
    },
    stageEl,
    startX: event.clientX,
    startY: event.clientY,
    moved: false,
  }
  try {
    ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
  } catch {
    // setPointerCapture can fail if the target is already detached.
  }
  window.addEventListener('pointermove', onTopologyZoneDrag)
  window.addEventListener('pointerup', stopTopologyZoneDrag, { once: true })
}
const startTopologyZoneDrag = (event: PointerEvent, zone:any) => {
  if ((event.target as HTMLElement)?.closest?.('.component-topology-zone-toggle, .component-topology-zone-resize-handle')) return
  const resizeEdges = topologyZoneResizeEdges(event, event.currentTarget as HTMLElement)
  beginTopologyZoneDrag(event, zone, resizeEdges.length && !zone.collapsed ? 'resize' : 'move', resizeEdges)
}
const startTopologyZoneResize = (event: PointerEvent, zone:any, edges: TopologyZoneDragEdge[]) => {
  beginTopologyZoneDrag(event, zone, 'resize', edges)
}
const onTopologyZoneDrag = (event: PointerEvent) => {
  const drag = topologyZoneDrag.value
  if (!drag) return
  if (hasComponentTopologyDragMoved({ startX: drag.startX, startY: drag.startY, currentX: event.clientX, currentY: event.clientY })) drag.moved = true
  if (drag.mode === 'resize') {
    const nextBounds = nextTopologyZoneResizeBounds(drag, event)
    componentNodePositions.value = {
      ...componentNodePositions.value,
      [topologyZonePositionKey(drag.key)]: {
        x: nextBounds.left,
        y: nextBounds.top,
        width: nextBounds.width,
        height: nextBounds.height,
      },
    }
    return
  }
  const nextBounds = nextComponentTopologyDragPosition({
    originX: drag.originBounds.left,
    originY: drag.originBounds.top,
    startX: drag.startX,
    startY: drag.startY,
    currentX: event.clientX,
    currentY: event.clientY,
    zoom: canvasZoom.value,
    minX: 16,
    minY: 46,
  })
  const dx = nextBounds.x - drag.originBounds.left
  const dy = nextBounds.y - drag.originBounds.top
  const updated: Record<string, TopologyCanvasPosition> = {}
  updated[topologyZonePositionKey(drag.key)] = {
    x: nextBounds.x,
    y: nextBounds.y,
    width: drag.originBounds.width,
    height: drag.originBounds.height,
  }
  for (const [key, origin] of Object.entries(drag.origins)) {
    updated[scopedCanvasPositionKey('environment', key)] = { x: origin.x + dx, y: origin.y + dy }
  }
  componentNodePositions.value = { ...componentNodePositions.value, ...updated }
}
const stopTopologyZoneDrag = (event?: PointerEvent) => {
  window.removeEventListener('pointermove', onTopologyZoneDrag)
  const drag = topologyZoneDrag.value
  if (drag && event) onTopologyZoneDrag(event)
  if (drag?.moved) {
    selectedNodeKeys.value = Object.keys(drag.origins)
    void saveCanvasLayoutAfterDrag()
  }
  topologyZoneDrag.value = null
}
const startCanvasMarquee = (event: PointerEvent) => {
  if (event.button !== 0) return
  if ((event.target as HTMLElement)?.closest?.('.component-topology-node')) return
  if ((event.target as HTMLElement)?.closest?.('.component-topology-zone')) return
  if ((event.target as HTMLElement)?.closest?.('.topology-controls')) return
  closeComponentContextMenu()
  selectedManualEdge.value = null
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
const deleteManualCanvasEdge = async (edge:any) => {
  if (edge?.source && edge.source !== 'manual') return false
  closeComponentContextMenu()
  const fromKey = String(edge.fromKey || '').trim()
  const toKey = String(edge.toKey || '').trim()
  const next = removeComponentTopologyManualEdge(manualCanvasEdges.value, fromKey, toKey)
  if (next.length === manualCanvasEdges.value.length) return false
  manualCanvasEdges.value = next
  selectedManualEdge.value = null
  try {
    await saveComponentNodePositions()
  } catch (e:any) {
    pageError.value = '保存画布连线失败：' + (e?.message || '未知错误')
  }
  return true
}
const deleteContextEdge = () => {
  const edge = componentContextMenu.value.edge || selectedManualEdge.value
  if (edge) void deleteManualCanvasEdge(edge)
}
const topologyNodeByKey = (key:string) => {
  const normalized = String(key || '').trim()
  if (!normalized) return null
  return [...environmentCanvasNodes.value, ...componentCanvasNodes.value]
    .find((node:any) => String(node.topologyId || node.id || node.name || '') === normalized) || null
}
const selectedCanvasNode = () => {
  const key = selectedTopologyKey.value || (selectedNodeKeys.value.length === 1 ? selectedNodeKeys.value[0] : '')
  return topologyNodeByKey(key)
}
const deleteSelectedCanvasItem = () => {
  if (selectedManualEdge.value) {
    void deleteManualCanvasEdge(selectedManualEdge.value)
    return true
  }
  const node = selectedCanvasNode()
  if (!node) return false
  deleteTopologyNode(node)
  return true
}
const componentEdgeHighlighted = (edge: { fromId: number; toId: number }) => {
  const edgeWithKeys = edge as any
  const key = selectedTopologyKey.value || (selectedComponent.value?.id ? `component:${selectedComponent.value.id}` : '')
  if (key) return edgeWithKeys.fromKey === key || edgeWithKeys.toKey === key
  const id = selectedComponent.value?.id
  return Boolean(id && (edge.fromId === id || edge.toId === id))
}
const templateFor = (type:string) => templates.value.find((item:any) => item.type === type)
const serviceTemplateForInstallation = (svc:any) => {
  const templateId = Number(svc?.templateId || svc?.serviceTemplateId || svc?.template?.id || 0)
  if (templateId) {
    const byId = templates.value.find((item:any) => Number(item.id) === templateId)
    if (byId) return byId
  }
  return templateFor(String(svc?.serviceType || svc?.type || '')) || svc?.template || null
}
const serviceChartVersion = (svc:any) => {
  const tmpl = serviceTemplateForInstallation(svc)
  const ver = tmpl?.chartVersion || ''
  return ver.startsWith('v') ? ver.slice(1) : ver
}
const serviceVersionOptionLabel = (tmpl:any) => {
  const app = String(tmpl?.appVersion || '').replace(/^v/, '')
  if (app) return `应用 v${app}`
  return '应用版本未标注'
}
const serviceTypeVersions = computed(() => {
  const svc = drawerService.value
  if (!svc) return []
  const type = svc.serviceType || svc.type || ''
  return templates.value.filter((t:any) => t.type === type && t.chartVersion)
})
const selectedChartVersion = ref('')
const normalizeServiceProductKey = (value:any) => {
  const text = String(value || '').toLowerCase()
  if (!text) return ''
  if (text.includes('argocd') || text.includes('argo-cd')) return 'argocd'
  if (text.includes('jenkins')) return 'jenkins'
  if (text.includes('gitea')) return 'gitea'
  if (text.includes('harbor')) return 'harbor'
  if (text.includes('docker-registry') || text.includes('registry.tar.gz') || text === 'registry' || text.includes('docker registry')) return 'docker-registry'
  if (text.includes('prometheus') || text.includes('grafana') || text.includes('monitor.tar.gz')) return 'prometheus-grafana'
  if (text.includes('loki') || text.includes('promtail') || text.includes('logging')) return 'loki'
  if (text.includes('postgres')) return 'postgresql'
  if (text.includes('mysql')) return 'mysql'
  if (text.includes('mongo')) return 'mongodb'
  if (text.includes('redis')) return 'redis'
  if (text.includes('rabbit')) return 'rabbitmq'
  if (text.includes('kafka')) return 'kafka'
  if (text.includes('minio')) return 'minio'
  return text.replace(/\.tar\.gz$/, '').replace(/^charts\//, '').replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '')
}
const serviceProductKey = (svc:any) => {
  const tmpl = serviceTemplateForInstallation(svc)
  const candidates = [
    svc?.productKey,
    svc?.templateType,
    svc?.templateName,
    svc?.chartName,
    svc?.chart,
    tmpl?.chartName,
    tmpl?.name,
    tmpl?.s3Key,
    tmpl?.type,
    svc?.serviceType,
    svc?.type,
  ]
  for (const candidate of candidates) {
    const key = normalizeServiceProductKey(candidate)
    if (key) return key
  }
  return ''
}
const serviceLogicalType = (svc:any) => String(svc?.serviceType || svc?.type || '').toLowerCase()
const serviceProductLabel = (svc:any) => {
  const tmpl = serviceTemplateForInstallation(svc)
  const key = serviceProductKey(svc)
  const labels: Record<string, string> = {
    argocd: 'ArgoCD',
    jenkins: 'Jenkins',
    gitea: 'Gitea',
    'docker-registry': 'Docker Registry',
    harbor: 'Harbor',
    'prometheus-grafana': 'Prometheus + Grafana',
    loki: 'Loki + Promtail',
    postgresql: 'PostgreSQL',
    mysql: 'MySQL',
    mongodb: 'MongoDB',
    redis: 'Redis',
    rabbitmq: 'RabbitMQ',
    kafka: 'Kafka',
    minio: 'MinIO',
  }
  return labels[key] || tmpl?.name || svcLabel(serviceLogicalType(svc)) || key || '服务'
}
const serviceCategory = (svc:any) => resolveServiceCategory(svc, templates.value)
const installedTools = computed(() => services.value.filter((svc:any) => serviceCategory(svc) !== 'infra'))
const installedInfra = computed(() => services.value.filter((svc:any) => serviceCategory(svc) === 'infra'))
const serviceCapability = (svc:any): CapabilityTab => resolveServiceCapability(svc, templates.value)
const capabilityTabs = computed<CapabilityTab[]>(() => buildEnvironmentCapabilityTabs(services.value, templates.value))
const activeCapabilityTab = computed(() =>
  capabilityTabs.value.find(tab => tab.key === activeTab.value)
  || knownCapabilityTabByKey.get(activeTab.value)
  || null
)
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
const workspaceActionVisible = (action: WorkspaceAction) => Boolean(action?.key && action.key !== 'refresh')
const activeCapabilityWorkspaceActions = computed<WorkspaceAction[]>(() =>
  (activeCapabilityWorkspace.value.actions || []).filter(workspaceActionVisible)
)
const workspaceUsesInternalActions = (type?: string) => ['mysql', 'postgresql'].includes(String(type || ''))
const activeCapabilityWorkspaceUsesInternalActions = computed(() =>
  workspaceUsesInternalActions(activeCapabilityService.value?.serviceType)
)
const capabilityWorkspaceKey = computed(() =>
  `${envRouteKey.value}:${activeCapabilityTab.value?.key || 'none'}:${activeCapabilityService.value?.id || 'none'}`
)
const registryServices = computed(() => services.value.filter((svc:any) => {
  const key = serviceProductKey(svc) || serviceLogicalType(svc)
  return ['registry', 'docker-registry', 'harbor'].includes(key)
}))
const registryCapabilityTargets = computed(() => environmentCapabilities.value.filter((cap:any) => {
  const capability = String(cap?.capability || '').toLowerCase()
  const serviceType = String(cap?.serviceType || cap?.refService?.serviceType || '').toLowerCase()
  return capability === 'registry' || ['registry', 'docker-registry', 'harbor'].includes(serviceType)
}))
const registryWorkspaceServiceTargets = computed(() => {
  const targets = [...registryServices.value]
  for (const cap of registryCapabilityTargets.value) {
    const svc = cap?.refService
    if (!svc?.id) continue
    if (!targets.some((item:any) => Number(item.id) === Number(svc.id))) targets.push(svc)
  }
  return targets
})
const registryWorkspaces = computed(() =>
  registryWorkspaceServiceTargets.value
    .map((svc:any) => capabilityWorkspaceCache.value[svc.id])
    .filter(Boolean) as ServiceWorkspace[]
)
const registryHostFromWorkspace = (workspace?: ServiceWorkspace | null) => {
  if (!workspace) return ''
  const trust = workspace.resources.find((x: WorkspaceResource) => x.type === 'Runtime Trust')
  const host = String(trust?.annotations?.registryHost || '').trim()
  if (host) return host
  const configured = (workspace.config || []).find(item => item.label === '外部访问地址')?.value || ''
  return String(configured).replace(/^https?:\/\//, '').replace(/\/$/, '')
}
const externalEndpointHost = (endpoint:any) => String(endpoint || '').trim().replace(/^https?:\/\//, '').replace(/\/$/, '')
const registryTargetOptions = computed<RegistryTargetOption[]>(() => {
  const options: RegistryTargetOption[] = []
  for (const svc of registryServices.value) {
    const workspace = capabilityWorkspaceCache.value[svc.id] || null
    const host = registryHostFromWorkspace(workspace)
    options.push({
      key: `service:${svc.id}`,
      label: `本环境 · ${svc.serviceName || svc.serviceType || '镜像仓库'}${host ? ` · ${host}` : ''}`,
      source: 'managed',
      host,
      service: svc,
      workspace,
    })
  }
  for (const cap of registryCapabilityTargets.value) {
    if (cap.source === 'shared' && cap.refService?.id) {
      const svc = cap.refService
      const workspace = capabilityWorkspaceCache.value[svc.id] || null
      const host = registryHostFromWorkspace(workspace)
      options.push({
        key: `capability:${cap.id}`,
        label: `共享资源 · ${svc.serviceName || svc.serviceType || '镜像仓库'}${host ? ` · ${host}` : ''}`,
        source: 'shared',
        host,
        service: svc,
        capability: cap,
        workspace,
      })
    } else if (cap.source === 'external') {
      const host = externalEndpointHost(cap.externalEndpoint)
      options.push({
        key: `capability:${cap.id}`,
        label: `外部资源 · ${cap.provider || cap.serviceType || '镜像仓库'}${host ? ` · ${host}` : ''}`,
        source: 'external',
        host,
        capability: cap,
      })
    }
  }
  return options
})
const selectedRegistryTarget = computed(() =>
  registryTargetOptions.value.find(item => item.key === configForm.value.registryTargetKey) || registryTargetOptions.value[0] || null
)
const registryTargetSelectionPayload = () => {
  const target = selectedRegistryTarget.value
  if (!target) return null
  return {
    key: target.key,
    source: target.source,
    host: target.host,
    serviceId: Number(target.service?.id || 0),
    capabilityId: Number(target.capability?.id || 0),
    serviceType: String(target.service?.serviceType || target.capability?.serviceType || target.capability?.provider || ''),
    name: String(target.service?.serviceName || target.capability?.provider || ''),
  }
}
const registryHostForDrawer = computed(() => {
  if (selectedRegistryTarget.value?.host) return selectedRegistryTarget.value.host
  for (const workspace of registryWorkspaces.value) {
    const trust = workspace.resources.find((x: WorkspaceResource) => x.type === 'Runtime Trust')
    const host = String(trust?.annotations?.registryHost || '').trim()
    if (host) return host
  }
  const configured = registryWorkspaces.value.flatMap(workspace => workspace.config || []).find(item => item.label === '外部访问地址')?.value || ''
  return String(configured).replace(/^https?:\/\//, '').replace(/\/$/, '')
})
const registryTargetKeyForHost = (host:string) => {
  const normalized = externalEndpointHost(host)
  if (!normalized) return registryTargetOptions.value[0]?.key || ''
  return registryTargetOptions.value.find(item =>
    externalEndpointHost(item.host) === normalized ||
    (item.host && normalized.startsWith(`${externalEndpointHost(item.host)}/`))
  )?.key || registryTargetOptions.value[0]?.key || ''
}
const registryTargetKeyFromConfig = (cfg:any, host:string) => {
  const savedKey = String(cfg?.registryTarget?.key || '').trim()
  if (savedKey) return savedKey
  return registryTargetKeyForHost(host)
}
const registryImageRepositories = computed<RegistryRepositoryOption[]>(() => {
  const targetWorkspace = selectedRegistryTarget.value?.workspace
  const resources = (targetWorkspace ? [targetWorkspace] : registryWorkspaces.value).flatMap(workspace => workspace.resources || [])
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
        repository: registryRepositorySuffix(String(annotations.repository || imageRepository || resource.name || '').trim(), registryHostForDrawer.value),
        tags,
        resource,
      }
    })
    .filter(item => item.repository)
})
const selectedRegistryRepository = computed(() =>
  registryImageRepositories.value.find(item => item.repository === splitImageRepositoryAndTag(configForm.value.imageTag).repository) || null
)
const selectedRegistryRepositoryTags = computed(() => selectedRegistryRepository.value?.tags || [])
const registryImageTagOptions = computed(() =>
  registryImageRepositories.value.flatMap((item) => {
    if (!item.tags.length) return [item.repository]
    return item.tags.map((tag) => `${item.repository}:${tag}`)
  })
)
const registryImageSelectionValid = computed(() => {
  const imageTag = String(configForm.value.imageTag || '').trim()
  return hasExplicitNonLatestImageTag(imageTag)
})
const registryImageFromConfig = computed(() => {
  return imageRefFromRegistryFields(String(configForm.value.repository || ''), String(configForm.value.imageTag || ''))
})
const sourceDeliveryImagePreview = computed(() => {
  const host = String(configForm.value.repository || registryHostForDrawer.value || '').trim().replace(/\/+$/, '')
  const version = String(configForm.value.version || '').trim()
  const current = configDrawer.value.component || {}
  const appIdentifier = String(app.value?.identifier || 'app').trim()
  const envIdentifier = String(env.value?.identifier || 'env').trim()
  const identifier = String(current.identifier || current.name || 'component').trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
  if (!host || !version || !identifier) return ''
  return `${host}/${appIdentifier}-${envIdentifier}/${identifier}:${version}`
})
const validateComponentDeliveryForm = () => {
  const deliveryMode = configForm.value.deliveryMode === 'source' ? 'source' : 'image'
  const version = deliveryMode === 'source'
    ? String(configForm.value.version || '').trim()
    : imageTagVersion(configForm.value.imageTag) || String(configForm.value.version || '').trim()
  const image = registryImageFromConfig.value || String(configForm.value.image || '').trim()
  const sourceRepoUrl = String(configForm.value.sourceRepoUrl || '').trim()
  if (deliveryMode === 'image' && (!version || version.toLowerCase() === 'latest')) {
    configDrawer.value.error = '请填写明确镜像 Tag，不能使用 latest。'
    return false
  }
  if (deliveryMode === 'source' && version && version.toLowerCase() === 'latest') {
    configDrawer.value.error = '镜像 Tag 不能使用 latest。'
    return false
  }
  if (deliveryMode === 'image' && !image) {
    configDrawer.value.error = '请填写镜像地址。'
    return false
  }
  if (deliveryMode === 'image' && !registryImageSelectionValid.value) {
    configDrawer.value.error = '请选择镜像仓库中的镜像，或填写明确的镜像:Tag。'
    return false
  }
  if (deliveryMode === 'source' && !sourceRepoUrl) {
    configDrawer.value.error = '请填写源码仓库地址。'
    return false
  }
  const containerPort = Number(configForm.value.containerPort || 0)
  if (!Number.isInteger(containerPort) || containerPort < 1 || containerPort > 65535) {
    configDrawer.value.error = '请填写 1-65535 范围内的容器端口。'
    return false
  }
  return true
}
const ensureRegistryWorkspaces = async () => {
  const targets = registryWorkspaceServiceTargets.value.filter((svc:any) => svc?.id)
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
const syncRegistryTargetSelection = () => {
  const target = selectedRegistryTarget.value
  if (!target) return
  if (target.host) configForm.value.repository = target.host
}
watch(registryHostForDrawer, (host) => {
  if (!host || !configDrawer.value.visible || configDrawer.value.kind !== 'component') return
  const current = String(configForm.value.repository || '').trim()
  if (!current || current !== host) configForm.value.repository = host
})
watch(registryTargetOptions, (options) => {
  if (!configDrawer.value.visible || configDrawer.value.kind !== 'component') return
  const current = String(configForm.value.registryTargetKey || '').trim()
  if (!current || !options.some(item => item.key === current)) {
    configForm.value.registryTargetKey = options[0]?.key || ''
    syncRegistryTargetSelection()
  }
})
const syncConfigVersionFromImageTag = () => {
  const currentTag = imageTagVersion(configForm.value.imageTag)
  if (currentTag) {
    configForm.value.version = currentTag
    return
  }
  if (selectedRegistryRepositoryTags.value.length) {
    configForm.value.imageTag = `${splitImageRepositoryAndTag(configForm.value.imageTag).repository}:${selectedRegistryRepositoryTags.value[0]}`
    configForm.value.version = selectedRegistryRepositoryTags.value[0]
  }
}

const workspaceComponentForService = (svc:any) => {
  const key = serviceProductKey(svc) || serviceLogicalType(svc)
  switch (key) {
    case 'monitor': case 'prometheus-grafana': case 'prometheus': case 'grafana': return MonitorWorkspace
    case 'log': case 'loki': case 'promtail': return LogWorkspace
    case 'deploy': case 'argocd': return ArgocdWorkspace
    case 'git': case 'gitea': return GiteaWorkspace
    case 'ci': case 'jenkins': return PipelineWorkspace
    case 'mysql': case 'postgresql': return DatabaseWorkspace
    case 'redis': return RedisWorkspace
    case 'mongodb': return MongoWorkspace
    case 'rabbitmq': return RabbitWorkspace
    case 'kafka': return KafkaWorkspace
    case 'minio': return MinioWorkspace
    case 'registry': case 'docker-registry': case 'harbor': return RegistryWorkspace
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
const beginWorkspaceActionForService = async (svc:any, action: WorkspaceAction, target?: string) => {
  if (!svc?.id) return
  activeWorkspaceActionServiceId.value = Number(svc.id)
  if (action.fields?.length) {
    capabilityWorkspaceError.value = ''
    capabilityWorkspaceMessage.value = ''
    configDrawer.value.error = ''
    configDrawer.value.message = ''
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
const beginCapabilityWorkspaceAction = async (action: WorkspaceAction, target?: string) => {
  await beginWorkspaceActionForService(activeCapabilityService.value, action, target)
}
const beginDrawerWorkspaceAction = async (action: WorkspaceAction, target?: string) => {
  await beginWorkspaceActionForService(drawerService.value, action, target)
}
const closeWorkspaceActionInline = () => {
  if (capabilityWorkspaceLoading.value) return
  activeCapabilityAction.value = null
  activeCapabilityActionTarget.value = undefined
  activeCapabilityActionParams.value = {}
  activeWorkspaceActionServiceId.value = null
  capabilityWorkspaceError.value = ''
  configDrawer.value.error = ''
}
const setWorkspaceActionCheckboxParam = (name: string, event: Event) => {
  activeCapabilityActionParams.value[name] = (event.target as HTMLInputElement).checked ? 'true' : 'false'
}
const setWorkspaceActionParam = ({ name, value }: { name: string; value: string }) => {
  activeCapabilityActionParams.value = {
    ...activeCapabilityActionParams.value,
    [name]: value,
  }
}
const submitWorkspaceActionInline = async () => {
  const action = activeCapabilityAction.value
  if (!action) return
  const validationMessage = validateWorkspaceActionParams(action.fields || [], activeCapabilityActionParams.value)
  if (validationMessage) {
    capabilityWorkspaceError.value = validationMessage
    if (activeWorkspaceActionBelongsToDrawer.value) configDrawer.value.error = validationMessage
    return
  }
  await runCapabilityWorkspaceAction(action, activeCapabilityActionTarget.value, activeCapabilityActionParams.value)
  if (!capabilityWorkspaceError.value) {
    activeCapabilityAction.value = null
    activeCapabilityActionTarget.value = undefined
    activeCapabilityActionParams.value = {}
    activeWorkspaceActionServiceId.value = null
  }
}
const serviceDrawerWorkspaceEmbedsActions = computed(() => {
  const type = serviceDrawerType.value
  return ['postgresql', 'mysql', 'mongodb', 'redis', 'rabbitmq', 'kafka', 'minio'].includes(type)
})
const serviceDrawerWorkspaceEmbeddedActionProps = computed(() => {
  if (!serviceDrawerWorkspaceEmbedsActions.value || !activeWorkspaceActionBelongsToDrawer.value) return {}
  return {
    activeAction: activeCapabilityAction.value,
    activeActionTarget: activeCapabilityActionTarget.value,
    actionParams: activeCapabilityActionParams.value,
    actionRunning: serviceDrawerWorkspaceLoading.value || capabilityWorkspaceLoading.value,
    actionError: configDrawer.value.error || capabilityWorkspaceError.value,
    onUpdateActionParam: setWorkspaceActionParam,
    onSubmitAction: submitWorkspaceActionInline,
    onCancelAction: closeWorkspaceActionInline,
  }
})
const activeWorkspaceActionService = computed(() => {
  const id = activeWorkspaceActionServiceId.value
  if (!id) return activeCapabilityService.value
  return services.value.find((item:any) => Number(item.id) === Number(id))
    || (Number(drawerService.value?.id) === Number(id) ? drawerService.value : null)
    || activeCapabilityService.value
})
const activeWorkspaceActionBelongsToDrawer = computed(() =>
  Boolean(activeWorkspaceActionServiceId.value && Number(drawerService.value?.id) === Number(activeWorkspaceActionServiceId.value))
)
const runCapabilityWorkspaceAction = async (action: WorkspaceAction, target?: string, params?: Record<string, string>) => {
  const svc = activeWorkspaceActionService.value
  if (!svc?.id || !action?.key) return
  capabilityWorkspaceLoading.value = true
  capabilityWorkspaceError.value = ''
  capabilityWorkspaceMessage.value = ''
  if (activeWorkspaceActionBelongsToDrawer.value) {
    configDrawer.value.error = ''
    configDrawer.value.message = ''
  }
  try {
    const res = await api.runServiceWorkspaceAction(envId.value, svc.id, action.key, target, params)
    capabilityWorkspaceCache.value = { ...capabilityWorkspaceCache.value, [svc.id]: res.data }
    capabilityWorkspaceMessage.value = '执行完成，工作台已刷新。'
    if (activeWorkspaceActionBelongsToDrawer.value) configDrawer.value.message = '执行完成，工作台已刷新。'
  } catch (e:any) {
    const message = '执行失败：' + (e?.message || '未知错误')
    capabilityWorkspaceError.value = message
    if (activeWorkspaceActionBelongsToDrawer.value) configDrawer.value.error = message
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
  const priority = ['git', 'registry', 'harbor', 'deploy', 'monitor', 'log']
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
const hasFailedServiceType = (types: string[]) => services.value.some((svc:any) => types.includes(svc.serviceType) && ['failed', 'error'].includes(String(svc.status || '').toLowerCase()))
const serviceForTypes = (types: string[]) => services.value.find((svc:any) => types.includes(svc.serviceType))
const deliveryServiceState = (types: string[]) => {
  if (hasFailedServiceType(types)) return 'failed'
  if (hasServiceType(types)) return 'ready'
  return 'missing'
}
const capabilityKeyForServiceTypes = (types: string[]) =>
  requiredEnvironmentCapabilities.find((item) => item.serviceTypes.some((type) => types.includes(type)))?.key || ''
const tabForServiceTypes = (types: string[]) => {
  const svc = serviceForTypes(types)
  return svc ? serviceCapability(svc).key : capabilityKeyForServiceTypes(types) || 'components'
}
const foundationStateLabel = (state: string) => ({
  ready: '已安装',
  failed: '异常',
  missing: '未安装',
} as Record<string, string>)[state] || '未知'
const environmentFoundationItems = computed(() => requiredEnvironmentCapabilities.map((item) => {
  const svc = serviceForTypes(item.serviceTypes)
  const state = deliveryServiceState(item.serviceTypes)
  return {
    ...item,
    service: svc,
    state,
    statusText: svc ? serviceStatusText(svc.status) : foundationStateLabel(state),
  }
}))
const missingRequiredEnvironmentCapabilities = computed(() =>
  environmentFoundationItems.value.filter((item) => item.state === 'missing')
)
const openFoundationCapability = (item: { key: string; service?: any }) => {
  if (item.service) {
    activeCapabilityServiceId.value = item.service.id
  }
  setActiveTab(item.key)
}
const deliverySteps = computed(() => [
  {
    key: 'source',
    label: '组件源码',
    description: components.value.length ? `${components.value.length} 个组件已创建` : '创建 source 组件后进入交付流程',
    state: components.value.length ? 'ready' : 'missing',
    targetTab: 'components' as const,
  },
  {
    key: 'git',
    label: '代码仓库',
    description: hasServiceType(['git']) ? '代码仓库能力已安装' : '需要安装代码仓库工具',
    state: deliveryServiceState(['git']),
    targetTab: tabForServiceTypes(['git']),
  },
  {
    key: 'registry',
    label: '镜像仓库',
    description: hasServiceType(['registry', 'harbor']) ? '镜像仓库能力已安装' : '需要安装镜像仓库',
    state: deliveryServiceState(['registry', 'harbor']),
    targetTab: tabForServiceTypes(['registry', 'harbor']),
  },
  {
    key: 'deploy',
    label: '持续部署',
    description: hasServiceType(['deploy']) ? '持续部署能力已安装' : '需要安装持续部署工具',
    state: deliveryServiceState(['deploy']),
    targetTab: tabForServiceTypes(['deploy']),
  },
  {
    key: 'monitor',
    label: '监控',
    description: hasServiceType(['monitor']) ? '监控能力已安装' : '需要安装监控工具',
    state: deliveryServiceState(['monitor']),
    targetTab: tabForServiceTypes(['monitor']),
  },
  {
    key: 'log',
    label: '日志',
    description: hasServiceType(['log']) ? '日志能力已安装' : '需要安装日志工具',
    state: deliveryServiceState(['log']),
    targetTab: tabForServiceTypes(['log']),
  },
])
const serviceProvisionModeOptions = computed(() => serviceProvisionModes.map((mode) => {
  const hasSupportedService = availableServices.value.some((svc:any) => serviceSupportsProvisionMode(svc, mode.key))
  const enabled = mode.key === 'managed'
      ? hasSupportedService
      : !isSystemSharedEnvironment.value && hasSupportedService
  const description = mode.description
  return { ...mode, enabled, description }
}))
const serviceStatusForProvisionMode = (serviceType:string, mode: ServiceProvisionMode) => {
  const normalizedType = String(serviceType || '').toLowerCase()
  const normalizedMode = mode === 'kubevirt' ? 'kubevirt' : 'managed'
  return services.value.find((svc:any) => {
    if (String(svc.serviceType || '').toLowerCase() !== normalizedType) return false
    const provisionMode = String(svc.provisionMode || 'managed').toLowerCase()
    if (normalizedMode === 'kubevirt') return provisionMode === 'kubevirt'
    return provisionMode !== 'kubevirt'
  }) || null
}
const visibleServiceOptions = computed(() =>
  availableServices.value
    .filter((svc:any) => serviceSupportsProvisionMode(svc, serviceProvisionMode.value))
    .map((svc:any) => {
      if (serviceProvisionMode.value === 'shared' || serviceProvisionMode.value === 'external') return svc
      const active = serviceStatusForProvisionMode(svc.type, serviceProvisionMode.value)
      return {
        ...svc,
        disabled: Boolean(active),
        statusText: active ? (active.status === 'installing' ? '安装中' : active.status === 'draft' || active.status === 'pending' ? '已添加' : active.status === 'failed' || active.status === 'error' ? '安装失败' : '已安装') : '可添加',
      }
    })
)
const selectedServiceTemplate = computed(() =>
  visibleServiceOptions.value.find((svc:any) => svc.type === serviceForm.value.serviceType) || null
)
const matchingSharedResources = computed(() => {
  const selectedType = String(serviceForm.value.serviceType || '').toLowerCase()
  if (!selectedType) return []
  const targetCapability = serviceCapability({ serviceType: selectedType }).key
  return sharedCapabilityResources.value.filter((resource:any) => {
    const serviceType = String(resource.serviceType || '').toLowerCase()
    const capability = String(resource.capability || '').toLowerCase()
    return serviceType === selectedType || capability === targetCapability
  })
})
const selectedSharedResource = computed(() =>
  matchingSharedResources.value.find((resource:any) => String(resource.id) === selectedSharedResourceId.value) || null
)
const serviceProvisionModeLabel = computed(() =>
  serviceProvisionModes.find(mode => mode.key === serviceProvisionMode.value)?.label || '使用方式'
)
const serviceSubmitLabel = computed(() => ({
  managed: '安装',
  shared: '引用公共服务',
  external: '创建外部连接',
  kubevirt: '创建 KubeVirt 服务',
} as Record<ServiceProvisionMode, string>)[serviceProvisionMode.value])
const serviceSubmitDisabled = computed(() => {
  if (serviceModalLoading.value || serviceSubmitting.value || !serviceForm.value.serviceType) return true
  if (serviceProvisionMode.value === 'shared') return !selectedSharedResource.value
  if (serviceProvisionMode.value === 'external') return !externalCapabilityForService(selectedServiceTemplate.value)
  return false
})
const selectableServiceCount = computed(() => visibleServiceOptions.value.filter((svc:any) => !svc.disabled).length)
const isActiveServiceInstalled = (serviceType:string, mode: ServiceProvisionMode = 'managed') =>
  Boolean(serviceStatusForProvisionMode(serviceType, mode === 'kubevirt' ? 'kubevirt' : 'managed'))

const activeCapabilityInstallLabel = computed(() => {
  const tab = activeCapabilityTab.value
  if (!tab) return '安装能力'
  return tab.category === 'infra' ? `创建${tab.label}` : `安装${tab.label}`
})

const activeCapabilityEmptyText = computed(() => {
  const tab = activeCapabilityTab.value
  if (!tab) return '当前环境还没有安装对应能力。'
  if (tab.key === 'monitoring-center') return '当前环境还没有监控工具。安装后可以从组件、数据库、缓存和消息队列卡片直接跳到指标视图。'
  if (tab.key === 'logging-center') return '当前环境还没有日志工具。安装后可以从任意卡片直接打开对应组件或中间件日志。'
  return `当前环境还没有${tab.label}能力。可以直接从这里安装工具或创建中间件。`
})

const serviceTypesForCapability = (key = '') => ({
  'code-repository': ['git'],
  'image-registry': ['registry', 'harbor'],
  'continuous-integration': ['ci'],
  'continuous-deployment': ['deploy'],
  'monitoring-center': ['monitor'],
  'logging-center': ['log'],
  databases: ['postgresql', 'mysql', 'mongodb'],
  cache: ['redis'],
  'message-queue': ['rabbitmq', 'kafka'],
  'object-storage': ['minio'],
} as Record<string, string[]>)[key] || []

const activeCapabilityInstallHint = computed(() => {
  const tab = activeCapabilityTab.value
  if (!tab) return ''
  if (tab.category === 'infra') return '选择服务后会先创建卡片，再在右侧配置规格、连接变量和部署。'
  return '选择服务后会直接安装工具，并打开对应工具卡片查看状态。'
})

const activeCapabilityInstallTemplates = computed(() => {
  const tab = activeCapabilityTab.value
  if (!tab) return []
  const items = buildPickerTemplates(templates.value, services.value, tab.category)
  const allowed = serviceTypesForCapability(tab.key)
  if (!allowed.length) return items
  const order = new Map(allowed.map((type, idx) => [type, idx]))
  return items
    .filter((item:any) => order.has(item.type))
    .sort((a:any, b:any) => (order.get(a.type) || 0) - (order.get(b.type) || 0))
})

const installCapabilityTemplate = async (tmpl:any) => {
  if (!tmpl?.type || tmpl.disabled || capabilityInlineInstallingType.value) return
  const selectedType = String(tmpl.type)
  const tab = activeCapabilityTab.value
  capabilityInlineInstallingType.value = selectedType
  capabilityInlineInstallError.value = ''
  capabilityWorkspaceMessage.value = ''
  try {
    const beforeIds = new Set(services.value.map((item:any) => Number(item.id)))
    if (tab?.category === 'infra') {
      await api.createServiceDraft(envId.value, { serviceType: selectedType })
    } else {
      await api.installService(envId.value, { serviceType: selectedType })
    }
    await refreshServices()
    const installed = services.value.find((item:any) => !beforeIds.has(Number(item.id)) && item.serviceType === selectedType)
      || services.value.find((item:any) => item.serviceType === selectedType)
    if (installed) {
      activeCapabilityServiceId.value = Number(installed.id)
      openServiceConfigDrawer(installed)
      capabilityWorkspaceMessage.value = `${svcLabel(selectedType)} 已添加。`
    }
    scheduleTemplateInstallPolling()
  } catch (e:any) {
    capabilityInlineInstallError.value = '添加失败：' + (e?.message || '未知错误')
  } finally {
    capabilityInlineInstallingType.value = ''
  }
}

const preferredServiceTypeFromPicker = (items:any[], preferredType = '') => {
  const preferred = items.find((svc:any) => !svc.disabled && svc.type === preferredType)
  if (preferred) return preferred.type
  return items.find((svc:any) => !svc.disabled)?.type || ''
}
const preferredServiceTypeForProvisionMode = (preferredType = '') =>
  preferredServiceTypeFromPicker(visibleServiceOptions.value, preferredType)
const selectServiceProvisionMode = (mode: ServiceProvisionMode) => {
  const option = serviceProvisionModeOptions.value.find(item => item.key === mode)
  if (!option?.enabled) return
  serviceProvisionMode.value = mode
  serviceModalError.value = ''
  serviceForm.value.serviceType = preferredServiceTypeForProvisionMode(serviceForm.value.serviceType)
  selectedSharedResourceId.value = ''
  if (mode === 'shared') {
    void loadSharedCapabilityResources().then(() => {
      selectedSharedResourceId.value = matchingSharedResources.value[0]?.id ? String(matchingSharedResources.value[0].id) : ''
    })
  }
}

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
  await Promise.all([loadEnvironmentCapabilities(), loadSharedCapabilityResources()])
  if (configDrawer.value.visible && configDrawer.value.kind === 'service' && configDrawer.value.service?.id) {
    const refreshed = services.value.find((item:any) => Number(item.id) === Number(configDrawer.value.service?.id))
    if (refreshed) configDrawer.value.service = refreshed
  }
  if (configDrawer.value.visible && configDrawer.value.kind === 'component' && configDrawer.value.component?.id) {
    const refreshed = components.value.find((item:any) => Number(item.id) === Number(configDrawer.value.component?.id))
    if (refreshed) configDrawer.value.component = refreshed
  }
  notifyEnvUpdated()
}

const loadEnvironmentCapabilities = async () => {
  if (!envId.value) {
    environmentCapabilities.value = []
    return
  }
  const res = await api.listEnvironmentCapabilities(envId.value)
  environmentCapabilities.value = Array.isArray(res.data) ? res.data : []
}

const loadSharedCapabilityResources = async () => {
  try {
    const res = await api.listSharedCapabilityResources()
    sharedCapabilityResources.value = Array.isArray(res.data) ? res.data : []
  } catch {
    sharedCapabilityResources.value = []
  }
}

const prepareServicePicker = async (mode:'tool'|'infra', preferredType = '') => {
  serviceModalMode.value = mode
  serviceProvisionMode.value = 'managed'
  selectedSharedResourceId.value = ''
  showServiceModal.value = true
  serviceModalError.value = ''
  const session = createPickerSessionState(templates.value, services.value, mode)
  availableServices.value = session.availableServices
  serviceForm.value.serviceType = preferredServiceTypeForProvisionMode(preferredType) || session.selectedType
  serviceModalLoading.value = session.loading
  serviceModalNotice.value = session.notice
  serviceModalError.value = session.error
  try {
    await refreshServices()
    if (templates.value.length === 0) await loadServiceTemplates()
    availableServices.value = filterTemplates(mode)
    const firstEnabledMode = serviceProvisionModeOptions.value.find(item => item.enabled)
    serviceProvisionMode.value = firstEnabledMode?.key || 'managed'
    serviceForm.value.serviceType = preferredServiceTypeForProvisionMode(preferredType)
    serviceModalNotice.value = pickerNotice(mode, availableServices.value.length, serviceForm.value.serviceType)
  } catch (e:any) {
    serviceModalError.value = '服务加载失败：' + (e?.message || '未知错误')
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
  if (!comp?.id || componentActionLoading.value) return false
  const version = componentDeployVersion(comp)
  if (!version || version.toLowerCase() === 'latest') {
    const message = '部署前需要在组件配置中填写明确版本，不能使用 latest。'
    pageError.value = message
    if (configDrawer.value.visible && Number(configDrawer.value.component?.id) === Number(comp.id)) {
      configDrawer.value.error = message
      configDrawer.value.message = ''
    }
    return false
  }
  componentActionLoading.value = true
  pageError.value = ''
  if (configDrawer.value.visible && Number(configDrawer.value.component?.id) === Number(comp.id)) {
    configDrawer.value.error = ''
    configDrawer.value.message = `正在提交 ${comp.name || '组件'} 部署...`
  }
  try {
    await api.deployComponent(Number(comp.id), { version })
    await refreshServices()
    scheduleComponentStatusPolling(Number(comp.id))
    scheduleTemplateInstallPolling()
    selectComponent(comp.id)
    const refreshed = components.value.find((item:any) => Number(item.id) === Number(comp.id))
    if (configDrawer.value.visible && Number(configDrawer.value.component?.id) === Number(comp.id)) {
      if (refreshed) configDrawer.value.component = refreshed
      configDrawer.value.message = `${comp.name || '组件'} 部署已提交，部署工具会继续同步集群资源。`
    }
    return true
  } catch (e:any) {
    const message = '部署失败：' + (e?.message || '未知错误')
    pageError.value = message
    if (configDrawer.value.visible && Number(configDrawer.value.component?.id) === Number(comp.id)) {
      configDrawer.value.error = message
      configDrawer.value.message = ''
    }
    return false
  } finally {
    componentActionLoading.value = false
    closeComponentContextMenu()
  }
}
const drawerRuntime = computed(() => configDrawer.value.component?.runtimeConfig || configDrawer.value.service?.runtimeConfig || {})
const componentDrawerType = computed(() => String(componentDrawerRole.value || configDrawer.value.component?.type || 'custom').toLowerCase())
const componentDrawerUsesSourceDelivery = computed(() => configForm.value.deliveryMode === 'source')
const componentDrawerRoleOptions = [
  { value: 'frontend', label: '前端 / Web 入口' },
  { value: 'backend', label: '后端 / API 服务' },
  { value: 'database', label: '数据库工作负载' },
  { value: 'middleware', label: '中间件工作负载' },
  { value: 'custom', label: '自定义工作负载' },
]
const componentDrawerProfileModel = computed(() => {
  const comp = configDrawer.value.component || {}
  return buildComponentProfile({
    component: { ...comp, type: componentDrawerType.value },
    form: {
      framework: configForm.value.framework,
      env: configForm.value.env,
      configMaps: configForm.value.configMaps,
      secrets: configForm.value.secrets,
      files: configForm.value.files,
      bindings: configForm.value.bindings,
    },
  })
})
const componentDrawerInferredFramework = computed(() => componentDrawerProfileModel.value.framework)
const componentDrawerDetectedCapabilities = computed(() => componentDrawerProfileModel.value.capabilityLabels)
const componentDrawerProfile = computed(() => componentDrawerProfileModel.value)
const componentDrawerBlueprintModel = computed(() => componentDrawerBlueprint(componentDrawerProfile.value))
const componentDrawerCapabilityRows = computed(() => {
  const comp = configDrawer.value.component || {}
  return [
    { label: '组件用途', value: componentDrawerRoleOptions.find(item => item.value === componentDrawerType.value)?.label || compTypeText(componentDrawerType.value) },
    { label: '配置模型', value: componentDrawerBlueprintModel.value.label },
    { label: '配置策略', value: componentDrawerBlueprintModel.value.configStrategyLabel },
    { label: '识别框架', value: componentDrawerInferredFramework.value === 'unknown' ? '未识别' : componentFrameworkLabel(componentDrawerInferredFramework.value) },
    { label: '交付方式', value: componentDeliveryModeLabel(comp) },
    { label: '已识别能力', value: componentDrawerDetectedCapabilities.value.join(', ') || '未识别' },
    { label: '配置来源', value: componentDrawerConfigSourceSummary.value },
    { label: '已连接服务', value: String(configForm.value.bindings.length) },
  ]
})
const componentDrawerConfigSourceSummary = computed(() => componentDrawerProfileModel.value.configSourceSummary)
const componentDrawerSuggestedActions = computed(() => {
  const actions: Array<{ key: string; label: string }> = []
  if (componentDrawerProfile.value.webEntry && componentDrawerBackendTargets.value.length && !nginxRouteRows.value.length) {
    actions.push({ key: 'proxy-route', label: '配置代理路由' })
  }
  if ((componentDrawerProfile.value.apiService || componentDrawerProfile.value.hasRuntimeDependencies) && componentDrawerDependencyTargets.value.length && !configForm.value.bindings.length) {
    actions.push({ key: 'select-dependency', label: '添加运行依赖' })
  }
  if (componentDrawerInferredFramework.value === 'springboot' && configForm.value.bindingMode !== 'springboot-file') {
    actions.push({ key: 'springboot-config-file', label: '使用 Spring Boot 配置文件' })
  }
  return actions
})
const componentDrawerBackendTargets = computed(() =>
  componentConnectionTargets.value.filter((target:any) =>
    target.kind === 'component' && componentTargetLooksLikeApi(target.component)
  )
)
const componentDrawerDependencyTargets = computed(() =>
  componentConnectionTargets.value.filter((target:any) => {
    const type = String(target.type || '').toLowerCase()
    if (['service', 'capability'].includes(String(target.kind || ''))) {
      return ['postgresql', 'mysql', 'mongodb', 'redis', 'rabbitmq', 'kafka', 'minio', 'log'].includes(type)
    }
    return componentTargetCanAcceptDependency(target.component)
  })
)
const componentTargetLooksLikeApi = (comp:any) => {
  return buildComponentProfile({ component: comp }).apiService
}
const componentTargetCanAcceptDependency = (comp:any) => {
  const type = String(comp?.type || '').toLowerCase()
  if (['backend', 'custom'].includes(type)) return true
  return componentTargetLooksLikeApi(comp)
}
const componentDrawerConfigKeySuggestions = computed(() => {
  return componentConfigKeySuggestions(componentDrawerProfile.value)
})
const drawerService = computed(() => configDrawer.value.kind === 'service' ? configDrawer.value.service : null)
const drawerCapability = computed(() => configDrawer.value.kind === 'capability' ? configDrawer.value.capability : null)
const drawerCapabilitySource = computed(() => String(drawerCapability.value?.source || '').toLowerCase())
const sharedCapabilityService = computed(() => drawerCapabilitySource.value === 'shared' ? (drawerCapability.value?.refService || null) : null)
const sharedCapabilityInternalEndpoint = computed(() => sharedCapabilityService.value ? serviceInternalEndpoint(sharedCapabilityService.value) : '')
const sharedCapabilityType = computed(() => {
  const cap = drawerCapability.value || {}
  const service = sharedCapabilityService.value || {}
  return String(service.serviceType || cap.serviceType || cap.provider || cap.capability || '').toLowerCase()
})
const sharedCapabilityEndpointParts = computed(() => splitEndpoint(sharedCapabilityInternalEndpoint.value, defaultServicePortForBinding(sharedCapabilityType.value)))
const sharedCapabilityUsername = computed(() => {
  const credentials = sharedCapabilityCredentials.value
  const fromSecret = capabilityCredentialValue(credentials, ['username', 'user', 'postgres-user', 'postgresql-username', 'mysql-user', 'mongodb-root-user', 'rabbitmq-username', 'root-user', 'access-key'])
  if (fromSecret) return fromSecret
  const defaults: Record<string, string> = {
    postgresql: 'postgres',
    mysql: 'root',
    mongodb: 'root',
    rabbitmq: 'user',
    minio: 'minioadmin',
  }
  if (sharedCapabilityType.value === 'redis' || sharedCapabilityType.value === 'kafka') return '无'
  return defaults[sharedCapabilityType.value] || '按共享资源配置'
})
const sharedCapabilityPasswordKeys = computed(() => {
  const type = sharedCapabilityType.value
  if (type === 'postgresql') return ['postgres-password', 'postgresql-password', 'password']
  if (type === 'mysql') return ['mysql-root-password', 'mysql-password', 'password']
  if (type === 'mongodb') return ['mongodb-root-password', 'mongodb-password', 'password']
  if (type === 'redis') return ['redis-password', 'password']
  if (type === 'rabbitmq') return ['rabbitmq-password', 'password']
  if (type === 'minio') return ['root-password', 'secret-key', 'secretkey', 'minio-secret-key', 'password']
  return ['password', 'token']
})
const sharedCapabilityPassword = computed(() => capabilityCredentialValue(sharedCapabilityCredentials.value, sharedCapabilityPasswordKeys.value))
const sharedCapabilitySecretVisible = (key:string) => sharedCapabilitySecretVisibleKeys.value.has(key)
const sharedCapabilityPasswordDisplay = computed(() => {
  if (!sharedCapabilitySecretVisible('password')) return '******'
  if (sharedCapabilityCredentialLoading.value) return '读取中...'
  return sharedCapabilityPassword.value || '未读取到密码'
})
const sharedCapabilityDefaultDatabase = computed(() => {
  const type = sharedCapabilityType.value
  if (type === 'mysql') return 'mysql'
  if (type === 'postgresql') return 'postgres'
  if (type === 'mongodb') return 'admin'
  return ''
})
const sharedCapabilityConnectionString = computed(() => {
  const type = sharedCapabilityType.value
  const [host, port] = sharedCapabilityEndpointParts.value
  const username = sharedCapabilityUsername.value
  const password = sharedCapabilityPasswordDisplay.value
  if (type === 'postgresql') return `postgresql://${username}:${password}@${host}:${port}/${sharedCapabilityDefaultDatabase.value}`
  if (type === 'mysql') return `mysql://${username}:${password}@${host}:${port}/${sharedCapabilityDefaultDatabase.value}`
  if (type === 'mongodb') return `mongodb://${username}:${password}@${host}:${port}/${sharedCapabilityDefaultDatabase.value}`
  if (type === 'redis') return `redis://:${password}@${host}:${port}/0`
  if (type === 'rabbitmq') return `amqp://${username}:${password}@${host}:${port}/`
  if (type === 'kafka') return `${host}:${port}`
  if (type === 'minio') return `http://${host}:${port}`
  return `${host}:${port}`
})
const sharedCapabilityConnectionRows = computed(() => {
  if (drawerCapabilitySource.value !== 'shared' || !sharedCapabilityService.value) return []
  const [host, port] = sharedCapabilityEndpointParts.value
  const rows: Array<{ label: string; value: string; secretKey?: string }> = [
    { label: '地址', value: host || '部署后生成' },
    { label: '端口', value: String(port || '-') },
    { label: '用户名', value: sharedCapabilityUsername.value },
  ]
  if (sharedCapabilityType.value !== 'kafka') rows.push({ label: '密码', value: sharedCapabilityPasswordDisplay.value, secretKey: 'password' })
  if (sharedCapabilityDefaultDatabase.value) rows.push({ label: '数据库', value: sharedCapabilityDefaultDatabase.value })
  return [
    ...rows,
    { label: '连接串', value: sharedCapabilityConnectionString.value, secretKey: ['kafka', 'minio'].includes(sharedCapabilityType.value) ? undefined : 'password' },
  ]
})
const drawerCapabilityRows = computed(() => {
  const cap = drawerCapability.value || {}
  const ref = cap.refService || {}
  const rows = [
    { label: '来源', value: capabilitySourceLabel(cap.source || '') },
    { label: '能力', value: capabilityLabel(cap.capability || '') },
    { label: 'Provider', value: cap.provider || '-' },
    { label: '服务类型', value: typeLabel(cap.serviceType || ref.serviceType || '') || '-' },
  ]
  if (drawerCapabilitySource.value === 'shared') {
    rows.push(
      { label: '共享服务', value: ref.serviceName || ref.serviceType || cap.serviceType || '-' },
      { label: '运行状态', value: serviceStatusText(ref.status || cap.validationStatus || '') },
    )
  }
  if (drawerCapabilitySource.value === 'external') {
    rows.push(
      { label: '外部地址', value: cap.externalEndpoint || '-' },
      { label: 'credentialSecretRef', value: cap.credentialSecretRef || '-' },
      { label: 'TLS 校验', value: cap.tlsInsecureSkipVerify ? '跳过' : '启用' },
    )
  }
  if (cap.validationMessage) rows.push({ label: '校验信息', value: cap.validationMessage })
  return rows
})
const externalCapabilityOptionFor = (capability:string) =>
  externalCapabilityOptions.find((item) => item.capability === capability)
const providerForCapability = (capability:string) =>
  externalCapabilityOptionFor(capability)?.provider || capability || 'external'
const serviceTypeForCapability = (capability:string) =>
  externalCapabilityOptionFor(capability)?.serviceType || capability || 'external'
const externalCapabilityPlaceholder = (cap:any) =>
  externalCapabilityOptionFor(cap?.capability)?.externalPlaceholder || 'https://service.example.com'
const capabilitySecretVisible = (key:string) => capabilitySecretVisibleKeys.value.has(key)
const capabilityCredentialValue = (credentials:any[], keys:string[]) => {
  const normalizedKeys = keys.map((key) => key.toLowerCase())
  const match = credentials.find((item:any) => normalizedKeys.includes(String(item.key || '').toLowerCase()))
  return match?.value ? String(match.value) : ''
}
const loadSharedCapabilityCredentials = async () => {
  const cap = drawerCapability.value
  if (!envId.value || !cap?.capability || sharedCapabilityCredentialLoading.value) return
  const requestKey = capabilityRequestKey(cap)
  if (sharedCapabilityCredentialCapability.value === requestKey && sharedCapabilityCredentials.value.length) return
  sharedCapabilityCredentialLoading.value = true
  sharedCapabilityCredentialError.value = ''
  try {
    const res = await api.getEnvironmentCapabilityCredentials(envId.value, requestKey)
    sharedCapabilityCredentials.value = Array.isArray(res?.data?.credentials) ? res.data.credentials : []
    sharedCapabilityCredentialCapability.value = requestKey
  } catch (e:any) {
    sharedCapabilityCredentials.value = []
    sharedCapabilityCredentialError.value = '读取共享资源凭据失败：' + (e?.message || '未知错误')
  } finally {
    sharedCapabilityCredentialLoading.value = false
  }
}
const toggleSharedCapabilitySecret = async (key:string) => {
  const next = new Set(sharedCapabilitySecretVisibleKeys.value)
  if (next.has(key)) {
    next.delete(key)
    sharedCapabilitySecretVisibleKeys.value = next
    return
  }
  await loadSharedCapabilityCredentials()
  next.add(key)
  sharedCapabilitySecretVisibleKeys.value = next
}
const loadCapabilityCredentials = async () => {
  const cap = drawerCapability.value
  if (!envId.value || !cap?.capability || !cap.credentialSecretRef || capabilityCredentialLoading.value) return
  capabilityCredentialLoading.value = true
  capabilityCredentialError.value = ''
  try {
    const res = await api.getEnvironmentCapabilityCredentials(envId.value, capabilityRequestKey(cap))
    const credentials = Array.isArray(res?.data?.credentials) ? res.data.credentials : []
    const endpoint = capabilityCredentialValue(credentials, ['endpoint'])
    const username = capabilityCredentialValue(credentials, ['username', 'user'])
    const password = capabilityCredentialValue(credentials, ['password'])
    const token = capabilityCredentialValue(credentials, ['token', 'access-token'])
    const authType = capabilityCredentialValue(credentials, ['authType', 'auth-type'])
    if (endpoint && !capabilityForm.value.externalEndpoint) capabilityForm.value.externalEndpoint = endpoint
    if (username && !capabilityForm.value.username) capabilityForm.value.username = username
    if (password && !capabilityForm.value.password) capabilityForm.value.password = password
    if (token && !capabilityForm.value.token) capabilityForm.value.token = token
    if (authType && capabilityForm.value.authType === 'none') capabilityForm.value.authType = authType
    if (password && capabilityForm.value.authType === 'none') capabilityForm.value.authType = 'basic'
    if (token && capabilityForm.value.authType === 'none') capabilityForm.value.authType = 'token'
  } catch (e:any) {
    capabilityCredentialError.value = '读取外部资源凭据失败：' + (e?.message || '未知错误')
  } finally {
    capabilityCredentialLoading.value = false
  }
}
const toggleCapabilitySecret = async (key:'password' | 'token') => {
  const next = new Set(capabilitySecretVisibleKeys.value)
  if (next.has(key)) {
    next.delete(key)
    capabilitySecretVisibleKeys.value = next
    return
  }
  if (!capabilityForm.value[key]) {
    await loadCapabilityCredentials()
  }
  next.add(key)
  capabilitySecretVisibleKeys.value = next
}
watch(() => drawerService.value?.id || '', () => {
  serviceDrawerRevealedSecrets.value = new Set()
  serviceDrawerSecretValues.value = {}
  serviceDrawerSecretLoadingKey.value = ''
})
const serviceDrawerLogicalType = computed(() => serviceLogicalType(drawerService.value))
const serviceDrawerType = computed(() => serviceProductKey(drawerService.value) || serviceDrawerLogicalType.value)
const serviceDrawerConfigType = computed(() => serviceConfigType({ ...(drawerService.value || {}), productKey: serviceDrawerType.value }))
const serviceDrawerWorkspace = computed<ServiceWorkspace | null>(() => {
  const svc = drawerService.value
  if (!svc?.id) return null
  return capabilityWorkspaceCache.value[svc.id] || null
})
const serviceDrawerProfile = computed(() => serviceConfigProfile({ ...(drawerService.value || {}), productKey: serviceDrawerConfigType.value }))
const serviceDrawerWorkspaceTabKey = computed<ConfigDrawerTab>(() => {
  const kind = serviceDrawerProfile.value.kind
  if (kind === 'database') return 'database'
  if (kind === 'redis') return 'data'
  if (kind === 'message-queue') return 'queues'
  if (kind === 'object-storage') return 'buckets'
  return 'workspace'
})
const serviceDrawerWorkspaceActive = computed(() =>
  configDrawer.value.kind === 'service' && configDrawerTab.value === serviceDrawerWorkspaceTabKey.value
)
const serviceDrawerWorkspaceData = computed<ServiceWorkspace>(() => serviceDrawerWorkspace.value || emptyCapabilityWorkspace)
const serviceDrawerWorkspaceActions = computed<WorkspaceAction[]>(() =>
  (serviceDrawerWorkspaceData.value.actions || []).filter(workspaceActionVisible)
)
const serviceDrawerWorkspaceUsesInternalActions = computed(() =>
  workspaceUsesInternalActions(serviceDrawerType.value)
)
const serviceDrawerWorkspaceReady = computed(() => Boolean(drawerService.value?.id && serviceDrawerWorkspace.value))
const serviceDrawerWorkspaceComponent = computed(() => drawerService.value ? workspaceComponentForService(drawerService.value) : null)
const serviceDrawerWorkspaceKey = computed(() =>
  `${envRouteKey.value}:drawer:${drawerService.value?.id || 'none'}:${serviceDrawerWorkspaceTabKey.value}`
)
const serviceDrawerWorkspaceTabLabel = computed(() => {
  const type = serviceDrawerType.value
  const labels: Record<string, string> = {
    git: '代码仓库',
    gitea: '代码仓库',
    deploy: '资源',
    argocd: '资源',
    ci: '流水线',
    jenkins: '流水线',
    registry: '镜像',
    'docker-registry': '镜像',
    harbor: '镜像',
    monitor: '大盘',
    'prometheus-grafana': '大盘',
    log: '日志查询',
    loki: '日志查询',
    postgresql: '数据库',
    mysql: '数据库',
    mongodb: '文档',
    redis: '缓存',
    rabbitmq: '队列',
    kafka: 'Topic',
    minio: '对象',
  }
  return labels[type] || serviceDrawerProfile.value.workspaceTitle || '工作台'
})
const serviceDrawerWorkspaceTitle = computed(() => {
  const type = serviceDrawerType.value
  const labels: Record<string, string> = {
    git: '代码仓库工作台',
    gitea: 'Gitea 代码仓库',
    deploy: 'ArgoCD 应用与资源拓扑',
    argocd: 'ArgoCD 应用与资源拓扑',
    ci: '流水线工作台',
    jenkins: 'Jenkins 流水线',
    registry: '镜像仓库工作台',
    'docker-registry': 'Docker Registry 镜像仓库',
    harbor: '镜像仓库工作台',
    monitor: '监控工作台',
    'prometheus-grafana': 'Prometheus + Grafana 监控',
    log: '日志工作台',
    loki: 'Loki 日志工作台',
    postgresql: '数据库对象',
    mysql: '数据库对象',
    mongodb: '数据库对象',
    redis: '缓存数据',
    rabbitmq: '队列对象',
    kafka: 'Topic 对象',
    minio: 'Bucket 与对象',
  }
  return labels[type] || serviceDrawerProfile.value.workspaceTitle || '服务工作台'
})
const serviceDrawerWorkspaceDescription = computed(() => {
  const type = serviceDrawerType.value
  const labels: Record<string, string> = {
    git: '查看仓库、文件、提交和仓库操作。',
    gitea: '查看 Gitea 仓库、文件、提交和仓库操作。',
    deploy: '查看 Application、同步状态、健康状态和集群资源树。',
    argocd: '查看 ArgoCD Application、同步状态、健康状态和集群资源树。',
    ci: '查看 Jenkins Job、构建状态，并触发工作台操作。',
    jenkins: '查看 Jenkins Job、构建状态，并触发工作台操作。',
    registry: '查看镜像仓库、Tag、Digest 和节点运行时信任信息。',
    'docker-registry': '查看 Docker Registry 仓库、Tag、Digest 和节点运行时信任信息。',
    harbor: '查看 Harbor 项目、镜像制品、Tag 和运行时信任信息。',
    monitor: '查看 Grafana 面板、Prometheus Target、告警和规则。',
    'prometheus-grafana': '查看 Grafana 面板、Prometheus Target、告警和规则。',
    log: '查看 Loki 日志对象、日志流和 Grafana 日志面板。',
    loki: '查看 Loki 日志对象、日志流和 Grafana 日志面板。',
    postgresql: '查看数据库、表、字段、连接和对象级操作。',
    mysql: '查看数据库、表、字段、连接和对象级操作。',
    mongodb: '查看数据库、集合、文档和对象级操作。',
    redis: '查看 keyspace、实例信息、连接和对象级操作。',
    rabbitmq: '查看队列、Exchange、连接和对象级操作。',
    kafka: '查看 Topic、Broker 资源、连接和对象级操作。',
    minio: '查看 Bucket、对象、访问连接和对象级操作。',
  }
  return labels[type] || '这里承载原服务页面的详细内容。'
})
const workspaceActionButtonClass = (action: WorkspaceAction) => [
  'workspace-action-btn',
  action?.tone === 'primary' ? 'workspace-action-btn--primary' : '',
  action?.tone === 'danger' ? 'workspace-action-btn--danger' : '',
].filter(Boolean)
const serviceDrawerWorkspaceEmptyText = computed(() => {
  if (!serviceStatusHasRuntime(drawerService.value)) return '服务尚未部署，部署成功后这里会显示原服务页面里的真实资源和操作。'
  if (serviceDrawerWorkspaceLoading.value) return '正在读取服务工作台。'
  return '服务工作台暂未返回资源，点击刷新重新读取运行态。'
})
const serviceDrawerConfigFields = computed(() => serviceDrawerProfile.value.fields)
const serviceDrawerVisibleConfigFields = computed(() => serviceDrawerConfigFields.value.filter((field) =>
  serviceConfigFieldVisible(field, serviceConfigForm.value) && !isServiceStorageConfigField(field),
))
const serviceDrawerVolumeFields = computed<ServiceVolumeField[]>(() => {
  const fields = serviceDrawerConfigFields.value
  return fields
    .filter((field) => isServiceStorageSizeField(field) && serviceConfigFieldVisible(field, serviceConfigForm.value))
    .map((field) => {
      const enabledField = serviceVolumeEnabledFieldForSize(field, fields)
      return {
        label: serviceVolumeLabel(field),
        description: serviceVolumeDescription(field, enabledField),
        enabledKey: enabledField?.key || '',
        sizeKey: field.key,
        placeholder: field.placeholder || String(field.defaultValue || '8Gi'),
      }
    })
})
const serviceVolumeSizePresets = ['1Gi', '5Gi', '10Gi', '20Gi', '50Gi']
const serviceDrawerPreviewValues = computed(() => {
  const svc = drawerService.value
  if (!svc) return {}
  return {
    ...serviceConfigValues(svc),
    ...serviceConfigValuesFromForm(serviceDrawerConfigType.value, serviceConfigForm.value),
  }
})
const serviceDrawerPreviewService = computed(() => {
  const svc = drawerService.value
  if (!svc) return null
  return {
    ...svc,
    values: serviceDrawerPreviewValues.value,
  }
})
const serviceDrawerConnectionPreview = computed(() => connectionBindingPreview({ env: [] }, serviceDrawerPreviewService.value || drawerService.value || {}))
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
const isServiceStorageConfigField = (field: ServiceConfigField) => {
  const key = String(field.key || '').toLowerCase()
  return key.includes('persistence') && (key.endsWith('.enabled') || key === 'persistence.enabled' || key.endsWith('.size') || key === 'persistence.size')
}
const isServiceStorageSizeField = (field: ServiceConfigField) => {
  const key = String(field.key || '').toLowerCase()
  return key.includes('persistence') && (key.endsWith('.size') || key === 'persistence.size')
}
const serviceVolumeEnabledFieldForSize = (sizeField: ServiceConfigField, fields: ServiceConfigField[]) => {
  const directKey = sizeField.key.replace(/size$/, 'enabled')
  const direct = fields.find((field) => field.key === directKey)
  if (direct) return direct
  return fields.find((field) => field.key === 'persistence.enabled')
}
const serviceVolumeLabel = (field: ServiceConfigField) => {
  const label = String(field.label || '数据卷')
    .replace(/Read Replica/gi, '只读副本')
    .replace(/Primary/gi, '主库')
    .replace(/Secondary/gi, '从库')
    .replace(/Master/gi, '主节点')
    .replace(/Replica/gi, '副本节点')
    .replace(/Cluster/gi, '集群节点')
    .replace(/容量$/, '')
    .replace(/存储$/, '数据卷')
    .trim()
  return label || '数据卷'
}
const serviceVolumeDescription = (sizeField: ServiceConfigField, enabledField?: ServiceConfigField) => {
  const label = String(sizeField.label || enabledField?.label || '').toLowerCase()
  if (label.includes('grafana')) return '保存 Grafana 仪表盘、插件和运行数据。'
  if (label.includes('loki')) return '保存日志索引和日志块数据。'
  if (label.includes('registry')) return '保存镜像层、Manifest 和 Tag 数据。'
  if (label.includes('jenkins')) return '保存流水线配置、任务历史和工作目录。'
  if (label.includes('harbor')) return '保存 Harbor 组件的运行数据。'
  if (label.includes('redis')) return '保存缓存快照、AOF 或集群节点数据。'
  if (label.includes('只读') || label.includes('replica') || label.includes('secondary') || label.includes('副本') || label.includes('从库')) return '保存副本实例的数据。'
  if (label.includes('主库') || label.includes('主节点') || label.includes('primary') || label.includes('master')) return '保存主实例的数据。'
  if (label.includes('queue') || label.includes('broker') || label.includes('controller')) return '保存消息队列和控制面数据。'
  if (label.includes('对象') || label.includes('bucket')) return '保存对象和 Bucket 元数据。'
  return '保存当前服务的持久化数据。'
}
const setServiceVolumeEnabled = (volume: ServiceVolumeField, event: Event) => {
  if (!volume.enabledKey) return
  const checked = Boolean((event.target as HTMLInputElement | null)?.checked)
  serviceConfigForm.value[volume.enabledKey] = checked
  if (checked && !String(serviceConfigForm.value[volume.sizeKey] || '').trim()) {
    serviceConfigForm.value[volume.sizeKey] = volume.placeholder || '8Gi'
  }
}
const setServiceVolumeSize = (volume: ServiceVolumeField, size: string) => {
  if (volume.enabledKey && !serviceConfigForm.value[volume.enabledKey]) return
  serviceConfigForm.value[volume.sizeKey] = size
}
const configDrawerTitle = computed(() => {
  if (configDrawer.value.kind === 'capability') return capabilityDisplayName(configDrawer.value.capability || {})
  return configDrawer.value.component?.name || configDrawer.value.service?.serviceName || configDrawer.value.service?.name || configDrawer.value.service?.serviceType || '-'
})
const configDrawerSubtitle = computed(() => {
  if (configDrawer.value.kind === 'capability') {
    const cap = configDrawer.value.capability || {}
    return `${capabilityLabel(cap.capability)} · ${capabilitySourceLabel(cap.source)}`
  }
  if (configDrawer.value.kind === 'service') {
    return `${serviceProductLabel(configDrawer.value.service)} · ${serviceStatusText(configDrawer.value.service?.status)}`
  }
  return `${compTypeText(configDrawer.value.component?.type)} · ${componentDeliveryModeLabel(configDrawer.value.component)}`
})
const configDrawerExternalUrl = computed(() => configDrawer.value.component?.externalUrl || configDrawer.value.service?.externalUrl || '')
const serviceDrawerInternalEndpoint = computed(() => drawerService.value && serviceStatusHasRuntime(drawerService.value) ? serviceInternalEndpoint(serviceDrawerPreviewService.value || drawerService.value) : '')
const componentDrawerServiceEndpoint = computed(() => {
  const comp = configDrawer.value.component
  const rc = comp?.runtimeConfig
  if (!rc?.serviceName || !rc?.namespace) return ''
  return `${rc.serviceName}.${rc.namespace}.svc.cluster.local`
})
const componentDrawerIngressUrl = computed(() => {
  return configDrawer.value.component?.ingressUrl || ''
})
const componentDrawerExternalAccessToggleVisible = computed(() => {
  const comp = configDrawer.value.component
  return configDrawer.value.kind === 'component' && comp?.runtimeConfig?.serviceName
})
const componentDrawerNodePortUrl = computed(() => {
  return configDrawer.value.component?.nodePortUrl || ''
})
const componentDrawerNodePortEnabled = computed(() => Boolean(componentDrawerNodePortUrl.value))
const componentDrawerExternalAccessEnabled = computed(() => Boolean(componentDrawerIngressUrl.value))
const componentDrawerExternalAccessLabel = computed(() => {
  if (componentExternalAccessLoading.value) return componentDrawerExternalAccessEnabled.value ? '关闭中...' : '开启中...'
  return componentDrawerExternalAccessEnabled.value ? '关闭 Ingress' : '开启 Ingress'
})
const serviceDrawerExternalAccessEnabled = computed(() => Boolean(configDrawer.value.kind === 'service' && configDrawerExternalUrl.value))
const serviceDrawerExternalAccessToggleVisible = computed(() => configDrawer.value.kind === 'service' && serviceStatusHasRuntime(drawerService.value) && serviceDrawerProfile.value.showConnectionBindings)
const serviceDrawerExternalAccessLabel = computed(() => {
  if (serviceExternalAccessLoading.value) return serviceDrawerExternalAccessEnabled.value ? '关闭中...' : '开启中...'
  return serviceDrawerExternalAccessEnabled.value ? '关闭外部访问' : '开启外部访问'
})
const configDrawerAvatarLabel = computed(() => {
  if (configDrawer.value.kind === 'capability') return 'CAP'
  if (configDrawer.value.kind === 'component') return String(configDrawer.value.component?.type || 'C').slice(0, 1).toUpperCase()
  const type = serviceDrawerType.value
  if (['postgresql', 'mysql', 'mongodb'].includes(type)) return 'DB'
  if (type === 'redis') return 'R'
  if (['rabbitmq', 'kafka'].includes(type)) return 'MQ'
  if (type === 'minio') return 'S3'
  if (type === 'argocd') return 'CD'
  if (type === 'jenkins') return 'CI'
  if (type === 'gitea') return 'Git'
  if (type === 'loki') return 'Log'
  if (type === 'prometheus-grafana') return 'Obs'
  return 'S'
})
const drawerStatusValue = computed(() => String(configDrawer.value.capability?.validationStatus || configDrawer.value.component?.status || configDrawer.value.service?.status || 'draft').toLowerCase())
const drawerStatusLabel = computed(() => {
  if (configDrawer.value.kind === 'service') return serviceStatusText(drawerStatusValue.value)
  const labels: Record<string, string> = {
    running: '运行中',
    linked: '已关联',
    syncing: '同步中',
    deploying: '部署中',
    building: '构建中',
    deployed: '已部署',
    pending: '未部署',
    draft: '未部署',
    failed: '失败',
    error: '异常',
  }
  return labels[drawerStatusValue.value] || drawerStatusValue.value || '未知'
})
const drawerDeploymentTitle = computed(() => {
  if (configDrawer.value.kind === 'capability') {
    return drawerCapabilitySource.value === 'external' ? 'External capability' : 'Shared capability'
  }
  if (configDrawer.value.kind === 'service') {
    return serviceStatusHasRuntime(drawerService.value) ? 'Active deployment' : 'Draft deployment'
  }
  return drawerStatusValue.value === 'running' ? 'Active deployment' : 'Ready to deploy'
})
const drawerDeploymentSubtitle = computed(() => {
  if (configDrawer.value.kind === 'capability') {
    const cap = drawerCapability.value || {}
    return `${capabilityLabel(cap.capability || '')} · ${capabilitySourceLabel(cap.source || '')}`
  }
  if (configDrawer.value.kind === 'service') {
    const type = svcLabel(serviceDrawerType.value) || serviceDrawerType.value || 'Service'
    return `${type} · ${drawerStatusLabel.value}`
  }
  const comp = configDrawer.value.component || {}
  return `${componentDeliveryModeLabel(comp)} · ${drawerStatusLabel.value}`
})
const drawerDeploymentRows = computed(() => {
  return [] as Array<{ label: string; value: string }>
})
const resourceSourceSummaryRows = computed(() => {
  if (configDrawer.value.kind === 'capability') {
    const cap = drawerCapability.value || {}
    const source = String(cap.source || '')
    if (source === 'shared') {
      return [
        { label: '来源类型', value: '平台共享', hint: '由共享资源池提供，业务环境只读引用。' },
        { label: '移除动作', value: '断开引用', hint: '只解除当前环境引用，不会删除共享资源池中的服务。' },
      ]
    }
    if (source === 'external') {
      return [
        { label: '来源类型', value: '外部资源', hint: '连接到集群外已有系统，由当前环境保存连接信息。' },
        { label: '移除动作', value: '断开外部连接', hint: '只删除连接记录和本地凭据，不会删除外部系统。' },
      ]
    }
    return [
      { label: '来源类型', value: '环境内资源', hint: '由 PAAP 在当前环境内安装和管理，生命周期归当前环境。' },
      { label: '移除动作', value: '删除卡片', hint: '移除当前环境内的能力记录。' },
    ]
  }
  if (configDrawer.value.kind === 'service') {
    return [
      { label: '来源类型', value: '环境内资源', hint: '由 PAAP 在当前环境内安装、升级和卸载，生命周期归当前环境。' },
      { label: '移除动作', value: '卸载服务', hint: '会卸载当前环境中的工具或中间件实例。' },
    ]
  }
  if (configDrawer.value.kind === 'component') {
    return [
      { label: '来源类型', value: '应用组件', hint: '属于当前应用在本环境中的业务工作负载。' },
      { label: '移除动作', value: '删除组件', hint: '会删除组件记录并清理该组件在集群中的运行态资源。' },
    ]
  }
  return [] as Array<{ label: string; value: string; hint?: string }>
})
const serviceDrawerVariableRows = computed(() => {
  const svc = drawerService.value
  if (!svc) return []
  const type = serviceDrawerType.value
  const endpoint = serviceDrawerInternalEndpoint.value || serviceDrawerConnectionPreview.value.bindings.find((item) => item.name.endsWith('_HOST'))?.value || svc.serviceName || svc.name || type
  const host = serviceDrawerConnectionPreview.value.bindings.find((item) => item.name.endsWith('_HOST'))?.value || endpoint
  const port = serviceDrawerConnectionPreview.value.bindings.find((item) => item.name.endsWith('_PORT'))?.value || ''
  const rows: Array<{ name: string; value: string; hint: string; secret?: boolean }> = []
  const add = (name: string, value: string, hint: string, secret = false) => rows.push({ name, value, hint, secret })
  if (type === 'postgresql' || type === 'mysql') {
    const prefix = type === 'mysql' ? 'MYSQL' : 'POSTGRES'
    const database = type === 'mysql' ? 'mysql' : 'postgres'
    const user = type === 'mysql' ? 'root' : 'postgres'
    add(`${prefix}_HOST`, host, '集群内服务名')
    add(`${prefix}_PORT`, port || (type === 'mysql' ? '3306' : '5432'), '数据库端口')
    add(`${prefix}_DATABASE`, database, '默认数据库')
    add(`${prefix}_USERNAME`, user, '默认账号')
    add(`${prefix}_PASSWORD`, '由敏感配置管理', '密码不会在页面明文展示', true)
    add(`${prefix}_URL`, `${type === 'mysql' ? 'mysql' : 'postgresql'}://${user}:******@${host}:${port || (type === 'mysql' ? '3306' : '5432')}/${database}`, '应用连接串', true)
  } else if (type === 'mongodb') {
    add('MONGODB_HOST', host, '集群内服务名')
    add('MONGODB_PORT', port || '27017', 'MongoDB 端口')
    add('MONGODB_USERNAME', 'root', '管理员账号')
    add('MONGODB_PASSWORD', '由敏感配置管理', '密码不会在页面明文展示', true)
    add('MONGODB_URL', `mongodb://root:******@${host}:${port || '27017'}/admin`, '应用连接串', true)
  } else if (type === 'redis') {
    add('REDIS_HOST', host, 'Redis 主节点服务名')
    add('REDIS_PORT', port || '6379', 'Redis 端口')
    add('REDIS_PASSWORD', '由敏感配置管理', '密码不会在页面明文展示', true)
    add('REDIS_URL', `redis://:******@${host}:${port || '6379'}/0`, '应用连接串', true)
    if (serviceConfigForm.value.architecture === 'cluster') {
      const clusterNodes = serviceDrawerConnectionPreview.value.bindings.find((item) => item.name === 'REDIS_CLUSTER_NODES')?.value || `${host}:${port || '6379'}`
      add('REDIS_CLUSTER_HOST', host, 'Redis Cluster 服务名')
      add('REDIS_CLUSTER_PORT', port || '6379', 'Redis Cluster 端口')
      add('REDIS_CLUSTER_NODES', clusterNodes, 'Cluster startup nodes')
    }
    if (serviceConfigForm.value.architecture === 'sentinel') {
      add('REDIS_SENTINEL_HOST', host, 'Sentinel 服务名')
      add('REDIS_SENTINEL_PORT', '26379', 'Sentinel 端口')
      add('REDIS_SENTINEL_MASTER_NAME', String(serviceDrawerPreviewValues.value['sentinel.masterSet'] || 'mymaster'), 'Sentinel master set')
    }
  } else if (type === 'rabbitmq') {
    add('RABBITMQ_HOST', host, 'RabbitMQ 服务名')
    add('RABBITMQ_PORT', port || '5672', 'AMQP 端口')
    add('RABBITMQ_USERNAME', 'user', '默认账号')
    add('RABBITMQ_PASSWORD', '由敏感配置管理', '密码不会在页面明文展示', true)
    add('RABBITMQ_URL', `amqp://user:******@${host}:${port || '5672'}/`, 'AMQP 连接串', true)
  } else if (type === 'kafka') {
    add('KAFKA_BROKERS', `${host}:${port || '9092'}`, 'Bootstrap servers')
    add('KAFKA_SECURITY_PROTOCOL', 'PLAINTEXT', '默认集群内协议')
  } else if (type === 'minio') {
    add('MINIO_ENDPOINT', `${host}:${port || '9000'}`, 'S3 兼容入口')
    add('MINIO_ACCESS_KEY', '由敏感配置管理', '访问密钥不会明文展示', true)
    add('MINIO_SECRET_KEY', '由敏感配置管理', '访问密钥不会明文展示', true)
  } else {
    for (const binding of serviceDrawerConnectionPreview.value.bindings) {
      add(binding.name, binding.value, '连接参数')
    }
  }
  return rows
})
const serviceDrawerSecretVisible = (name:string) => serviceDrawerRevealedSecrets.value.has(name)
const serviceDrawerSecretDisplayValue = (row:{ name: string; value: string }) => {
  if (!serviceDrawerSecretVisible(row.name)) return '******'
  return serviceDrawerSecretValues.value[row.name] || '未读取到凭据'
}
const serviceDrawerSecretKeysForName = (name:string, type:string) => {
  const upper = String(name || '').toUpperCase()
  if (upper === 'MINIO_ACCESS_KEY') return ['root-user', 'access-key', 'accesskey', 'minio-access-key', 'username']
  if (upper === 'MINIO_SECRET_KEY') return ['root-password', 'secret-key', 'secretkey', 'minio-secret-key', 'password']
  if (upper.includes('POSTGRES') || type === 'postgresql') return ['postgres-password', 'password']
  if (upper.includes('MYSQL') || type === 'mysql') return ['mysql-root-password', 'mysql-password', 'password']
  if (upper.includes('MONGODB') || type === 'mongodb') return ['mongodb-root-password', 'mongodb-password', 'password']
  if (upper.includes('REDIS') || type === 'redis') return ['redis-password', 'password']
  if (upper.includes('RABBITMQ') || type === 'rabbitmq') return ['rabbitmq-password', 'password']
  return ['password']
}
const serviceDrawerSecretDisplayFromCredential = (row:{ name: string; value: string }, secret:string) => {
  if (!secret) return ''
  const upper = String(row.name || '').toUpperCase()
  if (upper.endsWith('_URL')) return String(row.value || '').replace('******', encodeURIComponent(secret))
  return secret
}
const loadServiceDrawerSecretValue = async (row:{ name: string; value: string }) => {
  if (serviceDrawerSecretValues.value[row.name]) return
  const svc = drawerService.value
  if (!svc?.id) return
  serviceDrawerSecretLoadingKey.value = row.name
  try {
    const credentials = await serviceCredentials(svc)
    const secret = credentialValue(credentials, serviceDrawerSecretKeysForName(row.name, serviceDrawerType.value))
    serviceDrawerSecretValues.value = {
      ...serviceDrawerSecretValues.value,
      [row.name]: serviceDrawerSecretDisplayFromCredential(row, secret),
    }
  } catch (e:any) {
    serviceDrawerSecretValues.value = {
      ...serviceDrawerSecretValues.value,
      [row.name]: `读取失败：${e?.message || '未知错误'}`,
    }
  } finally {
    serviceDrawerSecretLoadingKey.value = ''
  }
}
const toggleServiceDrawerSecret = async (row:{ name: string; value: string }) => {
  const next = new Set(serviceDrawerRevealedSecrets.value)
  if (next.has(row.name)) {
    next.delete(row.name)
    serviceDrawerRevealedSecrets.value = next
    return
  }
  next.add(row.name)
  serviceDrawerRevealedSecrets.value = next
  await loadServiceDrawerSecretValue(row)
}
const serviceDrawerDatabaseRows = computed(() => {
  const type = serviceDrawerType.value
  const form = serviceConfigForm.value
	  return [
	    { label: '引擎', value: svcLabel(type) || type || '-' },
	    { label: '连接地址', value: serviceDrawerInternalEndpoint.value || '部署后生成' },
	    { label: '架构', value: String(form.architecture || 'standalone') },
	    { label: '存储', value: String(form['primary.persistence.size'] || form['persistence.size'] || '临时存储') },
	  ]
	})
const serviceDrawerDefaultDatabase = computed(() => serviceDrawerType.value === 'mysql' ? 'mysql' : 'postgres')
const serviceDrawerBackups = computed(() =>
  (serviceDrawerWorkspaceData.value.resources || []).filter((resource: WorkspaceResource) => resource.type === 'Backup')
)
const backupStorageLabel = (backup: WorkspaceResource) => {
  const storage = String(backup?.annotations?.storage || '').trim()
  if (!storage || storage.toLowerCase() === 'kubernetes secret') return '平台备份'
  return storage
}
const serviceDrawerBackupStorage = computed(() => {
  const backup = serviceDrawerBackups.value[0]
  if (backup?.annotations?.storage) return backupStorageLabel(backup)
  return '平台备份'
})
const serviceDrawerBackupAction = computed<WorkspaceAction>(() => {
  const action = (serviceDrawerWorkspaceData.value.actions || []).find((item: WorkspaceAction) => item.key === 'create_database_backup')
  if (action) {
    return {
      ...action,
      fields: (action.fields || [{ name: 'database', label: '数据库名', required: true }]).map((field: WorkspaceActionField) =>
        field.name === 'database'
          ? { ...field, default: field.default || serviceDrawerDefaultDatabase.value, placeholder: field.placeholder || serviceDrawerDefaultDatabase.value }
          : field
      ),
    }
  }
  return {
    key: 'create_database_backup',
    label: '创建备份',
    description: '导出指定数据库的表结构和数据，并保存为平台备份。',
    tone: 'primary',
    fields: [{ name: 'database', label: '数据库名', required: true, default: serviceDrawerDefaultDatabase.value, placeholder: serviceDrawerDefaultDatabase.value }],
  }
})
const serviceDrawerDataRows = computed(() => [
  { label: '模式', value: String(serviceConfigForm.value.architecture || 'standalone') },
  { label: '连接地址', value: serviceDrawerInternalEndpoint.value || '部署后生成' },
  { label: '存储', value: serviceConfigForm.value.architecture === 'cluster' ? (serviceConfigForm.value['persistence.enabled'] ? String(serviceConfigForm.value['persistence.size'] || '已启用') : '临时存储') : (serviceConfigForm.value['master.persistence.enabled'] ? String(serviceConfigForm.value['master.persistence.size'] || '已启用') : '临时存储') },
  { label: '副本', value: serviceConfigForm.value.architecture === 'cluster' ? String(serviceConfigForm.value['cluster.replicas'] || 0) : String(serviceConfigForm.value['replica.replicaCount'] || 0) },
  { label: '集群节点', value: serviceConfigForm.value.architecture === 'cluster' ? String(serviceConfigForm.value['cluster.nodes'] || 0) : '-' },
  { label: 'Sentinel', value: serviceConfigForm.value.architecture === 'sentinel' ? '启用' : '未启用' },
])
const serviceDrawerQueueRows = computed(() => {
  if (serviceDrawerType.value === 'kafka') {
	    return [
	      { label: '连接地址', value: serviceDrawerInternalEndpoint.value || '部署后生成' },
	      { label: '控制节点', value: String(serviceConfigForm.value['controller.replicaCount'] || 3) },
	      { label: '消息节点', value: String(serviceConfigForm.value['broker.replicaCount'] || 0) },
	      { label: '存储', value: serviceConfigForm.value['controller.persistence.enabled'] ? String(serviceConfigForm.value['controller.persistence.size'] || '已启用') : '临时存储' },
	    ]
	  }
	  return [
	    { label: 'AMQP', value: serviceDrawerInternalEndpoint.value || '部署后生成' },
	    { label: '副本', value: String(serviceConfigForm.value.replicaCount || 1) },
	    { label: 'VHost', value: '/' },
	    { label: '存储', value: serviceConfigForm.value['persistence.enabled'] ? String(serviceConfigForm.value['persistence.size'] || '已启用') : '临时存储' },
	  ]
	})
const serviceDrawerBucketRows = computed(() => [
  { label: '连接地址', value: serviceDrawerInternalEndpoint.value || '部署后生成' },
  { label: '模式', value: String(serviceConfigForm.value.mode || 'standalone') },
  { label: '节点', value: String(serviceConfigForm.value['statefulset.replicaCount'] || 1) },
  { label: '存储', value: serviceConfigForm.value['persistence.enabled'] ? String(serviceConfigForm.value['persistence.size'] || '已启用') : '临时存储' },
])
const runtimeMetricSamples = computed(() => Array.isArray(runtimeMetrics.value?.samples) ? runtimeMetrics.value.samples : [])
const runtimeLogSamples = computed(() => Array.isArray(runtimeLogs.value?.samples) ? runtimeLogs.value.samples : [])
const runtimeMetricCards = computed(() => {
  const summary = runtimeMetrics.value?.summary || {}
  return [
    { label: 'CPU', value: summary.cpu || '-', hint: runtimeMetrics.value?.available ? '当前用量' : '等待指标数据' },
    { label: '内存', value: summary.memory || '-', hint: runtimeMetrics.value?.available ? '当前用量' : '等待指标数据' },
    { label: '实例', value: String(summary.pods ?? runtimeMetricSamples.value.length), hint: `${summary.containers ?? runtimeMetricSamples.value.length} 个运行单元` },
    { label: '重启', value: String(summary.restarts ?? 0), hint: '运行实例重启次数' },
  ]
})
const runtimeMetricNumericValue = (sample:any, kind:'cpu' | 'memory') =>
  kind === 'cpu' ? Number(sample?.cpuCores || 0) : Number(sample?.memoryBytes || 0)
const runtimeMetricFormatValue = (raw:number, kind:'cpu' | 'memory') => {
  if (kind === 'cpu') {
    if (raw <= 0) return '0'
    if (raw < 1) return `${Math.round(raw * 1000)}m`
    return `${raw.toFixed(2)} core`
  }
  if (raw <= 0) return '0'
  if (raw < 1024 * 1024) return `${Math.round(raw / 1024)} KiB`
  if (raw < 1024 * 1024 * 1024) return `${Math.round(raw / 1024 / 1024)} MiB`
  return `${(raw / 1024 / 1024 / 1024).toFixed(2)} GiB`
}
const runtimeMetricChartFor = (kind:'cpu' | 'memory') => {
  const samples = runtimeMetricSamples.value
  const rawValues = samples.map((sample:any) => runtimeMetricNumericValue(sample, kind))
  const fallback = kind === 'cpu' ? Number(runtimeMetrics.value?.summary?.cpuCores || 0) : Number(runtimeMetrics.value?.summary?.memoryBytes || 0)
  const values: number[] = rawValues.length ? rawValues : [fallback]
  const chartValues: number[] = values.length === 1 ? [0, values[0]] : values
  const max = Math.max(...chartValues, kind === 'cpu' ? 0.001 : 1)
  const min = Math.min(...chartValues, 0)
  const span = Math.max(max - min, kind === 'cpu' ? 0.001 : 1)
  const pointList: Array<{ key: string; x: number; y: number }> = chartValues.map((value: number, idx: number) => {
    const x = chartValues.length === 1 ? 160 : 12 + (idx * 296) / Math.max(1, chartValues.length - 1)
    const y = 104 - ((value - min) / span) * 80
    return { key: `${kind}-${idx}`, x: Number(x.toFixed(2)), y: Number(y.toFixed(2)) }
  })
  const points = pointList.map((point: { x: number; y: number }) => `${point.x},${point.y}`).join(' ')
  const areaPath = pointList.length
    ? `M ${pointList[0].x} 104 L ${pointList.map((point: { x: number; y: number }) => `${point.x} ${point.y}`).join(' L ')} L ${pointList[pointList.length - 1].x} 104 Z`
    : ''
  const summary = runtimeMetrics.value?.summary || {}
  return {
    key: kind,
    label: kind === 'cpu' ? 'CPU 使用' : '内存使用',
    value: kind === 'cpu' ? (summary.cpu || runtimeMetricFormatValue(max, kind)) : (summary.memory || runtimeMetricFormatValue(max, kind)),
    points,
    pointList,
    areaPath,
    minLabel: runtimeMetricFormatValue(min, kind),
    maxLabel: runtimeMetricFormatValue(max, kind),
  }
}
const runtimeMetricCharts = computed(() => [
  runtimeMetricChartFor('cpu'),
  runtimeMetricChartFor('memory'),
])
const runtimeMetricBarStyle = (sample:any, kind:'cpu' | 'memory') => {
  const raw = runtimeMetricNumericValue(sample, kind)
  const samples = runtimeMetricSamples.value
  const max = Math.max(...samples.map((item:any) => runtimeMetricNumericValue(item, kind)), raw, kind === 'cpu' ? 0.01 : 1)
  const width = Math.max(raw > 0 ? 4 : 0, Math.min(100, (raw / max) * 100))
  return { width: `${width}%` }
}
const drawerConsoleKey = computed(() => {
  if (!configDrawer.value.visible) return ''
  if (configDrawer.value.kind === 'component') return `component:${configDrawer.value.component?.id || ''}`
  if (configDrawer.value.kind === 'capability') return ''
  return `service:${configDrawer.value.service?.id || ''}`
})
const drawerConsoleUrl = computed(() => {
  if (!envId.value || !drawerConsoleKey.value) return ''
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  if (configDrawer.value.kind === 'component') {
    const componentId = Number(configDrawer.value.component?.id || 0)
    if (!componentId) return ''
    return `${protocol}//${window.location.host}/api/v1/environments/${envId.value}/components/${componentId}/console`
  }
  const serviceId = Number(configDrawer.value.service?.id || 0)
  if (!serviceId) return ''
  return `${protocol}//${window.location.host}/api/v1/environments/${envId.value}/services/${serviceId}/console`
})
const drawerConsoleTargetLabel = computed(() => {
  const runtime = drawerRuntime.value || {}
  const workload = runtime.workloadName || runtime.serviceName || (configDrawer.value.kind === 'component' ? componentRuntimeIdentifier(configDrawer.value.component) : drawerService.value?.releaseName) || configDrawerTitle.value
  return workload || configDrawerTitle.value || '当前卡片'
})
const runtimeConsoleStatusText = computed(() => {
  if (runtimeConsoleConnecting.value) return '正在连接运行实例'
  if (runtimeConsoleConnected.value) return '已连接'
  return '未连接'
})
const runtimeConsoleDecoder = new TextDecoder()
const runtimeConsoleEncoder = new TextEncoder()
const writeToRuntimeConsole = (chunk: string) => {
  if (!chunk || !runtimeConsoleTerm) return
  runtimeConsoleTerm.write(chunk)
}
const ensureRuntimeConsoleTerm = async () => {
  if (runtimeConsoleTerm || !runtimeConsoleView.value) return
  const [{ Terminal }, { FitAddon }] = await Promise.all([
    import('@xterm/xterm'),
    import('@xterm/addon-fit'),
  ])
  await import('@xterm/xterm/css/xterm.css')
  const term = new Terminal({
    cursorBlink: true,
    convertEol: false,
    fontSize: 12,
    fontFamily: "var(--paap-mono, 'IBM Plex Mono', monospace)",
    theme: {
      background: '#161616',
      foreground: '#e0e0e0',
      cursor: '#e0e0e0',
      selectionBackground: 'rgba(141,141,141,0.35)',
    },
    scrollback: 5000,
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.open(runtimeConsoleView.value)
  fit.fit()
  term.onData((data) => {
    const socket = runtimeConsoleSocket.value
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(runtimeConsoleEncoder.encode(data))
    }
  })
  runtimeConsoleResizeObserver = new ResizeObserver(() => {
    try { fit.fit() } catch { /* ignore */ }
  })
  runtimeConsoleResizeObserver.observe(runtimeConsoleView.value)
  runtimeConsoleTerm = term
  runtimeConsoleFitAddon = fit
}
const destroyRuntimeConsoleTerm = () => {
  if (runtimeConsoleResizeObserver) {
    runtimeConsoleResizeObserver.disconnect()
    runtimeConsoleResizeObserver = null
  }
  if (runtimeConsoleTerm) {
    runtimeConsoleTerm.dispose()
    runtimeConsoleTerm = null
  }
  runtimeConsoleFitAddon = null
}
const resetRuntimeConsole = () => {
  runtimeConsoleError.value = ''
  destroyRuntimeConsoleTerm()
}
const disconnectDrawerConsole = () => {
  const socket = runtimeConsoleSocket.value
  runtimeConsoleSocket.value = null
  runtimeConsoleConnected.value = false
  runtimeConsoleConnecting.value = false
  if (socket && socket.readyState !== WebSocket.CLOSED && socket.readyState !== WebSocket.CLOSING) {
    socket.close()
  }
}
const connectDrawerConsole = async () => {
  if (!drawerConsoleUrl.value || runtimeConsoleConnecting.value || runtimeConsoleConnected.value) return
  disconnectDrawerConsole()
  runtimeConsoleError.value = ''
  runtimeConsoleConnecting.value = true
  await nextTick()
  try {
    await ensureRuntimeConsoleTerm()
  } catch (e: any) {
    runtimeConsoleConnecting.value = false
    runtimeConsoleError.value = '终端初始化失败：' + (e?.message || '未知错误')
    return
  }
  if (runtimeConsoleTerm) runtimeConsoleTerm.clear()
  const socket = new WebSocket(drawerConsoleUrl.value, runtimeConsoleWebSocketProtocols())
  socket.binaryType = 'arraybuffer'
  runtimeConsoleSocket.value = socket
  socket.onopen = () => {
    if (runtimeConsoleSocket.value !== socket) return
    runtimeConsoleConnecting.value = false
    runtimeConsoleConnected.value = true
    if (runtimeConsoleFitAddon) {
      try { runtimeConsoleFitAddon.fit() } catch { /* ignore */ }
    }
    runtimeConsoleTerm?.focus()
  }
  socket.onmessage = async (event) => {
    if (runtimeConsoleSocket.value !== socket) return
    if (typeof event.data === 'string') {
      writeToRuntimeConsole(event.data)
    } else if (event.data instanceof ArrayBuffer) {
      writeToRuntimeConsole(runtimeConsoleDecoder.decode(event.data))
    } else if (event.data instanceof Blob) {
      writeToRuntimeConsole(runtimeConsoleDecoder.decode(await event.data.arrayBuffer()))
    }
  }
  socket.onerror = () => {
    if (runtimeConsoleSocket.value !== socket) return
    runtimeConsoleError.value = '控制台连接失败，请确认当前卡片已经部署并且运行实例可用。'
  }
  socket.onclose = () => {
    if (runtimeConsoleSocket.value !== socket) return
    runtimeConsoleSocket.value = null
    runtimeConsoleConnected.value = false
    runtimeConsoleConnecting.value = false
  }
}
const showServiceRawVariables = () => {
  configDrawer.value.error = ''
  configDrawer.value.message = '服务变量由模板、敏感配置和运行态发现生成；当前页面已按可复制变量和原始环境变量分组展示。'
}
const drawerRuntimeMetricsKey = computed(() => {
  if (!configDrawer.value.visible) return ''
  if (configDrawer.value.kind === 'component') return `component:${configDrawer.value.component?.id || ''}`
  if (configDrawer.value.kind === 'capability') return ''
  return `service:${configDrawer.value.service?.id || ''}`
})
const drawerRuntimeLogsKey = computed(() => {
  if (!configDrawer.value.visible) return ''
  if (configDrawer.value.kind === 'component') return `component:${configDrawer.value.component?.id || ''}`
  if (configDrawer.value.kind === 'capability') return ''
  return `service:${configDrawer.value.service?.id || ''}`
})
const loadDrawerRuntimeMetrics = async (force = false) => {
  if (!envId.value || !configDrawer.value.visible || configDrawerTab.value !== 'runtime') return
  if (configDrawer.value.kind === 'capability') return
  if (runtimeMetricsLoading.value) return
  if (!force && runtimeMetrics.value?.targetKey === drawerRuntimeMetricsKey.value) return
  runtimeMetricsLoading.value = true
  runtimeMetricsError.value = ''
  try {
    const res = configDrawer.value.kind === 'component'
      ? await api.getComponentRuntimeMetrics(envId.value, Number(configDrawer.value.component?.id))
      : await api.getServiceRuntimeMetrics(envId.value, Number(configDrawer.value.service?.id))
    runtimeMetrics.value = { ...(res.data || {}), targetKey: drawerRuntimeMetricsKey.value }
  } catch (e:any) {
    runtimeMetrics.value = null
    runtimeMetricsError.value = '读取运行指标失败：' + (e?.message || '未知错误')
  } finally {
    runtimeMetricsLoading.value = false
  }
}
const loadDrawerRuntimeLogs = async (force = false) => {
  if (!envId.value || !configDrawer.value.visible || configDrawerTab.value !== 'logs') return
  if (configDrawer.value.kind === 'capability') return
  if (runtimeLogsLoading.value) return
  if (!force && runtimeLogs.value?.targetKey === drawerRuntimeLogsKey.value) return
  runtimeLogsLoading.value = true
  runtimeLogsError.value = ''
  try {
    const res = configDrawer.value.kind === 'component'
      ? await api.getComponentRuntimeLogs(envId.value, Number(configDrawer.value.component?.id), 200)
      : await api.getServiceRuntimeLogs(envId.value, Number(configDrawer.value.service?.id), 200)
    runtimeLogs.value = { ...(res.data || {}), targetKey: drawerRuntimeLogsKey.value }
  } catch (e:any) {
    runtimeLogs.value = null
    runtimeLogsError.value = '读取运行日志失败：' + (e?.message || '未知错误')
  } finally {
    runtimeLogsLoading.value = false
  }
}
watch([() => configDrawerTab.value, drawerRuntimeMetricsKey], () => {
  if (configDrawerTab.value === 'runtime') {
    void loadDrawerRuntimeMetrics()
  }
})
watch([() => configDrawerTab.value, drawerRuntimeLogsKey], () => {
  if (configDrawerTab.value === 'logs') {
    void loadDrawerRuntimeLogs()
  }
})
watch([() => configDrawerTab.value, drawerConsoleKey], ([tab, key], [, oldKey]) => {
  if (key !== oldKey) {
    disconnectDrawerConsole()
    resetRuntimeConsole()
  }
  if (tab === 'console' && key) {
    connectDrawerConsole()
  } else {
    disconnectDrawerConsole()
    destroyRuntimeConsoleTerm()
  }
})
const openComponentConfigDrawer = (comp:any, initialTab: ConfigDrawerTab = 'deploy') => {
  enterDrawerContext()
  const actual = components.value.find((item:any) => Number(item.id) === Number(comp?.id)) || comp
  if (!actual) return
  selectComponent(actual.id)
  configDrawerTab.value = initialTab
  configDrawer.value = { visible: true, kind: 'component', component: actual, service: null, capability: null, saving: false, error: '', message: '' }
  runtimeMetrics.value = null
  runtimeMetricsError.value = ''
  runtimeLogs.value = null
  runtimeLogsError.value = ''
  disconnectDrawerConsole()
  resetRuntimeConsole()
  loadComponentConfigForm(actual)
  void ensureRegistryWorkspaces()
}
const openServiceConfigDrawer = (svc:any) => {
  enterDrawerContext()
  const actual = services.value.find((item:any) => Number(item.id) === Number(svc?.id)) || svc
  if (!actual) return
  selectedTopologyKey.value = String(actual.topologyId || `service:${actual.id}`)
  configDrawerTab.value = 'deploy'
  configDrawer.value = { visible: true, kind: 'service', component: null, service: actual, capability: null, saving: false, error: '', message: '' }
  runtimeMetrics.value = null
  runtimeMetricsError.value = ''
  runtimeLogs.value = null
  runtimeLogsError.value = ''
  disconnectDrawerConsole()
  resetRuntimeConsole()
  configForm.value = defaultConfigForm()
  nginxRouteRows.value = []
  serviceConfigForm.value = serviceConfigFormFromInstallation(actual)
  selectedComponentConfigTemplateId.value = ''
  componentTemplateFieldValues.value = {}
  // Pre-select current version if installed, else pick first available
  const curVer = serviceChartVersion(actual)
  const versions = templates.value.filter((t:any) => t.type === (actual.serviceType || actual.type || '') && t.chartVersion)
  selectedChartVersion.value = curVer || versions[0]?.chartVersion || ''
  void loadServiceDrawerWorkspace(actual)
}
const openCapabilityConfigDrawer = (capability:any) => {
  enterDrawerContext()
  const actual = environmentCapabilities.value.find((item:any) => Number(item.id) === Number(capability?.id)) || capability
  if (!actual) return
  selectedTopologyKey.value = String(actual.topologyId || `capability:${actual.id || actual.capability}`)
  configDrawerTab.value = 'deploy'
  configDrawer.value = { visible: true, kind: 'capability', component: null, service: null, capability: actual, saving: false, error: '', message: '' }
  capabilityForm.value = {
    externalEndpoint: actual.externalEndpoint || '',
    authType: actual.credentialSecretRef ? 'existingSecret' : 'none',
    username: '',
    password: '',
    token: '',
    credentialSecretRef: actual.credentialSecretRef || '',
    tlsInsecureSkipVerify: Boolean(actual.tlsInsecureSkipVerify),
  }
  capabilitySecretVisibleKeys.value = new Set()
  capabilityCredentialError.value = ''
  sharedCapabilitySecretVisibleKeys.value = new Set()
  sharedCapabilityCredentialError.value = ''
  sharedCapabilityCredentialCapability.value = ''
  sharedCapabilityCredentials.value = []
  runtimeMetrics.value = null
  runtimeMetricsError.value = ''
  runtimeLogs.value = null
  runtimeLogsError.value = ''
  disconnectDrawerConsole()
  resetRuntimeConsole()
}
const closeConfigDrawer = () => {
  if (configDrawer.value.saving) return
  disconnectDrawerConsole()
  configDrawerTab.value = 'deploy'
  configDrawer.value = { visible: false, kind: 'component', component: null, service: null, capability: null, saving: false, error: '', message: '' }
  configForm.value = defaultConfigForm()
  nginxRouteRows.value = []
  selectedComponentConfigTemplateId.value = ''
  componentTemplateFieldValues.value = {}
  componentDrawerRole.value = 'custom'
  serviceConfigForm.value = defaultServiceConfigForm()
  capabilityForm.value = defaultCapabilityForm()
  capabilitySecretVisibleKeys.value = new Set()
  capabilityCredentialError.value = ''
  sharedCapabilitySecretVisibleKeys.value = new Set()
  sharedCapabilityCredentialError.value = ''
  sharedCapabilityCredentialCapability.value = ''
  sharedCapabilityCredentials.value = []
  runtimeMetrics.value = null
  runtimeMetricsError.value = ''
  runtimeLogs.value = null
  runtimeLogsError.value = ''
  resetRuntimeConsole()
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
  const configuredRegistryHost = String(cfg.registryTarget?.host || '').trim()
  const registryHost = configuredRegistryHost || registryHostForImageField(image, registryHostForDrawer.value)
  const bindings = normalizeConfigBindings(cfg.bindings)
  const deliveryMode = componentIsSourceDelivery(comp) ? 'source' : 'image'
  componentDrawerRole.value = String(comp?.type || 'custom').toLowerCase()
  configForm.value = {
    framework: cfg.framework || 'auto',
    deliveryMode,
    image,
    registryTargetKey: registryTargetKeyFromConfig(cfg, registryHost),
    repository: registryHost,
    imageTag: imageTagForImageField(image, registryHost),
    sourceRepoUrl: String(comp?.sourceRepoUrl || comp?.sourceMirrorRepoUrl || ''),
    sourceBranch: String(comp?.sourceBranch || 'main'),
    buildContext: String(comp?.buildContext || '.'),
    buildModule: String(comp?.buildModule || ''),
    dockerfilePath: String(comp?.dockerfilePath || ''),
    version: String(comp?.version || imageParts.tag || componentDeployVersion(comp) || ''),
    replicas: Number(runtime.replicas ?? comp?.replicas ?? 1),
    ...componentContainerPortForForm(comp, cfg),
    cpu: String(runtime.resources?.requests?.cpu || comp?.cpu || ''),
    memory: String(runtime.resources?.requests?.memory || comp?.memory || ''),
    env: markManagedConfigEnvRows(
      envSource.map((item:any) => configEnvFromRuntime(item)),
      bindings,
    ),
    configMaps: normalizeConfigObjectRows(cfg.configMaps),
    secrets: normalizeConfigObjectRows(cfg.secrets),
    files: normalizeConfigFiles(cfg.files),
    bindings,
    bindingTargetKey: '',
    bindingMode: 'recommended',
    commandText: linesFromArray(cfg.command?.length ? cfg.command : runtime.command),
    argsText: linesFromArray(cfg.args?.length ? cfg.args : runtime.args),
  }
  nginxRouteRows.value = nginxRoutesFromCurrentConfig()
  selectedComponentConfigTemplateId.value = componentConfigTemplateSelectionFromConfig(cfg)
  componentTemplateFieldValues.value = {}
  selectRecommendedComponentConfigTemplate()
}
const componentContainerPortForForm = (comp:any, cfg:any) => {
  const saved = Number(cfg?.containerPort || 0)
  if (saved > 0) return { containerPort: saved, containerPortSource: 'saved' as const }
  const runtimePorts = Array.isArray(comp?.runtimeConfig?.ports) ? comp.runtimeConfig.ports : []
  const detected = Number(runtimePorts.find((item:any) => Number(item) > 0) || 0)
  if (detected > 0) return { containerPort: detected, containerPortSource: 'detected' as const }
  return { containerPort: defaultComponentContainerPort(componentDrawerType.value), containerPortSource: 'default' as const }
}
const defaultComponentContainerPort = (componentType:string) => {
  return String(componentType || '').toLowerCase() === 'frontend' ? 80 : 8080
}
const configEnvFromRuntime = (item:any) => {
  if (item?.secretName || item?.secretKey) return { name: String(item.name || ''), source: 'secret' as const, value: '', refName: String(item.secretName || ''), refKey: String(item.secretKey || '') }
  if (item?.configMapName || item?.configMapKey) return { name: String(item.name || ''), source: 'configMap' as const, value: '', refName: String(item.configMapName || ''), refKey: String(item.configMapKey || '') }
  return { name: String(item?.name || ''), source: 'value' as const, value: String(item?.value || ''), refName: '', refKey: '' }
}
const managedConnectionEnvNames = (binding:any) => {
  const names = new Set(Object.keys(binding?.generated || {}).map((key) => String(key || '').trim()).filter(Boolean))
  const type = String(binding?.targetType || '').toLowerCase()
  const add = (items:string[]) => items.forEach((item) => names.add(item))
  if (type === 'postgresql') add(['POSTGRES_HOST', 'POSTGRES_PORT', 'POSTGRES_USERNAME', 'POSTGRES_DATABASE', 'POSTGRES_PASSWORD', 'SPRING_DATASOURCE_PASSWORD'])
  if (type === 'mysql') add(['MYSQL_HOST', 'MYSQL_PORT', 'MYSQL_USERNAME', 'MYSQL_DATABASE', 'MYSQL_PASSWORD', 'SPRING_DATASOURCE_PASSWORD'])
  if (type === 'redis') add(['REDIS_HOST', 'REDIS_PORT', 'REDIS_PASSWORD', 'REDIS_SENTINEL_HOST', 'REDIS_SENTINEL_PORT', 'REDIS_SENTINEL_MASTER_NAME'])
  if (type === 'mongodb') add(['MONGODB_HOST', 'MONGODB_PORT', 'MONGODB_USERNAME', 'MONGODB_DATABASE', 'MONGODB_PASSWORD', 'MONGODB_URL'])
  if (type === 'rabbitmq') add(['RABBITMQ_HOST', 'RABBITMQ_PORT', 'RABBITMQ_USERNAME', 'RABBITMQ_PASSWORD', 'RABBITMQ_URL'])
  if (type === 'eureka') add(['EUREKA_URL'])
  if (type === 'kafka') add(['KAFKA_BROKERS', 'KAFKA_BOOTSTRAP_SERVERS', 'KAFKA_SECURITY_PROTOCOL'])
  if (type === 'minio') add(['MINIO_ENDPOINT', 'MINIO_ACCESS_KEY', 'MINIO_SECRET_KEY', 'S3_ENDPOINT', 'AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY'])
  return names
}
const managedEnvKindLabel = (name:string, source?:string) => {
  const upper = String(name || '').toUpperCase()
  if (source === 'secret' || upper.includes('PASSWORD') || upper.includes('SECRET') || upper.includes('TOKEN') || upper.includes('ACCESS_KEY')) return '敏感信息'
  if (upper.includes('HOST') || upper.includes('ENDPOINT') || upper.includes('BROKERS') || upper.includes('URL')) return '连接地址'
  if (upper.includes('PORT')) return '端口'
  if (upper.includes('USERNAME') || upper.includes('USER')) return '用户名'
  if (upper.includes('DATABASE')) return '数据库'
  return '连接参数'
}
const managedEnvLabel = (binding:any, envItem:ComponentConfigEnvRow) => {
  const target = String(binding?.targetName || '').trim()
  const kind = managedEnvKindLabel(envItem.name, envItem.source)
  return target ? `${target} · ${kind}` : kind
}
const markManagedConfigEnvRows = (rows:ComponentConfigEnvRow[], bindings:any[]) => {
  const managedByName = new Map<string, any>()
  for (const binding of bindings || []) {
    if (String(binding?.targetKind || '').toLowerCase() !== 'service') continue
    for (const name of managedConnectionEnvNames(binding)) {
      if (name) managedByName.set(name, binding)
    }
  }
  return rows.map((row) => {
    const binding = managedByName.get(String(row.name || '').trim())
    if (!binding) return row
    return { ...row, managed: true, managedLabel: managedEnvLabel(binding, row) }
  })
}
const managedEnvDisplay = (envItem:ComponentConfigEnvRow) => envItem.managedLabel || `由平台注入${managedEnvKindLabel(envItem.name, envItem.source)}`
const clearManagedConnectionEnvForTarget = (target:any) => {
  if (!target?.key) return
  const names = new Set<string>()
  for (const binding of configForm.value.bindings || []) {
    if (binding.targetKey !== target.key) continue
    for (const name of managedConnectionEnvNames(binding)) names.add(name)
    for (const name of Object.keys(binding.generated || {})) names.add(name)
    if (['postgresql', 'mysql'].includes(String(binding.targetType || '').toLowerCase())) names.add('SPRING_DATASOURCE_PASSWORD')
  }
  if (!names.size) return
  configForm.value.env = configForm.value.env.filter((row:any) => !names.has(String(row.name || '').trim()) || !row.managed)
}
const linesFromArray = (items:any) => Array.isArray(items) ? items.map((item:any) => String(item).trim()).filter(Boolean).join('\n') : ''
const arrayFromLines = (text:string) => String(text || '').split('\n').map(item => item.trim()).filter(Boolean)
const addConfigEnv = () => {
  configForm.value.env.push({ name: '', source: 'value', value: '', refName: '', refKey: '' })
}
const removeConfigEnv = (idx:number) => {
  configForm.value.env.splice(idx, 1)
}
const normalizeConfigEnvSource = (envItem:ComponentConfigEnvRow) => {
  if (envItem.source === 'secret') {
    envItem.value = ''
    envItem.refName = envItem.refName || componentGeneratedSecretName()
    envItem.refKey = envItem.refKey || envItem.name
    return
  }
  if (envItem.source === 'configMap') {
    envItem.value = ''
    if (envItem.refName === componentGeneratedSecretName()) envItem.refName = ''
    envItem.refName = envItem.refName || componentGeneratedConfigMapName()
    envItem.refKey = envItem.refKey || envItem.name
    return
  }
  envItem.refName = ''
  envItem.refKey = ''
}
const componentTemplateTokenContext = () => ({
  componentName: componentGeneratedBaseName(),
  configMapName: componentGeneratedConfigMapName(),
  secretName: componentGeneratedSecretName(),
})
const componentTemplateFieldByKey = (key:string) =>
  componentTemplateFields.value.find((field:any) => componentTemplateFieldKey(field) === key) || null
const componentTemplateTargetRenderedValue = (field:any, target:any, credentials:any[] = []) => {
  const serviceType = String(target.type || '').toLowerCase()
  return componentTemplateRenderTargetValue(field, target, {
    credentials,
    endpoint: ['service', 'capability'].includes(String(target.kind || '')) ? connectionTargetEndpoint(target) : '',
    defaultPort: defaultServicePortForBinding(serviceType),
    credentialValue,
  })
}
const componentTemplateRenderFieldValue = (key:string, fieldValues:Record<string, any>, credentialsByTargetKey:Record<string, any[]> = {}) => {
  const raw = String(fieldValues[key] ?? '').trim()
  const field = componentTemplateFieldByKey(key)
  if (field && componentTemplateFieldType(field) === 'serviceref') {
    const target = componentTemplateTargetForValue(raw)
    if (target) return componentTemplateTargetRenderedValue(field, target, credentialsByTargetKey[target.key] || [])
  }
  return raw
}
const renderComponentTemplateValue = (value:string, context = componentTemplateTokenContext(), fieldValues:Record<string, any> = {}, credentialsByTargetKey:Record<string, any[]> = {}) =>
  renderPaapTemplateValue(value, {
    context,
    fieldValues,
    resolveFieldValue: (key, values) => {
      const rendered = componentTemplateRenderFieldValue(String(key), values, credentialsByTargetKey)
      return rendered || templatePlaceholderDefault(String(key), '')
    },
  })
const componentTemplateInlineHelp = (template:UserComponentConfigTemplate) =>
  String(template?.description || '下方已按模板抽取出需要填写的配置项。')
    .replace(/ConfigMap/g, '普通配置')
    .replace(/Secret keys/gi, '敏感配置项')
    .replace(/Secret/g, '敏感配置')
const componentTemplateOptionValue = (template:UserComponentConfigTemplate) => componentConfigTemplateSelectValue(template)
const componentConfigTemplateSelectionFromConfig = (cfg:any) => {
  const id = Number(cfg?.configTemplateId || cfg?.configTemplate?.id || 0)
  if (id > 0) return String(id)
  const key = String(cfg?.configTemplateKey || cfg?.configTemplate?.key || '').trim()
  if (key) return key
  const name = String(cfg?.configTemplateName || cfg?.configTemplate?.name || '').trim()
  if (name) return name
  return ''
}
const componentConfigTemplateSelectionPayload = () => {
  const template = selectedComponentConfigTemplate.value
  if (!template) return {
    configTemplateId: 0,
    configTemplateKey: '',
    configTemplateName: '',
    configTemplate: null,
  }
  const id = Number(template.id || 0)
  const key = String(template.key || '').trim()
  return {
    configTemplateId: id,
    configTemplateKey: key,
    configTemplateName: String(template.name || ''),
    configTemplate: {
      id,
      key,
      name: String(template.name || ''),
    },
  }
}
const componentTemplateMatchesCurrentComponent = (template:UserComponentConfigTemplate) => {
  return componentConfigTemplateMatchesComponent(template, {
    componentType: componentDrawerType.value,
    framework: configForm.value.framework,
    componentName: String(configDrawer.value.component?.name || ''),
  })
}
const componentSelectableConfigTemplates = computed(() =>
  componentUserConfigTemplates.value.filter((template) => componentTemplateMatchesCurrentComponent(template))
)
const selectedComponentConfigTemplate = computed(() =>
  componentSelectableConfigTemplates.value.find((template) => {
    const selected = String(selectedComponentConfigTemplateId.value || '').trim()
    return componentConfigTemplateMatchesSelection(template, selected)
  }) || null
)
const componentTemplateFields = computed(() =>
  Array.isArray(selectedComponentConfigTemplate.value?.fields)
    ? selectedComponentConfigTemplate.value.fields.filter((field:any) => componentTemplateFieldKey(field))
    : []
)
const componentTemplateSelectedServiceTargetForGroup = (field:any) => {
  const group = componentTemplateFieldKey(field).split('.')[0]
  const serviceRefField = componentTemplateFields.value.find((candidate:any) =>
    componentTemplateFieldMatchesServiceRef(field, candidate)
  ) || componentTemplateFields.value.find((candidate:any) =>
    componentTemplateFieldType(candidate) === 'serviceref'
    && componentTemplateFieldKey(candidate).startsWith(`${group}.`)
  )
  if (!serviceRefField) return null
  const target = componentTemplateTargetForValue(componentTemplateFieldValues.value[componentTemplateFieldKey(serviceRefField)])
  return ['service', 'capability'].includes(String(target?.kind || '')) ? target : null
}
const componentTemplateFieldAutofillsFromService = (field:any) =>
  componentTemplateFieldType(field) === 'password' && Boolean(componentTemplateSelectedServiceTargetForGroup(field))
const componentTemplateFieldRequiredForUser = (field:any) =>
  componentTemplateFieldRequired(field) && !componentTemplateFieldAutofillsFromService(field)
const componentTemplateFieldOptions = (field:any) => {
  if (componentTemplateFieldType(field) !== 'serviceref') return []
  const targets = componentTemplateFieldTargetTokens(field)
  const matchesBackend = targets.includes('backend')
  const candidates = matchesBackend
    ? componentDrawerBackendTargets.value
    : componentConnectionTargets.value.filter((target:any) => {
      if (!['service', 'capability'].includes(String(target.kind || ''))) return false
      if (!targets.length) return true
      return componentTemplateServiceTypeMatchesTargets(String(target.type || ''), targets)
    })
  return candidates.map((target:any) => ({
    value: target.key,
    label: targetOptionLabel(target),
    target,
  }))
}
const componentTemplateFieldUsesTargetSelect = (field:any) =>
  componentTemplateFieldType(field) === 'serviceref' && componentTemplateFieldOptions(field).length > 0
const componentTemplateFieldPlaceholder = (field:any) => {
  const type = componentTemplateFieldType(field)
  if (type === 'serviceref') {
    const targets = componentTemplateFieldTargetTokens(field)
    if (targets.includes('backend')) return componentTemplateFieldOptions(field).length ? '选择后端组件' : '暂无后端组件，可输入地址'
    if (targets.includes('redis')) return componentTemplateFieldOptions(field).length ? '选择 Redis' : '暂无 Redis，可输入地址'
    if (targets.some((item) => ['postgresql', 'mysql', 'mongodb'].includes(item))) return componentTemplateFieldOptions(field).length ? '选择数据库' : '暂无数据库，可输入地址'
    return componentTemplateFieldOptions(field).length ? '选择服务' : '暂无可选服务，可输入地址'
  }
  if (type === 'password' && componentTemplateFieldAutofillsFromService(field)) return '留空时使用所选服务的密码'
  return componentTemplateFieldDefaultValue(field) || componentTemplateFieldLabel(field)
}
const componentTemplateFieldHint = (field:any) => {
  if (field?.description) return String(field.description)
  if (componentTemplateFieldAutofillsFromService(field)) return '留空时平台会读取所选服务的连接凭据。'
  if (componentTemplateFieldType(field) === 'serviceref' && !componentTemplateFieldOptions(field).length) {
    return '画布上有匹配服务时会自动提供下拉选择。'
  }
  return ''
}
const componentTemplateTargetForValue = (value:string) =>
  componentConnectionTargets.value.find((target:any) => target.key === value) || null
const componentTemplateExistingTargetKey = (field:any) => {
  const targets = componentTemplateFieldTargetTokens(field)
  const binding = configForm.value.bindings.find((item:any) => {
    const targetType = String(item.targetType || '').toLowerCase()
    if (targets.includes('backend')) return item.role === 'backend' || targetType === 'backend'
    return targets.includes(targetType)
  })
  return binding?.targetKey || ''
}
const componentTemplateDefaultListRow = (field:any) => {
  const row: Record<string, string> = {}
  for (const itemField of componentTemplateListItemFields(field)) {
    const key = componentTemplateFieldKey(itemField)
    const options = componentTemplateFieldOptions(itemField)
    row[key] = componentTemplateFieldType(itemField) === 'serviceref' && options.length > 0
      ? options[0].value
      : componentTemplateFieldDefaultValue(itemField)
  }
  return row
}
const componentTemplateInitialFieldValue = (field:any) => {
  const existingValue = componentTemplateExistingFieldValue(field, {
    env: configForm.value.env,
    secrets: configForm.value.secrets,
    configMaps: configForm.value.configMaps,
  })
  if (existingValue && componentTemplateFieldType(field) !== 'serviceref') return existingValue
  if (componentTemplateFieldType(field) === 'list') return [componentTemplateDefaultListRow(field)]
  return templateInitialFieldValue(field, {
    existingTargetKey: componentTemplateExistingTargetKey(field),
    firstOptionValue: componentTemplateFieldOptions(field)[0]?.value || '',
  })
}
const componentTemplateNginxRouteField = () =>
  componentTemplateFields.value.find((field:any) => nginxTemplateListFieldSupportsRoutes(field)) || null
const syncNginxTemplateFieldValuesFromRouteRows = () => {
  const field = componentTemplateNginxRouteField()
  if (!field || !nginxRouteRows.value.length) return
  const key = componentTemplateFieldKey(field)
  const currentRows = Array.isArray(componentTemplateFieldValues.value[key]) ? componentTemplateFieldValues.value[key] : []
  const hasCurrentRoute = nginxTemplateListRowsToRouteRows(currentRows, field, componentDrawerBackendTargets.value)
    .some((route) => String(route.path || '').trim())
  if (hasCurrentRoute) return
  const rows = nginxRouteRowsToTemplateListRows(nginxRouteRows.value, field)
  if (rows.length) componentTemplateFieldValues.value = { ...componentTemplateFieldValues.value, [key]: rows }
}
const syncNginxRouteRowsFromTemplateFieldValues = () => {
  const field = componentTemplateNginxRouteField()
  if (!field) return
  const key = componentTemplateFieldKey(field)
  const rows = nginxTemplateListRowsToRouteRows(
    Array.isArray(componentTemplateFieldValues.value[key]) ? componentTemplateFieldValues.value[key] : [],
    field,
    componentDrawerBackendTargets.value,
  )
  if (rows.length) nginxRouteRows.value = rows
}
const initializeComponentTemplateFieldValues = (force = false) => {
  const next: Record<string, any> = {}
  for (const field of componentTemplateFields.value) {
    const key = componentTemplateFieldKey(field)
    const current = componentTemplateFieldValues.value[key]
    next[key] = !force && current !== undefined ? current : componentTemplateInitialFieldValue(field)
  }
  componentTemplateFieldValues.value = next
  syncNginxTemplateFieldValuesFromRouteRows()
}
const selectRecommendedComponentConfigTemplate = () => {
  const current = componentSelectableConfigTemplates.value.find((template) => componentConfigTemplateMatchesSelection(template, selectedComponentConfigTemplateId.value))
  if (current) {
    selectedComponentConfigTemplateId.value = componentTemplateOptionValue(current)
    initializeComponentTemplateFieldValues(false)
    return
  }
  if (selectedComponentConfigTemplateId.value && (componentConfigTemplatesLoading.value || componentSelectableConfigTemplates.value.length === 0)) return
  const recommended = [...componentSelectableConfigTemplates.value]
    .map((template) => ({
      template,
      score: componentConfigTemplateRecommendationScore(template, {
        componentType: componentDrawerType.value,
        framework: configForm.value.framework,
        componentName: String(configDrawer.value.component?.name || ''),
      }),
    }))
    .filter((item) => item.score >= 0)
    .sort((a, b) => b.score - a.score || String(a.template.name || '').localeCompare(String(b.template.name || '')))[0]?.template
  if (recommended && !selectedComponentConfigTemplateId.value) {
    selectedComponentConfigTemplateId.value = componentTemplateOptionValue(recommended)
    initializeComponentTemplateFieldValues(false)
    return
  }
  selectedComponentConfigTemplateId.value = ''
  componentTemplateFieldValues.value = {}
}
const componentTemplateRequiredFieldsComplete = computed(() =>
  templateRequiredFieldsComplete(componentTemplateFields.value, componentTemplateFieldValues.value, {
    isRequiredForUser: componentTemplateFieldRequiredForUser,
  })
)
const componentAdvancedConfigOpenByDefault = computed(() =>
  !selectedComponentConfigTemplate.value
  && Boolean(configForm.value.files.length || configForm.value.env.length || componentNginxRouteEditorVisible.value)
)
const componentCurrentConfigRows = computed(() => {
  const rows: Array<{ key: string; name: string; source: string; value: string }> = []
  configForm.value.env.forEach((item:any, idx:number) => {
    const name = String(item?.name || '').trim()
    if (!name) return
    rows.push({
      key: `env:${name}:${idx}`,
      name,
      source: configEnvSourceLabel(item),
      value: configEnvDisplayValue(item),
    })
  })
  configForm.value.files.forEach((item:any, idx:number) => {
    const name = String(item?.key || item?.name || '').trim()
    if (!name) return
    rows.push({
      key: `file:${name}:${idx}`,
      name,
      source: '配置文件',
      value: `${item.configMapName || 'ConfigMap'} -> ${item.mountPath || '未设置挂载路径'}`,
    })
  })
  configForm.value.configMaps.forEach((item:any, idx:number) => {
    const keys = Object.keys(item?.data || {}).filter(Boolean)
    if (!String(item?.name || '').trim() || !keys.length) return
    rows.push({
      key: `configmap:${item.name}:${idx}`,
      name: String(item.name),
      source: '应用配置',
      value: keys.join(', '),
    })
  })
  configForm.value.secrets.forEach((item:any, idx:number) => {
    const keys = Object.keys(item?.data || {}).filter(Boolean)
    if (!String(item?.name || '').trim() || !keys.length) return
    rows.push({
      key: `secret:${item.name}:${idx}`,
      name: String(item.name),
      source: '敏感配置',
      value: keys.join(', '),
    })
  })
  return rows
})
const configEnvSourceLabel = (item:any) => {
  if (item?.source === 'secret') return '敏感项'
  if (item?.source === 'configMap') return '应用配置'
  return '环境变量'
}
const configEnvDisplayValue = (item:any) => {
  const name = String(item?.name || '').trim()
  if (item?.source === 'secret') {
    const refName = String(item?.refName || componentGeneratedSecretName()).trim()
    const refKey = String(item?.refKey || name).trim()
    return `${refName}/${refKey}`
  }
  if (item?.source === 'configMap') {
    const refName = String(item?.refName || componentGeneratedConfigMapName()).trim()
    const refKey = String(item?.refKey || name).trim()
    return `${refName}/${refKey}`
  }
  const value = String(item?.value || '').trim()
  return value || '空值'
}
const componentTemplateServiceRefSignature = computed(() =>
  componentTemplateFields.value
    .filter((field:any) => componentTemplateFieldType(field) === 'serviceref')
    .map((field:any) => `${componentTemplateFieldKey(field)}=${componentTemplateFieldValues.value[componentTemplateFieldKey(field)] || ''}`)
    .join('|')
)
const autofillComponentTemplateServicePasswords = async () => {
  if (!configDrawer.value.visible || configDrawer.value.kind !== 'component') return
  const next = { ...componentTemplateFieldValues.value }
  const credentialsByTargetKey: Record<string, any[]> = {}
  let changed = false
  for (const field of componentTemplateFields.value) {
    if (componentTemplateFieldType(field) !== 'serviceref') continue
    const target = componentTemplateTargetForValue(next[componentTemplateFieldKey(field)])
    if (!target || !['service', 'capability'].includes(String(target.kind || ''))) continue
    const serviceType = String(target.type || '').toLowerCase()
    const passwordKeys = componentTemplateServicePasswordFieldKeys(componentTemplateFields.value, serviceType)
    if (!passwordKeys.some((key) => !String(next[key] || '').trim())) continue
    const credentials = await componentTemplateCredentialsForTarget(target, credentialsByTargetKey)
    const password = credentialValue(credentials, componentTemplateCredentialPasswordKeys(serviceType))
    if (!password) continue
    for (const passwordKey of passwordKeys) {
      if (String(next[passwordKey] || '').trim()) continue
      next[passwordKey] = password
      changed = true
    }
  }
  if (changed) componentTemplateFieldValues.value = next
}
const applySelectedComponentConfigTemplate = (options: { silent?: boolean } = {}) => {
  if (!selectedComponentConfigTemplate.value) return
  return applyUserComponentConfigTemplate(selectedComponentConfigTemplate.value, options)
}
watch(() => selectedComponentConfigTemplate.value ? componentTemplateOptionValue(selectedComponentConfigTemplate.value) : '', () => {
  initializeComponentTemplateFieldValues(true)
  void autofillComponentTemplateServicePasswords()
})
watch([componentSelectableConfigTemplates, () => configDrawer.value.visible, () => configDrawer.value.kind], () => {
  if (configDrawer.value.visible && configDrawer.value.kind === 'component') {
    selectedComponentConfigTemplateId.value = resolveComponentConfigTemplateSelection(componentSelectableConfigTemplates.value, selectedComponentConfigTemplateId.value)
    selectRecommendedComponentConfigTemplate()
    void autofillComponentTemplateServicePasswords()
  }
})
watch(componentTemplateServiceRefSignature, () => {
  void autofillComponentTemplateServicePasswords()
})
const componentConnectionTargetPriority = (target:any) => {
  if (target?.kind === 'service') return 10
  const source = String(target?.source || target?.capability?.source || '').toLowerCase()
  if (source === 'shared') return 20
  if (source === 'external') return 30
  return 40
}
const componentConnectionTargets = computed(() => {
  const currentId = Number(configDrawer.value.component?.id)
  const componentTargets = components.value
    .filter((comp:any) => Number(comp.id) !== currentId)
    .map((comp:any) => ({
      key: `component:${comp.id}`,
      kind: 'component',
      name: comp.name || `component-${comp.id}`,
      type: comp.type || 'custom',
      serviceName: componentRuntimeIdentifier(comp) || comp.name,
      component: comp,
    }))
  const serviceTargets = services.value
    .map((svc:any) => ({
      key: `service:${svc.id}`,
      kind: 'service',
      source: 'managed',
      name: svc.serviceName || svc.name || svc.serviceType,
      type: serviceProductKey(svc) || svc.serviceType || svc.type || 'service',
      namespace: svc.namespace || '',
      service: svc,
    }))
  const capabilityTargets = environmentCapabilities.value
    .flatMap((cap:any) => {
      const refService = cap.refService || {}
      const type = String(cap.serviceType || cap.provider || refService.serviceType || cap.capability || '').toLowerCase()
      if (!type) return []
      return [{
        key: `capability:${cap.id}`,
        kind: 'capability',
        source: String(cap.source || '').toLowerCase(),
        name: refService.serviceName || cap.provider || capabilityLabel(cap.capability || '') || type,
        type,
        namespace: refService.namespace || '',
        service: refService,
        capability: cap,
      }]
    })
    .sort((a:any, b:any) => componentConnectionTargetPriority(a) - componentConnectionTargetPriority(b))
  return [...componentTargets, ...serviceTargets, ...capabilityTargets]
})
const selectedConnectionTarget = computed(() =>
  componentConnectionTargets.value.find((item:any) => item.key === configForm.value.bindingTargetKey) || null
)
const runComponentDrawerSuggestion = (key: string) => {
  if (key === 'proxy-route') {
    configDrawerTab.value = 'variables'
    configForm.value.framework = 'nginx'
    if (!nginxRouteRows.value.length) addNginxRoute()
    return
  }
  if (key === 'select-dependency') {
    configDrawerTab.value = 'variables'
    configForm.value.bindingTargetKey = componentDrawerDependencyTargets.value[0]?.key || ''
    return
  }
  if (key === 'springboot-config-file') {
    configForm.value.framework = 'springboot'
    configForm.value.bindingMode = 'springboot-file'
    syncGeneratedSpringConfig()
  }
}
const configFrameworkOptions = componentFrameworkOptions
const targetTypeLabel = (target:any) => target?.kind === 'component' ? compTypeText(target.type) : typeLabel(target?.type || '')
const targetSourceLabel = (target:any) => {
  if (target?.kind === 'component') return '应用组件'
  const source = String(target?.source || target?.capability?.source || '').toLowerCase()
  if (target?.kind === 'service' || source === 'managed') return '本环境'
  if (source === 'shared') return '共享资源'
  if (source === 'external') return '外部资源'
  return '服务'
}
const targetOptionMeta = (target:any) => {
  if (target?.kind === 'component') return target.serviceName || ''
  const cap = target?.capability || {}
  if (String(target?.source || cap.source || '').toLowerCase() === 'external') {
    return externalEndpointHost(cap.externalEndpoint || '')
  }
  return target.namespace || target.service?.namespace || ''
}
const targetOptionLabel = (target:any) => [
  targetSourceLabel(target),
  target.name,
  targetTypeLabel(target) || '服务',
  targetOptionMeta(target),
].filter(Boolean).join(' · ')
const applySelectedConfigBinding = async () => {
  const target = selectedConnectionTarget.value
  const comp = configDrawer.value.component
  if (!target || !comp) {
    configDrawer.value.error = '请选择要连接的组件或中间件。'
    return
  }
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  try {
    if (target.kind === 'component') {
      applyComponentTargetBinding(target)
    } else {
      await applyServiceTargetBinding(target)
    }
    syncGeneratedSpringConfig()
  } catch (e:any) {
    configDrawer.value.error = '生成连接配置失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const applyComponentTargetBinding = (target:any) => {
  const role = String(target.type || '').toLowerCase() === 'backend' ? 'backend' : 'component'
  const value = `http://${target.serviceName || target.name}`
  if (role === 'backend' && configForm.value.framework === 'nginx') {
    ensureNginxRouteForTarget(target, value)
    configDrawer.value.message = '已选择转发目标，请填写匹配路径后更新代理配置。'
    return
  }
  const envName = role === 'backend' && String(configDrawer.value.component?.type || '').toLowerCase() === 'frontend'
    ? 'BACKEND_URL'
    : `${envPrefix(target.name)}_URL`
  upsertEnvValue(envName, value)
  upsertBinding({
    targetKey: target.key,
    targetKind: 'component',
    targetName: target.name,
    targetType: target.type,
    role,
    mode: 'env',
    confidence: 'high',
    source: 'user',
    generated: { [envName]: value },
  })
}
const managedEnvOptionsForTarget = (target:any, name:string, source: ComponentConfigEnvRow['source'] = 'value'): Partial<ComponentConfigEnvRow> => ({
  managed: true,
  managedLabel: managedEnvLabel({ targetName: target.name, targetType: target.type }, { name, source, value: '', refName: '', refKey: '' }),
})
const applyServiceTargetBinding = async (target:any) => {
  const serviceType = String(target.type || '').toLowerCase()
  const mode = resolvedBindingMode(target)
  const endpoint = connectionTargetEndpoint(target)
  const [host, portText] = splitEndpoint(endpoint, defaultServicePortForBinding(serviceType))
  const credentials = await connectionTargetCredentials(target)
  const secretName = componentGeneratedSecretName()
  const generated: Record<string, string> = {}
  clearManagedConnectionEnvForTarget(target)

  if (serviceType === 'postgresql' || serviceType === 'mysql') {
    const prefix = serviceType === 'mysql' ? 'MYSQL' : 'POSTGRES'
    const passwordKey = `${prefix}_PASSWORD`
    const springPasswordKey = 'SPRING_DATASOURCE_PASSWORD'
    const username = serviceType === 'mysql' ? 'root' : 'postgres'
    const database = serviceType === 'mysql' ? 'mysql' : 'postgres'
    const password = credentialValue(credentials, serviceType === 'mysql' ? ['mysql-root-password', 'mysql-password', 'password'] : ['postgres-password', 'password'])
    if (mode === 'springboot-file') {
      upsertSecretValue(secretName, springPasswordKey, password)
      upsertEnvSecret(springPasswordKey, secretName, springPasswordKey, managedEnvOptionsForTarget(target, springPasswordKey, 'secret'))
      generated[`${prefix}_HOST`] = host
      generated[`${prefix}_PORT`] = String(portText)
      generated[`${prefix}_USERNAME`] = username
      generated[`${prefix}_DATABASE`] = database
      generated[springPasswordKey] = `${secretName}/${springPasswordKey}`
    } else {
      upsertSecretValue(secretName, passwordKey, password)
      upsertEnvSecret(passwordKey, secretName, passwordKey, managedEnvOptionsForTarget(target, passwordKey, 'secret'))
      upsertEnvValue(`${prefix}_HOST`, host, managedEnvOptionsForTarget(target, `${prefix}_HOST`))
      upsertEnvValue(`${prefix}_PORT`, String(portText), managedEnvOptionsForTarget(target, `${prefix}_PORT`))
      upsertEnvValue(`${prefix}_USERNAME`, username, managedEnvOptionsForTarget(target, `${prefix}_USERNAME`))
      upsertEnvValue(`${prefix}_DATABASE`, database, managedEnvOptionsForTarget(target, `${prefix}_DATABASE`))
      generated[`${prefix}_HOST`] = host
      generated[`${prefix}_PORT`] = String(portText)
      generated[`${prefix}_USERNAME`] = username
      generated[`${prefix}_DATABASE`] = database
      generated[passwordKey] = `${secretName}/${passwordKey}`
    }
  } else if (serviceType === 'redis') {
    const passwordKey = 'REDIS_PASSWORD'
    const password = credentialValue(credentials, ['redis-password', 'password'])
    const redisValues = serviceConfigValues(target.service || {})
    const sentinel = redisValues['sentinel.enabled'] === 'true'
    const masterSet = redisValues['sentinel.masterSet'] || 'mymaster'
    upsertSecretValue(secretName, passwordKey, password)
    upsertEnvSecret(passwordKey, secretName, passwordKey, managedEnvOptionsForTarget(target, passwordKey, 'secret'))
    if (mode === 'springboot-file') {
      generated.REDIS_HOST = host
      generated.REDIS_PORT = String(portText)
      generated.REDIS_PASSWORD = `${secretName}/${passwordKey}`
      if (sentinel) {
        generated.REDIS_SENTINEL_HOST = host
        generated.REDIS_SENTINEL_PORT = '26379'
        generated.REDIS_SENTINEL_MASTER_NAME = masterSet
      }
    } else {
      upsertEnvValue('REDIS_HOST', host, managedEnvOptionsForTarget(target, 'REDIS_HOST'))
      upsertEnvValue('REDIS_PORT', String(portText), managedEnvOptionsForTarget(target, 'REDIS_PORT'))
      generated.REDIS_HOST = host
      generated.REDIS_PORT = String(portText)
      generated.REDIS_PASSWORD = `${secretName}/${passwordKey}`
      if (sentinel) {
        upsertEnvValue('REDIS_SENTINEL_HOST', host, managedEnvOptionsForTarget(target, 'REDIS_SENTINEL_HOST'))
        upsertEnvValue('REDIS_SENTINEL_PORT', '26379', managedEnvOptionsForTarget(target, 'REDIS_SENTINEL_PORT'))
        upsertEnvValue('REDIS_SENTINEL_MASTER_NAME', masterSet, managedEnvOptionsForTarget(target, 'REDIS_SENTINEL_MASTER_NAME'))
        generated.REDIS_SENTINEL_HOST = host
        generated.REDIS_SENTINEL_PORT = '26379'
        generated.REDIS_SENTINEL_MASTER_NAME = masterSet
      }
    }
  } else if (serviceType === 'mongodb') {
    const passwordKey = 'MONGODB_PASSWORD'
    const username = credentialValue(credentials, ['mongodb-root-user', 'mongodb-username', 'username']) || 'root'
    const database = 'admin'
    const password = credentialValue(credentials, ['mongodb-root-password', 'mongodb-password', 'password'])
    upsertSecretValue(secretName, passwordKey, password)
    upsertEnvSecret(passwordKey, secretName, passwordKey, managedEnvOptionsForTarget(target, passwordKey, 'secret'))
    upsertEnvValue('MONGODB_HOST', host, managedEnvOptionsForTarget(target, 'MONGODB_HOST'))
    upsertEnvValue('MONGODB_PORT', String(portText), managedEnvOptionsForTarget(target, 'MONGODB_PORT'))
    upsertEnvValue('MONGODB_USERNAME', username, managedEnvOptionsForTarget(target, 'MONGODB_USERNAME'))
    upsertEnvValue('MONGODB_DATABASE', database, managedEnvOptionsForTarget(target, 'MONGODB_DATABASE'))
    upsertEnvValue('MONGODB_URL', `mongodb://${username}:$(MONGODB_PASSWORD)@${host}:${portText}/${database}`, managedEnvOptionsForTarget(target, 'MONGODB_URL'))
    generated.MONGODB_HOST = host
    generated.MONGODB_PORT = String(portText)
    generated.MONGODB_USERNAME = username
    generated.MONGODB_DATABASE = database
    generated.MONGODB_PASSWORD = `${secretName}/${passwordKey}`
    generated.MONGODB_URL = 'uses MONGODB_PASSWORD'
  } else if (serviceType === 'rabbitmq') {
    const username = credentialValue(credentials, ['rabbitmq-username', 'username']) || 'user'
    const passwordKey = 'RABBITMQ_PASSWORD'
    const password = credentialValue(credentials, ['rabbitmq-password', 'password'])
    upsertSecretValue(secretName, passwordKey, password)
    upsertEnvSecret(passwordKey, secretName, passwordKey, managedEnvOptionsForTarget(target, passwordKey, 'secret'))
    upsertEnvValue('RABBITMQ_HOST', host, managedEnvOptionsForTarget(target, 'RABBITMQ_HOST'))
    upsertEnvValue('RABBITMQ_PORT', String(portText), managedEnvOptionsForTarget(target, 'RABBITMQ_PORT'))
    upsertEnvValue('RABBITMQ_USERNAME', username, managedEnvOptionsForTarget(target, 'RABBITMQ_USERNAME'))
    upsertEnvValue('RABBITMQ_URL', `amqp://${username}:$(RABBITMQ_PASSWORD)@${host}:${portText}/`, managedEnvOptionsForTarget(target, 'RABBITMQ_URL'))
    generated.RABBITMQ_HOST = host
    generated.RABBITMQ_PORT = String(portText)
    generated.RABBITMQ_USERNAME = username
    generated.RABBITMQ_PASSWORD = `${secretName}/${passwordKey}`
    generated.RABBITMQ_URL = 'uses RABBITMQ_PASSWORD'
  } else if (serviceType === 'eureka') {
    const eurekaURL = `http://${host}:${portText || 8761}/eureka/`
    upsertEnvValue('EUREKA_URL', eurekaURL, managedEnvOptionsForTarget(target, 'EUREKA_URL'))
    generated.EUREKA_URL = eurekaURL
  } else if (serviceType === 'kafka') {
    const brokers = `${host}:${portText}`
    upsertEnvValue('KAFKA_BROKERS', brokers, managedEnvOptionsForTarget(target, 'KAFKA_BROKERS'))
    upsertEnvValue('KAFKA_BOOTSTRAP_SERVERS', brokers, managedEnvOptionsForTarget(target, 'KAFKA_BOOTSTRAP_SERVERS'))
    upsertEnvValue('KAFKA_SECURITY_PROTOCOL', 'PLAINTEXT', managedEnvOptionsForTarget(target, 'KAFKA_SECURITY_PROTOCOL'))
    generated.KAFKA_BROKERS = brokers
    generated.KAFKA_BOOTSTRAP_SERVERS = brokers
    generated.KAFKA_SECURITY_PROTOCOL = 'PLAINTEXT'
  } else if (serviceType === 'minio') {
    const accessKeyName = 'MINIO_ACCESS_KEY'
    const secretKeyName = 'MINIO_SECRET_KEY'
    const accessKey = credentialValue(credentials, ['root-user', 'access-key', 'accesskey', 'minio-access-key', 'username'])
    const secretKey = credentialValue(credentials, ['root-password', 'secret-key', 'secretkey', 'minio-secret-key', 'password'])
    upsertSecretValue(secretName, accessKeyName, accessKey)
    upsertSecretValue(secretName, secretKeyName, secretKey)
    upsertEnvSecret(accessKeyName, secretName, accessKeyName, managedEnvOptionsForTarget(target, accessKeyName, 'secret'))
    upsertEnvSecret(secretKeyName, secretName, secretKeyName, managedEnvOptionsForTarget(target, secretKeyName, 'secret'))
    upsertEnvSecret('AWS_ACCESS_KEY_ID', secretName, accessKeyName, managedEnvOptionsForTarget(target, 'AWS_ACCESS_KEY_ID', 'secret'))
    upsertEnvSecret('AWS_SECRET_ACCESS_KEY', secretName, secretKeyName, managedEnvOptionsForTarget(target, 'AWS_SECRET_ACCESS_KEY', 'secret'))
    upsertEnvValue('MINIO_ENDPOINT', `${host}:${portText}`, managedEnvOptionsForTarget(target, 'MINIO_ENDPOINT'))
    upsertEnvValue('S3_ENDPOINT', `http://${host}:${portText}`, managedEnvOptionsForTarget(target, 'S3_ENDPOINT'))
    generated.MINIO_ENDPOINT = `${host}:${portText}`
    generated.MINIO_ACCESS_KEY = `${secretName}/${accessKeyName}`
    generated.MINIO_SECRET_KEY = `${secretName}/${secretKeyName}`
    generated.S3_ENDPOINT = `http://${host}:${portText}`
  } else {
    const prefix = envPrefix(target.name || serviceType)
    upsertEnvValue(`${prefix}_HOST`, host, managedEnvOptionsForTarget(target, `${prefix}_HOST`))
    upsertEnvValue(`${prefix}_PORT`, String(portText), managedEnvOptionsForTarget(target, `${prefix}_PORT`))
    generated[`${prefix}_HOST`] = host
    generated[`${prefix}_PORT`] = String(portText)
  }

  upsertBinding({
    targetKey: target.key,
    targetKind: target.kind === 'capability' ? 'capability' : 'service',
    targetName: target.name,
    targetType: serviceType,
    role: serviceRole(serviceType),
    mode,
    confidence: mode === 'recommended' ? 'medium' : 'high',
    source: 'user',
    generated,
  })
}
const resolvedBindingMode = (target:any) => {
  const selected = configForm.value.bindingMode
  if (selected && selected !== 'recommended') return selected
  const serviceType = String(target?.type || '').toLowerCase()
  if (configForm.value.framework === 'springboot' && ['postgresql', 'mysql', 'redis'].includes(serviceType)) return 'springboot-file'
  return 'env'
}
const serviceCredentials = async (svc:any) => {
  if (!envId.value || !svc?.id) return []
  const res = await api.getServiceCredentials(envId.value, Number(svc.id))
  return Array.isArray(res?.data?.credentials) ? res.data.credentials : []
}
const connectionTargetEndpoint = (target:any) => {
  if (target?.kind === 'capability') {
    const cap = target.capability || {}
    if (cap.source === 'external') return String(cap.externalEndpoint || '')
    return serviceInternalEndpoint(target.service || cap.refService || {})
  }
  return serviceInternalEndpoint(target?.service || {})
}
const connectionTargetCredentials = async (target:any) => {
  if (!envId.value) return []
  if (target?.kind === 'capability') {
    const cap = target.capability || {}
    if (!cap.capability) return []
    const res = await api.getEnvironmentCapabilityCredentials(envId.value, capabilityRequestKey(cap))
    return Array.isArray(res?.data?.credentials) ? res.data.credentials : []
  }
  return serviceCredentials(target?.service)
}
const credentialValue = (credentials:any[], keys:string[]) => {
  for (const key of keys) {
    const match = credentials.find((item:any) => String(item.key || '').toLowerCase() === key)
    if (match?.value) return String(match.value)
  }
  return ''
}
const splitEndpoint = (endpoint:string, fallbackPort:number) => {
  return componentTemplateSplitEndpoint(endpoint, fallbackPort)
}
const defaultServicePortForBinding = (serviceType:string) => ({
  postgresql: 5432,
  mysql: 3306,
  redis: 6379,
  mongodb: 27017,
  rabbitmq: 5672,
  kafka: 9092,
  minio: 9000,
  eureka: 8761,
  log: 3100,
} as Record<string, number>)[serviceType] || 80
const serviceRole = (serviceType:string) => {
  if (['postgresql', 'mysql', 'mongodb'].includes(serviceType)) return 'database'
  if (serviceType === 'redis') return 'cache'
  if (['rabbitmq', 'kafka'].includes(serviceType)) return 'message-queue'
  if (serviceType === 'log') return 'log'
  return 'service'
}
const envPrefix = (value:string) => String(value || 'SERVICE').toUpperCase().replace(/[^A-Z0-9]+/g, '_').replace(/^_+|_+$/g, '') || 'SERVICE'
const componentGeneratedBaseName = () => {
  const raw = componentRuntimeIdentifier(configDrawer.value.component) || configDrawer.value.component?.name || `component-${configDrawer.value.component?.id || 'app'}`
  return String(raw).toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 46) || 'component'
}
const componentGeneratedSecretName = () => `${componentGeneratedBaseName()}-secret`
const componentGeneratedConfigMapName = () => `${componentGeneratedBaseName()}-config`
const uniqueGeneratedConfigName = (base:string, index:number, used:Set<string>) => {
  const cleanBase = String(base || componentGeneratedConfigMapName()).trim() || componentGeneratedConfigMapName()
  let candidate = index === 0 ? cleanBase : `${cleanBase}-${index + 1}`
  let suffix = index + 2
  while (used.has(candidate)) {
    candidate = `${cleanBase}-${suffix}`
    suffix += 1
  }
  used.add(candidate)
  return candidate
}
const resolveTemplateObjectName = (rawName:string, fallbackBase:string, index:number, used:Set<string>) => {
  const rendered = String(rawName || '').trim()
  if (!rendered || rendered === fallbackBase) return uniqueGeneratedConfigName(fallbackBase, index, used)
  if (!used.has(rendered)) {
    used.add(rendered)
    return rendered
  }
  return uniqueGeneratedConfigName(rendered, index, used)
}
const upsertEnvValue = (name:string, value:string, options: Partial<ComponentConfigEnvRow> = {}) => {
  const idx = configForm.value.env.findIndex((item:any) => item.name === name)
  const row = { name, source: 'value' as const, value, refName: '', refKey: '', ...options }
  if (idx >= 0) configForm.value.env.splice(idx, 1, row)
  else configForm.value.env.push(row)
}
const upsertEnvSecret = (name:string, secretName:string, secretKey:string, options: Partial<ComponentConfigEnvRow> = {}) => {
  const idx = configForm.value.env.findIndex((item:any) => item.name === name)
  const row = { name, source: 'secret' as const, value: '', refName: secretName, refKey: secretKey, ...options }
  if (idx >= 0) configForm.value.env.splice(idx, 1, row)
  else configForm.value.env.push(row)
}
const upsertSecretValue = (name:string, key:string, value:string) => {
  const idx = configForm.value.secrets.findIndex((item:any) => item.name === name)
  const current = idx >= 0 ? configForm.value.secrets[idx] : { name, data: {} as Record<string, string> }
  current.data = { ...current.data, [key]: value }
  if (idx >= 0) configForm.value.secrets.splice(idx, 1, current)
  else configForm.value.secrets.push(current)
}
const componentTemplateCredentialsForTarget = async (target:any, cache:Record<string, any[]>) => {
  if (!target?.key || !['service', 'capability'].includes(String(target.kind || ''))) return []
  if (cache[target.key]) return cache[target.key]
  const credentials = await connectionTargetCredentials(target)
  cache[target.key] = credentials
  return credentials
}
const buildComponentTemplateFieldRenderValues = async () => {
  const fieldValues = { ...componentTemplateFieldValues.value }
  const credentialsByTargetKey: Record<string, any[]> = {}
  for (const field of componentTemplateFields.value) {
    if (componentTemplateFieldType(field) === 'list') {
      const listKey = componentTemplateFieldKey(field)
      const rows = Array.isArray(fieldValues[listKey]) ? fieldValues[listKey] : []
      fieldValues[listKey] = await Promise.all(rows.map(async (row:any) => {
        const nextRow = { ...row }
        for (const itemField of componentTemplateListItemFields(field)) {
          if (componentTemplateFieldType(itemField) !== 'serviceref') continue
          const itemKey = componentTemplateFieldKey(itemField)
          const target = componentTemplateTargetForValue(nextRow[itemKey])
          if (!target) continue
          const credentials = await componentTemplateCredentialsForTarget(target, credentialsByTargetKey)
          nextRow[itemKey] = componentTemplateTargetRenderedValue(itemField, target, credentials)
        }
        return nextRow
      }))
      continue
    }
    if (componentTemplateFieldType(field) !== 'serviceref') continue
    const key = componentTemplateFieldKey(field)
    const target = componentTemplateTargetForValue(fieldValues[key])
    if (!target) continue
    const credentials = await componentTemplateCredentialsForTarget(target, credentialsByTargetKey)
    fieldValues[key] = componentTemplateTargetRenderedValue(field, target, credentials)

    const serviceType = String(target.type || '').toLowerCase()
    const password = credentialValue(credentials, componentTemplateCredentialPasswordKeys(serviceType))
    for (const passwordKey of componentTemplateServicePasswordFieldKeys(componentTemplateFields.value, serviceType)) {
      if (!fieldValues[passwordKey]) fieldValues[passwordKey] = password
    }
    const username = credentialValue(credentials, componentTemplateCredentialUsernameKeys(serviceType)) || componentTemplateDefaultCredentialUsername(serviceType)
    for (const usernameKey of componentTemplateServiceUsernameFieldKeys(componentTemplateFields.value, serviceType)) {
      if (!fieldValues[usernameKey]) fieldValues[usernameKey] = username
    }
    if (['postgresql', 'mysql', 'mongodb'].includes(serviceType)) {
      if (!fieldValues['database.username']) fieldValues['database.username'] = username
      if (!fieldValues['database.password']) fieldValues['database.password'] = password
    }
    if (serviceType === 'redis' && !fieldValues['redis.password']) {
      fieldValues['redis.password'] = password
    }
  }
  return { fieldValues, credentialsByTargetKey }
}
const applyComponentTemplateServiceRefs = async () => {
  const applied = new Set<string>()
  for (const field of componentTemplateFields.value) {
    if (componentTemplateFieldType(field) === 'list') {
      const rows = Array.isArray(componentTemplateFieldValues.value[componentTemplateFieldKey(field)])
        ? componentTemplateFieldValues.value[componentTemplateFieldKey(field)]
        : []
      for (const row of rows) {
        for (const itemField of componentTemplateListItemFields(field)) {
          if (componentTemplateFieldType(itemField) !== 'serviceref') continue
          const target = componentTemplateTargetForValue(row?.[componentTemplateFieldKey(itemField)])
          if (!target?.key || applied.has(target.key)) continue
          applied.add(target.key)
          if (['service', 'capability'].includes(String(target.kind || ''))) await applyServiceTargetBinding(target)
          else {
            upsertBinding({
              targetKey: target.key,
              targetKind: 'component',
              targetName: target.name,
              targetType: target.type,
              role: String(target.type || '').toLowerCase() === 'backend' ? 'backend' : 'component',
              mode: 'configMap',
              confidence: 'high',
              source: 'user',
              generated: { templateField: componentTemplateFieldKey(itemField) },
            })
          }
        }
      }
      continue
    }
    if (componentTemplateFieldType(field) !== 'serviceref') continue
    const target = componentTemplateTargetForValue(componentTemplateFieldValues.value[componentTemplateFieldKey(field)])
    if (!target?.key || applied.has(target.key)) continue
    applied.add(target.key)
    if (target.kind === 'component') applyComponentTargetBinding(target)
    else await applyServiceTargetBinding(target)
  }
}
const applyUserComponentConfigTemplate = async (template: UserComponentConfigTemplate, options: { silent?: boolean } = {}) => {
  if (!template) return
  const silent = Boolean(options.silent)
  const context = componentTemplateTokenContext()
  if (!silent) {
    configDrawer.value.saving = true
    configDrawer.value.error = ''
  }
  try {
    if (template.framework && template.framework !== 'auto') configForm.value.framework = template.framework
    if (template.bindingMode) configForm.value.bindingMode = template.bindingMode
    await applyComponentTemplateServiceRefs()
    const { fieldValues, credentialsByTargetKey } = await buildComponentTemplateFieldRenderValues()
    const render = (value:string) => renderComponentTemplateValue(value, context, fieldValues, credentialsByTargetKey)

    const usedConfigNames = new Set<string>()
    const configBlocks = (template.configMaps || []).map((item, idx) => {
      const renderedName = render(item.name)
      const name = resolveTemplateObjectName(renderedName, context.configMapName, idx, usedConfigNames)
      const keys = new Set(Object.keys(item.data || {}).map((key) => render(key)).filter(Boolean))
      return { item, name, keys, renderedName }
    })
    for (const block of configBlocks) {
      const { item, name } = block
      for (const [key, value] of Object.entries(item.data || {})) {
        const renderedKey = render(key)
        if (renderedKey) upsertConfigMapValue(name, renderedKey, render(String(value)))
      }
    }
    const usedSecretNames = new Set<string>()
    const secretBlocks = (template.secrets || []).map((item, idx) => {
      const renderedName = render(item.name)
      const name = resolveTemplateObjectName(renderedName, context.secretName, idx, usedSecretNames)
      const keys = new Set(Object.keys(item.data || {}).map((key) => render(key)).filter(Boolean))
      return { item, name, keys, renderedName }
    })
    for (const block of secretBlocks) {
      const { item, name } = block
      for (const [key, value] of Object.entries(item.data || {})) {
        const renderedKey = render(key)
        if (renderedKey) upsertSecretValue(name, renderedKey, render(String(value)))
      }
    }
    for (const item of template.files || []) {
      const renderedKey = render(item.key)
      const rawConfigMapName = String(item.configMapName || '')
      const renderedConfigName = render(item.configMapName)
      const platformGeneratedRef = !rawConfigMapName.trim() || /\{\{\s*configMapName\s*\}\}/.test(rawConfigMapName)
      const matchedBlock = platformGeneratedRef
        ? configBlocks.find((block) => renderedKey && block.keys.has(renderedKey))
        : configBlocks.find((block) => block.name === renderedConfigName || block.renderedName === renderedConfigName)
      const configMapName = matchedBlock?.name || renderedConfigName || configBlocks[0]?.name || context.configMapName
      const mountPath = componentTemplateFileMountPath({
        templateFile: item,
        configMapName,
        key: renderedKey,
        existingFiles: configForm.value.files,
        render,
      })
      upsertConfigFile({
        name: render(item.name),
        configMapName,
        key: renderedKey,
        mountPath,
        readOnly: item.readOnly !== false,
      })
    }
    for (const item of template.env || []) {
      const name = render(item.name)
      if (!name) continue
      if (item.source === 'secret') {
        const secretKey = render(item.refKey) || name
        const renderedSecretName = render(item.refName)
        const secretName = secretBlocks.find((block) => secretKey && block.keys.has(secretKey))?.name || renderedSecretName || secretBlocks[0]?.name || context.secretName
        if (item.value) upsertSecretValue(secretName, secretKey, render(item.value))
        upsertEnvSecret(name, secretName, secretKey)
      } else if (item.source === 'configMap') {
        const refKey = render(item.refKey) || name
        const renderedRefName = render(item.refName)
        const refName = configBlocks.find((block) => refKey && block.keys.has(refKey))?.name || renderedRefName || configBlocks[0]?.name || context.configMapName
        const idx = configForm.value.env.findIndex((envItem:any) => envItem.name === name)
        const row = { name, source: 'configMap' as const, value: '', refName, refKey }
        if (idx >= 0) configForm.value.env.splice(idx, 1, row)
        else configForm.value.env.push(row)
      } else {
        upsertEnvValue(name, render(item.value))
      }
    }
    if (template.command?.length) configForm.value.commandText = template.command.map((item) => render(item)).join('\n')
    if (template.args?.length) configForm.value.argsText = template.args.map((item) => render(item)).join('\n')
    syncGeneratedSpringConfig()
    if (configForm.value.framework === 'nginx') nginxRouteRows.value = nginxRoutesFromCurrentConfig()
    if (!silent) configDrawer.value.message = `已应用组件模板 ${template.name}。`
  } catch (e:any) {
    const message = '配置模板处理失败：' + (e?.message || '未知错误')
    if (!silent) configDrawer.value.error = message
    if (silent) throw new Error(message)
  } finally {
    if (!silent) configDrawer.value.saving = false
  }
}
const prepareSelectedComponentConfigTemplateForSave = async () => {
  if (!selectedComponentConfigTemplate.value) return true
  if (!componentTemplateRequiredFieldsComplete.value) {
    configDrawer.value.error = '请先补全配置模板必填项。'
    return false
  }
  await applySelectedComponentConfigTemplate({ silent: true })
  return true
}
const componentNginxRouteEditorVisible = computed(() =>
  configDrawer.value.kind === 'component'
  && (configForm.value.framework === 'nginx' || componentDrawerType.value === 'frontend')
)
const nginxTargetUrl = (target:any) => `http://${target?.serviceName || target?.name || 'backend'}`
const ensureNginxRouteForTarget = (target:any, value = '') => {
  if (!target?.key) return
  const existing = nginxRouteRows.value.find((route) => route.targetKey === target.key)
  if (existing) {
    existing.targetUrl = value || existing.targetUrl || nginxTargetUrl(target)
    return
  }
  nginxRouteRows.value.push({
    path: '',
    targetKey: target.key,
    targetUrl: value || nginxTargetUrl(target),
  })
}
const nginxRoutesFromCurrentConfig = (): NginxRouteRow[] => {
  return nginxRouteRowsFromComponentConfig({
    bindings: configForm.value.bindings,
    configMaps: configForm.value.configMaps,
    backendTargets: componentDrawerBackendTargets.value,
  })
}
const addNginxRoute = () => {
  nginxRouteRows.value.push({
    path: '',
    targetKey: '',
    targetUrl: '',
  })
}
const removeNginxRoute = (idx:number) => {
  nginxRouteRows.value.splice(idx, 1)
}
const syncNginxRouteTargetUrl = (route:NginxRouteRow) => {
  const selectedUrl = String(route.targetUrl || '').trim().replace(/\/+$/, '')
  const target = componentDrawerBackendTargets.value.find((item:any) => nginxTargetUrl(item).replace(/\/+$/, '') === selectedUrl)
  route.targetKey = target?.key || ''
}
const normalizeNginxRoutePath = (value:string) => {
  const raw = String(value || '').trim()
  if (!raw) return ''
  const withStart = raw.startsWith('/') ? raw : `/${raw}`
  return withStart
}
const normalizedNginxRoutes = () => nginxRouteRows.value
  .map((route) => ({
    path: normalizeNginxRoutePath(route.path),
    targetUrl: String(route.targetUrl || '').trim().replace(/\/+$/, ''),
    targetKey: route.targetKey,
  }))
  .filter((route) => route.path || route.targetUrl)
const applyNginxRoutes = (options: { showMessage?: boolean } = {}) => {
  const routes = normalizedNginxRoutes()
  if (!routes.length || routes.some((route) => !route.path || !route.targetUrl)) {
    configDrawer.value.error = '请填写代理路由的匹配路径和转发地址。'
    return false
  }
  configDrawer.value.error = ''
  configForm.value.framework = 'nginx'
  for (const [index, route] of routes.entries()) {
    const target = componentDrawerBackendTargets.value.find((item:any) => item.key === route.targetKey)
    if (target) {
      upsertBinding({
        targetKey: target.key,
        targetKind: 'component',
        targetName: target.name,
        targetType: target.type,
        role: 'backend',
        mode: 'configMap',
        confidence: 'high',
        source: 'user',
        generated: { locationPath: route.path, proxyPass: route.targetUrl, [`PROXY_ROUTE_${index + 1}`]: `${route.path} -> ${route.targetUrl}` },
      })
    }
  }
  const configMapName = componentGeneratedConfigMapName()
  upsertConfigMapValue(configMapName, 'default.conf', buildNginxApiProxyConfigFromRoutes(routes))
  upsertConfigFile({
    name: 'nginx-api-proxy',
    configMapName,
    key: 'default.conf',
    mountPath: '/etc/nginx/conf.d/default.conf',
    readOnly: true,
  })
  if (options.showMessage !== false) configDrawer.value.message = `已生成 ${routes.length} 条前端代理路由。`
  return true
}
const buildNginxApiProxyConfigFromRoutes = (routes:Array<{ path:string; targetUrl:string }>) => {
  const routeBlocks = routes.map((route) => {
    const path = normalizeNginxRoutePath(route.path)
    const target = String(route.targetUrl || '').replace(/\/+$/, '')
    return [
      `  location ${path} {`,
      `    proxy_pass ${target};`,
      '    proxy_http_version 1.1;',
      '    proxy_set_header Host $host;',
      '    proxy_set_header X-Real-IP $remote_addr;',
      '    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;',
      '    proxy_set_header X-Forwarded-Proto $scheme;',
      '  }',
    ].join('\n')
  })
  return [
    'server {',
    '  listen 80;',
    '  server_name _;',
    '  root /usr/share/nginx/html;',
    '  index index.html;',
    '',
    '  location / {',
    '    try_files $uri $uri/ /index.html;',
    '  }',
    '',
    ...routeBlocks.flatMap((block) => [block, '']),
    '}',
    '',
  ].join('\n')
}
const upsertConfigMapValue = (name:string, key:string, value:string) => {
  const idx = configForm.value.configMaps.findIndex((item:any) => item.name === name)
  const current = idx >= 0 ? configForm.value.configMaps[idx] : { name, data: {} as Record<string, string> }
  current.data = { ...current.data, [key]: value }
  if (idx >= 0) configForm.value.configMaps.splice(idx, 1, current)
  else configForm.value.configMaps.push(current)
}
const upsertConfigFile = (file:any) => {
  configForm.value.files = mergeComponentConfigFile(configForm.value.files, file)
}
const upsertBinding = (binding:any) => {
  configForm.value.bindings = mergeComponentBinding(configForm.value.bindings, binding) as typeof configForm.value.bindings
}
const removeConfigBinding = (idx:number) => {
  configForm.value.bindings.splice(idx, 1)
  syncGeneratedSpringConfig()
}
const syncGeneratedSpringConfig = () => {
  if (configForm.value.framework !== 'springboot') return
  const springBindings = configForm.value.bindings.filter((item:any) => item.mode === 'springboot-file')
  if (!springBindings.length) return
  const content = buildSpringBootConfig(springBindings)
  const configMapName = componentGeneratedConfigMapName()
  upsertConfigMapValue(configMapName, 'application-paap.yml', content)
  upsertConfigFile({
    name: 'spring-paap-config',
    configMapName,
    key: 'application-paap.yml',
    mountPath: '/etc/paap/application-paap.yml',
    readOnly: true,
  })
  upsertEnvValue('SPRING_CONFIG_ADDITIONAL_LOCATION', 'file:/etc/paap/')
}
const buildSpringBootConfig = (bindings:any[]) => {
  const lines = ['spring:']
  const db = bindings.find((item:any) => ['postgresql', 'mysql'].includes(item.targetType))
  if (db) {
    const prefix = db.targetType === 'mysql' ? 'MYSQL' : 'POSTGRES'
    const driver = db.targetType === 'mysql' ? 'mysql' : 'postgresql'
    const database = db.generated?.[`${prefix}_DATABASE`] || (db.targetType === 'mysql' ? 'mysql' : 'postgres')
    lines.push('  datasource:')
    lines.push(`    url: jdbc:${driver}://${db.generated?.[`${prefix}_HOST`] || db.targetName}:${db.generated?.[`${prefix}_PORT`] || defaultServicePortForBinding(db.targetType)}/${database}`)
    lines.push(`    username: ${db.generated?.[`${prefix}_USERNAME`] || (db.targetType === 'mysql' ? 'root' : 'postgres')}`)
    lines.push('    password: ${SPRING_DATASOURCE_PASSWORD}')
  }
  const redis = bindings.find((item:any) => item.targetType === 'redis')
  if (redis) {
    lines.push('  data:')
    lines.push('    redis:')
    lines.push(`      host: ${redis.generated?.REDIS_HOST || redis.targetName}`)
    lines.push(`      port: ${redis.generated?.REDIS_PORT || 6379}`)
    lines.push('      password: ${REDIS_PASSWORD}')
  }
  return `${lines.join('\n')}\n`
}
const normalizeConfigObjectRows = (items:any[]) => Array.isArray(items)
  ? items.map((item:any) => ({
    name: String(item?.name || '').trim(),
    data: Object.fromEntries(Object.entries(item?.data || {}).map(([key, value]) => [String(key).trim(), String(value ?? '')]).filter(([key]) => key)),
  })).filter((item:any) => item.name && Object.keys(item.data).length)
  : []
const normalizeConfigFiles = (items:any[]) => Array.isArray(items)
  ? items.map((item:any) => ({
    name: String(item?.name || '').trim(),
    configMapName: String(item?.configMapName || '').trim(),
    key: String(item?.key || '').trim(),
    mountPath: String(item?.mountPath || '').trim(),
    readOnly: item?.readOnly !== false,
  })).filter((item:any) => item.configMapName && item.key && item.mountPath)
  : []
const normalizeConfigBindings = (items:any[]) => Array.isArray(items)
  ? items.map((item:any) => ({
    targetKey: String(item?.targetKey || '').trim(),
    targetKind: String(item?.targetKind || '').trim(),
    targetName: String(item?.targetName || '').trim(),
    targetType: String(item?.targetType || '').trim(),
    role: String(item?.role || '').trim(),
    mode: String(item?.mode || '').trim(),
    confidence: String(item?.confidence || '').trim(),
    source: String(item?.source || '').trim(),
    generated: Object.fromEntries(Object.entries(item?.generated || {}).map(([key, value]) => [String(key).trim(), String(value ?? '')]).filter(([key]) => key)),
  })).filter((item:any) => item.targetName || item.targetKey)
  : []
const configFormPayload = () => {
  const env = configForm.value.env
    .map((item:any) => {
      const name = String(item.name || '').trim()
      if (!name) return null
      if (item.source === 'secret') return { name, secretName: String(item.refName || componentGeneratedSecretName()).trim(), secretKey: String(item.refKey || name).trim() }
      if (item.source === 'configMap') return { name, configMapName: String(item.refName || componentGeneratedConfigMapName()).trim(), configMapKey: String(item.refKey || name).trim() }
      return { name, value: String(item.value || '') }
    })
    .filter(Boolean)
  const cfg = parseRuntimeConfig(configDrawer.value.component?.config)
  const bindingDeps = configForm.value.bindings
    .flatMap((item:any) => [item.targetKey, item.targetName])
    .map((item:any) => String(item || '').trim())
    .filter(Boolean)
  return {
    framework: configForm.value.framework === 'auto' ? '' : configForm.value.framework,
    ...componentConfigTemplateSelectionPayload(),
    registryTarget: registryTargetSelectionPayload(),
    env,
    configMaps: configForm.value.configMaps,
    secrets: configForm.value.secrets,
    files: configForm.value.files,
    bindings: configForm.value.bindings,
    dependencies: Array.from(new Set([...(cfg.dependencies || []), ...bindingDeps])),
    containerPort: ['saved', 'user'].includes(configForm.value.containerPortSource) && Number(configForm.value.containerPort || 0) > 0 ? Number(configForm.value.containerPort || 0) : 0,
    command: arrayFromLines(configForm.value.commandText),
    args: arrayFromLines(configForm.value.argsText),
  }
}
const validateConfigFileMountPaths = () => {
  const missing = configForm.value.files.find((item:any) =>
    String(item.configMapName || '').trim()
    && String(item.key || '').trim()
    && !String(item.mountPath || '').trim()
  )
  if (!missing) return true
  configDrawer.value.error = `请填写配置文件 ${missing.key || missing.name || ''} 的挂载路径。`
  return false
}
const saveConfigDrawer = async (options: { refresh?: boolean; includeDelivery?: boolean } = {}) => {
  const comp = configDrawer.value.component
  if (!comp?.id || configDrawer.value.saving) return
  const shouldRefresh = options.refresh !== false
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  configDrawer.value.message = ''
  try {
    const deliveryMode = configForm.value.deliveryMode === 'source' ? 'source' : 'image'
    const includeDelivery = options.includeDelivery === true
    const version = deliveryMode === 'source'
      ? String(configForm.value.version || '').trim()
      : imageTagVersion(configForm.value.imageTag) || String(configForm.value.version || '').trim()
    const image = registryImageFromConfig.value || String(configForm.value.image || '').trim()
    const sourceBranch = String(configForm.value.sourceBranch || 'main').trim() || 'main'
    const buildContext = String(configForm.value.buildContext || '.').trim() || '.'
    const buildModule = String(configForm.value.buildModule || '').trim()
    syncNginxRouteRowsFromTemplateFieldValues()
    const templateReady = await prepareSelectedComponentConfigTemplateForSave()
    if (!templateReady) return
    if (componentNginxRouteEditorVisible.value && normalizedNginxRoutes().length) {
      const nginxReady = applyNginxRoutes({ showMessage: false })
      if (!nginxReady) return
    }
    if (!validateConfigFileMountPaths()) return
    const deliveryPayload = includeDelivery
      ? {
          deliveryMode: configForm.value.deliveryMode,
          image: deliveryMode === 'image' ? image : '',
          version,
          sourceRepoUrl: configForm.value.sourceRepoUrl,
          sourceBranch,
          buildContext,
          buildModule,
          dockerfilePath: String(configForm.value.dockerfilePath || '').trim(),
        }
      : {}
    const res = await api.updateComponent(Number(comp.id), {
      type: componentDrawerRole.value,
      ...deliveryPayload,
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
    configDrawer.value.message = '配置已保存。'
    return updated
  } catch (e:any) {
    configDrawer.value.error = '保存配置失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const saveConfigDrawerIfComponent = async (options: { refresh?: boolean; includeDelivery?: boolean } = {}) => {
  if (configDrawer.value.kind !== 'component' || !configDrawer.value.component?.id) return true
  const saved = await saveConfigDrawer(options)
  return configDrawer.value.error ? false : (saved || true)
}
const saveServiceConfigDrawer = async () => {
  const svc = configDrawer.value.service
  if (!svc?.id || configDrawer.value.saving) return
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  configDrawer.value.message = ''
  try {
    const values = serviceConfigValuesFromForm(serviceDrawerConfigType.value, serviceConfigForm.value)
    const res = await api.updateService(envId.value, Number(svc.id), { values })
    const updated = res.data
    services.value = services.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    await refreshServices()
    scheduleTemplateInstallPolling()
    const next = services.value.find((item:any) => Number(item.id) === Number(updated.id)) || updated
    configDrawer.value.service = next
    serviceConfigForm.value = serviceConfigFormFromInstallation(next)
    await loadServiceDrawerWorkspace(next)
    configDrawer.value.message = '服务配置已保存。'
  } catch (e:any) {
    configDrawer.value.error = '保存服务配置失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const saveCapabilityConfigDrawer = async () => {
  const cap = drawerCapability.value
  if (!cap?.capability || configDrawer.value.saving) return
  if (cap.source !== 'external') {
    configDrawer.value.error = '共享资源只读，不能在业务环境中修改。'
    return
  }
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  configDrawer.value.message = ''
  const authType = String(capabilityForm.value.authType || 'none')
  const payload: any = {
    source: 'external',
    capabilityKey: capabilityRequestKey(cap),
    provider: cap.provider || providerForCapability(cap.capability),
    serviceType: cap.serviceType || serviceTypeForCapability(cap.capability),
    externalEndpoint: capabilityForm.value.externalEndpoint,
    authType,
    tlsInsecureSkipVerify: Boolean(capabilityForm.value.tlsInsecureSkipVerify),
  }
  const credentialSecretRef = String(capabilityForm.value.credentialSecretRef || cap.credentialSecretRef || '').trim()
  if (authType === 'basic') {
    payload.username = capabilityForm.value.username
    if (capabilityForm.value.password) payload.password = capabilityForm.value.password
    if (!payload.password && credentialSecretRef) payload.credentialSecretRef = credentialSecretRef
  } else if (authType === 'token') {
    if (capabilityForm.value.token) payload.token = capabilityForm.value.token
    if (!payload.token && credentialSecretRef) payload.credentialSecretRef = credentialSecretRef
  } else if (authType === 'existingSecret') {
    payload.credentialSecretRef = credentialSecretRef
  } else {
    payload.credentialSecretRef = ''
  }
  try {
    const res = await api.updateEnvironmentCapability(envId.value, capabilityRequestKey(cap), payload)
    await loadEnvironmentCapabilities()
    const updated = environmentCapabilities.value.find((item:any) => Number(item.id) === Number(res.data?.id || cap.id)) || res.data || cap
    const currentForm = capabilityForm.value
    configDrawer.value.capability = updated
    capabilityForm.value = {
      externalEndpoint: updated.externalEndpoint || currentForm.externalEndpoint,
      authType: updated.credentialSecretRef ? (authType === 'existingSecret' ? 'existingSecret' : authType) : 'none',
      username: currentForm.username,
      password: currentForm.password,
      token: currentForm.token,
      credentialSecretRef: updated.credentialSecretRef || '',
      tlsInsecureSkipVerify: Boolean(updated.tlsInsecureSkipVerify),
    }
    configDrawer.value.message = '外部资源配置已保存。'
  } catch (e:any) {
    configDrawer.value.error = '保存外部资源配置失败：' + (e?.message || '未知错误')
  } finally {
    configDrawer.value.saving = false
  }
}
const validateCapabilityConfigDrawer = async () => {
  const cap = drawerCapability.value
  if (!cap?.capability || capabilityValidationLoading.value) return
  if (cap.source !== 'external') {
    configDrawer.value.error = '只有外部资源需要验证连接。'
    return
  }
  capabilityValidationLoading.value = true
  configDrawer.value.error = ''
  configDrawer.value.message = ''
  try {
    const res = await api.validateEnvironmentCapability(envId.value, capabilityRequestKey(cap))
    await loadEnvironmentCapabilities()
    const updated = environmentCapabilities.value.find((item:any) => Number(item.id) === Number(res.data?.id || cap.id)) || res.data || cap
    configDrawer.value.capability = updated
    if (updated.validationStatus === 'valid') {
      configDrawer.value.message = updated.validationMessage || '外部资源连接验证通过。'
    } else {
      configDrawer.value.error = updated.validationMessage || '外部资源连接验证失败。'
    }
  } catch (e:any) {
    configDrawer.value.error = '验证外部资源连接失败：' + (e?.message || '未知错误')
  } finally {
    capabilityValidationLoading.value = false
  }
}
const deployServiceFromDrawer = async () => {
  const svc = configDrawer.value.service
  if (!svc?.id || configDrawer.value.saving || !serviceStatusCanDeploy(svc)) return
  configDrawer.value.saving = true
  configDrawer.value.error = ''
  configDrawer.value.message = `正在提交 ${svcLabel(svc.serviceType) || svc.serviceType || '服务'} 部署...`
  try {
    const values = serviceDrawerProfile.value.showDeploymentConfig
      ? serviceConfigValuesFromForm(serviceDrawerConfigType.value, serviceConfigForm.value)
      : {}
    const res = await api.installService(envId.value, { serviceType: svc.serviceType, values, chartVersion: selectedChartVersion.value || undefined })
    const updated = res.data
    if (updated?.id) {
      services.value = services.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    }
    await refreshServices()
    scheduleTemplateInstallPolling()
    const next = services.value.find((item:any) => Number(item.id) === Number(updated?.id || svc.id))
      || services.value.find((item:any) => item.serviceType === svc.serviceType)
      || updated
      || svc
    configDrawer.value.service = next
    serviceConfigForm.value = serviceConfigFormFromInstallation(next)
    await loadServiceDrawerWorkspace(next)
    notifyEnvUpdated()
    configDrawer.value.message = `${svcLabel(next?.serviceType || svc.serviceType) || svc.serviceType || '服务'} 部署已提交。`
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
  configDrawer.value.message = ''
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
    configDrawer.value.message = nextEnabled ? '外部访问已开启。' : '外部访问已关闭。'
  } catch (e:any) {
    configDrawer.value.error = '外部访问切换失败：' + (e?.message || '未知错误')
  } finally {
    serviceExternalAccessLoading.value = false
  }
}
const toggleComponentExternalAccess = async () => {
  const comp = configDrawer.value.component
  if (!comp?.id || componentExternalAccessLoading.value) return
  componentExternalAccessLoading.value = true
  configDrawer.value.error = ''
  configDrawer.value.message = ''
  try {
    const nextEnabled = !componentDrawerExternalAccessEnabled.value
    const res = await api.setComponentExternalAccess(envId.value, Number(comp.id), nextEnabled)
    if (Array.isArray(res.externalAccess)) {
      externalAccess.value = res.externalAccess
    }
    const updated = res.data
    if (updated?.id) {
      components.value = components.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    }
    const next = components.value.find((item:any) => Number(item.id) === Number(comp.id)) || updated || comp
    configDrawer.value.component = next
    notifyEnvUpdated()
    configDrawer.value.message = nextEnabled ? '外部访问已开启。' : '外部访问已关闭。'
  } catch (e:any) {
    configDrawer.value.error = '外部访问切换失败：' + (e?.message || '未知错误')
  } finally {
    componentExternalAccessLoading.value = false
  }
}
const toggleComponentNodePortAccess = async () => {
  const comp = configDrawer.value.component
  if (!comp?.id || componentNodePortLoading.value) return
  componentNodePortLoading.value = true
  configDrawer.value.error = ''
  configDrawer.value.message = ''
  try {
    const nextEnabled = !componentDrawerNodePortEnabled.value
    const res = await api.setComponentNodePortAccess(envId.value, Number(comp.id), nextEnabled)
    if (Array.isArray(res.externalAccess)) {
      externalAccess.value = res.externalAccess
    }
    const updated = res.data
    if (updated?.id) {
      components.value = components.value.map((item:any) => Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item)
    }
    const next = components.value.find((item:any) => Number(item.id) === Number(comp.id)) || updated || comp
    configDrawer.value.component = next
    notifyEnvUpdated()
    configDrawer.value.message = nextEnabled ? 'NodePort 已开启。' : 'NodePort 已关闭。'
  } catch (e:any) {
    configDrawer.value.error = 'NodePort 切换失败：' + (e?.message || '未知错误')
  } finally {
    componentNodePortLoading.value = false
  }
}
const deployDrawerComponent = async () => {
  const comp = configDrawer.value.component
  if (!comp?.id) return
  configDrawer.value.error = ''
  if (!validateComponentDeliveryForm()) return
  const saved = await saveConfigDrawerIfComponent({ refresh: false, includeDelivery: true })
  if (!saved) return
  const next = saved === true ? (components.value.find((item:any) => Number(item.id) === Number(comp.id)) || comp) : saved
  await deployComponent(next)
  const refreshed = components.value.find((item:any) => Number(item.id) === Number(comp.id))
  if (refreshed) {
    configDrawer.value.component = refreshed
    loadComponentConfigForm(refreshed)
  }
}
const openDeleteDialog = (dialog: PendingDeleteDialog) => {
  enterModalContext()
  pendingDeleteDialog.value = dialog
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
    if (dialog.kind === 'capability') await performDeleteCapability(dialog.target)
    else await performDeleteComponent(dialog.target)
    pendingDeleteDialog.value = null
  } catch (e:any) {
    dialog.error = e?.message || '删除失败'
  } finally {
    if (pendingDeleteDialog.value) pendingDeleteDialog.value.submitting = false
  }
}
const topologyDeleteTitle = (node:any) => {
  if (node?.topologyKind === 'capability') return capabilityRemovalActionLabel(node)
  if (node?.topologyKind === 'service') return '卸载服务'
  return '删除组件'
}
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
const resolveTopologyCapability = (node:any) => {
  if (!node) return null
  const capabilityId = Number(node.id)
  if (Number.isFinite(capabilityId) && capabilityId > 0) {
    return environmentCapabilities.value.find((item:any) => Number(item.id) === capabilityId) || node
  }
  const capability = String(node.capability || '').trim()
  if (capability) {
    return environmentCapabilities.value.find((item:any) => String(item.capability || '') === capability) || node
  }
  return node
}
const deleteTopologyNode = (node:any) => {
  if (node?.topologyKind === 'service') {
    const svc = resolveTopologyService(node)
    if (svc) beginUninstallService(svc)
    return
  }
  if (node?.topologyKind === 'capability') {
    const capability = resolveTopologyCapability(node)
    if (capability) void deleteCapabilityById(capability)
    return
  }
  void deleteComponentById(node)
}
const deleteComponentById = async (comp:any) => {
  if (!comp?.id || componentActionLoading.value) return
  openDeleteDialog({
    kind: 'component',
    label: '组件',
    name: String(comp.name || comp.id),
    message: '删除后会移除组件记录、画布位置、连线，并清理该组件在集群中的运行态资源。',
    target: comp,
    submitting: false,
    error: '',
  })
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
    deleteCanvasPositionKeys(nextPositions, key)
    componentNodePositions.value = nextPositions
    manualCanvasEdges.value = manualCanvasEdges.value.filter(edge => edge.fromKey !== key && edge.toKey !== key)
    selectedComponentId.value = null
    selectedTopologyKey.value = null
    selectedManualEdge.value = null
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
const deleteCapabilityById = async (cap:any) => {
  if (!cap?.capability || configDrawer.value.saving) return
  openDeleteDialog({
    kind: 'capability',
    label: '资源卡片',
    actionLabel: capabilityRemovalActionLabel(cap),
    name: capabilityDisplayName(cap),
    message: capabilityRemovalMessage(cap),
    target: cap,
    submitting: false,
    error: '',
  })
}
const performDeleteCapability = async (cap:any) => {
  if (!cap?.capability || configDrawer.value.saving) return
  configDrawer.value.saving = true
  pageError.value = ''
  try {
    const requestKey = capabilityRequestKey(cap)
    await api.deleteEnvironmentCapability(envId.value, requestKey)
    const key = `capability:${cap.id || cap.capability}`
    environmentCapabilities.value = environmentCapabilities.value.filter((item:any) => {
      if (cap.id && Number(item.id) === Number(cap.id)) return false
      return capabilityRequestKey(item) !== requestKey
    })
    const nextPositions = { ...componentNodePositions.value }
    deleteCanvasPositionKeys(nextPositions, key)
    componentNodePositions.value = nextPositions
    manualCanvasEdges.value = manualCanvasEdges.value.filter(edge => edge.fromKey !== key && edge.toKey !== key)
    selectedTopologyKey.value = null
    selectedManualEdge.value = null
    if (configDrawer.value.kind === 'capability') closeConfigDrawer()
    await saveComponentNodePositions()
    notifyEnvUpdated()
  } catch (e:any) {
    throw new Error('删除资源卡片失败：' + (e?.message || '未知错误'))
  } finally {
    configDrawer.value.saving = false
    closeComponentContextMenu()
  }
}
const deleteContextCapability = () => {
  const capability = componentContextMenu.value.capability
  closeComponentContextMenu()
  if (capability) void deleteCapabilityById(capability)
}
const deleteDrawerCapability = async () => {
  const capability = configDrawer.value.capability
  closeConfigDrawer()
  if (capability) await deleteCapabilityById(capability)
}
const runtimeEnvValue = (envItem:any) => {
  if (envItem?.secretName) return envItem.secretKey ? `由敏感配置管理 · ${envItem.secretKey}` : '由敏感配置管理'
  if (envItem?.configMapName) return envItem.configMapKey ? `由普通配置管理 · ${envItem.configMapKey}` : '由普通配置管理'
  return envItem?.value || '-'
}
const configureContextNode = () => {
  const comp = componentContextMenu.value.component
  const svc = componentContextMenu.value.service
  const capability = componentContextMenu.value.capability
  closeComponentContextMenu()
  if (comp?.id) openComponentConfigDrawer(comp, 'variables')
  else if (svc?.id) openServiceConfigDrawer(svc)
  else if (capability) openCapabilityConfigDrawer(capability)
}
const startRenameNode = (node: any) => {
  const key = String(node?.topologyId || node?.id || '')
  if (!key) return
  // A double-click still fires two click events first; the first one already
  // opened the sidebar via selectTopologyNode. Close it so the user lands in
  // the rename input without the sidebar stealing focus.
  closeConfigDrawer()
  renamingNodeKey.value = key
  renamingNodeValue.value = componentDisplayNames.value[key] || String(node?.name || '')
  nextTick(() => {
    const input = document.querySelector('.node-rename-input') as HTMLInputElement | null
    input?.focus()
    input?.select()
  })
}
const renameContextNode = () => {
  const comp = componentContextMenu.value.component
  const svc = componentContextMenu.value.service
  const capability = componentContextMenu.value.capability
  const node = comp || svc || capability
  closeComponentContextMenu()
  if (!node) return
  startRenameNode(node)
}
const submitRenameNode = () => {
  const key = renamingNodeKey.value
  if (!key) return
  const value = renamingNodeValue.value.trim()
  if (value) {
    componentDisplayNames.value = { ...componentDisplayNames.value, [key]: value }
  } else {
    const next = { ...componentDisplayNames.value }
    delete next[key]
    componentDisplayNames.value = next
  }
  renamingNodeKey.value = null
  renamingNodeValue.value = ''
  void saveComponentNodePositions()
}
const cancelRenameNode = () => {
  renamingNodeKey.value = null
  renamingNodeValue.value = ''
}
const deployContextComponent = async () => {
  const comp = componentContextMenu.value.component
  closeComponentContextMenu()
  if (!comp) return
  openComponentConfigDrawer(comp)
  configDrawerTab.value = 'deploy'
  await deployComponent(comp)
}
const deployContextService = () => {
  const svc = componentContextMenu.value.service
  closeComponentContextMenu()
  if (!svc) return
  openServiceConfigDrawer(svc)
  configDrawerTab.value = 'deploy'
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
        framework: cfg.framework,
        env: cfg.env,
        configMaps: cfg.configMaps,
        secrets: cfg.secrets,
        files: cfg.files,
        bindings: cfg.bindings,
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
  const comp = componentContextMenu.value.component
  const svc = componentContextMenu.value.service
  closeComponentContextMenu()
  if (comp) openComponentConfigDrawer(comp)
  if (svc) openServiceConfigDrawer(svc)
  configDrawerTab.value = 'runtime'
  void loadDrawerRuntimeMetrics(true)
}
const openContextNodeLogs = () => {
  const comp = componentContextMenu.value.component
  const svc = componentContextMenu.value.service
  closeComponentContextMenu()
  if (comp) openComponentConfigDrawer(comp)
  if (svc) openServiceConfigDrawer(svc)
  configDrawerTab.value = 'logs'
  void loadDrawerRuntimeLogs(true)
}
const openDrawerMonitoring = () => {
  configDrawerTab.value = 'runtime'
  void loadDrawerRuntimeMetrics(true)
}
const openDrawerLogs = () => {
  configDrawerTab.value = 'logs'
  void loadDrawerRuntimeLogs(true)
}
const componentRuntimeIdentifier = (comp:any) => {
  const path = String(comp?.gitPath || '').trim()
  if (path.includes('/')) return path.split('/').filter(Boolean).pop() || ''
  const image = String(comp?.registryImage || comp?.image || '').trim()
  const imageName = image.split('/').pop()?.split(':')[0] || ''
  return imageName
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
    const created = res.data
    await refreshServices()
    components.value = mergeCreatedCanvasResource(components.value, created)
    const actual = selectCreatedCanvasResource(components.value, created, type)
    if (actual?.id && createPoint) {
      const key = nodeCanvasPositionKey(canvasCreateScope.value, { topologyId: `component:${actual.id}` })
      componentNodePositions.value = {
        ...componentNodePositions.value,
        [key]: {
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
    const res = await api.createServiceDraft(envId.value, { serviceType })
    const created = res.data
    await refreshServices()
    services.value = mergeCreatedCanvasResource(services.value, created)
    const installed = selectCreatedCanvasResource(services.value, created, serviceType)
    if (installed) {
      if (createPoint) {
        const key = nodeCanvasPositionKey(canvasCreateScope.value, { topologyId: `service:${installed.id}` })
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
const placeCapabilityNode = async (capability:any) => {
  const createPoint = canvasCreatePoint.value
  if (!capability?.id || !createPoint) return
  const key = nodeCanvasPositionKey(canvasCreateScope.value, { topologyId: `capability:${capability.id}` })
  componentNodePositions.value = {
    ...componentNodePositions.value,
    [key]: {
      x: Math.max(12, createPoint.x - componentCanvasMetrics.nodeWidth / 2),
      y: Math.max(46, createPoint.y - componentCanvasMetrics.nodeHeight / 2),
    },
  }
  await saveComponentNodePositions()
}
const createSharedCapabilityReference = async (resource:any) => {
  closeComponentContextMenu()
  pageError.value = ''
  try {
    const res = await api.updateEnvironmentCapability(envId.value, resource.capability, {
      source: 'shared',
      capabilityKey: `shared-${resource.capability}-${resource.id}`,
      provider: resource.provider,
      serviceType: resource.serviceType,
      refServiceId: Number(resource.id),
    })
    await loadEnvironmentCapabilities()
    const created = res.data
    await placeCapabilityNode(created)
    openCapabilityConfigDrawer(created)
  } catch (e:any) {
    pageError.value = '添加共享资源失败：' + (e?.message || '未知错误')
  }
}
const createExternalCapabilityDraft = async (item:any) => {
  closeComponentContextMenu()
  pageError.value = ''
  try {
    const res = await api.updateEnvironmentCapability(envId.value, item.capability, {
      source: 'external',
      capabilityKey: `external-${item.capability}-${item.provider || item.serviceType || item.capability}`,
      provider: item.provider,
      serviceType: item.serviceType,
    })
    await loadEnvironmentCapabilities()
    const created = res.data
    await placeCapabilityNode(created)
    openCapabilityConfigDrawer(created)
  } catch (e:any) {
    pageError.value = '添加外部资源失败：' + (e?.message || '未知错误')
  }
}
const adoptCanvasResource = async () => {
  openAdoptResourceModal()
}
const openAdoptResourceModal = async () => {
  enterModalContext()
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
    components.value = mergeCreatedCanvasResource(components.value, res.data)
    const adopted = selectCreatedCanvasResource(components.value, res.data, res.data?.type)
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

const closeComponentDraftModal = () => {
  showComponentModal.value = false
  componentModalError.value = ''
}

const closeServiceInstallModal = () => {
  if (serviceSubmitting.value) return
  showServiceModal.value = false
  serviceModalError.value = ''
  serviceModalNotice.value = ''
  serviceProvisionMode.value = 'managed'
  selectedSharedResourceId.value = ''
}

const openServiceInstallModal = (mode:'tool'|'infra' = 'tool') => {
  enterModalContext()
  void prepareServicePicker(mode)
}

const openServiceModal = () => {
  openServiceInstallModal('tool')
}

const envStatusText = (status: string | undefined) => {
  return environmentStatusLabel(effectiveEnvironmentStatus({
    name: env.value?.name || '',
    status,
    errorMessage: env.value?.errorMessage,
    services: services.value,
    componentCount: components.value.length,
  }))
}
const selectServiceTemplate = (svc:any) => {
  if (svc.disabled) {
    serviceModalError.value = `${svc.name} 已添加、已安装或正在安装。`
    return
  }
  serviceModalError.value = ''
  serviceForm.value.serviceType = svc.type
  selectedSharedResourceId.value = ''
  if (serviceProvisionMode.value === 'shared') {
    void loadSharedCapabilityResources().then(() => {
      selectedSharedResourceId.value = matchingSharedResources.value[0]?.id ? String(matchingSharedResources.value[0].id) : ''
    })
  }
}

const submitComponent = async () => {
  const image = compForm.value.image.trim()
  const version = compForm.value.version.trim()
  const deliveryMode = compForm.value.deliveryMode === 'source' ? 'source' : 'image'
  const sourceRepoUrl = compForm.value.sourceRepoUrl.trim()
  const sourceBranch = compForm.value.sourceBranch.trim() || 'main'
  const buildContext = compForm.value.buildContext.trim() || '.'
  const buildModule = compForm.value.buildModule.trim()
  componentModalError.value = ''
  if (!compForm.value.name.trim()) { componentModalError.value = '请填写组件名称'; return }
  if (deliveryMode === 'image' && (!version || version.toLowerCase() === 'latest')) { componentModalError.value = '请填写明确版本号，不能使用 latest'; return }
  if (deliveryMode === 'source' && version && version.toLowerCase() === 'latest') { componentModalError.value = '源码交付版本不能使用 latest'; return }
  if (deliveryMode === 'image' && !image) { componentModalError.value = '请填写镜像地址'; return }
  if (deliveryMode === 'source' && !sourceRepoUrl) { componentModalError.value = '请填写源码仓库地址'; return }
  const payload = deliveryMode === 'source'
    ? { ...compForm.value, deliveryMode, sourceRepoUrl, sourceBranch, buildContext, buildModule, image: '', version, draftOnly: true }
    : { ...compForm.value, deliveryMode, image, version, draftOnly: true }
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
const submitManagedServiceInstall = async (selectedType: string) => {
  if (isActiveServiceInstalled(selectedType, 'managed')) {
    serviceModalError.value = `${svcLabel(selectedType)} 已添加、已安装或正在安装。`
    availableServices.value = filterTemplates(serviceModalMode.value)
    serviceForm.value.serviceType = preferredServiceTypeForProvisionMode()
    return
  }
  const beforeIds = new Set(services.value.map((item:any) => Number(item.id)))
  await api.installService(envId.value, { serviceType: selectedType })
  await refreshServices()
  const installed = services.value.find((item:any) => !beforeIds.has(Number(item.id)) && item.serviceType === selectedType)
    || services.value.find((item:any) => item.serviceType === selectedType)
  availableServices.value = filterTemplates(serviceModalMode.value)
  serviceForm.value.serviceType = preferredServiceTypeForProvisionMode()
  serviceModalNotice.value = pickerNotice(serviceModalMode.value, visibleServiceOptions.value.length, serviceForm.value.serviceType)
  showServiceModal.value = false
  if (installed) {
    setActiveTab('components')
    openServiceConfigDrawer(installed)
  }
}
const submitSharedServiceReference = async () => {
  const resource = selectedSharedResource.value
  if (!resource) {
    serviceModalError.value = `共享资源池中没有可引用的 ${svcLabel(serviceForm.value.serviceType)}。`
    return
  }
  await createSharedCapabilityReference(resource)
  showServiceModal.value = false
}
const submitExternalServiceConnection = async () => {
  const external = externalCapabilityForService(selectedServiceTemplate.value)
  if (!external) {
    serviceModalError.value = `${svcLabel(serviceForm.value.serviceType)} 暂不支持外部连接。`
    return
  }
  await createExternalCapabilityDraft(external)
  showServiceModal.value = false
}
const submitKubeVirtServiceInstall = async (selectedType: string) => {
  if (isActiveServiceInstalled(selectedType, 'kubevirt')) {
    serviceModalError.value = `${svcLabel(selectedType)} 已添加、已安装或正在安装。`
    availableServices.value = filterTemplates(serviceModalMode.value)
    serviceForm.value.serviceType = preferredServiceTypeForProvisionMode()
    return
  }
  const beforeIds = new Set(services.value.map((item:any) => Number(item.id)))
  await api.installService(envId.value, { serviceType: selectedType, provisionMode: 'kubevirt' })
  await refreshServices()
  const installed = services.value.find((item:any) => !beforeIds.has(Number(item.id)) && item.serviceType === selectedType && item.provisionMode === 'kubevirt')
    || services.value.find((item:any) => item.serviceType === selectedType && item.provisionMode === 'kubevirt')
    || services.value.find((item:any) => item.serviceType === selectedType)
  availableServices.value = filterTemplates(serviceModalMode.value)
  serviceForm.value.serviceType = preferredServiceTypeForProvisionMode()
  serviceModalNotice.value = pickerNotice(serviceModalMode.value, visibleServiceOptions.value.length, serviceForm.value.serviceType)
  showServiceModal.value = false
  if (installed) {
    setActiveTab('components')
    openServiceConfigDrawer(installed)
  }
}
const submitService = async () => {
  if (!serviceForm.value.serviceType) return
  const selectedType = serviceForm.value.serviceType
  serviceSubmitting.value = true
  serviceModalError.value = ''
  serviceModalNotice.value = ''
  try {
    if (serviceProvisionMode.value === 'managed') {
      await submitManagedServiceInstall(selectedType)
    } else if (serviceProvisionMode.value === 'shared') {
      await submitSharedServiceReference()
    } else if (serviceProvisionMode.value === 'external') {
      await submitExternalServiceConnection()
    } else {
      await submitKubeVirtServiceInstall(selectedType)
    }
  }
  catch(e:any) { serviceModalError.value = '处理失败：' + (e?.message || '未知错误') }
  finally { serviceSubmitting.value = false }
}

const beginUninstallService = (svc:any) => {
  enterModalContext()
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
  deleteCanvasPositionKeys(nextPositions, key)
  componentNodePositions.value = nextPositions
  manualCanvasEdges.value = manualCanvasEdges.value.filter(edge => edge.fromKey !== key && edge.toKey !== key)
  selectedTopologyKey.value = null
  selectedManualEdge.value = null
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
  background: linear-gradient(90deg, transparent, var(--paap-accent-glow), transparent);
}
.title-group { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.title-row { display: flex; align-items: center; gap: var(--paap-space-3); flex-wrap: wrap; }
.page-title { font-size: 24px; font-weight: 600; color: var(--paap-text); line-height: 1.2; letter-spacing: -0.02em; margin: 0; }
.title-id { font-family: var(--paap-mono); font-size: var(--paap-fs-code); color: var(--paap-muted); letter-spacing: 0.02em; }
.status-badge {
  display: inline-flex; align-items: center; gap: 5px;
  font-size: var(--paap-fs-small); font-weight: 500; padding: 2px 10px; border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle); color: var(--paap-muted);
}
.status-badge.running { background: var(--paap-success-soft); color: var(--paap-emerald); }
.status-badge.error { background: var(--paap-danger-soft); color: var(--paap-danger); }
.status-badge.creating { background: var(--paap-accent-soft); color: var(--paap-accent); }
.title-actions { flex-shrink: 0; display: flex; gap: var(--paap-space-2); align-self: center; flex-wrap: wrap; justify-content: flex-end; }
.page-error {
  border: 1px solid var(--paap-danger-border);
  background: var(--paap-danger-soft);
  color: var(--paap-danger-text-strong);
  border-radius: var(--paap-radius);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
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
  min-width: 0;
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
.stat-label { color: var(--paap-muted); font-size: var(--paap-fs-compact); font-weight: 500; }
.stat-value { margin-top: var(--paap-space-4); color: var(--paap-text); font-size: 42px; font-weight: 700; line-height: 1; letter-spacing: -0.02em; }
.stat-value.danger { color: var(--paap-danger); }
.stat-hint { margin-top: var(--paap-space-3); color: var(--paap-muted); font-size: var(--paap-fs-label); }
.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 420px);
  gap: var(--paap-space-6);
  margin-bottom: 0;
}
.overview-section {
  min-width: 0;
  padding: var(--paap-space-5);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.overview-topology-primary {
  overflow: hidden;
}
.overview-section--anchor { scroll-margin-top: 88px; }
.overview-wide { margin-bottom: 0; }
.overview-section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--paap-space-3); min-width: 0; margin-bottom: var(--paap-space-4); }
.overview-title { margin: 0; color: var(--paap-text); font-size: 18px; font-weight: 600; }
.overview-subtitle { color: var(--paap-muted); font-size: var(--paap-fs-label); max-width: 520px; text-align: right; line-height: 1.4; }
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
  font-size: var(--paap-fs-label);
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
.external-access-main strong { font-size: var(--paap-fs-compact); font-weight: 600; }
.external-access-main small { color: var(--paap-muted); font-size: var(--paap-fs-label); }
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
  color: var(--paap-muted);
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
  font-size: var(--paap-fs-label);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-overview-node small {
  grid-column: 2;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
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
  font-size: var(--paap-fs-label);
}
.component-summary-row strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
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
  font-size: var(--paap-fs-small);
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
.flow-step { display: grid; grid-template-columns: 14px minmax(0, 1fr) auto; align-items: start; gap: var(--paap-space-3); padding: 13px 0; border-bottom: 1px solid var(--paap-panel-subtle); }
.flow-step:last-child { border-bottom: none; }
.flow-dot { width: 9px; height: 9px; margin-top: 5px; border-radius: 50%; background: var(--paap-border-strong); }
.flow-step.ready .flow-dot { background: var(--paap-success); }
.flow-step.pending .flow-dot,
.flow-step.missing .flow-dot { background: var(--paap-muted); }
.flow-step.failed .flow-dot { background: var(--paap-danger); }
.flow-name { color: var(--paap-text); font-size: var(--paap-fs-compact); font-weight: 600; }
.flow-desc { margin-top: 2px; color: var(--paap-muted); font-size: var(--paap-fs-label); line-height: 1.4; }
.flow-link, .text-btn {
  border: none;
  background: transparent;
  color: var(--paap-accent);
  font-size: var(--paap-fs-label);
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
  border-bottom: 1px solid var(--paap-panel-subtle);
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.quick-row:last-child, .compact-row:last-child { border-bottom: none; }
.quick-row:hover, .compact-row:hover { background: var(--paap-panel-subtle); }
.quick-icon { display: inline-flex; align-items: center; justify-content: center; width: 28px; color: var(--paap-muted); flex-shrink: 0; }
.quick-icon.git { color: var(--paap-service-git); }
.quick-icon.ci { color: var(--paap-service-ci); }
.quick-icon.registry, .quick-icon.harbor { color: var(--paap-service-registry); }
.quick-icon.deploy { color: var(--paap-service-deploy); }
.quick-icon.monitor { color: var(--paap-service-monitor); }
.quick-icon.log { color: var(--paap-service-log); }
.quick-main { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.quick-name, .compact-name { color: var(--paap-text); font-size: var(--paap-fs-compact); font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.quick-desc { color: var(--paap-muted); font-size: var(--paap-fs-label); margin-top: 2px; }
.overview-empty {
  display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-3);
  padding: var(--paap-space-4);
  border: 1px dashed var(--paap-border-strong); border-radius: var(--paap-radius);
  background: var(--paap-panel-subtle); color: var(--paap-muted); font-size: var(--paap-fs-compact);
}
.overview-two-col { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--paap-space-4); }
.compact-head { display: flex; align-items: center; justify-content: space-between; padding-bottom: var(--paap-space-2); color: var(--paap-muted); font-size: var(--paap-fs-label); font-weight: 600; }
.compact-empty { padding: var(--paap-space-5) 0; color: var(--paap-muted); font-size: var(--paap-fs-compact); }

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
  font-size: var(--paap-fs-label);
  line-height: 1.4;
}
.capability-workspace-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  min-width: 0;
  flex-wrap: wrap;
}
.capability-service-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 28px;
  max-width: 100%;
  padding: 5px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: inherit;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  line-height: 1.25;
  overflow-wrap: anywhere;
  text-align: center;
  cursor: pointer;
}
.capability-service-pill.active {
  border-color: var(--paap-info-border);
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
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.capability-external-link:hover {
  border-color: var(--paap-info-border);
  background: var(--paap-accent-soft);
  text-decoration: none;
}
.workspace-action-strip {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: var(--paap-space-2);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel-subtle);
}
.workspace-action-strip--drawer {
  margin-top: calc(var(--paap-space-2) * -1);
}
.workspace-action-inline {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
  padding: var(--paap-space-3);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.workspace-action-inline--drawer {
  margin-top: calc(var(--paap-space-2) * -1);
}
.workspace-action-inline header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--paap-space-3);
  align-items: start;
}
.workspace-action-inline header > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}
.workspace-action-inline header span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 700;
  text-transform: uppercase;
}
.workspace-action-inline header strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  overflow-wrap: anywhere;
}
.workspace-action-inline p {
  margin: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.45;
}
.workspace-action-inline-form {
  display: grid;
  gap: var(--paap-space-3);
}
.workspace-action-inline-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.workspace-action-btn {
  min-height: 30px;
  max-width: 100%;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: inherit;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  line-height: 1.2;
  overflow-wrap: anywhere;
  cursor: pointer;
}
.workspace-action-btn:hover:not(:disabled) {
  border-color: var(--paap-border-strong);
  background: var(--paap-panel);
}
.workspace-action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.56;
}
.workspace-action-btn--primary {
  border-color: var(--paap-accent);
   background: var(--paap-accent);
   color: var(--paap-panel);
}
.workspace-action-btn--primary:hover:not(:disabled) {
  border-color: var(--paap-accent-hover);
  background: var(--paap-accent-hover);
}
.workspace-action-btn--danger {
  border-color: var(--paap-danger);
   background: var(--paap-danger);
   color: var(--paap-panel);
}
.workspace-action-btn--danger:hover:not(:disabled) {
  border-color: var(--paap-danger-dark);
  background: var(--paap-danger-dark);
}
.text-btn.danger { color: var(--paap-danger); }
.workspace-message {
  border: 1px solid var(--paap-success-border);
  background: var(--paap-success-soft);
  color: var(--paap-emerald-dark);
  border-radius: var(--paap-radius);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
  line-height: 1.4;
}
.workspace-loading {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  padding: 10px 12px;
  font-size: var(--paap-fs-label);
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
.service-desc { color: var(--paap-muted); line-height: 1.4; font-size: var(--paap-fs-compact); }
.service-error { color: var(--paap-danger); line-height: 1.4; font-size: var(--paap-fs-label); margin-top: 4px; max-width: 760px; overflow-wrap: anywhere; }
.service-meta { color: var(--paap-muted); font-size: var(--paap-fs-label); margin-top: 4px; }

.service-action {
  width: 32px; height: 32px; display: flex; align-items: center; justify-content: center;
  background: transparent; border: none; color: var(--paap-muted); cursor: pointer;
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
  font-size: var(--paap-fs-compact);
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
  box-shadow: inset 0 0 0 1px var(--paap-accent);
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
  border-color: var(--paap-border);
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: not-allowed;
}
.application-set-deploy-hint {
  margin: -4px var(--paap-space-5) var(--paap-space-3);
  padding: 8px 12px;
  border-left: 3px solid var(--paap-warning);
  background: var(--paap-warning-soft);
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  line-height: 1.33333;
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
  font-size: var(--paap-fs-label);
  line-height: 1.5;
  margin-top: 3px;
}
.component-topology-canvas {
  position: relative;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  min-height: 320px;
  overflow-x: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background:
    radial-gradient(circle at 1px 1px, rgba(141, 141, 141, 0.34) 1px, transparent 0),
    var(--paap-panel);
  background-size: 18px 18px;
}
.topology-controls {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: var(--paap-z-sticky);
  display: flex;
  gap: 0;
  align-items: stretch;
  padding: 0;
  overflow: hidden;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm, 6px);
  box-shadow: var(--paap-shadow-md);
}
.topology-control-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-right: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-text);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
  font-family: inherit;
}
.topology-control-btn:hover {
  background: var(--paap-accent-fill);
  color: var(--paap-accent);
}
.topology-zoom-label {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 10px;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  color: var(--paap-muted);
  min-width: 48px;
  text-align: center;
}
.component-topology-canvas--main {
  min-height: 800px;
  max-height: none;
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
  min-width: 0;
  max-width: 100%;
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
.component-topology-zone-legend {
  position: absolute;
  top: 14px;
  left: 14px;
  z-index: 3;
  display: flex;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
  max-width: calc(100% - 220px);
  pointer-events: none;
}
.component-topology-zone-legend span {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs, 4px);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 600;
}
.component-topology-zone {
  position: absolute;
  z-index: 1;
  display: block;
  overflow: visible;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm, 6px);
  background: rgba(255, 255, 255, 0.72);
  cursor: grab;
  pointer-events: auto;
  touch-action: none;
}
.component-topology-zone:active {
  cursor: grabbing;
}
.component-topology-zone--collapsed {
  background: rgba(255, 255, 255, 0.94);
}
.component-topology-zone--shared {
  border-color: var(--paap-accent-soft);
}
.component-topology-zone--external {
  border-color: var(--paap-text-04);
  background: rgba(244, 244, 244, 0.72);
}
.component-topology-zone-toggle {
  position: relative;
  left: 8px;
  top: 8px;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
  padding: 0 8px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs, 4px);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: inherit;
  font-size: var(--paap-fs-small);
  line-height: 1.2;
  cursor: pointer;
  pointer-events: auto;
}
.component-topology-zone-toggle:hover {
  border-color: var(--paap-accent);
  color: var(--paap-accent);
}
.component-topology-zone-toggle:focus-visible {
  outline: 2px solid var(--paap-accent);
  outline-offset: 1px;
}
.component-topology-zone-toggle span {
  color: var(--paap-text);
  font-weight: 700;
}
.component-topology-zone-resize-handle {
  position: absolute;
  z-index: 2;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  font: inherit;
  pointer-events: auto;
  touch-action: none;
}
.component-topology-zone-resize-handle:focus-visible {
  outline: 2px solid var(--paap-accent);
  outline-offset: 1px;
}
.component-topology-zone-resize-handle--top,
.component-topology-zone-resize-handle--bottom {
  left: 12px;
  right: 12px;
  height: 10px;
  cursor: ns-resize;
}
.component-topology-zone-resize-handle--top { top: -5px; }
.component-topology-zone-resize-handle--bottom { bottom: -5px; }
.component-topology-zone-resize-handle--left,
.component-topology-zone-resize-handle--right {
  top: 12px;
  bottom: 12px;
  width: 10px;
  cursor: ew-resize;
}
.component-topology-zone-resize-handle--left { left: -5px; }
.component-topology-zone-resize-handle--right { right: -5px; }
.component-topology-zone-resize-handle--top-left,
.component-topology-zone-resize-handle--top-right,
.component-topology-zone-resize-handle--bottom-right,
.component-topology-zone-resize-handle--bottom-left {
  width: 16px;
  height: 16px;
}
.component-topology-zone-resize-handle--top-left {
  top: -8px;
  left: -8px;
  cursor: nwse-resize;
}
.component-topology-zone-resize-handle--top-right {
  top: -8px;
  right: -8px;
  cursor: nesw-resize;
}
.component-topology-zone-resize-handle--bottom-right {
  right: -8px;
  bottom: -8px;
  cursor: nwse-resize;
}
.component-topology-zone-resize-handle--bottom-left {
  bottom: -8px;
  left: -8px;
  cursor: nesw-resize;
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
  background: var(--paap-overlay-white);
}
.component-canvas-empty-hint strong {
  color: var(--paap-text);
  font-size: 16px;
}
.component-canvas-empty-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
  pointer-events: auto;
}
.component-canvas-empty-actions button {
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  color: var(--paap-accent);
  font-family: inherit;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  cursor: pointer;
}
.component-canvas-empty-actions button:hover {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
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
  font-size: var(--paap-fs-small);
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
  z-index: 2;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}
.component-canvas-link {
  fill: none;
  stroke: var(--paap-accent);
  stroke-width: 2.2;
  stroke-dasharray: 7 5;
  stroke-linecap: round;
  stroke-linejoin: round;
  marker-end: url(#component-arrow);
  animation: dash-flow 20s linear infinite;
  transition: stroke 0.2s, stroke-width 0.2s;
  pointer-events: none;
}
.component-canvas-link:hover,
.component-canvas-link.active {
  stroke: var(--paap-accent-hover);
  stroke-width: 2.8;
}
@keyframes dash-flow {
  to { stroke-dashoffset: -240; }
}
.environment-canvas-link {
  marker-end: url(#environment-arrow);
}
.component-arrow-head {
  fill: var(--paap-accent);
}
.component-canvas-link.active {
  stroke: var(--paap-accent);
  stroke-width: 2.5;
}
.component-canvas-link--manual {
  cursor: pointer;
}
.component-canvas-link--selected {
  stroke: var(--paap-accent);
  stroke-width: 3;
  stroke-dasharray: none;
  animation: none;
}
.component-canvas-link-hit {
  fill: none;
  stroke: transparent;
  stroke-width: 14;
  pointer-events: stroke;
  cursor: pointer;
}
.component-topology-node {
  position: absolute;
  z-index: 4;
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  grid-template-rows: minmax(0, auto) minmax(0, auto);
  column-gap: 12px;
  row-gap: 2px;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius, 8px);
  background: var(--paap-panel);
  box-shadow: var(--paap-shadow-sm);
  box-sizing: border-box;
  text-align: left;
  cursor: grab;
  transition: border-color 110ms, background 110ms, box-shadow var(--paap-transition-normal, 200ms ease);
  touch-action: none;
}
.component-topology-node:active {
  cursor: grabbing;
}
.component-topology-node:hover,
.component-topology-node.active {
  border-color: var(--paap-accent);
  background: var(--paap-panel);
  box-shadow: var(--paap-shadow-md), 0 0 0 2px var(--paap-accent-ring);
  z-index: 10;
}
.component-topology-node strong { grid-column: 2; color: var(--paap-text); font-size: var(--paap-fs-compact); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.component-topology-node strong,
.component-topology-node small,
.component-topology-node .node-rename-input {
  min-width: 0;
}
.node-rename-input {
  grid-column: 2;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  background: var(--paap-card-bg, #fff);
  border: 1px solid var(--paap-accent);
  border-radius: var(--paap-radius-xs, 4px);
  padding: 1px 4px;
  outline: none;
  width: 100%;
  box-sizing: border-box;
}
.component-topology-node small { grid-column: 2; color: var(--paap-muted); font-size: var(--paap-fs-small); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.node-source-badge {
  position: absolute;
  right: 14px;
  bottom: 10px;
  display: inline-flex;
  align-items: center;
  max-width: 82px;
  min-height: 18px;
  padding: 0 6px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs, 4px);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-source-badge--managed {
  border-color: var(--paap-accent-soft);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.node-source-badge--shared {
  border-color: var(--paap-success-border);
  background: var(--paap-success-soft);
  color: var(--paap-success);
}
.node-source-badge--external {
  border-color: var(--paap-purple-bg);
  background: var(--paap-purple-bg);
  color: var(--paap-purple-text);
}
.node-source-badge--component,
.node-source-badge--deferred {
  border-color: var(--paap-border);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}
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
  color: var(--paap-muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 110ms, background 110ms, color 110ms;
}
.component-topology-node:hover .node-delete-action,
.component-topology-node:focus .node-delete-action,
.node-delete-action:focus {
  opacity: 1;
}
.node-delete-action:hover,
.node-delete-action:focus {
  border-color: var(--paap-danger);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  outline: none;
}
.component-topology-node--service {
  background: var(--paap-panel);
  border-color: var(--paap-border);
}
.component-topology-node--service:hover,
.component-topology-node--service.active {
  border-color: var(--paap-accent);
  background: var(--paap-panel);
}
.component-topology-node.selected {
  border-color: var(--paap-accent);
  box-shadow:
    inset 0 0 0 1px var(--paap-accent),
     0 0 0 2px var(--paap-accent-ring),
    var(--paap-shadow-sm);
}
.topology-marquee {
  fill: var(--paap-accent-fill);
  stroke: var(--paap-accent);
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
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.node-type-icon--frontend { background: var(--paap-danger-soft); color: var(--paap-danger); }
.node-type-icon--backend { background: var(--paap-accent-soft); color: var(--paap-accent); }
.node-type-icon--database,
.node-type-icon--postgresql,
.node-type-icon--mysql,
.node-type-icon--mongodb { background: var(--paap-purple-bg); color: var(--paap-purple-accent); }
.node-type-icon--redis,
.node-type-icon--middleware,
.node-type-icon--rabbitmq,
.node-type-icon--kafka { background: var(--paap-warning-soft); color: var(--paap-amber-dark); }
.node-status {
  position: absolute;
  top: 9px;
  left: 42px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--paap-border-strong);
  box-shadow: 0 0 0 2px var(--paap-panel);
}
.node-status.running, .node-status.linked { background: var(--paap-success); }
.node-status.error, .node-status.failed { background: var(--paap-danger); }
.node-status.creating, .node-status.deploying, .node-status.building, .node-status.installing, .node-status.syncing { background: var(--paap-accent); }
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
  font-size: var(--paap-fs-label);
  cursor: pointer;
}
.component-edge:hover,
.component-edge.active {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.component-edge-empty { margin-top: var(--paap-space-4); color: var(--paap-muted); font-size: var(--paap-fs-label); }
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
  z-index: var(--paap-z-menu);
  display: grid;
  width: 220px;
  padding: 6px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  box-shadow: var(--paap-shadow-md);
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
.component-context-menu button:has(.menu-icon) {
  grid-template-columns: 16px minmax(0, 1fr);
  grid-template-rows: auto auto;
  column-gap: 10px;
  row-gap: 2px;
  align-items: center;
}
.component-context-menu button:hover { background: var(--paap-accent-soft); }
.menu-icon {
  grid-row: 1 / span 2;
  grid-column: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--paap-muted);
}
.component-context-menu button:hover .menu-icon { color: var(--paap-text); }
.component-context-menu button:has(.menu-icon) span { grid-column: 2; }
.component-context-menu button:has(.menu-icon) small { grid-column: 2; }
.component-context-menu span { font-size: var(--paap-fs-compact); font-weight: 600; }
.component-context-menu small { color: var(--paap-muted); font-size: var(--paap-fs-small); }
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
  border: 2px solid var(--paap-accent);
  border-radius: 50%;
  background: var(--paap-panel);
  cursor: crosshair;
  opacity: 0;
  transition: opacity 0.2s;
}
.node-connector--top {
  top: 2px;
  left: 50%;
  transform: translateX(-50%);
}
.node-connector--right {
  right: 2px;
  top: 50%;
  transform: translateY(-50%);
}
.node-connector--bottom {
  bottom: 2px;
  left: 50%;
  transform: translateX(-50%);
}
.node-connector--left {
  left: 2px;
  top: 50%;
  transform: translateY(-50%);
}
.component-topology-node:hover .node-connector {
  opacity: 1;
}
.connection-drag-line {
  color: var(--paap-accent);
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
  z-index: var(--paap-z-drawer);
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
  padding: 0;
  background: transparent;
  pointer-events: none;
}
.config-drawer {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  width: clamp(760px, 46vw, 1184px);
  height: 100vh;
  border: 0;
  border-left: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  box-shadow: -4px 0 8px 0 var(--paap-shadow);
  overflow: hidden;
  pointer-events: auto;
}
.config-drawer-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  padding: 24px 38px 14px;
  background: var(--paap-panel);
}
.config-drawer-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  padding: 14px 38px;
  border-top: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.config-drawer-title-block {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: var(--paap-space-3);
  min-width: 0;
}
.config-drawer-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  width: 34px;
  height: 34px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-accent);
  font-size: var(--paap-fs-label);
  font-weight: 750;
}
.config-drawer-avatar--service {
  background: var(--paap-panel);
  color: var(--paap-muted);
}
.config-drawer-header h3 {
  margin: 2px 0;
  color: var(--paap-text);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: 0;
}
.config-drawer-header-actions {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
}
.config-drawer-header small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.config-drawer-tabs {
  display: flex;
  align-items: flex-end;
  gap: 0;
  min-height: 48px;
  padding: 0 24px;
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel);
  overflow-x: auto;
  scrollbar-width: thin;
}
.config-drawer-tabs button {
  position: relative;
  flex: 0 0 auto;
  height: 48px;
  padding: 0 16px;
  border: 0;
  background: transparent;
  color: var(--paap-muted);
  font-family: 'IBM Plex Sans', sans-serif;
  font-size: var(--paap-fs-body);
  font-weight: 400;
  letter-spacing: 0.16px;
  line-height: 48px;
  white-space: nowrap;
  cursor: pointer;
  transition: color 0.15s ease;
}
.config-drawer-tabs button:hover {
  color: var(--paap-text);
}
.config-drawer-tabs button.active {
  color: var(--paap-text);
  font-weight: 600;
}
.config-drawer-tabs button.active::after {
  content: "";
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: var(--paap-accent);
}
.config-drawer-body {
  display: grid;
  align-content: start;
  gap: 0;
  min-height: 0;
  overflow-y: auto;
  padding: 26px 38px 36px;
  background: var(--paap-panel);
}
.config-section {
  display: grid;
  gap: var(--paap-space-3);
  padding: 20px 24px 26px;
  border: 0;
  border-bottom: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.config-section + .config-section {
  padding-top: 20px;
}
.config-section:last-of-type {
  border-bottom: 0;
}
.config-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
}
.config-section-actions {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-3);
}
.config-section-title span,
.config-stack-field span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 700;
  text-transform: uppercase;
}
.config-form-grid label span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.config-section-title small {
  min-width: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 400;
  text-align: right;
}
.config-section-title a {
  color: var(--paap-accent);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  text-decoration: none;
}
.config-section--workspace {
  gap: var(--paap-space-4);
}
.drawer-workspace-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  min-width: 0;
}
.drawer-workspace-head p {
  margin: 6px 0 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.5;
}
.drawer-workspace-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
  flex-shrink: 0;
}
.config-section--workspace :deep(.tool-workspace) {
  min-width: 0;
}
.config-section--workspace :deep(.ws-tabs) {
  overflow-x: auto;
  padding-bottom: 2px;
}
.config-section--workspace :deep(.db-shell),
.config-section--workspace :deep(.monitor-shell),
.config-section--workspace :deep(.log-shell) {
  min-width: 0;
}
.config-section--workspace :deep(.table-wrap),
.config-section--workspace :deep(.terminal),
.config-section--workspace :deep(.card) {
  max-width: 100%;
}
.config-section--workspace :deep(.grafana-frame),
.config-section--workspace :deep(.loki-frame) {
  min-height: 460px;
}
.config-code {
  display: block;
  min-width: 0;
  padding: 8px 10px;
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
  overflow-wrap: anywhere;
}
.service-access-stack {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.config-deployment-card {
  display: grid;
  gap: var(--paap-space-4);
  padding: 16px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.config-deployment-main {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--paap-space-3);
}
.config-deployment-main strong {
  display: block;
  color: var(--paap-text);
  font-size: 15px;
  font-weight: 700;
}
.config-deployment-main small {
  display: block;
  margin-top: 2px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  overflow-wrap: anywhere;
}
.config-deployment-meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.config-deployment-meta div,
.config-variable-row {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.config-deployment-meta span,
.config-variable-row small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.config-deployment-meta strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  overflow-wrap: anywhere;
}
.config-deployment-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.source-semantics-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
}
.source-semantics-row {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.source-semantics-row > span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 600;
}
.source-semantics-row > strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  overflow-wrap: anywhere;
}
.source-semantics-row > small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.4;
}
.config-variable-list {
  display: grid;
  gap: 8px;
}
.config-variable-row {
  grid-template-columns: minmax(180px, 0.8fr) minmax(0, 1fr) minmax(120px, 0.6fr);
  align-items: center;
}
.config-variable-name {
  color: var(--paap-text);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.config-variable-row code {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  overflow-wrap: anywhere;
}
.config-variable-row code.masked {
  color: var(--paap-muted);
  letter-spacing: 0.08em;
}
.config-variable-value {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 32px;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.service-secret-reveal-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
  transition: border-color 110ms, color 110ms, box-shadow 110ms;
}
.service-secret-reveal-btn:hover:not(:disabled) {
  border-color: var(--paap-accent);
  color: var(--paap-accent);
}
.service-secret-reveal-btn:focus-visible {
  outline: 2px solid var(--paap-accent);
  outline-offset: 1px;
}
.service-secret-reveal-btn:disabled {
  cursor: progress;
  opacity: 0.5;
}
.config-inline-note {
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.5;
}
.service-access-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.service-access-row > span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.service-access-row code,
.service-access-row strong {
  min-width: 0;
  color: var(--paap-text);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
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
  grid-template-columns: minmax(220px, 0.85fr) minmax(240px, 1.15fr);
  gap: var(--paap-space-3);
  align-items: start;
}
.cds-image-field--full {
  grid-column: 1 / -1;
}
.cds-image-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.cds-image-field .cds-text-input {
  width: 100%;
  min-width: 0;
}
.cds-label {
  font-size: var(--paap-fs-label);
  font-weight: 400;
  line-height: 1.33333;
  letter-spacing: 0.32px;
  color: var(--paap-muted);
}
.cds-text-input {
  height: 32px;
  padding: 0 8px;
  background: var(--paap-panel-subtle);
  border: 1px solid var(--paap-border-strong);
  border-radius: 0;
  font-size: var(--paap-fs-body);
  line-height: 1.4;
  color: var(--paap-text);
  outline: none;
  min-width: 0;
  text-overflow: ellipsis;
  transition: border-color 110ms;
}
.cds-text-input:focus {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
}
.cds-text-input[readonly] {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  cursor: default;
}
.cds-text-input::placeholder {
  color: var(--paap-muted);
}
.cds-image-preview {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr);
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  padding: 4px 8px;
  background: var(--paap-panel);
  border-left: 2px solid var(--paap-accent);
}
.cds-image-preview__icon {
  flex-shrink: 0;
  color: var(--paap-accent);
}
.cds-image-preview__label {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  white-space: nowrap;
}
.cds-image-preview__text {
  font-family: monospace;
  font-size: var(--paap-fs-label);
  color: var(--paap-text);
  min-width: 0;
  overflow-wrap: anywhere;
  white-space: normal;
}
.config-kv-grid,
.config-ref-grid,
.config-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-3);
}
.service-config-form-grid {
  gap: 16px 24px;
}
.service-config-field {
  min-width: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: var(--paap-panel);
}
.service-config-field .bx--text-input,
.service-config-field .bx--select-input {
  min-height: 40px;
  padding-top: 10px;
  padding-bottom: 10px;
  background: var(--paap-panel-subtle);
  font-size: var(--paap-fs-compact);
}
.service-config-field:focus-within {
  background: var(--paap-panel);
}
.config-kv-grid div,
.config-ref-grid div {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.config-kv-grid span,
.config-ref-grid span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.config-kv-grid strong,
.config-ref-grid strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 550;
  overflow-wrap: anywhere;
}
.service-summary-grid {
  margin-top: var(--paap-space-3);
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
}
.service-volume-grid {
  display: grid;
  gap: var(--paap-space-3);
}
.service-volume-card {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.service-volume-card.disabled {
  background: var(--paap-panel-subtle);
}
.service-volume-card__head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--paap-space-3);
  align-items: center;
}
.service-volume-card__head > div {
  display: grid;
  min-width: 0;
  gap: 3px;
}
.service-volume-card strong,
.service-volume-card small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.service-volume-card strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
}
.service-volume-card small,
.service-volume-size span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.service-volume-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  white-space: nowrap;
  cursor: pointer;
}
.service-volume-toggle input {
  width: 16px;
  height: 16px;
}
.service-volume-size {
  display: grid;
  grid-template-columns: 80px minmax(160px, 240px);
  align-items: center;
  gap: var(--paap-space-2);
}
.service-volume-size .bx--text-input:disabled {
  color: var(--paap-muted);
  background: var(--paap-panel-subtle);
}
.service-volume-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}
.service-volume-preset {
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  cursor: pointer;
}
.service-volume-preset:hover:not(:disabled),
.service-volume-preset.active {
  border-color: var(--paap-accent);
  background: var(--paap-panel);
  color: var(--paap-accent);
}
.service-volume-preset:disabled {
  cursor: not-allowed;
  opacity: 0.48;
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
  font-size: var(--paap-fs-small);
  font-weight: 600;
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
  font-size: var(--paap-fs-label);
}
.service-topology-node-row small {
  grid-column: 1 / -1;
  color: var(--paap-muted);
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
.config-binding-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--paap-space-2);
  align-items: center;
}
.config-binding-list {
  display: grid;
  gap: var(--paap-space-2);
  margin-top: var(--paap-space-3);
}
.component-suggestion-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: var(--paap-space-3);
}
.component-preset-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
  margin-top: var(--paap-space-3);
}
.component-preset-card,
.component-discovered-row {
  min-width: 0;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.component-preset-card {
  display: grid;
  gap: 5px;
  padding: 10px;
}
.component-preset-card:hover,
.component-discovered-row:hover {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
}
.component-preset-card strong,
.component-discovered-row strong {
  min-width: 0;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-preset-card small,
.component-discovered-row small {
  min-width: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-preset-card code,
.component-discovered-row code {
  display: block;
  min-width: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  line-height: 1.35;
  overflow-wrap: anywhere;
  white-space: normal;
}
.component-template-panel {
  display: grid;
  gap: var(--paap-space-2);
  margin-top: var(--paap-space-3);
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
  min-width: 0;
}
.component-template-head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--paap-space-2);
}
.component-template-head > span {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}
.component-template-actions,
.component-template-editor-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.component-template-editor {
  display: grid;
  gap: var(--paap-space-2);
  padding: 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
  min-width: 0;
}
.component-preset-grid--custom {
  margin-top: 0;
}
.component-preset-card--custom {
  padding: 0;
  overflow: hidden;
}
.component-preset-card-main {
  display: grid;
  gap: 5px;
  width: 100%;
  padding: 10px;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.component-preset-card--custom > .text-btn {
  margin: 0 10px 10px;
}
.component-template-badge {
  justify-self: start;
  margin: 0 10px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  padding: 2px 8px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  line-height: 18px;
}
.component-config-flow {
  display: grid;
  gap: var(--paap-space-4);
  min-width: 0;
}
.component-config-flow--guided {
  gap: 12px;
}
.component-template-picker {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: var(--paap-panel);
}
.component-template-select {
  display: grid;
  grid-template-columns: minmax(112px, 0.28fr) minmax(0, 0.72fr);
  align-items: center;
  gap: 14px;
  min-width: 0;
}
.component-template-select > span,
.component-template-field-label,
.component-connected-list > span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.component-template-helper {
  margin: 0;
  padding-left: calc(28% + 14px);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.45;
}
.component-config-actions,
.component-advanced-tools {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.component-template-advanced-config {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 10px 0;
  border: 0;
  border-top: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.component-template-advanced-config > summary {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  color: var(--paap-muted);
  cursor: pointer;
  list-style: none;
}
.component-template-advanced-config > summary::-webkit-details-marker {
  display: none;
}
.component-template-advanced-config > summary span {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}
.component-template-advanced-config > summary small {
  min-width: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  text-align: right;
}
.component-template-advanced-config:not([open]) {
  padding-top: 8px;
  padding-bottom: 8px;
}
.component-template-advanced-config[open] > summary {
  padding-bottom: 8px;
  border-bottom: 1px solid var(--paap-border);
}
.component-current-config-panel {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.component-current-config-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-2);
  min-width: 0;
}
.component-current-config-head span {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}
.component-current-config-head small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.component-current-config-list {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.component-current-config-row {
  display: grid;
  grid-template-columns: minmax(150px, 0.9fr) minmax(0, 1.1fr);
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 7px 8px;
  border: 1px solid var(--paap-panel-subtle);
  background: var(--paap-surface);
}
.component-current-config-row span {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.component-current-config-row strong,
.component-current-config-row small,
.component-current-config-row code {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--paap-fs-label);
}
.component-current-config-row strong {
  color: var(--paap-text);
  font-weight: 600;
}
.component-current-config-row small {
  color: var(--paap-muted);
}
.component-current-config-row code {
  color: var(--paap-muted);
}
.component-config-warning {
  color: var(--paap-danger);
  font-size: var(--paap-fs-label);
}
.component-template-save-note {
  margin: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.45;
}
.component-connected-list {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
  margin-bottom: var(--paap-space-3);
}
.component-connected-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 8px 0;
}
.component-connected-row strong,
.component-connected-row small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-connected-row strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.component-connected-row small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.component-config-step {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
  padding-bottom: var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
}
.component-config-step:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}
.component-config-step__head {
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr) minmax(120px, 180px);
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
}
.component-config-step__head > span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}
.component-config-step__head strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-config-step__head .text-btn {
  justify-self: end;
}
.nginx-route-list {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.component-file-mount-panel {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 2px 0 10px;
  border-bottom: 1px solid var(--paap-border);
}
.component-file-mount-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-2);
  min-width: 0;
}
.component-file-mount-head span {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}
.component-file-mount-list {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.component-file-mount-row {
  display: grid;
  grid-template-columns: minmax(112px, 0.34fr) minmax(0, 0.56fr) auto;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.component-file-mount-source {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.component-file-mount-source strong,
.component-file-mount-source small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-file-mount-source strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.component-file-mount-source small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.component-file-readonly {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 32px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  white-space: nowrap;
}
.component-file-readonly input {
  width: 16px;
  height: 16px;
  margin: 0;
}
.nginx-route-panel {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 2px 0 10px;
  border-bottom: 1px solid var(--paap-border);
}
.nginx-route-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-2);
  min-width: 0;
}
.nginx-route-head span {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}
.component-template-advanced {
  margin-top: 0;
}
.nginx-route-row {
  display: grid;
  grid-template-columns: minmax(112px, 0.72fr) minmax(180px, 1fr) 32px;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 0;
}
.nginx-route-row .bx--text-input,
.nginx-route-row .bx--select-input {
  min-width: 0;
  font-size: var(--paap-fs-label);
}
.nginx-route-apply {
  justify-self: start;
}
.component-discovered-list {
  display: grid;
  gap: var(--paap-space-2);
  margin-top: var(--paap-space-3);
}
.config-section-subtitle {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 600;
  text-transform: uppercase;
}
.component-discovered-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--paap-space-2);
  align-items: center;
  padding: 8px 10px;
}
.component-discovered-row span {
  display: grid;
  min-width: 0;
  gap: 3px;
}
.component-suggestion-chip {
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-text);
  font: inherit;
  font-size: var(--paap-fs-label);
  line-height: 1;
  padding: 7px 9px;
  cursor: pointer;
}
.component-suggestion-chip:hover {
  border-color: var(--paap-accent);
  color: var(--paap-accent);
}
.config-binding-row {
  display: grid;
  grid-template-columns: minmax(130px, 1fr) minmax(0, 1fr) auto;
  gap: var(--paap-space-2);
  align-items: center;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.config-binding-row span {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.config-binding-row small,
.config-binding-row code {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.config-env-row {
  display: grid;
  grid-template-columns: minmax(160px, 1.2fr) 100px minmax(140px, 1fr) minmax(120px, 0.8fr) auto;
  gap: var(--paap-space-2);
  align-items: center;
}
.config-env-row .bx--text-input,
.config-env-row .bx--select-input {
  font-size: var(--paap-fs-compact);
  min-width: 0;
}
.config-env-row .danger {
  color: var(--paap-danger);
}
.config-env-row--managed {
  grid-template-columns: minmax(160px, 1fr) auto minmax(220px, 1.4fr) auto;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.config-env-managed-name,
.config-env-managed-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.config-env-managed-name {
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.config-env-managed-value {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.config-env-managed-secret {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  white-space: nowrap;
}
.config-readonly-row {
  display: grid;
  grid-template-columns: minmax(120px, 0.8fr) minmax(0, 1.2fr);
  gap: var(--paap-space-3);
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.config-readonly-row strong,
.config-readonly-row span {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: var(--paap-fs-label);
}
.backup-list {
  display: grid;
  gap: var(--paap-space-2);
}
.backup-row {
  display: grid;
  grid-template-columns: minmax(160px, 1fr) auto auto auto minmax(130px, 0.8fr);
  align-items: center;
  gap: var(--paap-space-3);
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.backup-row > div {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.backup-row strong,
.backup-row small,
.backup-row span,
.backup-row code {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--paap-fs-label);
}
.backup-row strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
}
.backup-row small {
  color: var(--paap-muted);
}
.backup-row span {
  color: var(--paap-muted);
}
.backup-row code {
  color: var(--paap-text);
}
.runtime-console-panel {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
}
.runtime-console-status {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.runtime-console-status span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}
.runtime-console-status span::before {
  content: "";
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--paap-border-strong);
}
.runtime-console-status.connected span::before {
  background: var(--paap-success);
}
.runtime-console-status.connecting span::before {
  background: var(--paap-accent);
}
.runtime-console-status small {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.runtime-console-output {
  height: 52vh;
  min-height: 320px;
  min-width: 0;
  margin: 0;
  padding: 8px;
  border: 1px solid #393939;
  border-radius: var(--paap-radius-sm, 6px);
  background: #161616;
  overflow: hidden;
}
.runtime-console-hint {
  margin: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.5;
}
.runtime-logs-panel {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
}
.runtime-log-meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--paap-space-2);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.runtime-log-meta span {
  padding: 3px 8px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
}
.runtime-log-list {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
}
.runtime-log-sample {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.runtime-log-sample header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--paap-space-2);
  align-items: center;
  min-width: 0;
}
.runtime-log-sample strong,
.runtime-log-sample small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.runtime-log-sample strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
}
.runtime-log-sample small {
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
}
.runtime-log-output {
  min-height: 220px;
  max-height: 44vh;
  min-width: 0;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid #393939;
  border-radius: 0;
  background: #161616;
  color: #ffffff;
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.runtime-metrics-panel {
  display: grid;
  gap: var(--paap-space-3);
}
.runtime-metric-cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--paap-space-2);
}
.runtime-metric-card {
  display: grid;
  gap: 4px;
  padding: 10px 12px;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
}
.runtime-metric-card span,
.runtime-metric-card small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.runtime-metric-card strong {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--paap-text);
  font-size: 18px;
  line-height: 1.15;
}
.runtime-metric-chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
  min-width: 0;
}
.runtime-metric-chart {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
}
.runtime-metric-chart header,
.runtime-metric-chart footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-2);
  min-width: 0;
}
.runtime-metric-chart header span,
.runtime-metric-chart footer span {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.runtime-metric-chart header strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.runtime-metric-chart svg {
  display: block;
  width: 100%;
  height: 140px;
  overflow: visible;
}
.runtime-metric-chart line {
  stroke: var(--paap-border);
  stroke-width: 1;
}
.runtime-metric-chart path {
  fill: var(--paap-accent-fill-2);
}
.runtime-metric-chart polyline {
  fill: none;
  stroke: var(--paap-accent);
  stroke-width: 2.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.runtime-metric-chart circle {
  fill: var(--paap-panel);
  stroke: var(--paap-accent);
  stroke-width: 2;
}
.runtime-metric-list {
  display: grid;
  gap: var(--paap-space-2);
}
.runtime-metric-row {
  display: grid;
  gap: var(--paap-space-2);
  padding: 10px 12px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.runtime-metric-row__head,
.runtime-metric-bars {
  display: grid;
  gap: var(--paap-space-2);
}
.runtime-metric-row__head {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
}
.runtime-metric-row__head span {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.runtime-metric-row__head strong,
.runtime-metric-row__head small {
  min-width: 0;
  overflow-wrap: anywhere;
}
.runtime-metric-row__head small,
.runtime-metric-bars span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.runtime-metric-bars {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.runtime-metric-bars > div {
  display: grid;
  grid-template-columns: auto auto;
  gap: 5px 8px;
  align-items: center;
}
.runtime-metric-bars strong {
  justify-self: end;
  font-size: var(--paap-fs-label);
}
.runtime-metric-bars i {
  grid-column: 1 / -1;
  display: block;
  height: 6px;
  overflow: hidden;
  background: var(--paap-panel-subtle);
  border: 1px solid var(--paap-border);
}
.runtime-metric-bars b {
  display: block;
  height: 100%;
  background: var(--paap-accent);
}
.config-empty {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.config-section details {
  display: grid;
  gap: var(--paap-space-3);
}
.config-section summary {
  cursor: pointer;
  color: var(--paap-text);
  font-weight: 600;
}
.config-section textarea {
  min-height: 56px;
  resize: vertical;
}
.relationship-modal { max-width: 560px; }
.relationship-help {
  margin: 0 0 var(--paap-space-4);
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
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
.relationship-target strong { color: var(--paap-text); font-size: var(--paap-fs-compact); font-weight: 600; }
.relationship-target small { color: var(--paap-muted); font-size: var(--paap-fs-label); }
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
  font-weight: 600;
  line-height: 1.3;
  overflow-wrap: anywhere;
}
.component-detail-head p {
  margin: 4px 0 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}
.component-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
}
.capability-config-section,
.external-capability-form {
  display: grid;
  gap: var(--paap-space-3);
}
.capability-summary-grid {
  margin-top: 0;
}
.capability-summary-grid .capability-secret-row {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
}
.capability-summary-grid .capability-secret-row span {
  grid-column: 1 / -1;
}
.capability-summary-grid .capability-secret-row strong {
  white-space: normal;
  word-break: break-all;
}
.capability-row-secret-toggle {
  width: 32px;
  height: 32px;
}
.capability-checkbox-field {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 38px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}
.capability-checkbox-field input {
  margin: 0;
}
.password-field-wrap {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 40px;
  min-width: 0;
}
.password-field-wrap .password-field-input {
  border-right: 0;
}
.password-visible-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 1px solid var(--paap-border-strong);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
  transition: border-color 110ms, color 110ms, box-shadow 110ms;
}
.password-visible-toggle:hover:not(:disabled) {
  color: var(--paap-accent);
}
.password-visible-toggle:disabled {
  cursor: progress;
  opacity: 0.5;
}
.password-field-wrap:focus-within .password-field-input,
.password-field-wrap:focus-within .password-visible-toggle {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
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
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 700;
  text-transform: uppercase;
}
.component-detail-grid strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
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
  font-size: var(--paap-fs-small);
  font-weight: 600;
}
.component-relation-panel button {
  min-height: 28px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  text-align: left;
  cursor: pointer;
}
.component-relation-panel button:hover { border-color: var(--paap-accent); color: var(--paap-accent); }
.component-relation-panel small { color: var(--paap-muted); font-size: var(--paap-fs-label); }
.component-count-label { color: var(--paap-muted); font-size: var(--paap-fs-label); }
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
  font-size: var(--paap-fs-label);
  text-align: left;
  vertical-align: middle;
}
.component-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
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
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.component-table td small {
  display: block;
  max-width: 220px;
  overflow: hidden;
  color: var(--paap-muted);
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
.status-dot.running,
.status-dot.linked { background: var(--paap-success); }
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
.empty-text { color: var(--paap-muted); line-height: 1.5; max-width: 420px; font-size: var(--paap-fs-body); }
.capability-install-panel {
  display: grid;
  gap: var(--paap-space-3);
  width: min(620px, 100%);
  margin-top: var(--paap-space-6);
  text-align: left;
}
.capability-install-head {
  display: grid;
  gap: 4px;
}
.capability-install-head span {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 700;
}
.capability-install-head small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.45;
}
.capability-install-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--paap-space-2);
}
.capability-install-card {
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr);
  align-items: start;
  gap: var(--paap-space-3);
  min-height: 68px;
  padding: 12px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
}
.capability-install-card:hover {
  border-color: var(--paap-accent);
  background: var(--paap-panel);
}
.capability-install-card.disabled,
.capability-install-card:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}
.capability-install-card.disabled:hover,
.capability-install-card:disabled:hover {
  border-color: var(--paap-border);
  background: var(--paap-panel);
}
.capability-install-card span:last-child {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.capability-install-card strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.capability-install-card small {
  min-width: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.35;
}

/* Modal */
.modal-container { background: var(--paap-panel); width: 100%; max-height: 90vh; overflow-y: auto; border-radius: var(--paap-radius); border: 1px solid var(--paap-border); box-shadow: var(--paap-shadow-lg); position: relative; }
.confirm-modal { max-width: 460px; }
.modal-header { display: flex; justify-content: space-between; align-items: flex-start; padding: var(--paap-space-5) var(--paap-space-6); border-bottom: 1px solid var(--paap-border); }
.modal-label { font-size: var(--paap-fs-small); color: var(--paap-muted); letter-spacing: 0.04em; margin-bottom: 4px; text-transform: uppercase; font-weight: 600; }
.modal-heading { font-size: 18px; font-weight: 600; color: var(--paap-text); line-height: 1.3; margin: 0; }
.modal-close { background: none; border: 1px solid var(--paap-border); color: var(--paap-muted); cursor: pointer; padding: 4px; line-height: 1; transition: background 110ms, color 110ms, border-color 110ms; margin-top: -4px; border-radius: var(--paap-radius-sm); width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; }
.modal-close:hover { background: var(--paap-panel-subtle); color: var(--paap-text); }
.modal-content { padding: var(--paap-space-6); }
.capability-action-field { margin: 0; }
.capability-action-textarea { min-height: 140px; resize: vertical; }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--paap-space-2); padding: var(--paap-space-4) var(--paap-space-6); border-top: 1px solid var(--paap-border); }
.modal-error {
  border: 1px solid var(--paap-danger);
  background: var(--paap-panel);
  color: var(--paap-danger);
  border-radius: 0;
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
  line-height: 1.4;
  margin: 0;
}
.confirm-text {
  margin: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  line-height: 1.5;
}

/* Form */
.bx--form-item { margin-bottom: var(--paap-space-5); }
.bx--label { display: block; font-size: var(--paap-fs-label); color: var(--paap-muted); margin-bottom: 6px; font-weight: 500; }
.bx--text-input {
  width: 100%; padding: 9px 12px; font-size: var(--paap-fs-body);
  border: 1px solid var(--paap-border-strong); border-radius: 0;
  background: var(--paap-panel); color: var(--paap-text); outline: none;
  font-family: inherit; transition: border-color 110ms, box-shadow 110ms;
}
.bx--text-input:focus { border-color: var(--paap-accent); box-shadow: inset 0 0 0 1px var(--paap-accent); }
.bx--text-input::placeholder { color: var(--paap-muted); }
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
  border-radius: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  font-weight: 500;
  cursor: pointer;
  background: var(--paap-panel);
  transition: border-color 110ms, background 110ms, color 110ms;
}
.delivery-option.active {
  border-color: var(--paap-accent);
  background: var(--paap-panel);
  color: var(--paap-text);
}
.delivery-option input { margin: 0; }

.bx--select { position: relative; }
.bx--select-input {
  width: 100%; padding: 9px 36px 9px 12px; font-size: var(--paap-fs-body);
  border: 1px solid var(--paap-border-strong); border-radius: 0;
  background: var(--paap-panel); color: var(--paap-text); outline: none;
  appearance: none; cursor: pointer; font-family: inherit;
  transition: border-color 110ms, box-shadow 110ms;
}
.bx--select-input:focus { border-color: var(--paap-accent); box-shadow: inset 0 0 0 1px var(--paap-accent); }
.bx--select__arrow { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); pointer-events: none; }

/* Service select grid in modal */
.no-data { color: var(--paap-muted); text-align: center; padding: var(--paap-space-8); }
.error-text { color: var(--paap-danger); }
.service-picker-summary { display: flex; align-items: center; gap: var(--paap-space-3); margin-bottom: var(--paap-space-4); padding: var(--paap-space-3) var(--paap-space-4); background: var(--paap-panel-subtle); border-left: 3px solid var(--paap-accent); border-radius: 0 var(--paap-radius-xs) var(--paap-radius-xs) 0; }
.summary-pill { display: inline-flex; align-items: center; justify-content: center; padding: 2px 8px; font-size: var(--paap-fs-small); font-weight: 600; background: var(--paap-accent-soft); color: var(--paap-accent); border-radius: var(--paap-radius-xs); }
.summary-text { color: var(--paap-muted); font-size: var(--paap-fs-compact); line-height: 1.4; }
.service-provision-mode-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--paap-space-2); margin-bottom: var(--paap-space-4); }
.service-provision-mode-card {
  display: flex; flex-direction: column; gap: 4px; min-height: 78px;
  border: 1px solid var(--paap-border); background: var(--paap-panel); color: var(--paap-text);
  padding: var(--paap-space-3); text-align: left; cursor: pointer; border-radius: var(--paap-radius-sm);
}
.service-provision-mode-card strong { font-size: var(--paap-fs-compact); line-height: 1.25; }
.service-provision-mode-card small { color: var(--paap-muted); font-size: var(--paap-fs-small); line-height: 1.35; }
.service-provision-mode-card:hover { border-color: var(--paap-accent); }
.service-provision-mode-card.selected { border-color: var(--paap-accent); background: var(--paap-accent-soft); }
.service-provision-mode-card.disabled { cursor: not-allowed; opacity: 0.48; }
.service-provision-mode-card.disabled:hover { border-color: var(--paap-border); }
.service-select-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--paap-space-3); }
.service-select-grid--compact { margin-top: var(--paap-space-3); }
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
.select-radio.selected { background: var(--paap-accent); border-color: var(--paap-accent); color: var(--paap-panel); }
.select-radio.disabled { background: var(--paap-panel-subtle); border-color: var(--paap-border-strong); color: var(--paap-muted); }

.service-name-row { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; }
.service-name { font-size: 15px; line-height: 1.4; font-weight: 600; }
.service-desc { color: var(--paap-muted); margin-top: 4px; line-height: 1.4; font-size: var(--paap-fs-compact); }
.shared-service-choice { margin-top: var(--paap-space-5); padding-top: var(--paap-space-4); border-top: 1px solid var(--paap-border-subtle); }

/* Buttons */
.bx--btn { display: inline-flex; align-items: center; justify-content: center; font-size: var(--paap-fs-compact); font-weight: 500; cursor: pointer; outline: none; border: none; height: 36px; padding: 0 14px; transition: all 0.15s; border-radius: var(--paap-radius-sm); gap: 6px; font-family: inherit; }
.bx--btn--primary { background: var(--paap-accent); color: #ffffff; }
.bx--btn--primary:hover { background: var(--paap-accent-hover); }
.bx--btn--secondary { background: var(--paap-panel); color: var(--paap-accent); border: 1px solid var(--paap-accent); }
.bx--btn--secondary:hover { background: var(--paap-accent-soft); color: var(--paap-accent-hover); border-color: var(--paap-accent-hover); }
.bx--btn--danger { background: var(--paap-danger); color: #ffffff; }
.bx--btn--danger:hover { background: var(--paap-danger-hover); }
.bx--btn--ghost { background: transparent; color: var(--paap-muted); border: 1px solid var(--paap-border); }
.bx--btn--ghost:hover { background: var(--paap-panel-subtle); color: var(--paap-text); }
.bx--btn:disabled { opacity: 0.5; cursor: not-allowed; }
.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  font: inherit;
  cursor: pointer;
  transition: background 110ms, border-color 110ms, color 110ms;
}
.icon-btn:hover {
  border-color: var(--paap-accent);
  background: var(--paap-panel-subtle);
  color: var(--paap-accent);
}
.icon-btn:focus-visible {
  outline: 2px solid var(--paap-accent);
  outline-offset: 1px;
}
.icon-btn--compact {
  width: 30px;
  height: 30px;
}
.icon-btn--danger:hover {
  border-color: var(--paap-danger);
  color: var(--paap-danger);
}

/* Service icon wrap */
.service-icon-wrap { width: 32px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; align-self: center; background: transparent; color: var(--paap-muted); }
.service-icon-wrap.deploy   { color: var(--paap-service-deploy); }
.service-icon-wrap.git      { color: var(--paap-service-git); }
.service-icon-wrap.log      { color: var(--paap-service-log); }
.service-icon-wrap.monitor  { color: var(--paap-service-monitor); }
.service-icon-wrap.registry { color: var(--paap-service-registry); }
.service-icon-wrap.harbor   { color: var(--paap-service-deploy); }
.service-icon-wrap.ci       { color: var(--paap-service-ci); }
.service-icon-wrap.mysql      { color: var(--paap-service-mysql); }
.service-icon-wrap.postgresql { color: var(--paap-service-deploy); }
.service-icon-wrap.mongodb    { color: var(--paap-service-monitor); }
.service-icon-wrap.redis      { color: var(--paap-service-ci); }
.service-icon-wrap.rabbitmq   { color: var(--paap-service-registry); }
.service-icon-wrap.kafka      { color: var(--paap-service-log); }
.service-icon-wrap.minio      { color: var(--paap-service-deploy); }
.service-icon-wrap.infra      { color: var(--paap-muted); }

/* Tag status dot */
.tag-dot { width: 6px; height: 6px; border-radius: 50%; display: inline-block; margin-right: 5px; vertical-align: middle; margin-top: -1px; background: var(--paap-border-strong); }
.tag-dot.running   { background: var(--paap-emerald); }
.tag-dot.installing{ backgound: var(--paap-danger); }
.tag-dot.failed    { background: var(--paap-danger); }
.tag-dot.deleting  { background: var(--paap-muted); }
.tag-dot.pending,
.tag-dot.draft     { background: var(--paap-border-strong); }

/* Tags */
.bx--tag { font-size: var(--paap-fs-small); padding: 2px 8px; border-radius: var(--paap-radius-full); font-weight: 500; }
.bx--tag--sm { font-size: 10px; padding: 1px 6px; }
.bx--tag--blue { background: var(--paap-accent-soft); color: var(--paap-accent); }
.bx--tag--green { background: var(--paap-success-soft); color: var(--paap-emerald); }
.bx--tag--gray { background: var(--paap-border); color: var(--paap-text); }
.bx--tag--red { background: var(--paap-danger-soft); color: var(--paap-danger); }

/* Responsive */
@media (max-width: 672px) {
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
  .config-drawer-shell {
    padding: 64px 8px 8px;
  }
  .config-drawer {
    width: calc(100vw - 16px);
  }
  .config-deployment-meta,
  .cds-image-fields,
  .config-variable-row,
  .component-template-field,
  .component-config-step__head,
  .runtime-metric-chart-grid {
    grid-template-columns: 1fr;
  }
  .component-config-step__head > span {
    justify-content: center;
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
  .service-provision-mode-grid { grid-template-columns: 1fr; }
  .service-select-grid { grid-template-columns: 1fr; }
  .form-row { grid-template-columns: 1fr; }
  .config-binding-form,
  .config-binding-row,
  .component-template-field,
  .component-template-select,
  .nginx-route-row,
  .component-preset-grid,
  .component-discovered-row {
    grid-template-columns: 1fr;
  }
  .component-template-helper {
    padding-left: 0;
  }
  .config-drawer-header,
  .config-drawer-body,
  .config-drawer-footer {
    padding-right: 18px;
    padding-left: 18px;
  }
  .config-drawer-tabs {
    padding: 0 18px;
  }
}
</style>
