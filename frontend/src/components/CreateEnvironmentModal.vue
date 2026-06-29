<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="modal-overlay"
      role="dialog"
      aria-modal="true"
      @click.self="emit('close')"
    >
      <div class="modal-container">
        <div class="modal-header">
          <div>
            <p class="modal-label">创建环境</p>
            <p class="modal-heading">新建环境</p>
          </div>
          <button
            class="modal-close"
            type="button"
            aria-label="关闭"
            :disabled="creating"
            @click="emit('close')"
          >
            <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-name`">环境名称 <span class="required">*</span></label>
            <input
              :id="`${dialogIdPrefix}-environment-name`"
              :value="form.name"
              class="rail-input"
              placeholder="例如：开发环境"
              @input="updateField('name', ($event.target as HTMLInputElement).value.trim())"
              @keyup.enter="emit('submit')"
            />
          </div>
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-identifier`">环境标识</label>
            <input
              :id="`${dialogIdPrefix}-environment-identifier`"
              :value="form.identifier"
              class="rail-input"
              placeholder="留空由后台生成"
              @input="updateField('identifier', ($event.target as HTMLInputElement).value.trim())"
            />
            <div class="form-helper">当前预览：{{ identifierPreview }}</div>
          </div>
          <div class="form-item">
            <label class="form-label">创建方式</label>
            <div class="radio-group">
              <label class="radio-item" :class="{ active: form.mode === 'blank' }">
                <input type="radio" value="blank" :checked="form.mode === 'blank'" @change="updateField('mode', 'blank')" />
                <span>创建空环境</span>
              </label>
              <label class="radio-item" :class="{ active: form.mode === 'empty' }">
                <input type="radio" value="empty" :checked="form.mode === 'empty'" @change="updateField('mode', 'empty')" />
                <span>创建基础环境</span>
              </label>
              <label class="radio-item" :class="{ active: form.mode === 'template' }">
                <input type="radio" value="template" :checked="form.mode === 'template'" @change="updateField('mode', 'template')" />
                <span>从模板创建</span>
              </label>
            </div>
          </div>
          <div v-if="form.mode === 'template'" class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-template`">选择模板</label>
            <select
              :id="`${dialogIdPrefix}-environment-template`"
              :value="form.templateId"
              class="rail-select"
              @change="updateField('templateId', ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="template in templates" :key="template.id" :value="String(template.id)">{{ template.name }}</option>
            </select>
          </div>
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-additional-namespaces`">附加命名空间</label>
            <textarea
              :id="`${dialogIdPrefix}-additional-namespaces`"
              :value="form.additionalNamespacesInput"
              class="rail-textarea"
              rows="3"
              placeholder="database:database&#10;cache:cache"
              @input="updateField('additionalNamespacesInput', ($event.target as HTMLTextAreaElement).value.trim())"
            ></textarea>
            <div class="form-helper">每行一个后缀，可写成 suffix:purpose；默认保留 app 工作负载空间。</div>
          </div>
          <div class="form-item">
            <div class="form-label-row">
              <label class="form-label">平台公共服务</label>
              <span v-if="sharedResourcesLoading" class="form-helper-inline">读取中...</span>
            </div>
            <div v-if="sharedResources.length" class="shared-service-grid">
              <label
                v-for="resource in sharedResources"
                :key="resource.id"
                class="shared-service-option"
                :class="{ active: form.sharedResourceIds.includes(String(resource.id)) }"
              >
                <input
                  type="checkbox"
                  :value="String(resource.id)"
                  :checked="form.sharedResourceIds.includes(String(resource.id))"
                  @change="toggleSharedResource(String(resource.id), ($event.target as HTMLInputElement).checked)"
                />
                <span>
                  <strong>{{ resource.serviceName || resource.serviceType }}</strong>
                  <small>{{ [resource.serviceType, resource.provider, resource.status].filter(Boolean).join(' · ') }}</small>
                </span>
              </label>
            </div>
            <div v-else class="form-helper">{{ sharedResourcesLoading ? '正在读取共享资源池。' : '共享资源池暂无可引用服务，环境创建后仍可在画布中添加。' }}</div>
            <div v-if="sharedResourcesError" class="form-helper form-helper--warning">{{ sharedResourcesError }}</div>
          </div>
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-ip-pool`">网络地址池</label>
            <input
              :id="`${dialogIdPrefix}-environment-ip-pool`"
              class="rail-input environment-ip-pool-state"
              value="暂未启用"
              readonly
              disabled
              :aria-describedby="`${dialogIdPrefix}-environment-ip-pool-helper`"
            />
            <div :id="`${dialogIdPrefix}-environment-ip-pool-helper`" class="form-helper">当前环境创建使用平台默认网络规划，自定义 IP 池将在后续版本启用。</div>
          </div>
          <div v-if="error" class="form-error" role="alert">{{ error }}</div>
        </div>
        <div class="modal-footer">
          <button type="button" class="rail-btn rail-btn--ghost" :disabled="creating" @click="emit('close')">取消</button>
          <button type="button" class="rail-btn rail-btn--primary" :disabled="!form.name || creating" @click="emit('submit')">
            {{ creating ? '创建中...' : '创建' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { toIdentifier } from '../utils/identifier'

type EnvironmentForm = {
  name: string
  identifier: string
  mode: string
  templateId: string
  additionalNamespacesInput: string
  sharedResourceIds: string[]
}

type SharedResource = {
  id: number | string
  capability?: string
  provider?: string
  serviceType?: string
  serviceName?: string
  status?: string
}

const props = defineProps<{
  visible: boolean
  creating: boolean
  error: string
  templates: Array<{ id: number | string; name: string }>
  sharedResources: SharedResource[]
  sharedResourcesLoading: boolean
  sharedResourcesError: string
  form: EnvironmentForm
  dialogIdPrefix: string
}>()

const emit = defineEmits<{
  (event: 'update:form', value: EnvironmentForm): void
  (event: 'close'): void
  (event: 'submit'): void
}>()

const identifierPreview = computed(() => toIdentifier(props.form.identifier || props.form.name, 'env'))

const updateField = (key: keyof EnvironmentForm, value: string) => {
  emit('update:form', { ...props.form, [key]: value })
}

const toggleSharedResource = (id: string, checked: boolean) => {
  const current = new Set(props.form.sharedResourceIds.map(String))
  if (checked) current.add(id)
  else current.delete(id)
  emit('update:form', { ...props.form, sharedResourceIds: Array.from(current) })
}
</script>

<style scoped>
.modal-container {
  width: min(520px, 100%);
  max-height: 90vh;
  overflow-y: auto;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  box-shadow: var(--paap-shadow-lg);
}

.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  padding: var(--paap-space-5) var(--paap-space-6);
  border-bottom: 1px solid var(--paap-border);
}

.modal-label {
  margin: 0 0 var(--paap-space-2);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  line-height: 1.333;
  letter-spacing: 0.32px;
  text-transform: uppercase;
}

.modal-heading {
  margin: 0;
  color: var(--paap-text);
  font-size: 20px;
  font-weight: 400;
  line-height: 1.4;
}

.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
}

.modal-close:hover {
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
}

.modal-close:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.modal-body {
  padding: var(--paap-space-6);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  padding: var(--paap-space-4) var(--paap-space-6);
  border-top: 1px solid var(--paap-border);
}

.form-item {
  display: grid;
  gap: var(--paap-space-2);
  margin-bottom: var(--paap-space-5);
}

.form-item:last-child {
  margin-bottom: 0;
}

.form-label {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 400;
  line-height: 1.333;
  letter-spacing: 0.32px;
}

.form-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-3);
}

.form-helper-inline {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.333;
}

.required,
.form-error {
  color: var(--paap-danger);
}

.form-helper {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.333;
  letter-spacing: 0.32px;
}

.form-helper--warning {
  color: var(--paap-warning);
}

.shared-service-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
}

.shared-service-option {
  display: flex;
  align-items: flex-start;
  gap: var(--paap-space-2);
  min-height: 58px;
  padding: 9px 10px;
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
}

.shared-service-option.active {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
  color: var(--paap-text);
}

.shared-service-option input {
  margin: 2px 0 0;
}

.shared-service-option span {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.shared-service-option strong,
.shared-service-option small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.shared-service-option strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
}

.shared-service-option small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.form-error {
  padding: 10px 12px;
  border: 1px solid var(--paap-danger);
  border-radius: 0;
  background: var(--paap-panel);
  font-size: var(--paap-fs-body);
  line-height: 1.285;
}

.radio-group {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--paap-space-2);
}

.radio-item {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 40px;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
}

.radio-item.active {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
  color: var(--paap-text);
}

.radio-item input {
  margin: 0;
}

@media (max-width: 672px) {
  .radio-group {
    grid-template-columns: 1fr;
  }

  .shared-service-grid {
    grid-template-columns: 1fr;
  }
}
</style>
