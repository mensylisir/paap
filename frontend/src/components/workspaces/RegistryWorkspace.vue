<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="ws-tabs">
        <button
          v-for="tab in availableTabs"
          :key="tab.key"
          class="ws-tab"
          :class="{ active: activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>

      <div v-if="runtimeTrust" class="runtime-trust">
        <div class="trust-head">
          <div>
            <div class="trust-title">节点运行时配置待确认</div>
            <div class="trust-sub">
              PAAP 内部调用工具继续使用集群内服务地址；业务运行实例拉镜像时使用下面这个 HTTPS registry。内网默认按自签或企业 CA 处理，内网自签 CA 需要下载后交给集群管理员，配置节点运行时信任以及 DNS/路由。
            </div>
          </div>
          <span class="badge orange">{{ runtimeTrust.status }}</span>
        </div>
        <div class="trust-grid">
          <div>
            <span class="trust-label">Registry Host</span>
            <code class="mono">{{ runtimeTrust.annotations?.registryHost || runtimeTrust.name }}</code>
          </div>
          <div>
            <span class="trust-label">Endpoint</span>
            <code class="mono">{{ runtimeTrust.annotations?.registryEndpoint || '-' }}</code>
          </div>
          <div>
            <span class="trust-label">containerd</span>
            <code class="mono">{{ runtimeTrust.annotations?.containerdHostsToml || '-' }}</code>
          </div>
          <div>
            <span class="trust-label">Docker</span>
            <code class="mono">{{ runtimeTrust.annotations?.dockerCertPath || '-' }}</code>
          </div>
          <div>
            <span class="trust-label">证书下载地址</span>
            <code class="mono">{{ runtimeTrust.annotations?.certificateURL || '-' }}</code>
          </div>
        </div>
        <div class="trust-note">
          <a
            v-if="runtimeTrust.annotations?.certificateURL"
            :href="runtimeTrust.annotations.certificateURL"
            class="act-btn primary cert-download"
            download
          >
            下载 CA 证书
          </a>
          <span>把证书链接、Registry Host 和运行时路径一起交给集群管理员配置节点信任。</span>
          <span>PAAP 会在可读取 CA 时自动同步给构建服务。</span>
          <span>可选兜底清单：</span><code class="mono">{{ runtimeTrust.annotations?.agentManifest || 'deploy/k8s/paap-node-registry-agent.yaml' }}</code>
        </div>
      </div>

      <div v-if="activeTab === 'repos'" class="tab-panel">
        <div v-if="repos.length" class="repo-list">
          <div v-for="repo in repos" :key="repo.name" class="card repo-card">
            <div class="repo-header">
              <div class="repo-title">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink:0"><path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>
                <span class="repo-name">{{ repo.name }}</span>
              </div>
              <span class="badge" :class="repo.status === 'Ready' ? 'green' : 'gray'">{{ repo.status }}</span>
            </div>

            <div class="repo-meta">
              <span v-if="repo.annotations?.project" class="chip">Project: {{ repo.annotations.project }}</span>
              <span v-if="repo.annotations?.artifactCount != null" class="chip">{{ repo.annotations.artifactCount }} artifacts</span>
            </div>

            <div v-if="repo.annotations?.tags?.length" class="repo-tags chip-list">
              <span v-for="tag in repo.annotations.tags.slice(0, 8)" :key="tag" class="chip">{{ tag }}</span>
              <span v-if="repo.annotations.tags.length > 8" class="chip">+{{ repo.annotations.tags.length - 8 }}</span>
            </div>

            <div v-if="repo.annotations?.digest" class="repo-digest mono">{{ repo.annotations.digest }}</div>

            <div v-if="repo.externalUrl" class="repo-footer">
              <a :href="repo.externalUrl" target="_blank" class="link external">{{ repoLinkLabel(repo) }}</a>
            </div>
            <div v-if="repo.actions?.length" class="repo-actions">
              <button
                v-for="action in repo.actions"
                :key="action.key || action.label"
                type="button"
                class="act-btn"
                :class="{ danger: action.tone === 'danger', primary: action.tone === 'primary' }"
                :title="action.description"
                @click="emit('action', action, action.target || repo.name)"
              >
                {{ action.label }}
              </button>
            </div>
          </div>
        </div>
        <div v-else class="empty-state empty-state--compact"><p class="empty-state__title">暂无镜像仓库数据</p></div>
      </div>

      <div v-if="activeTab === 'resources'" class="tab-panel">
        <div v-if="resources.length" class="table-wrap">
          <table class="data-table">
            <thead>
              <tr><th>名称</th><th>类型</th><th>状态</th><th>说明</th></tr>
            </thead>
            <tbody>
              <tr v-for="r in resources" :key="r.name + r.type">
                <td class="cell-name">
                  <a v-if="r.externalUrl" :href="r.externalUrl" target="_blank" class="link external">{{ r.name }}</a>
                  <span v-else>{{ r.name }}</span>
                </td>
                <td><span class="badge blue">{{ r.type }}</span></td>
                <td><span class="badge" :class="statusBadge(r.status)">{{ r.status }}</span></td>
                <td class="cell-desc">{{ r.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-state empty-state--compact"><p class="empty-state__title">暂无资源数据</p></div>
      </div>
    </template>
  </ToolWorkspaceFrame>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'

const props = defineProps<{
  resources: WorkspaceResource[]
}>()
const emit = defineEmits<{
  (event: 'action', action: WorkspaceAction, target?: string): void
}>()

const repos = computed(() =>
  props.resources.filter(
    r => r.type === 'Repository' || r.type === 'Image Repository' || r.type === 'Harbor Repository'
  )
)
const runtimeTrust = computed(() => props.resources.find(r => r.type === 'Runtime Trust'))
const repoLinkLabel = (repo: WorkspaceResource) =>
  repo.type === 'Harbor Repository' || repo.annotations?.serviceType === 'harbor'
    ? '在 Harbor 中查看'
    : '查看 Registry Tags'

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (repos.value.length) tabs.push({ key: 'repos', label: '镜像仓库', count: repos.value.length })
  tabs.push({ key: 'resources', label: '资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(repos.value.length ? 'repos' : 'resources')

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('synced')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('degraded')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.repo-list { display: flex; flex-direction: column; gap: var(--paap-space-3); }
.repo-card { display: flex; flex-direction: column; gap: var(--paap-space-2); }
.repo-header { display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-3); }
.repo-title { display: flex; align-items: center; gap: var(--paap-space-2); font-weight: 600; font-size: var(--paap-fs-body); color: var(--paap-text); }
.repo-name { word-break: break-all; }
.repo-meta { display: flex; gap: 6px; flex-wrap: wrap; }
.repo-tags { margin-top: 2px; }
.repo-digest { color: var(--paap-muted); font-size: var(--paap-fs-small); font-family: var(--paap-mono); }
.repo-footer { margin-top: var(--paap-space-1); }
.repo-actions { display: flex; flex-wrap: wrap; gap: 6px; margin-top: var(--paap-space-1); }
.runtime-trust {
  border: 1px solid var(--paap-warning-soft);
  background: var(--paap-warning-soft);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-4) var(--paap-space-5);
  margin-bottom: var(--paap-space-5);
}
.trust-head { display: flex; justify-content: space-between; align-items: flex-start; gap: var(--paap-space-3); margin-bottom: var(--paap-space-3); }
.trust-title { font-weight: 600; color: var(--paap-orange-text); margin-bottom: 3px; }
.trust-sub { font-size: var(--paap-fs-label); color: var(--paap-orange-text); line-height: 1.5; }
.trust-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: var(--paap-space-3); }
.trust-grid > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.trust-label { font-size: var(--paap-fs-small); color: var(--paap-orange-text); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
.trust-grid code, .trust-note code { word-break: break-all; color: var(--paap-orange-text, #7c2d12); font-family: var(--paap-mono); }
.trust-note { margin-top: var(--paap-space-3); font-size: var(--paap-fs-label); color: var(--paap-orange-text); }
.cert-download { display: inline-flex; align-items: center; text-decoration: none; margin-right: var(--paap-space-3); }
</style>
