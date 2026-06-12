<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="ws-toolbar">
        <div class="ws-tabs">
          <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
            {{ tab.label }}
            <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
          </button>
        </div>
        <div class="ws-toolbar-actions">
          <button type="button" class="act-btn primary" @click="emit('action', createRepoAction)">创建代码仓</button>
          <button type="button" class="act-btn" @click="emit('action', userKeyAction)">配置公钥</button>
        </div>
      </div>

      <div v-if="activeTab === 'repos'" class="tab-panel">
        <!-- Repo list -->
        <div v-if="!selectedRepo" class="repo-home">
          <div class="repo-home-main">
            <div class="repo-home-head">
              <div>
                <h3>代码仓库</h3>
                <p>按仓库查看源码、流水线和 GitOps 清单，点击仓库后进入文件树和代码编辑视图。</p>
              </div>
            </div>
            <div v-if="repos.length" class="repo-list">
              <div v-for="repo in repos" :key="repo.name" class="repo-card" @click="selectRepo(repo)">
                <div class="repo-main">
                  <div class="repo-header">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/></svg>
                    <span class="repo-name">{{ repo.name }}</span>
                    <span v-if="repo.annotations?.repositoryRole" class="visibility role">{{ repoRoleLabel(repo.annotations.repositoryRole) }}</span>
                    <span v-if="repo.annotations?.private" class="visibility private">Private</span>
                    <span v-else class="visibility public">Public</span>
                  </div>
                  <div class="repo-desc">默认分支 {{ repo.annotations?.branch || 'main' }} · {{ repo.annotations?.language || '未知语言' }}</div>
                </div>
                <div class="repo-meta">
                  <span class="meta-item" title="Stars"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 17.27L18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z"/></svg> {{ repo.annotations?.stars ?? 0 }}</span>
                  <span class="meta-item" title="Forks"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/></svg> {{ repo.annotations?.forks ?? 0 }}</span>
                  <span class="meta-item" title="Open Issues"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg> {{ repo.annotations?.issues ?? 0 }}</span>
                  <span class="meta-item" title="Last Updated"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z"/></svg> {{ repo.annotations?.updated || '-' }}</span>
                </div>
              </div>
            </div>
            <div v-else class="empty-line">暂无仓库数据，先创建代码仓。</div>
          </div>

          <aside class="repo-guide-panel">
            <div v-if="exampleCloneUrl" class="push-help">
              <div class="section-label">推送代码</div>
              <pre><code>git remote add origin {{ exampleCloneUrl }}
git push -u origin main</code></pre>
              <p>source 交付需要先把代码推到 Gitea，然后由 Jenkins/kpack 构建镜像并交给 ArgoCD 部署。</p>
            </div>
            <div v-else class="push-help">
              <div class="section-label">推送代码</div>
              <p>暂无可推送的真实仓库，先创建代码仓。</p>
            </div>
          </aside>
        </div>

        <!-- Repo detail -->
        <div v-else class="repo-detail">
          <div class="detail-toolbar">
            <button class="act-btn ghost" @click="clearRepositorySelection">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 12H5M12 19l-7-7 7-7"/></svg>
              返回列表
            </button>
            <div class="toolbar-actions">
              <button
                v-for="act in repoActions(selectedRepo)"
                :key="act.label"
                class="act-btn"
                :class="act.tone"
                @click="emit('action', act, act.target || selectedRepo?.name)"
              >
                {{ act.label }}
              </button>
              <a v-if="selectedRepo.externalUrl" :href="selectedRepo.externalUrl" target="_blank" rel="noreferrer" class="link external">在 Gitea 中打开 ↗</a>
            </div>
          </div>

          <div class="detail-header">
            <div class="detail-title-row">
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/></svg>
              <h3 class="detail-name">{{ selectedRepo.name }}</h3>
              <span v-if="selectedRepo.annotations?.repositoryRole" class="visibility role">{{ repoRoleLabel(selectedRepo.annotations.repositoryRole) }}</span>
              <span v-if="selectedRepo.annotations?.private" class="visibility private">Private</span>
              <span v-else class="visibility public">Public</span>
            </div>
            <p class="detail-desc">{{ selectedRepo.description || '暂无描述' }}</p>
          </div>

          <div class="detail-stats">
            <div class="stat-box">
              <span class="stat-num">{{ selectedRepo.annotations?.stars ?? 0 }}</span>
              <span class="stat-label">Stars</span>
            </div>
            <div class="stat-box">
              <span class="stat-num">{{ selectedRepo.annotations?.forks ?? 0 }}</span>
              <span class="stat-label">Forks</span>
            </div>
            <div class="stat-box">
              <span class="stat-num">{{ selectedRepo.annotations?.issues ?? 0 }}</span>
              <span class="stat-label">Issues</span>
            </div>
            <div class="stat-box">
              <span class="stat-num">{{ selectedRepo.annotations?.branch || 'main' }}</span>
              <span class="stat-label">默认分支</span>
            </div>
          </div>

          <div class="detail-section">
            <div class="section-label">Clone</div>
            <div class="clone-box">
              <code class="mono">{{ cloneUrl(selectedRepo) }}</code>
            </div>
          </div>

          <div v-if="selectedRepo.annotations?.sourceRepoURL" class="detail-section">
            <div class="section-label">外部源码来源</div>
            <div class="clone-box">
              <code class="mono">{{ selectedRepo.annotations.sourceRepoURL }}</code>
            </div>
          </div>

          <div class="repo-workbench">
            <section class="repo-main-column">
              <div class="repo-pathbar">
                <button type="button" class="branch-pill" @click="openRepositoryRoot">
                  {{ selectedRepo.annotations?.branch || 'main' }}
                </button>
                <div class="path-crumbs">
                  <button type="button" @click="openRepositoryRoot">{{ selectedRepo.name }}</button>
                  <template v-for="crumb in currentPathSegments" :key="crumb.path">
                    <span>/</span>
                    <button type="button" @click="selectFile(crumb.resource)">{{ crumb.label }}</button>
                  </template>
                </div>
              </div>

              <section v-if="!selectedFile || selectedFile.type === 'Directory'" class="file-editor-main repository-browser">
                <div class="browser-head">
                  <div>
                    <div class="editor-path">{{ selectedFile?.annotations?.path || selectedRepo.name }}</div>
                    <div class="editor-desc">{{ selectedFile?.description || '仓库文件、目录、流水线和部署清单。' }}</div>
                  </div>
                  <button type="button" class="act-btn ghost" @click="openRepositoryRoot">根目录</button>
                </div>
                <div class="gitlab-file-table">
                  <button
                    v-for="item in currentDirectoryItems"
                    :key="treeKey(item)"
                    type="button"
                    class="gitlab-file-row"
                    :class="{ directory: item.type === 'Directory' }"
                    @click="selectFile(item)"
                  >
                    <span class="file-kind">{{ item.type === 'Directory' ? 'DIR' : 'FILE' }}</span>
                    <strong>{{ displayFileName(item) }}</strong>
                    <small>{{ item.description }}</small>
                    <span>{{ fileSize(item) }}</span>
                  </button>
                  <div v-if="treeLoading" class="file-empty">目录加载中...</div>
                  <div v-if="treeError" class="file-empty error">{{ treeError }}</div>
                  <div v-if="!currentDirectoryItems.length" class="file-empty">当前目录没有可展示文件。</div>
                </div>
              </section>

              <section v-if="!selectedFile && readmePreview" class="readme-preview">
                <div class="readme-head">README</div>
                <pre class="readme-body"><code>{{ readmePreview }}</code></pre>
              </section>

              <section v-if="selectedFile && selectedFile.type !== 'Directory'" class="file-editor-main code-pane">
                <div class="editor-head">
                  <div>
                    <div class="editor-path">{{ selectedFile.annotations?.path || selectedFile.name }}</div>
                    <div class="editor-desc">{{ selectedFile.description }}</div>
                  </div>
                  <div class="editor-actions">
                    <button type="button" class="act-btn ghost" @click="selectedFile = null">返回文件</button>
                    <button type="button" class="act-btn ghost" :class="{ active: !editingFile }" @click="editingFile = false">查看</button>
                    <button type="button" class="act-btn ghost" :class="{ active: editingFile }" @click="editingFile = true">编辑</button>
                  </div>
                </div>
                <textarea
                  v-if="editingFile"
                  v-model="editedFileContent"
                  class="file-editor-textarea"
                  spellcheck="false"
                  aria-label="编辑文件内容"
                />
                <pre v-else class="file-preview main-preview"><code>{{ filePreviewText }}</code></pre>
                <div v-if="editingFile" class="editor-footer">
                  <span>当前版本仅在页面内编辑，提交保存需要后端文件写入接口。</span>
                  <button type="button" class="act-btn ghost" @click="resetEditedFile">重置</button>
                </div>
              </section>
            </section>

            <aside class="file-detail">
              <div class="detail-side-head">
                <span class="section-label">文件详情</span>
                <span class="badge blue">{{ selectedFile?.type || '-' }}</span>
              </div>
              <div v-if="selectedFile" class="file-summary">
                <div class="file-name">{{ selectedFile.name }}</div>
                <div class="file-desc">{{ selectedFile.description }}</div>
                <div class="file-meta-grid">
                  <div>
                    <span>状态</span>
                    <strong>{{ selectedFile.status }}</strong>
                  </div>
                  <div>
                    <span>默认分支</span>
                    <strong>{{ selectedRepo.annotations?.branch || 'main' }}</strong>
                  </div>
                  <div>
                    <span>语言</span>
                    <strong>{{ selectedRepo.annotations?.language || 'Container/Kubernetes' }}</strong>
                  </div>
                  <div>
                    <span>路径</span>
                    <strong>{{ selectedFile.annotations?.path || selectedFile.name }}</strong>
                  </div>
                  <div>
                    <span>大小</span>
                    <strong>{{ fileSize(selectedFile) }}</strong>
                  </div>
                </div>
              </div>
              <div v-else class="file-empty">选择文件查看详情。</div>

              <div class="commit-panel">
                <div class="section-label">最近提交</div>
                <div v-for="commit in recentCommits(selectedRepo)" :key="commit.sha || commit.message" class="commit-row">
                  <code>{{ commit.sha }}</code>
                  <div>
                    <strong>{{ commit.message }}</strong>
                    <span>{{ commit.author }} · {{ commit.time }}</span>
                  </div>
                </div>
                <div v-if="!recentCommits(selectedRepo).length" class="file-empty">暂无提交数据。</div>
              </div>

              <div class="repo-action-panel">
                <div class="section-label">仓库操作</div>
                <button
                  v-for="act in repoActions(selectedRepo)"
                  :key="'side-' + act.label"
                  class="act-btn"
                  :class="act.tone"
                  @click="emit('action', act, act.target || selectedRepo?.name)"
                >
                  {{ act.label }}
                </button>
              </div>
            </aside>
          </div>
        </div>
      </div>

      <div v-if="activeTab === 'resources'" class="tab-panel">
        <div v-if="resources.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>类型</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="r in resources" :key="r.name + r.type">
                <td class="cell-name">{{ r.name }}</td>
                <td><span class="badge blue">{{ r.type }}</span></td>
                <td><span class="badge" :class="statusBadge(r.status)">{{ r.status }}</span></td>
                <td class="cell-desc">{{ r.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无资源数据</div>
      </div>
    </template>
  </ToolWorkspaceFrame>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'
import { repositoryCloneUrl, repositoryContentsUrl, repositoryIdentity as repoIdentity, repositoryInitialPath } from './giteaRepository'

const props = defineProps<{
  resources: WorkspaceResource[]
}>()

const emit = defineEmits<{
  (e: 'action', action: WorkspaceAction, target?: string): void
}>()

const repos = computed(() => props.resources.filter(r => r.type === 'Repository'))
const repoIdentityList = computed(() => repos.value.map((repo) => repoIdentity(repo)).join('|'))
const selectedRepo = ref<WorkspaceResource | null>(null)
const selectedFile = ref<WorkspaceResource | null>(null)
const editingFile = ref(false)
const editedFileContent = ref('')
const lazyFileLoading = ref(false)
const lazyFileError = ref('')
const treeLoading = ref(false)
const treeError = ref('')
const loadedDirectories = ref<Record<string, boolean>>({})
const loadedRepoTrees = ref<Record<string, WorkspaceResource[]>>({})
let fileLoadToken = 0
let treeLoadToken = 0
const emptyFileText = '当前文件没有可预览内容。'

const fallbackRepoName = computed(() => repos.value[0]?.name || '')
const workspaceActions = computed(() => {
  const byKey = new Map<string, WorkspaceAction>()
  for (const repo of repos.value) {
    for (const action of repo.actions || []) {
      if (action.key) byKey.set(action.key, action)
    }
  }
  return byKey
})

const userFacingAction = (action: WorkspaceAction): WorkspaceAction => {
  if (action.key === 'create_gitea_repository') return { ...action, label: '创建代码仓' }
  if (action.key === 'add_gitea_user_key') return { ...action, label: '配置公钥' }
  if (action.key === 'reconcile_gitops') return { ...action, label: '同步代码仓' }
  return action
}

const actionByKey = (key: string, fallback: WorkspaceAction): WorkspaceAction =>
  userFacingAction(workspaceActions.value.get(key) || fallback)

const createRepoAction = computed(() => actionByKey('create_gitea_repository', {
  key: 'create_gitea_repository',
  label: '创建仓库',
  description: '在当前 Gitea 中创建一个新的代码仓库。',
  tone: 'primary',
  fields: [
    { name: 'name', label: '仓库名称', required: true, placeholder: fallbackRepoName.value },
    { name: 'description', label: '描述', placeholder: '组件源码或交付清单仓库' },
    { name: 'private', label: '私有仓库', type: 'checkbox', default: 'true' },
  ],
}))

const userKeyAction = computed(() => actionByKey('add_gitea_user_key', {
  key: 'add_gitea_user_key',
  label: '添加 SSH 公钥',
  description: '给 Gitea 用户添加 SSH 公钥，便于从本机 push 代码。',
  fields: [
    { name: 'title', label: '名称', required: true, placeholder: 'mensyli1-laptop' },
    { name: 'key', label: 'SSH 公钥', type: 'textarea', required: true, placeholder: 'ssh-ed25519 AAAA...' },
  ],
}))

const exampleCloneUrl = computed(() => {
  const repo = repos.value[0]
  return repo ? cloneUrl(repo) : ''
})

const selectRepo = (repo: WorkspaceResource) => {
  selectedRepo.value = repo
  selectedFile.value = null
  editingFile.value = false
  editedFileContent.value = ''
  lazyFileLoading.value = false
  lazyFileError.value = ''
  treeLoading.value = false
  treeError.value = ''
  fileLoadToken += 1
  treeLoadToken += 1
  ensureRepositoryTreeSlot(repo)
  void loadRepositoryDirectory(repo, repositoryInitialPath(repo))
}

const selectFile = (file: WorkspaceResource) => {
  selectedFile.value = file
  editingFile.value = false
  lazyFileError.value = ''
  if (selectedRepo.value && file.type === 'Directory') {
    void loadRepositoryDirectory(selectedRepo.value, String(file.annotations?.path || file.name), file)
  }
  void loadSelectedFileContent(file)
}

const openRepositoryRoot = () => {
  selectedFile.value = null
  editingFile.value = false
  if (selectedRepo.value) {
    void loadRepositoryDirectory(selectedRepo.value, repositoryInitialPath(selectedRepo.value))
  }
}

const resetEditedFile = () => {
  editedFileContent.value = fileContent(selectedFile.value)
}

const cloneUrl = (repo: WorkspaceResource) => {
  return repositoryCloneUrl(repo) || 'git@localhost:' + repo.name + '.git'
}

const repoRoleLabel = (role: unknown) => {
  const value = String(role || '')
  if (value === 'source') return '源码镜像'
  if (value === 'gitops') return '交付清单'
  return value
}

const ensureRepositoryTreeSlot = (repo: WorkspaceResource) => {
  const key = repoIdentity(repo)
  if (!(key in loadedRepoTrees.value)) {
    loadedRepoTrees.value = { ...loadedRepoTrees.value, [key]: [] }
  }
}

const repoTree = (repo: WorkspaceResource): WorkspaceResource[] => {
  return loadedRepoTrees.value[repoIdentity(repo)] || []
}

const clearRepositorySelection = () => {
  selectedRepo.value = null
  selectedFile.value = null
  editingFile.value = false
  editedFileContent.value = ''
  lazyFileLoading.value = false
  lazyFileError.value = ''
  treeLoading.value = false
  treeError.value = ''
  fileLoadToken += 1
  treeLoadToken += 1
}

const resetRepositoryLoadState = () => {
  clearRepositorySelection()
  loadedDirectories.value = {}
  loadedRepoTrees.value = {}
}

const currentDirectoryItems = computed(() => {
  if (!selectedRepo.value) return []
  if (selectedFile.value?.type === 'Directory') {
    const current = findByTreeKey(repoTree(selectedRepo.value), treeKey(selectedFile.value)) || selectedFile.value
    return current.children || []
  }
  return repoTree(selectedRepo.value)
})

const readmePreview = computed(() => {
  if (!selectedRepo.value || selectedFile.value) return ''
  const readme = repoTree(selectedRepo.value).find((item) =>
    item.type === 'File' && /^readme(\.|$)/i.test(displayFileName(item))
  )
  return fileContent(readme)
})

const findResourcePath = (items: WorkspaceResource[], target: WorkspaceResource | null, trail: WorkspaceResource[] = []): WorkspaceResource[] => {
  if (!target) return []
  for (const item of items) {
    const next = [...trail, item]
    if (treeKey(item) === treeKey(target)) return next
    if (item.children?.length) {
      const found = findResourcePath(item.children, target, next)
      if (found.length) return found
    }
  }
  return []
}

const findByTreeKey = (items: WorkspaceResource[], key: string): WorkspaceResource | null => {
  for (const item of items) {
    if (treeKey(item) === key) return item
    const found = findByTreeKey(item.children || [], key)
    if (found) return found
  }
  return null
}

const currentPathSegments = computed(() => {
  if (!selectedRepo.value || !selectedFile.value) return []
  return findResourcePath(repoTree(selectedRepo.value), selectedFile.value).map((resource) => ({
    resource,
    label: displayFileName(resource),
    path: String(resource.annotations?.path || resource.name),
  }))
})

const treeKey = (node: WorkspaceResource) =>
  `${String(node.annotations?.path || node.name)}:${node.type}`

const displayFileName = (node: WorkspaceResource) => {
  const path = String(node.annotations?.path || node.name)
  return path.split('/').filter(Boolean).pop() || node.name
}

const fileContent = (node?: WorkspaceResource | null) =>
  String(node?.annotations?.content || '')

const filePreviewText = computed(() => {
  if (lazyFileLoading.value) return '文件内容加载中...'
  if (lazyFileError.value) return lazyFileError.value
  return editedFileContent.value || fileContent(selectedFile.value) || emptyFileText
})

const rawFileUrl = (repo: WorkspaceResource, file: WorkspaceResource) => {
  return repositoryContentsUrl(repo, String(file.annotations?.path || file.name))
}

const contentItemsToResources = (items: any[]): WorkspaceResource[] => items.map((item) => {
  const type = String(item.type || '').toLowerCase()
  const resourceType = type === 'dir' || type === 'directory' ? 'Directory' : 'File'
  const path = String(item.path || item.name || '')
  const size = Number(item.size || 0)
  return {
    name: String(item.name || path),
    type: resourceType,
    status: 'Ready',
    description: resourceType === 'Directory'
      ? `目录 ${path}`
      : size > 0 ? `文件 ${path} · ${size} bytes` : `文件 ${path}`,
    annotations: {
      path,
      size,
      downloadURL: item.download_url || item.downloadUrl || '',
    },
    children: [],
  }
})

const decodeGiteaContent = (payload: any) => {
  const raw = String(payload?.content || '')
  if (!raw) return ''
  if (payload?.encoding === 'base64') {
    try {
      return decodeURIComponent(Array.from(atob(raw.replace(/\s/g, '')), c => `%${c.charCodeAt(0).toString(16).padStart(2, '0')}`).join(''))
    } catch {
      return atob(raw.replace(/\s/g, ''))
    }
  }
  return raw
}

const repositoryTreeKey = (repo: WorkspaceResource, path = '') => `${repoIdentity(repo)}:${path}`

const replaceTreeNodeChildren = (nodes: WorkspaceResource[], target: WorkspaceResource, children: WorkspaceResource[]): WorkspaceResource[] =>
  nodes.map((node) => {
    if (treeKey(node) === treeKey(target)) return { ...node, children }
    return { ...node, children: replaceTreeNodeChildren(node.children || [], target, children) }
  })

const loadRepositoryDirectory = async (repo: WorkspaceResource, path = '', target?: WorkspaceResource | null) => {
  const loadedKey = repositoryTreeKey(repo, path)
  if (loadedDirectories.value[loadedKey]) return
  const selectedRepoKey = selectedRepo.value ? repoIdentity(selectedRepo.value) : ''
  const requestRepoKey = repoIdentity(repo)
  const url = repositoryContentsUrl(repo, path)
  if (!url) return
  const token = ++treeLoadToken
  treeLoading.value = true
  treeError.value = ''
  try {
    const res = await fetch(url)
    if (!res.ok) throw new Error(res.statusText || `HTTP ${res.status}`)
    const payload = await res.json()
    if (token !== treeLoadToken || selectedRepoKey !== requestRepoKey || (selectedRepo.value && repoIdentity(selectedRepo.value) !== requestRepoKey)) return
    const items = Array.isArray(payload) ? payload : [payload]
    const children = contentItemsToResources(items)
    const currentTree = loadedRepoTrees.value[requestRepoKey] || []
    const nextTree = target ? replaceTreeNodeChildren(currentTree, target, children) : children
    loadedRepoTrees.value = {
      ...loadedRepoTrees.value,
      [requestRepoKey]: nextTree,
    }
    if (target && selectedFile.value && treeKey(selectedFile.value) === treeKey(target)) {
      selectedFile.value = { ...target, children }
    }
    loadedDirectories.value = { ...loadedDirectories.value, [loadedKey]: true }
  } catch (err: any) {
    if (token !== treeLoadToken || selectedRepoKey !== requestRepoKey || (selectedRepo.value && repoIdentity(selectedRepo.value) !== requestRepoKey)) return
    treeError.value = `目录加载失败：${err?.message || '未知错误'}`
  } finally {
    if (token === treeLoadToken && (!selectedRepo.value || repoIdentity(selectedRepo.value) === requestRepoKey)) treeLoading.value = false
  }
}

const loadSelectedFileContent = async (file: WorkspaceResource) => {
  if (!selectedRepo.value || file.type === 'Directory' || fileContent(file)) return
  const requestRepoKey = repoIdentity(selectedRepo.value)
  const url = rawFileUrl(selectedRepo.value, file)
  if (!url) return
  const token = ++fileLoadToken
  lazyFileLoading.value = true
  lazyFileError.value = ''
  try {
    const res = await fetch(url)
    if (!res.ok) throw new Error(res.statusText || `HTTP ${res.status}`)
    const payload = await res.json()
    const content = decodeGiteaContent(Array.isArray(payload) ? payload[0] : payload)
    if (token !== fileLoadToken || !selectedRepo.value || repoIdentity(selectedRepo.value) !== requestRepoKey) return
    file.annotations = { ...(file.annotations || {}), content, encoding: 'text' }
    if (selectedFile.value && treeKey(selectedFile.value) === treeKey(file)) {
      selectedFile.value = { ...file }
      editedFileContent.value = content
    }
  } catch (err: any) {
    if (token !== fileLoadToken || !selectedRepo.value || repoIdentity(selectedRepo.value) !== requestRepoKey) return
    lazyFileError.value = `文件内容加载失败：${err?.message || '未知错误'}`
  } finally {
    if (token === fileLoadToken && selectedRepo.value && repoIdentity(selectedRepo.value) === requestRepoKey) lazyFileLoading.value = false
  }
}

watch(repos, (nextRepos) => {
  if (!nextRepos.length) {
    clearRepositorySelection()
    return
  }

  if (!selectedRepo.value) return
  const selectedRepoKey = repoIdentity(selectedRepo.value)
  const nextRepo = nextRepos.find((repo) => repoIdentity(repo) === selectedRepoKey)
  if (!nextRepo) {
    clearRepositorySelection()
    return
  }

  const selectedFileKey = selectedFile.value ? treeKey(selectedFile.value) : ''
  selectedRepo.value = nextRepo
  ensureRepositoryTreeSlot(nextRepo)
  selectedFile.value = selectedFileKey ? findByTreeKey(repoTree(nextRepo), selectedFileKey) : null
}, { flush: 'sync' })

watch(repoIdentityList, () => {
  resetRepositoryLoadState()
}, { flush: 'sync' })

watch(selectedFile, resetEditedFile, { immediate: true })

const fileSize = (node?: WorkspaceResource | null) => {
  const size = Number(node?.annotations?.size || 0)
  if (!size) return '-'
  if (size < 1024) return `${size} B`
  return `${(size / 1024).toFixed(1)} KB`
}

const recentCommits = (repo: WorkspaceResource) => {
  const commits = Array.isArray(repo.annotations?.commits) ? repo.annotations?.commits : []
  return commits.map((commit: any) => ({
    sha: String(commit.sha || '').slice(0, 7),
    message: String(commit.message || ''),
    author: String(commit.author || ''),
    time: String(commit.time || ''),
  })).filter((commit) => commit.sha || commit.message)
}

const isUserFacingRepoAction = (action: WorkspaceAction) => {
  const key = String(action.key || '').toLowerCase()
  const label = String(action.label || '').toLowerCase()
  return key !== 'add_gitea_deploy_key' && !label.includes('deploy key')
}

const repoActions = (repo: WorkspaceResource): WorkspaceAction[] => {
  const actions = repo.actions?.length ? repo.actions : [
    { key: 'reconcile_gitops', label: '同步代码仓', description: '重新生成该组件的仓库内容。', target: repo.name },
    { key: 'refresh', label: '刷新', description: '重新读取仓库状态。', target: repo.name },
  ]
  return actions.filter(isUserFacingRepoAction).map(userFacingAction)
}

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (repos.value.length) tabs.push({ key: 'repos', label: '代码仓库', count: repos.value.length })
  tabs.push({ key: 'resources', label: '资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(repos.value.length ? 'repos' : 'resources')

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('synced') || v.includes('up')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('down') || v.includes('degraded')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.ws-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
  margin-bottom: var(--paap-space-5);
  flex-wrap: wrap;
}
.ws-toolbar .ws-tabs {
  margin-bottom: 0;
}
.ws-toolbar-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
.repo-home {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 360px);
  gap: var(--paap-space-4);
  align-items: start;
}
.repo-home-main {
  min-width: 0;
  display: grid;
  gap: var(--paap-space-3);
}
.repo-home-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
  min-height: 76px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  padding: var(--paap-space-4) var(--paap-space-5);
}
.repo-home-head h3 {
  margin: 0 0 4px;
  color: var(--paap-text);
  font-size: 16px;
  line-height: 1.2;
}
.repo-home-head p {
  margin: 0;
  color: var(--paap-muted);
  font-size: 12px;
  line-height: 1.5;
}
.repo-guide-panel {
  position: sticky;
  top: 12px;
  min-width: 0;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  padding: var(--paap-space-4);
}
.push-help {
  display: grid;
  gap: var(--paap-space-2);
}
.push-help pre {
  overflow: auto;
  margin: 0;
  border-radius: var(--paap-radius-sm);
  background: #0f1117;
  color: #e5e7eb;
  padding: var(--paap-space-3);
  font-size: 11px;
  line-height: 1.55;
}
.push-help code {
  font-family: var(--paap-mono);
}
.push-help p {
  margin: var(--paap-space-2) 0 0;
  color: var(--paap-muted);
  font-size: 12px;
  line-height: 1.5;
}
.repo-list { display: flex; flex-direction: column; gap: var(--paap-space-3); }
.repo-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--paap-space-3);
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-4) var(--paap-space-5);
  color: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
  cursor: pointer;
}
.repo-card:hover { border-color: var(--paap-border-strong); box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
.repo-header { display: flex; align-items: center; gap: var(--paap-space-2); margin-bottom: 4px; }
.repo-name { font-weight: 600; font-size: 14px; color: var(--paap-accent); }
.repo-desc { font-size: 12px; color: var(--paap-muted); }
.visibility { font-size: 10px; padding: 1px 6px; border-radius: var(--paap-radius-full); font-weight: 600; border: 1px solid; }
.visibility.public { color: #059669; border-color: #a7f3d0; background: var(--paap-success-soft); }
.visibility.private { color: #9a3412; border-color: #fed7aa; background: var(--paap-warning-soft); }
.visibility.role { color: var(--paap-accent); border-color: #bfdbfe; background: var(--paap-accent-soft); }
.repo-meta { display: flex; align-items: center; gap: var(--paap-space-4); flex-shrink: 0; }
.meta-item { display: flex; align-items: center; gap: 4px; font-size: 12px; color: var(--paap-muted); }
.meta-item svg { opacity: 0.7; }

/* Repo detail */
.repo-detail { animation: fadeIn 0.2s ease; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
.detail-toolbar { display: flex; justify-content: space-between; align-items: center; gap: var(--paap-space-3); margin-bottom: var(--paap-space-4); flex-wrap: wrap; }
.toolbar-actions { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; }
.detail-header { margin-bottom: var(--paap-space-5); }
.detail-title-row { display: flex; align-items: center; gap: var(--paap-space-3); flex-wrap: wrap; margin-bottom: 6px; }
.detail-name { font-size: 18px; font-weight: 600; color: var(--paap-text); margin: 0; }
.detail-desc { font-size: 13px; color: var(--paap-muted); margin: 0; }
.detail-stats { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: var(--paap-space-3); margin-bottom: var(--paap-space-5); }
.stat-box { background: var(--paap-panel); border: 1px solid var(--paap-border); border-radius: var(--paap-radius); padding: var(--paap-space-4); text-align: center; }
.stat-num { font-size: 18px; font-weight: 600; color: var(--paap-text); }
.stat-label { font-size: 11px; color: var(--paap-muted); margin-top: 4px; display: block; }
.detail-section { margin-bottom: var(--paap-space-5); }
.section-label { font-size: 11px; font-weight: 600; color: var(--paap-muted); text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 6px; }
.clone-box { background: var(--paap-panel-subtle); border: 1px solid var(--paap-border); border-radius: var(--paap-radius-xs); padding: 10px 12px; }
.clone-box code { font-size: 12px; color: var(--paap-text); font-family: var(--paap-mono); }
.repo-workbench { display: grid; grid-template-columns: minmax(0, 1fr) minmax(260px, 340px); gap: var(--paap-space-4); align-items: start; }
.repo-main-column, .file-detail { min-width: 0; }
.repo-main-column { display: grid; gap: var(--paap-space-3); }
.repo-pathbar {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  min-width: 0;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  padding: 10px 12px;
}
.branch-pill {
  flex-shrink: 0;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  height: 30px;
  padding: 0 10px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  font-family: inherit;
}
.path-crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
  color: var(--paap-muted-2);
  font-size: 13px;
}
.path-crumbs button {
  border: 0;
  background: transparent;
  color: var(--paap-accent);
  font-size: 13px;
  font-weight: 600;
  padding: 0;
  cursor: pointer;
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-editor-main { min-width: 0; background: var(--paap-panel); border: 1px solid var(--paap-border); border-radius: var(--paap-radius); overflow: hidden; }
.repository-browser { min-height: 360px; }
.browser-head { min-height: 58px; display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-3); padding: 12px 14px; border-bottom: 1px solid var(--paap-border); background: var(--paap-panel-subtle); }
.editor-head { min-height: 58px; display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-3); padding: 12px 14px; border-bottom: 1px solid var(--paap-border); background: var(--paap-panel-subtle); }
.editor-path { color: var(--paap-text); font-size: 14px; font-weight: 600; word-break: break-all; }
.editor-desc { color: var(--paap-muted); font-size: 12px; margin-top: 3px; line-height: 1.4; }
.editor-actions { display: inline-flex; align-items: center; gap: 6px; flex-shrink: 0; }
.editor-actions .act-btn.active { background: var(--cds-button-primary, var(--paap-accent)); color: var(--cds-text-on-color, #fff); border-color: var(--cds-button-primary, var(--paap-accent)); }
.gitlab-file-table { display: grid; }
.gitlab-file-row {
  display: grid;
  grid-template-columns: 58px minmax(160px, 0.42fr) minmax(0, 1fr) 74px;
  align-items: center;
  gap: var(--paap-space-3);
  min-height: 42px;
  border: none;
  border-bottom: 1px solid #f3f4f6;
  background: transparent;
  color: var(--paap-text);
  padding: 8px 12px;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
}
.gitlab-file-row:hover { background: var(--paap-panel-subtle); }
.gitlab-file-row:last-child { border-bottom: none; }
.gitlab-file-row strong { color: var(--paap-accent); font-size: 13px; word-break: break-word; }
.gitlab-file-row.directory strong { color: var(--paap-text); }
.gitlab-file-row small { color: var(--paap-muted); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.gitlab-file-row > span:last-child { color: var(--paap-muted-2); font-size: 12px; text-align: right; }
.file-kind {
  justify-self: start;
  border-radius: var(--paap-radius-xs);
  background: #f3f4f6;
  color: var(--paap-muted);
  padding: 2px 6px;
  font-size: 10px;
  font-weight: 600;
}
.gitlab-file-row.directory .file-kind { background: var(--paap-accent-soft); color: var(--paap-accent); }
.readme-preview {
  min-width: 0;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
}
.readme-head {
  min-height: 42px;
  display: flex;
  align-items: center;
  padding: 0 14px;
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  font-size: 13px;
  font-weight: 600;
}
.readme-body {
  margin: 0;
  max-height: 420px;
  overflow: auto;
  background: var(--paap-panel);
  color: var(--paap-text);
  padding: var(--paap-space-4);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
.readme-body code { font-family: var(--paap-mono); }
.file-editor-textarea { display: block; width: 100%; min-height: 620px; border: 0; resize: vertical; padding: var(--paap-space-4); outline: none; background: var(--paap-panel); color: var(--paap-text); font-family: var(--paap-mono); font-size: 12px; line-height: 1.6; tab-size: 2; }
.editor-footer { display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-3); padding: 10px 12px; border-top: 1px solid var(--paap-border); background: var(--paap-panel-subtle); color: var(--paap-muted); font-size: 12px; }
.directory-main { display: grid; gap: var(--paap-space-2); padding: var(--paap-space-4); }
.directory-row { display: grid; grid-template-columns: 16px minmax(140px, 0.45fr) minmax(0, 1fr); gap: var(--paap-space-2); align-items: center; border: 1px solid var(--paap-border); border-radius: var(--paap-radius-xs); background: var(--paap-panel); padding: 9px 10px; text-align: left; cursor: pointer; font-family: inherit; }
.directory-row:hover { border-color: var(--paap-border-strong); background: var(--paap-panel-subtle); }
.directory-row span { color: var(--paap-muted); }
.directory-row strong { color: var(--paap-text); font-size: 13px; word-break: break-all; }
.directory-row small { color: var(--paap-muted); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-tree { background: var(--paap-panel); border: 1px solid var(--paap-border); border-radius: var(--paap-radius); overflow: hidden; }
.tree-row { width: 100%; display: flex; align-items: center; gap: var(--paap-space-2); padding: 8px 14px; font-size: 13px; color: var(--paap-text); border: none; border-bottom: 1px solid #f3f4f6; background: transparent; text-align: left; font-family: inherit; }
.tree-row:not(.root) { cursor: pointer; }
.tree-row:not(.root):hover { background: var(--paap-panel-subtle); }
.tree-row.selected { background: var(--paap-accent-soft); color: var(--paap-accent); }
.tree-row.directory .tree-name { color: var(--paap-text-soft); }
.tree-row:last-child { border-bottom: none; }
.tree-row.root { background: var(--paap-panel-subtle); font-weight: 600; }
.tree-icon { font-size: 14px; }
.tree-name { font-weight: 600; min-width: 0; word-break: break-all; }
.tree-desc { color: var(--paap-muted-2); font-size: 12px; margin-left: auto; }
.file-detail { background: var(--paap-panel); border: 1px solid var(--paap-border); border-radius: var(--paap-radius); padding: var(--paap-space-4); }
.detail-side-head { display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-2); margin-bottom: var(--paap-space-3); }
.file-summary { display: flex; flex-direction: column; gap: var(--paap-space-2); margin-bottom: var(--paap-space-4); }
.file-name { font-size: 14px; font-weight: 600; color: var(--paap-text); word-break: break-all; }
.file-desc { font-size: 12px; color: var(--paap-muted); line-height: 1.5; }
.file-meta-grid { display: grid; gap: 6px; }
.file-meta-grid > div { display: grid; grid-template-columns: 76px minmax(0, 1fr); gap: var(--paap-space-2); font-size: 12px; }
.file-meta-grid span { color: var(--paap-muted-2); }
.file-meta-grid strong { color: var(--paap-text); font-weight: 600; word-break: break-all; }
.file-empty { color: var(--paap-muted-2); font-size: 12px; text-align: center; padding: var(--paap-space-5) 0; }
.file-preview { max-height: 360px; overflow: auto; margin: var(--paap-space-3) 0 0; border: 1px solid var(--paap-border); border-radius: var(--paap-radius); background: var(--paap-panel); color: var(--paap-text); padding: var(--paap-space-3); font-size: 12px; line-height: 1.55; white-space: pre-wrap; word-break: break-word; }
.main-preview { min-height: 620px; max-height: none; margin: 0; border: 0; border-radius: 0; padding: var(--paap-space-4); }
.file-preview code { font-family: var(--paap-mono); }
.commit-panel { border-top: 1px solid var(--paap-border); padding-top: var(--paap-space-3); margin-top: var(--paap-space-3); }
.commit-row { display: grid; grid-template-columns: 58px minmax(0, 1fr); gap: var(--paap-space-2); padding: var(--paap-space-2) 0; border-bottom: 1px solid #f3f4f6; font-size: 12px; }
.commit-row:last-child { border-bottom: none; }
.commit-row code { font-family: var(--paap-mono); color: var(--paap-accent); background: var(--paap-accent-soft); border-radius: var(--paap-radius-xs); padding: 2px 4px; align-self: start; }
.commit-row strong { display: block; color: var(--paap-text); font-weight: 600; margin-bottom: 2px; }
.commit-row span { color: var(--paap-muted); }
.repo-action-panel { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; border-top: 1px solid var(--paap-border); padding-top: var(--paap-space-3); margin-top: var(--paap-space-3); }
.repo-action-panel .section-label { width: 100%; margin-bottom: 0; }

@media (max-width: 900px) {
  .repo-home { grid-template-columns: 1fr; }
  .repo-home-head { align-items: flex-start; flex-direction: column; }
  .repo-guide-panel { position: static; }
  .repo-workbench { grid-template-columns: 1fr; }
  .repo-card { align-items: flex-start; flex-direction: column; }
  .repo-meta { flex-wrap: wrap; }
  .tree-desc { display: none; }
  .repo-pathbar { align-items: flex-start; flex-direction: column; }
  .gitlab-file-row { grid-template-columns: 46px minmax(120px, 1fr); }
  .gitlab-file-row small,
  .gitlab-file-row > span:last-child { display: none; }
}
</style>
